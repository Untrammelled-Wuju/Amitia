package workspace

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// defaultPreciseEditingService implements PreciseEditingService
// by delegating file I/O to the workspace Service.
type defaultPreciseEditingService struct {
	service *Service
}

// NewDefaultPreciseEditingService creates a PreciseEditingService backed by the given workspace Service.
func NewDefaultPreciseEditingService(svc *Service) PreciseEditingService {
	return &defaultPreciseEditingService{service: svc}
}

// Search scans workspace files for the configured query.
func (s *defaultPreciseEditingService) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if req.Query == "" {
		return &SearchResult{WorkspaceID: req.WorkspaceID, Matches: nil}, nil
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 100
	}

	var re *regexp.Regexp
	if req.Regex {
		var err error
		re, err = regexp.Compile(req.Query)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid regex: %v", ErrInvalidPath, err)
		}
	}

	result := &SearchResult{WorkspaceID: req.WorkspaceID}
	truncated := false

	err := s.walkWorkspace(ctx, req.WorkspaceID, func(filePath string, content []byte) error {
		// Apply include/exclude globs
		if len(req.IncludeGlobs) > 0 && !matchesAnyGlob(filePath, req.IncludeGlobs) {
			return nil
		}
		if len(req.ExcludeGlobs) > 0 && matchesAnyGlob(filePath, req.ExcludeGlobs) {
			return nil
		}

		fileHash := ComputeSHA256(content)
		matches := searchInContent(content, re, req.Query, req.ContextBefore, req.ContextAfter, fileHash)

		if len(result.Matches)+len(matches) > req.MaxResults {
			need := req.MaxResults - len(result.Matches)
			if need > 0 {
				result.Matches = append(result.Matches, matches[:need]...)
			}
			truncated = true
			return nil
		}
		result.Matches = append(result.Matches, matches...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	result.Total = len(result.Matches)
	result.Truncated = truncated
	return result, nil
}

// Patch applies a unified-diff style patch to a file.
func (s *defaultPreciseEditingService) Patch(ctx context.Context, req PatchRequest) (*PatchResult, error) {
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("%w: workspace ID required", ErrInvalidPath)
	}
	if req.FilePath == "" {
		return nil, fmt.Errorf("%w: file path required", ErrInvalidPath)
	}

	content, err := s.readFile(ctx, req.WorkspaceID, req.FilePath)
	if err != nil {
		return nil, err
	}

	// Verify base SHA256
	if req.BaseSHA256 != "" {
		actualHash := ComputeSHA256(content)
		if actualHash != req.BaseSHA256 {
			return nil, fmt.Errorf("%w: base SHA256 mismatch: expected %s, got %s", ErrWriteFailed, req.BaseSHA256, actualHash)
		}
	}

	// Apply patch
	patched, err := applyUnifiedPatch(content, req.Patch)
	if err != nil {
		return nil, fmt.Errorf("%w: patch apply failed: %v", ErrWriteFailed, err)
	}

	// Write back
	if err := s.writeFile(ctx, req.WorkspaceID, req.FilePath, patched); err != nil {
		return nil, err
	}

	return &PatchResult{
		Applied:   true,
		FilePath:  req.FilePath,
		NewSHA256: ComputeSHA256(patched),
	}, nil
}

// Replace performs exact text replacement in a file.
func (s *defaultPreciseEditingService) Replace(ctx context.Context, req ReplaceRequest) (*ReplaceResult, error) {
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("%w: workspace ID required", ErrInvalidPath)
	}
	if req.FilePath == "" {
		return nil, fmt.Errorf("%w: file path required", ErrInvalidPath)
	}
	if req.OldText == "" {
		return nil, fmt.Errorf("%w: old text must not be empty", ErrInvalidPath)
	}

	content, err := s.readFile(ctx, req.WorkspaceID, req.FilePath)
	if err != nil {
		return nil, err
	}

	original := string(content)
	actualCount := countOccurrences(original, req.OldText)

	if req.ExpectedOccurrences > 0 && actualCount != req.ExpectedOccurrences {
		return &ReplaceResult{
			Replaced:          false,
			ActualOccurrences: actualCount,
			FilePath:          req.FilePath,
		}, fmt.Errorf("%w: expected %d occurrences, found %d", ErrWriteFailed, req.ExpectedOccurrences, actualCount)
	}

	newContent := strings.ReplaceAll(original, req.OldText, req.NewText)
	if err := s.writeFile(ctx, req.WorkspaceID, req.FilePath, []byte(newContent)); err != nil {
		return nil, err
	}

	return &ReplaceResult{
		Replaced:          true,
		ActualOccurrences: actualCount,
		FilePath:          req.FilePath,
	}, nil
}

// Diff computes a unified diff between before and after file snapshots.
func (s *defaultPreciseEditingService) Diff(ctx context.Context, req DiffRequest) (*DiffResult, error) {
	result := &DiffResult{
		ChangedFiles: []string{},
	}

	var additions, deletions int
	var diffParts []string

	// Collect all file paths
	allFiles := make(map[string]bool)
	for p := range req.BeforeFiles {
		allFiles[p] = true
	}
	for p := range req.AfterFiles {
		allFiles[p] = true
	}

	for filePath := range allFiles {
		before := req.BeforeFiles[filePath]
		after := req.AfterFiles[filePath]

		if before == after {
			continue
		}

		result.ChangedFiles = append(result.ChangedFiles, filePath)

		beforeLines := splitLines(before)
		afterLines := splitLines(after)

		diff, adds, dels := unifiedDiff(filePath, beforeLines, afterLines)
		additions += adds
		deletions += dels
		diffParts = append(diffParts, diff)
	}

	result.UnifiedDiff = strings.Join(diffParts, "\n")
	result.Additions = additions
	result.Deletions = deletions
	return result, nil
}

// BeginTransaction creates a new edit transaction for the workspace.
func (s *defaultPreciseEditingService) BeginTransaction(ctx context.Context, workspaceID string) (*EditTransaction, error) {
	return &EditTransaction{
		ID:           uuid.NewString(),
		WorkspaceID:  workspaceID,
		BaseFiles:    make(map[string]FileSnapshot),
		ChangedFiles: make(map[string]FileSnapshot),
		State:        TxStateActive,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

// ApplyPatchTx applies a patch within a transaction, tracking base and changed state.
func (s *defaultPreciseEditingService) ApplyPatchTx(ctx context.Context, tx *EditTransaction, req PatchRequest) (*PatchResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("%w: transaction is nil", ErrOperationUnsupported)
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.State != TxStateActive {
		return nil, fmt.Errorf("%w: transaction is not active (state: %s)", ErrOperationUnsupported, tx.State)
	}

	// Read current file content
	content, err := s.readFile(ctx, tx.WorkspaceID, req.FilePath)
	if err != nil {
		return nil, err
	}

	// Snapshot base file if not already captured
	if _, exists := tx.BaseFiles[req.FilePath]; !exists {
		tx.BaseFiles[req.FilePath] = FileSnapshot{
			Path:    req.FilePath,
			SHA256:  ComputeSHA256(content),
			Content: append([]byte(nil), content...),
		}
	}

	// Verify base SHA256 against the snapshot's hash
	if req.BaseSHA256 != "" {
		baseSnap := tx.BaseFiles[req.FilePath]
		if baseSnap.SHA256 != req.BaseSHA256 {
			return nil, fmt.Errorf("%w: base SHA256 mismatch: expected %s, got %s", ErrWriteFailed, req.BaseSHA256, baseSnap.SHA256)
		}
	}

	// Apply patch
	patched, err := applyUnifiedPatch(content, req.Patch)
	if err != nil {
		return nil, fmt.Errorf("%w: patch apply failed: %v", ErrWriteFailed, err)
	}

	// Update changed files in-memory only
	tx.ChangedFiles[req.FilePath] = FileSnapshot{
		Path:    req.FilePath,
		SHA256:  ComputeSHA256(patched),
		Content: patched,
	}

	return &PatchResult{
		Applied:   true,
		FilePath:  req.FilePath,
		NewSHA256: ComputeSHA256(patched),
	}, nil
}

// PreviewDiff shows the diff between base files and current transaction state without writing.
func (s *defaultPreciseEditingService) PreviewDiff(ctx context.Context, tx *EditTransaction) (*DiffResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("%w: transaction is nil", ErrOperationUnsupported)
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.State != TxStateActive {
		return nil, fmt.Errorf("%w: transaction is not active (state: %s)", ErrOperationUnsupported, tx.State)
	}

	result := &DiffResult{
		ChangedFiles: []string{},
	}

	var (
		additions int
		deletions int
	)
	var diffParts []string

	for filePath, changedSnap := range tx.ChangedFiles {
		baseSnap, hasBase := tx.BaseFiles[filePath]
		before := ""
		if hasBase {
			before = string(baseSnap.Content)
		}
		after := string(changedSnap.Content)

		if before == after {
			continue
		}

		result.ChangedFiles = append(result.ChangedFiles, filePath)

		beforeLines := splitLines(before)
		afterLines := splitLines(after)

		diff, adds, dels := unifiedDiff(filePath, beforeLines, afterLines)
		additions += adds
		deletions += dels
		diffParts = append(diffParts, diff)
	}

	result.UnifiedDiff = strings.Join(diffParts, "\n")
	result.Additions = additions
	result.Deletions = deletions
	return result, nil
}

// Commit writes all changed files from the transaction to disk.
func (s *defaultPreciseEditingService) Commit(ctx context.Context, tx *EditTransaction) error {
	if tx == nil {
		return fmt.Errorf("%w: transaction is nil", ErrOperationUnsupported)
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.State != TxStateActive {
		return fmt.Errorf("%w: transaction is not active (state: %s)", ErrOperationUnsupported, tx.State)
	}

	// Write all changed files to disk
	for filePath, snap := range tx.ChangedFiles {
		if err := s.writeFile(ctx, tx.WorkspaceID, filePath, snap.Content); err != nil {
			// Transaction failure must not leave partial state:
			// we stop at the first error and do not mark committed.
			return fmt.Errorf("%w: failed to write %q: %v", ErrWriteFailed, filePath, err)
		}
	}

	tx.State = TxStateCommitted
	return nil
}

// Rollback discards all changes. Since ApplyPatchTx only stores changes
// in-memory, rollback simply clears them and marks the transaction.
func (s *defaultPreciseEditingService) Rollback(ctx context.Context, tx *EditTransaction) error {
	if tx == nil {
		return fmt.Errorf("%w: transaction is nil", ErrOperationUnsupported)
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.State != TxStateActive {
		return fmt.Errorf("%w: transaction is not active (state: %s)", ErrOperationUnsupported, tx.State)
	}

	// In-memory only: clear changed files. Workspace state is untouched.
	tx.ChangedFiles = make(map[string]FileSnapshot)
	tx.State = TxStateRolledBack
	return nil
}

// readFile reads a file from the workspace service using the mount-relative path.
func (s *defaultPreciseEditingService) readFile(ctx context.Context, workspaceID string, filePath string) ([]byte, error) {
	uri := s.buildFileURI(workspaceID, filePath)
	result, err := s.service.Read(ctx, uri, ReadOptions{})
	if err != nil {
		return nil, fmt.Errorf("%w: read %q: %v", ErrReadFailed, filePath, err)
	}
	return result.Content, nil
}

// writeFile writes content to a file in the workspace.
func (s *defaultPreciseEditingService) writeFile(ctx context.Context, workspaceID string, filePath string, content []byte) error {
	uri := s.buildFileURI(workspaceID, filePath)
	_, err := s.service.Write(ctx, uri, strings.NewReader(string(content)), WriteOptions{Overwrite: true, Atomic: true})
	if err != nil {
		return fmt.Errorf("%w: write %q: %v", ErrWriteFailed, filePath, err)
	}
	return nil
}

// buildFileURI constructs the full workspace URI for a file path.
func (s *defaultPreciseEditingService) buildFileURI(workspaceID string, filePath string) string {
	base := MountURI(WorkspaceID(workspaceID))
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + filePath
}

// walkWorkspace recursively lists files in a workspace and invokes the callback for each.
func (s *defaultPreciseEditingService) walkWorkspace(ctx context.Context, workspaceID string, cb func(filePath string, content []byte) error) error {
	return s.walkDir(ctx, workspaceID, "", cb)
}

func (s *defaultPreciseEditingService) walkDir(ctx context.Context, workspaceID string, dir string, cb func(filePath string, content []byte) error) error {
	uri := s.buildFileURI(workspaceID, dir)
	result, err := s.service.List(ctx, uri, ListOptions{})
	if err != nil {
		return fmt.Errorf("%w: list %q: %v", ErrListFailed, dir, err)
	}

	for _, entry := range result.Entries {
		relPath := entry.Name
		if dir != "" {
			relPath = dir + "/" + entry.Name
		}

		if entry.Type == WorkspaceEntryTypeDirectory {
			if err := s.walkDir(ctx, workspaceID, relPath, cb); err != nil {
				return err
			}
			continue
		}

		content, err := s.readFile(ctx, workspaceID, relPath)
		if err != nil {
			// Skip unreadable files
			continue
		}

		if err := cb(relPath, content); err != nil {
			return err
		}
	}
	return nil
}

// searchInContent finds all matches in a single file's content.
func searchInContent(content []byte, re *regexp.Regexp, literal string, ctxBefore, ctxAfter int, fileHash string) []SearchMatch {
	lines := splitLines(string(content))
	var matches []SearchMatch

	for lineNum, line := range lines {
		if re != nil {
			locs := re.FindAllStringIndex(line, -1)
			for _, loc := range locs {
				match := SearchMatch{
					Line:       lineNum + 1,
					Match:      line[loc[0]:loc[1]],
					FileSHA256: fileHash,
				}
				if ctxBefore > 0 {
					start := lineNum - ctxBefore
					if start < 0 {
						start = 0
					}
					for i := start; i < lineNum; i++ {
						match.ContextBefore = append(match.ContextBefore, lines[i])
					}
				}
				if ctxAfter > 0 {
					end := lineNum + ctxAfter
					if end >= len(lines) {
						end = len(lines) - 1
					}
					for i := lineNum + 1; i <= end; i++ {
						match.ContextAfter = append(match.ContextAfter, lines[i])
					}
				}
				matches = append(matches, match)
			}
		} else {
			// Literal search
			searchStart := 0
			for searchStart <= len(line)-len(literal) {
				idx := strings.Index(line[searchStart:], literal)
				if idx < 0 {
					break
				}
				absIdx := searchStart + idx
				match := SearchMatch{
					Line:       lineNum + 1,
					Match:      literal,
					FileSHA256: fileHash,
				}
				if ctxBefore > 0 {
					start := lineNum - ctxBefore
					if start < 0 {
						start = 0
					}
					for i := start; i < lineNum; i++ {
						match.ContextBefore = append(match.ContextBefore, lines[i])
					}
				}
				if ctxAfter > 0 {
					end := lineNum + ctxAfter
					if end >= len(lines) {
						end = len(lines) - 1
					}
					for i := lineNum + 1; i <= end; i++ {
						match.ContextAfter = append(match.ContextAfter, lines[i])
					}
				}
				matches = append(matches, match)
				searchStart = absIdx + len(literal)
			}
		}
	}

	return matches
}

// splitLines splits content into lines, preserving behavior for both LF and CRLF.
func splitLines(content string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// applyUnifiedPatch applies a simplified unified-diff patch to original content.
// It supports @@ -l,s +l,s @@ hunk headers and +/- line prefixes.
type hunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []string
}

func applyUnifiedPatch(original []byte, patchText string) ([]byte, error) {
	contentLines := splitLines(string(original))
	var result []string

	scanner := bufio.NewScanner(strings.NewReader(patchText))
	var hunks []hunk
	var current *hunk

	for scanner.Scan() {
		line := scanner.Text()

		if len(line) >= 4 && strings.HasPrefix(line, "@@") {
			// Parse hunk header: @@ -oldStart,oldCount +newStart,newCount @@
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			current = &h
			hunks = append(hunks, h)
			continue
		}

		if current == nil {
			continue
		}

		hidx := len(hunks) - 1
		hunks[hidx].lines = append(hunks[hidx].lines, line)
	}

	// If no hunks found, the patch might be a simple replacement
	if len(hunks) == 0 {
		return original, nil
	}

	// Apply hunks in order
	contentIdx := 0
	for _, h := range hunks {
		// Copy unchanged lines before this hunk
		for contentIdx < h.oldStart && contentIdx < len(contentLines) {
			result = append(result, contentLines[contentIdx])
			contentIdx++
		}

		// Apply hunk lines
		hunkIdx := 0
		for hunkIdx < len(h.lines) {
			hline := h.lines[hunkIdx]
			if len(hline) == 0 {
				// Treat empty context lines as valid context
				if contentIdx < len(contentLines) {
					result = append(result, contentLines[contentIdx])
					contentIdx++
				}
				hunkIdx++
				continue
			}

			switch hline[0] {
			case ' ':
				// Context line: must match original
				if contentIdx >= len(contentLines) || contentLines[contentIdx] != hline[1:] {
					return nil, fmt.Errorf("context mismatch at line %d: got %q, expected %q",
						contentIdx+1, safeGet(contentLines, contentIdx), hline[1:])
				}
				result = append(result, hline[1:])
				contentIdx++
			case '-':
				// Removal line: must match original
				if contentIdx >= len(contentLines) || contentLines[contentIdx] != hline[1:] {
					return nil, fmt.Errorf("removal mismatch at line %d: got %q, expected %q",
						contentIdx+1, safeGet(contentLines, contentIdx), hline[1:])
				}
				contentIdx++ // skip this line in original
			case '+':
				// Addition line
				result = append(result, hline[1:])
			default:
				return nil, fmt.Errorf("unexpected patch line: %q", hline)
			}
			hunkIdx++
		}
	}

	// Copy remaining lines after last hunk
	for contentIdx < len(contentLines) {
		result = append(result, contentLines[contentIdx])
		contentIdx++
	}

	return []byte(strings.Join(result, "\n")), nil
}

// parseHunkHeader parses a unified diff hunk header line.
func parseHunkHeader(line string) (hunk, error) {
	// Strip @@ markers
	inner := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(line, "@@"), "@@"))
	parts := strings.Split(inner, " ")
	if len(parts) != 2 {
		return hunk{}, fmt.Errorf("invalid hunk header: %s", line)
	}

	var oldStart, oldCount, newStart, newCount int

	oldPart := strings.TrimPrefix(parts[0], "-")
	if strings.Contains(oldPart, ",") {
		fmt.Sscanf(oldPart, "%d,%d", &oldStart, &oldCount)
	} else {
		fmt.Sscanf(oldPart, "%d", &oldStart)
		oldCount = 1
	}

	newPart := strings.TrimPrefix(parts[1], "+")
	if strings.Contains(newPart, ",") {
		fmt.Sscanf(newPart, "%d,%d", &newStart, &newCount)
	} else {
		fmt.Sscanf(newPart, "%d", &newStart)
		newCount = 1
	}

	return hunk{
		oldStart: oldStart - 1, // Convert to 0-indexed
		oldCount: oldCount,
		newStart: newStart - 1,
		newCount: newCount,
		lines:    []string{},
	}, nil
}

func safeGet(lines []string, idx int) string {
	if idx >= 0 && idx < len(lines) {
		return lines[idx]
	}
	return "<EOF>"
}

// unifiedDiff computes a unified diff between two line slices.
func unifiedDiff(filePath string, before, after []string) (string, int, int) {
	// Simple LCS-based diff
	lcs := computeLCS(before, after)

	additions := 0
	deletions := 0

	var diffLines []string
	diffLines = append(diffLines, fmt.Sprintf("--- a/%s", filePath))
	diffLines = append(diffLines, fmt.Sprintf("+++ b/%s", filePath))

	beforeIdx := 0
	afterIdx := 0
	lcsIdx := 0

	var hunkLines []string
	hunkStartBefore := -1
	hunkStartAfter := -1

	flushHunk := func() {
		if len(hunkLines) == 0 {
			return
		}
		bCount := 0
		aCount := 0
		for _, hl := range hunkLines {
			if len(hl) > 0 && hl[0] != '-' {
				aCount++
			}
			if len(hl) > 0 && hl[0] != '+' {
				bCount++
			}
		}
		diffLines = append(diffLines, fmt.Sprintf("@@ -%d,%d +%d,%d @@",
			hunkStartBefore+1, bCount, hunkStartAfter+1, aCount))
		diffLines = append(diffLines, hunkLines...)
		hunkLines = nil
		hunkStartBefore = -1
		hunkStartAfter = -1
	}

	for beforeIdx < len(before) || afterIdx < len(after) {
		if lcsIdx < len(lcs) && beforeIdx < len(before) && afterIdx < len(after) &&
			before[beforeIdx] == lcs[lcsIdx] && after[afterIdx] == lcs[lcsIdx] {
			// Match
			if len(hunkLines) > 0 {
				// Add context line
				hunkLines = append(hunkLines, " "+before[beforeIdx])
				if len(hunkLines) > 6 {
					flushHunk()
				}
			}
			beforeIdx++
			afterIdx++
			lcsIdx++
			continue
		}

		// Determine if this is a deletion, insertion, or substitution
		isDeletion := beforeIdx < len(before) && (lcsIdx >= len(lcs) || before[beforeIdx] != lcs[lcsIdx])
		isInsertion := afterIdx < len(after) && (lcsIdx >= len(lcs) || after[afterIdx] != lcs[lcsIdx])

		if isDeletion {
			if hunkStartBefore < 0 {
				hunkStartBefore = beforeIdx
				hunkStartAfter = afterIdx
			}
			hunkLines = append(hunkLines, "-"+before[beforeIdx])
			beforeIdx++
			deletions++
		}
		if isInsertion {
			if hunkStartBefore < 0 {
				hunkStartBefore = beforeIdx
				hunkStartAfter = afterIdx
			}
			hunkLines = append(hunkLines, "+"+after[afterIdx])
			afterIdx++
			additions++
		}
	}

	flushHunk()

	return strings.Join(diffLines, "\n"), additions, deletions
}

// computeLCS computes the longest common subsequence of two string slices.
func computeLCS(a, b []string) []string {
	m := len(a)
	n := len(b)

	// DP table for lengths
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	// Backtrack to find LCS
	result := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append(result, a[i-1])
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// Reverse
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

// Ensure defaultPreciseEditingService implements the interface.
var _ PreciseEditingService = (*defaultPreciseEditingService)(nil)
