package editing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type assetStore struct {
	dataDir string
	repo    Repository
}

func NewAssetStore(dataDir string, repo Repository) *assetStore {
	return &assetStore{
		dataDir: dataDir,
		repo:    repo,
	}
}

func (s *assetStore) ComputeHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func (s *assetStore) GetAssetStoreDir() string {
	return filepath.Join(s.dataDir, "desktop-pets", "assets")
}

func (s *assetStore) getFrameStoragePath(hash string) string {
	ab := hash[:2]
	cd := hash[2:4]
	return filepath.Join("desktop-pets", "assets", "frames", "sha256", ab, cd, hash+".png")
}

func (s *assetStore) atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func (s *assetStore) WriteAsset(ctx context.Context, data []byte, mimeType string, sourceType string, sourceRefID string) (*FrameAsset, error) {
	hash := s.ComputeHash(data)

	existing, err := s.repo.GetFrameAssetByHash(hash, mimeType)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	assetID := "asset-" + hash[:16]
	storagePath := s.getFrameStoragePath(hash)
	fullPath := filepath.Join(s.dataDir, storagePath)

	if err := s.atomicWrite(fullPath, data); err != nil {
		return nil, err
	}

	width, height := 0, 0
	if strings.HasPrefix(mimeType, "image/") {
		f, err := os.Open(fullPath)
		if err == nil {
			if cfg, err := png.DecodeConfig(f); err == nil {
				width = cfg.Width
				height = cfg.Height
			}
			f.Close()
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	asset := &FrameAsset{
		ID:           assetID,
		ContentHash:  hash,
		StoragePath:  storagePath,
		MimeType:     mimeType,
		Width:        width,
		Height:       height,
		ByteSize:     int64(len(data)),
		AlphaMode:    "straight",
		ColorSpace:   "sRGB",
		SourceType:   sourceType,
		SourceRefID:  sourceRefID,
		OriginalHash: hash,
		Status:       AssetStatusReady,
		CreatedAt:    now,
	}

	if err := s.repo.CreateFrameAsset(asset); err != nil {
		os.Remove(fullPath)
		return nil, fmt.Errorf("create frame asset record: %w", err)
	}

	return asset, nil
}

func (s *assetStore) GetAssetPath(assetID string) (string, error) {
	asset, err := s.repo.GetFrameAsset(assetID)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.dataDir, asset.StoragePath), nil
}

func (s *assetStore) GetAsset(assetID string) (*FrameAsset, error) {
	return s.repo.GetFrameAsset(assetID)
}

func (s *assetStore) ReadImage(assetID string) (image.Image, string, error) {
	asset, err := s.repo.GetFrameAsset(assetID)
	if err != nil {
		return nil, "", err
	}
	fullPath := filepath.Join(s.dataDir, asset.StoragePath)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, "", fmt.Errorf("open asset file: %w", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, "", fmt.Errorf("decode png: %w", err)
	}
	return img, "png", nil
}

func (s *assetStore) WriteMaskData(ctx context.Context, sessionID string, data []byte) (string, error) {
	hash := s.ComputeHash(data)
	ab := hash[:2]
	cd := hash[2:4]
	relPath := filepath.Join("desktop-pets", "assets", "masks", "sha256", ab, cd, hash+".dat")
	fullPath := filepath.Join(s.dataDir, relPath)

	if err := s.atomicWrite(fullPath, data); err != nil {
		return "", err
	}
	return relPath, nil
}

func (s *assetStore) GetRevisionDir(processingTaskID, actionKey, revisionID string) string {
	return filepath.Join(s.dataDir, "desktop-pets", "generation-tasks", processingTaskID, "processed", "actions", actionKey, "revisions", revisionID)
}

func (s *assetStore) EnsureRevisionDir(processingTaskID, actionKey, revisionID string) (string, error) {
	dir := s.GetRevisionDir(processingTaskID, actionKey, revisionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create revision dir: %w", err)
	}
	return dir, nil
}

func (s *assetStore) WriteManifest(revisionID string, manifest *RevisionManifest) (string, string, error) {
	dir := s.GetRevisionDir(manifest.ProcessingTaskID, manifest.ActionKey, revisionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", fmt.Errorf("create revision dir: %w", err)
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return "", "", fmt.Errorf("marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	if err := s.atomicWrite(manifestPath, data); err != nil {
		return "", "", err
	}

	hash := s.ComputeHash(data)
	relPath := filepath.Join("desktop-pets", "generation-tasks", manifest.ProcessingTaskID, "processed", "actions", manifest.ActionKey, "revisions", revisionID, "manifest.json")

	return relPath, hash, nil
}

func (s *assetStore) ReadManifest(revisionID string) (*RevisionManifest, error) {
	rev, err := s.repo.GetActionRevision(revisionID)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(s.dataDir, rev.ManifestPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest file: %w", err)
	}

	var manifest RevisionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	return &manifest, nil
}
