package task_runtime

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound100OrphanCheckpointCannotBeAcknowledgedOrProjected(t *testing.T) {
	for _, mode := range []string{"missing_cursor", "older_cursor", "foreign_cursor"} {
		t.Run(mode, func(t *testing.T) {
			svc, ctx, data, store, run, def, checkpoint := round16PendingCheckpointFixture(t)
			cursorKey := "checkpoint/" + taskOwnerCommitCursorID(run)
			switch mode {
			case "missing_cursor":
				delete(data.resources, cursorKey)
			case "older_cursor":
				var cursor ownedTaskCommitCursor
				if err := json.Unmarshal(data.resources[cursorKey].Body, &cursor); err != nil {
					t.Fatal(err)
				}
				cursor.CheckpointVersion = 0
				cursor.CheckpointID = ""
				data.resources[cursorKey].Body, _ = json.Marshal(cursor)
			case "foreign_cursor":
				data.resources[cursorKey].OwnerID = "foreign-owner"
			}
			port := svc.config.OwnedCheckpoints.(AcknowledgedTaskCheckpointPort)
			if err := port.SaveCheckpoint(ctx, run, checkpoint); err == nil {
				t.Fatal("unconfirmed Owner checkpoint was acknowledged")
			}
			if _, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil); err == nil {
				t.Fatal("orphan Owner checkpoint was projected")
			}
			if store.casCalls != 0 || store.checkpoint != nil {
				t.Fatal("orphan checkpoint modified local run")
			}
		})
	}
}

func TestRound100OrphanOutcomeCannotConfirmOrRecover(t *testing.T) {
	for _, mode := range []string{"missing_cursor", "wrong_terminal", "foreign_cursor"} {
		t.Run(mode, func(t *testing.T) {
			service, data, authority, run, _ := ownedCommittedOutcomeFixture(t)
			ctx := coordination.WithScope(t.Context(), authority)
			port := service.config.OwnedOutcomes.(AcknowledgedTaskOutcomePort)
			if err := port.SaveOutcome(ctx, run, "failed", nil, "code", "details"); err != nil {
				t.Fatal(err)
			}
			cursorKey := "checkpoint/" + taskOwnerCommitCursorID(run)
			switch mode {
			case "missing_cursor":
				delete(data.resources, cursorKey)
			case "wrong_terminal":
				resource := data.resources[cursorKey]
				var cursor ownedTaskCommitCursor
				if err := json.Unmarshal(resource.Body, &cursor); err != nil {
					t.Fatal(err)
				}
				cursor.Terminal = false
				cursor.OutcomeID = ""
				resource.Body, _ = json.Marshal(cursor)
			case "foreign_cursor":
				data.resources[cursorKey].OwnerID = "foreign-owner"
			}
			if _, err := port.InspectOutcome(ctx, run); err == nil {
				t.Fatal("orphan Owner terminal was treated as committed")
			}
			if err := port.SaveOutcome(ctx, run, "failed", nil, "code", "details"); err == nil {
				t.Fatal("orphan Owner terminal replay obtained success")
			}
		})
	}
}

func TestRound100OrphanSuccessResultCannotBeRead(t *testing.T) {
	service, data, authority, run, _ := ownedCommittedOutcomeFixture(t)
	ctx := coordination.WithScope(t.Context(), authority)
	port := service.config.OwnedOutcomes.(AcknowledgedTaskOutcomePort)
	body := []byte(`{"ok":true}`)
	result := &TaskRunResult{TaskRunID: run.TaskRunID, ResultType: ResultInlineJSON, ResultJSON: body, ResultHash: hashBytes(body)}
	if err := port.SaveOutcome(ctx, run, "succeeded", result, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := port.Result(ctx, run, result); err != nil {
		t.Fatalf("committed Owner result should be readable: %v", err)
	}
	delete(data.resources, "checkpoint/"+taskOwnerCommitCursorID(run))
	if _, err := port.Result(ctx, run, result); err == nil {
		t.Fatal("uncommitted Owner result was readable")
	}
}
