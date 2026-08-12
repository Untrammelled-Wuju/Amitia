package git

import (
	"time"
)

type GitRepositoryTrust string

const (
	GitTrustIsolated  GitRepositoryTrust = "isolated"
	GitTrustUserLocal GitRepositoryTrust = "user_local"
	GitTrustImported  GitRepositoryTrust = "imported"
)

type GitIsolationMode string

const (
	GitIsolationModeClone    GitIsolationMode = "git_clone"
	GitIsolationModeSnapshot GitIsolationMode = "snapshot"
)

type GitMountConfig struct {
	Mode      GitIsolationMode `json:"mode"`
	RootKey   string           `json:"rootKey"`
	Source    *GitSourceSpec   `json:"source,omitempty"`
	CreatedBy string           `json:"createdBy"`
	ExpiresAt *time.Time       `json:"expiresAt,omitempty"`
}

type GitSourceSpec struct {
	Type     string `json:"type"`
	URI      string `json:"uri"`
	Ref      string `json:"ref,omitempty"`
	Depth    int    `json:"depth,omitempty"`
	RemoteID string `json:"remoteId,omitempty"`
}

type GitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type GitStatusResult struct {
	RepositoryRootURI string           `json:"repositoryRootURI"`
	Branch            string           `json:"branch"`
	Detached          bool             `json:"detached"`
	Head              string           `json:"head"`
	Upstream          string           `json:"upstream"`
	Ahead             int              `json:"ahead"`
	Behind            int              `json:"behind"`
	Clean             bool             `json:"clean"`
	Entries           []GitStatusEntry `json:"entries"`
	Truncated         bool             `json:"truncated"`
	Shallow           bool             `json:"shallow"`
	HasSubmodules     bool             `json:"hasSubmodules"`
}

type GitStatusEntry struct {
	URI       string `json:"uri"`
	Staging   string `json:"staging"`
	Worktree  string `json:"worktree"`
	Untracked bool   `json:"untracked"`
	Conflict  bool   `json:"conflict"`
	Ignored   bool   `json:"ignored,omitempty"`
	IsBinary  bool   `json:"isBinary,omitempty"`
	IsLFS     bool   `json:"isLFS,omitempty"`
	OldURI    string `json:"oldUri,omitempty"`
}

type GitDiffRequest struct {
	WorkspaceURI string   `json:"workspaceUri"`
	Mode         string   `json:"mode"`
	Base         string   `json:"base"`
	Target       string   `json:"target"`
	Paths        []string `json:"paths,omitempty"`
	MaxBytes     int      `json:"maxBytes,omitempty"`
}

type GitDiffResult struct {
	Files     []GitDiffFile `json:"files"`
	Stats     GitDiffStats  `json:"stats"`
	Truncated bool          `json:"truncated"`
}

type GitDiffFile struct {
	URI        string `json:"uri"`
	OldURI     string `json:"oldUri,omitempty"`
	OldSize    *int64 `json:"oldSize,omitempty"`
	NewSize    *int64 `json:"newSize,omitempty"`
	OldHash    string `json:"oldHash,omitempty"`
	NewHash    string `json:"newHash,omitempty"`
	IsBinary   bool   `json:"isBinary"`
	IsLFS      bool   `json:"isLFS,omitempty"`
	Status     string `json:"status"`
	Patch      string `json:"patch,omitempty"`
	PatchTrunc bool   `json:"patchTrunc,omitempty"`
}

type GitDiffStats struct {
	FilesChanged int `json:"filesChanged"`
	Insertions   int `json:"insertions"`
	Deletions    int `json:"deletions"`
	Bytes        int `json:"bytes"`
}

type GitLogRequest struct {
	WorkspaceURI string `json:"workspaceUri"`
	Limit        int    `json:"limit,omitempty"`
	Path         string `json:"path,omitempty"`
	Ref          string `json:"ref,omitempty"`
}

type GitLogResult struct {
	Entries           []GitCommitInfo `json:"entries"`
	HistoryIncomplete bool            `json:"historyIncomplete"`
}

type GitCommitInfo struct {
	Hash           string    `json:"hash"`
	AuthorName     string    `json:"authorName"`
	AuthorEmail    string    `json:"authorEmail"`
	CommitterName  string    `json:"committerName"`
	CommitterEmail string    `json:"committerEmail"`
	AuthoredAt     time.Time `json:"authoredAt"`
	CommittedAt    time.Time `json:"committedAt"`
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	Parents        []string  `json:"parents"`
}

type GitAddRequest struct {
	WorkspaceURI string   `json:"workspaceUri"`
	Paths        []string `json:"paths"`
	All          bool     `json:"all"`
	Force        bool     `json:"force"`
}

type GitAddResult struct {
	Staged []string `json:"staged"`
}

type GitRestoreRequest struct {
	WorkspaceURI string   `json:"workspaceUri"`
	Paths        []string `json:"paths"`
	Source       string   `json:"source,omitempty"`
	Staged       bool     `json:"staged"`
	Worktree     bool     `json:"worktree"`
}

type GitRestoreResult struct {
	Restored []string `json:"restored"`
}

type GitCommitRequest struct {
	WorkspaceURI string      `json:"workspaceUri"`
	Message      string      `json:"message"`
	Author       *GitIdentity `json:"author,omitempty"`
}

type GitCommitResult struct {
	Hash         string   `json:"hash"`
	Branch       string   `json:"branch"`
	ParentHashes []string `json:"parentHashes"`
	FilesChanged int      `json:"filesChanged"`
}

type GitBranchInfo struct {
	Name      string `json:"name"`
	Current   bool   `json:"current"`
	Remote    string `json:"remote,omitempty"`
	RemoteRef string `json:"remoteRef,omitempty"`
	Commit    string `json:"commit,omitempty"`
	Pull      string `json:"pull,omitempty"`
	Push      string `json:"push,omitempty"`
}

type GitBranchListResult struct {
	Branches []GitBranchInfo `json:"branches"`
	Current  string          `json:"current"`
	Detached bool            `json:"detached"`
}

type GitCheckoutRequest struct {
	WorkspaceURI string `json:"workspaceUri"`
	Branch       string `json:"branch"`
	Create       bool   `json:"create"`
	FromRef      string `json:"fromRef,omitempty"`
	Detach       bool   `json:"detach"`
	Force        bool   `json:"force"`
}

type GitCheckoutResult struct {
	Branch   string `json:"branch"`
	Detached bool   `json:"detached"`
	Head     string `json:"head"`
}

type GitRemoteInfo struct {
	Name          string `json:"name"`
	FetchURL      string `json:"fetchUrl"`
	PushURL       string `json:"pushUrl"`
	Scheme        string `json:"scheme"`
	HasCredential bool   `json:"hasCredential"`
	Host          string `json:"host"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
}

type GitFetchRequest struct {
	WorkspaceURI string `json:"workspaceUri"`
	Remote       string `json:"remote,omitempty"`
	Depth        int    `json:"depth,omitempty"`
	Deepen       int    `json:"deepen,omitempty"`
}

type GitFetchResult struct {
	Remote      string   `json:"remote"`
	RefsUpdated []string `json:"refsUpdated"`
}

type GitPullRequest struct {
	WorkspaceURI string `json:"workspaceUri"`
	Remote       string `json:"remote,omitempty"`
	Branch       string `json:"branch,omitempty"`
}

type GitPullResult struct {
	Fetched        bool   `json:"fetched"`
	FastForwarded  bool   `json:"fastForwarded"`
	OldHead        string `json:"oldHead,omitempty"`
	NewHead        string `json:"newHead,omitempty"`
}

type GitPushRequest struct {
	WorkspaceURI string `json:"workspaceUri"`
	Remote       string `json:"remote"`
	LocalRef     string `json:"localRef"`
	RemoteRef    string `json:"remoteRef"`
	SetUpstream  bool   `json:"setUpstream"`
}

type GitPushResult struct {
	Remote  string `json:"remote"`
	OldHash string `json:"oldHash,omitempty"`
	NewHash string `json:"newHash"`
	Ref     string `json:"ref"`
}

type IsolatedCreateRequest struct {
	Name               string         `json:"name"`
	Mode               string         `json:"mode"`
	SourceWorkspaceURI string         `json:"sourceWorkspaceUri,omitempty"`
	GitRemote          *GitRemoteSpec `json:"gitRemote,omitempty"`
	Ref                string         `json:"ref,omitempty"`
	Depth              int            `json:"depth,omitempty"`
	ReadOnly           bool           `json:"readOnly"`
	Lifetime           string         `json:"lifetime,omitempty"`
}

type GitRemoteSpec struct {
	RemoteID string `json:"remoteId,omitempty"`
	URL      string `json:"url,omitempty"`
	Ref      string `json:"ref,omitempty"`
}

type IsolatedWorkspaceInfo struct {
	Mode       string     `json:"mode"`
	Source     string     `json:"source"`
	Git        bool       `json:"git"`
	Dirty      bool       `json:"dirty"`
	Branch     string     `json:"branch"`
	Head       string     `json:"head"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt time.Time  `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	SizeBytes  int64      `json:"sizeBytes"`
}

type WorkspaceOwnerRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
