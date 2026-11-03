package kernel

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

func TestOfficialPublisherTrustUsesBundledRoot(t *testing.T) {
	service := trust.NewTrustService(trust.TrustServiceConfig{})
	t.Setenv("AMITIA_OFFICIAL_PLUGIN_PUBLIC_KEY", "")
	if err := ensureOfficialPublisherTrust(context.Background(), service); err != nil {
		t.Fatal(err)
	}
	identity, err := service.Store().Get(context.Background(), "com.amitia")
	if err != nil {
		t.Fatal(err)
	}
	if identity.TrustLevel != trust.TrustLevelOfficial || !identity.OfficialRoot || len(identity.Keys) != 1 {
		t.Fatalf("unexpected official publisher identity: %+v", identity)
	}
	publicKey, err := hex.DecodeString(defaultOfficialPublisherPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Keys[0].KeyID != trust.ComputeKeyID(ed25519.PublicKey(publicKey)) {
		t.Fatalf("unexpected official key id: %s", identity.Keys[0].KeyID)
	}
}
