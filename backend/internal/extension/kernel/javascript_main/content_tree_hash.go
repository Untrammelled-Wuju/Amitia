package javascript_main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ContentTreeHash struct {
}

func NewContentTreeHash() *ContentTreeHash {
	return &ContentTreeHash{}
}

func (c *ContentTreeHash) Compute(rootDir string) (string, error) {
	entries := make([]string, 0)
	hashes := make([]string, 0)

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if c.shouldIgnore(relPath) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h := sha256.Sum256(data)
		entries = append(entries, relPath)
		hashes = append(hashes, hex.EncodeToString(h[:]))
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(entries)
	final := sha256.New()
	for i, entry := range entries {
		final.Write([]byte(entry))
		final.Write([]byte{0})
		final.Write([]byte(hashes[i]))
		final.Write([]byte{0})
	}
	return hex.EncodeToString(final.Sum(nil)), nil
}

func (c *ContentTreeHash) shouldIgnore(relPath string) bool {
	base := filepath.Base(relPath)
	if strings.HasPrefix(base, ".") {
		return true
	}
	ignored := map[string]bool{
		"node_modules": true,
		".git":         true,
		"Thumbs.db":    true,
		"desktop.ini":  true,
	}
	parts := strings.Split(relPath, "/")
	for _, p := range parts {
		if ignored[p] {
			return true
		}
	}
	return false
}

func (c *ContentTreeHash) ComputeFiles(files map[string][]byte) string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write(files[k])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
