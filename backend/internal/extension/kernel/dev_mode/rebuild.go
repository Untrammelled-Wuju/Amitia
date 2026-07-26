package dev_mode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type RevisionID string

type Revision struct {
	RevisionID  RevisionID
	WorkspaceID WorkspaceID
	ManifestHash string
	SourceHash   string
	BuiltAt      time.Time
	BuildDurationMs int64
	ArtifactPath string
	Errors       []BuildError
	Warnings     []string
	Status       RevisionStatus
}

type RevisionStatus string

const (
	RevisionStatusBuilding  RevisionStatus = "building"
	RevisionStatusSucceeded RevisionStatus = "succeeded"
	RevisionStatusFailed    RevisionStatus = "failed"
	RevisionStatusStale     RevisionStatus = "stale"
)

type BuildError struct {
	File    string
	Line    int
	Column  int
	Message string
	Code    string
}

type BuildOptions struct {
	Watch            bool
	SourceMap        bool
	Deterministic    bool
	IncludeResources bool
	OutDir           string
}

type RebuildPipeline struct {
	mu         sync.Mutex
	registry   *WorkspaceRegistry
	revisions  map[WorkspaceID]RevisionID
	building   map[WorkspaceID]bool
	history    map[WorkspaceID][]Revision
	maxHistory int
}

func NewRebuildPipeline(registry *WorkspaceRegistry) *RebuildPipeline {
	return &RebuildPipeline{
		registry:   registry,
		revisions:  make(map[WorkspaceID]RevisionID),
		building:   make(map[WorkspaceID]bool),
		history:    make(map[WorkspaceID][]Revision),
		maxHistory: 10,
	}
}

var (
	ErrBuildInProgress = errors.New("dev_mode: build already in progress")
	ErrBuildFailed     = errors.New("dev_mode: build failed")
	ErrNoSourceFiles   = errors.New("dev_mode: no source files found")
)

func (p *RebuildPipeline) Build(ctx context.Context, id WorkspaceID, opts BuildOptions) (*Revision, error) {
	p.mu.Lock()
	if p.building[id] {
		p.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrBuildInProgress, id)
	}
	p.building[id] = true
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.building, id)
		p.mu.Unlock()
	}()

	ws, err := p.registry.Get(id)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	revID := newRevisionID(ws, start)
	rev := Revision{
		RevisionID: revID,
		WorkspaceID: id,
		Status:     RevisionStatusBuilding,
		BuiltAt:    start,
	}

	manifestHash, err := hashFile(ws.ManifestPath)
	if err != nil {
		rev.Status = RevisionStatusFailed
		rev.Errors = append(rev.Errors, BuildError{
			File:    ws.ManifestPath,
			Message: err.Error(),
			Code:    "manifest_hash_failed",
		})
		p.commitRevision(id, rev)
		return &rev, fmt.Errorf("%w: %v", ErrBuildFailed, err)
	}
	rev.ManifestHash = manifestHash

	sourceHash, sourceFiles, err := hashSourceTree(ws.PathReference)
	if err != nil {
		rev.Status = RevisionStatusFailed
		rev.Errors = append(rev.Errors, BuildError{
			Message: err.Error(),
			Code:    "source_hash_failed",
		})
		p.commitRevision(id, rev)
		return &rev, fmt.Errorf("%w: %v", ErrBuildFailed, err)
	}
	if len(sourceFiles) == 0 {
		rev.Status = RevisionStatusFailed
		rev.Errors = append(rev.Errors, BuildError{
			Message: "no source files found in workspace",
			Code:    "no_source_files",
		})
		p.commitRevision(id, rev)
		return &rev, ErrNoSourceFiles
	}
	rev.SourceHash = sourceHash

	if opts.OutDir == "" {
		opts.OutDir = filepath.Join(ws.PathReference, "dist")
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		rev.Status = RevisionStatusFailed
		rev.Errors = append(rev.Errors, BuildError{
			File:    opts.OutDir,
			Message: err.Error(),
			Code:    "out_dir_create_failed",
		})
		p.commitRevision(id, rev)
		return &rev, fmt.Errorf("%w: %v", ErrBuildFailed, err)
	}

	for _, src := range sourceFiles {
		if err := copySourceToDist(src, opts.OutDir); err != nil {
			rev.Warnings = append(rev.Warnings, fmt.Sprintf("copy failed for %s: %v", src, err))
		}
	}
	rev.ArtifactPath = opts.OutDir

	rev.BuildDurationMs = time.Since(start).Milliseconds()
	rev.Status = RevisionStatusSucceeded

	if err := p.registry.SetCurrentRevision(id, revID); err != nil {
		return &rev, err
	}
	if err := p.registry.UpdateStatus(id, WorkspaceStatusReady, ""); err != nil {
		return &rev, err
	}

	p.commitRevision(id, rev)
	return &rev, nil
}

func (p *RebuildPipeline) commitRevision(id WorkspaceID, rev Revision) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.revisions[id] = rev.RevisionID
	history := p.history[id]
	history = append(history, rev)
	if len(history) > p.maxHistory {
		history = history[len(history)-p.maxHistory:]
	}
	p.history[id] = history
}

func (p *RebuildPipeline) CurrentRevision(id WorkspaceID) (Revision, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	revID, ok := p.revisions[id]
	if !ok {
		return Revision{}, false
	}
	for _, rev := range p.history[id] {
		if rev.RevisionID == revID {
			return rev, true
		}
	}
	return Revision{}, false
}

func (p *RebuildPipeline) History(id WorkspaceID) []Revision {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Revision, len(p.history[id]))
	copy(out, p.history[id])
	return out
}

func (p *RebuildPipeline) MarkStale(id WorkspaceID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	revID, ok := p.revisions[id]
	if !ok {
		return nil
	}
	history := p.history[id]
	for i := range history {
		if history[i].RevisionID == revID {
			history[i].Status = RevisionStatusStale
			break
		}
	}
	return nil
}

func newRevisionID(ws *DevelopmentWorkspace, t time.Time) RevisionID {
	seed := fmt.Sprintf("%s|%s|%d", ws.WorkspaceID, ws.ExtensionID, t.UnixNano())
	sum := sha256.Sum256([]byte(seed))
	return RevisionID(hex.EncodeToString(sum[:8]))
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func hashSourceTree(root string) (string, []string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			rel, _ := filepath.Rel(root, path)
			if rel == "node_modules" || rel == "dist" || rel == "package" || rel == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".mjs" || ext == ".cjs" || ext == ".json" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	if len(files) == 0 {
		return "", nil, nil
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		h.Write([]byte(rel))
		h.Write([]byte{0})
		data, err := os.ReadFile(f)
		if err != nil {
			return "", nil, err
		}
		h.Write(data)
		h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum), files, nil
}

func copySourceToDist(src, outDir string) error {
	rel := filepath.Base(src)
	dst := filepath.Join(outDir, rel)
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
