package git

import "context"

type RepositoryState struct {
	RepositoryPath string
	Branch         string
	Detached       bool
	Head           string
	Upstream       string
	Ahead          int
	Behind         int
	Clean          bool
	Shallow        bool
	HasSubmodules  bool
}

type CloneOptions struct {
	URL        string
	Ref        string
	Depth      int
	Auth       *GitAuthSpec
	ProgressFn func(phase string, current, total int64)
	CancelFn   func() bool
}

type CloneResult struct {
	RepositoryPath string
	Branch         string
	DefaultBranch  string
	Head           string
}

type GitAuthSpec struct {
	Type          string
	CredentialRef string
	SSHTrustRef   string
}

type AddOptions struct {
	Paths []string
	All   bool
	Force bool
}

type RestoreOptions struct {
	Paths    []string
	Source   string
	Staged   bool
	Worktree bool
}

type CommitOptions struct {
	Message string
	Author  *GitIdentity
}

type FetchOptions struct {
	Remote   string
	Depth    int
	Deepen   int
	Auth     *GitAuthSpec
	CancelFn func() bool
}

type FetchResult struct {
	RefsUpdated []string
}

type PullOptions struct {
	Remote   string
	Branch   string
	Auth     *GitAuthSpec
	CancelFn func() bool
}

type PushOptions struct {
	Remote      string
	LocalRef    string
	RemoteRef   string
	SetUpstream bool
	Auth        *GitAuthSpec
	Force       bool
	CancelFn    func() bool
}

type PushResult struct {
	OldHash string
	NewHash string
	Ref     string
}

type LogOptions struct {
	Limit int
	Path  string
	Ref   string
}

type BranchListResult struct {
	Branches []GitBranchInfo
	Current  string
	Detached bool
}

type CheckoutOptions struct {
	Branch  string
	Create  bool
	FromRef string
	Detach  bool
	Force   bool
}

type CheckoutResult struct {
	Branch   string
	Detached bool
	Head     string
}

type DiffOptions struct {
	Mode     string
	Base     string
	Target   string
	Paths    []string
	MaxBytes int
}

type GitEngine interface {
	Detect(ctx context.Context, path string) (*RepositoryState, error)

	Init(ctx context.Context, path string) error

	Clone(ctx context.Context, path string, opts CloneOptions) (*CloneResult, error)

	Status(ctx context.Context, path string, includeIgnored bool, limit int) (*GitStatusResult, error)

	Diff(ctx context.Context, path string, opts DiffOptions) (*GitDiffResult, error)

	Log(ctx context.Context, path string, opts LogOptions) (*GitLogResult, error)

	Add(ctx context.Context, path string, opts AddOptions) ([]string, error)

	Restore(ctx context.Context, path string, opts RestoreOptions) error

	Commit(ctx context.Context, path string, opts CommitOptions) (*GitCommitResult, error)

	ListBranches(ctx context.Context, path string) (*GitBranchListResult, error)

	Checkout(ctx context.Context, path string, opts CheckoutOptions) (*GitCheckoutResult, error)

	Fetch(ctx context.Context, path string, opts FetchOptions) (*GitFetchResult, error)

	PullFastForward(ctx context.Context, path string, opts PullOptions) (fetched bool, oldHead string, newHead string, err error)

	Push(ctx context.Context, path string, opts PushOptions) (*GitPushResult, error)

	ListRemotes(ctx context.Context, path string) ([]GitRemoteInfo, error)

	RemoteURL(ctx context.Context, path string, remote string) (string, error)

	SetRemoteURL(ctx context.Context, path string, remote string, url string) error

	AddRemote(ctx context.Context, path string, name string, url string) error

	HasSymlinkEntries(ctx context.Context, path string) (bool, error)

	HasSubmodules(ctx context.Context, path string) (bool, error)

	DefaultBranchFromRemote(ctx context.Context, url string, auth *GitAuthSpec) (string, error)

	ObjectStats(ctx context.Context, path string) (objects int, totalBytes int64, err error)

	WorkingTreeStats(ctx context.Context, path string) (files int, totalBytes int64, err error)

	Close() error
}

type GitEngineFactory func(dataRoot string) (GitEngine, error)
