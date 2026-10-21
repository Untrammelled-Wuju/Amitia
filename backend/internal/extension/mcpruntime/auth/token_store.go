package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
)

var ErrSecretNotFound = errors.New("MCP_SECRET_NOT_FOUND")

type SecretStore = secret.Store

type EncryptedFileStore struct {
	path string
	aead cipher.AEAD
	mu   sync.Mutex
}

func NewEncryptedFileStore(path, keyPath string) (*EncryptedFileStore, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	keyPath = filepath.Clean(strings.TrimSpace(keyPath))
	if path == "." || keyPath == "." || path == keyPath {
		return nil, fmt.Errorf("MCP secret store path is invalid")
	}
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &EncryptedFileStore{path: path, aead: aead}, nil
}

func (s *EncryptedFileStore) Put(ctx context.Context, namespace string, value []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(value) == 0 {
		return "", fmt.Errorf("MCP secret must not be empty")
	}
	reference := "mcp-secret://" + sanitizeNamespace(namespace) + "/" + uuid.NewString()
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.readLocked()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := s.aead.Seal(nil, nonce, value, []byte(reference))
	records[reference] = base64.RawStdEncoding.EncodeToString(append(nonce, sealed...))
	if err := s.writeLocked(records); err != nil {
		return "", err
	}
	return reference, nil
}

func (s *EncryptedFileStore) Get(ctx context.Context, reference string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(reference, "mcp-secret://") {
		return nil, ErrSecretNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	encoded, ok := records[reference]
	if !ok {
		return nil, ErrSecretNotFound
	}
	payload, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(payload) <= s.aead.NonceSize() {
		return nil, fmt.Errorf("MCP secret store is corrupted")
	}
	plain, err := s.aead.Open(nil, payload[:s.aead.NonceSize()], payload[s.aead.NonceSize():], []byte(reference))
	if err != nil {
		return nil, fmt.Errorf("MCP secret store decryption failed")
	}
	return plain, nil
}

func (s *EncryptedFileStore) Delete(ctx context.Context, reference string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.readLocked()
	if err != nil {
		return err
	}
	delete(records, reference)
	return s.writeLocked(records)
}

func (s *EncryptedFileStore) readLocked() (map[string]string, error) {
	records := map[string]string{}
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return records, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return records, nil
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("MCP secret store is corrupted")
	}
	return records, nil
}

func (s *EncryptedFileStore) writeLocked(records map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	raw, err := json.Marshal(records)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".mcp-secrets-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		if removeErr := os.Remove(s.path); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		return os.Rename(temporaryPath, s.path)
	}
	return nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		key, decodeErr := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if decodeErr != nil || len(key) != 32 {
			return nil, fmt.Errorf("invalid MCP secret store key")
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return loadOrCreateKey(path)
	}
	if err != nil {
		return nil, err
	}
	if _, err := file.WriteString(base64.RawStdEncoding.EncodeToString(key)); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return key, nil
}

func sanitizeNamespace(value string) string {
	value = strings.TrimSpace(value)
	var result strings.Builder
	for _, item := range value {
		if item >= 'a' && item <= 'z' || item >= 'A' && item <= 'Z' || item >= '0' && item <= '9' || item == '-' || item == '_' {
			result.WriteRune(item)
		}
	}
	if result.Len() == 0 {
		return "default"
	}
	return result.String()
}
