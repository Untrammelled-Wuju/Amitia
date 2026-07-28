package dev_mode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RevisionID string

type Revision struct {
	RevisionID      RevisionID
	WorkspaceID     WorkspaceID
	ManifestHash    string
	SourceHash      string
	BuiltAt         time.Time
	BuildDurationMs int64
	ArtifactPath    string
	Errors          []BuildError
	Warnings        []string
	Status          RevisionStatus
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
	nodePath   string
	tscPath    string
}

func NewRebuildPipeline(nodePath string) *RebuildPipeline {
	if nodePath == "" {
		nodePath = "node"
	}
	return &RebuildPipeline{
		revisions:  make(map[WorkspaceID]RevisionID),
		building:   make(map[WorkspaceID]bool),
		history:    make(map[WorkspaceID][]Revision),
		maxHistory: 10,
		nodePath:   nodePath,
	}
}

func (p *RebuildPipeline) WithRegistry(r *WorkspaceRegistry) *RebuildPipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.registry = r
	return p
}

func (p *RebuildPipeline) SetTscPath(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tscPath = path
}

var (
	ErrBuildInProgress   = errors.New("dev_mode: build already in progress")
	ErrBuildFailed       = errors.New("dev_mode: build failed")
	ErrNoSourceFiles     = errors.New("dev_mode: no source files found")
	ErrRegistryNotConfigured = errors.New("dev_mode: registry not configured")
)

func (p *RebuildPipeline) Build(ctx context.Context, id WorkspaceID, opts BuildOptions) (*Revision, error) {
	p.mu.Lock()
	if p.building[id] {
		p.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrBuildInProgress, id)
	}
	p.building[id] = true
	registry := p.registry
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.building, id)
		p.mu.Unlock()
	}()

	if registry == nil {
		return nil, ErrRegistryNotConfigured
	}

	ws, err := registry.Get(id)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	revID := newRevisionID(ws, start)
	rev := Revision{
		RevisionID:  revID,
		WorkspaceID: id,
		Status:      RevisionStatusBuilding,
		BuiltAt:     start,
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

	tsErrors := p.compileTypeScript(ctx, ws.PathReference, opts.OutDir)
	if len(tsErrors) > 0 {
		rev.Errors = append(rev.Errors, tsErrors...)
		rev.Status = RevisionStatusFailed
		rev.BuildDurationMs = time.Since(start).Milliseconds()
		rev.ArtifactPath = opts.OutDir
		p.commitRevision(id, rev)
		_ = registry.UpdateStatus(id, WorkspaceStatusFailed, fmt.Sprintf("typescript: %d errors", len(tsErrors)))
		return &rev, fmt.Errorf("%w: typescript compilation produced %d errors", ErrBuildFailed, len(tsErrors))
	}

	rev.ArtifactPath = opts.OutDir
	rev.BuildDurationMs = time.Since(start).Milliseconds()
	rev.Status = RevisionStatusSucceeded

	if err := registry.SetCurrentRevision(id, revID); err != nil {
		return &rev, err
	}
	if err := registry.UpdateStatus(id, WorkspaceStatusReady, ""); err != nil {
		return &rev, err
	}

	p.commitRevision(id, rev)
	return &rev, nil
}

func (p *RebuildPipeline) compileTypeScript(ctx context.Context, workspacePath, outputPath string) []BuildError {
	tsconfig := filepath.Join(workspacePath, "tsconfig.json")
	if _, err := os.Stat(tsconfig); err != nil {
		return nil
	}

	p.mu.Lock()
	tsc := p.tscPath
	nodeBin := p.nodePath
	p.mu.Unlock()

	if tsc == "" {
		candidate := filepath.Join(workspacePath, "node_modules", "typescript", "bin", "tsc")
		if _, err := os.Stat(candidate); err == nil {
			tsc = candidate
		} else {
			tsc = "tsc"
		}
	}
	if nodeBin == "" {
		nodeBin = "node"
	}

	args := []string{tsc, "--project", workspacePath, "--outDir", outputPath, "--sourceMap"}
	cmd := exec.CommandContext(ctx, nodeBin, args...)
	cmd.Dir = workspacePath
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	runErr := cmd.Run()
	out := combined.String()
	if runErr != nil {
		if errs := parseTSCErrors(out); len(errs) > 0 {
			return errs
		}
		return []BuildError{{
			Message: fmt.Sprintf("tsc invocation failed: %v; output: %s", runErr, strings.TrimSpace(out)),
			Code:    "tsc_invoke_failed",
		}}
	}
	return parseTSCErrors(out)
}

var tscErrorRe = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\):\s+(error|warning)\s+(TS\d+):\s+(.+)$`)

func parseTSCErrors(out string) []BuildError {
	var errs []BuildError
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := tscErrorRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[4] != "error" {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		colNo, _ := strconv.Atoi(m[3])
		errs = append(errs, BuildError{
			File:    m[1],
			Line:    lineNo,
			Column:  colNo,
			Code:    m[5],
			Message: m[6],
		})
	}
	return errs
}

func (p *RebuildPipeline) commitRevision(id WorkspaceID, rev Revision) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if rev.Status != RevisionStatusFailed {
		p.revisions[id] = rev.RevisionID
	}
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
