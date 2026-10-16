package gitworktree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrMergeNotFastForward = errors.New("target cannot fast-forward to the approved task revision")
var ErrMergeTargetChanged = errors.New("merge target branch or revision changed since preview")
var ErrMergePreviouslyAttempted = errors.New("merge already applied or has an unresolved audit record")

type MergeProposal struct {
	WorktreeID     string `json:"worktreeId"`
	TaskID         string `json:"taskId"`
	ConversationID string `json:"conversationId"`
	Revision       string `json:"revision"`
	SourceHead     string `json:"sourceHead"`
	TargetBranch   string `json:"targetBranch"`
	TargetHead     string `json:"targetHead"`
	PlanID         string `json:"planId"`
	FilesChanged   int    `json:"filesChanged"`
	ApprovedBy     string `json:"approvedBy"`
}

type MergeAudit struct {
	Version     int           `json:"version"`
	Plan        MergeProposal `json:"plan"`
	Actor       string        `json:"actor"`
	StartedAt   time.Time     `json:"startedAt"`
	FinishedAt  *time.Time    `json:"finishedAt,omitempty"`
	RecoveredAt *time.Time    `json:"recoveredAt,omitempty"`
	Result      string        `json:"result"`
	AppliedHead string        `json:"appliedHead,omitempty"`
}

func mergePlanFingerprint(plan MergeProposal, canonical string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		plan.WorktreeID, plan.TaskID, plan.ConversationID, plan.Revision,
		plan.SourceHead, canonical, plan.TargetBranch, plan.TargetHead,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func validMergeAuditRecord(audit MergeAudit, id string) bool {
	plan := audit.Plan
	if audit.Version != 1 || plan.WorktreeID != id || !ValidBinding(plan.TaskID, plan.ConversationID) ||
		len(plan.PlanID) != 64 || strings.Trim(plan.PlanID, "0123456789abcdef") != "" ||
		len(plan.Revision) != 64 || strings.Trim(plan.Revision, "0123456789abcdef") != "" ||
		!validBaseHead(plan.SourceHead) || !validBaseHead(plan.TargetHead) ||
		plan.SourceHead == "" || plan.TargetHead == "" || !validateRef(plan.TargetBranch) ||
		plan.FilesChanged <= 0 || plan.ApprovedBy == "" || plan.ApprovedBy != audit.Actor ||
		audit.StartedAt.IsZero() || len(audit.Actor) > 128 {
		return false
	}
	switch audit.Result {
	case "pending":
		return audit.FinishedAt == nil && audit.RecoveredAt == nil && audit.AppliedHead == ""
	case "failed-needs-review":
		return audit.FinishedAt != nil && audit.RecoveredAt == nil
	case "applied":
		return audit.FinishedAt != nil && audit.AppliedHead == plan.SourceHead
	default:
		return false
	}
}

func (m *Manager) mergeAuditPath(key, id string) string {
	return filepath.Join(m.managedRoot(key), ".merges", id+".json")
}

func (m *Manager) readMergeAudit(key, id string) (*MergeAudit, error) {
	path := m.mergeAuditPath(key, id)
	dir := filepath.Dir(path)
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidArgument
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil || filepath.Clean(real) != filepath.Clean(dir) {
		return nil, ErrInvalidArgument
	}
	info, err = os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 8192 || metadataFilePermissionsUnsafe(info.Mode().Perm()) {
		return nil, ErrInvalidArgument
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var audit MergeAudit
	if json.Unmarshal(bytes, &audit) != nil || !validMergeAuditRecord(audit, id) {
		return nil, ErrInvalidArgument
	}
	return &audit, nil
}

func (m *Manager) writeMergeAudit(key, id string, record MergeAudit) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(data) > 8192 {
		return ErrInvalidArgument
	}
	path := m.mergeAuditPath(key, id)
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil || filepath.Clean(real) != filepath.Clean(dir) {
		return ErrInvalidArgument
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidArgument
	}
	if info, err = os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return ErrInvalidArgument
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	temp, err := os.CreateTemp(dir, ".merge-*")
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
	if err := os.Rename(temp.Name(), path); err != nil {
		return err
	}
	return syncManagedMetadataDirectory(dir)
}

func (m *Manager) previewMergeLocked(ctx context.Context, repoRoot, id, taskID, conversationID string) (*MergeProposal, string, error) {
	entry, key, err := m.checkpointEntry(ctx, repoRoot, id, taskID, conversationID)
	if err != nil {
		return nil, "", err
	}
	if entry.Locked || entry.Prunable {
		return nil, "", ErrDirty
	}
	canonical, _, err := m.repository(ctx, repoRoot)
	if err != nil {
		return nil, "", err
	}
	if filepath.Clean(canonical) == filepath.Clean(entry.Path) {
		return nil, "", ErrInvalidArgument
	}
	if err = m.refuseConfiguredFilters(ctx, canonical); err != nil {
		return nil, "", err
	}
	checkpoint, err := m.readCheckpoint(key, id)
	if err != nil {
		return nil, "", err
	}
	sourceRevision, err := m.checkpointRevision(ctx, entry)
	if err != nil {
		return nil, "", err
	}
	if checkpoint == nil || checkpoint.Revision != sourceRevision || checkpoint.ApprovedAt == nil || checkpoint.ApprovedBy == "" {
		return nil, "", ErrApprovalDenied
	}
	if len(checkpoint.Tests) == 0 || !VerifiedTestEvidenceComplete(checkpoint.Tests[len(checkpoint.Tests)-1], sourceRevision) {
		return nil, "", ErrTestRequired
	}
	old, err := m.readMergeAudit(key, id)
	if err != nil {
		return nil, "", err
	}
	if old != nil {
		return nil, "", ErrMergePreviouslyAttempted
	}
	targetBranchRaw, err := m.command(ctx, canonical, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return nil, "", ErrMergeTargetChanged
	}
	targetBranch := strings.TrimSpace(string(targetBranchRaw))
	if !validateRef(targetBranch) {
		return nil, "", ErrInvalidArgument
	}
	targetHeadRaw, err := m.command(ctx, canonical, "rev-parse", "HEAD")
	if err != nil {
		return nil, "", err
	}
	targetHead := strings.TrimSpace(string(targetHeadRaw))
	if !validBaseHead(targetHead) || targetHead == "" {
		return nil, "", ErrInvalidArgument
	}
	status, err := m.command(ctx, canonical, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, "", err
	}
	if len(status) != 0 {
		return nil, "", ErrDirty
	}
	if targetHead == checkpoint.Head {
		return nil, "", ErrMergeNotFastForward
	}
	if _, err = m.command(ctx, canonical, "merge-base", "--is-ancestor", targetHead, checkpoint.Head); err != nil {
		return nil, "", ErrMergeNotFastForward
	}
	changed, err := m.command(ctx, canonical, "diff", "--name-only", "--no-renames", targetHead, checkpoint.Head)
	if err != nil {
		return nil, "", err
	}
	number := 0
	for _, line := range strings.Split(strings.TrimSpace(string(changed)), "\n") {
		if line != "" {
			number++
		}
	}
	if number == 0 {
		return nil, "", ErrMergeNotFastForward
	}
	plan := &MergeProposal{WorktreeID: id, TaskID: taskID, ConversationID: conversationID, Revision: sourceRevision, SourceHead: checkpoint.Head, TargetBranch: targetBranch, TargetHead: targetHead, FilesChanged: number, ApprovedBy: checkpoint.ApprovedBy}
	plan.PlanID = mergePlanFingerprint(*plan, canonical)
	return plan, key, nil
}

func (m *Manager) PreviewApprovedMerge(ctx context.Context, repoRoot, id, taskID, conversationID string) (*MergeProposal, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	plan, _, err := m.previewMergeLocked(ctx, repoRoot, id, taskID, conversationID)
	return plan, err
}

func (m *Manager) ApplyApprovedMerge(ctx context.Context, repoRoot, id, taskID, conversationID, planID, actor string) (*MergeAudit, error) {
	if len(planID) != 64 || strings.Trim(planID, "0123456789abcdef") != "" || actor == "" || len(actor) > 128 || strings.ContainsAny(actor, "\x00\r\n") {
		return nil, ErrInvalidArgument
	}
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, lockErr := m.lockRepositoryMerge(ctx, repoRoot)
	if lockErr != nil {
		return nil, lockErr
	}
	defer unlock()
	plan, key, err := m.previewMergeLocked(ctx, repoRoot, id, taskID, conversationID)
	if err != nil {
		return nil, err
	}
	if plan.PlanID != planID {
		return nil, ErrMergeTargetChanged
	}
	if actor != plan.ApprovedBy {
		return nil, ErrApprovalDenied
	}
	canonical, _, err := m.repository(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	record := MergeAudit{Version: 1, Plan: *plan, Actor: actor, StartedAt: time.Now().UTC(), Result: "pending"}
	if err = m.writeMergeAudit(key, id, record); err != nil {
		return nil, err
	}
	_, mergeErr := m.command(ctx, canonical, "merge", "--ff-only", "--no-edit", plan.SourceHead)
	finished := time.Now().UTC()
	record.FinishedAt = &finished
	newHead, headErr := m.command(ctx, canonical, "rev-parse", "HEAD")
	if headErr == nil {
		record.AppliedHead = strings.TrimSpace(string(newHead))
	}
	if mergeErr == nil && headErr == nil && record.AppliedHead == plan.SourceHead {
		record.Result = "applied"
	} else {
		record.Result = "failed-needs-review"
	}
	if err = m.writeMergeAudit(key, id, record); err != nil {
		return nil, errors.Join(ErrMergePreviouslyAttempted, err)
	}
	if record.Result != "applied" {
		return &record, ErrMergeTargetChanged
	}
	return &record, nil
}

func (m *Manager) ReadMergeAudit(ctx context.Context, repoRoot, id, taskID, conversationID string) (*MergeAudit, error) {
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
	result, err := m.readMergeAudit(key, id)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrCheckpointRequired
	}
	canonical, _, err := m.repository(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	if result.Plan.PlanID != mergePlanFingerprint(result.Plan, canonical) {
		return nil, ErrInvalidArgument
	}
	if result.Result == "pending" {
		result, err = m.reconcileMergeAuditLocked(ctx, repoRoot, entry, key, result)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
