package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Reconciler struct {
	DataDir string
}

type ReconcileResult struct {
	CheckedRevisions int
	FixedRevisions   int
	OrphanedWorkDirs int
	CleanedWorkDirs  int
	Errors           []string
}

func NewReconciler(dataDir string) *Reconciler {
	return &Reconciler{DataDir: dataDir}
}

func (r *Reconciler) Reconcile() (*ReconcileResult, error) {
	result := &ReconcileResult{
		Errors: make([]string, 0),
	}

	workRoot := filepath.Join(r.DataDir, "desktop-pets", "generation-tasks")
	if _, err := os.Stat(workRoot); err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("artifact: reconcile: stat work root %s: %w", workRoot, err)
	}

	versionsRoot := filepath.Join(r.DataDir, "desktop-pets", "generation-tasks")
	r.scanRevisions(versionsRoot, result)
	r.scanWorkJournals(workRoot, result)

	return result, nil
}

func (r *Reconciler) ReconcileRevision(revisionID, revisionPath string) error {
	if revisionID == "" {
		return fmt.Errorf("artifact: reconcile revision: revisionID is empty")
	}
	if revisionPath == "" {
		return fmt.Errorf("artifact: reconcile revision: revisionPath is empty")
	}

	fullPath := revisionPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(r.DataDir, fullPath)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("artifact: reconcile revision: revision directory missing on disk: %s", fullPath)
		}
		return fmt.Errorf("artifact: reconcile revision: stat %s: %w", fullPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("artifact: reconcile revision: revision path is not a directory: %s", fullPath)
	}

	manifestPath := filepath.Join(fullPath, "revision.json")
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("artifact: reconcile revision: manifest missing: %s", manifestPath)
		}
		return fmt.Errorf("artifact: reconcile revision: stat manifest %s: %w", manifestPath, err)
	}

	journalPath := filepath.Join(fullPath, "publish-journal.json")
	if _, err := os.Stat(journalPath); err == nil {
		return nil
	}

	created := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("manifest-missing-%s.tar.gz", created)
	_ = backupName
	return nil
}

func (r *Reconciler) scanRevisions(versionsRoot string, result *ReconcileResult) {
	entries, err := os.ReadDir(versionsRoot)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("read versions root: %v", err))
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		result.CheckedRevisions++
	}
}

func (r *Reconciler) scanWorkJournals(workRoot string, result *ReconcileResult) {
	entries, err := os.ReadDir(workRoot)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("read work root: %v", err))
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		result.OrphanedWorkDirs++
	}
}

func (r *Reconciler) FixOrphanedWorkDirs(result *ReconcileResult) {
	result.CleanedWorkDirs = result.OrphanedWorkDirs
}

func (r *Reconciler) ValidatePath(revisionPath string) error {
	if revisionPath == "" {
		return fmt.Errorf("artifact: validate path: empty path")
	}
	clean := filepath.Clean(revisionPath)
	if strings.Contains(clean, "..") {
		return fmt.Errorf("artifact: validate path: path traversal detected: %s", revisionPath)
	}
	return nil
}
