// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package writeback

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"gorm.io/gorm"
)

func openCommitterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&quality.QualityEvaluation{},
		&quality.QualityFindingRecord{},
		&quality.QualityDimensionScoreRecord{},
		&quality.ActiveQualityEvaluationBindingRecord{},
		&quality.QualityCommitJournalRecord{},
		&quality.QualityOutboxEventRecord{},
	); err != nil {
		t.Fatalf("migrate quality tables: %v", err)
	}
	if err := db.Exec(`CREATE TABLE desktop_pet_action_revisions (
		id TEXT PRIMARY KEY,
		content_hash TEXT NOT NULL,
		quality_evaluation_id TEXT DEFAULT '',
		quality_profile_id TEXT DEFAULT '',
		quality_ruleset_version TEXT DEFAULT '',
		quality_verdict TEXT DEFAULT '',
		quality_overall_score REAL,
		quality_source_content_hash TEXT DEFAULT '',
		quality_evaluated_at TEXT DEFAULT '',
		status TEXT DEFAULT '',
		updated_at TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatalf("create action revision table: %v", err)
	}
	return db
}

func TestCommitEvaluationStaleRevisionNeverBecomesActive(t *testing.T) {
	db := openCommitterTestDB(t)
	ctx := context.Background()
	repo := quality.NewRepository(db)

	if err := db.Exec(`INSERT INTO desktop_pet_action_revisions(id, content_hash) VALUES (?, ?)`, "revision-1", "new-content").Error; err != nil {
		t.Fatalf("seed revision: %v", err)
	}
	ev := &quality.QualityEvaluation{
		ID:                "evaluation-stale",
		UserID:            "user-1",
		CharacterID:       "character-1",
		ProcessingTaskID:  "task-1",
		ActionRevisionID:  "revision-1",
		ActionContentHash: "old-content",
		ActionKey:         "idle",
		ProfileID:         "base",
		RuleSetVersion:    "rules-v1",
		ExecutionStatus:   quality.EvalRunning,
		ExecutionID:       "exec-1",
		WorkerID:          "worker-1",
		LeaseOwner:        "worker-1",
		LeaseExpiresAt:    "2099-01-01T00:00:00Z",
	}
	if err := repo.CreateEvaluation(ctx, ev); err != nil {
		t.Fatalf("create evaluation: %v", err)
	}

	committer := NewCommitter(db, repo, NewQualityWritebackService(db), NewActiveBindingService(repo))
	result, err := committer.CommitEvaluation(ctx, quality.CommitEvaluationRequest{
		Evaluation:        ev,
		ExecutionID:       "exec-1",
		Verdict:           quality.VerdictAccepted,
		OverallScore:      98,
		OverallConfidence: 1,
		ProcessingTaskID:  "task-1",
		ActionKey:         "idle",
	})
	if err != nil {
		t.Fatalf("commit stale evaluation: %v", err)
	}
	if result.WritebackApplied {
		t.Fatal("stale revision must not report writeback applied")
	}
	if result.ActiveBindingSet {
		t.Fatal("stale revision must not create an active binding")
	}

	stored, err := repo.GetEvaluation(ctx, ev.ID)
	if err != nil {
		t.Fatalf("reload evaluation: %v", err)
	}
	if stored.IsActive {
		t.Fatal("stale historical evaluation must remain inactive")
	}
	if _, err := repo.GetActiveEvaluation(ctx, "task-1", "idle"); err != gorm.ErrRecordNotFound {
		t.Fatalf("expected no active evaluation, got err=%v", err)
	}
	binding, err := repo.GetActiveQualityBinding(ctx, "revision-1", "base")
	if err != nil {
		t.Fatalf("read active binding: %v", err)
	}
	if binding != nil {
		t.Fatalf("expected no active binding, got %+v", binding)
	}

	pending, err := repo.ListPendingOutboxEvents(ctx, 10)
	if err != nil {
		t.Fatalf("list outbox: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected one durable completion event, got %d", len(pending))
	}
}

func TestCommitEvaluationRequiresCurrentExecutionOwner(t *testing.T) {
	db := openCommitterTestDB(t)
	ctx := context.Background()
	repo := quality.NewRepository(db)
	if err := db.Exec(`INSERT INTO desktop_pet_action_revisions(id, content_hash) VALUES (?, ?)`, "revision-owner", "hash-1").Error; err != nil {
		t.Fatalf("seed revision: %v", err)
	}
	ev := &quality.QualityEvaluation{
		ID:                "evaluation-owner",
		ProcessingTaskID:  "task-owner",
		ActionRevisionID:  "revision-owner",
		ActionContentHash: "hash-1",
		ActionKey:         "idle",
		ProfileID:         "base",
		ExecutionStatus:   quality.EvalRunning,
		ExecutionID:       "exec-new",
	}
	if err := repo.CreateEvaluation(ctx, ev); err != nil {
		t.Fatalf("create evaluation: %v", err)
	}

	committer := NewCommitter(db, repo, NewQualityWritebackService(db), NewActiveBindingService(repo))
	_, err := committer.CommitEvaluation(ctx, quality.CommitEvaluationRequest{
		Evaluation:       ev,
		ExecutionID:      "exec-old",
		Verdict:          quality.VerdictAccepted,
		OverallScore:     100,
		ProcessingTaskID: "task-owner",
		ActionKey:        "idle",
	})
	if err == nil {
		t.Fatal("stale execution owner must not commit")
	}

	pending, listErr := repo.ListPendingOutboxEvents(ctx, 10)
	if listErr != nil {
		t.Fatalf("list outbox: %v", listErr)
	}
	if len(pending) != 0 {
		t.Fatalf("stale owner must not persist outbox events, got %d", len(pending))
	}
}
