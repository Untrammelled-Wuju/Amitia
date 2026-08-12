package git

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/workspace"
)

type GitController struct {
	workspace  *workspace.Service
	mounts     *workspace.Registry
	engine     GitEngine
	policy     GitPolicy
	roots      IsolatedRootResolver
	mu         sync.Mutex
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
	return c.engine.Status(ctx, root, includeIgnored, limit)
}

func (c *GitController) Diff(ctx context.Context, req GitDiffRequest) (*GitDiffResult, error) {
	root, err := c.resolveRepoPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	opts := DiffOptions{
		Mode:     req.Mode,
		Base:     req.Base,
		Target:   req.Target,
		Paths:    req.Paths,
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
	return c.engine.Diff(ctx, root, opts)
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
	if req.All && !req.Force {
		return nil, fmt.Errorf("%w: add all requires explicit force flag", ErrGitAddFailed)
	}
	if req.All {
		for _, p := range req.Paths {
			if p == "" {
				continue
			}
			if strings.HasPrefix(p, "/") {
				return nil, ErrGitPathOutsideRepository
			}
		}
	} else {
		if err := ValidatePaths(req.Paths); err != nil {
			return nil, err
		}
	}
	for _, p := range req.Paths {
		if IsSecretFile(p) {
			return nil, fmt.Errorf("%w: staged path %q matches secret file pattern", ErrGitAddFailed, p)
		}
	}
	opts := AddOptions{
		Paths: req.Paths,
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
	if err := ValidatePaths(req.Paths); err != nil {
		return nil, err
	}
	if len(req.Paths) == 0 {
		return nil, fmt.Errorf("%w: restore requires explicit paths", ErrGitRestoreFailed)
	}
	opts := RestoreOptions{
		Paths:    req.Paths,
		Source:   req.Source,
		Staged:   req.Staged,
		Worktree: req.Worktree,
	}
	if err := c.engine.Restore(ctx, root, opts); err != nil {
		return nil, err
	}
	return &GitRestoreResult{Restored: req.Paths}, nil
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
		Fetched:        fetched,
		FastForwarded:  newHead != "" && oldHead != newHead,
		OldHead:        oldHead,
		NewHead:        newHead,
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
	MountID        string    `json:"mountId"`
	Name           string    `json:"name"`
	RootKey        string    `json:"rootKey"`
	RepositoryPath string    `json:"repositoryPath"`
	Branch         string    `json:"branch"`
	Detached       bool      `json:"detached"`
	Clean          bool      `json:"clean"`
	CreatedAt      string    `json:"createdAt"`
}

func (c *GitController) CreateIsolatedFromClone(ctx context.Context, req IsolatedCreateRequest) (*IsolatedCreateResult, error) {
	if req.Mode != string(GitIsolationModeClone) && req.Mode != "" {
		return nil, fmt.Errorf("%w: mode %q not supported", ErrIsolatedCreateFailed, req.Mode)
	}
	if req.GitRemote == nil {
		return nil, fmt.Errorf("%w: gitRemote required for clone mode", ErrIsolatedCreateFailed)
	}
	
	rootsResolver := c.roots
	dataRoot := rootsResolver.DataRoot()
	opID := fmt.Sprintf("clone-%d", len(req.Name))
	stagingRoot := filepath.Join(dataRoot, "workspaces", IsolatedRootPrefix, StagingDirName, opID)
	if err := os.MkdirAll(stagingRoot, 0755); err != nil {
		return nil, fmt.Errorf("%w: cannot create staging: %v", ErrIsolatedCreateFailed, err)
	}
	cloneOpts := CloneOptions{
		URL:   req.GitRemote.URL,
		Ref:   req.Ref,
		Depth: req.Depth,
	}
	cloneResult, err := c.engine.Clone(ctx, stagingRoot, cloneOpts)
	if err != nil {
		os.RemoveAll(stagingRoot)
		return nil, fmt.Errorf("%w: clone failed: %v", ErrIsolatedCreateFailed, err)
	}
	if hasSym, _ := c.engine.HasSymlinkEntries(ctx, stagingRoot); hasSym && c.policy.SymlinkPolicy == "reject_repository_with_symlink" {
		os.RemoveAll(stagingRoot)
		return nil, fmt.Errorf("%w: repository contains symlink entries", ErrGitSymlinkUnsupported)
	}
	if hasFilter, _ := c.engine.HasSubmodules(ctx, stagingRoot); hasFilter {
		cloneResult.Branch = cloneResult.Branch
	}
	isolatedID := workspace.WorkspaceID(mountIDFromString(fmt.Sprintf("iso-%s", req.Name[:min(8, len(req.Name))])))
	isolatedRoot := filepath.Join(dataRoot, "workspaces", IsolatedRootPrefix, string(isolatedID))
	if err := os.Rename(stagingRoot, isolatedRoot); err != nil {
		os.RemoveAll(stagingRoot)
		return nil, fmt.Errorf("%w: cannot move to isolated root: %v", ErrIsolatedCreateFailed, err)
	}
	config := &GitMountConfig{
		Mode:    GitIsolationModeClone,
		RootKey: filepath.Join(IsolatedRootPrefix, string(isolatedID)),
		Source: &GitSourceSpec{
			Type: "git",
			URI:  req.GitRemote.URL,
			Ref:  req.Ref,
		},
		CreatedBy: "user",
	}
	configJSON, _ := json.Marshal(config)
	mount, err := c.mounts.RegisterIsolatedMount(ctx, req.Name, string(configJSON), req.ReadOnly)
	if err != nil {
		os.RemoveAll(isolatedRoot)
		return nil, fmt.Errorf("%w: register failed: %v", ErrIsolatedCreateFailed, err)
	}
	state, _ := c.engine.Detect(ctx, isolatedRoot)
	branch := ""
	detached := false
	if state != nil {
		branch = state.Branch
		detached = state.Detached
	} else {
		branch = cloneResult.Branch
	}
	return &IsolatedCreateResult{
		MountID:        string(mount.ID),
		Name:           req.Name,
		RootKey:        config.RootKey,
		RepositoryPath: isolatedRoot,
		Branch:         branch,
		Detached:       detached,
		Clean:          true,
		CreatedAt:      mount.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}, nil
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
	if err := c.mounts.RemoveMount(ctx, mountID); err != nil {
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
		Git:       true,
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
