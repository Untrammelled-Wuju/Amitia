package trust

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestRevocationListAddAndCheck(t *testing.T) {
	list := NewRevocationList("local")
	entry := RevocationEntry{
		EntryID:     "rev-1",
		PublisherID: "com.example",
		KeyID:       "k1",
		Source:      RevocationSourceUser,
		Severity:    RevocationSeverityHigh,
		Reason:      "key compromised",
		RevokedAt:   time.Now().UTC(),
	}
	if err := list.Add(entry); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := list.CheckKey("com.example", "k1"); got == nil {
		t.Fatal("expected revocation for key")
	}
	if got := list.CheckKey("com.example", "k2"); got != nil {
		t.Fatal("expected no revocation for k2")
	}
}

func TestRevocationListExpired(t *testing.T) {
	list := NewRevocationList("local")
	past := time.Now().Add(-1 * time.Hour)
	entry := RevocationEntry{
		EntryID:     "rev-1",
		PublisherID: "com.example",
		KeyID:       "k1",
		Source:      RevocationSourceUser,
		Severity:    RevocationSeverityHigh,
		Reason:      "key compromised",
		RevokedAt:   past.Add(-1 * time.Hour),
		ExpiresAt:   &past,
	}
	list.Add(entry)
	if got := list.CheckKey("com.example", "k1"); got != nil {
		t.Fatal("expected no active revocation after expiry")
	}
}

func TestRevocationListSupersede(t *testing.T) {
	list := NewRevocationList("local")
	entry1 := RevocationEntry{
		EntryID:     "rev-1",
		PublisherID: "com.example",
		KeyID:       "k1",
		Source:      RevocationSourceUser,
		Severity:    RevocationSeverityLow,
		Reason:      "low severity issue",
		RevokedAt:   time.Now().UTC(),
	}
	list.Add(entry1)
	if err := list.Supersede("rev-1", "rev-2"); err != nil {
		t.Fatalf("supersede: %v", err)
	}
	if got := list.CheckKey("com.example", "k1"); got != nil {
		t.Fatal("expected no active revocation after supersede")
	}
}

func TestRevocationListCheckPackage(t *testing.T) {
	list := NewRevocationList("local")
	entry := RevocationEntry{
		EntryID:     "rev-1",
		PackageHash: "sha256:bad",
		Source:      RevocationSourceOfficial,
		Severity:    RevocationSeverityCritical,
		Reason:      "malware",
		RevokedAt:   time.Now().UTC(),
	}
	list.Add(entry)
	if got := list.CheckPackage("sha256:bad"); got == nil {
		t.Fatal("expected package revocation")
	}
	if got := list.CheckPackage("sha256:good"); got != nil {
		t.Fatal("expected no revocation for good package")
	}
}

func TestRevocationListMerge(t *testing.T) {
	local := NewRevocationList("local")
	remote := NewRevocationList("official")
	remote.Add(RevocationEntry{
		EntryID:     "rev-1",
		PackageHash: "sha256:bad",
		Source:      RevocationSourceOfficial,
		Severity:    RevocationSeverityCritical,
		Reason:      "malware",
		RevokedAt:   time.Now().UTC(),
	})
	added := local.Merge(context.Background(), remote)
	if added != 1 {
		t.Fatalf("expected 1 added, got %d", added)
	}
	if local.CheckPackage("sha256:bad") == nil {
		t.Fatal("expected merged revocation")
	}
	added2 := local.Merge(context.Background(), remote)
	if added2 != 0 {
		t.Fatalf("expected 0 added on second merge, got %d", added2)
	}
}

func TestBlocklistBlockAndCheck(t *testing.T) {
	list := NewPackageBlocklist()
	entry := PackageBlockEntry{
		PackageHash: "sha256:bad",
		ExtensionID: "com.example/weather",
		Reason:      BlockReasonMalware,
		Details:     "trojan detected",
		BlockedAt:   time.Now().UTC(),
	}
	if err := list.Block(entry); err != nil {
		t.Fatalf("block: %v", err)
	}
	if got := list.Check("sha256:bad"); got == nil {
		t.Fatal("expected blocked entry")
	}
	if got := list.Check("sha256:good"); got != nil {
		t.Fatal("expected no block")
	}
}

func TestBlocklistUnblock(t *testing.T) {
	list := NewPackageBlocklist()
	entry := PackageBlockEntry{
		PackageHash: "sha256:bad",
		Reason:      BlockReasonMalware,
		BlockedAt:   time.Now().UTC(),
	}
	list.Block(entry)
	if err := list.Unblock("sha256:bad"); err != nil {
		t.Fatalf("unblock: %v", err)
	}
	if got := list.Check("sha256:bad"); got != nil {
		t.Fatal("expected no block after unblock")
	}
}

func TestBlocklistExpiredEntry(t *testing.T) {
	list := NewPackageBlocklist()
	past := time.Now().Add(-1 * time.Hour)
	entry := PackageBlockEntry{
		PackageHash: "sha256:bad",
		Reason:      BlockReasonPolicy,
		BlockedAt:   past.Add(-1 * time.Hour),
		ExpiresAt:   &past,
	}
	list.Block(entry)
	if got := list.Check("sha256:bad"); got != nil {
		t.Fatal("expected no active block for expired entry")
	}
}

func TestUserTrustGrantAndLookup(t *testing.T) {
	store := NewUserTrustStore()
	now := time.Now().UTC()
	decision := UserTrustDecision{
		DecisionID:   "d1",
		UserID:       "user-1",
		PublisherID:  "com.example",
		Scope:        TrustScopePublisher,
		GrantedLevel: TrustLevelUserTrusted,
		GrantedAt:    now,
	}
	if err := store.Grant(decision); err != nil {
		t.Fatalf("grant: %v", err)
	}
	got := store.Lookup(TrustScopePublisher, "com.example", "", "", "", "")
	if got == nil {
		t.Fatal("expected to find user trust decision")
	}
	if got.GrantedLevel != TrustLevelUserTrusted {
		t.Fatalf("expected user_trusted, got %s", got.GrantedLevel)
	}
}

func TestUserTrustRevoke(t *testing.T) {
	store := NewUserTrustStore()
	now := time.Now().UTC()
	decision := UserTrustDecision{
		DecisionID:   "d1",
		UserID:       "user-1",
		PublisherID:  "com.example",
		Scope:        TrustScopePublisher,
		GrantedLevel: TrustLevelUserTrusted,
		GrantedAt:    now,
	}
	store.Grant(decision)
	if err := store.Revoke("d1", "no longer trusted"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	got := store.Lookup(TrustScopePublisher, "com.example", "", "", "", "")
	if got != nil {
		t.Fatal("expected no active decision after revoke")
	}
}

func TestUserTrustBlockedScope(t *testing.T) {
	store := NewUserTrustStore()
	now := time.Now().UTC()
	store.Grant(UserTrustDecision{
		DecisionID:   "d1",
		UserID:       "user-1",
		PublisherID:  "com.example",
		Scope:        TrustScopePublisher,
		GrantedLevel: TrustLevelBlocked,
		GrantedAt:    now,
	})
	got := store.Lookup(TrustScopePublisher, "com.example", "", "", "", "")
	if got == nil {
		t.Fatal("expected to find blocked decision")
	}
	if got.GrantedLevel != TrustLevelBlocked {
		t.Fatalf("expected blocked, got %s", got.GrantedLevel)
	}
}

func TestKeyRotatorWithContinuity(t *testing.T) {
	store := NewPublisherStore()
	pub1, priv1, _ := ed25519.GenerateKey(nil)
	pub2, _, _ := ed25519.GenerateKey(nil)
	identity := PublisherIdentity{
		PublisherID: "com.example",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k-old",
				PublisherID: "com.example",
				PublicKey:   pub1,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
				CreatedAt:   time.Now().UTC(),
			},
		},
	}
	store.RegisterUserDecision(identity)

	continuitySig := ed25519.Sign(priv1, pub2)
	rotator := NewKeyRotator(store)
	result := rotator.Rotate(context.Background(), RotationRequest{
		PublisherID:         "com.example",
		OldKeyID:            "k-old",
		NewKeyID:            "k-new",
		NewPublicKey:        pub2,
		ContinuitySignature: continuitySig,
		Reason:              "routine rotation",
	})
	if !result.Success {
		t.Fatalf("expected success, got %s", result.Reason)
	}
	if result.NewKey.KeyID != "k-new" {
		t.Fatalf("expected new key k-new, got %s", result.NewKey.KeyID)
	}
}

func TestKeyRotatorRequiresContinuityForTrusted(t *testing.T) {
	store := NewPublisherStore()
	pub1, _, _ := ed25519.GenerateKey(nil)
	pub2, _, _ := ed25519.GenerateKey(nil)
	identity := PublisherIdentity{
		PublisherID: "com.example",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k-old",
				PublisherID: "com.example",
				PublicKey:   pub1,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	}
	store.RegisterUserDecision(identity)

	rotator := NewKeyRotator(store)
	result := rotator.Rotate(context.Background(), RotationRequest{
		PublisherID:  "com.example",
		OldKeyID:     "k-old",
		NewKeyID:     "k-new",
		NewPublicKey: pub2,
	})
	if result.Success {
		t.Fatal("expected failure due to missing continuity signature")
	}
}

func TestKeyRotatorRejectsBrokenContinuity(t *testing.T) {
	store := NewPublisherStore()
	pub1, _, _ := ed25519.GenerateKey(nil)
	pub2, _, _ := ed25519.GenerateKey(nil)
	_, priv3, _ := ed25519.GenerateKey(nil)
	identity := PublisherIdentity{
		PublisherID: "com.example",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k-old",
				PublisherID: "com.example",
				PublicKey:   pub1,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	}
	store.RegisterUserDecision(identity)

	badSig := ed25519.Sign(priv3, pub2)
	rotator := NewKeyRotator(store)
	result := rotator.Rotate(context.Background(), RotationRequest{
		PublisherID:         "com.example",
		OldKeyID:            "k-old",
		NewKeyID:            "k-new",
		NewPublicKey:        pub2,
		ContinuitySignature: badSig,
	})
	if result.Success {
		t.Fatal("expected failure due to broken continuity")
	}
}
