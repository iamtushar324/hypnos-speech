package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/iamtushar324/vokiri/server"
	"github.com/iamtushar324/vokiri/server/internal/auth"
	"github.com/iamtushar324/vokiri/server/internal/gateway"
)

var version = speech.Version

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := flag.String("addr", env("SPEECH_ADDR", "127.0.0.1:8680"), "HTTP listen address")
	state := flag.String("state-dir", env("SPEECH_STATE_DIR", "/state"), "retained private application state")
	origin := flag.String("public-url", env("SPEECH_PUBLIC_URL", "https://speech.tusharbhardwaj.space"), "canonical dashboard origin")
	dictionURL := flag.String("diction-url", env("SPEECH_DICTION_PUBLIC_URL", ""), "Diction capture endpoint for secure iPhone pairing")
	keyFile := flag.String("master-key-file", env("SPEECH_MASTER_KEY_FILE", "/credentials/state-master-key"), "32-byte state encryption key file")
	mint := flag.String("mint-token", "", "create a speech-only device key locally and write it to stdout")
	revoke := flag.String("revoke-token-file", "", "revoke a speech-only token read from a protected file while server is stopped")
	flag.Parse()
	if *mint != "" && *revoke != "" {
		return errors.New("choose mint-token or revoke-token-file")
	}
	masterKey, err := os.ReadFile(*keyFile)
	if err != nil {
		return fmt.Errorf("read state encryption key: %w", err)
	}
	if len(masterKey) != 32 {
		return errors.New("state encryption key must contain exactly 32 bytes")
	}
	pub, err := readCredential("SPEECH_CLERK_PUBLISHABLE_KEY", "SPEECH_CLERK_PUBLISHABLE_KEY_FILE", "/credentials/clerk-publishable-key")
	if err != nil {
		return err
	}
	secret, err := readCredential("SPEECH_CLERK_SECRET_KEY", "SPEECH_CLERK_SECRET_KEY_FILE", "/credentials/clerk-secret-key")
	if err != nil {
		return err
	}
	clerk, err := auth.New(auth.Config{SecretKey: secret, PublishableKey: pub,
		AllowedGoogleEmail: env("SPEECH_OWNER_EMAIL", ""),
		AuthorizedParties:  []string{*origin}})
	if err != nil {
		return fmt.Errorf("initialize Google authentication: %w", err)
	}
	groq, _ := readCredential("SPEECH_GROQ_API_KEY", "SPEECH_GROQ_API_KEY_FILE", "/credentials/groq-api-key")
	openrouter, _ := readCredential("SPEECH_OPENROUTER_API_KEY", "SPEECH_OPENROUTER_API_KEY_FILE", "/credentials/openrouter-api-key")
	gw, err := gateway.New(gateway.Config{StateDir: *state, MasterKey: masterKey,
		PublicURL: *origin, DictionPublicURL: *dictionURL, ClerkPublishableKey: clerk.PublishableKey(),
		ClerkFrontendAPI: clerk.FrontendAPI(), GroqAPIKey: groq, OpenRouterAPIKey: openrouter, VerifyOwner: clerk.VerifyOwner})
	if err != nil {
		return fmt.Errorf("initialize speech gateway: %w", err)
	}
	defer gw.Close()
	if *mint != "" {
		token, err := gw.MintDeviceKey(*mint)
		if err != nil {
			return err
		}
		fmt.Println(token)
		return nil
	}
	if *revoke != "" {
		b, err := os.ReadFile(*revoke)
		if err != nil {
			return fmt.Errorf("read token file: %w", err)
		}
		if err := gw.RevokeDeviceKey(strings.TrimSpace(string(b))); err != nil {
			return err
		}
		fmt.Println("Device key revoked")
		return nil
	}
	static := http.FileServer(http.FS(speech.WebAssets()))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "microphone=(self), camera=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://"+clerk.FrontendAPI()+" https://challenges.cloudflare.com; connect-src 'self' wss: https://"+clerk.FrontendAPI()+" https://clerk.shared.lcl.dev https://*.clerk.com https://challenges.cloudflare.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; media-src 'self' blob:; frame-src https://"+clerk.FrontendAPI()+" https://clerk.shared.lcl.dev https://accounts.google.com https://challenges.cloudflare.com; worker-src 'self' blob:; base-uri 'self'; object-src 'none'")
		if *dictionURL != "" && strings.HasPrefix(r.URL.Path, "/diction/") {
			http.StripPrefix("/diction", gw.DictionHandler()).ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/healthz" {
			w.Header().Set("Cache-Control", "no-store")
			gw.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/login" || r.URL.Path == "/sign-in" {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			b, err := fs.ReadFile(speech.WebAssets(), "index.html")
			if err != nil {
				http.Error(w, "Application assets unavailable", 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(b)
			return
		}
		static.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	log.Printf("Vokiri %s listening on %s", version, *addr)
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
	return nil
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func readCredential(name, fileName, fallback string) (string, error) {
	if v := os.Getenv(name); v != "" {
		return strings.TrimSpace(v), nil
	}
	b, err := os.ReadFile(env(fileName, fallback))
	if err != nil {
		return "", fmt.Errorf("read %s credential file: %w", name, err)
	}
	return strings.TrimSpace(string(b)), nil
}
