// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openQualityOwnershipTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:quality-ownership?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&QualityEvaluation{}))
	return db
}

func TestQualityLeaseOwnershipFence(t *testing.T) {
	db := openQualityOwnershipTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	require.NoError(t, repo.CreateEvaluation(ctx, &QualityEvaluation{
		ID:              "eval-owner-fence",
		ExecutionStatus: EvalPending,
	}))

	acquired, err := repo.AcquireLease(ctx, "eval-owner-fence", "exec-1", "worker-1", "5m")
	require.NoError(t, err)
	require.True(t, acquired)

	row, err := repo.GetEvaluation(ctx, "eval-owner-fence")
	require.NoError(t, err)
	require.Equal(t, "exec-1", row.ExecutionID)
	require.Equal(t, "worker-1", row.WorkerID)

	stale := *row
	stale.ExecutionStatus = EvalSucceeded
	updated, err := repo.UpdateEvaluationOwned(ctx, &stale, "exec-stale")
	require.NoError(t, err)
	require.False(t, updated)

	released, err := repo.ReleaseLease(ctx, "eval-owner-fence", "exec-stale")
	require.NoError(t, err)
	require.False(t, released)

	row, err = repo.GetEvaluation(ctx, "eval-owner-fence")
	require.NoError(t, err)
	require.Equal(t, "exec-1", row.ExecutionID)
	require.Equal(t, EvalRunning, row.ExecutionStatus)

	// Expire exec-1 and let a second worker acquire the row. The old worker must
	// no longer be able to update or release the new lease.
	require.NoError(t, db.Model(&QualityEvaluation{}).
		Where("id = ?", row.ID).
		Update("lease_expires_at", time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)).Error)
	acquired, err = repo.AcquireLease(ctx, row.ID, "exec-2", "worker-2", "5m")
	require.NoError(t, err)
	require.True(t, acquired)

	updated, err = repo.UpdateEvaluationOwned(ctx, &stale, "exec-1")
	require.NoError(t, err)
	require.False(t, updated)
	released, err = repo.ReleaseLease(ctx, row.ID, "exec-1")
	require.NoError(t, err)
	require.False(t, released)

	row, err = repo.GetEvaluation(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, "exec-2", row.ExecutionID)
	require.Equal(t, "worker-2", row.WorkerID)
}

func TestRecoverExpiredEvaluationIsCompareAndSwap(t *testing.T) {
	db := openQualityOwnershipTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	require.NoError(t, repo.CreateEvaluation(ctx, &QualityEvaluation{
		ID:              "eval-recovery-cas",
		ExecutionStatus: EvalPending,
	}))

	acquired, err := repo.AcquireLease(ctx, "eval-recovery-cas", "exec-live", "worker-live", "5m")
	require.NoError(t, err)
	require.True(t, acquired)

	recovered, err := repo.RecoverExpiredEvaluation(ctx, "eval-recovery-cas", "exec-live", time.Now().UTC())
	require.NoError(t, err)
	require.False(t, recovered, "a live heartbeat lease must not be recovered")

	require.NoError(t, db.Model(&QualityEvaluation{}).
		Where("id = ?", "eval-recovery-cas").
		Update("lease_expires_at", time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)).Error)

	recovered, err = repo.RecoverExpiredEvaluation(ctx, "eval-recovery-cas", "exec-stale", time.Now().UTC())
	require.NoError(t, err)
	require.False(t, recovered, "a stale recovery snapshot must not clear another execution")

	recovered, err = repo.RecoverExpiredEvaluation(ctx, "eval-recovery-cas", "exec-live", time.Now().UTC())
	require.NoError(t, err)
	require.True(t, recovered)

	row, err := repo.GetEvaluation(ctx, "eval-recovery-cas")
	require.NoError(t, err)
	require.Equal(t, EvalPending, row.ExecutionStatus)
	require.Empty(t, row.ExecutionID)
	require.Empty(t, row.LeaseExpiresAt)
}

func TestGetActiveEvaluationIgnoresNewerInactiveHistoricalResult(t *testing.T) {
	db := openQualityOwnershipTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	active := &QualityEvaluation{
		ID:               "eval-active",
		ProcessingTaskID: "task-active",
		ActionKey:        "idle",
		ExecutionStatus:  EvalSucceeded,
		IsActive:         true,
		CreatedAt:        "2026-08-31T01:00:00Z",
	}
	inactiveNewer := &QualityEvaluation{
		ID:               "eval-inactive-newer",
		ProcessingTaskID: "task-active",
		ActionKey:        "idle",
		ExecutionStatus:  EvalSucceeded,
		IsActive:         false,
		CreatedAt:        "2026-08-31T02:00:00Z",
	}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create active evaluation: %v", err)
	}
	if err := db.Create(inactiveNewer).Error; err != nil {
		t.Fatalf("create inactive evaluation: %v", err)
	}

	got, err := repo.GetActiveEvaluation(ctx, "task-active", "idle")
	if err != nil {
		t.Fatalf("get active evaluation: %v", err)
	}
	if got.ID != active.ID {
		t.Fatalf("expected active evaluation %s, got %s", active.ID, got.ID)
	}
}
