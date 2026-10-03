package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const ownerEmail = "owner@example.com"

type fixture struct {
	t      *testing.T
	key    *rsa.PrivateKey
	server *httptest.Server
	mu     sync.Mutex
	user   map[string]any
	status int
	calls  int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, key: key, status: http.StatusOK}
	f.user = validUser()
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fixture) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.URL.Path == "/jwks":
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk(f.key.PublicKey)}})
	case r.URL.Path == "/users/user_owner":
		f.calls++
		w.WriteHeader(f.status)
		if f.status/100 == 2 {
			json.NewEncoder(w).Encode(f.user)
		}
	default:
		http.NotFound(w, r)
	}
}

func (f *fixture) client(t *testing.T) *Client {
	t.Helper()
	c, err := New(Config{
		SecretKey: "test-secret", PublishableKey: publishableKey("clerk.example.test"),
		AllowedGoogleEmail: ownerEmail, AuthorizedParties: []string{"https://speech.tusharbhardwaj.space"},
		HTTPClient: f.server.Client(), APIBase: f.server.URL, JWKSURL: f.server.URL + "/jwks", Issuer: "https://issuer.example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (f *fixture) token(t *testing.T, azp string, exp time.Time, key *rsa.PrivateKey) string {
	t.Helper()
	claims := sessionClaims{AuthorizedParty: azp, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: "https://issuer.example.test", Subject: "user_owner", ExpiresAt: jwt.NewNumericDate(exp), IssuedAt: jwt.NewNumericDate(time.Now()),
	}}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "key-1"
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestVerifyOwner(t *testing.T) {
	f := newFixture(t)
	c := f.client(t)
	owner, err := c.VerifyOwner(context.Background(), f.token(t, "https://speech.tusharbhardwaj.space", time.Now().Add(time.Hour), f.key))
	if err != nil {
		t.Fatalf("VerifyOwner: %v", err)
	}
	if owner.ID != "user_owner" || owner.Email != ownerEmail || owner.Name != "Owner Example" {
		t.Fatalf("owner = %#v", owner)
	}
	if c.FrontendAPI() != "clerk.example.test" || c.PublishableKey() != publishableKey("clerk.example.test") {
		t.Fatal("public Clerk config changed")
	}
}

func TestVerifyOwnerRejectsInvalidOrWrongOrigin(t *testing.T) {
	f := newFixture(t)
	for _, test := range []struct{ name, token string }{
		{"wrong azp", f.token(t, "https://toolyard.tusharbhardwaj.space", time.Now().Add(time.Hour), f.key)},
		{"expired", f.token(t, "https://speech.tusharbhardwaj.space", time.Now().Add(-time.Hour), f.key)},
		{"forged", f.token(t, "https://speech.tusharbhardwaj.space", time.Now().Add(time.Hour), mustKey(t))},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.client(t).VerifyOwner(context.Background(), test.token)
			if !errorsIs(err, ErrInvalidToken) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestVerifyOwnerRejectsUntrustedClerkProfiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"wrong primary email", func(u map[string]any) {
			u["email_addresses"].([]any)[0].(map[string]any)["email_address"] = "other@example.com"
		}},
		{"unverified primary email", func(u map[string]any) {
			u["email_addresses"].([]any)[0].(map[string]any)["verification"] = map[string]any{"status": "unverified"}
		}},
		{"non Google provider", func(u map[string]any) {
			u["external_accounts"].([]any)[0].(map[string]any)["provider"] = "oauth_github"
		}},
		{"unverified Google", func(u map[string]any) {
			u["external_accounts"].([]any)[0].(map[string]any)["verification"] = map[string]any{"status": "unverified"}
		}},
		{"banned", func(u map[string]any) { u["banned"] = true }},
		{"locked", func(u map[string]any) { u["locked"] = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.mu.Lock()
			test.mutate(f.user)
			f.mu.Unlock()
			_, err := f.client(t).VerifyOwner(context.Background(), f.token(t, "https://speech.tusharbhardwaj.space", time.Now().Add(time.Hour), f.key))
			if !errorsIs(err, ErrNotOwner) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestVerifyOwnerFailsClosedOnProviderOutageAndCachesShortly(t *testing.T) {
	f := newFixture(t)
	f.mu.Lock()
	f.status = http.StatusServiceUnavailable
	f.mu.Unlock()
	_, err := f.client(t).VerifyOwner(context.Background(), f.token(t, "https://speech.tusharbhardwaj.space", time.Now().Add(time.Hour), f.key))
	if !errorsIs(err, ErrUnavailable) {
		t.Fatalf("outage error = %v", err)
	}

	f = newFixture(t)
	c := f.client(t)
	token := f.token(t, "https://speech.tusharbhardwaj.space", time.Now().Add(time.Hour), f.key)
	if _, err := c.VerifyOwner(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, err := c.VerifyOwner(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	calls := f.calls
	f.mu.Unlock()
	if calls != 1 {
		t.Fatalf("owner backend calls = %d, want 1", calls)
	}
}

func validUser() map[string]any {
	return map[string]any{"id": "user_owner", "primary_email_address_id": "email_primary", "first_name": "Owner", "last_name": "Example", "email_addresses": []any{map[string]any{"id": "email_primary", "email_address": ownerEmail, "verification": map[string]any{"status": "verified"}}}, "external_accounts": []any{map[string]any{"provider": "oauth_google", "email_address": ownerEmail, "verification": map[string]any{"status": "verified"}}}}
}

func jwk(k rsa.PublicKey) map[string]string {
	return map[string]string{"kty": "RSA", "kid": "key-1", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes())}
}

func publishableKey(host string) string {
	return "pk_test_" + base64.RawStdEncoding.EncodeToString([]byte(host+"$"))
}
func mustKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}
func errorsIs(got, want error) bool { return got != nil && strings.Contains(got.Error(), want.Error()) }
