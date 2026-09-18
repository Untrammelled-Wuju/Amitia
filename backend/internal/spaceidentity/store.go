package spaceidentity

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = 1

type Identity struct {
	SchemaVersion int       `json:"schemaVersion"`
	SpaceID       string    `json:"spaceId"`
	InstanceID    string    `json:"instanceId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Profile struct {
	SchemaVersion int            `json:"schemaVersion"`
	DisplayName   string         `json:"displayName"`
	Avatar        string         `json:"avatar,omitempty"`
	Bio           string         `json:"bio,omitempty"`
	UserLabel     string         `json:"userLabel,omitempty"`
	Preferences   map[string]any `json:"preferences,omitempty"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

type Store struct {
	mu       sync.RWMutex
	dataDir  string
	identity Identity
}

func Open(dataDir string) (*Store, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil, errors.New("spaceidentity: dataDir is required")
	}
	s := &Store{dataDir: dataDir}
	if err := s.loadOrCreate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Identity() Identity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identity
}

func (s *Store) SpaceID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identity.SpaceID
}

func (s *Store) identityPath() string { return filepath.Join(s.dataDir, "identity", "space.json") }
func (s *Store) profilePath() string  { return filepath.Join(s.dataDir, "profile", "profile.json") }

func (s *Store) loadOrCreate() error {
	path := s.identityPath()
	data, err := os.ReadFile(path)
	if err == nil {
		var id Identity
		if json.Unmarshal(data, &id) == nil && validID(id.SpaceID, "space_") && validID(id.InstanceID, "inst_") {
			if id.SchemaVersion == 0 {
				id.SchemaVersion = SchemaVersion
			}
			s.identity = id
			return nil
		}
		return fmt.Errorf("spaceidentity: invalid identity file %s", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("spaceidentity: read identity: %w", err)
	}

	now := time.Now().UTC()
	id := Identity{SchemaVersion: SchemaVersion, SpaceID: newID("space_"), InstanceID: newID("inst_"), CreatedAt: now}
	if err := atomicWriteJSON(path, id, 0o600); err != nil {
		return fmt.Errorf("spaceidentity: create identity: %w", err)
	}
	s.identity = id
	return nil
}

func (s *Store) ReadProfile() (Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(s.profilePath())
	if errors.Is(err, os.ErrNotExist) {
		return Profile{SchemaVersion: SchemaVersion, DisplayName: "", Preferences: map[string]any{}}, nil
	}
	if err != nil {
		return Profile{}, err
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, err
	}
	if p.SchemaVersion == 0 {
		p.SchemaVersion = SchemaVersion
	}
	if p.Preferences == nil {
		p.Preferences = map[string]any{}
	}
	return p, nil
}

func (s *Store) WriteProfile(p Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.SchemaVersion = SchemaVersion
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	p.Avatar = strings.TrimSpace(p.Avatar)
	p.Bio = strings.TrimSpace(p.Bio)
	p.UserLabel = strings.TrimSpace(p.UserLabel)
	if p.Preferences == nil {
		p.Preferences = map[string]any{}
	}
	p.UpdatedAt = time.Now().UTC()
	if len([]rune(p.DisplayName)) > 100 || len([]rune(p.UserLabel)) > 100 || len([]rune(p.Bio)) > 1000 {
		return errors.New("spaceidentity: profile field too long")
	}
	return atomicWriteJSON(s.profilePath(), p, 0o600)
}

func newID(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	return prefix + strings.ToLower(enc)
}

func validID(v, prefix string) bool {
	return strings.HasPrefix(v, prefix) && len(v) >= len(prefix)+16
}

func atomicWriteJSON(path string, v any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

var (
	defaultMu    sync.RWMutex
	defaultStore *Store
)

// InitializeDefault establishes the process-wide canonical personal Space.
// It is safe to call more than once for the same data directory.
func InitializeDefault(dataDir string) (*Store, error) {
	s, err := Open(dataDir)
	if err != nil {
		return nil, err
	}
	defaultMu.Lock()
	defaultStore = s
	defaultMu.Unlock()
	return s, nil
}

func DefaultSpaceID() string {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if defaultStore == nil {
		return ""
	}
	return defaultStore.SpaceID()
}

func DefaultStore() *Store {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultStore
}
