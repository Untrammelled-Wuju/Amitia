package gitworktree

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	amitialog "log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrCheckpointRequired = errors.New("task review checkpoint is missing or stale")
var ErrTestRequired = errors.New("verified test run is required before approval")
var ErrApprovalDenied = errors.New("checkpoint is not eligible for approval")

type VerifiedTest struct {
	Profile         string    `json:"profile"`
	Revision        string    `json:"revision"`
	StartedAt       time.Time `json:"startedAt"`
	DurationMillis  int64     `json:"durationMillis"`
	ExitCode        int       `json:"exitCode"`
	Passed          bool      `json:"passed"`
	LogExcerpt      string    `json:"logExcerpt"`
	OutputDigest    string    `json:"outputDigest"`
	OutputTruncated bool      `json:"outputTruncated,omitempty"`
}

type ReviewCheckpoint struct {
	Version        int            `json:"version"`
	WorktreeID     string         `json:"worktreeId"`
	TaskID         string         `json:"taskId"`
	ConversationID string         `json:"conversationId"`
	BaseHead       string         `json:"baseHead"`
	Head           string         `json:"head"`
	CreatedAt      time.Time      `json:"createdAt"`
	Revision       string         `json:"revision"`
	Tests          []VerifiedTest `json:"tests,omitempty"`
	ApprovedAt     *time.Time     `json:"approvedAt,omitempty"`
	ApprovedBy     string         `json:"approvedBy,omitempty"`
}

func VerifiedTestEvidenceComplete(item VerifiedTest, revision string) bool {
	if !item.Passed || item.Revision == "" || item.Revision != revision || item.ExitCode != 0 || item.StartedAt.IsZero() || item.DurationMillis < 0 || item.OutputTruncated {
		return false
	}
	if _, valid := verifiedProfile(item.Profile); !valid {
		return false
	}
	if len(item.OutputDigest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(item.OutputDigest)
	return err == nil
}

func (m *Manager) checkpointPath(key, id string) string {
	return filepath.Join(m.managedRoot(key), ".checkpoints", id+".json")
}

func (m *Manager) checkpointEntry(ctx context.Context, repoRoot, id, taskID, conversationID string) (Entry, string, error) {
	if !managedID.MatchString(id) || !ValidBinding(taskID, conversationID) {
		return Entry{}, "", ErrInvalidArgument
	}
	entries, canonical, err := m.list(ctx, repoRoot)
	if err != nil {
		return Entry{}, "", err
	}
	_, key, err := m.repository(ctx, canonical)
	if err != nil {
		return Entry{}, "", err
	}
	for _, entry := range entries {
		if !entry.Managed || entry.ID != id {
			continue
		}
		if entry.TaskBinding == nil || entry.TaskBinding.TaskID != taskID || entry.TaskBinding.ConversationID != conversationID {
			return Entry{}, "", ErrBindingMismatch
		}
		if !validBaseHead(entry.Head) || entry.Head == "" || entry.TaskBinding.BaseHead == "" {
			return Entry{}, "", ErrCheckpointRequired
		}
		return entry, key, nil
	}
	return Entry{}, "", ErrNotManaged
}

func (m *Manager) readCheckpoint(key, id string) (*ReviewCheckpoint, error) {
	path := m.checkpointPath(key, id)
	directory := filepath.Dir(path)
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidArgument
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(directory) {
		return nil, ErrInvalidArgument
	}
	info, err = os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 || metadataFilePermissionsUnsafe(info.Mode().Perm()) {
		return nil, ErrInvalidArgument
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var record ReviewCheckpoint
	if err = json.Unmarshal(data, &record); err != nil || record.Version != 1 || record.WorktreeID != id || !ValidBinding(record.TaskID, record.ConversationID) || !validBaseHead(record.Head) || !validBaseHead(record.BaseHead) || record.Head == "" || record.Revision == "" || record.CreatedAt.IsZero() {
		return nil, ErrInvalidArgument
	}
	if len(record.Tests) > 16 || len(record.ApprovedBy) > 128 {
		return nil, ErrInvalidArgument
	}
	return &record, nil
}

func (m *Manager) writeCheckpoint(key, id string, record ReviewCheckpoint) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(data) > 65536 {
		return ErrInvalidArgument
	}
	dir := filepath.Dir(m.checkpointPath(key, id))
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(dir) {
		return ErrInvalidArgument
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidArgument
	}
	temp, err := os.CreateTemp(dir, ".checkpoint-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err = temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	path := m.checkpointPath(key, id)
	if info, err = os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return ErrInvalidArgument
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return err
	}
	return syncManagedMetadataDirectory(dir)
}

func (m *Manager) checkpointRevision(ctx context.Context, entry Entry) (string, error) {
	result, err := m.command(ctx, entry.Path, "status", "--porcelain=v1", "--untracked-files=all", "-z")
	if err != nil {
		return "", err
	}
	if len(result) != 0 {
		return "", ErrDirty
	}
	head, err := m.command(ctx, entry.Path, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(head)) != entry.Head {
		return "", ErrReviewOutdated
	}
	parent, err := m.command(ctx, entry.Path, "rev-parse", "--verify", entry.TaskBinding.BaseHead+"^{commit}")
	if err != nil || strings.TrimSpace(string(parent)) != entry.TaskBinding.BaseHead {
		return "", ErrCheckpointRequired
	}
	_, err = m.command(ctx, entry.Path, "merge-base", "--is-ancestor", entry.TaskBinding.BaseHead, entry.Head)
	if err != nil {
		return "", ErrApprovalDenied
	}
	sum := sha256.Sum256([]byte(entry.ID + "\x00" + entry.TaskBinding.TaskID + "\x00" + entry.TaskBinding.ConversationID + "\x00" + entry.TaskBinding.BaseHead + "\x00" + entry.Head))
	return hex.EncodeToString(sum[:]), nil
}

func (m *Manager) PrepareCheckpoint(ctx context.Context, repoRoot, id, taskID, conversationID string) (*ReviewCheckpoint, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	defer unlock()
	entry, key, err := m.checkpointEntry(ctx, repoRoot, id, taskID, conversationID)
	if err != nil {
		return nil, err
	}
	if err := m.refusePendingMergeAudit(key, id); err != nil {
		return nil, err
	}
	revision, err := m.checkpointRevision(ctx, entry)
	if err != nil {
		return nil, err
	}
	saved, err := m.readCheckpoint(key, id)
	if err != nil {
		return nil, err
	}
	if saved != nil && saved.Revision == revision && saved.TaskID == taskID && saved.ConversationID == conversationID {
		return saved, nil
	}
	record := ReviewCheckpoint{Version: 1, WorktreeID: id, TaskID: taskID, ConversationID: conversationID, BaseHead: entry.TaskBinding.BaseHead, Head: entry.Head, Revision: revision, CreatedAt: time.Now().UTC()}
	if err = m.writeCheckpoint(key, id, record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (m *Manager) ReadCheckpoint(ctx context.Context, repoRoot, id, taskID, conversationID string) (*ReviewCheckpoint, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	defer unlock()
	entry, key, err := m.checkpointEntry(ctx, repoRoot, id, taskID, conversationID)
	if err != nil {
		return nil, err
	}
	record, err := m.readCheckpoint(key, id)
	if err != nil {
		return nil, err
	}
	if record == nil || record.TaskID != taskID || record.ConversationID != conversationID {
		return nil, ErrCheckpointRequired
	}
	revision, err := m.checkpointRevision(ctx, entry)
	if err != nil {
		return nil, err
	}
	if record.Revision != revision {
		return nil, ErrCheckpointRequired
	}
	return record, nil
}

func verifiedProfile(profile string) ([]string, bool) {
	switch profile {
	case "go":
		return []string{"go", "test", "-json", "-count=1", "./..."}, true
	case "go-race":
		return []string{"go", "test", "-json", "-count=1", "-race", "./..."}, true
	}
	return nil, false
}

type goTestEvent struct {
	Action  string `json:"Action"`
	Test    string `json:"Test"`
	Package string `json:"Package"`
}

func parseGoTestEvent(output []byte) (goTestEvent, bool) {
	var event goTestEvent
	if json.Unmarshal(output, &event) != nil || event.Package == "" {
		return goTestEvent{}, false
	}
	return event, true
}

func hasActualGoTest(output []byte) bool {
	event, valid := parseGoTestEvent(output)
	return valid && event.Action == "pass" && event.Test != ""
}

const maxTestEvidenceBytes = 2 << 20
const maxTestExcerptBytes = 8192

type boundedTestEvidence struct {
	mu             sync.Mutex
	digest         hash.Hash
	bytes          int64
	truncated      bool
	tail           []byte
	line           []byte
	sawGoTest      bool
	sawPackagePass bool
	sawFailure     bool
	lineOverflow   bool
}

func (w *boundedTestEvidence) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.digest == nil {
		w.digest = sha256.New()
	}
	n := len(p)
	if _, err := w.digest.Write(p); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	w.bytes += int64(n)
	if w.bytes > maxTestEvidenceBytes {
		w.truncated = true
	}
	remaining := p
	for len(remaining) > 0 {
		index := bytes.IndexByte(remaining, '\n')
		if index < 0 {
			if !w.lineOverflow {
				if len(w.line)+len(remaining) > maxTestExcerptBytes {
					w.line = w.line[:0]
					w.lineOverflow = true
				} else {
					w.line = append(w.line, remaining...)
				}
			}
			break
		}
		if !w.lineOverflow && len(w.line)+index <= maxTestExcerptBytes {
			w.line = append(w.line, remaining[:index]...)
			w.observeGoEvent(w.line)
		}
		w.line = w.line[:0]
		w.lineOverflow = false
		remaining = remaining[index+1:]
	}
	if n >= maxTestExcerptBytes {
		w.tail = append(w.tail[:0], p[n-maxTestExcerptBytes:]...)
	} else {
		w.tail = append(w.tail, p...)
		if len(w.tail) > maxTestExcerptBytes {
			w.tail = append(w.tail[:0], w.tail[len(w.tail)-maxTestExcerptBytes:]...)
		}
	}
	return n, nil
}

func (w *boundedTestEvidence) observeGoEvent(line []byte) {
	event, valid := parseGoTestEvent(line)
	if !valid {
		return
	}
	switch event.Action {
	case "pass":
		if event.Test != "" {
			w.sawGoTest = true
		} else {
			w.sawPackagePass = true
		}
	case "fail":
		w.sawFailure = true
	}
}

func (m *Manager) RunCheckpointTest(ctx context.Context, repoRoot, id, taskID, conversationID, revision, profile string) (*ReviewCheckpoint, error) {
	args, valid := verifiedProfile(profile)
	if !valid || revision == "" {
		return nil, ErrInvalidArgument
	}
	finish, err := m.lockCheckpointExecution(ctx, repoRoot, id)
	if err != nil {
		return nil, err
	}
	defer finish()
	mutationMu.Lock()
	unlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		mutationMu.Unlock()
		return nil, err
	}
	entry, key, err := m.checkpointEntry(ctx, repoRoot, id, taskID, conversationID)
	if err != nil {
		unlock()
		mutationMu.Unlock()
		return nil, err
	}
	if err := m.refusePendingMergeAudit(key, id); err != nil {
		unlock()
		mutationMu.Unlock()
		return nil, err
	}
	record, err := m.readCheckpoint(key, id)
	if err != nil {
		unlock()
		mutationMu.Unlock()
		return nil, err
	}
	actual, err := m.checkpointRevision(ctx, entry)
	if err != nil {
		unlock()
		mutationMu.Unlock()
		return nil, err
	}
	if record == nil || record.Revision != actual || revision != actual {
		unlock()
		mutationMu.Unlock()
		return nil, ErrCheckpointRequired
	}
	if record.ApprovedAt != nil {
		unlock()
		mutationMu.Unlock()
		return nil, ErrApprovalDenied
	}
	started := time.Now().UTC()
	pending := VerifiedTest{Profile: profile, Revision: revision, StartedAt: started, ExitCode: -2, LogExcerpt: "test execution interrupted before verification"}
	record.Tests = append(record.Tests, pending)
	if len(record.Tests) > 6 {
		record.Tests = record.Tests[len(record.Tests)-6:]
	}
	if err = m.writeCheckpoint(key, id, *record); err != nil {
		unlock()
		mutationMu.Unlock()
		return nil, err
	}
	unlock()
	mutationMu.Unlock()
	executable, lookupErr := exec.LookPath(args[0])
	runctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	evidence := &boundedTestEvidence{digest: sha256.New()}
	runErr := lookupErr
	if runErr == nil {
		command := exec.CommandContext(runctx, executable, args[1:]...)
		command.Dir = entry.Path
		command.Env = safeCheckpointTestEnvironment()
		command.Stdout = evidence
		command.Stderr = evidence
		configureCheckpointTestProcess(command)
		runErr = command.Start()
		if runErr == nil {
			cancelProcessGroup := context.AfterFunc(runctx, func() { terminateCheckpointTestProcess(command) })
			runErr = command.Wait()
			if runctx.Err() != nil {
				terminateCheckpointTestProcess(command)
			}
			cancelProcessGroup()
		}
	}
	if lookupErr != nil {
		if _, err := evidence.Write([]byte("test runner executable is unavailable\n")); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	} else if runErr != nil && evidence.bytes == 0 {
		if _, err := evidence.Write([]byte("test runner failed without output\n")); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	duration := time.Since(started).Milliseconds()
	exit := 0
	if runErr != nil {
		exit = -1
		var failed *exec.ExitError
		if errors.As(runErr, &failed) {
			exit = failed.ExitCode()
		}
	}
	if runctx.Err() != nil {
		exit = -2
	}
	evidence.mu.Lock()
	digest := hex.EncodeToString(evidence.digest.Sum(nil))
	output := append([]byte(nil), evidence.tail...)
	outputTruncated := evidence.truncated
	evidence.observeGoEvent(evidence.line)
	passedEvidence := evidence.sawGoTest && evidence.sawPackagePass && !evidence.sawFailure
	evidence.mu.Unlock()
	passed := runErr == nil && runctx.Err() == nil && !outputTruncated && passedEvidence
	auditCtx, auditCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer auditCancel()
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, err = m.lockRepositoryMerge(auditCtx, repoRoot)
	if err != nil {
		return nil, err
	}
	defer unlock()
	current, key, err := m.checkpointEntry(auditCtx, repoRoot, id, taskID, conversationID)
	if err != nil {
		return nil, err
	}
	if err := m.refusePendingMergeAudit(key, id); err != nil {
		return nil, err
	}
	currentRecord, err := m.readCheckpoint(key, id)
	if err != nil {
		return nil, err
	}
	currentRev, err := m.checkpointRevision(auditCtx, current)
	if err != nil {
		return nil, err
	}
	if currentRecord == nil || currentRev != revision || currentRecord.Revision != revision {
		return nil, ErrCheckpointRequired
	}
	if currentRecord.ApprovedAt != nil {
		return nil, ErrApprovalDenied
	}
	if len(currentRecord.Tests) == 0 || !currentRecord.Tests[len(currentRecord.Tests)-1].StartedAt.Equal(started) || currentRecord.Tests[len(currentRecord.Tests)-1].Profile != profile {
		return nil, ErrReviewOutdated
	}
	item := VerifiedTest{Profile: profile, Revision: revision, StartedAt: started, DurationMillis: duration, ExitCode: exit, Passed: passed, LogExcerpt: string(bytes.ToValidUTF8(output, []byte("?"))), OutputDigest: digest, OutputTruncated: outputTruncated}
	currentRecord.Tests[len(currentRecord.Tests)-1] = item
	if err = m.writeCheckpoint(key, id, *currentRecord); err != nil {
		return nil, err
	}
	return currentRecord, nil
}

func (m *Manager) ApproveCheckpoint(ctx context.Context, repoRoot, id, taskID, conversationID, revision, approver string) (*ReviewCheckpoint, error) {
	if len(approver) == 0 || len(approver) > 128 || strings.ContainsAny(approver, "\x00\r\n") {
		return nil, ErrInvalidArgument
	}
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	defer unlock()
	entry, key, err := m.checkpointEntry(ctx, repoRoot, id, taskID, conversationID)
	if err != nil {
		return nil, err
	}
	if err := m.refusePendingMergeAudit(key, id); err != nil {
		return nil, err
	}
	record, err := m.readCheckpoint(key, id)
	if err != nil {
		return nil, err
	}
	actual, err := m.checkpointRevision(ctx, entry)
	if err != nil {
		return nil, err
	}
	if record == nil || revision != actual || record.Revision != revision {
		return nil, ErrCheckpointRequired
	}
	if record.ApprovedAt != nil {
		return nil, ErrApprovalDenied
	}
	passed := len(record.Tests) > 0 && VerifiedTestEvidenceComplete(record.Tests[len(record.Tests)-1], revision)
	if !passed {
		return nil, ErrTestRequired
	}
	now := time.Now().UTC()
	record.ApprovedAt = &now
	record.ApprovedBy = approver
	if err = m.writeCheckpoint(key, id, *record); err != nil {
		return nil, err
	}
	return record, nil
}

func (m *Manager) InvalidateCheckpoint(ctx context.Context, repoRoot, id string) error {
	if !managedID.MatchString(id) {
		return ErrInvalidArgument
	}
	finish, err := m.lockCheckpointExecution(ctx, repoRoot, id)
	if err != nil {
		return err
	}
	defer finish()
	unlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		return err
	}
	defer unlock()
	_, key, err := m.repository(ctx, repoRoot)
	if err != nil {
		return err
	}
	if err := m.refusePendingMergeAudit(key, id); err != nil {
		return err
	}
	return m.invalidateCheckpointByKey(key, id)
}

func (m *Manager) invalidateCheckpointByKey(key, id string) error {
	path := m.checkpointPath(key, id)
	directory := filepath.Dir(path)
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidArgument
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(directory) {
		return ErrInvalidArgument
	}
	info, err = os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return ErrInvalidArgument
	}
	if err = os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("invalidate checkpoint %s: %w", strconv.Quote(id), err)
	}
	return syncManagedMetadataDirectory(directory)
}
