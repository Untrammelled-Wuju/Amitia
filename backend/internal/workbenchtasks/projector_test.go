package workbenchtasks

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

type taskRecordReader struct {
	records []*interaction.InteractionRecord
	err     error
}

func (f taskRecordReader) ListByScope(context.Context, interaction.InteractionScope) ([]*interaction.InteractionRecord, error) {
	return f.records, f.err
}

func taskRecord(space, conv, id, state string) *interaction.InteractionRecord {
	r := interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: space, ConversationID: conv})
	r.UpdatedAt = time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	r.RecoveryDescriptor = &interaction.RecoveryDescriptor{MultiAgent: &interaction.MultiAgentRecoveryRef{
		CoordinationID: id, Status: state, AssignmentRefs: []interaction.AssignmentRecoveryRef{
			{AssignmentID: "worker-1", WorkerID: "worker", Role: "coding_worker", Status: "succeeded", Objective: "implement terminal"},
			{AssignmentID: "worker-2", WorkerID: "worker", Role: "coding_worker", Status: "running", Objective: "run test suite"},
		},
	}}
	return r
}

func TestListProjectsOnlyConversationScopedCheckpoint(t *testing.T) {
	ctx := context.Background()
	good := taskRecord("space-a", "conv-a", "task-a", "running")
	otherSpace := taskRecord("space-b", "conv-a", "task-b", "running")
	otherConversation := taskRecord("space-a", "conv-b", "task-c", "running")
	child := taskRecord("space-a", "conv-a", "task-d", "running")
	child.Scope.Source = "multi_agent"
	res, err := List(ctx, taskRecordReader{records: []*interaction.InteractionRecord{good, otherSpace, otherConversation, child}}, "space-a", "conv-a")
	if err != nil || len(res.Tasks) != 1 || res.Tasks[0].CoordinationID != "task-a" {
		t.Fatalf("scope failed: %+v %v", res, err)
	}
	got := res.Tasks[0]
	if got.CompletedCount != 1 || got.AssignmentCount != 2 || got.FailedCount != 0 || got.RecoveryState != "checkpoint-only" || !got.Assignments[0].ReviewRequired {
		t.Fatalf("projection failed: %+v", got)
	}
}

func TestListRejectsTamperedAndDuplicateCheckpoint(t *testing.T) {
	good := taskRecord("space", "conv", "first", "running")
	good.RecoveryDescriptor.ComputeFingerprint()
	tampered := taskRecord("space", "conv", "second", "running")
	tampered.RecoveryDescriptor.ComputeFingerprint()
	tampered.RecoveryDescriptor.MultiAgent.Status = "failed"
	invalid := taskRecord("space", "conv", "third", "unknown")
	duplicate := taskRecord("space", "conv", "first", "failed")
	res, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{good, tampered, invalid, duplicate}}, "space", "conv")
	if err != nil || len(res.Tasks) != 1 {
		t.Fatalf("unsafe record leaked: %+v %v", res, err)
	}
}

func TestListCapsRecordsAndSanitizesObjective(t *testing.T) {
	var records []*interaction.InteractionRecord
	for i := 0; i < 45; i++ {
		r := taskRecord("space", "conv", fmt.Sprintf("task-%d", i), "running")
		r.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Objective = "任务" + strings.Repeat("a", 300)
		records = append(records, r)
	}
	res, err := List(context.Background(), taskRecordReader{records: records}, "space", "conv")
	if err != nil || len(res.Tasks) != 30 || !res.Truncated || len([]rune(res.Tasks[0].Assignments[0].Objective)) > 220 {
		t.Fatalf("limit failed: %+v %v", res, err)
	}
}

type fakeSnapshot struct {
	snapshot interaction.CoordinationSnapshot
}

func (f fakeSnapshot) Snapshot(context.Context, string) (interaction.CoordinationSnapshot, error) {
	return f.snapshot, nil
}

func TestSnapshotMustMatchDurableParentAndAssignments(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "running")
	live := interaction.CoordinationSnapshot{CoordinationID: "coord", ParentInteractionID: record.ID, Status: interaction.CoordinationSucceeded, Assignments: []interaction.CoordinationAssignmentSnapshot{
		{AssignmentID: "worker-1", Status: interaction.AssignmentSucceeded}, {AssignmentID: "worker-2", Status: interaction.AssignmentSucceeded},
	}}
	got, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv", fakeSnapshot{snapshot: live})
	if err != nil || got.Tasks[0].RecoveryState != "runtime-observed" || got.Tasks[0].CompletedCount != 2 || got.Tasks[0].Status != "succeeded" {
		t.Fatalf("live projection failed: %+v %v", got, err)
	}
	live.ParentInteractionID = "foreign-parent"
	got, _ = List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv", fakeSnapshot{snapshot: live})
	if got.Tasks[0].RecoveryState != "checkpoint-only" {
		t.Fatalf("foreign snapshot was accepted: %+v", got)
	}
	live.ParentInteractionID = record.ID
	live.Assignments[1].AssignmentID = "other-worker"
	got, _ = List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv", fakeSnapshot{snapshot: live})
	if got.Tasks[0].RecoveryState != "needs-reconciliation" || got.Tasks[0].RecoveryReason != "runtime-checkpoint-conflict" {
		t.Fatalf("worker mismatch was accepted: %+v", got)
	}
}

type boundedTaskReader struct {
	taskRecordReader
	count int
}

func (b *boundedTaskReader) ListRecentWorkbenchTaskRecords(_ context.Context, scope interaction.InteractionScope, limit int) ([]*interaction.InteractionRecord, error) {
	b.count++
	if limit != 121 || scope.SpaceID != "space" || scope.ConversationID != "conv" {
		panic("unbounded request")
	}
	return b.records, b.err
}
func TestListPrefersDatabaseBoundedRead(t *testing.T) {
	b := &boundedTaskReader{taskRecordReader: taskRecordReader{records: []*interaction.InteractionRecord{taskRecord("space", "conv", "coord", "running")}}}
	got, err := List(context.Background(), b, "space", "conv")
	if err != nil || b.count != 1 || len(got.Tasks) != 1 {
		t.Fatalf("bounded read failed: %+v %v", got, err)
	}
}

type countingSnapshotReader struct{ calls int }

func (r *countingSnapshotReader) Snapshot(context.Context, string) (interaction.CoordinationSnapshot, error) {
	r.calls++
	return interaction.CoordinationSnapshot{}, fmt.Errorf("offline")
}

func TestLiveSnapshotBudgetLimitsBlockingLookups(t *testing.T) {
	records := make([]*interaction.InteractionRecord, 15)
	for i := range records {
		records[i] = taskRecord("space", "conv", fmt.Sprintf("task-%02d", i), "running")
		records[i].ID = fmt.Sprintf("parent-%d", i)
	}
	reader := &countingSnapshotReader{}
	result, err := List(context.Background(), taskRecordReader{records: records}, "space", "conv", reader)
	if err != nil || len(result.Tasks) != 15 || reader.calls != 8 || !result.LiveObservationTruncated {
		t.Fatalf("live snapshot budget failed: %+v %v, calls=%d", result, err, reader.calls)
	}
}

func TestListHonorsCancelledRequestBeforeQuery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := List(ctx, taskRecordReader{records: []*interaction.InteractionRecord{taskRecord("space", "conv", "coord", "running")}}, "space", "conv")
	if err != context.Canceled || len(result.Tasks) != 0 {
		t.Fatalf("cancellation returned stale results: %+v %v", result, err)
	}
}
