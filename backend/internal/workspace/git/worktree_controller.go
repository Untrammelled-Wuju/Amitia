package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/gitworktree"
	"github.com/u-ai/backend/internal/workspace"
)

type WorktreeCreateRequest struct {
	WorkspaceURI   string `json:"workspaceUri"`
	Name           string `json:"name"`
	Branch         string `json:"branch,omitempty"`
	Ref            string `json:"ref,omitempty"`
	TaskID         string `json:"taskId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
}

type WorktreeRemoveRequest struct {
	WorkspaceURI string `json:"workspaceUri"`
	ID           string `json:"id"`
}

type WorktreeReleaseRequest struct {
	WorkspaceURI   string `json:"workspaceUri"`
	ID             string `json:"id"`
	TaskID         string `json:"taskId"`
	ConversationID string `json:"conversationId"`
}

// WorktreeList returns every git-linked worktree and marks which ones are
// managed by U-Ai. Unmanaged paths are informational and never removable.
func (c *GitController) WorktreeList(ctx context.Context, workspaceURI string) ([]gitworktree.Entry, error) {
	root, err := c.resolveRepoPath(workspaceURI)
	if err != nil {
		return nil, err
	}
	manager, err := gitworktree.New(c.roots.DataRoot())
	if err != nil {
		return nil, err
	}
	entries, err := manager.List(ctx, root)
	if err != nil {
		return nil, err
	}
	mounts, err := c.workspace.ListMounts(ctx)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if !entries[i].Managed {
			continue
		}
		for _, mount := range mounts {
			if mount.Kind == workspace.WorkspaceKindLocal && filepath.Clean(mount.LocalRoot) == filepath.Clean(entries[i].Path) {
				entries[i].WorkspaceURI = mount.RootURI
				break
			}
		}
	}
	return entries, nil
}

func (c *GitController) WorktreeCreate(ctx context.Context, req WorktreeCreateRequest) (*gitworktree.Entry, error) {
	if (req.TaskID != "" || req.ConversationID != "") && !gitworktree.ValidBinding(req.TaskID, req.ConversationID) {
		return nil, gitworktree.ErrInvalidArgument
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	root, mount, err := c.resolveRepositoryPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	if mount.ReadOnly {
		return nil, fmt.Errorf("%w: workspace is read-only", ErrGitOperationUnsupported)
	}
	if submodules, err := c.engine.HasSubmodules(ctx, root); err != nil || submodules {
		return nil, fmt.Errorf("%w: worktree checkout does not initialize submodules", ErrGitSubmoduleUnsupported)
	}
	if symlinks, err := c.engine.HasSymlinkEntries(ctx, root); err != nil || symlinks {
		return nil, fmt.Errorf("%w: worktree checkout does not allow tracked symlinks", ErrGitSymlinkUnsupported)
	}
	if len(req.Name) > 100 || strings.ContainsAny(req.Name, "\x00\n\r") {
		return nil, gitworktree.ErrInvalidArgument
	}
	manager, err := gitworktree.New(c.roots.DataRoot())
	if err != nil {
		return nil, err
	}
	var entry gitworktree.Entry
	created := true
	if req.TaskID != "" {
		entry, created, err = manager.EnsureTask(ctx, root, strings.TrimSpace(req.Branch), strings.TrimSpace(req.Ref), req.TaskID, req.ConversationID)
	} else {
		entry, err = manager.Create(ctx, root, strings.TrimSpace(req.Branch), strings.TrimSpace(req.Ref))
	}
	if err != nil {
		return nil, err
	}
	rollback := func(cause error) error {
		if !created {
			return cause
		}
		if entry.TaskBinding != nil {
			if releaseErr := manager.Release(ctx, root, entry.ID, req.TaskID, req.ConversationID); releaseErr != nil {
				return fmt.Errorf("%w; release task binding: %v", cause, releaseErr)
			}
		}
		if _, removeErr := manager.Remove(ctx, root, entry.ID); removeErr != nil {
			return fmt.Errorf("%w; cleanup worktree: %v", cause, removeErr)
		}
		return cause
	}
	mounts, err := c.workspace.ListMounts(ctx)
	if err == nil {
		for _, registered := range mounts {
			if registered.Kind == workspace.WorkspaceKindLocal && filepath.Clean(registered.LocalRoot) == filepath.Clean(entry.Path) {
				entry.WorkspaceURI = registered.RootURI
				return &entry, nil
			}
		}
	}
	if err != nil {
		return nil, rollback(fmt.Errorf("list worktree mounts: %w", err))
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Agent Worktree " + entry.ID[:8]
	}
	registered, err := c.workspace.RegisterLocalMount(ctx, name, entry.Path, false)
	if err != nil {
		return nil, rollback(fmt.Errorf("register worktree mount: %w", err))
	}
	entry.WorkspaceURI = registered.RootURI
	return &entry, nil
}

type WorktreeReviewRequest struct {
	WorkspaceURI   string `json:"workspaceUri"`
	ID             string `json:"id"`
	TaskID         string `json:"taskId"`
	ConversationID string `json:"conversationId"`
	ExpectedHead   string `json:"expectedHead,omitempty"`
}

type WorktreeReviewResult struct {
	Entry            gitworktree.Entry `json:"entry"`
	Status           *GitStatusResult  `json:"status"`
	Diff             *GitDiffResult    `json:"diff"`
	UntrackedCount   int               `json:"untrackedCount"`
	ConflictedCount  int               `json:"conflictedCount"`
	ReviewIncomplete bool              `json:"reviewIncomplete"`
	BaselineKnown    bool              `json:"baselineKnown"`
}

func (c *GitController) WorktreeReview(ctx context.Context, req WorktreeReviewRequest) (*WorktreeReviewResult, error) {
	if !gitworktree.ValidBinding(req.TaskID, req.ConversationID) || len(req.ID) != 32 {
		return nil, gitworktree.ErrInvalidArgument
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	root, _, err := c.resolveRepositoryPath(req.WorkspaceURI)
	if err != nil {
		return nil, err
	}
	manager, err := gitworktree.New(c.roots.DataRoot())
	if err != nil {
		return nil, err
	}
	entries, err := manager.List(ctx, root)
	if err != nil {
		return nil, err
	}
	var target *gitworktree.Entry
	for i := range entries {
		if entries[i].Managed && entries[i].ID == req.ID {
			target = &entries[i]
			break
		}
	}
	if target == nil {
		return nil, gitworktree.ErrNotManaged
	}
	if target.TaskBinding == nil || target.TaskBinding.TaskID != req.TaskID || target.TaskBinding.ConversationID != req.ConversationID {
		return nil, gitworktree.ErrBindingMismatch
	}
	mounts, err := c.workspace.ListMounts(ctx)
	if err != nil {
		return nil, err
	}
	for _, mount := range mounts {
		if mount.Kind == workspace.WorkspaceKindLocal && filepath.Clean(mount.LocalRoot) == filepath.Clean(target.Path) {
			target.WorkspaceURI = mount.RootURI
			break
		}
	}
	if target.WorkspaceURI == "" {
		return nil, fmt.Errorf("%w: worktree is not registered", gitworktree.ErrNotManaged)
	}
	if req.ExpectedHead != "" && req.ExpectedHead != target.Head {
		return nil, gitworktree.ErrReviewOutdated
	}
	status, err := c.engine.Status(ctx, target.Path, false, c.policy.MaxStatusEntries)
	if err != nil {
		return nil, err
	}
	baseline := target.TaskBinding.BaseHead
	if baseline == "" {
		baseline = "HEAD"
	}
	diff, err := c.engine.Diff(ctx, target.Path, DiffOptions{Mode: "worktree", Base: baseline, MaxBytes: c.policy.MaxDiffBytes})
	if err != nil {
		return nil, err
	}
	if status.Head != target.Head {
		return nil, gitworktree.ErrReviewOutdated
	}
	confirmed, err := manager.List(ctx, root)
	if err != nil {
		return nil, err
	}
	stillBound := false
	for _, current := range confirmed {
		if current.Managed && current.ID == req.ID {
			if current.TaskBinding == nil || current.TaskBinding.TaskID != req.TaskID || current.TaskBinding.ConversationID != req.ConversationID {
				return nil, gitworktree.ErrBindingMismatch
			}
			if current.Head != target.Head || current.TaskBinding.BaseHead != target.TaskBinding.BaseHead {
				return nil, gitworktree.ErrReviewOutdated
			}
			stillBound = true
			break
		}
	}
	if !stillBound {
		return nil, gitworktree.ErrNotManaged
	}
	status.RepositoryRootURI = target.WorkspaceURI
	for i := range status.Entries {
		status.Entries[i].URI = qualifyWorkspacePath(target.WorkspaceURI, status.Entries[i].URI)
		if status.Entries[i].OldURI != "" {
			status.Entries[i].OldURI = qualifyWorkspacePath(target.WorkspaceURI, status.Entries[i].OldURI)
		}
	}
	for i := range diff.Files {
		diff.Files[i].URI = qualifyWorkspacePath(target.WorkspaceURI, diff.Files[i].URI)
		if diff.Files[i].OldURI != "" {
			diff.Files[i].OldURI = qualifyWorkspacePath(target.WorkspaceURI, diff.Files[i].OldURI)
		}
	}
	result := &WorktreeReviewResult{Entry: *target, Status: status, Diff: diff, BaselineKnown: target.TaskBinding.BaseHead != ""}
	for _, changed := range status.Entries {
		if changed.Untracked {
			result.UntrackedCount++
		}
		if changed.Conflict {
			result.ConflictedCount++
		}
	}
	result.ReviewIncomplete = !result.BaselineKnown || diff.Truncated || status.Truncated || result.UntrackedCount > 0 || result.ConflictedCount > 0
	return result, nil
}

func (c *GitController) WorktreeRemove(ctx context.Context, req WorktreeRemoveRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	root, mount, err := c.resolveRepositoryPath(req.WorkspaceURI)
	if err != nil {
		return err
	}
	if mount.ReadOnly {
		return fmt.Errorf("%w: workspace is read-only", ErrGitOperationUnsupported)
	}
	manager, err := gitworktree.New(c.roots.DataRoot())
	if err != nil {
		return err
	}
	entries, err := manager.List(ctx, root)
	if err != nil {
		return err
	}
	var path string
	for _, entry := range entries {
		if entry.Managed && entry.ID == req.ID {
			path = entry.Path
			break
		}
	}
	if path == "" {
		return gitworktree.ErrNotManaged
	}
	// Do not remove a tree if an unrelated registered mount shares this path.
	mounts, err := c.workspace.ListMounts(ctx)
	if err != nil {
		return err
	}
	var registered []workspace.WorkspaceMount
	for _, mount := range mounts {
		if mount.Kind == workspace.WorkspaceKindLocal && filepath.Clean(mount.LocalRoot) == filepath.Clean(path) {
			registered = append(registered, mount)
		}
	}
	if len(registered) > 1 {
		return fmt.Errorf("%w: duplicate worktree mounts", gitworktree.ErrInvalidArgument)
	}
	if _, err := manager.Remove(ctx, root, req.ID); err != nil {
		return err
	}
	for _, mount := range registered {
		if err := c.workspace.RemoveMount(ctx, mount.ID); err != nil {
			return fmt.Errorf("worktree removed, workspace mount cleanup failed: %w", err)
		}
	}
	return nil
}

func (c *GitController) WorktreeRelease(ctx context.Context, req WorktreeReleaseRequest) error {
	if !gitworktree.ValidBinding(req.TaskID, req.ConversationID) {
		return gitworktree.ErrInvalidArgument
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	root, mount, err := c.resolveRepositoryPath(req.WorkspaceURI)
	if err != nil {
		return err
	}
	if mount.ReadOnly {
		return fmt.Errorf("%w: workspace is read-only", ErrGitOperationUnsupported)
	}
	manager, err := gitworktree.New(c.roots.DataRoot())
	if err != nil {
		return err
	}
	return manager.Release(ctx, root, req.ID, req.TaskID, req.ConversationID)
}
