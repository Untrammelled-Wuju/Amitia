package git

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/workspace"
)

type GitController struct {
	workspace *workspace.Service
	mounts    *workspace.Registry
	engine    GitEngine
	policy    GitPolicy
	roots     IsolatedRootResolver
	mu        sync.Mutex
}

func NewGitController(
	ws *workspace.Service,
	mounts *workspace.Registry,
	engine GitEngine,
	policy GitPolicy,
	roots IsolatedRootResolver,
) *GitController {
	return &GitController{
		workspace: ws,
		mounts:    mounts,
		engine:    engine,
		policy:    policy,
		roots:     roots,
	}
}

func (c *GitController) resolveRepositoryPath(workspaceURI string) (string, workspace.WorkspaceMount, error) {
	mountID, err := parseMountIDFromURI(workspaceURI)
	if err != nil {
		return "", workspace.WorkspaceMount{}, err
	}
	mount, ok := c.mounts.GetMount(mountID)
	if !ok {
		return "", workspace.WorkspaceMount{}, workspace.ErrMountNotFound
	}
	if mount.Kind != workspace.WorkspaceKindLocal && mount.Kind != workspace.WorkspaceKindIsolated {
		return "", workspace.WorkspaceMount{}, fmt.Errorf("%w: mount is not a git-compatible kind", ErrGitOperationUnsupported)
	}
	root, err := c.roots.ResolveRoot(mount)
	if err != nil {
		return "", workspace.WorkspaceMount{}, err
	}
	return root, mount, nil
}

func (c *GitController) resolveRepoPath(workspaceURI string) (string, error) {
	root, _, err := c.resolveRepositoryPath(workspaceURI)
	return root, err
}

func parseMountIDFromURI(uri string) (workspace.WorkspaceID, error) {
	if !strings.HasPrefix(uri, "amitia://workspace/@") {
		return "", workspace.ErrInvalidURI
	}
	rest := strings.TrimPrefix(uri, "amitia://workspace/@")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", workspace.ErrInvalidURI
	}
	return workspace.WorkspaceID(parts[0]), nil
}

func (c *GitController) Detect(ctx context.Context, workspaceURI string) (*RepositoryState, error) {
	root, err := c.resolveRepoPath(workspaceURI)
	if err != nil {
		return nil, err
	}
	return c.engine.Detect(ctx, root)
}

func (c *GitController) Status(ctx context.Context, workspaceURI string, includeIgnored bool) (*GitStatusResult, error) {
	root, err := c.resolveRepoPath(workspaceURI)
	if err != nil {
		return nil, err
	}
	limit := c.policy.MaxStatusEntries
	result, err := c.engine.Status(ctx, root, includeIgnored, limit)
	if err != nil {
		return nil, err
	}
	result.RepositoryRootURI = workspaceURI
	for i := range result.Entries {
		result.Entries[i].URI = qualifyWorkspacePath(workspaceURI, result.Entries[i].URI)
		if result.Entries[i].OldURI != "" {
			result.Entries[i].OldURI = qualifyWorkspacePath(workspaceURI, result.Entries[i].OldURI)
		}
	}
	return result, nil
}

func (c *GitController) Diff(ctx context.Context, req GitDiffRequest) (*GitDiffResult, error) {
	root, err := c.resolveRepoPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	paths, err := normalizeWorkspacePaths(req.WorkspaceURI, req.Paths)
	if err != nil {
		return nil, err
	}
	opts := DiffOptions{
		Mode:     req.Mode,
		Base:     req.Base,
		Target:   req.Target,
		Paths:    paths,
		MaxBytes: req.MaxBytes,
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = c.policy.MaxDiffBytes
	}
	if err := ValidateRefName(opts.Base); err != nil && opts.Base != "" {
		return nil, err
	}
	if err := ValidateRefName(opts.Target); err != nil && opts.Target != "" {
		return nil, err
	}
	result, err := c.engine.Diff(ctx, root, opts)
	if err != nil {
		return nil, err
	}
	for i := range result.Files {
		result.Files[i].URI = qualifyWorkspacePath(req.WorkspaceURI, result.Files[i].URI)
		if result.Files[i].OldURI != "" {
			result.Files[i].OldURI = qualifyWorkspacePath(req.WorkspaceURI, result.Files[i].OldURI)
		}
	}
	return result, nil
}

func (c *GitController) Log(ctx context.Context, req GitLogRequest) (*GitLogResult, error) {
	root, err := c.resolveRepoPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > c.policy.MaxLogLimit {
		limit = c.policy.MaxLogLimit
	}
	opts := LogOptions{
		Limit: limit,
		Path:  req.Path,
		Ref:   req.Ref,
	}
	if req.Ref != "" {
		if err := ValidateRefName(req.Ref); err != nil {
			return nil, err
		}
	}
	return c.engine.Log(ctx, root, opts)
}

func (c *GitController) Add(ctx context.Context, req GitAddRequest) (*GitAddResult, error) {
	root, _, err := c.resolveRepositoryPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	paths, err := normalizeWorkspacePaths(req.WorkspaceURI, req.Paths)
	if err != nil {
		return nil, err
	}
	if req.All && !req.Force {
		return nil, fmt.Errorf("%w: add all requires explicit force flag", ErrGitAddFailed)
	}
	if req.All {
		for _, p := range paths {
			if p == "" {
				continue
			}
			if strings.HasPrefix(p, "/") {
				return nil, ErrGitPathOutsideRepository
			}
		}
	} else {
		if err := ValidatePaths(paths); err != nil {
			return nil, err
		}
	}
	for _, p := range paths {
		if IsSecretFile(p) {
			return nil, fmt.Errorf("%w: staged path %q matches secret file pattern", ErrGitAddFailed, p)
		}
	}
	opts := AddOptions{
		Paths: paths,
		All:   req.All,
		Force: req.Force,
	}
	staged, err := c.engine.Add(ctx, root, opts)
	if err != nil {
		return nil, err
	}
	return &GitAddResult{Staged: staged}, nil
}

func (c *GitController) Restore(ctx context.Context, req GitRestoreRequest) (*GitRestoreResult, error) {
	root, err := c.resolveRepoPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	paths, err := normalizeWorkspacePaths(req.WorkspaceURI, req.Paths)
	if err != nil {
		return nil, err
	}
	if err := ValidatePaths(paths); err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%w: restore requires explicit paths", ErrGitRestoreFailed)
	}
	opts := RestoreOptions{
		Paths:    paths,
		Source:   req.Source,
		Staged:   req.Staged,
		Worktree: req.Worktree,
	}
	if err := c.engine.Restore(ctx, root, opts); err != nil {
		return nil, err
	}
	restored := make([]string, 0, len(paths))
	for _, path := range paths {
		restored = append(restored, qualifyWorkspacePath(req.WorkspaceURI, path))
	}
	return &GitRestoreResult{Restored: restored}, nil
}

func (c *GitController) Commit(ctx context.Context, req GitCommitRequest) (*GitCommitResult, error) {
	root, err := c.resolveRepoPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	if err := ValidateCommitMessage(req.Message); err != nil {
		return nil, err
	}
	opts := CommitOptions{
		Message: req.Message,
		Author:  req.Author,
	}
	result, err := c.engine.Commit(ctx, root, opts)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GitController) ListBranches(ctx context.Context, workspaceURI string) (*GitBranchListResult, error) {
	root, err := c.resolveRepoPath(workspaceURI)
	if err != nil {
		return nil, err
	}
	return c.engine.ListBranches(ctx, root)
}

func (c *GitController) Checkout(ctx context.Context, req GitCheckoutRequest) (*GitCheckoutResult, error) {
	root, err := c.resolveRepoPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	if req.Branch == "" && !req.Detach {
		return nil, fmt.Errorf("%w: branch name required", ErrGitCheckoutFailed)
	}
	if req.Branch != "" {
		if err := ValidateRefName(req.Branch); err != nil {
			return nil, err
		}
	}
	if req.Force && !c.policy.ForcePushAllowed {
		return nil, fmt.Errorf("%w: force checkout not enabled", ErrGitCheckoutFailed)
	}
	opts := CheckoutOptions{
		Branch:  req.Branch,
		Create:  req.Create,
		FromRef: req.FromRef,
		Detach:  req.Detach,
		Force:   req.Force,
	}
	return c.engine.Checkout(ctx, root, opts)
}

func (c *GitController) Fetch(ctx context.Context, req GitFetchRequest) (*GitFetchResult, error) {
	root, err := c.resolveRepoPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	opts := FetchOptions{
		Remote: req.Remote,
		Depth:  req.Depth,
		Deepen: req.Deepen,
	}
	return c.engine.Fetch(ctx, root, opts)
}

func (c *GitController) Pull(ctx context.Context, req GitPullRequest) (*GitPullResult, error) {
	root, err := c.resolveRepoPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	opts := PullOptions{
		Remote: req.Remote,
		Branch: req.Branch,
	}
	fetched, oldHead, newHead, err := c.engine.PullFastForward(ctx, root, opts)
	if err != nil {
		return nil, err
	}
	return &GitPullResult{
		Fetched:       fetched,
		FastForwarded: newHead != "" && oldHead != newHead,
		OldHead:       oldHead,
		NewHead:       newHead,
	}, nil
}

func (c *GitController) Push(ctx context.Context, req GitPushRequest) (*GitPushResult, error) {
	root, err := c.resolveRepoPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	if req.Remote == "" || req.LocalRef == "" || req.RemoteRef == "" {
		return nil, fmt.Errorf("%w: remote, localRef, and remoteRef are required", ErrGitPushRejected)
	}
	if err := ValidateRefName(req.LocalRef); err != nil {
		return nil, err
	}
	if err := ValidateRefName(req.RemoteRef); err != nil {
		return nil, err
	}
	opts := PushOptions{
		Remote:      req.Remote,
		LocalRef:    req.LocalRef,
		RemoteRef:   req.RemoteRef,
		SetUpstream: req.SetUpstream,
	}
	result, err := c.engine.Push(ctx, root, opts)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GitController) ListRemotes(ctx context.Context, workspaceURI string) ([]GitRemoteInfo, error) {
	root, err := c.resolveRepoPath(workspaceURI)
	if err != nil {
		return nil, err
	}
	remotes, err := c.engine.ListRemotes(ctx, root)
	if err != nil {
		return nil, err
	}
	for i := range remotes {
		remotes[i].FetchURL = SanitizeRemoteURL(remotes[i].FetchURL)
		remotes[i].PushURL = SanitizeRemoteURL(remotes[i].PushURL)
	}
	return remotes, nil
}

type IsolatedCreateResult struct {
	MountID        string `json:"mountId"`
	Name           string `json:"name"`
	RootKey        string `json:"rootKey"`
	RepositoryPath string `json:"repositoryPath"`
	Branch         string `json:"branch"`
	Detached       bool   `json:"detached"`
	Clean          bool   `json:"clean"`
	CreatedAt      string `json:"createdAt"`
}

func (c *GitController) CreateIsolated(ctx context.Context, req IsolatedCreateRequest) (*IsolatedCreateResult, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrIsolatedCreateFailed)
	}
	mode := GitIsolationMode(strings.TrimSpace(req.Mode))
	if mode == "" {
		if req.GitRemote != nil {
			mode = GitIsolationModeClone
		} else {
			mode = GitIsolationModeSnapshot
		}
	}
	if mode != GitIsolationModeClone && mode != GitIsolationModeSnapshot {
		return nil, fmt.Errorf("%w: mode %q not supported", ErrIsolatedCreateFailed, mode)
	}

	dataRoot := c.roots.DataRoot()
	opID := uuid.NewString()
	stagingRoot := filepath.Join(dataRoot, "workspaces", IsolatedRootPrefix, StagingDirName, opID)
	if err := os.MkdirAll(filepath.Dir(stagingRoot), 0o755); err != nil {
		return nil, fmt.Errorf("%w: cannot create staging parent: %v", ErrIsolatedCreateFailed, err)
	}
	_ = os.RemoveAll(stagingRoot)

	var source *GitSourceSpec
	branch := ""
	detached := false
	clean := true

	switch mode {
	case GitIsolationModeClone:
		if req.GitRemote == nil || strings.TrimSpace(req.GitRemote.URL) == "" {
			return nil, fmt.Errorf("%w: gitRemote.url is required for clone mode", ErrIsolatedCreateFailed)
		}
		if err := ValidateRemoteURL(req.GitRemote.URL); err != nil {
			return nil, err
		}
		cloneResult, err := c.engine.Clone(ctx, stagingRoot, CloneOptions{URL: req.GitRemote.URL, Ref: req.Ref, Depth: req.Depth})
		if err != nil {
			_ = os.RemoveAll(stagingRoot)
			return nil, fmt.Errorf("%w: clone failed: %v", ErrIsolatedCreateFailed, err)
		}
		if hasSym, _ := c.engine.HasSymlinkEntries(ctx, stagingRoot); hasSym && c.policy.SymlinkPolicy == "reject_repository_with_symlink" {
			_ = os.RemoveAll(stagingRoot)
			return nil, fmt.Errorf("%w: repository contains symlink entries", ErrGitSymlinkUnsupported)
		}
		branch = cloneResult.Branch
		source = &GitSourceSpec{Type: "git", URI: req.GitRemote.URL, Ref: req.Ref, Depth: req.Depth, RemoteID: req.GitRemote.RemoteID}
	case GitIsolationModeSnapshot:
		if strings.TrimSpace(req.SourceWorkspaceURI) == "" {
			return nil, fmt.Errorf("%w: sourceWorkspaceUri is required for snapshot mode", ErrIsolatedCreateFailed)
		}
		sourceRoot, _, err := c.resolveRepositoryPath(req.SourceWorkspaceURI)
		if err != nil {
			return nil, fmt.Errorf("%w: snapshot source: %v", ErrIsolatedCreateFailed, err)
		}
		if err := copySnapshotTree(sourceRoot, stagingRoot, c.policy.SymlinkPolicy); err != nil {
			_ = os.RemoveAll(stagingRoot)
			return nil, err
		}
		source = &GitSourceSpec{Type: "workspace", URI: req.SourceWorkspaceURI, Ref: req.Ref}
		if state, detectErr := c.engine.Detect(ctx, stagingRoot); detectErr == nil && state != nil {
			branch, detached, clean = state.Branch, state.Detached, state.Clean
		}
	}

	rootID := uuid.NewString()
	rootKey := filepath.Join(IsolatedRootPrefix, rootID)
	isolatedRoot := filepath.Join(dataRoot, "workspaces", rootKey)
	if err := os.MkdirAll(filepath.Dir(isolatedRoot), 0o755); err != nil {
		_ = os.RemoveAll(stagingRoot)
		return nil, fmt.Errorf("%w: create isolated parent: %v", ErrIsolatedCreateFailed, err)
	}
	if err := os.Rename(stagingRoot, isolatedRoot); err != nil {
		_ = os.RemoveAll(stagingRoot)
		return nil, fmt.Errorf("%w: cannot move to isolated root: %v", ErrIsolatedCreateFailed, err)
	}

	config := &GitMountConfig{Mode: mode, RootKey: rootKey, Source: source, CreatedBy: "user", ExpiresAt: parseIsolationLifetime(req.Lifetime)}
	configJSON, _ := json.Marshal(config)
	mount, err := c.workspace.RegisterIsolatedMount(ctx, req.Name, string(configJSON), req.ReadOnly)
	if err != nil {
		_ = os.RemoveAll(isolatedRoot)
		return nil, fmt.Errorf("%w: register failed: %v", ErrIsolatedCreateFailed, err)
	}
	if state, detectErr := c.engine.Detect(ctx, isolatedRoot); detectErr == nil && state != nil {
		branch, detached, clean = state.Branch, state.Detached, state.Clean
	}
	return &IsolatedCreateResult{
		MountID: string(mount.ID), Name: req.Name, RootKey: config.RootKey, RepositoryPath: isolatedRoot,
		Branch: branch, Detached: detached, Clean: clean, CreatedAt: mount.CreatedAt.Format(time.RFC3339),
	}, nil
}

// CreateIsolatedFromClone is retained for callers/tests that used the original
// clone-only entry point. New HTTP callers should use CreateIsolated.
func (c *GitController) CreateIsolatedFromClone(ctx context.Context, req IsolatedCreateRequest) (*IsolatedCreateResult, error) {
	req.Mode = string(GitIsolationModeClone)
	return c.CreateIsolated(ctx, req)
}

func parseIsolationLifetime(raw string) *time.Time {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "session" || raw == "manual" || raw == "forever" {
		return nil
	}
	var d time.Duration
	var err error
	if strings.HasSuffix(raw, "d") {
		days, parseErr := time.ParseDuration(strings.TrimSuffix(raw, "d") + "h")
		if parseErr == nil {
			d = days * 24
		} else {
			err = parseErr
		}
	} else {
		d, err = time.ParseDuration(raw)
	}
	if err != nil || d <= 0 {
		return nil
	}
	expires := time.Now().UTC().Add(d)
	return &expires
}

func copySnapshotTree(sourceRoot, targetRoot, symlinkPolicy string) error {
	sourceRoot = filepath.Clean(sourceRoot)
	targetRoot = filepath.Clean(targetRoot)
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return fmt.Errorf("%w: create snapshot root: %v", ErrIsolatedCreateFailed, err)
	}
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		path = filepath.Clean(path)
		// A local workspace can contain the isolated-workspace staging directory
		// itself. Never recurse into the destination while taking a snapshot.
		if path == targetRoot || strings.HasPrefix(path, targetRoot+string(filepath.Separator)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if symlinkPolicy == "reject_repository_with_symlink" {
				return fmt.Errorf("%w: snapshot contains symlink %q", ErrGitSymlinkUnsupported, rel)
			}
			return nil
		}
		dst := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			_ = src.Close()
			return err
		}
		_, copyErr := io.Copy(dstFile, src)
		srcCloseErr := src.Close()
		dstCloseErr := dstFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if srcCloseErr != nil {
			return srcCloseErr
		}
		return dstCloseErr
	})
}

func normalizeWorkspacePaths(workspaceURI string, paths []string) ([]string, error) {
	result := make([]string, 0, len(paths))
	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		if strings.HasPrefix(path, "amitia://workspace/") {
			base := workspaceURI
			if !strings.HasSuffix(base, "/") {
				base += "/"
			}
			if !strings.HasPrefix(path, base) {
				return nil, ErrGitPathOutsideRepository
			}
			path = strings.TrimPrefix(path, base)
		}
		path = strings.TrimPrefix(filepath.ToSlash(path), "/")
		if path == "" {
			continue
		}
		if err := ValidatePaths([]string{path}); err != nil {
			return nil, err
		}
		result = append(result, path)
	}
	return result, nil
}

func qualifyWorkspacePath(workspaceURI, path string) string {
	path = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(path)), "/")
	if path == "" || strings.HasPrefix(path, "amitia://workspace/") {
		return path
	}
	if !strings.HasSuffix(workspaceURI, "/") {
		workspaceURI += "/"
	}
	return workspaceURI + path
}

func (c *GitController) DeleteIsolated(ctx context.Context, workspaceURI string, force bool) error {
	mountID, err := parseMountIDFromURI(workspaceURI)
	if err != nil {
		return err
	}
	mount, ok := c.mounts.GetMount(mountID)
	if !ok {
		return workspace.ErrMountNotFound
	}
	if mount.Kind != workspace.WorkspaceKindIsolated {
		return fmt.Errorf("%w: not an isolated mount", ErrIsolatedDeleteFailed)
	}
	root, err := c.roots.ResolveRoot(mount)
	if err != nil {
		return err
	}
	state, err := c.engine.Detect(ctx, root)
	if err == nil && state != nil && !state.Clean && !force {
		return ErrIsolatedDirty
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.workspace.RemoveMount(ctx, mountID); err != nil {
		return err
	}
	return os.RemoveAll(root)
}

func (c *GitController) IsolatedInfo(ctx context.Context, workspaceURI string) (*IsolatedWorkspaceInfo, error) {
	mountID, err := parseMountIDFromURI(workspaceURI)
	if err != nil {
		return nil, err
	}
	mount, ok := c.mounts.GetMount(mountID)
	if !ok {
		return nil, workspace.ErrMountNotFound
	}
	if mount.Kind != workspace.WorkspaceKindIsolated {
		return nil, fmt.Errorf("%w: not an isolated mount", ErrGitOperationUnsupported)
	}
	root, err := c.roots.ResolveRoot(mount)
	if err != nil {
		return nil, err
	}
	state, _ := c.engine.Detect(ctx, root)
	cfg, _ := ParseGitMountConfig(mount.BackendConfig)
	sourceStr := ""
	if cfg.Source != nil {
		sourceStr = SanitizeRemoteURL(cfg.Source.URI)
	}
	branch := ""
	head := ""
	dirty := false
	if state != nil {
		branch = state.Branch
		head = state.Head
		dirty = !state.Clean
	}
	files, totalBytes, _ := c.engine.WorkingTreeStats(ctx, root)
	_ = files
	return &IsolatedWorkspaceInfo{
		Mode:      string(cfg.Mode),
		Source:    sourceStr,
		Git:       state != nil,
		Dirty:     dirty,
		Branch:    branch,
		Head:      head,
		CreatedAt: mount.CreatedAt,
		SizeBytes: totalBytes,
		ExpiresAt: cfg.ExpiresAt,
	}, nil
}

func mountIDFromString(s string) string {
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
