package artifact

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
)

type ArtifactStore struct {
	DataDir string
}

func NewArtifactStore(dataDir string) *ArtifactStore {
	return &ArtifactStore{DataDir: dataDir}
}

func (s *ArtifactStore) WriteArtifact(workDir *WorkDirectory, kind string, frameIndex *int, data []byte, filename string) (*contracts.ProcessingArtifact, error) {
	if workDir == nil {
		return nil, fmt.Errorf("artifact: write artifact: workDir is nil")
	}
	if kind == "" {
		return nil, fmt.Errorf("artifact: write artifact: kind is empty")
	}
	if filename == "" {
		return nil, fmt.Errorf("artifact: write artifact: filename is empty")
	}

	subdir := workDir.SubdirByKind(kind)
	targetPath, err := s.SafeJoin(subdir, filename)
	if err != nil {
		return nil, fmt.Errorf("artifact: write artifact: safe join: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, fmt.Errorf("artifact: write artifact: mkdir: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return nil, fmt.Errorf("artifact: write artifact: write file: %w", err)
	}

	contentHash, err := s.ComputeFileHash(targetPath)
	if err != nil {
		return nil, fmt.Errorf("artifact: write artifact: compute hash: %w", err)
	}

	relPath, err := filepath.Rel(workDir.RootPath, targetPath)
	if err != nil {
		return nil, fmt.Errorf("artifact: write artifact: relative path: %w", err)
	}

	artifact := &contracts.ProcessingArtifact{
		ID:           uuid.New().String(),
		RevisionID:   workDir.RevisionID,
		FrameIndex:   frameIndex,
		ArtifactKind: kind,
		Stage:        stageFromKind(kind),
		RelativePath: filepath.ToSlash(relPath),
		MimeType:     mimeFromKind(kind),
		ByteSize:     int64(len(data)),
		ContentHash:  contentHash,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	return artifact, nil
}

func (s *ArtifactStore) WriteImage(workDir *WorkDirectory, subdir string, frameIndex int, img *image.NRGBA, filename string) (string, error) {
	if workDir == nil {
		return "", fmt.Errorf("artifact: write image: workDir is nil")
	}
	if img == nil {
		return "", fmt.Errorf("artifact: write image: image is nil")
	}
	if filename == "" {
		return "", fmt.Errorf("artifact: write image: filename is empty")
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("artifact: write image: encode png: %w", err)
	}

	targetDir, err := s.SafeJoin(workDir.RootPath, subdir)
	if err != nil {
		return "", fmt.Errorf("artifact: write image: safe join dir: %w", err)
	}
	targetPath, err := s.SafeJoin(targetDir, filename)
	if err != nil {
		return "", fmt.Errorf("artifact: write image: safe join file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", fmt.Errorf("artifact: write image: mkdir: %w", err)
	}

	if err := os.WriteFile(targetPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("artifact: write image: write file: %w", err)
	}

	relPath, err := filepath.Rel(workDir.RootPath, targetPath)
	if err != nil {
		return "", fmt.Errorf("artifact: write image: relative path: %w", err)
	}

	return filepath.ToSlash(relPath), nil
}

func (s *ArtifactStore) WriteMask(workDir *WorkDirectory, frameIndex int, mask *image.Gray, filename string) (string, error) {
	if workDir == nil {
		return "", fmt.Errorf("artifact: write mask: workDir is nil")
	}
	if mask == nil {
		return "", fmt.Errorf("artifact: write mask: mask is nil")
	}
	if filename == "" {
		return "", fmt.Errorf("artifact: write mask: filename is empty")
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, mask); err != nil {
		return "", fmt.Errorf("artifact: write mask: encode png: %w", err)
	}

	targetPath, err := s.SafeJoin(workDir.MasksDir, filename)
	if err != nil {
		return "", fmt.Errorf("artifact: write mask: safe join: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", fmt.Errorf("artifact: write mask: mkdir: %w", err)
	}

	if err := os.WriteFile(targetPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("artifact: write mask: write file: %w", err)
	}

	relPath, err := filepath.Rel(workDir.RootPath, targetPath)
	if err != nil {
		return "", fmt.Errorf("artifact: write mask: relative path: %w", err)
	}

	return filepath.ToSlash(relPath), nil
}

func (s *ArtifactStore) WriteJSON(workDir *WorkDirectory, subdir string, filename string, v interface{}) (string, error) {
	if workDir == nil {
		return "", fmt.Errorf("artifact: write json: workDir is nil")
	}
	if filename == "" {
		return "", fmt.Errorf("artifact: write json: filename is empty")
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("artifact: write json: marshal: %w", err)
	}

	targetDir, err := s.SafeJoin(workDir.RootPath, subdir)
	if err != nil {
		return "", fmt.Errorf("artifact: write json: safe join dir: %w", err)
	}
	targetPath, err := s.SafeJoin(targetDir, filename)
	if err != nil {
		return "", fmt.Errorf("artifact: write json: safe join file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", fmt.Errorf("artifact: write json: mkdir: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "", fmt.Errorf("artifact: write json: write file: %w", err)
	}

	relPath, err := filepath.Rel(workDir.RootPath, targetPath)
	if err != nil {
		return "", fmt.Errorf("artifact: write json: relative path: %w", err)
	}

	return filepath.ToSlash(relPath), nil
}

func (s *ArtifactStore) ReadArtifact(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("artifact: read artifact: path is empty")
	}

	fullPath := path
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(s.DataDir, path)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("artifact: read artifact: read %s: %w", fullPath, err)
	}
	return data, nil
}

func (s *ArtifactStore) SafeJoin(base, relative string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("artifact: safe join: base is empty")
	}
	if relative == "" {
		return "", fmt.Errorf("artifact: safe join: relative is empty")
	}

	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("artifact: safe join: relative path must not be absolute: %s", relative)
	}

	cleanBase := filepath.Clean(base)
	joined := filepath.Join(cleanBase, relative)
	cleanJoined := filepath.Clean(joined)

	if !isPathUnderBase(cleanBase, cleanJoined) {
		return "", fmt.Errorf("artifact: safe join: path escapes base: %s -> %s", cleanBase, cleanJoined)
	}

	info, err := os.Lstat(cleanJoined)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("artifact: safe join: symlink not allowed: %s", cleanJoined)
		}
	}

	return cleanJoined, nil
}

func (s *ArtifactStore) ComputeFileHash(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("artifact: compute file hash: path is empty")
	}
	return computeFileHash(path)
}

func (s *ArtifactStore) ComputePixelHash(img *image.NRGBA) string {
	if img == nil {
		return ""
	}
	h := sha256.New()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(img.Bounds().Dx()))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(img.Bounds().Dy()))
	h.Write(buf[:])
	h.Write(img.Pix)
	return hex.EncodeToString(h.Sum(nil))
}

func stageFromKind(kind string) string {
	switch kind {
	case contracts.ArtifactKindCellSource:
		return "split"
	case contracts.ArtifactKindForeground:
		return "background"
	case contracts.ArtifactKindMask:
		return "background"
	case contracts.ArtifactKindNormalized:
		return "normalize"
	case contracts.ArtifactKindFrame:
		return "encode"
	case contracts.ArtifactKindMeasurement:
		return "measure"
	case contracts.ArtifactKindTransform:
		return "transform"
	case contracts.ArtifactKindManifest:
		return "publish"
	default:
		return "unknown"
	}
}

func mimeFromKind(kind string) string {
	switch kind {
	case contracts.ArtifactKindMeasurement, contracts.ArtifactKindTransform, contracts.ArtifactKindManifest:
		return "application/json"
	default:
		return "image/png"
	}
}
