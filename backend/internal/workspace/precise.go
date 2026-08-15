package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// SearchRequest defines a search query across a workspace.
type SearchRequest struct {
	WorkspaceID   string   `json:"workspaceId"`
	Query         string   `json:"query"`
	Regex         bool     `json:"regex"`
	IncludeGlobs  []string `json:"includeGlobs,omitempty"`
	ExcludeGlobs  []string `json:"excludeGlobs,omitempty"`
	MaxResults    int      `json:"maxResults"`
	ContextBefore int      `json:"contextBefore"`
	ContextAfter  int      `json:"contextAfter"`
}

// SearchMatch represents a single match within a file.
type SearchMatch struct {
	Path          string   `json:"path"`
	Line          int      `json:"line"`
	Match         string   `json:"match"`
	ContextBefore []string `json:"contextBefore,omitempty"`
	ContextAfter  []string `json:"contextAfter,omitempty"`
	FileSHA256    string   `json:"fileSha256"`
}

// SearchResult aggregates search matches.
type SearchResult struct {
	WorkspaceID string        `json:"workspaceId"`
	Matches     []SearchMatch `json:"matches"`
	Total       int           `json:"total"`
	Truncated   bool          `json:"truncated"`
}

// PatchRequest applies a unified-diff style patch to a single file.
type PatchRequest struct {
	WorkspaceID string `json:"workspaceId"`
	FilePath    string `json:"filePath"`
	BaseSHA256  string `json:"baseSha256"`
	Patch       string `json:"patch"`
}

// PatchResult reports the outcome of a patch operation.
type PatchResult struct {
	Applied   bool   `json:"applied"`
	FilePath  string `json:"filePath"`
	NewSHA256 string `json:"newSha256"`
}

// ReplaceRequest performs exact text replacement in a file.
type ReplaceRequest struct {
	WorkspaceID         string `json:"workspaceId"`
	FilePath            string `json:"filePath"`
	OldText             string `json:"oldText"`
	NewText             string `json:"newText"`
	ExpectedOccurrences int    `json:"expectedOccurrences"`
}

// ReplaceResult reports the outcome of a replace operation.
type ReplaceResult struct {
	Replaced          bool   `json:"replaced"`
	ActualOccurrences int    `json:"actualOccurrences"`
	FilePath          string `json:"filePath"`
}

// DiffRequest compares two file snapshots.
type DiffRequest struct {
	WorkspaceID string            `json:"workspaceId"`
	BeforeFiles map[string]string `json:"beforeFiles"`
	AfterFiles  map[string]string `json:"afterFiles"`
}

// DiffResult holds unified-diff comparison results.
type DiffResult struct {
	ChangedFiles []string `json:"changedFiles"`
	UnifiedDiff  string   `json:"unifiedDiff"`
	Additions    int      `json:"additions"`
	Deletions    int      `json:"deletions"`
}

// FileSnapshot captures the state of a file at a point in time.
type FileSnapshot struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Content []byte `json:"-"`
}

// EditTransaction tracks a multi-file edit with rollback capability.
type EditTransaction struct {
	ID           string                  `json:"id"`
	WorkspaceID  string                  `json:"workspaceId"`
	BaseFiles    map[string]FileSnapshot `json:"baseFiles"`
	ChangedFiles map[string]FileSnapshot `json:"changedFiles"`
	WrittenFiles map[string]FileSnapshot `json:"writtenFiles"`
	State        string                  `json:"state"`
	Journal      *TransactionJournal     `json:"journal"`
	Version      int                     `json:"version"`
	CreatedAt    time.Time               `json:"createdAt"`
	UpdatedAt    time.Time               `json:"updatedAt"`
	mu           sync.Mutex
}

// TransactionJournal records transaction lifecycle for crash recovery.
type TransactionJournal struct {
	TxID          string            `json:"txId"`
	WorkspaceID   string            `json:"workspaceId"`
	BaseHashes    map[string]string `json:"baseHashes"`
	ChangedHashes map[string]string `json:"changedHashes"`
	WrittenFiles  []string          `json:"writtenFiles"`
	State         string            `json:"state"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

const (
	TxStateActive       = "active"
	TxStateCommitted    = "committed"
	TxStateRolledBack   = "rolled_back"
	TxStateCommitFailed = "commit_failed"
)

// PreciseEditingService provides fine-grained file editing operations
// with content-addressable integrity checking and transaction support.
type PreciseEditingService interface {
	// Search finds literal or regex patterns across workspace files.
	Search(ctx context.Context, req SearchRequest) (*SearchResult, error)

	// Patch applies a unified-diff patch to a file, verifying base integrity.
	Patch(ctx context.Context, req PatchRequest) (*PatchResult, error)

	// Replace performs exact text substitution with occurrence count validation.
	Replace(ctx context.Context, req ReplaceRequest) (*ReplaceResult, error)

	// Diff computes a unified diff between before and after file snapshots.
	Diff(ctx context.Context, req DiffRequest) (*DiffResult, error)

	// BeginTransaction starts a new edit transaction, snapshotting base files.
	BeginTransaction(ctx context.Context, workspaceID string) (*EditTransaction, error)

	// ApplyPatchTx applies a patch within an existing transaction context.
	ApplyPatchTx(ctx context.Context, tx *EditTransaction, req PatchRequest) (*PatchResult, error)

	// PreviewDiff shows the diff between base files and current transaction state.
	PreviewDiff(ctx context.Context, tx *EditTransaction) (*DiffResult, error)

	// Commit writes all changed files from the transaction to the workspace.
	Commit(ctx context.Context, tx *EditTransaction) error

	// Rollback discards all changes and restores base file state.
	Rollback(ctx context.Context, tx *EditTransaction) error
}

// ComputeSHA256 returns the hex-encoded SHA-256 hash of the given data.
func ComputeSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// matchesAnyGlob reports whether the path matches any of the provided glob patterns.
func matchesAnyGlob(path string, globs []string) bool {
	for _, pattern := range globs {
		if matchGlob(path, pattern) {
			return true
		}
	}
	return false
}

// matchGlob performs simplified glob matching:
//   - "*" matches any sequence of non-separator characters
//   - "**" matches any sequence including separators
//   - "?" matches a single non-separator character
func matchGlob(path, pattern string) bool {
	patIdx := 0
	pathIdx := 0
	starIdx := -1
	starPathIdx := -1

	for pathIdx < len(path) {
		if patIdx < len(pattern) {
			p := pattern[patIdx]
			if p == '*' {
				// Check for ** (double star)
				if patIdx+1 < len(pattern) && pattern[patIdx+1] == '*' {
					// ** matches zero or more path segments
					patIdx += 2
					// Try to find a match for the rest of the pattern
					for i := pathIdx; i <= len(path); i++ {
						if matchGlob(path[i:], pattern[patIdx:]) {
							return true
						}
					}
					return false
				}
				starIdx = patIdx
				starPathIdx = pathIdx
				patIdx++
				continue
			}
			if p == '?' || p == path[pathIdx] {
				patIdx++
				pathIdx++
				continue
			}
		}
		if starIdx >= 0 {
			patIdx = starIdx + 1
			starPathIdx++
			pathIdx = starPathIdx
			continue
		}
		return false
	}

	// Consume trailing single stars
	for patIdx < len(pattern) && pattern[patIdx] == '*' {
		patIdx++
	}
	return patIdx == len(pattern)
}

// countOccurrences returns the number of non-overlapping occurrences of substr in s.
func countOccurrences(s, substr string) int {
	if substr == "" {
		return 0
	}
	count := 0
	for {
		idx := strings.Index(s, substr)
		if idx < 0 {
			break
		}
		count++
		s = s[idx+len(substr):]
	}
	return count
}
