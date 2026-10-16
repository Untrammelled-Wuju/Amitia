package gitworktree

import (
	"context"
	"path/filepath"
	"strings"
	"time"
)

func (m *Manager) reconcileMergeAuditLocked(ctx context.Context, repoRoot string, entry Entry, key string, audit *MergeAudit) (*MergeAudit, error) {
	plan := audit.Plan
	if plan.WorktreeID != entry.ID || entry.TaskBinding == nil || plan.TaskID != entry.TaskBinding.TaskID || plan.ConversationID != entry.TaskBinding.ConversationID || plan.SourceHead != entry.Head || audit.Actor != plan.ApprovedBy || audit.FinishedAt != nil || audit.AppliedHead != "" {
		return audit, nil
	}
	checkpoint, err := m.readCheckpoint(key, entry.ID)
	if err != nil {
		return nil, err
	}
	if checkpoint == nil || checkpoint.Revision != plan.Revision || checkpoint.Head != plan.SourceHead || checkpoint.TaskID != plan.TaskID || checkpoint.ConversationID != plan.ConversationID || checkpoint.ApprovedAt == nil || checkpoint.ApprovedBy != plan.ApprovedBy || len(checkpoint.Tests) == 0 {
		return audit, nil
	}
	last := checkpoint.Tests[len(checkpoint.Tests)-1]
	if !VerifiedTestEvidenceComplete(last, plan.Revision) {
		return audit, nil
	}
	canonical, _, err := m.repository(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	if plan.PlanID != mergePlanFingerprint(plan, canonical) || !validBaseHead(plan.TargetHead) || !validateRef(plan.TargetBranch) || filepath.Clean(canonical) == filepath.Clean(entry.Path) {
		return audit, nil
	}
	branch, err := m.command(ctx, canonical, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || strings.TrimSpace(string(branch)) != plan.TargetBranch {
		return audit, nil
	}
	head, err := m.command(ctx, canonical, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(head)) != plan.SourceHead {
		return audit, nil
	}
	if _, err = m.command(ctx, canonical, "merge-base", "--is-ancestor", plan.TargetHead, plan.SourceHead); err != nil {
		return audit, nil
	}
	status, err := m.command(ctx, canonical, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || len(status) != 0 {
		return audit, nil
	}
	now := time.Now().UTC()
	updated := *audit
	updated.Result = "applied"
	updated.AppliedHead = plan.SourceHead
	updated.FinishedAt = &now
	updated.RecoveredAt = &now
	if err := m.writeMergeAudit(key, entry.ID, updated); err != nil {
		return nil, err
	}
	return &updated, nil
}
