package gateway

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"hypnos.local/speech/internal/auth"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(Config{StateDir: t.TempDir(), MasterKey: bytes.Repeat([]byte{7}, 32), PublicURL: "https://speech.test", GroqAPIKey: "test-key", VerifyOwner: func(_ context.Context, token string) (auth.Owner, error) {
		if token != "owner" {
			return auth.Owner{}, auth.ErrInvalidToken
		}
		return auth.Owner{ID: "u1", Email: "owner@example.test", Name: "Owner"}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStateEncryptedPersistsAndRejectsWrongKey(t *testing.T) {
	dir := t.TempDir()
	key := bytes.Repeat([]byte{1}, 32)
	s, err := newStateStore(dir, key, "secret-provider-key", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.update(func(st *persistedState) error {
		st.Snippets = append(st.Snippets, Snippet{ID: "one", Trigger: "secret trigger", Text: "private transcript"})
		st.History = append(st.History, History{ID: "audio-one", Kind: "stt", AudioAvailable: true})
		st.AudioTypes["audio-one"] = "audio/webm"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.saveAudio("audio-one", []byte("webm-private-audio")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.statePath())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("private transcript")) || bytes.Contains(raw, []byte("secret-provider-key")) {
		t.Fatal("encrypted state leaked plaintext")
	}
	if _, err := newStateStore(dir, key, "", ""); err == nil {
		t.Fatal("concurrent state open unexpectedly acquired lock")
	}
	s.close()
	reopened, err := newStateStore(dir, key, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.snapshot().Snippets; len(got) != 1 || got[0].Text != "private transcript" {
		t.Fatalf("state did not round trip: %#v", got)
	}
	if got := reopened.snapshot().AudioTypes["audio-one"]; got != "audio/webm" {
		t.Fatalf("audio MIME did not persist: %q", got)
	}
	if got, err := reopened.loadAudio("audio-one"); err != nil || string(got) != "webm-private-audio" {
		t.Fatalf("audio did not round trip: %q %v", got, err)
	}
	if err := reopened.update(func(st *persistedState) error {
		p := st.Providers["groq"]
		p.APIKey = ""
		p.APIKeyInitialized = true
		st.Providers["groq"] = p
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reopened.close()
	cleared, err := newStateStore(dir, key, "secret-provider-key", "")
	if err != nil {
		t.Fatal(err)
	}
	if cleared.snapshot().Providers["groq"].APIKey != "" {
		t.Fatal("cleared bootstrap credential was restored on restart")
	}
	cleared.close()
	if _, err := newStateStore(dir, bytes.Repeat([]byte{2}, 32), "", ""); err == nil {
		t.Fatal("wrong master key unexpectedly decrypted state")
	}
}

func TestAdminOriginAndDeviceKeyBoundary(t *testing.T) {
	s := testServer(t)
	token, err := s.MintDeviceKey("MacBook")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	defer ts.Close()
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer owner")
	req.Header.Set("Origin", "https://evil.test")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin write status=%d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("device key reached admin API: status=%d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("device key could not reach speech API: status=%d", resp.StatusCode)
	}
}

func TestGroqBatchWebSocketHasNoFakePartials(t *testing.T) {
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing provider authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"text":"hello Hypnos","duration":0.1}`)
	}))
	defer provider.Close()
	s := testServer(t)
	s.client = provider.Client()
	if err := s.state.update(func(st *persistedState) error {
		p := st.Providers["groq"]
		p.Endpoint = provider.URL
		st.Providers["groq"] = p
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	u.Path = "/v1/audio/stream"
	c, _, err := websocket.Dial(context.Background(), u.String(), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer owner"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	writeTestJSON(t, c, map[string]any{"type": "start", "model": "whisper-large-v3-turbo", "language": "en"})
	var ready map[string]any
	readTestJSON(t, c, &ready)
	if ready["type"] != "ready" || ready["mode"] != "batch" {
		t.Fatalf("unexpected ready: %#v", ready)
	}
	if err := c.Write(context.Background(), websocket.MessageBinary, make([]byte, 3200)); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, c, map[string]any{"action": "done"})
	var final map[string]any
	readTestJSON(t, c, &final)
	if final["type"] != "final" || final["text"] != "hello Hypnos" {
		t.Fatalf("unexpected final frame: %#v", final)
	}
}

func TestMicrosoftBatchRequestContract(t *testing.T) {
	var gotDefinition map[string]any
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/speechtotext/transcriptions:transcribe" || r.URL.Query().Get("api-version") != "2025-10-15" {
			t.Errorf("wrong Microsoft URL: %s", r.URL.String())
		}
		if r.Header.Get("Ocp-Apim-Subscription-Key") != "azure-key" {
			t.Error("missing Azure key header")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		f, _, err := r.FormFile("audio")
		if err != nil {
			t.Errorf("audio field missing: %v", err)
		} else {
			f.Close()
		}
		if err := json.Unmarshal([]byte(r.FormValue("definition")), &gotDefinition); err != nil {
			t.Errorf("bad definition: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"combinedPhrases":[{"text":"namaste world"}],"durationMilliseconds":1250}`)
	}))
	defer provider.Close()
	s := testServer(t)
	s.client = provider.Client()
	res, err := s.microsoftTranscribe(context.Background(), providerState{APIKey: "azure-key", Endpoint: provider.URL, BatchDeployment: "MAI-Transcribe-2"}, "hi-IN", "", "audio/wav", pcm16WAV(make([]byte, 320), 16000), []DictionaryEntry{{Word: "Hypnos"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "namaste world" || res.Duration != 1.25 {
		t.Fatalf("unexpected result: %#v", res)
	}
	enhanced := gotDefinition["enhancedMode"].(map[string]any)
	if enhanced["model"] != "MAI-Transcribe-2" {
		t.Fatalf("wrong definition: %#v", gotDefinition)
	}
}

func TestGroqSpeechChunksAndRepairsStreamingWAV(t *testing.T) {
	requests := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var in struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Error(err)
		}
		if len([]rune(in.Input)) > 200 {
			t.Errorf("chunk has %d runes", len([]rune(in.Input)))
		}
		wav := pcm16WAV([]byte{1, 2, 3, 4}, 24000)
		binary.LittleEndian.PutUint32(wav[4:8], ^uint32(0))
		binary.LittleEndian.PutUint32(wav[40:44], ^uint32(0))
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(wav)
	}))
	defer provider.Close()
	s := testServer(t)
	s.client = provider.Client()
	input := strings.Repeat("नमस्ते ", 80) + "Done."
	wav, err := s.groqSpeech(context.Background(), providerState{APIKey: "test-key", Endpoint: provider.URL}, "canopylabs/orpheus-v1-english", input, "troy")
	if err != nil {
		t.Fatal(err)
	}
	p, err := parseWAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	if requests < 2 || len(p.data) != requests*4 {
		t.Fatalf("requests=%d data=%d", requests, len(p.data))
	}
	if binary.LittleEndian.Uint32(wav[4:8]) == ^uint32(0) || binary.LittleEndian.Uint32(wav[40:44]) == ^uint32(0) {
		t.Fatal("combined WAV retained placeholder lengths")
	}
}

func writeTestJSON(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}
func readTestJSON(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	typ, b, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("unexpected frame type %v", typ)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

func TestOfflineRevocationPersists(t *testing.T) {
	s := testServer(t)
	token, err := s.MintDeviceKey("Recovery check")
	if err != nil {
		t.Fatal(err)
	}
	if !s.useDeviceKey(token) {
		t.Fatal("new key rejected")
	}
	if err := s.RevokeDeviceKey(token); err != nil {
		t.Fatal(err)
	}
	if s.useDeviceKey(token) {
		t.Fatal("revoked key accepted")
	}
	if err := s.RevokeDeviceKey("spk_unknown"); err == nil {
		t.Fatal("unknown key accepted for revocation")
	}
	cfg := s.cfg
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.useDeviceKey(token) {
		t.Fatal("revoked key returned after restart")
	}
}

func TestDeletingDefaultProfileRestoresFallback(t *testing.T) {
	s := testServer(t)
	if err := s.state.update(func(st *persistedState) error {
		st.Profiles = append(st.Profiles, Profile{ID: "default", Name: "Dictation", STTModel: "whisper-large-v3-turbo"})
		st.Settings.DefaultProfileID = "default"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/profiles/default", nil)
	req.Header.Set("Authorization", "Bearer owner")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status %d", w.Code)
	}
	profile, err := resolveProfile(s.state.snapshot(), "")
	if err != nil || profile != nil {
		t.Fatal("deleted profile still controls fallback")
	}
}
