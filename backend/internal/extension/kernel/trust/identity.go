package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type TrustLevel string

const (
	TrustLevelOfficial    TrustLevel = "official"
	TrustLevelTrusted     TrustLevel = "trusted"
	TrustLevelUserTrusted TrustLevel = "user_trusted"
	TrustLevelUnknown     TrustLevel = "unknown"
	TrustLevelBlocked     TrustLevel = "blocked"
	TrustLevelRevoked     TrustLevel = "revoked"
	TrustLevelDevelopment TrustLevel = "development"
)

func (l TrustLevel) IsValid() bool {
	switch l {
	case TrustLevelOfficial, TrustLevelTrusted, TrustLevelUserTrusted,
		TrustLevelUnknown, TrustLevelBlocked, TrustLevelRevoked, TrustLevelDevelopment:
		return true
	}
	return false
}

func (l TrustLevel) AllowsInstallation() bool {
	switch l {
	case TrustLevelOfficial, TrustLevelTrusted, TrustLevelUserTrusted,
		TrustLevelUnknown, TrustLevelDevelopment:
		return true
	}
	return false
}

func (l TrustLevel) AllowsAutoUpdate() bool {
	switch l {
	case TrustLevelOfficial, TrustLevelTrusted, TrustLevelUserTrusted:
		return true
	}
	return false
}

func (l TrustLevel) AllowsHighRiskRuntime() bool {
	switch l {
	case TrustLevelOfficial, TrustLevelTrusted:
		return true
	}
	return false
}

func (l TrustLevel) IsBlocked() bool {
	return l == TrustLevelBlocked || l == TrustLevelRevoked
}

type KeyState string

const (
	KeyStateActive      KeyState = "active"
	KeyStateRotated     KeyState = "rotated"
	KeyStateExpired     KeyState = "expired"
	KeyStateRevoked     KeyState = "revoked"
	KeyStateCompromised KeyState = "compromised"
	KeyStateUnknown     KeyState = "unknown"
)

func (s KeyState) IsValid() bool {
	switch s {
	case KeyStateActive, KeyStateRotated, KeyStateExpired,
		KeyStateRevoked, KeyStateCompromised, KeyStateUnknown:
		return true
	}
	return false
}

func (s KeyState) IsUsable() bool {
	return s == KeyStateActive
}

type KeyAlgorithm string

const (
	AlgorithmEd25519 KeyAlgorithm = "ed25519"
)

type PublisherKey struct {
	KeyID         string       `json:"key_id"`
	PublisherID   string       `json:"publisher_id"`
	PublicKey     []byte       `json:"public_key"`
	Algorithm     KeyAlgorithm `json:"algorithm"`
	State         KeyState     `json:"state"`
	CreatedAt     time.Time    `json:"created_at"`
	ExpiresAt     *time.Time   `json:"expires_at,omitempty"`
	RotatedAt     *time.Time   `json:"rotated_at,omitempty"`
	RotatedFrom   string       `json:"rotated_from,omitempty"`
	RevokedAt     *time.Time   `json:"revoked_at,omitempty"`
	RevokedReason string       `json:"revoked_reason,omitempty"`
	ContinuitySignedBy string  `json:"continuity_signed_by,omitempty"`
}

func (k *PublisherKey) Fingerprint() string {
	if len(k.PublicKey) == 0 {
		return ""
	}
	h := sha256.Sum256(k.PublicKey)
	return "sha256:" + hex.EncodeToString(h[:])
}

func (k *PublisherKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*k.ExpiresAt)
}

func (k *PublisherKey) IsRevoked() bool {
	return k.State == KeyStateRevoked || k.State == KeyStateCompromised || k.RevokedAt != nil
}

func (k *PublisherKey) IsUsable() bool {
	if k.IsExpired() || k.IsRevoked() {
		return false
	}
	return k.State.IsUsable()
}

func (k *PublisherKey) Validate() error {
	if k.KeyID == "" {
		return errors.New("trust: key id required")
	}
	if k.PublisherID == "" {
		return errors.New("trust: publisher id required")
	}
	if len(k.PublicKey) == 0 {
		return errors.New("trust: public key required")
	}
	if k.Algorithm == "" {
		return errors.New("trust: algorithm required")
	}
	if !k.Algorithm.IsValid() {
		return fmt.Errorf("trust: unsupported algorithm %s", k.Algorithm)
	}
	if !k.State.IsValid() {
		return fmt.Errorf("trust: invalid key state %s", k.State)
	}
	return nil
}

func (a KeyAlgorithm) IsValid() bool {
	return a == AlgorithmEd25519
}

type PublisherIdentity struct {
	PublisherID    string         `json:"publisher_id"`
	DisplayName    string         `json:"display_name"`
	Contact        string         `json:"contact,omitempty"`
	Keys           []PublisherKey `json:"keys"`
	TrustLevel     TrustLevel     `json:"trust_level"`
	Source         TrustSource    `json:"source"`
	FirstSeenAt    time.Time      `json:"first_seen_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	OfficialRoot   bool           `json:"official_root"`
}

type TrustSource string

const (
	TrustSourceBuiltin      TrustSource = "builtin"
	TrustSourceOfficialFeed TrustSource = "official_feed"
	TrustSourceUserDecision TrustSource = "user_decision"
	TrustSourceDevelopment  TrustSource = "development"
	TrustSourceCached       TrustSource = "cached"
)

func (s TrustSource) IsValid() bool {
	switch s {
	case TrustSourceBuiltin, TrustSourceOfficialFeed, TrustSourceUserDecision,
		TrustSourceDevelopment, TrustSourceCached:
		return true
	}
	return false
}

func (p *PublisherIdentity) ActiveKey() *PublisherKey {
	for i := range p.Keys {
		if p.Keys[i].IsUsable() {
			return &p.Keys[i]
		}
	}
	return nil
}

func (p *PublisherIdentity) FindKey(keyID string) *PublisherKey {
	for i := range p.Keys {
		if p.Keys[i].KeyID == keyID {
			return &p.Keys[i]
		}
	}
	return nil
}

func (p *PublisherIdentity) AddKey(key PublisherKey) error {
	if err := key.Validate(); err != nil {
		return err
	}
	if p.FindKey(key.KeyID) != nil {
		return fmt.Errorf("trust: key %s already exists", key.KeyID)
	}
	if key.PublisherID != p.PublisherID {
		return errors.New("trust: key publisher mismatch")
	}
	p.Keys = append(p.Keys, key)
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *PublisherIdentity) RevokeKey(keyID string, reason string) error {
	for i := range p.Keys {
		if p.Keys[i].KeyID == keyID {
			now := time.Now().UTC()
			p.Keys[i].State = KeyStateRevoked
			p.Keys[i].RevokedAt = &now
			p.Keys[i].RevokedReason = reason
			p.UpdatedAt = now
			return nil
		}
	}
	return fmt.Errorf("trust: key %s not found", keyID)
}

func (p *PublisherIdentity) RotateKey(oldKeyID, newKeyID string, newPublicKey []byte) error {
	oldKey := p.FindKey(oldKeyID)
	if oldKey == nil {
		return fmt.Errorf("trust: old key %s not found", oldKeyID)
	}
	if !oldKey.IsUsable() {
		return fmt.Errorf("trust: old key %s not usable", oldKeyID)
	}
	if p.FindKey(newKeyID) != nil {
		return fmt.Errorf("trust: new key %s already exists", newKeyID)
	}
	now := time.Now().UTC()
	oldKey.State = KeyStateRotated
	oldKey.RotatedAt = &now
	newKey := PublisherKey{
		KeyID:         newKeyID,
		PublisherID:   p.PublisherID,
		PublicKey:     newPublicKey,
		Algorithm:     oldKey.Algorithm,
		State:         KeyStateActive,
		CreatedAt:     now,
		RotatedFrom:   oldKeyID,
		ContinuitySignedBy: oldKeyID,
	}
	p.Keys = append(p.Keys, newKey)
	p.UpdatedAt = now
	return nil
}

var (
	ErrPublisherNotFound   = errors.New("trust: publisher not found")
	ErrKeyNotFound         = errors.New("trust: key not found")
	ErrKeyAlreadyExists    = errors.New("trust: key already exists")
	ErrInvalidIdentity     = errors.New("trust: invalid identity")
	ErrInvalidKey          = errors.New("trust: invalid key")
	ErrKeyNotUsable        = errors.New("trust: key not usable")
	ErrContinuityBroken    = errors.New("trust: key continuity broken")
	ErrTrustBlocked        = errors.New("trust: trust blocked")
	ErrRevoked             = errors.New("trust: revoked")
	ErrPackageBlocked      = errors.New("trust: package blocked")
	ErrOwnershipTransfer   = errors.New("trust: ownership transfer required")
	ErrDevelopmentScopeOnly = errors.New("trust: development scope only")
)
