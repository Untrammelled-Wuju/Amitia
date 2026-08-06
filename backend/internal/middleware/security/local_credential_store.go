// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type LocalCredentialStore struct {
	mu        sync.RWMutex
	tokenFile string
	token     string
	version   string
}

func NewLocalCredentialStore(tokenFile string) (*LocalCredentialStore, error) {
	if strings.TrimSpace(tokenFile) == "" {
		return nil, errors.New("local token file is required")
	}

	token := strings.TrimSpace(readInstanceFile(tokenFile))
	if len(token) < 32 {
		generated, err := newLocalToken()
		if err != nil {
			return nil, fmt.Errorf("generate local token: %w", err)
		}
		if err := atomicWriteCredential(tokenFile, generated); err != nil {
			return nil, fmt.Errorf("write local token: %w", err)
		}
		token = generated
	}

	return &LocalCredentialStore{
		tokenFile: tokenFile,
		token:     token,
		version:   credentialVersion(token),
	}, nil
}

func newLocalToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func credentialVersion(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

func (s *LocalCredentialStore) Validate(candidate string) bool {
	s.mu.RLock()
	expected := s.token
	s.mu.RUnlock()

	if len(candidate) != len(expected) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1
}

func (s *LocalCredentialStore) Version() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

func (s *LocalCredentialStore) Rotate(newToken string) (string, string, error) {
	newToken = strings.TrimSpace(newToken)
	if len(newToken) < 32 {
		return "", "", errors.New("new local token is too short")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldVersion := s.version
	newVersion := credentialVersion(newToken)

	if err := atomicWriteCredential(s.tokenFile, newToken); err != nil {
		return "", "", err
	}

	s.token = newToken
	s.version = newVersion

	return oldVersion, newVersion, nil
}

func atomicWriteCredential(tokenFile string, token string) error {
	dir := filepath.Dir(tokenFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	file, err := os.CreateTemp(dir, ".local-token-*")
	if err != nil {
		return err
	}

	tempPath := file.Name()
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if err := file.Chmod(0o600); err != nil {
		return err
	}

	if _, err := file.WriteString(token); err != nil {
		return err
	}

	if err := file.Sync(); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	if err := os.Rename(tempPath, tokenFile); err != nil {
		return err
	}

	cleanup = false
	return nil
}
