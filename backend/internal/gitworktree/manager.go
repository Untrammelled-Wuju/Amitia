// Package gitworktree manages per-repository worktrees in a private, bounded
// directory. It never accepts arbitrary destination paths from API callers.
package gitworktree

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrInvalidArgument = errors.New("invalid worktree argument")
	ErrNotManaged      = errors.New("worktree is not managed by U-Ai")
	ErrDirty           = errors.New("worktree has uncommitted changes or is locked")
)

const MaxManagedWorktrees = 16

var managedID = regexp.MustCompile(`^[0-9a-f]{32}$`)
var safeRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/\-]{0,159}$`)

type Entry struct {
	ID           string       `json:"id,omitempty"`
	Path         string       `json:"path"`
	Head         string       `json:"head"`
	Branch       string       `json:"branch,omitempty"`
	Detached     bool         `json:"detached"`
	Locked       bool         `json:"locked"`
	Prunable     bool         `json:"prunable"`
	Managed      bool         `json:"managed"`
	WorkspaceURI string       `json:"workspaceUri,omitempty"`
	TaskBinding  *TaskBinding `json:"taskBinding,omitempty"`
}

type Manager struct {
	gitPath  string
	dataRoot string
}

// dataRoot is the existing U-Ai private application data directory, never a
// browser-supplied path. Repository paths come from authorized workspace mounts.
func New(dataRoot string) (*Manager, error) {
	executable, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(dataRoot) == "" {
		return nil, ErrInvalidArgument
	}
	root, err := filepath.Abs(dataRoot)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	return &Manager{gitPath: executable, dataRoot: root}, nil
}

func (m *Manager) command(ctx context.Context, root string, arguments ...string) ([]byte, error) {
	args := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "protocol.file.allow=never", "-C", root}
	args = append(args, arguments...)
	cmd := exec.CommandContext(ctx, m.gitPath, args...)
	cmd.Env = safeGitEnvironment()
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		message := strings.TrimSpace(string(output))
		if len(message) > 300 {
			message = message[:300]
		}
		return nil, fmt.Errorf("git worktree operation failed: %s: %w", message, err)
	}
	return output, nil
}

func (m *Manager) repository(ctx context.Context, root string) (string, string, error) {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", "", err
	}
	data, err := m.command(ctx, canonical, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", err
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(string(data)))
	if err != nil {
		return "", "", err
	}
	if filepath.Clean(top) != filepath.Clean(canonical) {
		return "", "", fmt.Errorf("%w: mount must point to git worktree root", ErrInvalidArgument)
	}
	// Resolve Git common directory so a linked worktree and main worktree use
	// the same managed group, while never trusting the caller's group string.
	gitCommon, err := m.command(ctx, canonical, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", "", err
	}
	common, err := filepath.Abs(strings.TrimSpace(string(gitCommon)))
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(filepath.Clean(common)))
	return canonical, hex.EncodeToString(sum[:12]), nil
}

func (m *Manager) managedRoot(key string) string {
	return filepath.Join(m.dataRoot, "workspaces", "agent-worktrees", key)
}

func parsePorcelain(data []byte) []Entry {
	result := make([]Entry, 0)
	for _, block := range bytes.Split(data, []byte{0, 0}) {
		entry := Entry{}
		for _, line := range bytes.Split(block, []byte{0}) {
			str := string(line)
			switch {
			case strings.HasPrefix(str, "worktree "):
				entry.Path = strings.TrimPrefix(str, "worktree ")
			case strings.HasPrefix(str, "HEAD "):
				entry.Head = strings.TrimPrefix(str, "HEAD ")
			case strings.HasPrefix(str, "branch "):
				entry.Branch = strings.TrimPrefix(str, "branch refs/heads/")
			case str == "detached":
				entry.Detached = true
			case strings.HasPrefix(str, "locked"):
				entry.Locked = true
			case strings.HasPrefix(str, "prunable"):
				entry.Prunable = true
			}
		}
		if entry.Path != "" {
			result = append(result, entry)
		}
	}
	return result
}

func (m *Manager) list(ctx context.Context, root string) ([]Entry, string, error) {
	canonical, key, err := m.repository(ctx, root)
	if err != nil {
		return nil, "", err
	}
	raw, err := m.command(ctx, canonical, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, "", err
	}
	managed := m.managedRoot(key)
	entries := parsePorcelain(raw)
	for i := range entries {
		parent := filepath.Dir(entries[i].Path)
		id := filepath.Base(entries[i].Path)
		if filepath.Clean(parent) == filepath.Clean(managed) && managedID.MatchString(id) {
			entries[i].ID = id
			entries[i].Managed = true
			binding, bindingErr := m.readBinding(key, id)
			if bindingErr != nil {
				return nil, "", bindingErr
			}
			entries[i].TaskBinding = binding
		}
	}
	return entries, canonical, nil
}

func (m *Manager) List(ctx context.Context, repoRoot string) ([]Entry, error) {
	entries, _, err := m.list(ctx, repoRoot)
	return entries, err
}

func validateRef(ref string) bool {
	return safeRef.MatchString(ref) && !strings.Contains(ref, "..") &&
		!strings.Contains(ref, "@{") && !strings.HasSuffix(ref, ".lock") &&
		!strings.HasSuffix(ref, "/") && !strings.HasSuffix(ref, ".")
}

func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// refuseConfiguredFilters disallows arbitrary smudge/process/clean commands
// configured inside repository metadata. Global and system Git configuration
// are excluded by the command environment; repository filters still require
// an explicit check before any checkout operation.
func (m *Manager) refuseConfiguredFilters(ctx context.Context, root string) error {
	cmd := exec.CommandContext(ctx, m.gitPath,
		"-c", "core.hooksPath=/dev/null", "-C", root,
		"config", "--get-regexp", `^filter\..*\.(smudge|clean|process)$`)
	cmd.Env = safeGitEnvironment()
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(bytes.TrimSpace(output)) > 0 {
		return fmt.Errorf("%w: repository config enables external Git filters", ErrInvalidArgument)
	}
	if err == nil {
		return nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return nil
	}
	return fmt.Errorf("%w: cannot validate git filter configuration: %v", ErrInvalidArgument, err)
}

// Create produces a detached worktree by default. Branch creation is opt-in.
// It never runs force, removes an existing directory, or checks out a remote URL.
func (m *Manager) Create(ctx context.Context, repoRoot, branch, ref string) (Entry, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, err := m.lockRepositoryAllocation(ctx, repoRoot)
	if err != nil {
		return Entry{}, err
	}
	defer unlock()
	mergeUnlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		return Entry{}, err
	}
	defer mergeUnlock()
	return m.createLocked(ctx, repoRoot, branch, ref)
}

func (m *Manager) createLocked(ctx context.Context, repoRoot, branch, ref string) (Entry, error) {
	if ref == "" {
		ref = "HEAD"
	}
	if !validateRef(ref) || (branch != "" && !validateRef(branch)) {
		return Entry{}, ErrInvalidArgument
	}
	entries, canonical, err := m.list(ctx, repoRoot)
	if err != nil {
		return Entry{}, err
	}
	_, key, err := m.repository(ctx, canonical)
	if err != nil {
		return Entry{}, err
	}
	count := 0
	for _, entry := range entries {
		if entry.Managed {
			count++
		}
	}
	if count >= MaxManagedWorktrees {
		return Entry{}, fmt.Errorf("%w: repository limit reached", ErrInvalidArgument)
	}
	if err := m.refuseConfiguredFilters(ctx, canonical); err != nil {
		return Entry{}, err
	}
	// Reject invalid/non-commit refs and branch names before allocating disk.
	if _, err := m.command(ctx, canonical, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
		return Entry{}, ErrInvalidArgument
	}
	if branch != "" {
		if _, err := m.command(ctx, canonical, "check-ref-format", "--branch", branch); err != nil {
			return Entry{}, ErrInvalidArgument
		}
	}
	parent := m.managedRoot(key)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return Entry{}, err
	}
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil || filepath.Clean(realParent) != filepath.Clean(parent) {
		return Entry{}, fmt.Errorf("%w: managed directory cannot be a symlink", ErrInvalidArgument)
	}
	id, err := newID()
	if err != nil {
		return Entry{}, err
	}
	target := filepath.Join(parent, id)
	if _, err := os.Lstat(target); err == nil {
		return Entry{}, ErrInvalidArgument
	} else if !os.IsNotExist(err) {
		return Entry{}, err
	}
	args := []string{"worktree", "add"}
	if branch == "" {
		args = append(args, "--detach")
	} else {
		args = append(args, "-b", branch)
	}
	args = append(args, "--", target, ref)
	if _, err := m.command(ctx, canonical, args...); err != nil {
		return Entry{}, m.recoverFailedCreate(ctx, canonical, id, target, err)
	}
	updated, _, err := m.list(ctx, canonical)
	if err != nil {
		return Entry{}, m.recoverFailedCreate(ctx, canonical, id, target, err)
	}
	for _, item := range updated {
		if item.Managed && item.ID == id {
			return item, nil
		}
	}
	return Entry{}, m.recoverFailedCreate(ctx, canonical, id, target, fmt.Errorf("%w: new worktree not found", ErrNotManaged))
}

// Remove is deliberately non-force. Git refuses dirty or locked worktrees.
// Only a worktree created under the private U-Ai managed directory can be removed.
func (m *Manager) Remove(ctx context.Context, repoRoot, id string) (Entry, error) {
	finish, err := m.lockCheckpointExecution(ctx, repoRoot, id)
	if err != nil {
		return Entry{}, err
	}
	defer finish()
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, err := m.lockRepositoryAllocation(ctx, repoRoot)
	if err != nil {
		return Entry{}, err
	}
	defer unlock()
	return m.removeLocked(ctx, repoRoot, id)
}

func (m *Manager) removeLocked(ctx context.Context, repoRoot, id string) (Entry, error) {
	mergeUnlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		return Entry{}, err
	}
	defer mergeUnlock()
	return m.removeWithMergeLock(ctx, repoRoot, id)
}

func (m *Manager) removeWithMergeLock(ctx context.Context, repoRoot, id string) (Entry, error) {
	if !managedID.MatchString(id) {
		return Entry{}, ErrInvalidArgument
	}
	entries, canonical, err := m.list(ctx, repoRoot)
	if err != nil {
		return Entry{}, err
	}
	var target Entry
	found := false
	for _, item := range entries {
		if item.Managed && item.ID == id {
			target = item
			found = true
			break
		}
	}
	if !found {
		return Entry{}, ErrNotManaged
	}
	if target.TaskBinding != nil {
		return Entry{}, ErrBound
	}
	if target.Locked || target.Prunable {
		return Entry{}, ErrDirty
	}
	info, err := os.Lstat(target.Path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Entry{}, ErrNotManaged
	}
	_, key, err := m.repository(ctx, canonical)
	if err != nil {
		return Entry{}, err
	}
	if err := m.refusePendingMergeAudit(key, id); err != nil {
		return Entry{}, err
	}
	status, err := m.command(ctx, target.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return Entry{}, err
	}
	if len(status) != 0 {
		return Entry{}, ErrDirty
	}
	operationRoot := canonical
	if filepath.Clean(operationRoot) == filepath.Clean(target.Path) {
		operationRoot = ""
		for _, item := range entries {
			if filepath.Clean(item.Path) != filepath.Clean(target.Path) && !item.Prunable {
				operationRoot = item.Path
				break
			}
		}
		if operationRoot == "" {
			return Entry{}, ErrInvalidArgument
		}
	}
	if err := m.invalidateCheckpointByKey(key, id); err != nil {
		return Entry{}, err
	}
	if _, err := m.command(ctx, operationRoot, "worktree", "remove", "--", target.Path); err != nil {
		return Entry{}, fmt.Errorf("%w: %v", ErrDirty, err)
	}
	return target, nil
}
