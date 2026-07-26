package trust

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

func newTestPublisherStore(t *testing.T) *PublisherStore {
	t.Helper()
	return NewPublisherStore()
}

func generateTestKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func TestPublisherKeyFingerprint(t *testing.T) {
	pub, _ := generateTestKey(t)
	key := PublisherKey{
		KeyID:       "k1",
		PublisherID: "com.example",
		PublicKey:   pub,
		Algorithm:   AlgorithmEd25519,
		State:       KeyStateActive,
	}
	fp := key.Fingerprint()
	if fp == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	if len(fp) < 10 {
		t.Fatalf("fingerprint too short: %s", fp)
	}
}

func TestPublisherKeyIsExpired(t *testing.T) {
	pub, _ := generateTestKey(t)
	pastTime := time.Now().Add(-1 * time.Hour)
	key := PublisherKey{
		KeyID:       "k1",
		PublisherID: "com.example",
		PublicKey:   pub,
		Algorithm:   AlgorithmEd25519,
		State:       KeyStateActive,
		ExpiresAt:   &pastTime,
	}
	if !key.IsExpired() {
		t.Fatal("expected key to be expired")
	}
	if key.IsUsable() {
		t.Fatal("expired key should not be usable")
	}
}

func TestPublisherIdentityAddAndRevokeKey(t *testing.T) {
	pub, _ := generateTestKey(t)
	identity := &PublisherIdentity{
		PublisherID: "com.example",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
	}
	key := PublisherKey{
		KeyID:       "k1",
		PublisherID: "com.example",
		PublicKey:   pub,
		Algorithm:   AlgorithmEd25519,
		State:       KeyStateActive,
	}
	if err := identity.AddKey(key); err != nil {
		t.Fatalf("add key: %v", err)
	}
	if err := identity.AddKey(key); err == nil {
		t.Fatal("expected duplicate key error")
	}
	if identity.ActiveKey() == nil {
		t.Fatal("expected active key")
	}
	if err := identity.RevokeKey("k1", "compromised"); err != nil {
		t.Fatalf("revoke key: %v", err)
	}
	if identity.ActiveKey() != nil {
		t.Fatal("expected no active key after revoke")
	}
	if !identity.FindKey("k1").IsRevoked() {
		t.Fatal("expected revoked key")
	}
}

func TestPublisherIdentityRotateKey(t *testing.T) {
	pub1, _ := generateTestKey(t)
	pub2, _ := generateTestKey(t)
	identity := &PublisherIdentity{
		PublisherID: "com.example",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
	}
	oldKey := PublisherKey{
		KeyID:       "k-old",
		PublisherID: "com.example",
		PublicKey:   pub1,
		Algorithm:   AlgorithmEd25519,
		State:       KeyStateActive,
	}
	identity.AddKey(oldKey)
	if err := identity.RotateKey("k-old", "k-new", pub2); err != nil {
		t.Fatalf("rotate key: %v", err)
	}
	if identity.FindKey("k-old").State != KeyStateRotated {
		t.Fatal("expected old key state rotated")
	}
	newKey := identity.FindKey("k-new")
	if newKey == nil {
		t.Fatal("expected new key")
	}
	if newKey.State != KeyStateActive {
		t.Fatalf("expected new key active, got %s", newKey.State)
	}
	if newKey.RotatedFrom != "k-old" {
		t.Fatalf("expected rotated from k-old, got %s", newKey.RotatedFrom)
	}
	if newKey.ContinuitySignedBy != "k-old" {
		t.Fatalf("expected continuity signed by k-old, got %s", newKey.ContinuitySignedBy)
	}
	if identity.ActiveKey().KeyID != "k-new" {
		t.Fatal("expected new key to be active")
	}
}

func TestPublisherStoreRegisterAndGet(t *testing.T) {
	store := newTestPublisherStore(t)
	pub, _ := generateTestKey(t)
	identity := PublisherIdentity{
		PublisherID: "com.example",
		DisplayName: "Example",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k1",
				PublisherID: "com.example",
				PublicKey:   pub,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	}
	if err := store.RegisterUserDecision(identity); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, err := store.Get(context.Background(), "com.example")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PublisherID != "com.example" {
		t.Fatalf("expected publisher id com.example, got %s", got.PublisherID)
	}
	if got.TrustLevel != TrustLevelTrusted {
		t.Fatalf("expected trusted level, got %s", got.TrustLevel)
	}
}

func TestPublisherStoreBuiltinRootCannotBeOverwritten(t *testing.T) {
	store := newTestPublisherStore(t)
	pub, _ := generateTestKey(t)
	root := PublisherIdentity{
		PublisherID: "com.amitia.official",
		DisplayName: "Amitia",
		TrustLevel:  TrustLevelOfficial,
		Source:      TrustSourceBuiltin,
		Keys: []PublisherKey{
			{
				KeyID:       "root-1",
				PublisherID: "com.amitia.official",
				PublicKey:   pub,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	}
	if err := store.RegisterBuiltinRoot("com.amitia.official", root); err != nil {
		t.Fatalf("register builtin root: %v", err)
	}
	if err := store.RegisterUserDecision(root); err == nil {
		t.Fatal("expected error overwriting builtin root")
	}
}

func TestPublisherStoreSetTrustLevel(t *testing.T) {
	store := newTestPublisherStore(t)
	pub, _ := generateTestKey(t)
	identity := PublisherIdentity{
		PublisherID: "com.example",
		TrustLevel:  TrustLevelUnknown,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k1",
				PublisherID: "com.example",
				PublicKey:   pub,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	}
	store.RegisterUserDecision(identity)
	if err := store.SetTrustLevel(context.Background(), "com.example", TrustLevelUserTrusted); err != nil {
		t.Fatalf("set trust level: %v", err)
	}
	got, _ := store.Get(context.Background(), "com.example")
	if got.TrustLevel != TrustLevelUserTrusted {
		t.Fatalf("expected user_trusted, got %s", got.TrustLevel)
	}
}

func TestPublisherStoreRevokeTrust(t *testing.T) {
	store := newTestPublisherStore(t)
	pub, _ := generateTestKey(t)
	identity := PublisherIdentity{
		PublisherID: "com.example",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k1",
				PublisherID: "com.example",
				PublicKey:   pub,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	}
	store.RegisterUserDecision(identity)
	if err := store.RevokeTrust(context.Background(), "com.example", "violation"); err != nil {
		t.Fatalf("revoke trust: %v", err)
	}
	got, _ := store.Get(context.Background(), "com.example")
	if got.TrustLevel != TrustLevelRevoked {
		t.Fatalf("expected revoked, got %s", got.TrustLevel)
	}
	if !got.Keys[0].IsRevoked() {
		t.Fatal("expected key to be revoked")
	}
}

func TestTrustLevelAllowsInstallation(t *testing.T) {
	cases := []struct {
		level     TrustLevel
		installOK bool
		updateOK  bool
		riskOK    bool
	}{
		{TrustLevelOfficial, true, true, true},
		{TrustLevelTrusted, true, true, true},
		{TrustLevelUserTrusted, true, true, false},
		{TrustLevelUnknown, true, false, false},
		{TrustLevelBlocked, false, false, false},
		{TrustLevelRevoked, false, false, false},
		{TrustLevelDevelopment, true, false, false},
	}
	for _, c := range cases {
		if c.level.AllowsInstallation() != c.installOK {
			t.Errorf("level %s install: expected %v, got %v", c.level, c.installOK, c.level.AllowsInstallation())
		}
		if c.level.AllowsAutoUpdate() != c.updateOK {
			t.Errorf("level %s update: expected %v, got %v", c.level, c.updateOK, c.level.AllowsAutoUpdate())
		}
		if c.level.AllowsHighRiskRuntime() != c.riskOK {
			t.Errorf("level %s risk: expected %v, got %v", c.level, c.riskOK, c.level.AllowsHighRiskRuntime())
		}
	}
}
