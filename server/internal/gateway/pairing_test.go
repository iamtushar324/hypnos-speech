package gateway

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestPairingRevealedOnceAndDeviceCannotAdmin(t *testing.T) {
	s := testServer(t)
	s.cfg.DictionPublicURL = "https://hypnos.test.ts.net:8444"
	req := httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(`{"name":"iPhone"}`))
	req.Header.Set("Authorization", "Bearer owner")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create status %d", w.Code)
	}
	var key struct {
		ID, Token string
		Diction   map[string]string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &key); err != nil {
		t.Fatal(err)
	}
	link, err := url.Parse(key.Diction["pairing_uri"])
	if err != nil || link.Scheme != "diction" || link.Host != "pair" || link.Query().Get("key") != key.Token || link.Query().Get("url") != s.cfg.DictionPublicURL {
		t.Fatal("invalid pairing payload")
	}
	png, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(key.Diction["qr_data_url"], "data:image/png;base64,"))
	if err != nil || !strings.HasPrefix(string(png), "\x89PNG") {
		t.Fatal("invalid pairing QR")
	}
	req = httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	req.Header.Set("Authorization", "Bearer owner")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), key.Token) || strings.Contains(w.Body.String(), "pairing_uri") || strings.Contains(w.Body.String(), "qr_data_url") {
		t.Fatal("pairing secret repeated in key list")
	}
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+key.Token)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("pairing key gained admin access")
	}
}

func TestConcurrentPairingKeysRevokeOnlyTheirOwnToken(t *testing.T) {
	s := testServer(t)
	type issuedKey struct{ ID, Token string }
	keys := make([]issuedKey, 12)
	var wg sync.WaitGroup
	for i := range keys {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(fmt.Sprintf(`{"name":"iPhone %d"}`, i)))
			r.Header.Set("Authorization", "Bearer owner")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &keys[i]) != nil {
				t.Errorf("key %d creation failed", i)
			}
		}(i)
	}
	wg.Wait()
	for i, key := range keys {
		r := httptest.NewRequest(http.MethodDelete, "/api/keys/"+key.ID, nil)
		r.Header.Set("Authorization", "Bearer owner")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatalf("key %d revocation failed", i)
		}
		for j, other := range keys {
			r = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			r.Header.Set("Authorization", "Bearer "+other.Token)
			w = httptest.NewRecorder()
			s.ServeHTTP(w, r)
			want := 200
			if j <= i {
				want = 401
			}
			if w.Code != want {
				t.Fatalf("revoking key %d changed wrong key %d: status %d", i, j, w.Code)
			}
		}
	}
}
