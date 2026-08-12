package git

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type fakeRepository struct {
	path           string
	branches       []string
	currentBranch  string
	head           string
	detached       bool
	clean          bool
	shallow        bool
	hasSubmodules  bool
	remotes        map[string]string
	index          map[string]string
	worktree       map[string][]byte
	commits        []GitCommitInfo
	symlinks       map[string]string
	hasExtFilter   bool
	mu             sync.Mutex
}

type FakeGitEngine struct {
	dataRoot    string
	repos       map[string]*fakeRepository
	mu          sync.Mutex
	initCalled  bool
	cloneCalled bool
}

func NewFakeGitEngine(dataRoot string) *FakeGitEngine {
	return &FakeGitEngine{
		dataRoot: dataRoot,
		repos:    make(map[string]*fakeRepository),
	}
}

func (e *FakeGitEngine) Detect(ctx context.Context, path string) (*RepositoryState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return nil, ErrRepositoryNotFound
	}
	return &RepositoryState{
		RepositoryPath: repo.path,
		Branch:         repo.currentBranch,
		Detached:       repo.detached,
		Head:           repo.head,
		Clean:          repo.clean,
		Shallow:        repo.shallow,
		HasSubmodules:  repo.hasSubmodules,
	}, nil
}

func (e *FakeGitEngine) Init(ctx context.Context, path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.repos[path]; ok {
		return ErrRepositoryInvalid
	}
	e.repos[path] = &fakeRepository{
		path:          path,
		branches:      []string{"main"},
		currentBranch: "main",
		head:          "abc123",
		clean:         true,
		remotes:       make(map[string]string),
		index:         make(map[string]string),
		worktree:      make(map[string][]byte),
		symlinks:      make(map[string]string),
	}
	e.initCalled = true
	return nil
}

func (e *FakeGitEngine) Clone(ctx context.Context, path string, opts CloneOptions) (*CloneResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cloneCalled = true
	branch := opts.Ref
	if branch == "" {
		branch = "main"
	}
	repo := &fakeRepository{
		path:          path,
		branches:      []string{branch},
		currentBranch: branch,
		head:          "abc123",
		clean:         true,
		remotes:       map[string]string{"origin": opts.URL},
		index:         make(map[string]string),
		worktree:      make(map[string][]byte),
		symlinks:      make(map[string]string),
	}
	if opts.Depth > 0 {
		repo.shallow = true
	}
	e.repos[path] = repo
	return &CloneResult{
		RepositoryPath: path,
		Branch:         branch,
		DefaultBranch:  branch,
		Head:           "abc123",
	}, nil
}

func (e *FakeGitEngine) Status(ctx context.Context, path string, includeIgnored bool, limit int) (*GitStatusResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return nil, ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	var entries []GitStatusEntry
	count := 0
	for fp := range repo.index {
		if count >= limit {
			break
		}
		staging := "M"
		worktree := " "
		if _, ok := repo.worktree[fp]; !ok {
			worktree = "D"
		}
		entries = append(entries, GitStatusEntry{
			URI:      "amitya://workspace/@test/" + fp,
			Staging:  staging,
			Worktree: worktree,
		})
		count++
	}
	return &GitStatusResult{
		Branch:        repo.currentBranch,
		Head:          repo.head,
		Detached:      repo.detached,
		Clean:         repo.clean,
		Entries:       entries,
		Shallow:       repo.shallow,
		HasSubmodules: repo.hasSubmodules,
	}, nil
}

func (e *FakeGitEngine) Diff(ctx context.Context, path string, opts DiffOptions) (*GitDiffResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return nil, ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	var files []GitDiffFile
	for fp := range repo.index {
		status := "modified"
		files = append(files, GitDiffFile{
			URI:    "amitya://workspace/@test/" + fp,
			Status: status,
			Patch:  "--- a/" + fp + "\n+++ b/" + fp,
		})
	}
	return &GitDiffResult{
		Files: files,
		Stats: GitDiffStats{FilesChanged: len(files)},
	}, nil
}

func (e *FakeGitEngine) Log(ctx context.Context, path string, opts LogOptions) (*GitLogResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return nil, ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	limit := opts.Limit
	if limit == 0 {
		limit = 50
	}
	if limit > len(repo.commits) {
		limit = len(repo.commits)
	}
	entries := make([]GitCommitInfo, limit)
	copy(entries, repo.commits[:limit])
	return &GitLogResult{
		Entries:           entries,
		HistoryIncomplete: repo.shallow,
	}, nil
}

func (e *FakeGitEngine) Add(ctx context.Context, path string, opts AddOptions) ([]string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return nil, ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	var staged []string
	if opts.All {
		for fp := range repo.worktree {
			repo.index[fp] = fmt.Sprintf("%x", time.Now().UnixNano())
			staged = append(staged, fp)
		}
	} else {
		for _, p := range opts.Paths {
			if _, ok := repo.worktree[p]; ok {
				repo.index[p] = fmt.Sprintf("%x", time.Now().UnixNano())
				staged = append(staged, p)
			}
		}
	}
	return staged, nil
}

func (e *FakeGitEngine) Restore(ctx context.Context, path string, opts RestoreOptions) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, p := range opts.Paths {
		if opts.Worktree {
			if _, ok := repo.index[p]; ok {
				repo.worktree[p] = []byte("restored")
			}
		}
		if opts.Staged {
			delete(repo.index, p)
		}
	}
	return nil
}

func (e *FakeGitEngine) Commit(ctx context.Context, path string, opts CommitOptions) (*GitCommitResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return nil, ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.index) == 0 {
		return nil, ErrGitCommitFailed
	}
	hash := fmt.Sprintf("commit-%d", len(repo.commits)+1)
	commit := GitCommitInfo{
		Hash:    hash,
		Subject: opts.Message,
		Parents: []string{repo.head},
	}
	if opts.Author != nil {
		commit.AuthorName = opts.Author.Name
		commit.AuthorEmail = opts.Author.Email
	}
	repo.head = hash
	repo.commits = append([]GitCommitInfo{commit}, repo.commits...)
	repo.index = make(map[string]string)
	repo.clean = true
	return &GitCommitResult{
		Hash:         hash,
		Branch:       repo.currentBranch,
		ParentHashes: commit.Parents,
		FilesChanged: len(repo.index),
	}, nil
}

func (e *FakeGitEngine) ListBranches(ctx context.Context, path string) (*GitBranchListResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return nil, ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	var branches []GitBranchInfo
	for _, b := range repo.branches {
		branches = append(branches, GitBranchInfo{
			Name:    b,
			Current: b == repo.currentBranch,
		})
	}
	return &GitBranchListResult{
		Branches: branches,
		Current:  repo.currentBranch,
		Detached: repo.detached,
	}, nil
}

func (e *FakeGitEngine) Checkout(ctx context.Context, path string, opts CheckoutOptions) (*GitCheckoutResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return nil, ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if !repo.clean && !opts.Force {
		return nil, ErrGitDirty
	}
	if opts.Detach {
		repo.detached = true
		repo.currentBranch = opts.Branch
		return &GitCheckoutResult{Branch: opts.Branch, Detached: true, Head: repo.head}, nil
	}
	if opts.Create {
		repo.branches = append(repo.branches, opts.Branch)
	}
	for _, b := range repo.branches {
		if b == opts.Branch {
			repo.currentBranch = b
			repo.detached = false
			return &GitCheckoutResult{Branch: b, Head: repo.head}, nil
		}
	}
	return nil, ErrBranchNotFound
}

func (e *FakeGitEngine) Fetch(ctx context.Context, path string, opts FetchOptions) (*GitFetchResult, error) {
	return &GitFetchResult{Remote: opts.Remote, RefsUpdated: []string{"refs/remotes/origin/main"}}, nil
}

func (e *FakeGitEngine) PullFastForward(ctx context.Context, path string, opts PullOptions) (bool, string, string, error) {
	return true, "old123", "new456", nil
}

func (e *FakeGitEngine) Push(ctx context.Context, path string, opts PushOptions) (*GitPushResult, error) {
	return &GitPushResult{
		OldHash: "old123",
		NewHash: "new456",
		Ref:     opts.RemoteRef,
	}, nil
}

func (e *FakeGitEngine) ListRemotes(ctx context.Context, path string) ([]GitRemoteInfo, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return nil, ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	var remotes []GitRemoteInfo
	for name, url := range repo.remotes {
		scheme := "https"
		if strings.HasPrefix(url, "ssh://") || strings.Contains(url, "@") {
			scheme = "ssh"
		}
		remotes = append(remotes, GitRemoteInfo{
			Name:     name,
			FetchURL: url,
			PushURL:  url,
			Scheme:   scheme,
		})
	}
	return remotes, nil
}

func (e *FakeGitEngine) RemoteURL(ctx context.Context, path string, remote string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return "", ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if url, ok := repo.remotes[remote]; ok {
		return url, nil
	}
	return "", ErrGitRemoteNotFound
}

func (e *FakeGitEngine) SetRemoteURL(ctx context.Context, path string, remote string, url string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.remotes[remote] = url
	return nil
}

func (e *FakeGitEngine) AddRemote(ctx context.Context, path string, name string, url string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return ErrRepositoryNotFound
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.remotes[name] = url
	return nil
}

func (e *FakeGitEngine) HasSymlinkEntries(ctx context.Context, path string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return false, ErrRepositoryNotFound
	}
	return len(repo.symlinks) > 0, nil
}

func (e *FakeGitEngine) HasSubmodules(ctx context.Context, path string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return false, ErrRepositoryNotFound
	}
	return repo.hasSubmodules, nil
}

func (e *FakeGitEngine) DefaultBranchFromRemote(ctx context.Context, url string, auth *GitAuthSpec) (string, error) {
	return "main", nil
}

func (e *FakeGitEngine) ObjectStats(ctx context.Context, path string) (int, int64, error) {
	return 10, 1024, nil
}

func (e *FakeGitEngine) WorkingTreeStats(ctx context.Context, path string) (int, int64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return 0, 0, ErrRepositoryNotFound
	}
	var total int64
	for _, data := range repo.worktree {
		total += int64(len(data))
	}
	return len(repo.worktree), total, nil
}

func (e *FakeGitEngine) Close() error {
	return nil
}

func (e *FakeGitEngine) AddFile(path string, filePath string, content []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return
	}
	repo.worktree[filePath] = content
}

func (e *FakeGitEngine) AddSymlink(path string, linkPath string, target string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return
	}
	repo.symlinks[linkPath] = target
}

func (e *FakeGitEngine) SetClean(path string, clean bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	repo, ok := e.repos[path]
	if !ok {
		return
	}
	repo.clean = clean
}
