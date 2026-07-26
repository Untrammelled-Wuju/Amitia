package trust

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

func setupPolicyEngine(t *testing.T) (*LifecyclePolicyEngine, *PublisherStore, *UserTrustStore, *RevocationList, *PackageBlocklist) {
	t.Helper()
	store := NewPublisherStore()
	pub, _, _ := ed25519.GenerateKey(nil)
	root := PublisherIdentity{
		PublisherID: "com.amitia.official",
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
	store.RegisterBuiltinRoot("com.amitia.official", root)
	userTrust := NewUserTrustStore()
	revList := NewRevocationList("local")
	blocklist := NewPackageBlocklist()
	engine := NewLifecyclePolicyEngine(store, userTrust, revList, blocklist)
	return engine, store, userTrust, revList, blocklist
}

func TestPolicyAllowsOfficialPublisher(t *testing.T) {
	engine, store, _, _, _ := setupPolicyEngine(t)
	pub, _, _ := ed25519.GenerateKey(nil)
	store.RegisterUserDecision(PublisherIdentity{
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
	})

	result := engine.Evaluate(context.Background(), PolicyInput{
		PublisherID: "com.example",
		KeyID:       "k1",
		PackageHash: "sha256:pkg",
		SignatureResult: SignatureVerificationResult{
			Valid:  true,
			Status: SignatureStatusValid,
		},
	})
	if result.Action != LifecycleActionAllow {
		t.Fatalf("expected allow, got %s: %s", result.Action, result.Reason)
	}
}

func TestPolicyDeniesBlockedPackage(t *testing.T) {
	engine, _, _, _, blocklist := setupPolicyEngine(t)
	blocklist.Block(PackageBlockEntry{
		PackageHash: "sha256:bad",
		Reason:      BlockReasonMalware,
		BlockedAt:   time.Now().UTC(),
	})
	result := engine.Evaluate(context.Background(), PolicyInput{
		PackageHash: "sha256:bad",
		SignatureResult: SignatureVerificationResult{
			Valid:  true,
			Status: SignatureStatusValid,
		},
	})
	if result.Action != LifecycleActionDeny {
		t.Fatalf("expected deny, got %s", result.Action)
	}
}

func TestPolicyDeniesRevokedKey(t *testing.T) {
	engine, _, _, revList, _ := setupPolicyEngine(t)
	revList.Add(RevocationEntry{
		EntryID:     "r1",
		PublisherID: "com.example",
		KeyID:       "k1",
		Source:      RevocationSourceUser,
		Severity:    RevocationSeverityHigh,
		Reason:      "compromised",
		RevokedAt:   time.Now().UTC(),
	})
	result := engine.Evaluate(context.Background(), PolicyInput{
		PublisherID: "com.example",
		KeyID:       "k1",
		SignatureResult: SignatureVerificationResult{
			Valid:  true,
			Status: SignatureStatusValid,
		},
	})
	if result.Action != LifecycleActionDeny {
		t.Fatalf("expected deny, got %s", result.Action)
	}
}

func TestPolicyUnknownPublisherRequiresConfirmation(t *testing.T) {
	engine, _, _, _, _ := setupPolicyEngine(t)
	result := engine.Evaluate(context.Background(), PolicyInput{
		PublisherID: "com.unknown",
		KeyID:       "k1",
		SignatureResult: SignatureVerificationResult{
			Valid:  false,
			Status: SignatureStatusUnknownKey,
		},
	})
	if result.Action != LifecycleActionInstallDisabled {
		t.Fatalf("expected install_disabled, got %s", result.Action)
	}
	if !result.RequiresUserInput {
		t.Fatal("expected user input required")
	}
}

func TestPolicyUnknownPublisherAllowedBySetting(t *testing.T) {
	engine, _, _, _, _ := setupPolicyEngine(t)
	result := engine.Evaluate(context.Background(), PolicyInput{
		PublisherID: "com.unknown",
		KeyID:       "k1",
		SignatureResult: SignatureVerificationResult{
			Valid:  false,
			Status: SignatureStatusUnknownKey,
		},
		UserSettings: UserPolicySettings{
			AllowUnknownPublisher: true,
		},
	})
	if result.Action != LifecycleActionAllow {
		t.Fatalf("expected allow, got %s: %s", result.Action, result.Reason)
	}
}

func TestPolicyDeniesInvalidSignature(t *testing.T) {
	engine, _, _, _, _ := setupPolicyEngine(t)
	result := engine.Evaluate(context.Background(), PolicyInput{
		PublisherID: "com.example",
		KeyID:       "k1",
		SignatureResult: SignatureVerificationResult{
			Valid:  false,
			Status: SignatureStatusInvalidSignature,
		},
	})
	if result.Action != LifecycleActionDeny {
		t.Fatalf("expected deny, got %s", result.Action)
	}
}

func TestPolicyOwnershipTransferRequiresConfirmation(t *testing.T) {
	engine, _, _, _, _ := setupPolicyEngine(t)
	result := engine.Evaluate(context.Background(), PolicyInput{
		PublisherID:         "com.new",
		KeyID:               "k1",
		IsUpdate:            true,
		PreviousPublisherID: "com.old",
		SignatureResult: SignatureVerificationResult{
			Valid:  true,
			Status: SignatureStatusValid,
		},
	})
	if result.Action != LifecycleActionAllowWithConfirm {
		t.Fatalf("expected allow_with_confirmation, got %s", result.Action)
	}
	if !result.RequiresUserInput {
		t.Fatal("expected user input required")
	}
}

func TestPolicyHighRiskRuntimeRequiresTrusted(t *testing.T) {
	engine, _, _, _, _ := setupPolicyEngine(t)
	result := engine.Evaluate(context.Background(), PolicyInput{
		PublisherID: "com.unknown",
		RuntimeTypeRisk: RuntimeRiskCritical,
		SignatureResult: SignatureVerificationResult{
			Valid:  false,
			Status: SignatureStatusUnknownKey,
		},
		UserSettings: UserPolicySettings{
			AllowUnknownPublisher: true,
		},
	})
	if result.Action != LifecycleActionDeny {
		t.Fatalf("expected deny for critical runtime from unknown publisher, got %s: %s", result.Action, result.Reason)
	}
}

func TestPolicyAutoUpdateEligibility(t *testing.T) {
	engine, store, _, _, _ := setupPolicyEngine(t)
	pub, _, _ := ed25519.GenerateKey(nil)
	store.RegisterUserDecision(PublisherIdentity{
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
	})
	eligibility := engine.EvaluateAutoUpdate(context.Background(), PolicyInput{
		PublisherID: "com.example",
		KeyID:       "k1",
		IsUpdate:    true,
		SignatureResult: SignatureVerificationResult{
			Valid:  true,
			Status: SignatureStatusValid,
		},
	})
	if !eligibility.Eligible {
		t.Fatalf("expected eligible, got %s", eligibility.Reason)
	}
}

func TestPolicyAutoUpdateDeniedForUnknownPublisher(t *testing.T) {
	engine, _, _, _, _ := setupPolicyEngine(t)
	eligibility := engine.EvaluateAutoUpdate(context.Background(), PolicyInput{
		PublisherID: "com.unknown",
		KeyID:       "k1",
		IsUpdate:    true,
		SignatureResult: SignatureVerificationResult{
			Valid:  true,
			Status: SignatureStatusValid,
		},
	})
	if eligibility.Eligible {
		t.Fatal("expected not eligible for unknown publisher")
	}
}

func TestPolicyAutoUpdateDeniedForOwnershipTransfer(t *testing.T) {
	engine, store, _, _, _ := setupPolicyEngine(t)
	pub, _, _ := ed25519.GenerateKey(nil)
	store.RegisterUserDecision(PublisherIdentity{
		PublisherID: "com.new",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k1",
				PublisherID: "com.new",
				PublicKey:   pub,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	})
	eligibility := engine.EvaluateAutoUpdate(context.Background(), PolicyInput{
		PublisherID:         "com.new",
		KeyID:               "k1",
		IsUpdate:            true,
		PreviousPublisherID: "com.old",
		SignatureResult: SignatureVerificationResult{
			Valid:  true,
			Status: SignatureStatusValid,
		},
	})
	if eligibility.Eligible {
		t.Fatal("expected not eligible for ownership transfer")
	}
}

func TestUpdateContinuityVersionRegression(t *testing.T) {
	store := NewPublisherStore()
	transferLog := NewOwnershipTransferLog()
	checker := NewUpdateContinuityChecker(store, transferLog)
	result := checker.Check(context.Background(), UpdateContinuityCheck{
		ExtensionID:         "com.example/weather",
		PreviousVersion:     "2.0.0",
		NewVersion:          "1.0.0",
		PreviousPublisherID: "com.example",
		NewPublisherID:      "com.example",
	})
	if !result.IsValid {
		t.Fatalf("expected valid (warning only), got %s", result.Reason)
	}
	if !result.IsVersionRegression {
		t.Fatal("expected version regression warning")
	}
}

func TestUpdateContinuityKeyRotation(t *testing.T) {
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
				State:       KeyStateRotated,
			},
			{
				KeyID:             "k-new",
				PublisherID:       "com.example",
				PublicKey:         pub2,
				Algorithm:         AlgorithmEd25519,
				State:             KeyStateActive,
				RotatedFrom:       "k-old",
				ContinuitySignedBy: "k-old",
			},
		},
	}
	store.RegisterUserDecision(identity)
	transferLog := NewOwnershipTransferLog()
	checker := NewUpdateContinuityChecker(store, transferLog)
	result := checker.Check(context.Background(), UpdateContinuityCheck{
		ExtensionID:         "com.example/weather",
		PreviousVersion:     "1.0.0",
		NewVersion:          "1.1.0",
		PreviousPublisherID: "com.example",
		NewPublisherID:      "com.example",
		PreviousKeyID:       "k-old",
		NewKeyID:            "k-new",
	})
	if !result.IsValid {
		t.Fatalf("expected valid, got %s", result.Reason)
	}
	if !result.IsKeyRotation {
		t.Fatal("expected key rotation")
	}
}

func TestUpdateContinuityOwnershipTransfer(t *testing.T) {
	store := NewPublisherStore()
	transferLog := NewOwnershipTransferLog()
	checker := NewUpdateContinuityChecker(store, transferLog)
	result := checker.Check(context.Background(), UpdateContinuityCheck{
		ExtensionID:         "com.example/weather",
		PreviousVersion:     "1.0.0",
		NewVersion:          "1.1.0",
		PreviousPublisherID: "com.old",
		NewPublisherID:      "com.new",
	})
	if result.IsValid {
		t.Fatal("expected invalid for unauthorized transfer")
	}
	if !result.IsOwnershipTransfer {
		t.Fatal("expected ownership transfer")
	}
}

func TestOwnershipTransferAuthorize(t *testing.T) {
	store := NewPublisherStore()
	pub1, _, _ := ed25519.GenerateKey(nil)
	pub2, _, _ := ed25519.GenerateKey(nil)
	store.RegisterUserDecision(PublisherIdentity{
		PublisherID: "com.old",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k-old",
				PublisherID: "com.old",
				PublicKey:   pub1,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	})
	store.RegisterUserDecision(PublisherIdentity{
		PublisherID: "com.new",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k-new",
				PublisherID: "com.new",
				PublicKey:   pub2,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	})
	transferLog := NewOwnershipTransferLog()
	checker := NewUpdateContinuityChecker(store, transferLog)
	result, err := checker.AuthorizeTransfer(context.Background(), OwnershipTransferRequest{
		ExtensionID:     "com.example/weather",
		OldPublisherID:  "com.old",
		NewPublisherID:  "com.new",
		AuthorizationBy: "k-old",
		AcceptanceBy:    "k-new",
		UserConfirmed:   true,
		Reason:          "acquisition",
	})
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}

	continuity := checker.Check(context.Background(), UpdateContinuityCheck{
		ExtensionID:         "com.example/weather",
		PreviousVersion:     "1.0.0",
		NewVersion:          "1.1.0",
		PreviousPublisherID: "com.old",
		NewPublisherID:      "com.new",
	})
	if !continuity.IsValid {
		t.Fatalf("expected valid after authorized transfer, got %s", continuity.Reason)
	}
}

func TestDevelopmentWorkspaceRegisterAndRevoke(t *testing.T) {
	store := NewPublisherStore()
	userTrust := NewUserTrustStore()
	mgr := NewDevelopmentTrustManager(store, userTrust)
	pub, _, _ := ed25519.GenerateKey(nil)
	err := mgr.Register(context.Background(), RegisterWorkspaceRequest{
		WorkspacePath: "/workspace/dev",
		PublisherID:   "com.dev",
		KeyID:         "k-dev",
		PublicKey:     pub,
		TTL:           24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !mgr.IsDevelopmentWorkspace("/workspace/dev") {
		t.Fatal("expected development workspace")
	}
	w, err := mgr.Get("/workspace/dev")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if w.PublisherID != "com.dev" {
		t.Fatalf("expected publisher com.dev, got %s", w.PublisherID)
	}
	if err := mgr.Revoke(context.Background(), "/workspace/dev", "no longer used"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if mgr.IsDevelopmentWorkspace("/workspace/dev") {
		t.Fatal("expected workspace revoked")
	}
}

func TestDevelopmentWorkspaceCannotPromote(t *testing.T) {
	store := NewPublisherStore()
	userTrust := NewUserTrustStore()
	mgr := NewDevelopmentTrustManager(store, userTrust)
	if err := mgr.CannotPromoteToOfficial("/workspace/dev"); err == nil {
		t.Fatal("expected error when promoting development trust")
	}
}

func TestTrustServiceVerifyAndEvaluate(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	root := PublisherIdentity{
		PublisherID: "com.amitia.official",
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
	service := NewTrustService(TrustServiceConfig{
		BuiltinRoots: []PublisherIdentity{root},
	})

	pub2, priv2, _ := ed25519.GenerateKey(nil)
	service.Store().RegisterUserDecision(PublisherIdentity{
		PublisherID: "com.example",
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       "k1",
				PublisherID: "com.example",
				PublicKey:   pub2,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
			},
		},
	})

	signer := NewSigner("com.example", "k1", priv2)
	payload := SignaturePayload{
		ExtensionID:     "com.example/weather",
		Version:         "1.0.0",
		ManifestVersion: 2,
		ManifestHash:    "sha256:abc",
		ContentTreeHash: "sha256:def",
		PackageHash:     "sha256:ghi",
		PublisherID:     "com.example",
		KeyID:           "k1",
		CreatedAt:       time.Now().UTC(),
	}
	doc, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	resp := service.VerifyAndEvaluate(context.Background(), VerifyRequest{
		Document:              doc,
		Payload:               payload,
		ActualManifestHash:    "sha256:abc",
		ActualContentTreeHash: "sha256:def",
		ActualPackageHash:     "sha256:ghi",
	}, PolicyInput{
		ExtensionID: "com.example/weather",
		Version:     "1.0.0",
	})
	if !resp.Signature.Valid {
		t.Fatalf("expected valid signature, got %s: %s", resp.Signature.Status, resp.Signature.Reason)
	}
	if resp.Trust.Action != LifecycleActionAllow {
		t.Fatalf("expected allow, got %s: %s", resp.Trust.Action, resp.Trust.Reason)
	}
}

func TestTrustServiceBlockAndVerify(t *testing.T) {
	service := NewTrustService(TrustServiceConfig{})
	err := service.BlockPackage(context.Background(), PackageBlockEntry{
		PackageHash: "sha256:bad",
		Reason:      BlockReasonMalware,
		BlockedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("block: %v", err)
	}
	if service.Blocklist().Check("sha256:bad") == nil {
		t.Fatal("expected block to be active")
	}
}

func TestTrustServiceSnapshotAndRestore(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	root := PublisherIdentity{
		PublisherID: "com.amitia.official",
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
	service := NewTrustService(TrustServiceConfig{
		BuiltinRoots: []PublisherIdentity{root},
	})
	service.UserTrust().Grant(UserTrustDecision{
		DecisionID:   "d1",
		UserID:       "user-1",
		PublisherID:  "com.example",
		Scope:        TrustScopePublisher,
		GrantedLevel: TrustLevelUserTrusted,
		GrantedAt:    time.Now().UTC(),
	})
	service.Blocklist().Block(PackageBlockEntry{
		PackageHash: "sha256:bad",
		Reason:      BlockReasonMalware,
		BlockedAt:   time.Now().UTC(),
	})

	snapshot := service.Snapshot()
	if len(snapshot.UserDecisions) != 1 {
		t.Fatalf("expected 1 user decision, got %d", len(snapshot.UserDecisions))
	}

	service2 := NewTrustService(TrustServiceConfig{})
	if err := service2.Restore(snapshot); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if service2.Blocklist().Check("sha256:bad") == nil {
		t.Fatal("expected block restored")
	}
	if service2.UserTrust().Lookup(TrustScopePublisher, "com.example", "", "", "", "") == nil {
		t.Fatal("expected user trust restored")
	}
}
