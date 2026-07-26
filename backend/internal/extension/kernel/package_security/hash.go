package package_security

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

type ContentHasher struct{}

func NewContentHasher() *ContentHasher {
	return &ContentHasher{}
}

type ContentHashResult struct {
	ArchiveHash     string            `json:"archive_hash"`
	ContentTreeHash string            `json:"content_tree_hash"`
	EntryHashes     map[string]string `json:"entry_hashes"`
	Algorithm       string            `json:"algorithm"`
}

func (h *ContentHasher) HashArchive(raw []byte) string {
	hash := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func (h *ContentHasher) HashEntry(content []byte) string {
	hash := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func (h *ContentHasher) HashEntryRaw(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func (h *ContentHasher) HashContentTree(entries []ArchiveEntryInfo, contentReader func(path string) ([]byte, error)) (string, map[string]string, error) {
	sorted := make([]ArchiveEntryInfo, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].NormalizedPath < sorted[j].NormalizedPath
	})

	entryHashes := make(map[string]string)
	treeHash := sha256.New()

	for _, entry := range sorted {
		content, err := contentReader(entry.Path)
		if err != nil {
			return "", nil, err
		}

		entryHash := sha256.Sum256(content)
		entryHashStr := hex.EncodeToString(entryHash[:])
		entryHashes[entry.NormalizedPath] = entryHashStr

		treeHash.Write([]byte(entry.NormalizedPath))
		treeHash.Write([]byte{0})
		treeHash.Write([]byte(entryHashStr))
		treeHash.Write([]byte{0})
	}

	return "sha256:" + hex.EncodeToString(treeHash.Sum(nil)), entryHashes, nil
}

func (h *ContentHasher) BuildChecksumsFile(entryHashes map[string]string) []byte {
	keys := make([]string, 0, len(entryHashes))
	for k := range entryHashes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(entryHashes[key])
		builder.WriteString("  ")
		builder.WriteString(key)
		builder.WriteByte('\n')
	}
	return []byte(builder.String())
}

func (h *ContentHasher) VerifyEntry(path string, content []byte, expectedHash string) bool {
	actual := h.HashEntry(content)
	return actual == expectedHash
}
