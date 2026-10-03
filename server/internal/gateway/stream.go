package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type streamStart struct {
	Type      string `json:"type"`
	Model     string `json:"model"`
	Language  string `json:"language"`
	ProfileID string `json:"profile_id"`
	Action    string `json:"action"`
}
type streamEvent struct {
	Type       string          `json:"type"`
	Delta      string          `json:"delta"`
	Transcript string          `json:"transcript"`
	Text       string          `json:"text"`
	Error      json.RawMessage `json:"error"`
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	public, _ := url.Parse(s.cfg.PublicURL)
	opts := &websocket.AcceptOptions{OriginPatterns: []string{public.Hostname()}}
	c, err := websocket.Accept(w, r, opts)
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(maxAudioBody)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	token := bearerToken(r.Header.Get("Authorization"))
	if token != "" {
		if !s.authorizeToken(ctx, token) {
			s.wsError(ctx, c, "authentication required")
			_ = c.Close(websocket.StatusPolicyViolation, "authentication required")
			return
		}
	} else {
		authCtx, authCancel := context.WithTimeout(ctx, 5*time.Second)
		typ, data, readErr := c.Read(authCtx)
		authCancel()
		if readErr != nil || typ != websocket.MessageText {
			s.wsError(ctx, c, "authentication required")
			_ = c.Close(websocket.StatusPolicyViolation, "authentication required")
			return
		}
		var msg struct {
			Type  string `json:"type"`
			Token string `json:"token"`
		}
		if json.Unmarshal(data, &msg) != nil || msg.Type != "auth" || !s.authorizeToken(ctx, msg.Token) {
			s.wsError(ctx, c, "authentication required")
			_ = c.Close(websocket.StatusPolicyViolation, "authentication required")
			return
		}
	}
	startCtx, startCancel := context.WithTimeout(ctx, 10*time.Second)
	typ, data, readErr := c.Read(startCtx)
	startCancel()
	if readErr != nil || typ != websocket.MessageText {
		s.wsError(ctx, c, "start message required")
		return
	}
	var start streamStart
	if json.Unmarshal(data, &start) != nil || start.Type != "start" {
		s.wsError(ctx, c, "start message required")
		return
	}
	st := s.state.snapshot()
	profile, pErr := resolveProfile(st, start.ProfileID)
	if pErr != nil {
		s.wsError(ctx, c, pErr.Error())
		return
	}
	if start.Model == "" && profile != nil {
		start.Model = profile.STTModel
	}
	if start.Model == "" {
		start.Model = st.Settings.DefaultSTTModel
	}
	if start.Language == "" && profile != nil {
		start.Language = profile.Language
	}
	if start.Language == "" {
		start.Language = st.Settings.Language
	}
	model, ok := s.model(start.Model, "stt")
	if !ok {
		s.wsError(ctx, c, "unknown STT model")
		return
	}
	if !model.Configured {
		s.wsError(ctx, c, "selected STT provider is not configured")
		return
	}
	if err := acquire(ctx, s.sttSlots); err != nil {
		s.wsError(ctx, c, "transcription capacity is busy")
		return
	}
	defer release(s.sttSlots)
	if model.ID == "mai-transcribe-2-streaming" {
		s.microsoftStream(ctx, c, start, st, model)
		return
	}
	if model.Provider == "groq" {
		s.groqBatchStream(ctx, c, start, st, model)
		return
	}
	s.wsError(ctx, c, "selected model does not support this streaming API")
}

func (s *Server) authorizeToken(ctx context.Context, token string) bool {
	if token == "" {
		return false
	}
	if _, err := s.cfg.VerifyOwner(ctx, token); err == nil {
		return true
	}
	return strings.HasPrefix(token, "spk_") && s.useDeviceKey(token)
}
func (s *Server) wsWrite(ctx context.Context, c *websocket.Conn, v any) error {
	b, _ := json.Marshal(v)
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.Write(writeCtx, websocket.MessageText, b)
}
func (s *Server) wsError(ctx context.Context, c *websocket.Conn, message string) {
	_ = s.wsWrite(ctx, c, map[string]any{"type": "error", "message": message})
}

func (s *Server) groqBatchStream(ctx context.Context, c *websocket.Conn, start streamStart, st persistedState, model Model) {
	if err := s.wsWrite(ctx, c, map[string]any{"type": "ready", "mode": "batch"}); err != nil {
		return
	}
	var pcm bytes.Buffer
	for {
		readCtx, readCancel := context.WithTimeout(ctx, 60*time.Second)
		typ, data, err := c.Read(readCtx)
		readCancel()
		if err != nil {
			return
		}
		if typ == websocket.MessageBinary {
			if pcm.Len()+len(data) > maxAudioBody {
				s.wsError(ctx, c, "audio stream is too large")
				return
			}
			pcm.Write(data)
			continue
		}
		var cmd streamStart
		if json.Unmarshal(data, &cmd) != nil {
			continue
		}
		if cmd.Action != "done" && cmd.Type != "commit" {
			continue
		}
		break
	}
	if pcm.Len() == 0 {
		s.wsError(ctx, c, "audio stream is empty")
		return
	}
	audio := pcm16WAV(pcm.Bytes(), 16000)
	id := newID()
	started := time.Now()
	if err := s.state.saveAudio(id, audio); err != nil {
		s.wsError(ctx, c, "audio history could not be saved")
		return
	}
	res, err := s.groqTranscribe(ctx, st.Providers["groq"], model.ID, start.Language, "", "stream.wav", "audio/wav", audio)
	status := "ok"
	safeErr := ""
	if err != nil {
		status = "error"
		safeErr = "provider request failed"
	}
	cleanup := cleanupResult{RawText: res.Text, Status: "fallback", Error: "transcription_failed"}
	if err == nil {
		cleanup = s.cleanupTranscription(ctx, res.Text, st)
	}
	text := cleanup.Text
	latency := time.Since(started).Milliseconds()
	baseCost := 0.0
	if err == nil {
		baseCost = sttCost(model.ID, float64(pcm.Len())/32000)
	}
	h := History{ID: id, Kind: "stt", Model: model.ID, Provider: model.Provider, CreatedAt: started.UTC(), Status: status, LatencyMS: latency, DurationSeconds: float64(pcm.Len()) / 32000, EstimatedCostUSD: baseCost, Error: safeErr, AudioAvailable: true, AudioContentType: "audio/wav"}
	applyCleanupHistory(&h, cleanup, st.Settings.SaveHistoryText)
	if saveErr := s.appendHistory(h); saveErr != nil {
		s.wsError(ctx, c, "history could not be saved")
		return
	}
	if err != nil {
		s.wsError(ctx, c, "transcription provider could not complete the request")
		return
	}
	frame := map[string]any{"type": "final", "text": text, "model": model.ID, "provider": model.Provider, "id": id}
	for k, v := range cleanupResponse(cleanup) {
		frame[k] = v
	}
	_ = s.wsWrite(ctx, c, frame)
	_ = c.Close(websocket.StatusNormalClosure, "complete")
}

func (s *Server) microsoftStream(ctx context.Context, c *websocket.Conn, start streamStart, st persistedState, model Model) {
	p := st.Providers["microsoft"]
	upURL, err := microsoftStreamURL(p.Endpoint)
	if err != nil {
		s.wsError(ctx, c, "Microsoft streaming endpoint is invalid")
		return
	}
	headers := http.Header{}
	headers.Set("api-key", p.APIKey)
	wsClient := &http.Client{Transport: s.client.Transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	up, dialResp, err := websocket.Dial(ctx, upURL, &websocket.DialOptions{HTTPHeader: headers, HTTPClient: wsClient})
	if dialResp != nil && dialResp.Body != nil {
		dialResp.Body.Close()
	}
	if err != nil {
		s.wsError(ctx, c, "Microsoft streaming provider is unavailable")
		return
	}
	defer up.CloseNow()
	up.SetReadLimit(4 << 20)
	sessionCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err = waitForProviderEvent(sessionCtx, up, "session.created"); err != nil {
		s.wsError(ctx, c, "Microsoft streaming session could not start")
		return
	}
	language := any(nil)
	switch strings.ToLower(start.Language) {
	case "en", "en-in":
		language = "en-IN"
	case "en-us":
		language = "en-US"
	case "hi", "hi-in":
		language = "hi-IN"
	case "", "auto":
	default:
		s.wsError(ctx, c, "unsupported streaming language")
		return
	}
	update := map[string]any{"type": "session.update", "session": map[string]any{"type": "transcription", "audio": map[string]any{"input": map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": 16000}, "transcription": map[string]any{"model": p.StreamDeployment, "language": language}, "turn_detection": nil, "noise_reduction": nil}}}}
	if err = writeWSJSON(sessionCtx, up, update); err != nil {
		s.wsError(ctx, c, "Microsoft streaming session could not be configured")
		return
	}
	if err = waitForProviderEvent(sessionCtx, up, "session.updated"); err != nil {
		s.wsError(ctx, c, "Microsoft streaming session could not be configured")
		return
	}
	if err = s.wsWrite(ctx, c, map[string]any{"type": "ready", "mode": "streaming"}); err != nil {
		return
	}
	type incoming struct {
		typ  websocket.MessageType
		data []byte
		err  error
	}
	clientCh := make(chan incoming, 1)
	providerCh := make(chan incoming, 1)
	go func() {
		for {
			readCtx, readCancel := context.WithTimeout(ctx, 60*time.Second)
			t, b, e := c.Read(readCtx)
			readCancel()
			select {
			case clientCh <- incoming{t, b, e}:
			case <-ctx.Done():
				return
			}
			if e != nil {
				return
			}
		}
	}()
	go func() {
		for {
			t, b, e := up.Read(ctx)
			select {
			case providerCh <- incoming{t, b, e}:
			case <-ctx.Done():
				return
			}
			if e != nil {
				return
			}
		}
	}()
	var pcm bytes.Buffer
	var finalized, provisional string
	committed := false
	started := time.Now()
	for {
		select {
		case msg := <-clientCh:
			if msg.err != nil {
				return
			}
			if msg.typ == websocket.MessageBinary {
				if committed {
					continue
				}
				if pcm.Len()+len(msg.data) > maxAudioBody {
					s.wsError(ctx, c, "audio stream is too large")
					return
				}
				pcm.Write(msg.data)
				if err := writeWSJSON(ctx, up, map[string]any{"type": "input_audio_buffer.append", "audio": base64.StdEncoding.EncodeToString(msg.data)}); err != nil {
					s.wsError(ctx, c, "Microsoft streaming provider disconnected")
					return
				}
				continue
			}
			var cmd streamStart
			if json.Unmarshal(msg.data, &cmd) == nil && (cmd.Action == "done" || cmd.Type == "commit") {
				if pcm.Len() == 0 {
					s.wsError(ctx, c, "audio stream is empty")
					return
				}
				if err := writeWSJSON(ctx, up, map[string]any{"type": "input_audio_buffer.commit"}); err != nil {
					s.wsError(ctx, c, "Microsoft streaming provider disconnected")
					return
				}
				committed = true
			}
		case msg := <-providerCh:
			if msg.err != nil {
				s.wsError(ctx, c, "Microsoft streaming provider disconnected")
				return
			}
			if msg.typ != websocket.MessageText {
				continue
			}
			var ev streamEvent
			if json.Unmarshal(msg.data, &ev) != nil {
				continue
			}
			switch ev.Type {
			case "conversation.item.input_audio_transcription.delta":
				finalized += ev.Delta
				provisional = ""
				_ = s.wsWrite(ctx, c, map[string]any{"type": "partial", "text": finalized})
			case "conversation.item.input_audio_transcription.intermediate":
				provisional = firstNonempty(ev.Transcript, ev.Text, ev.Delta)
				_ = s.wsWrite(ctx, c, map[string]any{"type": "partial", "text": finalized + provisional})
			case "conversation.item.input_audio_transcription.completed":
				rawText := firstNonempty(ev.Transcript, ev.Text, finalized+provisional)
				cleanup := s.cleanupTranscription(ctx, rawText, st)
				text := cleanup.Text
				id := newID()
				audio := pcm16WAV(pcm.Bytes(), 16000)
				if err := s.state.saveAudio(id, audio); err != nil {
					s.wsError(ctx, c, "audio history could not be saved")
					return
				}
				latency := time.Since(started).Milliseconds()
				duration := float64(pcm.Len()) / 32000
				h := History{ID: id, Kind: "stt", Model: model.ID, Provider: "microsoft", CreatedAt: started.UTC(), Status: "ok", LatencyMS: latency, DurationSeconds: duration, EstimatedCostUSD: sttCost(model.ID, duration), AudioAvailable: true, AudioContentType: "audio/wav"}
				applyCleanupHistory(&h, cleanup, st.Settings.SaveHistoryText)
				if err := s.appendHistory(h); err != nil {
					s.wsError(ctx, c, "history could not be saved")
					return
				}
				frame := map[string]any{"type": "final", "text": text, "model": model.ID, "provider": "microsoft", "id": id}
				for k, v := range cleanupResponse(cleanup) {
					frame[k] = v
				}
				_ = s.wsWrite(ctx, c, frame)
				_ = c.Close(websocket.StatusNormalClosure, "complete")
				return
			case "error":
				s.wsError(ctx, c, "Microsoft streaming provider reported an error")
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func microsoftStreamURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "", errors.New("invalid endpoint")
	}
	host := u.Hostname()
	if strings.HasSuffix(host, ".cognitiveservices.azure.com") {
		name := strings.TrimSuffix(host, ".cognitiveservices.azure.com")
		host = name + ".services.ai.azure.com"
	}
	u.Scheme = "wss"
	u.Host = host
	u.Path = "/mai/v1/realtime"
	u.RawQuery = "intent=transcription"
	return u.String(), nil
}
func waitForProviderEvent(ctx context.Context, c *websocket.Conn, want string) error {
	for {
		typ, b, err := c.Read(ctx)
		if err != nil {
			return err
		}
		if typ != websocket.MessageText {
			continue
		}
		var ev struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(b, &ev) != nil {
			continue
		}
		if ev.Type == "error" {
			return errors.New("provider error")
		}
		if ev.Type == want {
			return nil
		}
	}
}
func writeWSJSON(ctx context.Context, c *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, b)
}
func firstNonempty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func pcm16WAV(pcm []byte, sampleRate int) []byte {
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(36+len(pcm)))
	out.WriteString("WAVEfmt ")
	_ = binary.Write(&out, binary.LittleEndian, uint32(16))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&out, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(&out, binary.LittleEndian, uint16(2))
	_ = binary.Write(&out, binary.LittleEndian, uint16(16))
	out.WriteString("data")
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(pcm)))
	out.Write(pcm)
	return out.Bytes()
}

var _ = fmt.Sprint
