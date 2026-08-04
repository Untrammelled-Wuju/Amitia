package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"github.com/u-ai/backend/internal/desktoppet/security"
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
		journal := NewJournal(revisionID, journalPath)
		if err := journal.Load(); err != nil {
			return fmt.Errorf("artifact: reconcile revision: load journal: %w", err)
		}
		if journal.IsStageDone("files_published") && !journal.IsStageDone("db_committed") {
			return fmt.Errorf("artifact: reconcile revision: revision %s has files_published but not db_committed", revisionID)
		}
	}

	return nil
}

func (r *Reconciler) CleanOrphanedWorkDirs(maxAge time.Duration) ([]string, error) {
	if maxAge <= 0 {
		return nil, fmt.Errorf("artifact: clean orphaned work dirs: maxAge must be positive")
	}

	workRoot := filepath.Join(r.DataDir, "desktop-pets", "generation-tasks")

	var cleaned []string

	taskDirs, err := os.ReadDir(workRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return cleaned, nil
		}
		return nil, fmt.Errorf("artifact: clean orphaned work dirs: read task dirs: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)

	for _, taskDir := range taskDirs {
		if !taskDir.IsDir() {
			continue
		}
		taskWorkRoot := filepath.Join(workRoot, taskDir.Name(), "processed", "work")
		if _, err := os.Stat(taskWorkRoot); err != nil {
			continue
		}

		err := filepath.Walk(taskWorkRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if path == taskWorkRoot {
				return nil
			}
			if !info.IsDir() {
				return nil
			}

			journalPath := filepath.Join(path, "publish-journal.json")
			if _, err := os.Stat(journalPath); err != nil {
				return nil
			}

			dirModTime := info.ModTime()
			if dirModTime.After(cutoff) {
				return nil
			}

			journal := NewJournal("", journalPath)
			if err := journal.Load(); err != nil {
				return nil
			}

			if journal.IsStageDone("db_committed") {
				if err := security.SafeRemoveTree(path); err == nil {
					cleaned = append(cleaned, path)
				}
				if filepath.Dir(path) != taskWorkRoot {
					return filepath.SkipDir
				}
				return nil
			}

			if journal.IsStageDone("files_published") && dirModTime.Before(cutoff) {
				if err := security.SafeRemoveTree(path); err == nil {
					cleaned = append(cleaned, path)
				}
				if filepath.Dir(path) != taskWorkRoot {
					return filepath.SkipDir
				}
				return nil
			}

			return nil
		})
		if err != nil {
			continue
		}
	}

	return cleaned, nil
}

func (r *Reconciler) scanRevisions(root string, result *ReconcileResult) {
	taskDirs, err := os.ReadDir(root)
	if err != nil {
		if !os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("scan revisions: read %s: %v", root, err))
		}
		return
	}

	for _, taskDir := range taskDirs {
		if !taskDir.IsDir() {
			continue
		}
		versionsDir := filepath.Join(root, taskDir.Name(), "processed", "versions")
		if _, err := os.Stat(versionsDir); err != nil {
			continue
		}
		r.scanVersionDirs(versionsDir, result)
	}
}

func (r *Reconciler) scanVersionDirs(versionsDir string, result *ReconcileResult) {
	versionDirs, err := os.ReadDir(versionsDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("scan versions: read %s: %v", versionsDir, err))
		return
	}

	for _, versionDir := range versionDirs {
		if !versionDir.IsDir() {
			continue
		}
		actionsDir := filepath.Join(versionsDir, versionDir.Name(), "actions")
		if _, err := os.Stat(actionsDir); err != nil {
			continue
		}
		r.scanActionDirs(actionsDir, result)
	}
}

func (r *Reconciler) scanActionDirs(actionsDir string, result *ReconcileResult) {
	actionDirs, err := os.ReadDir(actionsDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("scan actions: read %s: %v", actionsDir, err))
		return
	}

	for _, actionDir := range actionDirs {
		if !actionDir.IsDir() {
			continue
		}
		revisionsDir := filepath.Join(actionsDir, actionDir.Name(), "revisions")
		if _, err := os.Stat(revisionsDir); err != nil {
			continue
		}
		r.scanRevisionDirs(revisionsDir, result)
	}
}

func (r *Reconciler) scanRevisionDirs(revisionsDir string, result *ReconcileResult) {
	revisionDirs, err := os.ReadDir(revisionsDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("scan revisions: read %s: %v", revisionsDir, err))
		return
	}

	for _, revDir := range revisionDirs {
		if !revDir.IsDir() {
			continue
		}
		revPath := filepath.Join(revisionsDir, revDir.Name())
		result.CheckedRevisions++

		manifestPath := filepath.Join(revPath, "revision.json")
		if _, err := os.Stat(manifestPath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("revision %s: manifest missing", revPath))
			continue
		}

		journalPath := filepath.Join(revPath, "publish-journal.json")
		if _, err := os.Stat(journalPath); err == nil {
			journal := NewJournal(revDir.Name(), journalPath)
			if err := journal.Load(); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("revision %s: load journal: %v", revPath, err))
				continue
			}
			if journal.IsStageDone("files_published") && !journal.IsStageDone("db_committed") {
				result.Errors = append(result.Errors,
					fmt.Sprintf("revision %s: files_published but not db_committed", revPath))
			}
		}
	}
}

func (r *Reconciler) scanWorkJournals(root string, result *ReconcileResult) {
	taskDirs, err := os.ReadDir(root)
	if err != nil {
		if !os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("scan work journals: read %s: %v", root, err))
		}
		return
	}

	for _, taskDir := range taskDirs {
		if !taskDir.IsDir() {
			continue
		}
		if strings.HasPrefix(taskDir.Name(), ".") {
			continue
		}
		workRoot := filepath.Join(root, taskDir.Name(), "processed", "work")
		if _, err := os.Stat(workRoot); err != nil {
			continue
		}
		r.scanWorkDirForJournals(workRoot, result)
	}
}

func (r *Reconciler) scanWorkDirForJournals(workRoot string, result *ReconcileResult) {
	err := filepath.Walk(workRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == workRoot {
			return nil
		}
		if !info.IsDir() {
			return nil
		}

		journalPath := filepath.Join(path, "publish-journal.json")
		if _, err := os.Stat(journalPath); err != nil {
			return nil
		}

		journal := NewJournal("", journalPath)
		if err := journal.Load(); err != nil {
			return nil
		}

		result.OrphanedWorkDirs++

		if journal.IsStageDone("files_published") && !journal.IsStageDone("db_committed") {
			result.Errors = append(result.Errors,
				fmt.Sprintf("work dir %s: files_published but not db_committed", path))
		}

		return nil
	})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("scan work journals in %s: %v", workRoot, err))
	}
}
