package staging

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxChunkSize = 1048576

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
	targetDir := filepath.Join(i.baseDir, "resources", "blobs", dateDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("create target dir: %w", err)
	}

	targetPath := filepath.Join(targetDir, safeFilename)
	tmpPath := targetPath + ".tmp"

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(req.ContentBase64))
	hash := sha256.New()
	var totalWritten int64
	buf := make([]byte, MaxChunkSize)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			wn, werr := f.Write(buf[:n])
			if werr != nil {
				f.Close()
				os.Remove(tmpPath)
				return nil, fmt.Errorf("write chunk: %w", werr)
			}
			hash.Write(buf[:n])
			totalWritten += int64(wn)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			os.Remove(tmpPath)
			return nil, fmt.Errorf("decode chunk: %w", readErr)
		}
	}
	f.Close()

	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("rename temp file: %w", err)
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	resourceURI := fmt.Sprintf("amitia://resources/blobs/%s/%s", dateDir, safeFilename)

	return &StagingImportResult{
		ResourceURI: resourceURI,
		Size:        totalWritten,
		MimeType:    req.MimeType,
		ImportedAt:  now.Format(time.RFC3339),
		Checksum:    checksum,
	}, nil
}

func (i *StagingImporter) Release(nativeStagingID string) error {
	if nativeStagingID == "" {
		return fmt.Errorf("missing nativeStagingId")
	}
	if !strings.HasPrefix(nativeStagingID, "nativeStaging:") {
		return fmt.Errorf("invalid nativeStagingId format")
	}
	filename := strings.TrimPrefix(nativeStagingID, "nativeStaging:")
	safeFilename := sanitizeFilename(filename)
	if safeFilename == "" {
		return fmt.Errorf("invalid staging filename")
	}
	pattern := filepath.Join(i.baseDir, "resources", "blobs", "*", safeFilename)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob staging files: %w", err)
	}
	var lastErr error
	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			lastErr = err
		}
	}
	return lastErr
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
