package gateway

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/iamtushar324/vokiri/server/internal/auth"
)

const (
	maxJSONBody  = 128 << 10
	maxAudioBody = 25 << 20
)

type Server struct {
	cfg          Config
	state        *stateStore
	client       *http.Client
	started      time.Time
	sttSlots     chan struct{}
	ttsSlots     chan struct{}
	cleanupSlots chan struct{}
	closeOnce    sync.Once
}

func New(cfg Config) (*Server, error) {
	if cfg.VerifyOwner == nil {
		return nil, errors.New("owner verifier is required")
	}
	if cfg.PublicURL == "" {
		return nil, errors.New("public URL is required")
	}
	u, err := url.Parse(cfg.PublicURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("public URL must be an absolute https URL")
	}
	if cfg.DictionPublicURL != "" {
		d, err := url.Parse(cfg.DictionPublicURL)
		if err != nil || d.Scheme != "https" || d.Host == "" || d.User != nil || d.RawQuery != "" || d.Fragment != "" || (d.Path != "" && d.Path != "/") {
			return nil, errors.New("Diction URL must be a plain HTTPS origin")
		}
		cfg.DictionPublicURL = strings.TrimRight(cfg.DictionPublicURL, "/")
	}
	store, err := newStateStore(cfg.StateDir, cfg.MasterKey, cfg.GroqAPIKey, cfg.OpenRouterAPIKey)
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg: cfg, state: store, started: time.Now(),
		client:   &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		sttSlots: make(chan struct{}, 3), ttsSlots: make(chan struct{}, 2), cleanupSlots: make(chan struct{}, 2),
	}, nil
}

func (s *Server) Close() error {
	s.closeOnce.Do(func() { s.state.close() })
	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/healthz" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if r.URL.Path == "/api/config" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"service_name": "Vokiri", "public_url": s.cfg.PublicURL, "clerk": map[string]string{"publishable_key": s.cfg.ClerkPublishableKey, "frontend_api": s.cfg.ClerkFrontendAPI}, "diction": map[string]string{"endpoint": s.cfg.DictionPublicURL}})
		return
	}
	if r.URL.Path == "/v1/audio/stream" {
		s.handleStream(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v1/") {
		if _, ok := s.authorizeSpeech(r); !ok {
			unauthorized(w)
			return
		}
		s.routeSpeech(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		owner, err := s.verifyOwner(r)
		if err != nil {
			unauthorized(w)
			return
		}
		if isWrite(r.Method) && !s.validOrigin(r) {
			writeError(w, http.StatusForbidden, "request origin is not allowed")
			return
		}
		s.routeAdmin(w, r, owner)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) routeSpeech(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/models":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleModels(w)
	case "/v1/audio/transcriptions":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		s.handleTranscription(w, r)
	case "/v1/audio/speech":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		s.handleSpeech(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) routeAdmin(w http.ResponseWriter, r *http.Request, owner auth.Owner) {
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	switch parts[0] {
	case "me":
		if len(parts) != 1 || r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, owner)
	case "dashboard":
		if len(parts) != 1 || r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleDashboard(w)
	case "models":
		if len(parts) != 1 || r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleModels(w)
	case "settings":
		if len(parts) != 1 {
			http.NotFound(w, r)
			return
		}
		s.handleSettings(w, r)
	case "providers":
		s.handleProviders(w, r, parts)
	case "dictionary", "snippets", "profiles":
		s.handleCollection(w, r, parts)
	case "history":
		s.handleHistory(w, r, parts)
	case "keys":
		s.handleKeys(w, r, parts)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) verifyOwner(r *http.Request) (auth.Owner, error) {
	t := bearerToken(r.Header.Get("Authorization"))
	if t == "" {
		return auth.Owner{}, errors.New("missing bearer token")
	}
	return s.cfg.VerifyOwner(r.Context(), t)
}

func (s *Server) authorizeSpeech(r *http.Request) (auth.Owner, bool) {
	t := bearerToken(r.Header.Get("Authorization"))
	if t == "" {
		return auth.Owner{}, false
	}
	if owner, err := s.cfg.VerifyOwner(r.Context(), t); err == nil {
		return owner, true
	}
	if strings.HasPrefix(t, "spk_") && s.useDeviceKey(t) {
		return auth.Owner{ID: "device", Name: "Speech device"}, true
	}
	return auth.Owner{}, false
}

func bearerToken(v string) string {
	p := strings.SplitN(v, " ", 2)
	if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(p[1])
}

func (s *Server) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	want, err1 := url.Parse(s.cfg.PublicURL)
	got, err2 := url.Parse(origin)
	return err1 == nil && err2 == nil && strings.EqualFold(want.Scheme, got.Scheme) && strings.EqualFold(want.Host, got.Host)
}

func isWrite(m string) bool {
	return m == http.MethodPost || m == http.MethodPut || m == http.MethodPatch || m == http.MethodDelete
}

func (s *Server) MintDeviceKey(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 {
		return "", errors.New("device name must be between 1 and 80 characters")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := "spk_" + base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(token))
	id := newID()
	now := time.Now().UTC()
	err := s.state.update(func(st *persistedState) error {
		st.Keys = append(st.Keys, DeviceKey{ID: id, Name: name, Prefix: token[:12], Hash: hex.EncodeToString(hash[:]), CreatedAt: now})
		return nil
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// RevokeDeviceKey invalidates a token through the local, exclusively locked state.
// It is intended for offline recovery; normal administration uses /api/keys.
func (s *Server) RevokeDeviceKey(token string) error {
	if !strings.HasPrefix(token, "spk_") {
		return errors.New("invalid device key")
	}
	sum := sha256.Sum256([]byte(token))
	encoded := hex.EncodeToString(sum[:])
	return s.state.update(func(st *persistedState) error {
		for i := range st.Keys {
			if subtle.ConstantTimeCompare([]byte(st.Keys[i].Hash), []byte(encoded)) == 1 {
				st.Keys[i].Revoked = true
				return nil
			}
		}
		return errors.New("device key was not found")
	})
}

func (s *Server) useDeviceKey(token string) bool {
	h := sha256.Sum256([]byte(token))
	encoded := hex.EncodeToString(h[:])
	found := ""
	now := time.Now().UTC()
	st := s.state.snapshot()
	for _, k := range st.Keys {
		if !k.Revoked && len(k.Hash) == len(encoded) && subtle.ConstantTimeCompare([]byte(k.Hash), []byte(encoded)) == 1 {
			found = k.ID
			break
		}
	}
	if found == "" {
		return false
	}
	_ = s.state.update(func(st *persistedState) error {
		for i := range st.Keys {
			if st.Keys[i].ID == found {
				st.Keys[i].LastUsedAt = &now
			}
		}
		return nil
	})
	return true
}

func newID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request must contain one JSON value")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}
func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, "authentication required")
}
func internalError(w http.ResponseWriter, err error) {
	slog.Error("speech gateway request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "the request could not be completed")
}

func acquire(ctx context.Context, sem chan struct{}) error {
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("service is busy")
	}
}
func release(sem chan struct{}) { <-sem }
