package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/ioshostbridge"
	"github.com/u-ai/backend/internal/secretstore"
)

type EncryptedFileStore struct {
	path string
	aead cipher.AEAD
	mu   sync.Mutex
}

var hostKeyLocks sync.Map

func NewEncryptedFileStore(path, keyPath string) (*EncryptedFileStore, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	keyPath = filepath.Clean(strings.TrimSpace(keyPath))
	if path == "." || keyPath == "." || path == keyPath {
		return nil, fmt.Errorf("secret store path is invalid")
	}
	var key []byte
	var err error
	bridge, bridgeErr := ioshostbridge.FromEnvironment()
	if bridgeErr != nil {
		return nil, bridgeErr
	}
	if bridge != nil {
		key, err = loadOrCreateHostKey(path, keyPath)
	} else {
		key, err = loadOrCreateKey(keyPath)
	}
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

func loadOrCreateHostKey(storePath, keyPath string) ([]byte, error) {
	lockValue, _ := hostKeyLocks.LoadOrStore(filepath.Clean(keyPath), &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	key, err := secretstore.Read(keyPath)
	if err == nil {
		if len(key) != 32 {
			return nil, ErrSecretStoreCorrupted
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if data, err := os.ReadFile(storePath); err == nil && len(data) > 0 {
		return nil, ErrSecretDecryptionFailed
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := secretstore.Write(keyPath, key); err != nil {
		return nil, err
	}
	return key, nil
}

func (s *EncryptedFileStore) Put(ctx context.Context, namespace string, value []byte) (string, error) {
	ref := schemeCanonical + sanitizeNamespace(namespace) + "/" + uuid.NewString()
	if err := s.PutReference(ctx, ref, value); err != nil {
		return "", err
	}
	return ref, nil
}

func (s *EncryptedFileStore) PutReference(ctx context.Context, rawRef string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ref, err := ParseRef(rawRef)
	if err != nil || rawRef != ref.String() || len(value) == 0 {
		return ErrSecretRefInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.readLocked()
	if err != nil {
		return err
	}
	if _, exists := records[rawRef]; exists {
		return ErrSecretRefInvalid
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := s.aead.Seal(nil, nonce, value, []byte(rawRef))
	records[rawRef] = base64.RawStdEncoding.EncodeToString(append(nonce, sealed...))
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.writeLocked(records)
}

func (s *EncryptedFileStore) Get(ctx context.Context, ref string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rawRef := ref
	if strings.HasPrefix(rawRef, schemeLegacy) {
		rawRef = legacyPrefixReplacer.Replace(rawRef)
	}
	if !strings.HasPrefix(rawRef, schemeCanonical) {
		return nil, ErrSecretNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	encoded, ok := records[rawRef]
	if !ok {
		return nil, ErrSecretNotFound
	}
	payload, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(payload) <= s.aead.NonceSize() {
		return nil, ErrSecretStoreCorrupted
	}
	plain, err := s.aead.Open(nil, payload[:s.aead.NonceSize()], payload[s.aead.NonceSize():], []byte(rawRef))
	if err != nil {
		return nil, ErrSecretDecryptionFailed
	}
	return plain, nil
}

func (s *EncryptedFileStore) Delete(ctx context.Context, ref string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rawRef := ref
	if strings.HasPrefix(rawRef, schemeLegacy) {
		rawRef = legacyPrefixReplacer.Replace(rawRef)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.readLocked()
	if err != nil {
		return err
	}
	delete(records, rawRef)
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
		return nil, ErrSecretStoreCorrupted
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
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".secrets-*")
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
			return nil, fmt.Errorf("invalid secret store key")
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
