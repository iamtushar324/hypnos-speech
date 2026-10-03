package gateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *Server) models() []Model {
	st := s.state.snapshot()
	groq := st.Providers["groq"]
	ms := st.Providers["microsoft"]
	g := groq.APIKey != ""
	msBatch := ms.APIKey != "" && ms.Endpoint != ""
	msStream := ms.APIKey != "" && ms.Endpoint != "" && ms.StreamDeployment != ""
	msTTS := ms.APIKey != "" && ms.Region != ""
	return []Model{
		{ID: "whisper-large-v3-turbo", Provider: "groq", Kind: "stt", Name: "Whisper Large V3 Turbo", Description: "Fast multilingual batch transcription on Groq", Configured: g, Languages: []string{"multilingual"}},
		{ID: "whisper-large-v3", Provider: "groq", Kind: "stt", Name: "Whisper Large V3", Description: "High-accuracy multilingual batch transcription on Groq", Configured: g, Languages: []string{"multilingual"}},
		{ID: "canopylabs/orpheus-v1-english", Provider: "groq", Kind: "tts", Name: "Orpheus English", Description: "Expressive English speech on Groq", Configured: g, Languages: []string{"en"}},
		{ID: "mai-transcribe-2", Provider: "microsoft", Kind: "stt", Name: "MAI-Transcribe-2", Description: "Latest Microsoft multilingual batch transcription", Configured: msBatch, Languages: []string{"en", "hi"}},
		{ID: "mai-transcribe-2-streaming", Provider: "microsoft", Kind: "stt", Name: "MAI-Transcribe-2 Streaming", Description: "Live Microsoft transcription with incremental results", Configured: msStream, Streaming: true, Languages: []string{"en", "hi"}},
		{ID: "mai-voice-2.1-flash", Provider: "microsoft", Kind: "tts", Name: "MAI-Voice-2.1 Flash", Description: "Low-latency Microsoft speech synthesis", Configured: msTTS, Languages: []string{"en", "hi"}},
		{ID: "mai-voice-2.1", Provider: "microsoft", Kind: "tts", Name: "MAI-Voice-2.1", Description: "High-quality Microsoft speech synthesis", Configured: msTTS, Languages: []string{"en", "hi"}},
	}
}

func (s *Server) model(id, kind string) (Model, bool) {
	for _, m := range s.models() {
		if m.ID == id && m.Kind == kind {
			return m, true
		}
	}
	return Model{}, false
}
func (s *Server) handleModels(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{"data": s.models()})
}

func providerView(id string, p providerState) Provider {
	name := map[string]string{"groq": "Groq", "microsoft": "Microsoft Azure Speech", "openrouter": "OpenRouter"}[id]
	configured := p.APIKey != ""
	if id == "microsoft" {
		configured = configured && (p.Endpoint != "" || p.Region != "")
	}
	return Provider{ID: id, Name: name, Configured: configured, HasAPIKey: p.APIKey != "", Endpoint: p.Endpoint, Region: p.Region, StreamDeployment: p.StreamDeployment, BatchDeployment: p.BatchDeployment, VerifiedAt: p.VerifiedAt, LastError: p.LastError}
}
func (s *Server) providers() []Provider {
	st := s.state.snapshot()
	return []Provider{providerView("groq", st.Providers["groq"]), providerView("microsoft", st.Providers["microsoft"]), providerView("openrouter", st.Providers["openrouter"])}
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": s.providers()})
		return
	}
	if len(parts) < 2 || len(parts) > 3 || (parts[1] != "groq" && parts[1] != "microsoft" && parts[1] != "openrouter") {
		http.NotFound(w, r)
		return
	}
	id := parts[1]
	if len(parts) == 3 {
		if parts[2] != "test" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		s.testProvider(w, r, id)
		return
	}
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	var in struct {
		APIKey           *string `json:"api_key"`
		ClearAPIKey      bool    `json:"clear_api_key"`
		Endpoint         string  `json:"endpoint"`
		Region           string  `json:"region"`
		StreamDeployment string  `json:"stream_deployment"`
		BatchDeployment  string  `json:"batch_deployment"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Endpoint = strings.TrimSpace(in.Endpoint)
	in.Region = strings.TrimSpace(in.Region)
	in.StreamDeployment = strings.TrimSpace(in.StreamDeployment)
	in.BatchDeployment = strings.TrimSpace(in.BatchDeployment)
	if id == "openrouter" {
		if in.Endpoint != "" && in.Endpoint != openRouterEndpoint {
			writeError(w, http.StatusBadRequest, "OpenRouter endpoint is fixed")
			return
		}
		in.Endpoint = openRouterEndpoint
	}
	if in.Endpoint != "" {
		if err := validateProviderEndpoint(id, in.Endpoint); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if in.Region != "" && !azureRegionRE.MatchString(in.Region) {
		writeError(w, http.StatusBadRequest, "Microsoft region is invalid")
		return
	}
	if len(in.Region) > 80 || len(in.StreamDeployment) > 120 || len(in.BatchDeployment) > 120 {
		writeError(w, http.StatusBadRequest, "provider setting is too long")
		return
	}
	err := s.state.update(func(st *persistedState) error {
		p := st.Providers[id]
		if in.ClearAPIKey {
			p.APIKey = ""
			p.APIKeyInitialized = true
		} else if in.APIKey != nil && strings.TrimSpace(*in.APIKey) != "" {
			p.APIKey = strings.TrimSpace(*in.APIKey)
			p.APIKeyInitialized = true
		}
		p.Endpoint = in.Endpoint
		p.Region = in.Region
		p.StreamDeployment = in.StreamDeployment
		p.BatchDeployment = in.BatchDeployment
		p.VerifiedAt = nil
		p.LastError = ""
		st.Providers[id] = p
		return nil
	})
	if err != nil {
		internalError(w, err)
		return
	}
	st := s.state.snapshot()
	writeJSON(w, http.StatusOK, providerView(id, st.Providers[id]))
}

func validateProviderEndpoint(id, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("endpoint must be a plain HTTPS URL")
	}
	host := strings.ToLower(u.Hostname())
	if id == "groq" && host != "api.groq.com" {
		return errors.New("Groq endpoint must use api.groq.com")
	}
	if id == "openrouter" && strings.TrimRight(raw, "/") != openRouterEndpoint {
		return errors.New("OpenRouter endpoint is fixed")
	}
	if id == "microsoft" && !(strings.HasSuffix(host, ".services.ai.azure.com") || strings.HasSuffix(host, ".cognitiveservices.azure.com")) {
		return errors.New("Microsoft endpoint must use an Azure AI resource hostname")
	}
	return nil
}

func (s *Server) testProvider(w http.ResponseWriter, r *http.Request, id string) {
	st := s.state.snapshot()
	p := st.Providers[id]
	now := time.Now().UTC()
	ok := false
	message := ""
	if p.APIKey == "" {
		writeError(w, http.StatusBadRequest, "provider API key is not configured")
		return
	}
	if id == "groq" || id == "openrouter" {
		path := "/models"
		label := "Groq"
		if id == "openrouter" {
			path = "/key"
			label = "OpenRouter"
		}
		req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(p.Endpoint, "/")+path, nil)
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
		resp, err := s.client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			ok = resp.StatusCode >= 200 && resp.StatusCode < 300
		}
		if ok {
			message = label + " credentials verified"
		} else {
			message = label + " did not accept the configured credentials"
		}
	} else {
		message = "Microsoft configuration saved; verification requires an inference request"
	}
	err := s.state.update(func(st *persistedState) error {
		p := st.Providers[id]
		if ok {
			p.VerifiedAt = &now
			p.LastError = ""
		} else {
			p.VerifiedAt = nil
			p.LastError = message
		}
		st.Providers[id] = p
		return nil
	})
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": ok, "message": message, "verified_at": func() any {
		if ok {
			return now
		}
		return nil
	}()})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, s.state.snapshot().Settings)
		return
	}
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	var in Settings
	if !decodeJSON(w, r, &in) {
		return
	}
	if _, ok := s.model(in.DefaultSTTModel, "stt"); !ok {
		writeError(w, http.StatusBadRequest, "unknown STT model")
		return
	}
	if _, ok := s.model(in.DefaultTTSModel, "tts"); !ok {
		writeError(w, http.StatusBadRequest, "unknown TTS model")
		return
	}
	if in.DefaultVoice == "" || len(in.DefaultVoice) > 100 || len(in.Language) > 40 {
		writeError(w, http.StatusBadRequest, "invalid voice or language")
		return
	}
	in.CleanupModel = strings.TrimSpace(in.CleanupModel)
	if in.CleanupTimeoutSeconds < 2 || in.CleanupTimeoutSeconds > 30 {
		writeError(w, http.StatusBadRequest, "cleanup timeout must be between 2 and 30 seconds")
		return
	}
	if (in.CleanupModel != "" && !validCleanupModel(in.CleanupModel)) || len(in.CleanupPrompt) > maxCleanupPromptBytes {
		writeError(w, http.StatusBadRequest, "cleanup model or prompt is too long")
		return
	}
	if in.CleanupEnabled && (in.CleanupModel == "" || strings.TrimSpace(in.CleanupPrompt) == "") {
		writeError(w, http.StatusBadRequest, "cleanup model and prompt are required when cleanup is enabled")
		return
	}
	if in.DefaultProfileID != "" && !profileExists(s.state.snapshot(), in.DefaultProfileID) {
		writeError(w, http.StatusBadRequest, "default profile does not exist")
		return
	}
	if err := s.state.update(func(st *persistedState) error { st.Settings = in; return nil }); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func profileExists(st persistedState, id string) bool {
	for _, v := range st.Profiles {
		if v.ID == id {
			return true
		}
	}
	return false
}

func (s *Server) handleCollection(w http.ResponseWriter, r *http.Request, parts []string) {
	name := parts[0]
	if len(parts) > 2 {
		http.NotFound(w, r)
		return
	}
	st := s.state.snapshot()
	if len(parts) == 1 && r.Method == http.MethodGet {
		switch name {
		case "dictionary":
			writeJSON(w, 200, map[string]any{"data": st.Dictionary})
		case "snippets":
			writeJSON(w, 200, map[string]any{"data": st.Snippets})
		case "profiles":
			writeJSON(w, 200, map[string]any{"data": st.Profiles})
		}
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPost {
		s.createCollection(w, r, name)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPut {
		s.updateCollection(w, r, name, parts[1])
		return
	}
	if len(parts) == 2 && r.Method == http.MethodDelete {
		s.deleteCollection(w, name, parts[1])
		return
	}
	methodNotAllowed(w)
}

func validateProfile(s *Server, p Profile) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 100 {
		return errors.New("profile name is required")
	}
	if p.STTModel != "" {
		if _, ok := s.model(p.STTModel, "stt"); !ok {
			return errors.New("unknown STT model")
		}
	}
	if p.TTSModel != "" {
		if _, ok := s.model(p.TTSModel, "tts"); !ok {
			return errors.New("unknown TTS model")
		}
	}
	if len(p.Description) > 500 || len(p.Language) > 40 || len(p.Voice) > 100 {
		return errors.New("profile value is too long")
	}
	return nil
}

func (s *Server) createCollection(w http.ResponseWriter, r *http.Request, name string) {
	id := newID()
	switch name {
	case "dictionary":
		var v DictionaryEntry
		if !decodeJSON(w, r, &v) {
			return
		}
		v.ID = id
		v.Word = strings.TrimSpace(v.Word)
		if v.Word == "" || v.Replacement == "" || len(v.Word) > 100 || len(v.Replacement) > 200 || len(v.Notes) > 500 {
			writeError(w, 400, "invalid dictionary entry")
			return
		}
		err := s.state.update(func(st *persistedState) error { st.Dictionary = append(st.Dictionary, v); return nil })
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, 201, v)
	case "snippets":
		var v Snippet
		if !decodeJSON(w, r, &v) {
			return
		}
		v.ID = id
		v.Trigger = strings.TrimSpace(v.Trigger)
		if v.Trigger == "" || v.Text == "" || len(v.Trigger) > 200 || len(v.Text) > 10000 {
			writeError(w, 400, "invalid snippet")
			return
		}
		err := s.state.update(func(st *persistedState) error { st.Snippets = append(st.Snippets, v); return nil })
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, 201, v)
	case "profiles":
		var v Profile
		if !decodeJSON(w, r, &v) {
			return
		}
		v.ID = id
		if err := validateProfile(s, v); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		err := s.state.update(func(st *persistedState) error { st.Profiles = append(st.Profiles, v); return nil })
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, 201, v)
	}
}

func (s *Server) updateCollection(w http.ResponseWriter, r *http.Request, name, id string) {
	found := false
	switch name {
	case "dictionary":
		var v DictionaryEntry
		if !decodeJSON(w, r, &v) {
			return
		}
		v.ID = id
		v.Word = strings.TrimSpace(v.Word)
		if v.Word == "" || v.Replacement == "" || len(v.Word) > 100 || len(v.Replacement) > 200 || len(v.Notes) > 500 {
			writeError(w, 400, "invalid dictionary entry")
			return
		}
		err := s.state.update(func(st *persistedState) error {
			for i := range st.Dictionary {
				if st.Dictionary[i].ID == id {
					st.Dictionary[i] = v
					found = true
				}
			}
			return nil
		})
		if err != nil {
			internalError(w, err)
			return
		}
		if !found {
			writeError(w, 404, "entry not found")
			return
		}
		writeJSON(w, 200, v)
	case "snippets":
		var v Snippet
		if !decodeJSON(w, r, &v) {
			return
		}
		v.ID = id
		v.Trigger = strings.TrimSpace(v.Trigger)
		if v.Trigger == "" || v.Text == "" || len(v.Trigger) > 200 || len(v.Text) > 10000 {
			writeError(w, 400, "invalid snippet")
			return
		}
		err := s.state.update(func(st *persistedState) error {
			for i := range st.Snippets {
				if st.Snippets[i].ID == id {
					st.Snippets[i] = v
					found = true
				}
			}
			return nil
		})
		if err != nil {
			internalError(w, err)
			return
		}
		if !found {
			writeError(w, 404, "snippet not found")
			return
		}
		writeJSON(w, 200, v)
	case "profiles":
		var v Profile
		if !decodeJSON(w, r, &v) {
			return
		}
		v.ID = id
		if err := validateProfile(s, v); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		err := s.state.update(func(st *persistedState) error {
			for i := range st.Profiles {
				if st.Profiles[i].ID == id {
					st.Profiles[i] = v
					found = true
				}
			}
			return nil
		})
		if err != nil {
			internalError(w, err)
			return
		}
		if !found {
			writeError(w, 404, "profile not found")
			return
		}
		writeJSON(w, 200, v)
	}
}

func (s *Server) deleteCollection(w http.ResponseWriter, name, id string) {
	found := false
	err := s.state.update(func(st *persistedState) error {
		switch name {
		case "dictionary":
			for i, v := range st.Dictionary {
				if v.ID == id {
					st.Dictionary = append(st.Dictionary[:i], st.Dictionary[i+1:]...)
					found = true
					break
				}
			}
		case "snippets":
			for i, v := range st.Snippets {
				if v.ID == id {
					st.Snippets = append(st.Snippets[:i], st.Snippets[i+1:]...)
					found = true
					break
				}
			}
		case "profiles":
			for i, v := range st.Profiles {
				if v.ID == id {
					st.Profiles = append(st.Profiles[:i], st.Profiles[i+1:]...)
					if st.Settings.DefaultProfileID == id {
						st.Settings.DefaultProfileID = ""
					}
					found = true
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		internalError(w, err)
		return
	}
	if !found {
		writeError(w, 404, "item not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if len(parts) == 3 && parts[2] == "audio" {
		s.historyAudio(w, parts[1])
		return
	}
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 {
			writeError(w, 400, "invalid limit")
			return
		}
		if n > 200 {
			n = 200
		}
		limit = n
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	st := s.state.snapshot()
	out := make([]History, 0, limit)
	for i := len(st.History) - 1; i >= 0 && len(out) < limit; i-- {
		h := st.History[i]
		if q == "" || strings.Contains(strings.ToLower(h.Text), q) || strings.Contains(strings.ToLower(h.Model), q) {
			out = append(out, h)
		}
	}
	writeJSON(w, 200, map[string]any{"data": out})
}
func (s *Server) historyAudio(w http.ResponseWriter, id string) {
	st := s.state.snapshot()
	var h *History
	for i := range st.History {
		if st.History[i].ID == id {
			h = &st.History[i]
			break
		}
	}
	if h == nil || !h.AudioAvailable {
		writeError(w, 404, "audio not found")
		return
	}
	b, err := s.state.loadAudio(id)
	if err != nil {
		internalError(w, err)
		return
	}
	ct := st.AudioTypes[id]
	if ct == "" && h.Kind == "tts" {
		ct = "audio/wav"
	}
	if ct == "" {
		ct = http.DetectContentType(b)
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", id+audioExt(ct)))
	w.WriteHeader(200)
	_, _ = w.Write(b)
}
func audioExt(ct string) string {
	switch ct {
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	default:
		return ".bin"
	}
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request, parts []string) {
	st := s.state.snapshot()
	if len(parts) == 1 && r.Method == http.MethodGet {
		data := make([]map[string]any, 0, len(st.Keys))
		for _, k := range st.Keys {
			data = append(data, map[string]any{"id": k.ID, "name": k.Name, "prefix": k.Prefix, "created_at": k.CreatedAt, "last_used_at": k.LastUsedAt, "revoked": k.Revoked})
		}
		writeJSON(w, 200, map[string]any{"data": data})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPost {
		var in struct {
			Name string `json:"name"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		token, err := s.MintDeviceKey(in.Name)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		now := s.state.snapshot()
		var k DeviceKey
		for _, candidate := range now.Keys {
			if candidate.Hash == hashSecret(token) {
				k = candidate
				break
			}
		}
		response := map[string]any{"id": k.ID, "name": k.Name, "prefix": k.Prefix, "token": token}
		pairing, err := s.dictionPairing(token)
		if err != nil {
			_ = s.RevokeDeviceKey(token)
			internalError(w, err)
			return
		}
		if pairing != nil {
			response["diction"] = pairing
		}
		writeJSON(w, 201, response)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodDelete {
		id := parts[1]
		found := false
		err := s.state.update(func(st *persistedState) error {
			for i := range st.Keys {
				if st.Keys[i].ID == id {
					st.Keys[i].Revoked = true
					found = true
				}
			}
			return nil
		})
		if err != nil {
			internalError(w, err)
			return
		}
		if !found {
			writeError(w, 404, "key not found")
			return
		}
		w.WriteHeader(204)
		return
	}
	methodNotAllowed(w)
}

func (s *Server) handleDashboard(w http.ResponseWriter) {
	st := s.state.snapshot()
	var stats struct {
		Requests         int     `json:"requests"`
		STTRequests      int     `json:"stt_requests"`
		TTSRequests      int     `json:"tts_requests"`
		Errors           int     `json:"errors"`
		AvgLatencyMS     float64 `json:"avg_latency_ms"`
		EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	}
	var total int64
	for _, h := range st.History {
		stats.Requests++
		if h.Kind == "stt" {
			stats.STTRequests++
		} else if h.Kind == "tts" {
			stats.TTSRequests++
		}
		if h.Status != "ok" {
			stats.Errors++
		}
		total += h.LatencyMS
		stats.EstimatedCostUSD += h.EstimatedCostUSD
	}
	if stats.Requests > 0 {
		stats.AvgLatencyMS = float64(total) / float64(stats.Requests)
	}
	recent := append([]History(nil), st.History...)
	sort.Slice(recent, func(i, j int) bool { return recent[i].CreatedAt.After(recent[j].CreatedAt) })
	if len(recent) > 10 {
		recent = recent[:10]
	}
	writeJSON(w, 200, map[string]any{"stats": stats, "providers": s.providers(), "recent": recent, "system": map[string]any{"version": version, "uptime_seconds": int64(time.Since(s.started).Seconds())}})
}

func generateSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func hashSecret(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
