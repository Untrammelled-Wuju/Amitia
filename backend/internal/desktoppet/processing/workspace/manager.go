package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/processing/artifact"
	"github.com/u-ai/backend/internal/desktoppet/security"
)

type WorkspaceManager struct {
	dataDir string
}

func NewWorkspaceManager(dataDir string) *WorkspaceManager {
	return &WorkspaceManager{dataDir: dataDir}
}

type Workspace struct {
	RootPath           string
	ProcessingTaskID   string
	ProcessingActionID string
	AttemptID          string
	CommitID           string
	CellsDir           string
	ForegroundDir      string
	MasksDir           string
	NormalizedDir      string
	FramesDir          string
	TransformsDir      string
	MeasurementsDir    string
	StagingDir         string
	JournalPath        string
	RevisionMetaPath   string
	WorkDir            *artifact.WorkDirectory
}

func (m *WorkspaceManager) workspaceRoot(processingTaskID, processingActionID, attemptID, commitID string) string {
	return filepath.Join(
		m.dataDir,
		"desktop-pets",
		"processing-workspaces",
		processingTaskID,
		processingActionID,
		attemptID,
		commitID,
	)
}

func (m *WorkspaceManager) CreateWorkspace(processingTaskID, processingActionID, attemptID, commitID string) (*Workspace, error) {
	if processingTaskID == "" {
		return nil, fmt.Errorf("workspace: processingTaskID is empty")
	}
	if processingActionID == "" {
		return nil, fmt.Errorf("workspace: processingActionID is empty")
	}
	if attemptID == "" {
		return nil, fmt.Errorf("workspace: attemptID is empty")
	}
	if commitID == "" {
		return nil, fmt.Errorf("workspace: commitID is empty")
	}

	rootPath := m.workspaceRoot(processingTaskID, processingActionID, attemptID, commitID)

	w := &Workspace{
		RootPath:           rootPath,
		ProcessingTaskID:   processingTaskID,
		ProcessingActionID: processingActionID,
		AttemptID:          attemptID,
		CommitID:           commitID,
		CellsDir:           filepath.Join(rootPath, "cells"),
		ForegroundDir:      filepath.Join(rootPath, "foreground"),
		MasksDir:           filepath.Join(rootPath, "masks"),
		NormalizedDir:      filepath.Join(rootPath, "normalized"),
		FramesDir:          filepath.Join(rootPath, "frames"),
		TransformsDir:      filepath.Join(rootPath, "transforms"),
		MeasurementsDir:    filepath.Join(rootPath, "measurements"),
		StagingDir:         filepath.Join(rootPath, "staging"),
		JournalPath:        filepath.Join(rootPath, "publish-journal.json"),
		RevisionMetaPath:   filepath.Join(rootPath, "revision.json"),
	}

	w.WorkDir = &artifact.WorkDirectory{
		RootPath:         w.RootPath,
		ProcessingTaskID: w.ProcessingTaskID,
		ExecutionID:      w.AttemptID,
		ActionKey:        w.ProcessingActionID,
		RevisionID:       w.CommitID,
		CellsDir:         w.CellsDir,
		ForegroundDir:    w.ForegroundDir,
		MasksDir:         w.MasksDir,
		NormalizedDir:    w.NormalizedDir,
		FramesDir:        w.FramesDir,
		TransformsDir:    w.TransformsDir,
		MeasurementsDir:  w.MeasurementsDir,
		JournalPath:      w.JournalPath,
		RevisionMetaPath: w.RevisionMetaPath,
	}

	dirs := []string{
		w.RootPath,
		w.CellsDir,
		w.ForegroundDir,
		w.MasksDir,
		w.NormalizedDir,
		w.FramesDir,
		w.TransformsDir,
		w.MeasurementsDir,
		w.StagingDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("workspace: create dir %s: %w", d, err)
		}
	}

	return w, nil
}

func (m *WorkspaceManager) CleanupWorkspace(processingTaskID, processingActionID, attemptID, commitID string) error {
	if processingTaskID == "" {
		return fmt.Errorf("workspace: processingTaskID is empty")
	}
	rootPath := m.workspaceRoot(processingTaskID, processingActionID, attemptID, commitID)
	return removeDirWithRetry(rootPath, 5)
}

func (m *WorkspaceManager) CleanupTaskWorkspace(processingTaskID string) error {
	if processingTaskID == "" {
		return fmt.Errorf("workspace: processingTaskID is empty")
	}
	taskDir := filepath.Join(
		m.dataDir,
		"desktop-pets",
		"processing-workspaces",
		processingTaskID,
	)
	return removeDirWithRetry(taskDir, 5)
}

func (m *WorkspaceManager) StagingDir(processingTaskID, processingActionID, attemptID, commitID string) string {
	rootPath := m.workspaceRoot(processingTaskID, processingActionID, attemptID, commitID)
	return filepath.Join(rootPath, "staging")
}

func (m *WorkspaceManager) FinalDir(processingTaskID, processingActionID, revisionID string) string {
	_ = processingActionID
	return filepath.Join(
		m.dataDir,
		"desktop-pets",
		"processing-tasks",
		processingTaskID,
		"revisions",
		revisionID,
	)
}

func (m *WorkspaceManager) AtomicPublish(stagingPath, finalPath string) error {
	if stagingPath == "" {
		return fmt.Errorf("workspace: stagingPath is empty")
	}
	if finalPath == "" {
		return fmt.Errorf("workspace: finalPath is empty")
	}

	if _, err := os.Stat(finalPath); err == nil {
		return fmt.Errorf("workspace: final path already exists: %s", finalPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("workspace: stat final path %s: %w", finalPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return fmt.Errorf("workspace: create final parent dir: %w", err)
	}

	if err := os.Rename(stagingPath, finalPath); err != nil {
		if _, statErr := os.Stat(finalPath); statErr == nil {
			if rmErr := security.RemoveDirNoSymlinks(finalPath); rmErr != nil {
				return fmt.Errorf("workspace: remove existing final path %s: %w", finalPath, rmErr)
			}
		}
		if err := os.Rename(stagingPath, finalPath); err != nil {
			return fmt.Errorf("workspace: atomic publish rename %s -> %s: %w", stagingPath, finalPath, err)
		}
	}

	return nil
}

func removeDirWithRetry(dir string, maxAttempts int) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := security.RemoveDirNoSymlinks(dir); err != nil {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return nil
		}
		lastErr = fmt.Errorf("directory still exists after removal: %s", dir)
		time.Sleep(200 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to remove directory after %d attempts: %s", maxAttempts, dir)
	}
	return lastErr
}
