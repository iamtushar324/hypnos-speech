package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type transcriptionResult struct {
	Text     string
	Duration float64
}

var azureRegionRE = regexp.MustCompile(`^[a-z0-9-]+$`)

func (s *Server) handleTranscription(w http.ResponseWriter, r *http.Request) {
	if err := acquire(r.Context(), s.sttSlots); err != nil {
		writeError(w, http.StatusServiceUnavailable, "transcription capacity is busy")
		return
	}
	defer release(s.sttSlots)
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioBody)
	if err := r.ParseMultipartForm(maxAudioBody); err != nil {
		writeError(w, http.StatusBadRequest, "invalid or oversized multipart request")
		return
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "multipart file field is required")
		return
	}
	defer f.Close()
	audio, err := io.ReadAll(io.LimitReader(f, maxAudioBody+1))
	if err != nil || len(audio) == 0 || len(audio) > maxAudioBody {
		writeError(w, http.StatusBadRequest, "audio file is empty or too large")
		return
	}
	ct := h.Header.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(audio)
	}
	modelID := strings.TrimSpace(r.FormValue("model"))
	language := strings.TrimSpace(r.FormValue("language"))
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	profileID := strings.TrimSpace(r.FormValue("profile_id"))
	if len(modelID) > 200 || len(language) > 40 || len(prompt) > 10000 || len(profileID) > 100 {
		writeError(w, http.StatusBadRequest, "transcription option is too long")
		return
	}
	format := r.FormValue("response_format")
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "text" {
		writeError(w, 400, "response_format must be json or text")
		return
	}
	st := s.state.snapshot()
	profile, err := resolveProfile(st, profileID)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if modelID == "" && profile != nil {
		modelID = profile.STTModel
	}
	if modelID == "" {
		modelID = st.Settings.DefaultSTTModel
	}
	if language == "" && profile != nil {
		language = profile.Language
	}
	if language == "" {
		language = st.Settings.Language
	}
	m, ok := s.model(modelID, "stt")
	if !ok {
		writeError(w, 400, "unknown STT model")
		return
	}
	if !m.Configured {
		writeError(w, 503, "selected STT provider is not configured")
		return
	}
	if m.Streaming {
		writeError(w, 400, "streaming models must use /v1/audio/stream")
		return
	}
	id := newID()
	start := time.Now()
	if err := s.state.saveAudio(id, audio); err != nil {
		internalError(w, err)
		return
	}
	var result transcriptionResult
	if m.Provider == "groq" {
		result, err = s.groqTranscribe(r.Context(), st.Providers["groq"], modelID, language, prompt, h.Filename, ct, audio)
	} else {
		result, err = s.microsoftTranscribe(r.Context(), st.Providers["microsoft"], language, prompt, ct, audio, st.Dictionary)
	}
	status := "ok"
	safeErr := ""
	if err != nil {
		status = "error"
		safeErr = "provider request failed"
	}
	cleanup := cleanupResult{Text: "", RawText: result.Text, Status: "fallback", Error: "transcription_failed"}
	if err == nil {
		cleanup = s.cleanupTranscription(r.Context(), result.Text, st)
	}
	text := cleanup.Text
	latency := time.Since(start).Milliseconds()
	cost := 0.0
	if err == nil {
		cost = sttCost(modelID, result.Duration)
	}
	hist := History{ID: id, Kind: "stt", Model: modelID, Provider: m.Provider, CreatedAt: start.UTC(), Status: status, LatencyMS: latency, DurationSeconds: result.Duration, EstimatedCostUSD: cost, Error: safeErr, AudioAvailable: true, AudioContentType: ct}
	applyCleanupHistory(&hist, cleanup, st.Settings.SaveHistoryText)
	if saveErr := s.appendHistory(hist); saveErr != nil {
		internalError(w, saveErr)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "transcription provider could not complete the request")
		return
	}
	if format == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Speech-ID", id)
		w.Header().Set("X-Speech-Cleanup-Status", cleanup.Status)
		w.Header().Set("X-Speech-Cleanup-Model", cleanup.Model)
		w.Header().Set("X-Speech-Cleanup-Latency-Ms", fmt.Sprint(cleanup.LatencyMS))
		if cleanup.Error != "" {
			w.Header().Set("X-Speech-Cleanup-Error", cleanup.Error)
		}
		if cleanup.CostUSD != nil {
			w.Header().Set("X-Speech-Cleanup-Cost-USD", strconv.FormatFloat(*cleanup.CostUSD, 'f', -1, 64))
		}
		_, _ = io.WriteString(w, text)
		return
	}
	response := map[string]any{"text": text, "model": modelID, "provider": m.Provider, "latency_ms": latency, "id": id}
	for k, v := range cleanupResponse(cleanup) {
		response[k] = v
	}
	writeJSON(w, 200, response)
}

func resolveProfile(st persistedState, id string) (*Profile, error) {
	if id == "" {
		if st.Settings.DefaultProfileID == "" {
			return nil, nil
		}
		id = st.Settings.DefaultProfileID
	}
	for i := range st.Profiles {
		if st.Profiles[i].ID == id {
			return &st.Profiles[i], nil
		}
	}
	return nil, errors.New("profile not found")
}

func (s *Server) groqTranscribe(ctx context.Context, p providerState, model, language, prompt, filename, contentType string, audio []byte) (transcriptionResult, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return transcriptionResult{}, err
	}
	if _, err = part.Write(audio); err != nil {
		return transcriptionResult{}, err
	}
	_ = mw.WriteField("model", model)
	_ = mw.WriteField("response_format", "verbose_json")
	if language != "" && language != "auto" {
		_ = mw.WriteField("language", normalizeGroqLanguage(language))
	}
	if prompt != "" {
		_ = mw.WriteField("prompt", prompt)
	}
	if err = mw.Close(); err != nil {
		return transcriptionResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.Endpoint, "/")+"/audio/transcriptions", &body)
	if err != nil {
		return transcriptionResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := s.client.Do(req)
	if err != nil {
		return transcriptionResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return transcriptionResult{}, fmt.Errorf("groq returned status %d", resp.StatusCode)
	}
	var out struct {
		Text     string  `json:"text"`
		Duration float64 `json:"duration"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return transcriptionResult{}, err
	}
	if strings.TrimSpace(out.Text) == "" {
		return transcriptionResult{}, errors.New("provider returned an empty transcript")
	}
	return transcriptionResult{Text: out.Text, Duration: out.Duration}, nil
}

func normalizeGroqLanguage(v string) string {
	v = strings.ToLower(v)
	switch v {
	case "en-in", "en-us", "en-gb":
		return "en"
	case "hi-in":
		return "hi"
	}
	return v
}

func (s *Server) microsoftTranscribe(ctx context.Context, p providerState, language, prompt, contentType string, audio []byte, dictionary []DictionaryEntry) (transcriptionResult, error) {
	if !supportedMicrosoftAudio(contentType) {
		return transcriptionResult{}, errors.New("Microsoft MAI batch supports WAV, MP3, FLAC, OGG, or WebM audio")
	}
	phrases := make([]string, 0, len(dictionary)+1)
	for _, d := range dictionary {
		if strings.TrimSpace(d.Word) != "" {
			phrases = append(phrases, d.Word)
		}
	}
	if prompt != "" {
		phrases = append(phrases, prompt)
	}
	locales := []string{}
	if language != "" && language != "auto" {
		switch strings.ToLower(language) {
		case "hi", "hi-in":
			locales = []string{"hi-IN"}
		case "en", "en-in":
			locales = []string{"en-IN"}
		case "en-us":
			locales = []string{"en-US"}
		default:
			return transcriptionResult{}, errors.New("unsupported Microsoft transcription language")
		}
	}
	batchModel := p.BatchDeployment
	if batchModel == "" {
		batchModel = "MAI-Transcribe-2"
	}
	definition := map[string]any{"enhancedMode": map[string]any{"enabled": true, "task": "transcribe", "model": batchModel}, "modelOptions": map[string]any{"transcribeStyle": "clean", "timestamps": "none"}, "phraseList": map[string]any{"phrases": phrases}, "locales": locales}
	defJSON, _ := json.Marshal(definition)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	header := make(map[string][]string)
	header["Content-Disposition"] = []string{`form-data; name="audio"; filename="audio` + audioExtension(contentType) + `"`}
	header["Content-Type"] = []string{contentType}
	part, err := mw.CreatePart(textproto.MIMEHeader(header))
	if err != nil {
		return transcriptionResult{}, err
	}
	if _, err = part.Write(audio); err != nil {
		return transcriptionResult{}, err
	}
	_ = mw.WriteField("definition", string(defJSON))
	if err = mw.Close(); err != nil {
		return transcriptionResult{}, err
	}
	endpoint, err := microsoftBatchURL(p.Endpoint)
	if err != nil {
		return transcriptionResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return transcriptionResult{}, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", p.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := s.client.Do(req)
	if err != nil {
		return transcriptionResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return transcriptionResult{}, fmt.Errorf("microsoft returned status %d", resp.StatusCode)
	}
	var out struct {
		CombinedPhrases []struct {
			Text string `json:"text"`
		} `json:"combinedPhrases"`
		DurationMilliseconds int64 `json:"durationMilliseconds"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&out); err != nil {
		return transcriptionResult{}, err
	}
	var texts []string
	for _, v := range out.CombinedPhrases {
		if strings.TrimSpace(v.Text) != "" {
			texts = append(texts, v.Text)
		}
	}
	if len(texts) == 0 {
		return transcriptionResult{}, errors.New("provider returned an empty transcript")
	}
	return transcriptionResult{Text: strings.Join(texts, " "), Duration: float64(out.DurationMilliseconds) / 1000}, nil
}

func supportedMicrosoftAudio(ct string) bool {
	ct = strings.ToLower(strings.Split(ct, ";")[0])
	switch ct {
	case "audio/wav", "audio/x-wav", "audio/mpeg", "audio/mp3", "audio/flac", "audio/x-flac", "audio/ogg", "audio/webm", "video/webm", "application/octet-stream":
		return true
	}
	return false
}
func audioExtension(ct string) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.Contains(ct, "wav"):
		return ".wav"
	case strings.Contains(ct, "mpeg") || strings.Contains(ct, "mp3"):
		return ".mp3"
	case strings.Contains(ct, "flac"):
		return ".flac"
	case strings.Contains(ct, "ogg"):
		return ".ogg"
	case strings.Contains(ct, "webm"):
		return ".webm"
	}
	return ".bin"
}

func microsoftBatchURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := u.Host
	if strings.HasSuffix(u.Hostname(), ".services.ai.azure.com") {
		name := strings.TrimSuffix(u.Hostname(), ".services.ai.azure.com")
		host = name + ".cognitiveservices.azure.com"
	}
	u.Scheme = "https"
	u.Host = host
	u.Path = "/speechtotext/transcriptions:transcribe"
	u.RawQuery = "api-version=2025-10-15"
	return u.String(), nil
}

func (s *Server) handleSpeech(w http.ResponseWriter, r *http.Request) {
	if err := acquire(r.Context(), s.ttsSlots); err != nil {
		writeError(w, 503, "speech synthesis capacity is busy")
		return
	}
	defer release(s.ttsSlots)
	var in struct {
		Model          string `json:"model"`
		Input          string `json:"input"`
		Voice          string `json:"voice"`
		ResponseFormat string `json:"response_format"`
		ProfileID      string `json:"profile_id"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Input = strings.TrimSpace(in.Input)
	if in.Input == "" || len([]rune(in.Input)) > 4000 {
		writeError(w, 400, "input must contain between 1 and 4000 characters")
		return
	}
	if in.ResponseFormat == "" {
		in.ResponseFormat = "wav"
	}
	if in.ResponseFormat != "wav" {
		writeError(w, 400, "only wav response_format is supported")
		return
	}
	st := s.state.snapshot()
	profile, err := resolveProfile(st, in.ProfileID)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if in.Model == "" && profile != nil {
		in.Model = profile.TTSModel
	}
	if in.Model == "" {
		in.Model = st.Settings.DefaultTTSModel
	}
	if in.Voice == "" && profile != nil {
		in.Voice = profile.Voice
	}
	if in.Voice == "" {
		in.Voice = st.Settings.DefaultVoice
	}
	m, ok := s.model(in.Model, "tts")
	if !ok {
		writeError(w, 400, "unknown TTS model")
		return
	}
	if !m.Configured {
		writeError(w, 503, "selected TTS provider is not configured")
		return
	}
	id := newID()
	start := time.Now()
	var audio []byte
	if m.Provider == "groq" {
		audio, err = s.groqSpeech(r.Context(), st.Providers["groq"], in.Model, in.Input, in.Voice)
	} else {
		audio, err = s.microsoftSpeech(r.Context(), st.Providers["microsoft"], in.Model, in.Input, in.Voice)
	}
	latency := time.Since(start).Milliseconds()
	status := "ok"
	safeErr := ""
	if err != nil {
		status = "error"
		safeErr = "provider request failed"
	}
	histText := in.Input
	if !st.Settings.SaveHistoryText {
		histText = ""
	}
	cost := 0.0
	if err == nil {
		cost = ttsCost(in.Model, len([]rune(in.Input)))
	}
	hist := History{ID: id, Kind: "tts", Model: in.Model, Provider: m.Provider, CreatedAt: start.UTC(), Status: status, LatencyMS: latency, InputChars: len([]rune(in.Input)), Text: histText, EstimatedCostUSD: cost, Error: safeErr, Voice: in.Voice, AudioAvailable: err == nil, AudioContentType: "audio/wav"}
	if err == nil {
		if saveErr := s.state.saveAudio(id, audio); saveErr != nil {
			internalError(w, saveErr)
			return
		}
	}
	if saveErr := s.appendHistory(hist); saveErr != nil {
		internalError(w, saveErr)
		return
	}
	if err != nil {
		writeError(w, 502, "speech provider could not complete the request")
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("X-Speech-ID", id)
	w.Header().Set("X-Speech-Latency-Ms", fmt.Sprint(latency))
	w.Header().Set("Content-Length", fmt.Sprint(len(audio)))
	w.WriteHeader(200)
	_, _ = w.Write(audio)
}

func (s *Server) groqSpeech(ctx context.Context, p providerState, model, input, voice string) ([]byte, error) {
	chunks := sentenceChunks(input, 200)
	waves := make([][]byte, 0, len(chunks))
	total := 0
	for _, chunk := range chunks {
		payload, _ := json.Marshal(map[string]any{"model": model, "input": chunk, "voice": voice, "response_format": "wav"})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.Endpoint, "/")+"/audio/speech", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}
		remaining := (64 << 20) - total
		if remaining <= 0 {
			resp.Body.Close()
			return nil, errors.New("provider audio response is too large")
		}
		b, readErr := io.ReadAll(io.LimitReader(resp.Body, int64(remaining+1)))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("groq returned status %d", resp.StatusCode)
		}
		if len(b) > remaining {
			return nil, errors.New("provider audio response is too large")
		}
		total += len(b)
		waves = append(waves, b)
	}
	return concatenateWAV(waves)
}

func (s *Server) microsoftSpeech(ctx context.Context, p providerState, model, input, voice string) ([]byte, error) {
	if !azureRegionRE.MatchString(p.Region) {
		return nil, errors.New("invalid Azure region")
	}
	azureVoice, err := microsoftVoice(voice, model)
	if err != nil {
		return nil, err
	}
	escaped := new(bytes.Buffer)
	_ = xml.EscapeText(escaped, []byte(input))
	ssml := `<speak version="1.0" xml:lang="en-IN"><voice name="` + azureVoice + `">` + escaped.String() + `</voice></speak>`
	endpoint := "https://" + p.Region + ".tts.speech.microsoft.com/cognitiveservices/v1"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(ssml))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", p.APIKey)
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "riff-24khz-16bit-mono-pcm")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("microsoft returned status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (64<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 64<<20 {
		return nil, errors.New("provider audio response is too large")
	}
	if _, err := parseWAV(b); err != nil {
		return nil, err
	}
	return b, nil
}

func microsoftVoice(voice, model string) (string, error) {
	suffix := "MAI-Voice-2.1"
	if model == "mai-voice-2.1-flash" {
		suffix = "MAI-Voice-2.1-Flash"
	}
	v := strings.ToLower(strings.TrimSpace(voice))
	switch v {
	case "harper", "en-us-harper":
		return "en-US-Harper:" + suffix, nil
	case "dhruv", "en-in-dhruv", "troy":
		return "en-IN-Dhruv:" + suffix, nil
	case "hi-in-dhruv", "dhruv-hindi":
		return "hi-IN-Dhruv:" + suffix, nil
	default:
		return "", errors.New("unsupported Microsoft voice")
	}
}

func sentenceChunks(s string, max int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	out := []string{}
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, word := range words {
		wordRunes := []rune(word)
		for len(wordRunes) > max {
			flush()
			out = append(out, string(wordRunes[:max]))
			wordRunes = wordRunes[max:]
		}
		word = string(wordRunes)
		extra := len(wordRunes)
		if cur.Len() > 0 {
			extra++
		}
		if len([]rune(cur.String()))+extra > max {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(word)
		if strings.HasSuffix(word, ".") || strings.HasSuffix(word, "!") || strings.HasSuffix(word, "?") {
			flush()
		}
	}
	flush()
	return out
}

func (s *Server) appendHistory(h History) error {
	return s.state.update(func(st *persistedState) error {
		if st.AudioTypes == nil {
			st.AudioTypes = map[string]string{}
		}
		if h.AudioContentType != "" {
			st.AudioTypes[h.ID] = h.AudioContentType
		}
		st.History = append(st.History, h)
		return nil
	})
}
func sttCost(model string, seconds float64) float64 {
	rate := 0.0
	switch model {
	case "whisper-large-v3-turbo":
		rate = .04
		if seconds < 10 {
			seconds = 10
		}
	case "whisper-large-v3":
		rate = .111
		if seconds < 10 {
			seconds = 10
		}
	case "mai-transcribe-2":
		rate = .10
	case "mai-transcribe-2-streaming":
		rate = .54
	}
	return rate * seconds / 3600
}
func ttsCost(model string, chars int) float64 {
	rate := 22.0
	if model == "mai-voice-2.1-flash" {
		rate = 15
	}
	return rate * float64(chars) / 1_000_000
}

func applyVocabulary(text string, dictionary []DictionaryEntry, snippets []Snippet) string {
	if snippet, ok := exactSnippet(text, snippets); ok {
		return snippet
	}
	return applyDictionary(text, dictionary)
}
func replaceWholeWord(s, word, repl string) string {
	if word == "" {
		return s
	}
	source := []rune(s)
	needle := []rune(word)
	var out strings.Builder
	for pos := 0; pos < len(source); {
		end := pos + len(needle)
		left := pos == 0 || !isWordRune(source[pos-1])
		right := end == len(source) || (end < len(source) && !isWordRune(source[end]))
		if end <= len(source) && left && right && strings.EqualFold(string(source[pos:end]), word) {
			out.WriteString(repl)
			pos = end
		} else {
			out.WriteRune(source[pos])
			pos++
		}
	}
	return out.String()
}
func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' }
