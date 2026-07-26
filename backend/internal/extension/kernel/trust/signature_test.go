package trust

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

func makeTestSigner(t *testing.T, publisherID, keyID string) (*Signer, *PublisherStore, *PublisherKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	store := NewPublisherStore()
	identity := PublisherIdentity{
		PublisherID: publisherID,
		DisplayName: publisherID,
		TrustLevel:  TrustLevelTrusted,
		Source:      TrustSourceUserDecision,
		Keys: []PublisherKey{
			{
				KeyID:       keyID,
				PublisherID: publisherID,
				PublicKey:   pub,
				Algorithm:   AlgorithmEd25519,
				State:       KeyStateActive,
				CreatedAt:   time.Now().UTC(),
			},
		},
	}
	if err := store.RegisterUserDecision(identity); err != nil {
		t.Fatalf("register: %v", err)
	}
	signer := NewSigner(publisherID, keyID, priv)
	key := identity.Keys[0]
	return signer, store, &key
}

func TestSignerAndVerifier(t *testing.T) {
	signer, store, key := makeTestSigner(t, "com.example", "k1")
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
	if doc.Format != SignatureFormatV1 {
		t.Fatalf("expected format v1, got %s", doc.Format)
	}
	if doc.Algorithm != SignatureAlgorithmEd25519 {
		t.Fatalf("expected ed25519, got %s", doc.Algorithm)
	}
	if doc.PublisherID != "com.example" {
		t.Fatalf("expected publisher com.example, got %s", doc.PublisherID)
	}

	verifier := NewSignatureVerifier(store)
	result := verifier.Verify(context.Background(), VerifyInput{
		Document:              doc,
		ActualPayload:         payload,
		ActualManifestHash:    "sha256:abc",
		ActualContentTreeHash: "sha256:def",
		ActualPackageHash:     "sha256:ghi",
	})
	if !result.Valid {
		t.Fatalf("expected valid signature, got status %s: %s", result.Status, result.Reason)
	}
	if result.KeyFingerprint != key.Fingerprint() {
		t.Fatalf("expected fingerprint %s, got %s", key.Fingerprint(), result.KeyFingerprint)
	}
}

func TestVerifierRejectsTamperedPayload(t *testing.T) {
	signer, store, _ := makeTestSigner(t, "com.example", "k1")
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
	doc, _ := signer.Sign(payload)

	tampered := payload
	tampered.Version = "1.0.1"

	verifier := NewSignatureVerifier(store)
	result := verifier.Verify(context.Background(), VerifyInput{
		Document:      doc,
		ActualPayload: tampered,
	})
	if result.Valid {
		t.Fatal("expected invalid signature for tampered payload")
	}
	if result.Status != SignatureStatusPayloadMismatch && result.Status != SignatureStatusInvalidSignature {
		t.Fatalf("expected payload_mismatch or invalid_signature, got %s", result.Status)
	}
}

func TestVerifierRejectsUnknownKey(t *testing.T) {
	signer, _, _ := makeTestSigner(t, "com.example", "k1")
	payload := SignaturePayload{
		ExtensionID: "com.example/weather",
		Version:     "1.0.0",
		PublisherID: "com.example",
		KeyID:       "k1",
		CreatedAt:   time.Now().UTC(),
	}
	doc, _ := signer.Sign(payload)

	otherStore := NewPublisherStore()
	verifier := NewSignatureVerifier(otherStore)
	result := verifier.Verify(context.Background(), VerifyInput{
		Document:      doc,
		ActualPayload: payload,
	})
	if result.Valid {
		t.Fatal("expected unknown key")
	}
	if result.Status != SignatureStatusUnknownKey {
		t.Fatalf("expected unknown_key, got %s", result.Status)
	}
}

func TestVerifierRejectsRevokedKey(t *testing.T) {
	signer, store, _ := makeTestSigner(t, "com.example", "k1")
	payload := SignaturePayload{
		ExtensionID: "com.example/weather",
		Version:     "1.0.0",
		PublisherID: "com.example",
		KeyID:       "k1",
		CreatedAt:   time.Now().UTC(),
	}
	doc, _ := signer.Sign(payload)

	store.RevokeKey(context.Background(), "com.example", "k1", "compromised")

	verifier := NewSignatureVerifier(store)
	result := verifier.Verify(context.Background(), VerifyInput{
		Document:      doc,
		ActualPayload: payload,
	})
	if result.Valid {
		t.Fatal("expected revoked key rejection")
	}
	if result.Status != SignatureStatusRevokedKey {
		t.Fatalf("expected revoked_key, got %s", result.Status)
	}
}

func TestVerifierRejectsContentMismatch(t *testing.T) {
	signer, store, _ := makeTestSigner(t, "com.example", "k1")
	payload := SignaturePayload{
		ExtensionID:     "com.example/weather",
		Version:         "1.0.0",
		ManifestHash:    "sha256:abc",
		ContentTreeHash: "sha256:def",
		PackageHash:     "sha256:ghi",
		PublisherID:     "com.example",
		KeyID:           "k1",
		CreatedAt:       time.Now().UTC(),
	}
	doc, _ := signer.Sign(payload)

	verifier := NewSignatureVerifier(store)
	result := verifier.Verify(context.Background(), VerifyInput{
		Document:              doc,
		ActualPayload:         payload,
		ActualManifestHash:    "sha256:DIFFERENT",
		ActualContentTreeHash: "sha256:def",
		ActualPackageHash:     "sha256:ghi",
	})
	if result.Valid {
		t.Fatal("expected invalid due to manifest hash mismatch")
	}
	if result.Status != SignatureStatusPayloadMismatch {
		t.Fatalf("expected payload_mismatch, got %s: %s", result.Status, result.Reason)
	}
}

func TestVerifierRejectsUnsupportedAlgorithm(t *testing.T) {
	_, store, _ := makeTestSigner(t, "com.example", "k1")
	verifier := NewSignatureVerifier(store)
	doc := SignatureDocument{
		Format:      SignatureFormatV1,
		Algorithm:   "rsa-2048",
		PublisherID: "com.example",
		KeyID:       "k1",
	}
	result := verifier.Verify(context.Background(), VerifyInput{
		Document:      doc,
		ActualPayload: SignaturePayload{},
	})
	if result.Valid {
		t.Fatal("expected unsupported algorithm")
	}
	if result.Status != SignatureStatusUnsupportedAlgorithm {
		t.Fatalf("expected unsupported_algorithm, got %s", result.Status)
	}
}

func TestVerifierRejectsPublisherMismatch(t *testing.T) {
	signer, _, _ := makeTestSigner(t, "com.example", "k1")
	payload := SignaturePayload{
		ExtensionID: "com.example/weather",
		Version:     "1.0.0",
		PublisherID: "com.evil",
		KeyID:       "k1",
		CreatedAt:   time.Now().UTC(),
	}
	_, err := signer.Sign(payload)
	if err == nil {
		t.Fatal("expected signer to reject mismatched publisher")
	}
}

func TestPayloadCanonicalizationStable(t *testing.T) {
	payload := SignaturePayload{
		ExtensionID:     "com.example/weather",
		Version:         "1.0.0",
		ManifestVersion: 2,
		ManifestHash:    "sha256:abc",
		ContentTreeHash: "sha256:def",
		PackageHash:     "sha256:ghi",
		PublisherID:     "com.example",
		KeyID:           "k1",
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	h1 := payload.PayloadHash()
	h2 := payload.PayloadHash()
	if h1 != h2 {
		t.Fatalf("canonicalization not stable: %s != %s", h1, h2)
	}
	if h1 == "" {
		t.Fatal("expected non-empty payload hash")
	}
}
