package package_security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type PublisherTrustLevel string

const (
	TrustOfficial    PublisherTrustLevel = "official"
	TrustTrusted     PublisherTrustLevel = "trusted"
	TrustUserTrusted PublisherTrustLevel = "user_trusted"
	TrustUnknown     PublisherTrustLevel = "unknown"
	TrustBlocked     PublisherTrustLevel = "blocked"
	TrustRevoked     PublisherTrustLevel = "revoked"
	TrustDevelopment PublisherTrustLevel = "development"
)

func (l PublisherTrustLevel) IsValid() bool {
	switch l {
	case TrustOfficial, TrustTrusted, TrustUserTrusted, TrustUnknown,
		TrustBlocked, TrustRevoked, TrustDevelopment:
		return true
	}
	return false
}

type PublisherKey struct {
	KeyID       string     `json:"key_id"`
	PublisherID string     `json:"publisher_id"`
	PublicKey   []byte     `json:"public_key"`
	Algorithm   string     `json:"algorithm"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	Revoked     bool       `json:"revoked"`
}

func (k *PublisherKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

func (k *PublisherKey) IsRevoked() bool {
	return k.Revoked
}

func (k *PublisherKey) Fingerprint() string {
	h := sha256.Sum256(k.PublicKey)
	return "sha256:" + hex.EncodeToString(h[:])
}

type PublisherTrustService struct {
	keys          map[string]*PublisherKey
	publisherKeys map[string][]string
	trustLevels   map[string]PublisherTrustLevel
}

func NewPublisherTrustService() *PublisherTrustService {
	return &PublisherTrustService{
		keys:          make(map[string]*PublisherKey),
		publisherKeys: make(map[string][]string),
		trustLevels:   make(map[string]PublisherTrustLevel),
	}
}

func (s *PublisherTrustService) RegisterKey(pub *PublisherKey) {
	s.keys[pub.KeyID] = pub
	s.publisherKeys[pub.PublisherID] = append(s.publisherKeys[pub.PublisherID], pub.KeyID)
	if _, exists := s.trustLevels[pub.PublisherID]; !exists {
		s.trustLevels[pub.PublisherID] = TrustUnknown
	}
}

func (s *PublisherTrustService) GetKey(ctx context.Context, keyID string) (*PublisherKey, error) {
	key, ok := s.keys[keyID]
	if !ok {
		return nil, ErrUnknownPublisher
	}
	return key, nil
}

func (s *PublisherTrustService) Evaluate(ctx context.Context, publisherID string, keyID string, sigResult SignatureVerificationResult) PublisherTrustResult {
	result := PublisherTrustResult{
		PublisherID: publisherID,
		KeyID:       keyID,
		Level:       TrustUnknown,
	}

	if level, ok := s.trustLevels[publisherID]; ok {
		result.Level = level
	}

	if key, ok := s.keys[keyID]; ok {
		if key.IsRevoked() {
			result.Level = TrustRevoked
			result.Blocked = true
			result.Reason = "key revoked"
			return result
		}
		if key.IsExpired() {
			result.Level = TrustUnknown
			result.Warnings = append(result.Warnings, "key expired")
		}
	}

	if result.Level == TrustBlocked || result.Level == TrustRevoked {
		result.Blocked = true
	}

	return result
}

func (s *PublisherTrustService) Trust(ctx context.Context, publisherID string, level PublisherTrustLevel) error {
	if !level.IsValid() {
		return ErrInvalidTrustLevel
	}
	s.trustLevels[publisherID] = level
	return nil
}

func (s *PublisherTrustService) RevokeTrust(ctx context.Context, publisherID string, keyID string) error {
	if key, ok := s.keys[keyID]; ok {
		key.Revoked = true
		now := time.Now()
		key.RevokedAt = &now
	}
	s.trustLevels[publisherID] = TrustRevoked
	return nil
}

type PublisherTrustResult struct {
	PublisherID string
	KeyID       string
	Level       PublisherTrustLevel
	Blocked     bool
	Reason      string
	Warnings    []string
}

type PublisherTrustRequest struct {
	PublisherID string
	Level       PublisherTrustLevel
}
