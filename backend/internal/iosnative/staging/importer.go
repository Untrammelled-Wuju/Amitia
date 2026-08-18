package staging

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

const (
	MaxChunkSize      = 1048576
	DefaultMaxReadSize = 50 * 1048576 * 1024
)

type NativeResourceBridge interface {
	Stat(nativeStagingID string) (size int64, mimeType string, filename string, err error)
	ReadChunk(nativeStagingID string, offset, length int64) ([]byte, error)
	Release(nativeStagingID string) error
}

type StagingImportRequest struct {
	NativeStagingID string `json:"nativeStagingId"`
	TaskRunID       string `json:"taskRunId,omitempty"`
	MimeType        string `json:"mimeType"`
	Filename        string `json:"filename,omitempty"`
	Source          string `json:"source,omitempty"`
	MaxReadBytes    int64  `json:"maxReadBytes,omitempty"`
}

type StagingImportResult struct {
	ResourceURI string `json:"resourceUri"`
	Size        int64  `json:"size"`
	MimeType    string `json:"mimeType"`
	Filename    string `json:"filename"`
	ImportedAt  string `json:"importedAt"`
	Checksum    string `json:"checksum"`
}

type StagingImporter struct {
	baseDir    string
	maxChunk   int64
	maxRead    int64
}

func NewStagingImporter(baseDir string) *StagingImporter {
	return &StagingImporter{
		baseDir:  baseDir,
		maxChunk: MaxChunkSize,
		maxRead:  DefaultMaxReadSize,
	}
}

func (i *StagingImporter) ImportWithBridge(req StagingImportRequest, bridge NativeResourceBridge) (*StagingImportResult, error) {
	if req.NativeStagingID == "" {
		return nil, fmt.Errorf("missing nativeStagingId")
	}
	if !strings.HasPrefix(req.NativeStagingID, "nativeStaging:") {
		return nil, fmt.Errorf("invalid nativeStagingId format")
	}
	if bridge == nil {
		return nil, fmt.Errorf("native bridge unavailable")
	}

	maxRead := i.maxRead
	if req.MaxReadBytes > 0 {
		maxRead = req.MaxReadBytes
	}

	now := time.Now().UTC()

	size, bridgeMimeType, bridgeFilename, err := bridge.Stat(req.NativeStagingID)
	if err != nil {
		return nil, fmt.Errorf("native stat failed: %w", err)
	}

	mimeType := req.MimeType
	if mimeType == "" {
		mimeType = bridgeMimeType
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	filename := req.Filename
	if filename == "" {
		filename = bridgeFilename
	}
	if filename == "" {
		filename = strings.TrimPrefix(req.NativeStagingID, "nativeStaging:")
	}
	safeFilename := sanitizeFilename(filename)
	if safeFilename == "" {
		safeFilename = generateDefaultFilename(mimeType, now)
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

	hash := sha256.New()
	var totalWritten int64
	remaining := maxRead
	var chunkErr error

	if size > maxRead {
		size = maxRead
	}

	for offset := int64(0); offset < size && remaining > 0; {
		chunkLen := i.maxChunk
		if chunkLen > remaining {
			chunkLen = remaining
		}

		data, err := bridge.ReadChunk(req.NativeStagingID, offset, chunkLen)
		if err != nil {
			chunkErr = err
			break
		}
		if len(data) == 0 {
			break
		}

		if _, werr := f.Write(data); werr != nil {
			chunkErr = werr
			break
		}
		hash.Write(data)
		written := int64(len(data))
		totalWritten += written
		remaining -= written
		offset += written

		if int64(len(data)) < chunkLen {
			break
		}
	}

	f.Close()

	if chunkErr != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("stream chunk: %w", chunkErr)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("rename temp file: %w", err)
	}

	if rerr := bridge.Release(req.NativeStagingID); rerr != nil {
		fmt.Printf("failed to release native staging %s: %v\n", req.NativeStagingID, rerr)
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	resourceURI := fmt.Sprintf("amitia://native/blobs/%s/%s", dateDir, safeFilename)

	return &StagingImportResult{
		ResourceURI: resourceURI,
		Size:        totalWritten,
		MimeType:    mimeType,
		Filename:    safeFilename,
		ImportedAt:  now.Format(time.RFC3339),
		Checksum:    checksum,
	}, nil
}

type sliceBridge struct {
	data []byte
}

func (s *sliceBridge) Stat(id string) (int64, string, string, error) {
	return int64(len(s.data)), "application/octet-stream", "", nil
}

func (s *sliceBridge) ReadChunk(id string, offset, length int64) ([]byte, error) {
	if offset >= int64(len(s.data)) {
		return nil, io.EOF
	}
	end := offset + length
	if end > int64(len(s.data)) {
		end = int64(len(s.data))
	}
	return s.data[offset:end], nil
}

func (s *sliceBridge) Release(id string) error { return nil }

func (i *StagingImporter) ImportFromData(req StagingImportRequest, data []byte) (*StagingImportResult, error) {
	return i.ImportWithBridge(req, &sliceBridge{data: data})
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
