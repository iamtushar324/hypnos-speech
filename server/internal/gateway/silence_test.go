package gateway

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/coder/websocket"
)

// Synthetic audible input for provider-routing tests. Zero PCM is now correctly skipped.
func audiblePCM() []byte {
	data := make([]byte, 16000/5*2)
	for i := 0; i < len(data)/2; i++ {
		n := int16(2000 * math.Sin(2*math.Pi*220*float64(i)/16000))
		binary.LittleEndian.PutUint16(data[i*2:], uint16(n))
	}
	return data
}

func TestSilenceGate(t *testing.T) {
	quiet := append([]byte(nil), audiblePCM()...)
	for i := 0; i < len(quiet); i += 2 {
		n := int16(binary.LittleEndian.Uint16(quiet[i:])) / 20 // -53 dBFS RMS, deliberately soft.
		binary.LittleEndian.PutUint16(quiet[i:], uint16(n))
	}
	noise := make([]byte, 32000)
	dc := make([]byte, 32000)
	click := make([]byte, 32000)
	for i := 0; i < len(noise); i += 2 {
		binary.LittleEndian.PutUint16(noise[i:], uint16(int16((i*17)%31-15)))
		binary.LittleEndian.PutUint16(dc[i:], 1000)
	}
	binary.LittleEndian.PutUint16(click[3200:], 30000)
	binary.LittleEndian.PutUint16(click[20000:], 30000)
	speechLate := append(make([]byte, 16000*10*2), quiet...)
	for _, tc := range []struct {
		name   string
		pcm    []byte
		silent bool
	}{
		{"empty capture", nil, true}, {"silence", make([]byte, 32000), true},
		{"quiet noise", noise, true}, {"DC offset", dc, true}, {"mic click", click, true},
		{"audible signal", audiblePCM(), false}, {"soft signal", quiet, false},
		{"soft signal after long silence", speechLate, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := silentWAV(pcm16WAV(tc.pcm, 16000)); got != tc.silent {
				t.Fatalf("silent=%v, want %v", got, tc.silent)
			}
		})
	}
	// Do not classify malformed or undecodable input as silent.
	for _, input := range [][]byte{[]byte("compressed-or-unknown"), []byte("RIFF"), pcm16WAV([]byte{0}, 16000)} {
		if silentWAV(input) {
			t.Fatal("unknown audio was silently discarded")
		}
	}
	if _, err := parseWAV(pcm16WAV(nil, 16000)); err == nil {
		t.Fatal("TTS parser must still reject empty audio")
	}
}

func TestSilenceGateFormatsAndChannels(t *testing.T) {
	for _, format := range []uint16{1, 3} {
		for _, bits := range []int{8, 16, 24, 32, 64} {
			if format == 1 && bits == 64 || format == 3 && bits < 32 {
				continue
			}
			for _, rate := range []int{8000, 16000, 44100, 48000} {
				for _, voiced := range []bool{false, true} {
					pcm := make([]byte, rate/5*2*bits/8)
					for i := 0; i < rate/5; i++ {
						for channel := 0; channel < 2; channel++ {
							sample := 0.0
							if voiced {
								sample = 0.05 * math.Sin(2*math.Pi*220*float64(i)/float64(rate))
							}
							if channel == 1 {
								sample = -sample
							} // opposite phase must not cancel
							out := pcm[(i*2+channel)*bits/8:]
							if format == 3 {
								if bits == 32 {
									binary.LittleEndian.PutUint32(out, math.Float32bits(float32(sample)))
								} else {
									binary.LittleEndian.PutUint64(out, math.Float64bits(sample))
								}
							} else {
								switch bits {
								case 8:
									out[0] = byte(128 + sample*127)
								case 16:
									binary.LittleEndian.PutUint16(out, uint16(int16(sample*32767)))
								case 24:
									n := int32(sample * 8388607)
									out[0], out[1], out[2] = byte(n), byte(n>>8), byte(n>>16)
								case 32:
									binary.LittleEndian.PutUint32(out, uint32(int32(sample*2147483647)))
								}
							}
						}
					}
					wav := pcm16WAV(pcm, rate)
					binary.LittleEndian.PutUint16(wav[20:], format)
					binary.LittleEndian.PutUint16(wav[22:], 2)
					binary.LittleEndian.PutUint32(wav[28:], uint32(rate*2*bits/8))
					binary.LittleEndian.PutUint16(wav[32:], uint16(2*bits/8))
					binary.LittleEndian.PutUint16(wav[34:], uint16(bits))
					if got := silentWAV(wav); got == voiced {
						t.Fatalf("format=%d bits=%d rate=%d voiced=%v silent=%v", format, bits, rate, voiced, got)
					}
				}
			}
		}
	}
}

func noProviderServer(t *testing.T) (*Server, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `{"text":"invented words","duration":1}`)
	}))
	t.Cleanup(provider.Close)
	s := testServer(t)
	s.client = provider.Client()
	enableCleanup(t, s, provider.URL)
	return s, calls
}

func assertNoSpeechSideEffects(t *testing.T, s *Server, calls *atomic.Int32) {
	t.Helper()
	if calls.Load() != 0 {
		t.Fatalf("provider calls=%d", calls.Load())
	}
	if len(s.state.snapshot().History) != 0 {
		t.Fatal("silent request created history")
	}
	files, err := os.ReadDir(filepath.Join(s.state.dir, "audio"))
	if err != nil || len(files) != 0 {
		t.Fatalf("silent request retained server audio: count=%d err=%v", len(files), err)
	}
}

func TestSilentMultipartSkipsProvidersCleanupAndHistory(t *testing.T) {
	for _, route := range []string{"/v1/audio/transcriptions", "/diction/v1/audio/transcriptions"} {
		for _, format := range []string{"json", "text"} {
			t.Run(route+"/"+format, func(t *testing.T) {
				s, calls := noProviderServer(t)
				ts := silenceTestHTTP(t, s)
				token, err := s.MintDeviceKey("silence-test")
				if err != nil {
					t.Fatal(err)
				}
				var body bytes.Buffer
				mw := multipart.NewWriter(&body)
				part, _ := mw.CreateFormFile("file", "capture.wav")
				_, _ = part.Write(pcm16WAV(make([]byte, 32000), 16000))
				_ = mw.WriteField("response_format", format)
				_ = mw.Close()
				req, _ := http.NewRequest(http.MethodPost, ts.URL+route, &body)
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", mw.FormDataContentType())
				response, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				data, _ := io.ReadAll(response.Body)
				if response.StatusCode != 200 || response.Header.Get("X-Speech-No-Speech") != "true" {
					t.Fatalf("status=%d body=%s", response.StatusCode, data)
				}
				if format == "text" {
					if len(data) != 0 {
						t.Fatalf("silent text=%q", data)
					}
				} else {
					var result map[string]any
					if json.Unmarshal(data, &result) != nil || result["text"] != "" || result["no_speech"] != true {
						t.Fatalf("response=%s", data)
					}
				}
				assertNoSpeechSideEffects(t, s, calls)
			})
		}
	}
}

func TestSilentBatchSocketsReturnEmptyFinal(t *testing.T) {
	for _, diction := range []bool{false, true} {
		s, calls := noProviderServer(t)
		ts := silenceTestHTTP(t, s)
		token, err := s.MintDeviceKey("silence-test")
		if err != nil {
			t.Fatal(err)
		}
		target, _ := url.Parse(ts.URL)
		target.Scheme = "ws"
		target.Path = "/v1/audio/stream"
		if diction {
			target.Path = "/diction/v1/audio/stream"
			target.RawQuery = "codec=pcm"
		}
		c, _, err := websocket.Dial(context.Background(), target.String(), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
		if err != nil {
			t.Fatal(err)
		}
		defer c.CloseNow()
		if !diction {
			writeTestJSON(t, c, map[string]any{"type": "start", "model": "whisper-large-v3-turbo"})
			var ready map[string]any
			readTestJSON(t, c, &ready)
		}
		writeDictionFrame(t, c, websocket.MessageBinary, make([]byte, 32000))
		writeTestJSON(t, c, map[string]any{"action": "done"})
		var final map[string]any
		readTestJSON(t, c, &final)
		if final["text"] != "" {
			t.Fatalf("silent text=%#v", final)
		}
		if diction {
			if final["mode"] != "transcribe" || len(final) != 2 {
				t.Fatalf("Diction wire changed: %#v", final)
			}
		} else if final["type"] != "final" || final["no_speech"] != true {
			t.Fatalf("native final=%#v", final)
		}
		assertNoSpeechSideEffects(t, s, calls)
	}
}

func BenchmarkSilentFiveSeconds(b *testing.B) {
	audio := pcm16WAV(make([]byte, 5*16000*2), 16000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !silentWAV(audio) {
			b.Fatal("silence not detected")
		}
	}
}

func silenceTestHTTP(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/", s)
	mux.Handle("/diction/", http.StripPrefix("/diction", s.DictionHandler()))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestAudibleShortPhraseIsNotFilteredOrModified(t *testing.T) {
	audio := pcm16WAV(audiblePCM(), 16000)
	calls := atomic.Int32{}
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if err := r.ParseMultipartForm(maxAudioBody); err != nil {
			t.Error(err)
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		got, _ := io.ReadAll(f)
		if !bytes.Equal(got, audio) {
			t.Error("speech audio changed before transcription")
		}
		_, _ = io.WriteString(w, `{"text":"Thank you.","duration":0.2}`)
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
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "capture.wav")
	_, _ = part.Write(audio)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &body)
	req.Header.Set("Authorization", "Bearer owner")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	response := httptest.NewRecorder()
	s.ServeHTTP(response, req)
	var result map[string]any
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	if response.Code != 200 || result["text"] != "Thank you." || calls.Load() != 1 {
		t.Fatalf("code=%d calls=%d result=%#v", response.Code, calls.Load(), result)
	}
}
