package workbenchtasks

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

func TestRound62TaskRunAssociationOnlyFromExplicitDurableRef(t *testing.T) {
	record := taskRecord("space", "conversation", "coordination-only", "running")
	list, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conversation")
	if err != nil || len(list.Tasks) != 1 || list.Tasks[0].TaskRunID != "" {
		t.Fatalf("invented TaskRun association: %+v %v", list, err)
	}
	record.RecoveryDescriptor.Task = &interaction.RecoveryTaskRef{TaskRunID: "run_123"}
	list, err = List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conversation")
	if err != nil || len(list.Tasks) != 1 || list.Tasks[0].TaskRunID != "run_123" {
		t.Fatalf("explicit association missing: %+v %v", list, err)
	}
	record.RecoveryDescriptor.Task.TaskRunID = "../other-owner"
	list, err = List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conversation")
	if err != nil || len(list.Tasks) != 1 || list.Tasks[0].TaskRunID != "" {
		t.Fatalf("unsafe association exposed: %+v %v", list, err)
	}
}

func TestRound62TamperedTaskAssociationCannotBypassFingerprint(t *testing.T) {
	record := taskRecord("space", "conversation", "coord", "running")
	record.RecoveryDescriptor.Task = &interaction.RecoveryTaskRef{TaskRunID: "run_good"}
	record.RecoveryDescriptor.ComputeFingerprint()
	record.RecoveryDescriptor.Task.TaskRunID = "run_other"
	list, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conversation")
	if err != nil || len(list.Tasks) != 0 {
		t.Fatalf("tampered recovery fingerprint accepted: %+v %v", list, err)
	}
}
