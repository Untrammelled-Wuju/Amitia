package package_security

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type SnapshotStatus string

const (
	SnapshotActive   SnapshotStatus = "active"
	SnapshotRetained SnapshotStatus = "retained"
	SnapshotExpired  SnapshotStatus = "expired"
)

type RollbackSnapshot struct {
	SnapshotID     string         `json:"snapshot_id"`
	PackageID      string         `json:"package_id"`
	Version        string         `json:"version"`
	Status         SnapshotStatus `json:"status"`
	ArtifactPath   string         `json:"artifact_path"`
	ContentHash    string         `json:"content_hash"`
	ManifestHash   string         `json:"manifest_hash,omitempty"`
	OwnerID        string         `json:"owner_id"`
	CreatedAt      time.Time      `json:"created_at"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
}

type SnapshotManager struct {
	baseDir   string
	snapshots map[string]*RollbackSnapshot
}

func NewSnapshotManager(baseDir string) *SnapshotManager {
	return &SnapshotManager{
		baseDir:   baseDir,
		snapshots: make(map[string]*RollbackSnapshot),
	}
}

func (m *SnapshotManager) CreateSnapshot(ctx context.Context, sourcePath, packageID, version, ownerID string) (*RollbackSnapshot, error) {
	id := "snap_" + uuid.NewString()
	snapDir := filepath.Join(m.baseDir, id)

	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		return nil, err
	}

	if err := copyDir(sourcePath, snapDir); err != nil {
		os.RemoveAll(snapDir)
		return nil, err
	}

	hasher := NewContentHasher()
	contentHash := computeDirHash(snapDir, hasher)

	snapshot := &RollbackSnapshot{
		SnapshotID:   id,
		PackageID:    packageID,
		Version:      version,
		Status:       SnapshotActive,
		ArtifactPath: snapDir,
		ContentHash:  contentHash,
		OwnerID:      ownerID,
		CreatedAt:    time.Now(),
	}

	m.snapshots[id] = snapshot
	return snapshot, nil
}

func (m *SnapshotManager) GetSnapshot(ctx context.Context, snapshotID string) (*RollbackSnapshot, error) {
	snapshot, ok := m.snapshots[snapshotID]
	if !ok {
		return nil, ErrSnapshotNotFound
	}
	return snapshot, nil
}

func (m *SnapshotManager) Retain(ctx context.Context, snapshotID string) error {
	snapshot, ok := m.snapshots[snapshotID]
	if !ok {
		return ErrSnapshotNotFound
	}
	snapshot.Status = SnapshotRetained
	return nil
}

func (m *SnapshotManager) Delete(ctx context.Context, snapshotID string) error {
	snapshot, ok := m.snapshots[snapshotID]
	if !ok {
		return ErrSnapshotNotFound
	}

	if err := os.RemoveAll(snapshot.ArtifactPath); err != nil {
		return err
	}

	delete(m.snapshots, snapshotID)
	return nil
}

func (m *SnapshotManager) CleanupExpired(ctx context.Context) []string {
	var cleaned []string
	now := time.Now()

	for id, snap := range m.snapshots {
		if snap.ExpiresAt != nil && now.After(*snap.ExpiresAt) {
			if err := m.Delete(ctx, id); err == nil {
				cleaned = append(cleaned, id)
			}
		}
	}

	return cleaned
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o700); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0o600); err != nil {
				return err
			}
		}
	}

	return nil
}

func computeDirHash(dir string, hasher *ContentHasher) string {
	return "sha256:placeholder"
}
