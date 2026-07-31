package packageformat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
)

const TreeHashAlgorithm = "amitia-tree-sha256-v1"

type FileEntry struct {
	Path   string
	SHA256 string
	Bytes  int64
}

func ComputeTreeHash(entries []FileEntry) string {
	sorted := make([]FileEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	h := sha256.New()
	for _, e := range sorted {
		h.Write([]byte("file"))
		h.Write([]byte{0})
		h.Write([]byte(e.Path))
		h.Write([]byte{0})
		h.Write([]byte(strconv.FormatInt(e.Bytes, 10)))
		h.Write([]byte{0})

		rawHash, err := hex.DecodeString(e.SHA256)
		if err != nil {
			rawHash = []byte(e.SHA256)
		}
		h.Write(rawHash)
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil))
}

func ComputeContentRootHash(entries []FileEntry, manifestHash string, manifestBytes int64) string {
	merged := make([]FileEntry, len(entries))
	copy(merged, entries)
	merged = append(merged, FileEntry{
		Path:   ManifestPseudoEntryPath,
		SHA256: manifestHash,
		Bytes:  manifestBytes,
	})
	return ComputeTreeHash(merged)
}

func ComputeManifestHash(manifest *Manifest) (string, error) {
	if manifest == nil {
		return "", fmt.Errorf("manifest is nil")
	}
	clone := *manifest
	clone.Integrity.ManifestHash = ""
	clone.Integrity.ContentRootHash = ""
	data, err := CanonicalJSON(clone)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
