package package_security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/google/uuid"
)

type StagingStatus string

const (
	StagingCreated   StagingStatus = "created"
	StagingPopulated StagingStatus = "populated"
	StagingSealed    StagingStatus = "sealed"
	StagingExpired   StagingStatus = "expired"
	StagingCleaned   StagingStatus = "cleaned"
)

type StagingArea struct {
	ID          string        `json:"id"`
	Path        string        `json:"path"`
	Owner       string        `json:"owner"`
	Source      string        `json:"source"`
	Status      StagingStatus `json:"status"`
	ContentHash string        `json:"content_hash"`
	CreatedAt   time.Time     `json:"created_at"`
	ExpiresAt   time.Time     `json:"expires_at"`
	SealedAt    *time.Time    `json:"sealed_at,omitempty"`
}

type StagingManager struct {
	baseDir  string
	stagings map[string]*StagingArea
}

func NewStagingManager(baseDir string) *StagingManager {
	return &StagingManager{
		baseDir:  baseDir,
		stagings: make(map[string]*StagingArea),
	}
}

func (m *StagingManager) Create(ctx context.Context, purpose string) (*StagingArea, error) {
	id := "staging_" + uuid.NewString()
	stagingPath := filepath.Join(m.baseDir, id)

	if err := os.MkdirAll(stagingPath, 0o700); err != nil {
		return nil, err
	}

	area := &StagingArea{
		ID:        id,
		Path:      stagingPath,
		Owner:     purpose,
		Source:    purpose,
		Status:    StagingCreated,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	m.stagings[id] = area
	return area, nil
}

func (m *StagingManager) Get(ctx context.Context, stagingID string) (*StagingArea, error) {
	area, ok := m.stagings[stagingID]
	if !ok {
		return nil, ErrStagingNotFound
	}
	return area, nil
}

func (m *StagingManager) Seal(ctx context.Context, stagingID string) error {
	area, ok := m.stagings[stagingID]
	if !ok {
		return ErrStagingNotFound
	}

	if area.Status != StagingPopulated {
		return ErrStagingNotReady
	}

	hash, err := m.computeContentHash(area)
	if err != nil {
		return err
	}

	area.ContentHash = hash
	now := time.Now()
	area.Status = StagingSealed
	area.SealedAt = &now

	return nil
}

func (m *StagingManager) Verify(ctx context.Context, stagingID string) error {
	area, ok := m.stagings[stagingID]
	if !ok {
		return ErrStagingNotFound
	}

	if area.Status != StagingSealed {
		return ErrStagingNotSealed
	}

	currentHash, err := m.computeContentHash(area)
	if err != nil {
		return err
	}

	if currentHash != area.ContentHash {
		return ErrStagingTampered
	}

	return nil
}

func (m *StagingManager) Cleanup(ctx context.Context, stagingID string) error {
	area, ok := m.stagings[stagingID]
	if !ok {
		return nil
	}

	if err := robustRemoveAll(area.Path); err != nil {
		return err
	}

	area.Status = StagingCleaned
	delete(m.stagings, stagingID)
	return nil
}

func robustRemoveAll(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	if runtime.GOOS != "windows" {
		return nil
	}

	var dirs []string
	walkErr := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		os.Chmod(p, 0o666)
		return os.Remove(p)
	})
	if walkErr != nil {
		return walkErr
	}

	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}

	if _, err := os.Stat(path); err == nil {
		return os.RemoveAll(path)
	}

	return nil
}

func (m *StagingManager) CleanupExpired(ctx context.Context) []string {
	var cleaned []string
	now := time.Now()

	for id, area := range m.stagings {
		if now.After(area.ExpiresAt) {
			if err := m.Cleanup(ctx, id); err == nil {
				cleaned = append(cleaned, id)
			}
		}
	}

	return cleaned
}

func (m *StagingManager) MarkPopulated(ctx context.Context, stagingID string) error {
	area, ok := m.stagings[stagingID]
	if !ok {
		return ErrStagingNotFound
	}
	area.Status = StagingPopulated
	return nil
}

func (m *StagingManager) computeContentHash(area *StagingArea) (string, error) {
	h := sha256.New()
	err := filepath.Walk(area.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(area.Path, path)
		h.Write([]byte(rel))
		h.Write([]byte{0})
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fileHash := sha256.Sum256(data)
		h.Write(fileHash[:])
		h.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", err
	}

	hash := "sha256:" + hex.EncodeToString(h.Sum(nil))
	return hash, nil
}
