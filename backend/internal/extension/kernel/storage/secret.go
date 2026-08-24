package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

type SecretRef struct {
	RefID     string
	Owner     ResourceOwner
	Name      string
	Version   int64
	Algorithm string
	CreatedAt time.Time
	RotatedAt *time.Time
	ExpiresAt *time.Time
	Revoked   bool
}

type SecretValue struct {
	RefID     string
	Version   int64
	Plaintext []byte
	ExpiresAt *time.Time
}

type SecretCreateRequest struct {
	Owner     ResourceOwner
	Name      string
	Plaintext []byte
	TTL       time.Duration
	Algorithm string
	Shared    bool
}

type SecretReadRequest struct {
	Owner   ResourceOwner
	Name    string
	RefID   string
	Version int64
}

type SecretRotateRequest struct {
	Owner     ResourceOwner
	Name      string
	Plaintext []byte
}

type SecretShareRequest struct {
	Owner       ResourceOwner
	Name        string
	TargetOwner ResourceOwner
	ExpiresIn   time.Duration
}

type SecretBroker interface {
	Create(ctx context.Context, request SecretCreateRequest) (SecretRef, error)
	Read(ctx context.Context, request SecretReadRequest) (SecretValue, error)
	Rotate(ctx context.Context, request SecretRotateRequest) (SecretRef, error)
	Revoke(ctx context.Context, owner ResourceOwner, name string) error
	Share(ctx context.Context, request SecretShareRequest) (SecretRef, error)
	List(ctx context.Context, owner ResourceOwner) ([]SecretRef, error)
	Delete(ctx context.Context, owner ResourceOwner, name string) error
}

var (
	ErrSecretNotFound     = errors.New("secret: not found")
	ErrSecretRevoked      = errors.New("secret: revoked")
	ErrSecretExpired      = errors.New("secret: expired")
	ErrSecretNameConflict = errors.New("secret: name conflict")
	ErrSecretAccessDenied = errors.New("secret: access denied")
	ErrInvalidAlgorithm   = errors.New("secret: invalid algorithm")
)

const (
	AlgorithmAES256GCM = "aes-256-gcm"
	AlgorithmPlain     = "plain"
)

type DefaultSecretBroker struct {
	mu          sync.RWMutex
	secrets     map[string]*secretEntry
	masterKey   []byte
	auditWriter SecretAuditWriter
}

type secretEntry struct {
	ref      SecretRef
	cipher   []byte
	nonce    []byte
	versions []secretVersion
	shared   map[string]time.Time
}

type secretVersion struct {
	version   int64
	cipher    []byte
	nonce     []byte
	createdAt time.Time
	expiresAt *time.Time
	revoked   bool
}

type SecretAuditWriter interface {
	RecordSecretEvent(ctx context.Context, event string, ref SecretRef, owner ResourceOwner)
}

func NewDefaultSecretBroker(masterKey []byte) *DefaultSecretBroker {
	if len(masterKey) < 32 {
		k := sha256.Sum256(append([]byte("amitia-default-key"), masterKey...))
		masterKey = k[:]
	}
	return &DefaultSecretBroker{
		secrets:   make(map[string]*secretEntry),
		masterKey: masterKey[:32],
	}
}

func (b *DefaultSecretBroker) SetAuditWriter(w SecretAuditWriter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.auditWriter = w
}

func (b *DefaultSecretBroker) Create(_ context.Context, request SecretCreateRequest) (SecretRef, error) {
	if request.Owner.ExtensionID == "" {
		return SecretRef{}, ErrInvalidOwner
	}
	if request.Name == "" {
		return SecretRef{}, ErrInvalidNamespace
	}
	if len(request.Plaintext) == 0 {
		return SecretRef{}, ErrInvalidNamespace
	}
	algo := request.Algorithm
	if algo == "" {
		algo = AlgorithmAES256GCM
	}
	if algo != AlgorithmAES256GCM && algo != AlgorithmPlain {
		return SecretRef{}, ErrInvalidAlgorithm
	}
	key := secretKey(request.Owner, request.Name)
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.secrets[key]; exists {
		return SecretRef{}, ErrSecretNameConflict
	}
	cipherData, nonce, err := b.encryptLocked(algo, request.Plaintext)
	if err != nil {
		return SecretRef{}, err
	}
	now := time.Now().UTC()
	ref := SecretRef{
		RefID:     newSecretRefID(request.Owner, request.Name),
		Owner:     request.Owner,
		Name:      request.Name,
		Version:   1,
		Algorithm: algo,
		CreatedAt: now,
	}
	entry := &secretEntry{
		ref:    ref,
		cipher: cipherData,
		nonce:  nonce,
		versions: []secretVersion{{
			version:   1,
			cipher:    cipherData,
			nonce:     nonce,
			createdAt: now,
			expiresAt: expiryFromTTL(now, request.TTL),
		}},
		shared: make(map[string]time.Time),
	}
	b.secrets[key] = entry
	b.recordEventLocked("create", ref, request.Owner)
	return ref, nil
}

func (b *DefaultSecretBroker) Read(_ context.Context, request SecretReadRequest) (SecretValue, error) {
	if request.RefID == "" && request.Name == "" {
		return SecretValue{}, ErrInvalidNamespace
	}
	b.mu.RLock()
	var entry *secretEntry
	if request.RefID != "" {
		for _, e := range b.secrets {
			if e.ref.RefID == request.RefID {
				entry = e
				break
			}
		}
	} else if request.Name != "" {
		if existing, ok := b.secrets[secretKey(request.Owner, request.Name)]; ok {
			entry = existing
		} else {
			for _, e := range b.secrets {
				if e.ref.Name == request.Name {
					entry = e
					break
				}
			}
		}
	}
	b.mu.RUnlock()
	if entry == nil {
		return SecretValue{}, ErrSecretNotFound
	}
	if entry.ref.Revoked {
		return SecretValue{}, ErrSecretRevoked
	}
	if !b.canAccessLocked(entry, request.Owner) {
		return SecretValue{}, ErrSecretAccessDenied
	}
	targetVersion := request.Version
	if targetVersion == 0 {
		targetVersion = entry.ref.Version
	}
	var version *secretVersion
	for i := range entry.versions {
		if entry.versions[i].version == targetVersion {
			version = &entry.versions[i]
			break
		}
	}
	if version == nil {
		return SecretValue{}, ErrSecretNotFound
	}
	if version.revoked {
		return SecretValue{}, ErrSecretRevoked
	}
	if version.expiresAt != nil && time.Now().UTC().After(*version.expiresAt) {
		return SecretValue{}, ErrSecretExpired
	}
	plaintext, err := b.decryptLocked(entry.ref.Algorithm, version.cipher, version.nonce)
	if err != nil {
		return SecretValue{}, err
	}
	return SecretValue{
		RefID:     entry.ref.RefID,
		Version:   version.version,
		Plaintext: plaintext,
		ExpiresAt: version.expiresAt,
	}, nil
}

func (b *DefaultSecretBroker) Rotate(_ context.Context, request SecretRotateRequest) (SecretRef, error) {
	if len(request.Plaintext) == 0 {
		return SecretRef{}, ErrInvalidNamespace
	}
	key := secretKey(request.Owner, request.Name)
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.secrets[key]
	if !ok {
		return SecretRef{}, ErrSecretNotFound
	}
	if !b.canAccessLocked(entry, request.Owner) {
		return SecretRef{}, ErrSecretAccessDenied
	}
	cipherData, nonce, err := b.encryptLocked(entry.ref.Algorithm, request.Plaintext)
	if err != nil {
		return SecretRef{}, err
	}
	now := time.Now().UTC()
	newVersion := entry.ref.Version + 1
	entry.versions = append(entry.versions, secretVersion{
		version:   newVersion,
		cipher:    cipherData,
		nonce:     nonce,
		createdAt: now,
	})
	entry.ref.Version = newVersion
	entry.ref.RotatedAt = &now
	b.recordEventLocked("rotate", entry.ref, request.Owner)
	return entry.ref, nil
}

func (b *DefaultSecretBroker) Revoke(_ context.Context, owner ResourceOwner, name string) error {
	key := secretKey(owner, name)
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.secrets[key]
	if !ok {
		return ErrSecretNotFound
	}
	if !b.canAccessLocked(entry, owner) {
		return ErrSecretAccessDenied
	}
	entry.ref.Revoked = true
	for i := range entry.versions {
		entry.versions[i].revoked = true
	}
	b.recordEventLocked("revoke", entry.ref, owner)
	return nil
}

func (b *DefaultSecretBroker) Share(_ context.Context, request SecretShareRequest) (SecretRef, error) {
	key := secretKey(request.Owner, request.Name)
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.secrets[key]
	if !ok {
		return SecretRef{}, ErrSecretNotFound
	}
	if !b.canAccessLocked(entry, request.Owner) {
		return SecretRef{}, ErrSecretAccessDenied
	}
	if request.ExpiresIn > 0 {
		entry.shared[request.TargetOwner.String()] = time.Now().UTC().Add(request.ExpiresIn)
	} else {
		entry.shared[request.TargetOwner.String()] = time.Time{}
	}
	b.recordEventLocked("share", entry.ref, request.Owner)
	return entry.ref, nil
}

func (b *DefaultSecretBroker) List(_ context.Context, owner ResourceOwner) ([]SecretRef, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var refs []SecretRef
	prefix := owner.String() + "/"
	for k, entry := range b.secrets {
		if !startsWithSecretOwner(k, owner.String()) && !b.canAccessLocked(entry, owner) {
			continue
		}
		_ = prefix
		refs = append(refs, entry.ref)
	}
	return refs, nil
}

func (b *DefaultSecretBroker) Delete(_ context.Context, owner ResourceOwner, name string) error {
	key := secretKey(owner, name)
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.secrets[key]
	if !ok {
		return ErrSecretNotFound
	}
	if !b.canAccessLocked(entry, owner) {
		return ErrSecretAccessDenied
	}
	b.recordEventLocked("delete", entry.ref, owner)
	delete(b.secrets, key)
	return nil
}

func (b *DefaultSecretBroker) encryptLocked(algo string, plaintext []byte) ([]byte, []byte, error) {
	if algo == AlgorithmPlain {
		return plaintext, nil, nil
	}
	block, err := aes.NewCipher(b.masterKey)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	cipherData := gcm.Seal(nil, nonce, plaintext, nil)
	return cipherData, nonce, nil
}

func (b *DefaultSecretBroker) decryptLocked(algo string, cipherData, nonce []byte) ([]byte, error) {
	if algo == AlgorithmPlain {
		return cipherData, nil
	}
	block, err := aes.NewCipher(b.masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("secret: invalid nonce size")
	}
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return nil, fmt.Errorf("secret: decrypt failed: %w", err)
	}
	return plaintext, nil
}

func (b *DefaultSecretBroker) canAccessLocked(entry *secretEntry, accessor ResourceOwner) bool {
	if entry.ref.Owner.String() == accessor.String() {
		return true
	}
	if exp, ok := entry.shared[accessor.String()]; ok {
		if exp.IsZero() {
			return true
		}
		return time.Now().UTC().Before(exp)
	}
	return false
}

func (b *DefaultSecretBroker) findKeyByRefID(refID string, owner ResourceOwner) string {
	for k, entry := range b.secrets {
		if entry.ref.RefID == refID && b.canAccessLocked(entry, owner) {
			return k
		}
	}
	return ""
}

func (b *DefaultSecretBroker) recordEventLocked(event string, ref SecretRef, owner ResourceOwner) {
	if b.auditWriter != nil {
		b.auditWriter.RecordSecretEvent(context.Background(), event, ref, owner)
	}
}

func secretKey(owner ResourceOwner, name string) string {
	return fmt.Sprintf("%s/%s", owner.String(), name)
}

func startsWithSecretOwner(key, owner string) bool {
	return key == owner+"/" || len(key) > len(owner)+1 && key[:len(owner)+1] == owner+"/"
}

func newSecretRefID(owner ResourceOwner, name string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s:%d", owner.String(), name, time.Now().UnixNano())))
}

func expiryFromTTL(now time.Time, ttl time.Duration) *time.Time {
	if ttl <= 0 {
		return nil
	}
	exp := now.Add(ttl)
	return &exp
}

var _ SecretBroker = (*DefaultSecretBroker)(nil)
