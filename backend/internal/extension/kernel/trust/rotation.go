package trust

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"
)

type RotationRequest struct {
	PublisherID         string
	OldKeyID            string
	NewKeyID            string
	NewPublicKey        []byte
	ContinuitySignature []byte
	Reason              string
}

type RotationResult struct {
	Success  bool
	OldKey   PublisherKey
	NewKey   PublisherKey
	Warnings []string
	Reason   string
}

type KeyRotator struct {
	store *PublisherStore
}

func NewKeyRotator(store *PublisherStore) *KeyRotator {
	return &KeyRotator{store: store}
}

func (r *KeyRotator) Rotate(ctx context.Context, req RotationRequest) RotationResult {
	result := RotationResult{Reason: req.Reason}
	if req.PublisherID == "" || req.OldKeyID == "" || req.NewKeyID == "" {
		result.Reason = "missing required fields"
		return result
	}
	if len(req.NewPublicKey) != ed25519.PublicKeySize {
		result.Reason = "invalid new public key size"
		return result
	}

	identity, err := r.store.Get(ctx, req.PublisherID)
	if err != nil {
		result.Reason = fmt.Sprintf("publisher not found: %v", err)
		return result
	}

	oldKey := identity.FindKey(req.OldKeyID)
	if oldKey == nil {
		result.Reason = fmt.Sprintf("old key %s not found", req.OldKeyID)
		return result
	}
	result.OldKey = *oldKey

	if !oldKey.IsUsable() {
		result.Reason = fmt.Sprintf("old key not usable (state=%s)", oldKey.State)
		return result
	}

	if identity.FindKey(req.NewKeyID) != nil {
		result.Reason = fmt.Sprintf("new key %s already exists", req.NewKeyID)
		return result
	}

	if len(req.ContinuitySignature) > 0 {
		if err := verifyContinuitySignature(oldKey, req.NewPublicKey, req.ContinuitySignature); err != nil {
			result.Reason = fmt.Sprintf("continuity verification failed: %v", err)
			return result
		}
	} else {
		if identity.OfficialRoot || identity.TrustLevel == TrustLevelTrusted {
			result.Reason = "continuity signature required for official/trusted publishers"
			return result
		}
		result.Warnings = append(result.Warnings, "continuity signature not provided; user confirmation required")
	}

	if err := r.store.RotateKey(ctx, req.PublisherID, req.OldKeyID, req.NewKeyID, req.NewPublicKey); err != nil {
		result.Reason = fmt.Sprintf("rotate failed: %v", err)
		return result
	}

	updated, _ := r.store.Get(ctx, req.PublisherID)
	if updated != nil {
		if newKey := updated.FindKey(req.NewKeyID); newKey != nil {
			result.NewKey = *newKey
		}
	}

	result.Success = true
	return result
}

func verifyContinuitySignature(oldKey *PublisherKey, newPublicKey []byte, signature []byte) error {
	if oldKey == nil || len(oldKey.PublicKey) != ed25519.PublicKeySize {
		return errors.New("trust: invalid old key")
	}
	if len(newPublicKey) != ed25519.PublicKeySize {
		return errors.New("trust: invalid new public key")
	}
	if !ed25519.Verify(ed25519.PublicKey(oldKey.PublicKey), newPublicKey, signature) {
		return ErrContinuityBroken
	}
	return nil
}

type RotationRecord struct {
	PublisherID        string    `json:"publisher_id"`
	OldKeyID           string    `json:"old_key_id"`
	NewKeyID           string    `json:"new_key_id"`
	RotatedAt          time.Time `json:"rotated_at"`
	Reason             string    `json:"reason,omitempty"`
	ContinuityVerified bool      `json:"continuity_verified"`
}

type RotationLog struct {
	records []RotationRecord
}

func NewRotationLog() *RotationLog {
	return &RotationLog{}
}

func (l *RotationLog) Append(record RotationRecord) {
	if record.RotatedAt.IsZero() {
		record.RotatedAt = time.Now().UTC()
	}
	l.records = append(l.records, record)
}

func (l *RotationLog) List(publisherID string) []RotationRecord {
	if publisherID == "" {
		return l.records
	}
	var out []RotationRecord
	for _, r := range l.records {
		if r.PublisherID == publisherID {
			out = append(out, r)
		}
	}
	return out
}

func (l *RotationLog) LastRotation(publisherID string) *RotationRecord {
	for i := len(l.records) - 1; i >= 0; i-- {
		if l.records[i].PublisherID == publisherID {
			return &l.records[i]
		}
	}
	return nil
}
