package package_security

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"
)

type SignatureStatus string

const (
	SignatureUnsigned           SignatureStatus = "unsigned"
	SignatureValid              SignatureStatus = "valid"
	SignatureInvalid            SignatureStatus = "invalid"
	SignatureUnknownKey         SignatureStatus = "unknown_key"
	SignatureRevokedKey         SignatureStatus = "revoked_key"
	SignatureExpiredKey         SignatureStatus = "expired_key"
	SignaturePublisherMismatch  SignatureStatus = "publisher_mismatch"
	SignatureContentMismatch    SignatureStatus = "content_mismatch"
	SignatureUnsupportedAlgorithm SignatureStatus = "unsupported_algorithm"
)

type PackageSignature struct {
	Algorithm       string    `json:"algorithm"`
	KeyID           string    `json:"key_id"`
	PublisherID     string    `json:"publisher_id"`
	SignedAt        time.Time `json:"signed_at"`
	ManifestHash    string    `json:"manifest_hash"`
	ContentTreeHash string    `json:"content_tree_hash"`
	Signature       []byte    `json:"signature"`
}

type SignatureVerificationInput struct {
	Signature       PackageSignature
	PublicKey       []byte
	ActualManifestHash    string
	ActualContentTreeHash string
}

type SignatureVerificationResult struct {
	Status    SignatureStatus
	KeyID     string
	Fingerprint string
	Algorithm string
}

type SignatureVerifier struct {
	trustedKeys map[string][]byte
}

func NewSignatureVerifier() *SignatureVerifier {
	return &SignatureVerifier{
		trustedKeys: make(map[string][]byte),
	}
}

func (v *SignatureVerifier) AddTrustedKey(keyID string, publicKey []byte) {
	v.trustedKeys[keyID] = publicKey
}

func (v *SignatureVerifier) Verify(ctx context.Context, input SignatureVerificationInput) SignatureVerificationResult {
	result := SignatureVerificationResult{
		KeyID:     input.Signature.KeyID,
		Algorithm: input.Signature.Algorithm,
	}

	if input.Signature.Algorithm != "ed25519" {
		result.Status = SignatureUnsupportedAlgorithm
		return result
	}

	if len(input.PublicKey) == 0 {
		result.Status = SignatureUnknownKey
		return result
	}

	if len(input.PublicKey) != ed25519.PublicKeySize {
		result.Status = SignatureInvalid
		return result
	}

	fingerprintHash := sha256.Sum256(input.PublicKey)
	result.Fingerprint = "sha256:" + hex.EncodeToString(fingerprintHash[:])

	if input.Signature.KeyID != "" && input.Signature.KeyID != result.Fingerprint {
		result.Status = SignaturePublisherMismatch
		return result
	}

	if input.Signature.ContentTreeHash != "" && input.Signature.ContentTreeHash != input.ActualContentTreeHash {
		result.Status = SignatureContentMismatch
		return result
	}

	if input.Signature.ManifestHash != "" && input.Signature.ManifestHash != input.ActualManifestHash {
		result.Status = SignatureContentMismatch
		return result
	}

	message := buildSignatureMessage(input.Signature)
	if !ed25519.Verify(ed25519.PublicKey(input.PublicKey), []byte(message), input.Signature.Signature) {
		result.Status = SignatureInvalid
		return result
	}

	result.Status = SignatureValid
	return result
}

func buildSignatureMessage(sig PackageSignature) string {
	return sig.PublisherID + ":" + sig.ContentTreeHash + ":" + sig.ManifestHash
}

func ParseSignatureDocument(raw json.RawMessage) (*PackageSignature, error) {
	type sigDoc struct {
		Algorithm       string `json:"algorithm"`
		KeyID           string `json:"key_id"`
		PublisherID     string `json:"publisher_id"`
		SignedAt        string `json:"signed_at"`
		ManifestHash    string `json:"manifest_hash"`
		ContentTreeHash string `json:"content_tree_hash"`
		Signature       string `json:"signature"`
	}

	var doc sigDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}

	sigBytes, err := base64.StdEncoding.DecodeString(doc.Signature)
	if err != nil {
		return nil, err
	}

	signedAt, _ := time.Parse(time.RFC3339, doc.SignedAt)

	return &PackageSignature{
		Algorithm:       doc.Algorithm,
		KeyID:           doc.KeyID,
		PublisherID:     doc.PublisherID,
		SignedAt:        signedAt,
		ManifestHash:    doc.ManifestHash,
		ContentTreeHash: doc.ContentTreeHash,
		Signature:       sigBytes,
	}, nil
}
