package installation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	releaseStagingDir   = "staging"
	releasePublishedDir = "published"
	releaseArchivesDir  = "archives"
	installDirName      = "install"
	trashDirName        = "trash"
)

type ReleaseStorage struct {
	dataDir string
}

func NewReleaseStorage(dataDir string) *ReleaseStorage {
	return &ReleaseStorage{dataDir: dataDir}
}

func (s *ReleaseStorage) BaseDir() string {
	return filepath.Join(s.dataDir, "desktop-pets", "releases")
}

func (s *ReleaseStorage) StagingDir(releaseID string) string {
	return filepath.Join(s.BaseDir(), releaseStagingDir, releaseID)
}

func (s *ReleaseStorage) PublishedDir(petID, releaseID string) string {
	return filepath.Join(s.BaseDir(), releasePublishedDir, petID, releaseID)
}

func (s *ReleaseStorage) PublishedStorageKey(petID, releaseID string) string {
	return fmt.Sprintf("%s/%s/%s", releasePublishedDir, petID, releaseID)
}

func (s *ReleaseStorage) ArchivePath(petID, releaseID string) string {
	return filepath.Join(s.BaseDir(), releaseArchivesDir, petID, releaseID+".zip")
}

func (s *ReleaseStorage) ArchiveStorageKey(petID, releaseID string) string {
	return fmt.Sprintf("%s/%s/%s.zip", releaseArchivesDir, petID, releaseID)
}

func (s *ReleaseStorage) InstallDir(installationID string) string {
	return filepath.Join(s.dataDir, "desktop-pets", "installations", installationID)
}

func (s *ReleaseStorage) InstallStorageKey(installationID string) string {
	return fmt.Sprintf("installations/%s", installationID)
}

func (s *ReleaseStorage) TrashDir(installationID string) string {
	return filepath.Join(s.dataDir, "desktop-pets", "trash", installationID)
}

func (s *ReleaseStorage) TrashStorageKey(installationID string) string {
	return fmt.Sprintf("trash/%s", installationID)
}

func (s *ReleaseStorage) EnsureStagingDir(releaseID string) error {
	return os.MkdirAll(s.StagingDir(releaseID), 0o755)
}

func (s *ReleaseStorage) EnsurePublishedDir(petID, releaseID string) error {
	return os.MkdirAll(s.PublishedDir(petID, releaseID), 0o755)
}

func (s *ReleaseStorage) RemoveStagingDir(releaseID string) error {
	return removeTree(s.StagingDir(releaseID))
}

func (s *ReleaseStorage) RemovePublishedDir(petID, releaseID string) error {
	return removeTree(s.PublishedDir(petID, releaseID))
}

func (s *ReleaseStorage) RemoveInstallDir(installationID string) error {
	return removeTree(s.InstallDir(installationID))
}

func (s *ReleaseStorage) MoveStagingToPublished(petID, releaseID string) error {
	staging := s.StagingDir(releaseID)
	published := s.PublishedDir(petID, releaseID)
	parent := filepath.Dir(published)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create published parent dir: %w", err)
	}
	if err := os.Rename(staging, published); err != nil {
		return fmt.Errorf("atomic move staging to published: %w", err)
	}
	return nil
}

func (s *ReleaseStorage) MoveInstallToTrash(installationID string) error {
	install := s.InstallDir(installationID)
	trash := s.TrashDir(installationID)
	if _, err := os.Stat(install); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(trash), 0o755); err != nil {
		return fmt.Errorf("create trash parent dir: %w", err)
	}
	if err := os.Rename(install, trash); err != nil {
		return fmt.Errorf("atomic move install to trash: %w", err)
	}
	return nil
}

func (s *ReleaseStorage) StageReleaseToInstall(petID, releaseID, installationID string) error {
	published := s.PublishedDir(petID, releaseID)
	install := s.InstallDir(installationID)
	parent := filepath.Dir(install)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create install parent dir: %w", err)
	}
	if err := copyDirContents(published, install); err != nil {
		return fmt.Errorf("copy published to install: %w", err)
	}
	return nil
}

func (s *ReleaseStorage) AtomicSwapInstall(stagingDir, installationID string) error {
	install := s.InstallDir(installationID)
	trash := s.TrashDir(installationID)
	if _, err := os.Stat(install); err == nil {
		if err := os.MkdirAll(filepath.Dir(trash), 0o755); err != nil {
			return fmt.Errorf("create trash parent dir: %w", err)
		}
		if err := os.Rename(install, trash); err != nil {
			return fmt.Errorf("move old install to trash: %w", err)
		}
	}
	parent := filepath.Dir(install)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create install parent dir: %w", err)
	}
	if err := os.Rename(stagingDir, install); err != nil {
		return fmt.Errorf("atomic swap staging to install: %w", err)
	}
	return nil
}

func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDirContents(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFileContents(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFileContents(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func (s *ReleaseStorage) ImportStagingDir() string {
	return filepath.Join(s.BaseDir(), "import-staging")
}

func (s *ReleaseStorage) EnsureImportStagingDir() error {
	return os.MkdirAll(s.ImportStagingDir(), 0o755)
}

func (s *ReleaseStorage) ResolveImportPackageDir(packageDir string) (string, error) {
	if packageDir == "" {
		return "", fmt.Errorf("package dir is empty")
	}
	if filepath.IsAbs(packageDir) {
		return "", fmt.Errorf("absolute package dir is not allowed: %s", packageDir)
	}
	cleaned := filepath.Clean(packageDir)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal is not allowed in package dir: %s", packageDir)
	}
	for _, part := range strings.Split(filepath.ToSlash(cleaned), "/") {
		if part == ".." {
			return "", fmt.Errorf("path traversal is not allowed in package dir: %s", packageDir)
		}
	}
	root := s.ImportStagingDir()
	abs := filepath.Clean(filepath.Join(root, cleaned))
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", fmt.Errorf("invalid package dir: %w", err)
	}
	if rel != "." && strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("package dir escapes import staging root: %s", packageDir)
	}
	return abs, nil
}
