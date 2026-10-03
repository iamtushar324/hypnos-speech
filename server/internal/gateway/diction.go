package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// DictionHandler exposes the deliberately small compatibility surface used by
// Diction's iOS keyboard. It must be mounted below /diction with StripPrefix.
// Device keys accepted here remain speech-only and cannot reach administration.
func (s *Server) DictionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		switch r.URL.Path {
		case "/":
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				methodNotAllowed(w)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"service": "Hypnos Speech for Diction", "endpoint": s.cfg.DictionPublicURL, "health": "/health", "models": "/v1/models", "pairing_required": true})
		case "/health":
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				methodNotAllowed(w)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": "hypnos-speech/" + version})
		case "/v1/models":
			if r.Method != http.MethodGet {
				methodNotAllowed(w)
				return
			}
			s.dictionModels(w)
		case "/v1/auth/key":
			if r.Method != http.MethodGet {
				methodNotAllowed(w)
				return
			}
			token, ok := s.dictionDeviceToken(r)
			if !ok {
				unauthorized(w)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"key": token, "rotation": false})
		case "/v1/audio/transcriptions":
			if r.Method != http.MethodPost {
				methodNotAllowed(w)
				return
			}
			if _, ok := s.dictionDeviceToken(r); !ok {
				unauthorized(w)
				return
			}
			if dictionUnsupportedIntent(r.URL.Query()) {
				writeError(w, http.StatusBadRequest, "Diction editing and selected-text operations are not supported")
				return
			}
			s.dictionTranscription(w, r)
		case "/v1/audio/stream":
			if r.Method != http.MethodGet {
				methodNotAllowed(w)
				return
			}
			s.dictionStream(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func (s *Server) dictionModels(w http.ResponseWriter) {
	type openAIModel struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}
	type modelInfo struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Available   bool   `json:"available"`
	}
	type providerGroup struct {
		ID     string      `json:"id"`
		Name   string      `json:"name"`
		Models []modelInfo `json:"models"`
	}
	data := make([]openAIModel, 0)
	groups := map[string]*providerGroup{}
	providerNames := map[string]string{"groq": "Groq", "microsoft": "Microsoft Azure Speech"}
	for _, model := range s.models() {
		// Diction's final-only wire contract cannot expose incremental results.
		// The MAI streaming model remains available through the native API.
		if model.Kind != "stt" || model.Streaming || !model.Configured || (model.Provider == "groq" && model.ID != "whisper-large-v3-turbo") {
			continue
		}
		data = append(data, openAIModel{ID: model.ID, Object: "model", Created: 0, OwnedBy: model.Provider})
		group := groups[model.Provider]
		if group == nil {
			group = &providerGroup{ID: model.Provider, Name: providerNames[model.Provider]}
			groups[model.Provider] = group
		}
		group.Models = append(group.Models, modelInfo{ID: model.ID, Name: model.Name, Description: model.Description, Available: true})
	}
	providers := make([]providerGroup, 0, 2)
	for _, id := range []string{"groq", "microsoft"} {
		if group := groups[id]; group != nil {
			providers = append(providers, *group)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list", "data": data, "providers": providers,
		"capabilities": map[string]bool{
			"llm": false, "text_process": false, "text_suggest": false,
			"text_summarize": false, "formatting": false, "pairing": true, "key_rotation": false,
		},
	})
}

func (s *Server) dictionDeviceToken(r *http.Request) (string, bool) {
	token := bearerToken(r.Header.Get("Authorization"))
	if !strings.HasPrefix(token, "spk_") || !s.useDeviceKey(token) {
		return "", false
	}
	return token, true
}

func (s *Server) dictionTranscription(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioBody)
	if err := r.ParseMultipartForm(maxAudioBody); err != nil {
		writeError(w, http.StatusBadRequest, "invalid or oversized multipart request")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	model, err := s.dictionModel(r.FormValue("model"), false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if r.Form == nil {
		r.Form = url.Values{}
	}
	r.Form.Set("model", model.ID)
	language, err := dictionLanguage(r.FormValue("language"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	r.Form.Set("language", language)
	if r.MultipartForm != nil {
		if r.MultipartForm.Value == nil {
			r.MultipartForm.Value = map[string][]string{}
		}
		r.MultipartForm.Value["model"] = []string{model.ID}
		r.MultipartForm.Value["language"] = []string{language}
	}
	s.handleTranscription(w, r)
}

func (s *Server) dictionStream(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.dictionDeviceToken(r); !ok {
		unauthorized(w)
		return
	}
	if !s.dictionValidOrigin(r) {
		writeError(w, http.StatusForbidden, "request origin is not allowed")
		return
	}
	query := r.URL.Query()
	if dictionUnsupportedIntent(query) {
		writeError(w, http.StatusBadRequest, "Diction editing and selected-text operations are not supported")
		return
	}
	if err := dictionCodec(query.Get("codec"), r.Header.Get("Sec-WebSocket-Protocol")); err != nil {
		writeError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}
	model, err := s.dictionModel(query.Get("model"), true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !model.Configured {
		writeError(w, http.StatusServiceUnavailable, "selected transcription provider is not configured")
		return
	}
	language, err := dictionLanguage(query.Get("language"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sampleRate, err := dictionSampleRate(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// No subprotocol is selected. Diction clients offering diction.opus.v1 then
	// use their documented PCM fallback instead of sending undecodable Opus.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.dictionOriginPatterns()})
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(maxAudioBody)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	if err := acquire(ctx, s.sttSlots); err != nil {
		dictionWSError(ctx, c, "transcription capacity is busy")
		return
	}
	defer release(s.sttSlots)

	var pcm bytes.Buffer
	contextSeen := false
	for {
		readCtx, readCancel := context.WithTimeout(ctx, 60*time.Second)
		typ, data, readErr := c.Read(readCtx)
		readCancel()
		if readErr != nil {
			return
		}
		switch typ {
		case websocket.MessageBinary:
			contextSeen = true
			if pcm.Len()+len(data) > maxAudioBody {
				dictionWSError(ctx, c, "audio stream is too large")
				_ = c.Close(websocket.StatusMessageTooBig, "audio stream is too large")
				return
			}
			_, _ = pcm.Write(data)
		case websocket.MessageText:
			if len(data) > 64<<10 {
				dictionWSError(ctx, c, "context message is too large")
				_ = c.Close(websocket.StatusPolicyViolation, "context message is too large")
				return
			}
			var message struct {
				Action   string `json:"action"`
				Language string `json:"language"`
			}
			if err := json.Unmarshal(data, &message); err != nil {
				if !contextSeen && pcm.Len() == 0 {
					contextSeen = true
					continue
				}
				dictionWSError(ctx, c, "invalid Diction control message")
				return
			}
			if message.Action != "done" {
				// Diction may send one descriptive context object before audio. It
				// is deliberately ignored: this adapter offers transcription only.
				if !contextSeen && pcm.Len() == 0 {
					contextSeen = true
					continue
				}
				dictionWSError(ctx, c, "unexpected Diction control message")
				return
			}
			if message.Language != "" {
				language, err = dictionLanguage(message.Language)
				if err != nil {
					dictionWSError(ctx, c, err.Error())
					return
				}
			}
			if pcm.Len() == 0 || pcm.Len()%2 != 0 {
				dictionWSError(ctx, c, "audio stream is empty or contains a partial PCM16 frame")
				return
			}
			s.dictionFinish(ctx, c, model, language, sampleRate, pcm.Bytes())
			return
		}
	}
}

func (s *Server) dictionValidOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	if s.validOrigin(r) {
		return true
	}
	want, wantErr := url.Parse(s.cfg.DictionPublicURL)
	got, gotErr := url.Parse(origin)
	return wantErr == nil && gotErr == nil && want.Scheme != "" && want.Host != "" &&
		strings.EqualFold(want.Scheme, got.Scheme) && strings.EqualFold(want.Host, got.Host)
}

func (s *Server) dictionOriginPatterns() []string {
	patterns := make([]string, 0, 2)
	for _, raw := range []string{s.cfg.PublicURL, s.cfg.DictionPublicURL} {
		if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
			patterns = append(patterns, u.Hostname())
		}
	}
	return patterns
}

func (s *Server) dictionFinish(ctx context.Context, c *websocket.Conn, model Model, language string, sampleRate int, pcm []byte) {
	state := s.state.snapshot()
	audio := pcm16WAV(pcm, sampleRate)
	id := newID()
	started := time.Now()
	if err := s.state.saveAudio(id, audio); err != nil {
		dictionWSError(ctx, c, "audio history could not be saved")
		return
	}
	var result transcriptionResult
	var err error
	switch model.Provider {
	case "groq":
		result, err = s.groqTranscribe(ctx, state.Providers["groq"], model.ID, normalizeGroqLanguage(language), "", "diction.wav", "audio/wav", audio)
	case "microsoft":
		result, err = s.microsoftTranscribe(ctx, state.Providers["microsoft"], language, "", "audio/wav", audio, state.Dictionary)
	default:
		err = errors.New("unsupported transcription provider")
	}
	duration := float64(len(pcm)) / float64(sampleRate*2)
	status, safeError, cost := "ok", "", 0.0
	if err != nil {
		status, safeError = "error", "provider request failed"
	} else {
		cost = sttCost(model.ID, duration)
	}
	cleanup := cleanupResult{RawText: result.Text, Status: "fallback", Error: "transcription_failed"}
	if err == nil {
		cleanup = s.cleanupTranscription(ctx, result.Text, state)
	}
	text := cleanup.Text
	latency := time.Since(started).Milliseconds()
	history := History{ID: id, Kind: "stt", Model: model.ID, Provider: model.Provider, CreatedAt: started.UTC(), Status: status, LatencyMS: latency, DurationSeconds: duration, EstimatedCostUSD: cost, Error: safeError, AudioAvailable: true, AudioContentType: "audio/wav"}
	applyCleanupHistory(&history, cleanup, state.Settings.SaveHistoryText)
	if saveErr := s.appendHistory(history); saveErr != nil {
		dictionWSError(ctx, c, "history could not be saved")
		return
	}
	if err != nil {
		dictionWSError(ctx, c, "transcription provider could not complete the request")
		return
	}
	_ = dictionWSWrite(ctx, c, map[string]any{"text": text, "mode": "transcribe"})
	_ = c.Close(websocket.StatusNormalClosure, "complete")
}

func (s *Server) dictionModel(raw string, stream bool) (Model, error) {
	id := strings.ToLower(strings.TrimSpace(raw))
	switch id {
	case "", "auto", "default", "turbo", "whisper-1", "large-v3-turbo", "groq/whisper-large-v3-turbo", "groq:whisper-large-v3-turbo", "parakeet", "parakeet-tdt-0.6b-v3", "nvidia/parakeet-tdt-0.6b-v3":
		id = "whisper-large-v3-turbo"
	case "whisper", "large-v3", "groq/whisper-large-v3", "groq:whisper-large-v3":
		id = "whisper-large-v3"
	case "microsoft/mai-transcribe-2", "microsoft:mai-transcribe-2":
		id = "mai-transcribe-2"
	}
	model, ok := s.model(id, "stt")
	if !ok {
		return Model{}, fmt.Errorf("unknown transcription model %q", raw)
	}
	if stream && model.Streaming {
		return Model{}, errors.New("Diction requires a final-only transcription model")
	}
	return model, nil
}

func dictionLanguage(raw string) (string, error) {
	language := strings.ToLower(strings.TrimSpace(raw))
	switch language {
	case "", "auto":
		return "auto", nil
	case "en", "en-in", "en-us", "en-gb":
		return "en", nil
	case "hi", "hi-in":
		return "hi", nil
	default:
		return "", fmt.Errorf("unsupported transcription language %q", raw)
	}
}

func dictionCodec(raw, subprotocolHeader string) error {
	codec := strings.ToLower(strings.TrimSpace(raw))
	if codec == "" {
		codec = "pcm"
	}
	switch codec {
	case "pcm", "pcm16", "pcm_s16le", "linear16":
		return nil
	case "opus":
		for _, offered := range strings.Split(subprotocolHeader, ",") {
			if strings.EqualFold(strings.TrimSpace(offered), "diction.opus.v1") {
				// Deliberately leave the subprotocol unselected. Current Diction
				// then sends its PCM fallback even though codec=opus remains in
				// the original query string.
				return nil
			}
		}
		return errors.New("Opus requires Diction's PCM fallback negotiation")
	default:
		return errors.New("unsupported Diction audio codec")
	}
}

func dictionSampleRate(query url.Values) (int, error) {
	raw := query.Get("sample_rate")
	if raw == "" {
		raw = query.Get("sampleRate")
	}
	if raw == "" {
		return 16000, nil
	}
	rate, err := strconv.Atoi(raw)
	if err != nil || rate < 8000 || rate > 48000 {
		return 0, errors.New("sample rate must be between 8000 and 48000 Hz")
	}
	return rate, nil
}

func dictionUnsupportedIntent(query url.Values) bool {
	for _, key := range []string{"intent", "mode", "action", "operation"} {
		value := strings.ToLower(strings.TrimSpace(query.Get(key)))
		if strings.Contains(value, "edit") || strings.Contains(value, "select") || strings.Contains(value, "rewrite") {
			return true
		}
	}
	for _, key := range []string{"selected", "selection", "selected_text", "selectedText"} {
		if strings.TrimSpace(query.Get(key)) != "" {
			return true
		}
	}
	return false
}

func dictionWSWrite(ctx context.Context, c *websocket.Conn, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.Write(writeCtx, websocket.MessageText, payload)
}

func dictionWSError(ctx context.Context, c *websocket.Conn, message string) {
	_ = dictionWSWrite(ctx, c, map[string]any{"error": message})
}
