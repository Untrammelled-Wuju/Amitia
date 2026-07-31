package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrPackageGenerationNotFound = errors.New("package generation not found")
	ErrPackageGenerationConflict = errors.New("package generation conflict")
	ErrPackageGenerationCAS      = errors.New("package generation current compare-and-swap failed")
	ErrPackageGenerationUnsafe   = errors.New("package generation path unsafe")
)

type PackageGenerationCurrent struct {
	ExtensionID  string    `json:"extensionID"`
	GenerationID string    `json:"generationID"`
	Version      string    `json:"version"`
	ArtifactID   string    `json:"artifactID"`
	TreeHash     string    `json:"treeHash"`
	OperationID  string    `json:"operationID"`
	FencingToken int64     `json:"fencingToken"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type PackageGenerationPrepareRequest struct {
	ExtensionID      string
	GenerationID     string
	Version          string
	ArtifactID       string
	OperationID      string
	SourcePath       string
	ExpectedTreeHash string
	FencingToken     int64
}

type PackagePreparedGeneration struct {
	Current        PackageGenerationCurrent
	StagingPath    string
	GenerationPath string
}

type PackageQuarantinedCurrent struct {
	Current PackageGenerationCurrent
	Path    string
}

type PackageGenerationStore struct {
	root string
	mu   sync.Mutex
}

func NewPackageGenerationStore(root string) *PackageGenerationStore {
	return &PackageGenerationStore{root: root}
}

func NewPackageGenerationStoreForArtifacts(store *PackageArtifactStore) *PackageGenerationStore {
	if store == nil {
		return &PackageGenerationStore{}
	}
	return NewPackageGenerationStore(store.root)
}

func (s *PackageGenerationStore) PrepareGeneration(ctx context.Context, request PackageGenerationPrepareRequest) (PackagePreparedGeneration, error) {
	if err := validateGenerationIdentity(request.ExtensionID, request.GenerationID, request.OperationID); err != nil {
		return PackagePreparedGeneration{}, err
	}
	if strings.TrimSpace(request.Version) == "" || strings.TrimSpace(request.ArtifactID) == "" {
		return PackagePreparedGeneration{}, fmt.Errorf("%w: generation metadata incomplete", ErrPackageGenerationUnsafe)
	}
	source, err := filepath.Abs(request.SourcePath)
	if err != nil {
		return PackagePreparedGeneration{}, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return PackagePreparedGeneration{}, err
	}
	if !info.IsDir() {
		return PackagePreparedGeneration{}, fmt.Errorf("generation source is not a directory")
	}
	sourceTreeHash, err := computeGenerationTreeHash(ctx, source)
	if err != nil {
		return PackagePreparedGeneration{}, err
	}
	if request.ExpectedTreeHash != "" && !equalTreeHash(sourceTreeHash, request.ExpectedTreeHash) {
		return PackagePreparedGeneration{}, fmt.Errorf("generation tree hash mismatch")
	}
	staging, generation, err := s.paths(request.ExtensionID, request.GenerationID, request.OperationID)
	if err != nil {
		return PackagePreparedGeneration{}, err
	}
	installationsRoot := filepath.Join(s.root, "installations")
	retainedGenerationRoot := filepath.Join(installationsRoot, safeDirectoryName(request.ExtensionID), "generations")
	if pathWithin(source, installationsRoot) && !pathWithin(source, retainedGenerationRoot) {
		return PackagePreparedGeneration{}, fmt.Errorf("%w: source overlaps installation store", ErrPackageGenerationUnsafe)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, statErr := os.Stat(generation); statErr == nil && existing.IsDir() {
		treeHash, verifyErr := computeGenerationTreeHash(ctx, generation)
		if verifyErr != nil {
			return PackagePreparedGeneration{}, verifyErr
		}
		if !equalTreeHash(treeHash, sourceTreeHash) {
			return PackagePreparedGeneration{}, fmt.Errorf("%w: committed generation tree differs", ErrPackageGenerationConflict)
		}
		current := currentFromPrepare(request, treeHash)
		return PackagePreparedGeneration{Current: current, GenerationPath: generation}, nil
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return PackagePreparedGeneration{}, statErr
	}
	if err := os.MkdirAll(filepath.Dir(staging), 0o700); err != nil {
		return PackagePreparedGeneration{}, err
	}
	if existing, statErr := os.Stat(staging); statErr == nil && existing.IsDir() {
		treeHash, verifyErr := computeGenerationTreeHash(ctx, staging)
		if verifyErr == nil && equalTreeHash(treeHash, sourceTreeHash) {
			return PackagePreparedGeneration{Current: currentFromPrepare(request, treeHash), StagingPath: staging, GenerationPath: generation}, nil
		}
		if verifyErr != nil {
			return PackagePreparedGeneration{}, verifyErr
		}
		return PackagePreparedGeneration{}, fmt.Errorf("%w: staged generation tree differs", ErrPackageGenerationConflict)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return PackagePreparedGeneration{}, statErr
	}
	temp := staging + ".preparing"
	if err := os.RemoveAll(temp); err != nil {
		return PackagePreparedGeneration{}, err
	}
	if err := os.MkdirAll(temp, 0o700); err != nil {
		return PackagePreparedGeneration{}, err
	}
	if err := copyGenerationTree(ctx, source, temp); err != nil {
		os.RemoveAll(temp)
		return PackagePreparedGeneration{}, err
	}
	treeHash, err := computeGenerationTreeHash(ctx, temp)
	if err != nil {
		os.RemoveAll(temp)
		return PackagePreparedGeneration{}, err
	}
	if request.ExpectedTreeHash != "" && !equalTreeHash(treeHash, request.ExpectedTreeHash) {
		os.RemoveAll(temp)
		return PackagePreparedGeneration{}, fmt.Errorf("generation tree hash mismatch")
	}
	if err := syncTree(temp); err != nil {
		os.RemoveAll(temp)
		return PackagePreparedGeneration{}, err
	}
	if err := os.Rename(temp, staging); err != nil {
		os.RemoveAll(temp)
		return PackagePreparedGeneration{}, err
	}
	if err := syncDirectory(filepath.Dir(staging)); err != nil {
		return PackagePreparedGeneration{}, err
	}
	return PackagePreparedGeneration{Current: currentFromPrepare(request, treeHash), StagingPath: staging, GenerationPath: generation}, nil
}

func (s *PackageGenerationStore) CommitGeneration(ctx context.Context, prepared PackagePreparedGeneration) (PackagePreparedGeneration, error) {
	if err := validateCurrent(prepared.Current); err != nil {
		return PackagePreparedGeneration{}, err
	}
	staging, generation, err := s.paths(prepared.Current.ExtensionID, prepared.Current.GenerationID, prepared.Current.OperationID)
	if err != nil {
		return PackagePreparedGeneration{}, err
	}
	if prepared.StagingPath != "" && !samePath(prepared.StagingPath, staging) {
		return PackagePreparedGeneration{}, fmt.Errorf("%w: staging path mismatch", ErrPackageGenerationUnsafe)
	}
	if prepared.GenerationPath != "" && !samePath(prepared.GenerationPath, generation) {
		return PackagePreparedGeneration{}, fmt.Errorf("%w: generation path mismatch", ErrPackageGenerationUnsafe)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if info, statErr := os.Stat(generation); statErr == nil && info.IsDir() {
		treeHash, verifyErr := computeGenerationTreeHash(ctx, generation)
		if verifyErr != nil {
			return PackagePreparedGeneration{}, verifyErr
		}
		if !equalTreeHash(treeHash, prepared.Current.TreeHash) {
			return PackagePreparedGeneration{}, fmt.Errorf("%w: committed generation tree differs", ErrPackageGenerationConflict)
		}
		prepared.StagingPath = ""
		prepared.GenerationPath = generation
		return prepared, nil
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return PackagePreparedGeneration{}, statErr
	}
	treeHash, err := computeGenerationTreeHash(ctx, staging)
	if err != nil {
		if os.IsNotExist(err) {
			return PackagePreparedGeneration{}, ErrPackageGenerationNotFound
		}
		return PackagePreparedGeneration{}, err
	}
	if !equalTreeHash(treeHash, prepared.Current.TreeHash) {
		return PackagePreparedGeneration{}, fmt.Errorf("generation tree hash mismatch")
	}
	if err := syncTree(staging); err != nil {
		return PackagePreparedGeneration{}, err
	}
	if err := os.MkdirAll(filepath.Dir(generation), 0o700); err != nil {
		return PackagePreparedGeneration{}, err
	}
	if err := os.Rename(staging, generation); err != nil {
		return PackagePreparedGeneration{}, err
	}
	if err := syncDirectory(filepath.Dir(generation)); err != nil {
		return PackagePreparedGeneration{}, err
	}
	prepared.StagingPath = ""
	prepared.GenerationPath = generation
	return prepared, nil
}

func (s *PackageGenerationStore) ReadCurrent(extensionID string) (PackageGenerationCurrent, error) {
	if err := validatePathSegment("extension ID", extensionID, true); err != nil {
		return PackageGenerationCurrent{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readCurrentLocked(extensionID)
}

func (s *PackageGenerationStore) SwitchCurrent(extensionID, expectedGenerationID string, next PackageGenerationCurrent) error {
	if extensionID != next.ExtensionID {
		return fmt.Errorf("%w: extension ID mismatch", ErrPackageGenerationUnsafe)
	}
	if err := validateCurrent(next); err != nil {
		return err
	}
	_, generation, err := s.paths(next.ExtensionID, next.GenerationID, next.OperationID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	actualHash, err := computeGenerationTreeHash(context.Background(), generation)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrPackageGenerationNotFound
		}
		return err
	}
	if !equalTreeHash(actualHash, next.TreeHash) {
		return fmt.Errorf("generation tree hash mismatch")
	}
	current, readErr := s.readCurrentLocked(extensionID)
	if readErr != nil && !errors.Is(readErr, ErrPackageGenerationNotFound) {
		return readErr
	}
	if readErr == nil && current.GenerationID == next.GenerationID && current.ArtifactID == next.ArtifactID && equalTreeHash(current.TreeHash, next.TreeHash) && current.OperationID == next.OperationID {
		return nil
	}
	actualGenerationID := ""
	if readErr == nil {
		actualGenerationID = current.GenerationID
	}
	if actualGenerationID != expectedGenerationID {
		return fmt.Errorf("%w: expected %q, found %q", ErrPackageGenerationCAS, expectedGenerationID, actualGenerationID)
	}
	next.UpdatedAt = time.Now().UTC()
	return s.replaceCurrentLocked(extensionID, next)
}

func (s *PackageGenerationStore) VerifyGeneration(ctx context.Context, current PackageGenerationCurrent) error {
	if err := validateCurrent(current); err != nil {
		return err
	}
	_, generation, err := s.paths(current.ExtensionID, current.GenerationID, current.OperationID)
	if err != nil {
		return err
	}
	treeHash, err := computeGenerationTreeHash(ctx, generation)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrPackageGenerationNotFound
		}
		return err
	}
	if !equalTreeHash(treeHash, current.TreeHash) {
		return fmt.Errorf("generation tree hash mismatch")
	}
	return nil
}

func (s *PackageGenerationStore) QuarantineGeneration(ctx context.Context, current PackageGenerationCurrent) (string, error) {
	if err := validateCurrent(current); err != nil {
		return "", err
	}
	_, generation, err := s.paths(current.ExtensionID, current.GenerationID, current.OperationID)
	if err != nil {
		return "", err
	}
	quarantine, err := s.quarantinePath(current)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	active, readErr := s.readCurrentLocked(current.ExtensionID)
	if readErr == nil && active.GenerationID == current.GenerationID {
		return "", fmt.Errorf("%w: active generation cannot be quarantined", ErrPackageGenerationConflict)
	}
	if readErr != nil && !errors.Is(readErr, ErrPackageGenerationNotFound) {
		return "", readErr
	}
	if _, statErr := os.Stat(quarantine); statErr == nil {
		treeHash, verifyErr := computeGenerationTreeHash(ctx, quarantine)
		if verifyErr != nil {
			return "", verifyErr
		}
		if !equalTreeHash(treeHash, current.TreeHash) {
			return "", fmt.Errorf("%w: quarantine tree differs", ErrPackageGenerationConflict)
		}
		return quarantine, nil
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return "", statErr
	}
	if err := s.verifyGenerationPath(ctx, generation, current.TreeHash); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(quarantine), 0o700); err != nil {
		return "", err
	}
	if err := os.Rename(generation, quarantine); err != nil {
		return "", err
	}
	if err := syncDirectory(filepath.Dir(generation)); err != nil {
		return "", err
	}
	if err := syncDirectory(filepath.Dir(quarantine)); err != nil {
		return "", err
	}
	return quarantine, nil
}

func (s *PackageGenerationStore) RestoreQuarantinedGeneration(ctx context.Context, current PackageGenerationCurrent) error {
	if err := validateCurrent(current); err != nil {
		return err
	}
	_, generation, err := s.paths(current.ExtensionID, current.GenerationID, current.OperationID)
	if err != nil {
		return err
	}
	quarantine, err := s.quarantinePath(current)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, statErr := os.Stat(generation); statErr == nil {
		return s.verifyGenerationPath(ctx, generation, current.TreeHash)
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	if err := s.verifyGenerationPath(ctx, quarantine, current.TreeHash); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(generation), 0o700); err != nil {
		return err
	}
	if err := os.Rename(quarantine, generation); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(quarantine)); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(generation))
}

func (s *PackageGenerationStore) QuarantineCurrent(extensionID, expectedGenerationID, operationID string) (PackageQuarantinedCurrent, error) {
	if err := validateGenerationIdentity(extensionID, expectedGenerationID, operationID); err != nil {
		return PackageQuarantinedCurrent{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	currentPath, err := s.statePath(extensionID, "current.json")
	if err != nil {
		return PackageQuarantinedCurrent{}, err
	}
	quarantinePath := filepath.Join(s.root, "quarantine", "current", safeDirectoryName(extensionID), operationID)
	if !pathWithin(quarantinePath, filepath.Join(s.root, "quarantine")) {
		return PackageQuarantinedCurrent{}, ErrPackageGenerationUnsafe
	}
	quarantinedCurrentPath := filepath.Join(quarantinePath, "current.json")
	if _, statErr := os.Stat(currentPath); os.IsNotExist(statErr) {
		current, readErr := readCurrentFile(quarantinedCurrentPath)
		if readErr != nil {
			return PackageQuarantinedCurrent{}, readErr
		}
		if current.GenerationID != expectedGenerationID {
			return PackageQuarantinedCurrent{}, ErrPackageGenerationCAS
		}
		return PackageQuarantinedCurrent{Current: current, Path: quarantinePath}, nil
	} else if statErr != nil {
		return PackageQuarantinedCurrent{}, statErr
	}
	current, err := readCurrentFile(currentPath)
	if err != nil {
		return PackageQuarantinedCurrent{}, err
	}
	if current.GenerationID != expectedGenerationID {
		return PackageQuarantinedCurrent{}, ErrPackageGenerationCAS
	}
	if err := os.MkdirAll(quarantinePath, 0o700); err != nil {
		return PackageQuarantinedCurrent{}, err
	}
	moved := []string{}
	for _, name := range []string{"previous.backup.json", "previous.json", "current.json"} {
		source, pathErr := s.statePath(extensionID, name)
		if pathErr != nil {
			return PackageQuarantinedCurrent{}, pathErr
		}
		if _, statErr := os.Stat(source); os.IsNotExist(statErr) {
			continue
		} else if statErr != nil {
			return PackageQuarantinedCurrent{}, statErr
		}
		if renameErr := os.Rename(source, filepath.Join(quarantinePath, name)); renameErr != nil {
			for index := len(moved) - 1; index >= 0; index-- {
				os.Rename(filepath.Join(quarantinePath, moved[index]), filepath.Join(filepath.Dir(currentPath), moved[index]))
			}
			return PackageQuarantinedCurrent{}, renameErr
		}
		moved = append(moved, name)
	}
	if err := syncDirectory(filepath.Dir(currentPath)); err != nil {
		return PackageQuarantinedCurrent{}, err
	}
	if err := syncDirectory(quarantinePath); err != nil {
		return PackageQuarantinedCurrent{}, err
	}
	return PackageQuarantinedCurrent{Current: current, Path: quarantinePath}, nil
}

func (s *PackageGenerationStore) RestoreQuarantinedCurrent(state PackageQuarantinedCurrent) error {
	if err := validateCurrent(state.Current); err != nil {
		return err
	}
	if !pathWithin(state.Path, filepath.Join(s.root, "quarantine", "current")) {
		return ErrPackageGenerationUnsafe
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	currentPath, err := s.statePath(state.Current.ExtensionID, "current.json")
	if err != nil {
		return err
	}
	if existing, readErr := readCurrentFile(currentPath); readErr == nil {
		if existing.GenerationID == state.Current.GenerationID && existing.OperationID == state.Current.OperationID {
			return nil
		}
		return ErrPackageGenerationCAS
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	if err := os.MkdirAll(filepath.Dir(currentPath), 0o700); err != nil {
		return err
	}
	for _, name := range []string{"previous.backup.json", "previous.json", "current.json"} {
		source := filepath.Join(state.Path, name)
		if _, statErr := os.Stat(source); os.IsNotExist(statErr) {
			continue
		} else if statErr != nil {
			return statErr
		}
		target := filepath.Join(filepath.Dir(currentPath), name)
		if _, statErr := os.Stat(target); statErr == nil {
			return ErrPackageGenerationCAS
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		if err := os.Rename(source, target); err != nil {
			return err
		}
	}
	if err := syncDirectory(filepath.Dir(currentPath)); err != nil {
		return err
	}
	return syncDirectory(state.Path)
}

func (s *PackageGenerationStore) RestoreCurrent(extensionID, expectedGenerationID string) (PackageGenerationCurrent, error) {
	if err := validatePathSegment("extension ID", extensionID, true); err != nil {
		return PackageGenerationCurrent{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.readCurrentLocked(extensionID)
	if err != nil {
		return PackageGenerationCurrent{}, err
	}
	if current.GenerationID != expectedGenerationID {
		return PackageGenerationCurrent{}, fmt.Errorf("%w: expected %q, found %q", ErrPackageGenerationCAS, expectedGenerationID, current.GenerationID)
	}
	previousPath, err := s.statePath(extensionID, "previous.json")
	if err != nil {
		return PackageGenerationCurrent{}, err
	}
	previous, err := readCurrentFile(previousPath)
	if err != nil {
		if os.IsNotExist(err) {
			currentPath, pathErr := s.statePath(extensionID, "current.json")
			if pathErr != nil {
				return PackageGenerationCurrent{}, pathErr
			}
			if removeErr := os.Remove(currentPath); removeErr != nil && !os.IsNotExist(removeErr) {
				return PackageGenerationCurrent{}, removeErr
			}
			if syncErr := syncDirectory(filepath.Dir(currentPath)); syncErr != nil {
				return PackageGenerationCurrent{}, syncErr
			}
			return PackageGenerationCurrent{}, nil
		}
		return PackageGenerationCurrent{}, err
	}
	if err := validateCurrent(previous); err != nil {
		return PackageGenerationCurrent{}, err
	}
	if err := s.verifyCurrentGeneration(context.Background(), previous); err != nil {
		return PackageGenerationCurrent{}, err
	}
	previous.UpdatedAt = time.Now().UTC()
	if err := s.replaceCurrentLocked(extensionID, previous); err != nil {
		return PackageGenerationCurrent{}, err
	}
	return previous, nil
}

func (s *PackageGenerationStore) readCurrentLocked(extensionID string) (PackageGenerationCurrent, error) {
	currentPath, err := s.statePath(extensionID, "current.json")
	if err != nil {
		return PackageGenerationCurrent{}, err
	}
	current, err := readCurrentFile(currentPath)
	if err == nil {
		if validateErr := validateCurrent(current); validateErr != nil {
			return PackageGenerationCurrent{}, validateErr
		}
		if current.ExtensionID != extensionID {
			return PackageGenerationCurrent{}, fmt.Errorf("%w: current extension mismatch", ErrPackageGenerationConflict)
		}
		previousPath, pathErr := s.statePath(extensionID, "previous.json")
		if pathErr != nil {
			return PackageGenerationCurrent{}, pathErr
		}
		backupPath, pathErr := s.statePath(extensionID, "previous.backup.json")
		if pathErr != nil {
			return PackageGenerationCurrent{}, pathErr
		}
		if _, previousErr := os.Stat(previousPath); os.IsNotExist(previousErr) {
			if _, backupErr := os.Stat(backupPath); backupErr == nil {
				if renameErr := os.Rename(backupPath, previousPath); renameErr != nil {
					return PackageGenerationCurrent{}, renameErr
				}
				if syncErr := syncDirectory(filepath.Dir(currentPath)); syncErr != nil {
					return PackageGenerationCurrent{}, syncErr
				}
			}
		}
		return current, nil
	}
	if !os.IsNotExist(err) {
		return PackageGenerationCurrent{}, err
	}
	previousPath, pathErr := s.statePath(extensionID, "previous.json")
	if pathErr != nil {
		return PackageGenerationCurrent{}, pathErr
	}
	previous, previousErr := readCurrentFile(previousPath)
	if previousErr != nil {
		if os.IsNotExist(previousErr) {
			return PackageGenerationCurrent{}, ErrPackageGenerationNotFound
		}
		return PackageGenerationCurrent{}, previousErr
	}
	if validateErr := validateCurrent(previous); validateErr != nil {
		return PackageGenerationCurrent{}, validateErr
	}
	if previous.ExtensionID != extensionID {
		return PackageGenerationCurrent{}, fmt.Errorf("%w: previous extension mismatch", ErrPackageGenerationConflict)
	}
	if verifyErr := s.verifyCurrentGeneration(context.Background(), previous); verifyErr != nil {
		return PackageGenerationCurrent{}, verifyErr
	}
	if renameErr := os.Rename(previousPath, currentPath); renameErr != nil {
		return PackageGenerationCurrent{}, renameErr
	}
	backupPath, pathErr := s.statePath(extensionID, "previous.backup.json")
	if pathErr != nil {
		return PackageGenerationCurrent{}, pathErr
	}
	if _, backupErr := os.Stat(backupPath); backupErr == nil {
		if renameErr := os.Rename(backupPath, previousPath); renameErr != nil {
			return PackageGenerationCurrent{}, renameErr
		}
	}
	if syncErr := syncDirectory(filepath.Dir(currentPath)); syncErr != nil {
		return PackageGenerationCurrent{}, syncErr
	}
	return previous, nil
}

func (s *PackageGenerationStore) replaceCurrentLocked(extensionID string, current PackageGenerationCurrent) error {
	currentPath, err := s.statePath(extensionID, "current.json")
	if err != nil {
		return err
	}
	previousPath, err := s.statePath(extensionID, "previous.json")
	if err != nil {
		return err
	}
	backupPath, err := s.statePath(extensionID, "previous.backup.json")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(currentPath), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(current)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(currentPath), ".current-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	keep := false
	defer func() {
		temp.Close()
		if !keep {
			os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if _, err := os.Stat(currentPath); err == nil {
			if err := preserveCurrentFile(currentPath, previousPath); err != nil {
				return err
			}
			if err := syncDirectory(filepath.Dir(currentPath)); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(tempPath, currentPath); err != nil {
			return err
		}
		keep = true
		return syncDirectory(filepath.Dir(currentPath))
	}
	if _, err := os.Stat(currentPath); err == nil {
		if removeErr := os.Remove(backupPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
		hadPrevious := false
		if _, previousErr := os.Stat(previousPath); previousErr == nil {
			if renameErr := os.Rename(previousPath, backupPath); renameErr != nil {
				return renameErr
			}
			hadPrevious = true
		} else if !os.IsNotExist(previousErr) {
			return previousErr
		}
		if renameErr := os.Rename(currentPath, previousPath); renameErr != nil {
			if hadPrevious {
				os.Rename(backupPath, previousPath)
			}
			return renameErr
		}
		if syncErr := syncDirectory(filepath.Dir(currentPath)); syncErr != nil {
			os.Rename(previousPath, currentPath)
			if hadPrevious {
				os.Rename(backupPath, previousPath)
			}
			return syncErr
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tempPath, currentPath); err != nil {
		if _, restoreErr := os.Stat(previousPath); restoreErr == nil {
			os.Rename(previousPath, currentPath)
			if _, backupErr := os.Stat(backupPath); backupErr == nil {
				os.Rename(backupPath, previousPath)
			}
			syncDirectory(filepath.Dir(currentPath))
		}
		return err
	}
	keep = true
	if err := syncDirectory(filepath.Dir(currentPath)); err != nil {
		return err
	}
	if removeErr := os.Remove(backupPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return removeErr
	}
	return syncDirectory(filepath.Dir(currentPath))
}

func preserveCurrentFile(currentPath, previousPath string) error {
	data, err := os.ReadFile(currentPath)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(previousPath), ".previous-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	keep := false
	defer func() {
		temp.Close()
		if !keep {
			os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, previousPath); err != nil {
		return err
	}
	keep = true
	return nil
}

func (s *PackageGenerationStore) paths(extensionID, generationID, operationID string) (string, string, error) {
	if err := s.validateRoot(); err != nil {
		return "", "", err
	}
	if err := validateGenerationIdentity(extensionID, generationID, operationID); err != nil {
		return "", "", err
	}
	base := filepath.Join(s.root, "installations", safeDirectoryName(extensionID))
	staging := filepath.Join(base, "staging", operationID)
	generation := filepath.Join(base, "generations", generationID)
	if !pathWithin(staging, filepath.Join(s.root, "installations")) || !pathWithin(generation, filepath.Join(s.root, "installations")) {
		return "", "", ErrPackageGenerationUnsafe
	}
	return staging, generation, nil
}

func (s *PackageGenerationStore) statePath(extensionID, name string) (string, error) {
	if err := s.validateRoot(); err != nil {
		return "", err
	}
	if err := validatePathSegment("extension ID", extensionID, true); err != nil {
		return "", err
	}
	path := filepath.Join(s.root, "installations", safeDirectoryName(extensionID), name)
	if !pathWithin(path, filepath.Join(s.root, "installations")) {
		return "", ErrPackageGenerationUnsafe
	}
	return path, nil
}

func (s *PackageGenerationStore) quarantinePath(current PackageGenerationCurrent) (string, error) {
	if err := s.validateRoot(); err != nil {
		return "", err
	}
	if err := validateCurrent(current); err != nil {
		return "", err
	}
	path := filepath.Join(s.root, "quarantine", "generations", safeDirectoryName(current.ExtensionID), current.GenerationID+"-"+current.OperationID)
	if !pathWithin(path, filepath.Join(s.root, "quarantine")) {
		return "", ErrPackageGenerationUnsafe
	}
	return path, nil
}

func (s *PackageGenerationStore) validateRoot() error {
	if strings.TrimSpace(s.root) == "" {
		return fmt.Errorf("%w: generation store root empty", ErrPackageGenerationUnsafe)
	}
	return nil
}

func (s *PackageGenerationStore) verifyCurrentGeneration(ctx context.Context, current PackageGenerationCurrent) error {
	_, generation, err := s.paths(current.ExtensionID, current.GenerationID, current.OperationID)
	if err != nil {
		return err
	}
	return s.verifyGenerationPath(ctx, generation, current.TreeHash)
}

func (s *PackageGenerationStore) verifyGenerationPath(ctx context.Context, path, expectedHash string) error {
	hash, err := computeGenerationTreeHash(ctx, path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrPackageGenerationNotFound
		}
		return err
	}
	if !equalTreeHash(hash, expectedHash) {
		return fmt.Errorf("generation tree hash mismatch")
	}
	return nil
}

func currentFromPrepare(request PackageGenerationPrepareRequest, treeHash string) PackageGenerationCurrent {
	return PackageGenerationCurrent{ExtensionID: request.ExtensionID, GenerationID: request.GenerationID, Version: request.Version, ArtifactID: request.ArtifactID, TreeHash: treeHash, OperationID: request.OperationID, FencingToken: request.FencingToken, UpdatedAt: time.Now().UTC()}
}

func validateCurrent(current PackageGenerationCurrent) error {
	if err := validateGenerationIdentity(current.ExtensionID, current.GenerationID, current.OperationID); err != nil {
		return err
	}
	if strings.TrimSpace(current.Version) == "" || strings.TrimSpace(current.ArtifactID) == "" || strings.TrimSpace(current.TreeHash) == "" {
		return fmt.Errorf("%w: current metadata incomplete", ErrPackageGenerationUnsafe)
	}
	return nil
}

func validateGenerationIdentity(extensionID, generationID, operationID string) error {
	if err := validatePathSegment("extension ID", extensionID, true); err != nil {
		return err
	}
	if err := validatePathSegment("generation ID", generationID, false); err != nil {
		return err
	}
	return validatePathSegment("operation ID", operationID, false)
}

func validatePathSegment(label, value string, extension bool) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed != value || value == "." || value == ".." || strings.Contains(value, "..") || strings.ContainsAny(value, ":\x00") || filepath.VolumeName(value) != "" {
		return fmt.Errorf("%w: invalid %s", ErrPackageGenerationUnsafe, label)
	}
	if !extension && strings.ContainsAny(value, "/\\") {
		return fmt.Errorf("%w: invalid %s", ErrPackageGenerationUnsafe, label)
	}
	if strings.HasSuffix(value, ".") || strings.HasSuffix(value, " ") {
		return fmt.Errorf("%w: invalid %s", ErrPackageGenerationUnsafe, label)
	}
	reserved := strings.ToUpper(strings.SplitN(value, ".", 2)[0])
	if reserved == "CON" || reserved == "PRN" || reserved == "AUX" || reserved == "NUL" || reserved == "COM1" || reserved == "COM2" || reserved == "COM3" || reserved == "COM4" || reserved == "COM5" || reserved == "COM6" || reserved == "COM7" || reserved == "COM8" || reserved == "COM9" || reserved == "LPT1" || reserved == "LPT2" || reserved == "LPT3" || reserved == "LPT4" || reserved == "LPT5" || reserved == "LPT6" || reserved == "LPT7" || reserved == "LPT8" || reserved == "LPT9" {
		return fmt.Errorf("%w: invalid %s", ErrPackageGenerationUnsafe, label)
	}
	for _, r := range value {
		if r < 0x20 {
			return fmt.Errorf("%w: invalid %s", ErrPackageGenerationUnsafe, label)
		}
	}
	if !extension && safeDirectoryName(value) != value {
		return fmt.Errorf("%w: invalid %s", ErrPackageGenerationUnsafe, label)
	}
	return nil
}

func copyGenerationTree(ctx context.Context, source, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("%w: unsupported source entry", ErrPackageGenerationUnsafe)
		}
		target := filepath.Join(destination, relative)
		if !pathWithin(target, destination) {
			return ErrPackageGenerationUnsafe
		}
		if info.IsDir() {
			return os.Mkdir(target, info.Mode().Perm())
		}
		return copyGenerationFile(path, target, info.Mode().Perm())
	})
}

func copyGenerationFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		output.Close()
		if !ok {
			os.Remove(destination)
		}
	}()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func computeGenerationTreeHash(ctx context.Context, root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("generation path is not a directory")
	}
	type entry struct {
		path string
		hash string
	}
	entries := []entry{}
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root || info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: unsupported generation entry", ErrPackageGenerationUnsafe)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{path: filepath.ToSlash(relative), hash: hex.EncodeToString(hash.Sum(nil))})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	hash := sha256.New()
	for _, entry := range entries {
		hash.Write([]byte(entry.path))
		hash.Write([]byte{0})
		hash.Write([]byte(entry.hash))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func syncTree(root string) error {
	directories := []string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			directories = append(directories, path)
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: unsupported generation entry", ErrPackageGenerationUnsafe)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		syncErr := file.Sync()
		closeErr := file.Close()
		if syncErr != nil {
			if runtime.GOOS == "windows" && (errors.Is(syncErr, os.ErrPermission) || errors.Is(syncErr, os.ErrInvalid)) {
				return closeErr
			}
			return syncErr
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := syncDirectory(directories[index]); err != nil {
			return err
		}
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		if runtime.GOOS == "windows" && (errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrInvalid)) {
			return nil
		}
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	if err != nil {
		if runtime.GOOS == "windows" && (errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrInvalid)) {
			return closeErr
		}
		return err
	}
	return closeErr
}

func readCurrentFile(path string) (PackageGenerationCurrent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PackageGenerationCurrent{}, err
	}
	var current PackageGenerationCurrent
	if err := json.Unmarshal(data, &current); err != nil {
		return PackageGenerationCurrent{}, err
	}
	return current, nil
}

func pathWithin(path, root string) bool {
	absolutePath, pathErr := filepath.Abs(path)
	absoluteRoot, rootErr := filepath.Abs(root)
	if pathErr != nil || rootErr != nil {
		return false
	}
	relative, err := filepath.Rel(absoluteRoot, absolutePath)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func samePath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(leftAbsolute), filepath.Clean(rightAbsolute))
	}
	return filepath.Clean(leftAbsolute) == filepath.Clean(rightAbsolute)
}

func equalTreeHash(left, right string) bool {
	return strings.EqualFold(strings.TrimPrefix(left, "sha256:"), strings.TrimPrefix(right, "sha256:"))
}
