package trust

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type SignatureFormat string

const (
	SignatureFormatV1 SignatureFormat = "amitiax-signature-v1"
)

type SignatureAlgorithm string

const (
	SignatureAlgorithmEd25519 SignatureAlgorithm = "ed25519"
)

type SignatureDocument struct {
	Format      SignatureFormat    `json:"format"`
	Algorithm   SignatureAlgorithm `json:"algorithm"`
	PublisherID string             `json:"publisherId"`
	KeyID       string             `json:"keyId"`
	PayloadHash string             `json:"payloadHash"`
	Signature   string             `json:"signature"`
	CreatedAt   time.Time          `json:"createdAt"`
	Channel     string             `json:"channel,omitempty"`
}

type SignaturePayload struct {
	ExtensionID       string    `json:"extensionId"`
	Version           string    `json:"version"`
	ManifestVersion   int       `json:"manifestVersion"`
	ManifestHash      string    `json:"manifestHash"`
	ContentTreeHash   string    `json:"contentTreeHash"`
	PackageHash       string    `json:"packageHash"`
	PublisherID       string    `json:"publisherId"`
	KeyID             string    `json:"keyId"`
	CreatedAt         time.Time `json:"createdAt"`
	CompatibilityHash string    `json:"compatibilityHash,omitempty"`
	Channel           string    `json:"channel,omitempty"`
}

func (p SignaturePayload) CanonicalBytes() ([]byte, error) {
	canonical := struct {
		ExtensionID       string    `json:"extensionId"`
		Version           string    `json:"version"`
		ManifestVersion   int       `json:"manifestVersion"`
		ManifestHash      string    `json:"manifestHash"`
		ContentTreeHash   string    `json:"contentTreeHash"`
		PackageHash       string    `json:"packageHash"`
		PublisherID       string    `json:"publisherId"`
		KeyID             string    `json:"keyId"`
		CreatedAt         time.Time `json:"createdAt"`
		CompatibilityHash string    `json:"compatibilityHash,omitempty"`
		Channel           string    `json:"channel,omitempty"`
	}{
		ExtensionID:       p.ExtensionID,
		Version:           p.Version,
		ManifestVersion:   p.ManifestVersion,
		ManifestHash:      p.ManifestHash,
		ContentTreeHash:   p.ContentTreeHash,
		PackageHash:       p.PackageHash,
		PublisherID:       p.PublisherID,
		KeyID:             p.KeyID,
		CreatedAt:         p.CreatedAt.UTC(),
		CompatibilityHash: p.CompatibilityHashHash(),
		Channel:           p.Channel,
	}
	return json.Marshal(canonical)
}

func (p SignaturePayload) CompatibilityHashHash() string {
	if p.CompatibilityHash == "" {
		return ""
	}
	h := sha256.Sum256([]byte(p.CompatibilityHash))
	return "sha256:" + hex.EncodeToString(h[:])
}

func (p SignaturePayload) PayloadHash() string {
	bytes, err := p.CanonicalBytes()
	if err != nil {
		return ""
	}
	h := sha256.Sum256(bytes)
	return "sha256:" + hex.EncodeToString(h[:])
}

type SignatureVerificationResult struct {
	Valid             bool
	Status            SignatureStatus
	PublisherID       string
	KeyID             string
	KeyFingerprint    string
	Algorithm         SignatureAlgorithm
	SignatureDocument SignatureDocument
	Payload           SignaturePayload
	Warnings          []string
	Reason            string
}

type SignatureStatus string

const (
	SignatureStatusValid                SignatureStatus = "valid"
	SignatureStatusInvalidSignature     SignatureStatus = "invalid_signature"
	SignatureStatusUnknownKey           SignatureStatus = "unknown_key"
	SignatureStatusRevokedKey           SignatureStatus = "revoked_key"
	SignatureStatusExpiredKey           SignatureStatus = "expired_key"
	SignatureStatusPublisherMismatch    SignatureStatus = "publisher_mismatch"
	SignatureStatusContentMismatch      SignatureStatus = "content_mismatch"
	SignatureStatusUnsupportedAlgorithm SignatureStatus = "unsupported_algorithm"
	SignatureStatusMalformedDocument    SignatureStatus = "malformed_document"
	SignatureStatusPayloadMismatch      SignatureStatus = "payload_mismatch"
)

type SignatureVerifier struct {
	store *PublisherStore
}

func NewSignatureVerifier(store *PublisherStore) *SignatureVerifier {
	return &SignatureVerifier{store: store}
}

type VerifyInput struct {
	Document              SignatureDocument
	ActualPayload         SignaturePayload
	ActualManifestHash    string
	ActualContentTreeHash string
	ActualPackageHash     string
}

func (v *SignatureVerifier) Verify(ctx context.Context, input VerifyInput) SignatureVerificationResult {
	result := SignatureVerificationResult{
		SignatureDocument: input.Document,
		Algorithm:         input.Document.Algorithm,
		PublisherID:       input.Document.PublisherID,
		KeyID:             input.Document.KeyID,
	}

	if input.Document.Format != SignatureFormatV1 {
		result.Status = SignatureStatusMalformedDocument
		result.Reason = fmt.Sprintf("unsupported signature format %s", input.Document.Format)
		return result
	}

	if input.Document.Algorithm != SignatureAlgorithmEd25519 {
		result.Status = SignatureStatusUnsupportedAlgorithm
		result.Reason = fmt.Sprintf("unsupported algorithm %s", input.Document.Algorithm)
		return result
	}

	identity, err := v.store.Get(ctx, input.Document.PublisherID)
	if err != nil {
		result.Status = SignatureStatusUnknownKey
		result.Reason = "publisher not registered"
		return result
	}

	key := identity.FindKey(input.Document.KeyID)
	if key == nil {
		result.Status = SignatureStatusUnknownKey
		result.Reason = fmt.Sprintf("key %s not found for publisher %s", input.Document.KeyID, input.Document.PublisherID)
		return result
	}

	result.KeyFingerprint = key.Fingerprint()

	if key.IsRevoked() {
		result.Status = SignatureStatusRevokedKey
		result.Reason = key.RevokedReason
		return result
	}

	if key.IsExpired() {
		result.Status = SignatureStatusExpiredKey
		result.Reason = "key expired"
		return result
	}

	if !key.IsUsable() {
		result.Status = SignatureStatusRevokedKey
		result.Reason = fmt.Sprintf("key state %s not usable", key.State)
		return result
	}

	if input.Document.PublisherID != input.ActualPayload.PublisherID {
		result.Status = SignatureStatusPublisherMismatch
		result.Reason = "document publisher does not match payload publisher"
		return result
	}

	if input.Document.KeyID != input.ActualPayload.KeyID {
		result.Status = SignatureStatusPublisherMismatch
		result.Reason = "document key id does not match payload key id"
		return result
	}

	if input.ActualManifestHash != "" && input.ActualPayload.ManifestHash != "" &&
		input.ActualManifestHash != input.ActualPayload.ManifestHash {
		result.Status = SignatureStatusPayloadMismatch
		result.Reason = "manifest hash mismatch"
		return result
	}

	if input.ActualContentTreeHash != "" && input.ActualPayload.ContentTreeHash != "" &&
		input.ActualContentTreeHash != input.ActualPayload.ContentTreeHash {
		result.Status = SignatureStatusPayloadMismatch
		result.Reason = "content tree hash mismatch"
		return result
	}

	if input.ActualPackageHash != "" && input.ActualPayload.PackageHash != "" &&
		input.ActualPackageHash != input.ActualPayload.PackageHash {
		result.Status = SignatureStatusPayloadMismatch
		result.Reason = "package hash mismatch"
		return result
	}

	payloadHash := input.ActualPayload.PayloadHash()
	if input.Document.PayloadHash != payloadHash {
		result.Status = SignatureStatusPayloadMismatch
		result.Reason = "payload hash mismatch"
		return result
	}

	sigBytes, err := decodeSignature(input.Document.Signature)
	if err != nil {
		result.Status = SignatureStatusMalformedDocument
		result.Reason = fmt.Sprintf("malformed signature: %v", err)
		return result
	}

	canonicalBytes, err := input.ActualPayload.CanonicalBytes()
	if err != nil {
		result.Status = SignatureStatusMalformedDocument
		result.Reason = fmt.Sprintf("canonical encode failed: %v", err)
		return result
	}

	if len(key.PublicKey) != ed25519.PublicKeySize {
		result.Status = SignatureStatusInvalidSignature
		result.Reason = "invalid public key size"
		return result
	}

	if !ed25519.Verify(ed25519.PublicKey(key.PublicKey), canonicalBytes, sigBytes) {
		result.Status = SignatureStatusInvalidSignature
		result.Reason = "ed25519 verification failed"
		return result
	}

	result.Payload = input.ActualPayload
	result.Status = SignatureStatusValid
	result.Valid = true
	return result
}

func decodeSignature(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty signature")
	}
	if strings.HasPrefix(s, "base64:") {
		return decodeBase64(strings.TrimPrefix(s, "base64:"))
	}
	return decodeBase64(s)
}

func decodeBase64(s string) ([]byte, error) {
	return base64Decode(s)
}

type Signer struct {
	publisherID string
	keyID       string
	privateKey  ed25519.PrivateKey
	publicKey   ed25519.PublicKey
}

func NewSigner(publisherID, keyID string, privateKey ed25519.PrivateKey) *Signer {
	return &Signer{
		publisherID: publisherID,
		keyID:       keyID,
		privateKey:  privateKey,
		publicKey:   privateKey.Public().(ed25519.PublicKey),
	}
}

func (s *Signer) Sign(payload SignaturePayload) (SignatureDocument, error) {
	if payload.PublisherID != s.publisherID {
		return SignatureDocument{}, errors.New("trust: publisher id mismatch")
	}
	if payload.KeyID != s.keyID {
		return SignatureDocument{}, errors.New("trust: key id mismatch")
	}

	canonicalBytes, err := payload.CanonicalBytes()
	if err != nil {
		return SignatureDocument{}, fmt.Errorf("trust: canonical encode failed: %w", err)
	}

	sig := ed25519.Sign(s.privateKey, canonicalBytes)
	doc := SignatureDocument{
		Format:      SignatureFormatV1,
		Algorithm:   SignatureAlgorithmEd25519,
		PublisherID: s.publisherID,
		KeyID:       s.keyID,
		PayloadHash: payload.PayloadHash(),
		Signature:   "base64:" + base64Encode(sig),
		CreatedAt:   time.Now().UTC(),
		Channel:     payload.Channel,
	}
	return doc, nil
}

func (s *Signer) PublicKey() []byte {
	return []byte(s.publicKey)
}

func (s *Signer) KeyID() string {
	return s.keyID
}

func (s *Signer) PublisherID() string {
	return s.publisherID
}

func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(nil)
}

func CanonicalizePayloads(payloads []SignaturePayload) string {
	sorted := make([]SignaturePayload, len(payloads))
	copy(sorted, payloads)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ExtensionID != sorted[j].ExtensionID {
			return sorted[i].ExtensionID < sorted[j].ExtensionID
		}
		return sorted[i].Version < sorted[j].Version
	})
	var b strings.Builder
	for _, p := range sorted {
		b.WriteString(p.PayloadHash())
		b.WriteString("\n")
	}
	h := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(h[:])
}
