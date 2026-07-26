package package_security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

type ManifestIntegrity struct {
	Algorithm       string            `json:"algorithm"`
	ContentTreeHash string            `json:"content_tree_hash"`
	Files           map[string]string `json:"files,omitempty"`
}

type ManifestBindingVerifier struct {
	hasher *ContentHasher
}

func NewManifestBindingVerifier() *ManifestBindingVerifier {
	return &ManifestBindingVerifier{
		hasher: NewContentHasher(),
	}
}

type IntegrityVerificationResult struct {
	Passed          bool
	ContentTreeHash string
	MismatchedFiles []string
	MissingFiles    []string
	ExtraFiles      []string
}

func (v *ManifestBindingVerifier) Verify(ctx context.Context, manifestRaw json.RawMessage, entries []ArchiveEntryInfo, contentReader func(path string) ([]byte, error)) (*IntegrityVerificationResult, error) {
	result := &IntegrityVerificationResult{Passed: true}

	contentTreeHash, entryHashes, err := v.hasher.HashContentTree(entries, contentReader)
	if err != nil {
		return nil, err
	}
	result.ContentTreeHash = contentTreeHash

	var integrity ManifestIntegrity
	type manifestWithIntegrity struct {
		Integrity *ManifestIntegrity `json:"integrity"`
	}
	var mwi manifestWithIntegrity
	if json.Unmarshal(manifestRaw, &mwi) == nil && mwi.Integrity != nil {
		integrity = *mwi.Integrity
	}

	if integrity.ContentTreeHash != "" && integrity.ContentTreeHash != contentTreeHash {
		result.Passed = false
		return result, nil
	}

	if integrity.Files != nil {
		for file, expectedHash := range integrity.Files {
			actualHash, ok := entryHashes[file]
			if !ok {
				result.MissingFiles = append(result.MissingFiles, file)
				result.Passed = false
				continue
			}
			expectedHash = strings.TrimPrefix(expectedHash, "sha256:")
			if actualHash != expectedHash {
				result.MismatchedFiles = append(result.MismatchedFiles, file)
				result.Passed = false
			}
		}

		for path := range entryHashes {
			if _, ok := integrity.Files[path]; !ok {
				result.ExtraFiles = append(result.ExtraFiles, path)
			}
		}
	}

	return result, nil
}

func (v *ManifestBindingVerifier) BuildIntegrity(entries []ArchiveEntryInfo, contentReader func(path string) ([]byte, error)) (*ManifestIntegrity, error) {
	contentTreeHash, entryHashes, err := v.hasher.HashContentTree(entries, contentReader)
	if err != nil {
		return nil, err
	}

	files := make(map[string]string)
	for path, hash := range entryHashes {
		files[path] = "sha256:" + hash
	}

	return &ManifestIntegrity{
		Algorithm:       "sha256",
		ContentTreeHash: contentTreeHash,
		Files:           files,
	}, nil
}

func HashBytes(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
