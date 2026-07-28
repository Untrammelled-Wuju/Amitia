package trust

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const SignatureFilePath = "META-INF/amitia-signature.json"

type ArtifactEntry struct {
	Path   string
	MIME   string
	Size   int64
	SHA256 string
}

func ComputeKeyID(publicKey ed25519.PublicKey) string {
	if len(publicKey) == 0 {
		return ""
	}
	h := sha256.Sum256(publicKey)
	return "sha256:" + hex.EncodeToString(h[:])
}

func ComputeCanonicalArtifactHash(entries []ArtifactEntry) string {
	filtered := make([]ArtifactEntry, 0, len(entries))
	for _, e := range entries {
		normalized := normalizeArtifactPath(e.Path)
		if normalized == SignatureFilePath {
			continue
		}
		copy := e
		copy.Path = normalized
		filtered = append(filtered, copy)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Path < filtered[j].Path
	})
	h := sha256.New()
	for _, e := range filtered {
		h.Write([]byte(e.Path))
		h.Write([]byte{0})
		h.Write([]byte(e.MIME))
		h.Write([]byte{0})
		h.Write([]byte(fmt.Sprintf("%d", e.Size)))
		h.Write([]byte{0})
		h.Write([]byte(e.SHA256))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func normalizeArtifactPath(p string) string {
	cleaned := filepath.ToSlash(p)
	cleaned = strings.TrimPrefix(cleaned, "./")
	return cleaned
}

func CollectArtifactEntriesFromDir(dir string) ([]ArtifactEntry, error) {
	var entries []ArtifactEntry
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		normalized := normalizeArtifactPath(rel)
		if normalized == SignatureFilePath {
			return nil
		}
		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, ArtifactEntry{
			Path:   normalized,
			MIME:   detectMIME(normalized),
			Size:   info.Size(),
			SHA256: hash,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func detectMIME(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return "application/json"
	case ".js", ".mjs":
		return "application/javascript"
	case ".wasm":
		return "application/wasm"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

type PackageSigner struct {
	signer *Signer
}

func NewPackageSigner(signer *Signer) *PackageSigner {
	return &PackageSigner{signer: signer}
}

type PackageSignatureInput struct {
	ExtensionID       string
	Version           string
	ManifestVersion   int
	ManifestHash      string
	ContentTreeHash   string
	ArtifactHash      string
	CompatibilityHash string
	Channel           string
}

func (ps *PackageSigner) SignPackage(input PackageSignatureInput) (SignatureDocument, SignaturePayload, error) {
	payload := SignaturePayload{
		ExtensionID:       input.ExtensionID,
		Version:           input.Version,
		ManifestVersion:   input.ManifestVersion,
		ManifestHash:      input.ManifestHash,
		ContentTreeHash:   input.ContentTreeHash,
		PackageHash:       input.ArtifactHash,
		PublisherID:       ps.signer.PublisherID(),
		KeyID:             ps.signer.KeyID(),
		CreatedAt:         time.Now().UTC(),
		CompatibilityHash: input.CompatibilityHash,
		Channel:           input.Channel,
	}
	doc, err := ps.signer.Sign(payload)
	if err != nil {
		return SignatureDocument{}, SignaturePayload{}, fmt.Errorf("trust: sign package: %w", err)
	}
	return doc, payload, nil
}

func SerializeSignatureDocument(doc SignatureDocument) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

func ParseSignatureDocument(data []byte) (SignatureDocument, error) {
	var doc SignatureDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return SignatureDocument{}, fmt.Errorf("trust: parse signature document: %w", err)
	}
	return doc, nil
}

func ParseSignaturePayload(data []byte) (SignaturePayload, error) {
	var payload SignaturePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return SignaturePayload{}, fmt.Errorf("trust: parse signature payload: %w", err)
	}
	return payload, nil
}

type PackageVerificationInput struct {
	Document             SignatureDocument
	ActualExtensionID    string
	ActualVersion        string
	ActualManifestVersion int
	ActualManifestHash   string
	ActualContentTreeHash string
	ActualArtifactHash   string
}

func (v *SignatureVerifier) VerifyPackage(ctx context.Context, input PackageVerificationInput) SignatureVerificationResult {
	actualPayload := SignaturePayload{
		ExtensionID:       input.ActualExtensionID,
		Version:           input.ActualVersion,
		ManifestVersion:   input.ActualManifestVersion,
		ManifestHash:      input.ActualManifestHash,
		ContentTreeHash:   input.ActualContentTreeHash,
		PackageHash:       input.ActualArtifactHash,
		PublisherID:       input.Document.PublisherID,
		KeyID:             input.Document.KeyID,
		CreatedAt:         input.Document.CreatedAt,
	}
	return v.Verify(ctx, VerifyInput{
		Document:              input.Document,
		ActualPayload:         actualPayload,
		ActualManifestHash:    input.ActualManifestHash,
		ActualContentTreeHash: input.ActualContentTreeHash,
		ActualPackageHash:     input.ActualArtifactHash,
	})
}

func IsBlockingSignatureStatus(status SignatureStatus) bool {
	switch status {
	case SignatureStatusInvalidSignature,
		SignatureStatusRevokedKey,
		SignatureStatusExpiredKey,
		SignatureStatusPublisherMismatch,
		SignatureStatusContentMismatch,
		SignatureStatusUnsupportedAlgorithm,
		SignatureStatusMalformedDocument,
		SignatureStatusPayloadMismatch:
		return true
	default:
		return false
	}
}

func IsSignatureValid(result SignatureVerificationResult) bool {
	return result.Valid && result.Status == SignatureStatusValid
}

var ErrSignatureRequired = errors.New("trust: signature required for production install")
