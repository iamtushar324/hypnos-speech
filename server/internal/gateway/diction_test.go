package gateway

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func dictionTestHTTP(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.StripPrefix("/diction", s.DictionHandler()))
	t.Cleanup(ts.Close)
	return ts
}

func TestDictionPublicDiscoveryAndPairingKey(t *testing.T) {
	s := testServer(t)
	ts := dictionTestHTTP(t, s)
	for _, path := range []string{"/", "/health", "/v1/models"} {
		resp, err := http.Get(ts.URL + "/diction" + path)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d", path, resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if path == "/health" && body["version"] != "vokiri/"+version {
			t.Fatalf("unexpected health: %#v", body)
		}
		if path == "/v1/models" {
			providers := body["providers"].([]any)
			if len(providers) != 1 || providers[0].(map[string]any)["id"] != "groq" {
				t.Fatalf("unexpected providers: %#v", providers)
			}
			models := providers[0].(map[string]any)["models"].([]any)
			if len(models) != 1 || models[0].(map[string]any)["id"] != "whisper-large-v3-turbo" || models[0].(map[string]any)["available"] != true {
				t.Fatalf("unexpected models: %#v", models)
			}
			caps := body["capabilities"].(map[string]any)
			if caps["pairing"] != true || caps["key_rotation"] != false || caps["llm"] != false || caps["text_process"] != false {
				t.Fatalf("unsafe capabilities: %#v", caps)
			}
		}
	}
	token, err := s.MintDeviceKey("Diction")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/diction/v1/auth/key", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var paired map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&paired); err != nil {
		t.Fatal(err)
	}
	if paired["key"] != token || paired["rotation"] != false || len(paired) != 2 {
		t.Fatalf("unexpected pairing response: %#v", paired)
	}
}

func TestDictionStreamPCMContextDoneAndHistory(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		if r.FormValue("model") != "whisper-large-v3-turbo" {
			t.Errorf("model=%q", r.FormValue("model"))
		}
		if r.FormValue("language") != "en" {
			t.Errorf("language=%q", r.FormValue("language"))
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		audio, _ := io.ReadAll(f)
		f.Close()
		if len(audio) < 44 || string(audio[:4]) != "RIFF" || binary.LittleEndian.Uint32(audio[24:28]) != 16000 {
			t.Errorf("bad WAV header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"text":"hello diction","duration":0.1}`)
	}))
	defer provider.Close()
	s := testServer(t)
	s.cfg.DictionPublicURL = "https://diction.test"
	s.client = provider.Client()
	if err := s.state.update(func(st *persistedState) error {
		p := st.Providers["groq"]
		p.Endpoint = provider.URL
		st.Providers["groq"] = p
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	token, err := s.MintDeviceKey("iPhone")
	if err != nil {
		t.Fatal(err)
	}
	ts := dictionTestHTTP(t, s)
	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	u.Path = "/diction/v1/audio/stream"
	u.RawQuery = "model=parakeet&language=en-IN&sample_rate=16000&codec=opus"
	c, resp, err := websocket.Dial(context.Background(), u.String(), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}, "Origin": []string{s.cfg.DictionPublicURL}}, Subprotocols: []string{"diction.opus.v1"}})
	if err != nil {
		if resp != nil {
			t.Log(resp.Status)
		}
		t.Fatal(err)
	}
	defer c.CloseNow()
	if c.Subprotocol() != "" {
		t.Fatalf("server negotiated unsupported subprotocol %q", c.Subprotocol())
	}
	writeDictionFrame(t, c, websocket.MessageText, []byte(`{"application":"Notes","before":"descriptive prose"}`))
	writeDictionFrame(t, c, websocket.MessageBinary, audiblePCM())
	writeDictionFrame(t, c, websocket.MessageText, []byte(`{"action":"done","language":"en-IN"}`))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	typ, data, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("frame type=%v", typ)
	}
	var final map[string]any
	if err := json.Unmarshal(data, &final); err != nil {
		t.Fatal(err)
	}
	if final["text"] != "hello diction" || final["mode"] != "transcribe" || len(final) != 2 {
		t.Fatalf("unexpected final: %#v", final)
	}
	state := s.state.snapshot()
	if len(state.History) != 1 || state.History[0].Model != "whisper-large-v3-turbo" || !state.History[0].AudioAvailable {
		t.Fatalf("history=%#v", state.History)
	}
	if _, err := s.state.loadAudio(state.History[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestDictionAuthenticationCodecIntentAndOrigin(t *testing.T) {
	s := testServer(t)
	token, err := s.MintDeviceKey("iPhone")
	if err != nil {
		t.Fatal(err)
	}
	ts := dictionTestHTTP(t, s)
	check := func(raw string, headers http.Header, want int) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, raw, nil)
		req.Header = headers
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("%s status=%d want=%d", raw, resp.StatusCode, want)
		}
	}
	check(ts.URL+"/diction/v1/audio/stream", nil, http.StatusUnauthorized)
	authHeader := http.Header{"Authorization": []string{"Bearer " + token}}
	check(ts.URL+"/diction/v1/audio/stream?codec=opus", authHeader, http.StatusUnsupportedMediaType)
	check(ts.URL+"/diction/v1/audio/stream?intent=edit", authHeader, http.StatusBadRequest)
	originHeader := authHeader.Clone()
	originHeader.Set("Origin", "https://evil.example")
	check(ts.URL+"/diction/v1/audio/stream", originHeader, http.StatusForbidden)
	s.cfg.DictionPublicURL = "https://hypnos.husky-canopus.ts.net:8444"
	magicHeader := authHeader.Clone()
	magicHeader.Set("Origin", s.cfg.DictionPublicURL)
	check(ts.URL+"/diction/v1/audio/stream?codec=opus", magicHeader, http.StatusUnsupportedMediaType)
	if err := s.RevokeDeviceKey(token); err != nil {
		t.Fatal(err)
	}
	check(ts.URL+"/diction/v1/audio/stream", authHeader, http.StatusUnauthorized)
}

func TestDictionCodecNegotiation(t *testing.T) {
	for _, codec := range []string{"", "pcm", "pcm16", "pcm_s16le", "linear16"} {
		if err := dictionCodec(codec, ""); err != nil {
			t.Errorf("codec %q: %v", codec, err)
		}
	}
	if err := dictionCodec("opus", "diction.opus.v1"); err != nil {
		t.Fatalf("offered Opus fallback rejected: %v", err)
	}
	if err := dictionCodec("opus", ""); err == nil {
		t.Fatal("explicit Opus without fallback was accepted")
	}
	if err := dictionCodec("aac", ""); err == nil {
		t.Fatal("unsupported codec was accepted")
	}
}

func TestDictionPCMQueryUpgrades(t *testing.T) {
	s := testServer(t)
	token, err := s.MintDeviceKey("PCM iPhone")
	if err != nil {
		t.Fatal(err)
	}
	ts := dictionTestHTTP(t, s)
	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	u.Path = "/diction/v1/audio/stream"
	u.RawQuery = "codec=pcm&model=whisper-large-v3-turbo"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, resp, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
	if err != nil {
		if resp != nil {
			t.Log(resp.Status)
		}
		t.Fatal(err)
	}
	_ = c.Close(websocket.StatusNormalClosure, "test complete")
}

func TestDictionHTTPAliasAndUnknownModel(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
		}
		if r.FormValue("model") != "whisper-large-v3-turbo" {
			t.Errorf("model=%q", r.FormValue("model"))
		}
		if r.FormValue("language") != "en" {
			t.Errorf("language=%q", r.FormValue("language"))
		}
		_, _ = io.WriteString(w, `{"text":"http diction","duration":1}`)
	}))
	defer provider.Close()
	s := testServer(t)
	s.client = provider.Client()
	_ = s.state.update(func(st *persistedState) error {
		p := st.Providers["groq"]
		p.Endpoint = provider.URL
		st.Providers["groq"] = p
		return nil
	})
	token, _ := s.MintDeviceKey("iPhone")
	ts := dictionTestHTTP(t, s)
	post := func(model string) (int, string) {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, _ := mw.CreateFormFile("file", "audio.wav")
		_, _ = part.Write(pcm16WAV(audiblePCM(), 16000))
		_ = mw.WriteField("model", model)
		_ = mw.Close()
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/diction/v1/audio/transcriptions?language=en-IN", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, string(data)
	}
	if status, body := post("whisper-1"); status != http.StatusOK || !strings.Contains(body, "http diction") {
		t.Fatalf("alias status=%d body=%s", status, body)
	}
	if status, _ := post("invented-model"); status != http.StatusBadRequest {
		t.Fatalf("unknown model status=%d", status)
	}
}

func writeDictionFrame(t *testing.T, c *websocket.Conn, typ websocket.MessageType, data []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Write(ctx, typ, data); err != nil {
		t.Fatal(err)
	}
}
