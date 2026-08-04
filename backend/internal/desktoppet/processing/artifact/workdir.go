package artifact

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/desktoppet/security"
)

type WorkDirectory struct {
	RootPath         string
	ProcessingTaskID string
	ExecutionID      string
	ActionKey        string
	RevisionID       string
	CellsDir         string
	ForegroundDir    string
	MasksDir         string
	NormalizedDir    string
	FramesDir        string
	TransformsDir    string
	MeasurementsDir  string
	JournalPath      string
	RevisionMetaPath string
}

func NewWorkDirectory(dataDir, processingTaskID, executionID, actionKey, revisionID string) (*WorkDirectory, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("artifact: dataDir is empty")
	}
	if processingTaskID == "" {
		return nil, fmt.Errorf("artifact: processingTaskID is empty")
	}
	if executionID == "" {
		return nil, fmt.Errorf("artifact: executionID is empty")
	}
	if actionKey == "" {
		return nil, fmt.Errorf("artifact: actionKey is empty")
	}
	if revisionID == "" {
		return nil, fmt.Errorf("artifact: revisionID is empty")
	}

	rootPath := filepath.Join(
		dataDir,
		"desktop-pets",
		"generation-tasks",
		processingTaskID,
		"processed",
		"work",
		executionID,
		actionKey,
		revisionID,
	)

	wd := &WorkDirectory{
		RootPath:         rootPath,
		ProcessingTaskID: processingTaskID,
		ExecutionID:      executionID,
		ActionKey:        actionKey,
		RevisionID:       revisionID,
		CellsDir:         filepath.Join(rootPath, "cells"),
		ForegroundDir:    filepath.Join(rootPath, "foreground"),
		MasksDir:         filepath.Join(rootPath, "masks"),
		NormalizedDir:    filepath.Join(rootPath, "normalized"),
		FramesDir:        filepath.Join(rootPath, "frames"),
		TransformsDir:    filepath.Join(rootPath, "transforms"),
		MeasurementsDir:  filepath.Join(rootPath, "measurements"),
		JournalPath:      filepath.Join(rootPath, "publish-journal.json"),
		RevisionMetaPath: filepath.Join(rootPath, "revision.json"),
	}

	return wd, nil
}

func (w *WorkDirectory) Create() error {
	dirs := []string{
		w.RootPath,
		w.CellsDir,
		w.ForegroundDir,
		w.MasksDir,
		w.NormalizedDir,
		w.FramesDir,
		w.TransformsDir,
		w.MeasurementsDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("artifact: create workdir %s: %w", d, err)
		}
	}
	return nil
}

func (w *WorkDirectory) Clean(deleter security.SafeTreeDeleter) error {
	if w.RootPath == "" {
		return fmt.Errorf("artifact: rootPath is empty")
	}
	if deleter == nil {
		return fmt.Errorf("artifact: deleter is nil")
	}
	storageKey := w.ProcessingTaskID + "/processed/work/" + w.ExecutionID + "/" + w.ActionKey + "/" + w.RevisionID
	if err := deleter.SafeDelete(security.RootGenerationArtifacts, storageKey, security.DeleteExpectation{
		EntityType: "processing_attempt",
		EntityID:   w.ExecutionID,
	}); err != nil {
		return fmt.Errorf("artifact: clean workdir %s: %w", w.RootPath, err)
	}
	return nil
}

func (w *WorkDirectory) Exists() bool {
	if w.RootPath == "" {
		return false
	}
	info, err := os.Stat(w.RootPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (w *WorkDirectory) SubdirByKind(kind string) string {
	switch kind {
	case "cell_source":
		return w.CellsDir
	case "foreground":
		return w.ForegroundDir
	case "mask":
		return w.MasksDir
	case "normalized":
		return w.NormalizedDir
	case "frame":
		return w.FramesDir
	case "transform":
		return w.TransformsDir
	case "measurement":
		return w.MeasurementsDir
	default:
		return w.RootPath
	}
}
