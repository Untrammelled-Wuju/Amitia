package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CLIGitEngine is the production GitEngine implementation backed by the
// system git executable. Commands are executed with terminal prompting and
// repository hooks disabled so API requests cannot block on interactive input
// or execute arbitrary local hook programs.
type CLIGitEngine struct {
	gitPath string
}

func NewCLIGitEngine() (*CLIGitEngine, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("%w: git executable not found", ErrGitUnavailable)
	}
	return &CLIGitEngine{gitPath: path}, nil
}

func (e *CLIGitEngine) command(ctx context.Context, path string, args ...string) *exec.Cmd {
	base := []string{"-c", "core.hooksPath=/dev/null", "-c", "protocol.file.allow=never"}
	if path != "" {
		base = append(base, "-C", path)
	}
	base = append(base, args...)
	cmd := exec.CommandContext(ctx, e.gitPath, base...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	return cmd
}

func (e *CLIGitEngine) run(ctx context.Context, path string, args ...string) ([]byte, error) {
	cmd := e.command(ctx, path, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ErrGitCancelled
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ErrGitTimeout
		}
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

func (e *CLIGitEngine) runOptional(ctx context.Context, path string, args ...string) string {
	out, err := e.run(ctx, path, args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (e *CLIGitEngine) Detect(ctx context.Context, path string) (*RepositoryState, error) {
	root, err := e.run(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, ErrRepositoryNotFound
	}
	rootPath := strings.TrimSpace(string(root))
	branch := e.runOptional(ctx, path, "symbolic-ref", "--quiet", "--short", "HEAD")
	detached := branch == ""
	head := e.runOptional(ctx, path, "rev-parse", "--verify", "HEAD")
	upstream := e.runOptional(ctx, path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	ahead, behind := 0, 0
	if upstream != "" && head != "" {
		counts := strings.Fields(e.runOptional(ctx, path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}"))
		if len(counts) == 2 {
			ahead, _ = strconv.Atoi(counts[0])
			behind, _ = strconv.Atoi(counts[1])
		}
	}
	status, err := e.run(ctx, path, "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitStatusFailed, err)
	}
	shallow := strings.EqualFold(e.runOptional(ctx, path, "rev-parse", "--is-shallow-repository"), "true")
	hasSubmodules, _ := e.HasSubmodules(ctx, path)
	return &RepositoryState{
		RepositoryPath: rootPath,
		Branch:         branch,
		Detached:       detached,
		Head:           head,
		Upstream:       upstream,
		Ahead:          ahead,
		Behind:         behind,
		Clean:          len(bytes.TrimSpace(status)) == 0,
		Shallow:        shallow,
		HasSubmodules:  hasSubmodules,
	}, nil
}

func (e *CLIGitEngine) Init(ctx context.Context, path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("%w: %v", ErrRepositoryInvalid, err)
	}
	if _, err := e.run(ctx, "", "init", "-b", "main", path); err != nil {
		// Older Git versions may not support -b. Keep a compatibility fallback.
		if _, fallbackErr := e.run(ctx, "", "init", path); fallbackErr != nil {
			return fmt.Errorf("%w: %v", ErrRepositoryInvalid, fallbackErr)
		}
	}
	return nil
}

func (e *CLIGitEngine) Clone(ctx context.Context, path string, opts CloneOptions) (*CloneResult, error) {
	if err := ValidateRemoteURL(opts.URL); err != nil {
		return nil, err
	}
	if opts.CancelFn != nil && opts.CancelFn() {
		return nil, ErrGitCancelled
	}
	args := []string{"clone", "--no-recurse-submodules"}
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	if strings.TrimSpace(opts.Ref) != "" {
		if err := ValidateRefName(opts.Ref); err != nil {
			return nil, err
		}
		args = append(args, "--branch", opts.Ref)
	}
	args = append(args, opts.URL, path)
	if opts.ProgressFn != nil {
		opts.ProgressFn("clone", 0, 1)
	}
	if _, err := e.run(ctx, "", args...); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIsolatedCreateFailed, err)
	}
	state, err := e.Detect(ctx, path)
	if err != nil {
		return nil, err
	}
	defaultBranch, _ := e.DefaultBranchFromRemote(ctx, opts.URL, opts.Auth)
	if defaultBranch == "" {
		defaultBranch = state.Branch
	}
	if opts.ProgressFn != nil {
		opts.ProgressFn("clone", 1, 1)
	}
	return &CloneResult{RepositoryPath: path, Branch: state.Branch, DefaultBranch: defaultBranch, Head: state.Head}, nil
}

func parsePorcelainStatus(data []byte, includeIgnored bool, limit int) ([]GitStatusEntry, bool) {
	if limit <= 0 {
		limit = DefaultMaxStatusEntries
	}
	parts := bytes.Split(data, []byte{0})
	entries := make([]GitStatusEntry, 0, min(len(parts), limit))
	truncated := false
	for i := 0; i < len(parts); i++ {
		record := string(parts[i])
		if record == "" || len(record) < 3 {
			continue
		}
		x, y := record[0], record[1]
		name := strings.TrimSpace(record[3:])
		old := ""
		if (x == 'R' || x == 'C' || y == 'R' || y == 'C') && i+1 < len(parts) && len(parts[i+1]) > 0 {
			old = name
			i++
			name = string(parts[i])
		}
		ignored := x == '!' && y == '!'
		if ignored && !includeIgnored {
			continue
		}
		if len(entries) >= limit {
			truncated = true
			continue
		}
		untracked := x == '?' && y == '?'
		conflict := x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D')
		entries = append(entries, GitStatusEntry{
			URI:       filepath.ToSlash(name),
			OldURI:    filepath.ToSlash(old),
			Staging:   string(x),
			Worktree:  string(y),
			Untracked: untracked,
			Conflict:  conflict,
			Ignored:   ignored,
		})
	}
	return entries, truncated
}

func (e *CLIGitEngine) Status(ctx context.Context, path string, includeIgnored bool, limit int) (*GitStatusResult, error) {
	args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=normal"}
	if includeIgnored {
		args = append(args, "--ignored=matching")
	}
	out, err := e.run(ctx, path, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitStatusFailed, err)
	}
	entries, truncated := parsePorcelainStatus(out, includeIgnored, limit)
	state, err := e.Detect(ctx, path)
	if err != nil {
		return nil, err
	}
	return &GitStatusResult{
		Branch:        state.Branch,
		Detached:      state.Detached,
		Head:          state.Head,
		Upstream:      state.Upstream,
		Ahead:         state.Ahead,
		Behind:        state.Behind,
		Clean:         len(entries) == 0,
		Entries:       entries,
		Truncated:     truncated,
		Shallow:       state.Shallow,
		HasSubmodules: state.HasSubmodules,
	}, nil
}

func diffArgs(opts DiffOptions) []string {
	args := []string{"diff", "--no-ext-diff", "--no-color"}
	switch opts.Mode {
	case "staged", "index", "cached":
		args = append(args, "--cached")
	}
	if opts.Base != "" && opts.Target != "" {
		args = append(args, opts.Base, opts.Target)
	} else if opts.Base != "" {
		args = append(args, opts.Base)
	} else if opts.Target != "" {
		args = append(args, opts.Target)
	}
	return args
}

func (e *CLIGitEngine) Diff(ctx context.Context, path string, opts DiffOptions) (*GitDiffResult, error) {
	base := diffArgs(opts)
	nameArgs := append(append([]string{}, base...), "--name-status", "-z")
	if len(opts.Paths) > 0 {
		nameArgs = append(nameArgs, "--")
		nameArgs = append(nameArgs, opts.Paths...)
	}
	nameOut, err := e.run(ctx, path, nameArgs...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitDiffFailed, err)
	}
	parts := bytes.Split(nameOut, []byte{0})
	files := make([]GitDiffFile, 0)
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxDiffBytes
	}
	used := 0
	truncated := false
	for i := 0; i < len(parts); i++ {
		if len(parts[i]) == 0 {
			continue
		}
		statusRaw := string(parts[i])
		if i+1 >= len(parts) {
			break
		}
		i++
		oldPath := ""
		filePath := string(parts[i])
		if strings.HasPrefix(statusRaw, "R") || strings.HasPrefix(statusRaw, "C") {
			oldPath = filePath
			if i+1 >= len(parts) {
				break
			}
			i++
			filePath = string(parts[i])
		}
		patchArgs := append(append([]string{}, base...), "--", filePath)
		patch, patchErr := e.run(ctx, path, patchArgs...)
		patchText := ""
		isBinary := false
		patchTrunc := false
		if patchErr == nil {
			isBinary = bytes.Contains(patch, []byte("Binary files ")) || bytes.Contains(patch, []byte("GIT binary patch"))
			remaining := maxBytes - used
			if remaining <= 0 {
				truncated, patchTrunc = true, true
			} else if len(patch) > remaining {
				patch = patch[:remaining]
				truncated, patchTrunc = true, true
			}
			used += len(patch)
			patchText = string(patch)
		}
		status := "modified"
		switch statusRaw[0] {
		case 'A':
			status = "added"
		case 'D':
			status = "deleted"
		case 'R':
			status = "renamed"
		case 'C':
			status = "copied"
		case 'T':
			status = "type_changed"
		case 'U':
			status = "unmerged"
		}
		files = append(files, GitDiffFile{URI: filepath.ToSlash(filePath), OldURI: filepath.ToSlash(oldPath), Status: status, Patch: patchText, PatchTrunc: patchTrunc, IsBinary: isBinary})
	}
	stats := GitDiffStats{FilesChanged: len(files), Bytes: used}
	numArgs := append(append([]string{}, base...), "--numstat")
	if len(opts.Paths) > 0 {
		numArgs = append(numArgs, "--")
		numArgs = append(numArgs, opts.Paths...)
	}
	if numOut, numErr := e.run(ctx, path, numArgs...); numErr == nil {
		scanner := bufio.NewScanner(bytes.NewReader(numOut))
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 || fields[0] == "-" || fields[1] == "-" {
				continue
			}
			a, _ := strconv.Atoi(fields[0])
			d, _ := strconv.Atoi(fields[1])
			stats.Insertions += a
			stats.Deletions += d
		}
	}
	return &GitDiffResult{Files: files, Stats: stats, Truncated: truncated}, nil
}

func (e *CLIGitEngine) Log(ctx context.Context, path string, opts LogOptions) (*GitLogResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	format := "%H%x1f%an%x1f%ae%x1f%cn%x1f%ce%x1f%at%x1f%ct%x1f%s%x1f%b%x1f%P%x1e"
	args := []string{"log", "--date=unix", "--format=" + format, "-n", strconv.Itoa(limit)}
	if opts.Ref != "" {
		args = append(args, opts.Ref)
	}
	if opts.Path != "" {
		args = append(args, "--", opts.Path)
	}
	out, err := e.run(ctx, path, args...)
	if err != nil {
		// An unborn repository has no log yet; expose it as an empty history.
		if strings.Contains(err.Error(), "does not have any commits") || strings.Contains(err.Error(), "unknown revision") || strings.Contains(err.Error(), "bad default revision") {
			return &GitLogResult{Entries: []GitCommitInfo{}}, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrGitLogFailed, err)
	}
	entries := make([]GitCommitInfo, 0)
	for _, record := range strings.Split(string(out), "\x1e") {
		record = strings.Trim(record, "\r\n ")
		if record == "" {
			continue
		}
		f := strings.Split(record, "\x1f")
		if len(f) < 10 {
			continue
		}
		authored, _ := strconv.ParseInt(strings.TrimSpace(f[5]), 10, 64)
		committed, _ := strconv.ParseInt(strings.TrimSpace(f[6]), 10, 64)
		parents := strings.Fields(strings.TrimSpace(f[9]))
		entries = append(entries, GitCommitInfo{
			Hash: strings.TrimSpace(f[0]), AuthorName: f[1], AuthorEmail: f[2], CommitterName: f[3], CommitterEmail: f[4],
			AuthoredAt: time.Unix(authored, 0).UTC(), CommittedAt: time.Unix(committed, 0).UTC(), Subject: f[7], Body: strings.TrimSpace(f[8]), Parents: parents,
		})
	}
	shallow := strings.EqualFold(e.runOptional(ctx, path, "rev-parse", "--is-shallow-repository"), "true")
	return &GitLogResult{Entries: entries, HistoryIncomplete: shallow}, nil
}

func (e *CLIGitEngine) Add(ctx context.Context, path string, opts AddOptions) ([]string, error) {
	args := []string{"add"}
	if opts.All {
		args = append(args, "--all")
	} else {
		args = append(args, "--")
		args = append(args, opts.Paths...)
	}
	if _, err := e.run(ctx, path, args...); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitAddFailed, err)
	}
	out, err := e.run(ctx, path, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitAddFailed, err)
	}
	return splitNUL(out), nil
}

func (e *CLIGitEngine) Restore(ctx context.Context, path string, opts RestoreOptions) error {
	args := []string{"restore"}
	if opts.Source != "" {
		args = append(args, "--source", opts.Source)
	}
	if opts.Staged {
		args = append(args, "--staged")
	}
	if opts.Worktree {
		args = append(args, "--worktree")
	}
	args = append(args, "--")
	args = append(args, opts.Paths...)
	if _, err := e.run(ctx, path, args...); err != nil {
		return fmt.Errorf("%w: %v", ErrGitRestoreFailed, err)
	}
	return nil
}

func (e *CLIGitEngine) Commit(ctx context.Context, path string, opts CommitOptions) (*GitCommitResult, error) {
	stagedOut, _ := e.run(ctx, path, "diff", "--cached", "--name-only", "-z")
	filesChanged := len(splitNUL(stagedOut))
	if filesChanged == 0 {
		return nil, ErrGitCommitFailed
	}
	args := []string{"commit", "-m", opts.Message, "--no-verify"}
	if opts.Author != nil && strings.TrimSpace(opts.Author.Name) != "" && strings.TrimSpace(opts.Author.Email) != "" {
		args = append(args, "--author", fmt.Sprintf("%s <%s>", opts.Author.Name, opts.Author.Email))
	}
	if _, err := e.run(ctx, path, args...); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitCommitFailed, err)
	}
	hash := e.runOptional(ctx, path, "rev-parse", "HEAD")
	branch := e.runOptional(ctx, path, "symbolic-ref", "--quiet", "--short", "HEAD")
	parents := strings.Fields(e.runOptional(ctx, path, "show", "-s", "--format=%P", "HEAD"))
	return &GitCommitResult{Hash: hash, Branch: branch, ParentHashes: parents, FilesChanged: filesChanged}, nil
}

func (e *CLIGitEngine) ListBranches(ctx context.Context, path string) (*GitBranchListResult, error) {
	out, err := e.run(ctx, path, "for-each-ref", "--format=%(refname:short)%00%(objectname)%00%(upstream:short)%00", "refs/heads")
	if err != nil {
		return nil, err
	}
	current := e.runOptional(ctx, path, "symbolic-ref", "--quiet", "--short", "HEAD")
	branches := make([]GitBranchInfo, 0)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) < 2 {
			continue
		}
		up := ""
		if len(f) > 2 {
			up = f[2]
		}
		remote, remoteRef := "", ""
		if idx := strings.Index(up, "/"); idx > 0 {
			remote, remoteRef = up[:idx], up[idx+1:]
		}
		branches = append(branches, GitBranchInfo{Name: f[0], Current: f[0] == current, Commit: f[1], Remote: remote, RemoteRef: remoteRef, Pull: up, Push: up})
	}
	return &GitBranchListResult{Branches: branches, Current: current, Detached: current == ""}, nil
}

func (e *CLIGitEngine) Checkout(ctx context.Context, path string, opts CheckoutOptions) (*GitCheckoutResult, error) {
	args := []string{"checkout"}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.Detach {
		args = append(args, "--detach", opts.Branch)
	} else if opts.Create {
		args = append(args, "-b", opts.Branch)
		if opts.FromRef != "" {
			args = append(args, opts.FromRef)
		}
	} else {
		args = append(args, opts.Branch)
	}
	if _, err := e.run(ctx, path, args...); err != nil {
		if strings.Contains(err.Error(), "would be overwritten") || strings.Contains(err.Error(), "local changes") {
			return nil, ErrGitDirty
		}
		return nil, fmt.Errorf("%w: %v", ErrGitCheckoutFailed, err)
	}
	state, err := e.Detect(ctx, path)
	if err != nil {
		return nil, err
	}
	return &GitCheckoutResult{Branch: state.Branch, Detached: state.Detached, Head: state.Head}, nil
}

func (e *CLIGitEngine) Fetch(ctx context.Context, path string, opts FetchOptions) (*GitFetchResult, error) {
	remote := strings.TrimSpace(opts.Remote)
	if remote == "" {
		remote = "origin"
	}
	args := []string{"fetch", "--prune", remote}
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	if opts.Deepen > 0 {
		args = append(args, "--deepen", strconv.Itoa(opts.Deepen))
	}
	if _, err := e.run(ctx, path, args...); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitFetchFailed, err)
	}
	return &GitFetchResult{Remote: remote, RefsUpdated: []string{"FETCH_HEAD"}}, nil
}

func (e *CLIGitEngine) PullFastForward(ctx context.Context, path string, opts PullOptions) (bool, string, string, error) {
	oldHead := e.runOptional(ctx, path, "rev-parse", "HEAD")
	args := []string{"pull", "--ff-only"}
	if opts.Remote != "" {
		args = append(args, opts.Remote)
	}
	if opts.Branch != "" {
		args = append(args, opts.Branch)
	}
	if _, err := e.run(ctx, path, args...); err != nil {
		if strings.Contains(err.Error(), "Not possible to fast-forward") || strings.Contains(err.Error(), "diverg") {
			return false, oldHead, oldHead, ErrGitPullNotFastForward
		}
		return false, oldHead, oldHead, fmt.Errorf("%w: %v", ErrGitFetchFailed, err)
	}
	newHead := e.runOptional(ctx, path, "rev-parse", "HEAD")
	return oldHead != newHead, oldHead, newHead, nil
}

func (e *CLIGitEngine) Push(ctx context.Context, path string, opts PushOptions) (*GitPushResult, error) {
	oldHash := e.runOptional(ctx, path, "ls-remote", opts.Remote, opts.RemoteRef)
	if fields := strings.Fields(oldHash); len(fields) > 0 {
		oldHash = fields[0]
	} else {
		oldHash = ""
	}
	args := []string{"push"}
	if opts.SetUpstream {
		args = append(args, "--set-upstream")
	}
	args = append(args, opts.Remote, opts.LocalRef+":"+opts.RemoteRef)
	if _, err := e.run(ctx, path, args...); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitPushRejected, err)
	}
	newHash := e.runOptional(ctx, path, "rev-parse", opts.LocalRef)
	return &GitPushResult{OldHash: oldHash, NewHash: newHash, Ref: opts.RemoteRef}, nil
}

func (e *CLIGitEngine) ListRemotes(ctx context.Context, path string) ([]GitRemoteInfo, error) {
	out, err := e.run(ctx, path, "remote")
	if err != nil {
		return nil, err
	}
	result := make([]GitRemoteInfo, 0)
	for _, name := range strings.Fields(string(out)) {
		fetchURL := e.runOptional(ctx, path, "remote", "get-url", name)
		pushURL := e.runOptional(ctx, path, "remote", "get-url", "--push", name)
		scheme, host := remoteParts(fetchURL)
		result = append(result, GitRemoteInfo{Name: name, FetchURL: fetchURL, PushURL: pushURL, Scheme: scheme, Host: host, HasCredential: strings.Contains(fetchURL, "@")})
	}
	return result, nil
}

func remoteParts(raw string) (string, string) {
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" {
		return u.Scheme, u.Hostname()
	}
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		rest := raw[at+1:]
		if colon := strings.Index(rest, ":"); colon >= 0 {
			return "ssh", rest[:colon]
		}
	}
	if colon := strings.Index(raw, ":"); colon > 0 && !strings.Contains(raw[:colon], "/") {
		return "ssh", raw[:colon]
	}
	if filepath.IsAbs(raw) {
		return "file", "localhost"
	}
	return "", ""
}

func (e *CLIGitEngine) RemoteURL(ctx context.Context, path string, remote string) (string, error) {
	out, err := e.run(ctx, path, "remote", "get-url", remote)
	if err != nil {
		return "", ErrGitRemoteNotFound
	}
	return strings.TrimSpace(string(out)), nil
}
func (e *CLIGitEngine) SetRemoteURL(ctx context.Context, path string, remote string, rawURL string) error {
	if err := ValidateRemoteURL(rawURL); err != nil {
		return err
	}
	_, err := e.run(ctx, path, "remote", "set-url", remote, rawURL)
	return err
}
func (e *CLIGitEngine) AddRemote(ctx context.Context, path string, name string, rawURL string) error {
	if err := ValidateRemoteURL(rawURL); err != nil {
		return err
	}
	_, err := e.run(ctx, path, "remote", "add", name, rawURL)
	return err
}

func (e *CLIGitEngine) HasSymlinkEntries(ctx context.Context, path string) (bool, error) {
	out, err := e.run(ctx, path, "ls-files", "-s", "-z")
	if err != nil {
		return false, err
	}
	for _, rec := range bytes.Split(out, []byte{0}) {
		if bytes.HasPrefix(rec, []byte("120000 ")) {
			return true, nil
		}
	}
	return false, nil
}
func (e *CLIGitEngine) HasSubmodules(ctx context.Context, path string) (bool, error) {
	out, err := e.run(ctx, path, "ls-files", "-s", "-z")
	if err != nil {
		return false, nil
	}
	for _, rec := range bytes.Split(out, []byte{0}) {
		if bytes.HasPrefix(rec, []byte("160000 ")) {
			return true, nil
		}
	}
	_, statErr := os.Stat(filepath.Join(path, ".gitmodules"))
	return statErr == nil, nil
}
func (e *CLIGitEngine) DefaultBranchFromRemote(ctx context.Context, rawURL string, auth *GitAuthSpec) (string, error) {
	if err := ValidateRemoteURL(rawURL); err != nil {
		return "", err
	}
	out, err := e.run(ctx, "", "ls-remote", "--symref", rawURL, "HEAD")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "ref: ") && strings.HasSuffix(line, "\tHEAD") {
			ref := strings.TrimSuffix(strings.TrimPrefix(line, "ref: "), "\tHEAD")
			return strings.TrimPrefix(ref, "refs/heads/"), nil
		}
	}
	return "", nil
}
func (e *CLIGitEngine) ObjectStats(ctx context.Context, path string) (objects int, totalBytes int64, err error) {
	out, err := e.run(ctx, path, "count-objects", "-v")
	if err != nil {
		return 0, 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		p := strings.SplitN(line, ":", 2)
		if len(p) != 2 {
			continue
		}
		key, val := strings.TrimSpace(p[0]), strings.TrimSpace(p[1])
		n, _ := strconv.ParseInt(val, 10, 64)
		switch key {
		case "count", "in-pack":
			objects += int(n)
		case "size", "size-pack":
			totalBytes += n * 1024
		}
	}
	return objects, totalBytes, nil
}
func (e *CLIGitEngine) WorkingTreeStats(ctx context.Context, path string) (files int, totalBytes int64, err error) {
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !d.IsDir() {
			info, infoErr := d.Info()
			if infoErr != nil {
				return infoErr
			}
			files++
			totalBytes += info.Size()
		}
		return nil
	})
	return
}
func (e *CLIGitEngine) Close() error { return nil }

func splitNUL(data []byte) []string {
	parts := bytes.Split(data, []byte{0})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) > 0 {
			out = append(out, filepath.ToSlash(string(p)))
		}
	}
	return out
}
