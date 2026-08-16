package staging

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type StagingImportRequest struct {
	NativeStagingID string `json:"nativeStagingId"`
	TaskRunID       string `json:"taskRunId,omitempty"`
	ContentBase64   string `json:"contentBase64,omitempty"`
	MimeType        string `json:"mimeType"`
	Filename        string `json:"filename,omitempty"`
	Source          string `json:"source,omitempty"`
}

type StagingImportResult struct {
	ResourceURI string `json:"resourceUri"`
	Size        int64  `json:"size"`
	MimeType    string `json:"mimeType"`
	ImportedAt  string `json:"importedAt"`
	Checksum    string `json:"checksum"`
}

type StagingImporter struct {
	baseDir string
}

func NewStagingImporter(baseDir string) *StagingImporter {
	return &StagingImporter{baseDir: baseDir}
}

func (i *StagingImporter) Import(req StagingImportRequest) (*StagingImportResult, error) {
	if req.NativeStagingID == "" {
		return nil, fmt.Errorf("missing nativeStagingId")
	}
	if !strings.HasPrefix(req.NativeStagingID, "nativeStaging:") {
		return nil, fmt.Errorf("invalid nativeStagingId format")
	}
	if req.ContentBase64 == "" {
		return nil, fmt.Errorf("missing contentBase64")
	}

	data, err := decodeBase64(req.ContentBase64)
	if err != nil {
		return nil, fmt.Errorf("decode contentBase64: %w", err)
	}

	now := time.Now().UTC()
	filename := req.Filename
	if filename == "" {
		filename = strings.TrimPrefix(req.NativeStagingID, "nativeStaging:")
	}
	safeFilename := sanitizeFilename(filename)
	if safeFilename == "" {
		safeFilename = generateDefaultFilename(req.MimeType, now)
	}

	dateDir := now.Format("2006/01/02")
	targetDir := filepath.Join(i.baseDir, "native", "staging", dateDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("create target dir: %w", err)
	}

	targetPath := filepath.Join(targetDir, safeFilename)
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	checksum := sha256.Sum256(data)

	return &StagingImportResult{
		ResourceURI: fmt.Sprintf("amitia://native/staging/%s/%s", dateDir, safeFilename),
		Size:        int64(len(data)),
		MimeType:    req.MimeType,
		ImportedAt:  now.Format(time.RFC3339),
		Checksum:    hex.EncodeToString(checksum[:]),
	}, nil
}

func (i *StagingImporter) Release(nativeStagingID string) error {
	return nil
}

func decodeBase64(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		if r == '/' || r == '\\' || r == 0 {
			continue
		}
		b.WriteRune(r)
	}
	result := b.String()
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}

func generateDefaultFilename(mimeType string, t time.Time) string {
	ext := extensionForMimeType(mimeType)
	return fmt.Sprintf("staged_%d%s", t.UnixNano(), ext)
}

func extensionForMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav":
		return ".wav"
	case "application/pdf":
		return ".pdf"
	default:
		return ".bin"
	}
}
