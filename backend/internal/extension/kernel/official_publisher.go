package kernel

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

const defaultOfficialPublisherPublicKey = trust.DefaultOfficialPublisherPublicKey

func ensureOfficialPublisherTrust(ctx context.Context, service *trust.TrustService) error {
	if service == nil {
		return errors.New("kernel: trust service unavailable")
	}
	store := service.Store()
	if _, err := store.Get(ctx, trust.OfficialPublisherID); err == nil {
		return nil
	}
	raw := strings.TrimSpace(os.Getenv("AMITIA_OFFICIAL_PLUGIN_PUBLIC_KEY"))
	if raw == "" {
		raw = defaultOfficialPublisherPublicKey
	}
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "0x"), "0X")
	publicKey, err := hex.DecodeString(raw)
	if err != nil {
		return err
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return errors.New("official publisher public key must be a 32-byte Ed25519 key")
	}
	keyID := trust.ComputeKeyID(ed25519.PublicKey(publicKey))
	now := time.Now().UTC()
	identity := trust.PublisherIdentity{
		PublisherID: trust.OfficialPublisherID,
		DisplayName: "Amitia",
		Keys: []trust.PublisherKey{{
			KeyID: keyID, PublisherID: trust.OfficialPublisherID, PublicKey: publicKey,
			Algorithm: trust.AlgorithmEd25519, State: trust.KeyStateActive, CreatedAt: now,
		}},
		TrustLevel:  trust.TrustLevelOfficial,
		Source:      trust.TrustSourceBuiltin,
		FirstSeenAt: now,
		UpdatedAt:   now,
	}
	return store.RegisterBuiltinRoot(trust.OfficialPublisherID, identity)
}
