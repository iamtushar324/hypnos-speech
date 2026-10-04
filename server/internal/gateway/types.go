package gateway

import (
	"context"
	"time"

	"github.com/iamtushar324/vokiri/server"
	"github.com/iamtushar324/vokiri/server/internal/auth"
)

const version = speech.Version

type Config struct {
	StateDir            string
	MasterKey           []byte
	PublicURL           string
	DictionPublicURL    string
	ClerkPublishableKey string
	ClerkFrontendAPI    string
	GroqAPIKey          string
	OpenRouterAPIKey    string
	VerifyOwner         func(context.Context, string) (auth.Owner, error)
}

type Settings struct {
	DefaultSTTModel       string `json:"default_stt_model"`
	DefaultTTSModel       string `json:"default_tts_model"`
	DefaultVoice          string `json:"default_voice"`
	Language              string `json:"language"`
	SaveHistoryText       bool   `json:"save_history_text"`
	DefaultProfileID      string `json:"default_profile_id"`
	CleanupEnabled        bool   `json:"cleanup_enabled"`
	CleanupModel          string `json:"cleanup_model"`
	CleanupPrompt         string `json:"cleanup_prompt"`
	CleanupTimeoutSeconds int    `json:"cleanup_timeout_seconds"`
}

type Provider struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Configured       bool       `json:"configured"`
	HasAPIKey        bool       `json:"has_api_key"`
	Endpoint         string     `json:"endpoint"`
	Region           string     `json:"region"`
	StreamDeployment string     `json:"stream_deployment"`
	BatchDeployment  string     `json:"batch_deployment"`
	VerifiedAt       *time.Time `json:"verified_at"`
	LastError        string     `json:"last_error"`
}

type providerState struct {
	APIKey            string     `json:"api_key"`
	APIKeyInitialized bool       `json:"api_key_initialized"`
	Endpoint          string     `json:"endpoint"`
	Region            string     `json:"region"`
	StreamDeployment  string     `json:"stream_deployment"`
	BatchDeployment   string     `json:"batch_deployment"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	LastError         string     `json:"last_error,omitempty"`
}

type Model struct {
	ID          string   `json:"id"`
	Provider    string   `json:"provider"`
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Configured  bool     `json:"configured"`
	Streaming   bool     `json:"streaming"`
	Languages   []string `json:"languages"`
}

type DictionaryEntry struct {
	ID          string `json:"id"`
	Word        string `json:"word"`
	Replacement string `json:"replacement"`
	Notes       string `json:"notes"`
}

type Snippet struct {
	ID      string `json:"id"`
	Trigger string `json:"trigger"`
	Text    string `json:"text"`
}

type Profile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	STTModel    string `json:"stt_model"`
	TTSModel    string `json:"tts_model"`
	Voice       string `json:"voice"`
}

type History struct {
	ID                  string    `json:"id"`
	Kind                string    `json:"kind"`
	Model               string    `json:"model"`
	Provider            string    `json:"provider"`
	CreatedAt           time.Time `json:"created_at"`
	Status              string    `json:"status"`
	LatencyMS           int64     `json:"latency_ms"`
	DurationSeconds     float64   `json:"duration_seconds"`
	InputChars          int       `json:"input_chars"`
	Text                string    `json:"text"`
	EstimatedCostUSD    float64   `json:"estimated_cost_usd"`
	Error               string    `json:"error"`
	Voice               string    `json:"voice"`
	AudioAvailable      bool      `json:"audio_available"`
	AudioContentType    string    `json:"-"`
	RawText             string    `json:"raw_text,omitempty"`
	CleanupStatus       string    `json:"cleanup_status,omitempty"`
	CleanupModel        string    `json:"cleanup_model,omitempty"`
	CleanupLatencyMS    int64     `json:"cleanup_latency_ms,omitempty"`
	CleanupError        string    `json:"cleanup_error,omitempty"`
	CleanupCostUSD      *float64  `json:"cleanup_cost_usd,omitempty"`
	CleanupInputTokens  int       `json:"cleanup_input_tokens,omitempty"`
	CleanupOutputTokens int       `json:"cleanup_output_tokens,omitempty"`
}

type DeviceKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Hash       string     `json:"hash"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	Revoked    bool       `json:"revoked"`
}

type persistedState struct {
	Settings   Settings                 `json:"settings"`
	Providers  map[string]providerState `json:"providers"`
	Dictionary []DictionaryEntry        `json:"dictionary"`
	Snippets   []Snippet                `json:"snippets"`
	Profiles   []Profile                `json:"profiles"`
	History    []History                `json:"history"`
	AudioTypes map[string]string        `json:"audio_types"`
	Keys       []DeviceKey              `json:"keys"`
}
