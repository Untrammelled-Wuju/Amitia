package execution

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestM07CapabilityApprovalSingleClaimAcrossRunners(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:m07-approval-claim?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	repo := NewSQLiteResumeRepository(db)
	if err := repo.InitTable(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	row := ResumeContext{ResumeID: "approval-claim-54", RootExecutionID: "root", ParentExecutionID: "parent",
		Type: ResumeTypeCapabilityAcquisition, State: ResumeStatePending, RequiredCapabilityID: "skill.test",
		AcquisitionTransactionID: "approval-token-54", CreatedAt: now, UpdatedAt: now}
	if err := repo.Save(ctx, row); err != nil {
		t.Fatal(err)
	}
	var winners atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			claimed, err := repo.ClaimPending(ctx, row.ResumeID)
			if err != nil {
				t.Errorf("claim failed: %v", err)
				return
			}
			if claimed {
				winners.Add(1)
			}
		}()
	}
	group.Wait()
	if winners.Load() != 1 {
		t.Fatalf("approval claimed %d times", winners.Load())
	}
	saved, err := repo.GetByAcquisitionTransactionID(ctx, row.AcquisitionTransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || saved.State != ResumeStateInProgress {
		t.Fatalf("expected in_progress, got %#v", saved)
	}
}
