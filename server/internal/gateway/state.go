package gateway

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// Persisted encryption format: keep these legacy AAD values across product renames.
const stateAAD = "hypnos-speech-state-v1"

type stateStore struct {
	mu    sync.RWMutex
	dir   string
	key   []byte
	lock  *os.File
	state persistedState
}

func newStateStore(dir string, key []byte, groqKey, openRouterKey string) (*stateStore, error) {
	if dir == "" {
		return nil, errors.New("state directory is required")
	}
	if len(key) != 32 {
		return nil, errors.New("master key must contain exactly 32 bytes")
	}
	if err := os.MkdirAll(filepath.Join(dir, "audio"), 0700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, fmt.Errorf("protect state directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open state lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, errors.New("speech state is already open by another process")
	}
	ok := false
	defer func() {
		if !ok {
			_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
			_ = lock.Close()
		}
	}()
	s := &stateStore{dir: dir, key: append([]byte(nil), key...), lock: lock}
	s.state = defaultState(groqKey, openRouterKey)
	data, err := os.ReadFile(s.statePath())
	if err == nil {
		plain, decErr := s.decrypt(data, []byte(stateAAD))
		if decErr != nil {
			return nil, fmt.Errorf("decrypt state: %w", decErr)
		}
		if err := json.Unmarshal(plain, &s.state); err != nil {
			return nil, fmt.Errorf("decode state: %w", err)
		}
		if s.state.Providers == nil {
			s.state.Providers = map[string]providerState{}
		}
		if s.state.AudioTypes == nil {
			s.state.AudioTypes = map[string]string{}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read state: %w", err)
	} else if err := s.persistLocked(s.state); err != nil {
		return nil, err
	}
	changed := false
	if _, exists := s.state.Providers["openrouter"]; !exists {
		s.state.Providers["openrouter"] = providerState{Endpoint: openRouterEndpoint}
		changed = true
	}
	op := s.state.Providers["openrouter"]
	if op.Endpoint != openRouterEndpoint {
		op.Endpoint = openRouterEndpoint
		s.state.Providers["openrouter"] = op
		changed = true
	}
	if s.state.Settings.CleanupModel == "" {
		s.state.Settings.CleanupModel = defaultCleanupModel
		changed = true
	}
	if s.state.Settings.CleanupPrompt == "" {
		s.state.Settings.CleanupPrompt = defaultCleanupPrompt
		changed = true
	}
	if s.state.Settings.CleanupTimeoutSeconds == 0 {
		s.state.Settings.CleanupTimeoutSeconds = 10
		changed = true
	}
	// Mounted bootstrap keys only fill credentials never explicitly initialized.
	if groqKey != "" && !s.state.Providers["groq"].APIKeyInitialized {
		p := s.state.Providers["groq"]
		p.APIKey = groqKey
		p.APIKeyInitialized = true
		s.state.Providers["groq"] = p
		changed = true
	}
	if openRouterKey != "" && !s.state.Providers["openrouter"].APIKeyInitialized {
		p := s.state.Providers["openrouter"]
		p.APIKey = openRouterKey
		p.APIKeyInitialized = true
		p.Endpoint = openRouterEndpoint
		s.state.Providers["openrouter"] = p
		changed = true
	}
	if changed {
		if err := s.persistLocked(s.state); err != nil {
			return nil, err
		}
	}
	ok = true
	return s, nil
}

func defaultState(groqKey, openRouterKey string) persistedState {
	return persistedState{
		Settings: Settings{DefaultSTTModel: "whisper-large-v3-turbo", DefaultTTSModel: "canopylabs/orpheus-v1-english", DefaultVoice: "troy", Language: "auto", SaveHistoryText: true, CleanupModel: defaultCleanupModel, CleanupPrompt: defaultCleanupPrompt, CleanupTimeoutSeconds: 10},
		Providers: map[string]providerState{
			"groq":       {APIKey: groqKey, APIKeyInitialized: groqKey != "", Endpoint: "https://api.groq.com/openai/v1"},
			"microsoft":  {},
			"openrouter": {APIKey: openRouterKey, APIKeyInitialized: openRouterKey != "", Endpoint: openRouterEndpoint},
		},
		Dictionary: []DictionaryEntry{}, Snippets: []Snippet{}, Profiles: []Profile{}, History: []History{}, AudioTypes: map[string]string{}, Keys: []DeviceKey{},
	}
}

func (s *stateStore) statePath() string { return filepath.Join(s.dir, "state.enc") }

func (s *stateStore) snapshot() persistedState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.state)
	var out persistedState
	_ = json.Unmarshal(b, &out)
	return out
}

func (s *stateStore) update(fn func(*persistedState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(s.state)
	if err != nil {
		return err
	}
	var next persistedState
	if err := json.Unmarshal(b, &next); err != nil {
		return err
	}
	if err := fn(&next); err != nil {
		return err
	}
	if err := s.persistLocked(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *stateStore) persistLocked(st persistedState) error {
	plain, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	sealed, err := s.encrypt(plain, []byte(stateAAD))
	if err != nil {
		return err
	}
	return atomicPrivateWrite(s.statePath(), sealed)
}

func atomicPrivateWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".speech-*")
	if err != nil {
		return fmt.Errorf("create temporary state: %w", err)
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	_ = d.Close()
	if err != nil {
		return err
	}
	ok = true
	return nil
}

func (s *stateStore) encrypt(plain, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return append(nonce, gcm.Seal(nil, nonce, plain, aad)...), nil
}

func (s *stateStore) decrypt(sealed, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("encrypted value is truncated")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], aad)
}

func (s *stateStore) saveAudio(id string, audio []byte) error {
	sealed, err := s.encrypt(audio, []byte("hypnos-speech-audio-v1:"+id))
	if err != nil {
		return err
	}
	return atomicPrivateWrite(filepath.Join(s.dir, "audio", id+".enc"), sealed)
}

func (s *stateStore) loadAudio(id string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(s.dir, "audio", id+".enc"))
	if err != nil {
		return nil, err
	}
	return s.decrypt(b, []byte("hypnos-speech-audio-v1:"+id))
}

func (s *stateStore) close() {
	for i := range s.key {
		s.key[i] = 0
	}
	if s.lock != nil {
		_ = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		_ = s.lock.Close()
		s.lock = nil
	}
}
