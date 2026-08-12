//go:build linux && !android

package ssh

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type HostKeyStore struct {
	mu    sync.RWMutex
	path  string
	hosts map[string]KnownHost
}

func NewHostKeyStore(path string, autoCreate bool) (*HostKeyStore, error) {
	store := &HostKeyStore{
		path:  path,
		hosts: make(map[string]KnownHost),
	}

	if path == "" {
		return store, nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if autoCreate {
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0700); err != nil {
				return nil, ErrInternal(fmt.Sprintf("failed to create known_hosts dir: %v", err))
			}
			if err := store.save(); err != nil {
				return nil, err
			}
		}
		return store, nil
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *HostKeyStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return ErrInternal(fmt.Sprintf("failed to read known_hosts: %v", err))
	}

	var hosts []KnownHost
	if err := json.Unmarshal(data, &hosts); err != nil {
		return ErrInternal(fmt.Sprintf("failed to parse known_hosts: %v", err))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, h := range hosts {
		key := hostKeyIndex(h.Host, h.Port)
		s.hosts[key] = h
	}
	return nil
}

func (s *HostKeyStore) save() error {
	s.mu.RLock()
	hosts := make([]KnownHost, 0, len(s.hosts))
	for _, h := range s.hosts {
		hosts = append(hosts, h)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(hosts, "", "  ")
	if err != nil {
		return ErrInternal(fmt.Sprintf("failed to marshal known_hosts: %v", err))
	}

	if s.path == "" {
		return nil
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return ErrInternal(fmt.Sprintf("failed to write known_hosts tmp: %v", err))
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return ErrInternal(fmt.Sprintf("failed to rename known_hosts: %v", err))
	}

	return nil
}

func (s *HostKeyStore) Get(host string, port int) (KnownHost, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := hostKeyIndex(host, port)
	h, ok := s.hosts[key]
	return h, ok
}

func (s *HostKeyStore) Put(host KnownHost) error {
	s.mu.Lock()
	key := hostKeyIndex(host.Host, host.Port)
	s.hosts[key] = host
	s.mu.Unlock()
	return s.save()
}

func (s *HostKeyStore) Delete(host string, port int) error {
	s.mu.Lock()
	key := hostKeyIndex(host, port)
	delete(s.hosts, key)
	s.mu.Unlock()
	return s.save()
}

func (s *HostKeyStore) List() []KnownHost {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]KnownHost, 0, len(s.hosts))
	for _, h := range s.hosts {
		result = append(result, h)
	}
	return result
}

func (s *HostKeyStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.hosts)
}

func hostKeyIndex(host string, port int) string {
	if port == 22 || port == 0 {
		return host
	}
	return fmt.Sprintf("[%s]:%d", host, port)
}

func ComputeFingerprint(pubKeyBytes []byte) string {
	hash := sha256.Sum256(pubKeyBytes)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])
}
