package workbenchtasks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

func TestCheckpointHistoryScopeIntegrityAndStatus(t *testing.T) {
	good := taskRecord("a", "c", "coord", "running")
	good.UpdatedAt = time.Now().Add(-time.Minute)
	good.Status = "processing"
	older := taskRecord("a", "c", "coord", "succeeded")
	older.ID = "parent-older"
	older.UpdatedAt = time.Now().Add(-time.Hour)
	older.Status = "completed"
	otherSpace := taskRecord("b", "c", "coord", "running")
	otherConv := taskRecord("a", "other", "coord", "running")
	otherTask := taskRecord("a", "c", "other", "running")
	wrongParent := taskRecord("a", "c", "coord", "running")
	wrongParent.ID = "wrong"
	wrongParent.RecoveryDescriptor.MultiAgent.ParentGoalSpaceID = "b"
	duplicate := *good
	duplicate.Status = "completed"
	result, err := ListCheckpoints(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{older, good, otherSpace, otherConv, otherTask, wrongParent, &duplicate}}, "a", "c", "coord")
	if err != nil || len(result.Records) != 2 || result.Records[0].CoordinationStatus != "running" || result.Records[0].ParentTerminal || !result.Records[1].ParentTerminal {
		t.Fatalf("history scoped: %+v %v", result, err)
	}
	if result.Source != "sampled-durable-checkpoints" || result.Records[1].CompletedCount != 1 {
		t.Fatalf("status history incorrect: %+v", result)
	}
}

func TestCheckpointHistoryRejectsTamperAndReportsConflict(t *testing.T) {
	signed := taskRecord("a", "c", "coord", "running")
	signed.RecoveryDescriptor.ComputeFingerprint()
	signed.RecoveryDescriptor.MultiAgent.Status = "failed"
	conflict := taskRecord("a", "c", "coord", "running")
	conflict.ID = "conflict"
	conflict.Status = "completed"
	result, err := ListCheckpoints(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{signed, conflict}}, "a", "c", "coord")
	if err != nil || len(result.Records) != 1 || result.Records[0].Consistent {
		t.Fatalf("checkpoint conflict not detected: %+v %v", result, err)
	}
}

func TestCheckpointHistoryBoundedAndCancelled(t *testing.T) {
	records := make([]*interaction.InteractionRecord, 0, 35)
	for i := 0; i < 35; i++ {
		r := taskRecord("a", "c", "coord", "running")
		r.ID = fmt.Sprint("item", i)
		r.UpdatedAt = time.Now().Add(-time.Duration(i) * time.Minute)
		records = append(records, r)
	}
	result, err := ListCheckpoints(context.Background(), taskRecordReader{records: records}, "a", "c", "coord")
	if err != nil || len(result.Records) != 20 || !result.Truncated {
		t.Fatalf("history limit incorrect: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ListCheckpoints(ctx, taskRecordReader{records: records}, "a", "c", "coord")
	if err != context.Canceled {
		t.Fatalf("cancel not propagated: %v", err)
	}
}

func TestHistoryHonorsCancelledRequestBeforeQuery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := ListCheckpoints(ctx, taskRecordReader{}, "space", "conv", "coord")
	if err != context.Canceled || len(result.Records) != 0 {
		t.Fatalf("cancellation returned results: %+v %v", result, err)
	}
}
