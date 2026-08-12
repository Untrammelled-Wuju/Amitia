package screenshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Artifact struct {
	ResourceURI      string    `json:"resourceUri"`
	MIMEType         string    `json:"mimeType"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	SizeBytes        int64     `json:"sizeBytes"`
	ContentHash      string    `json:"contentHash,omitempty"`
	DisplayID        int       `json:"displayId"`
	CaptureTimestamp time.Time `json:"captureTimestamp"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

func (a Artifact) IsValid() bool {
	return a.ResourceURI != "" &&
		a.MIMEType != "" &&
		a.Width > 0 &&
		a.Height > 0 &&
		a.SizeBytes > 0
}

func (a Artifact) IsExpired(now time.Time) bool {
	return !a.ExpiresAt.IsZero() && now.After(a.ExpiresAt)
}

func ComputeContentHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for hash: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("compute sha256: %w", err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func AtomicWrite(path string, writer func(tmp *os.File) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".screenshot-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	defer func() {
		tmp.Close()
		if _, statErr := os.Stat(tmpName); statErr == nil {
			os.Remove(tmpName)
		}
	}()

	if err := writer(tmp); err != nil {
		return err
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}

func SafeResourceName(requestID string, ext string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' {
			return r
		}
		return '_'
	}, requestID)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return safe + ext
}
