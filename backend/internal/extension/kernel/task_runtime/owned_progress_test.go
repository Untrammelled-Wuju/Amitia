package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedProgressStore struct {
	completionFenceStore
	progress *TaskRunProgress
}

func (s *ownedProgressStore) PutProgress(_ context.Context, _ string, _ int64, payload []byte) error {
	var progress TaskRunProgress
	if err := json.Unmarshal(payload, &progress); err != nil {
		return err
	}
	s.progress = &progress
	return nil
}

func (s *ownedProgressStore) GetProgress(context.Context, string) (*TaskRunProgress, error) {
	return s.progress, nil
}

func TestOwnedTaskProgressKeepsDetailsAtOwnerAndRejectsStaleSourceRevision(t *testing.T) {
	service, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"private":"input"}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.ExecutionAttemptID, run.Status = 1, "attempt", RunStatusRunning
	data := &taskInputData{}
	ctx := coordination.WithScope(t.Context(), authority)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	store := &ownedProgressStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
	service.store = store
	port := AcknowledgedTaskProgressPort{Data: data}
	service.config.OwnedProgress = port
	current, total := 1.0, 2.0
	service.handleProgress(ctx, run.TaskRunID, 1, &current, &total, nil, "private-stage", "private-progress", run)
	if store.progress == nil || store.progress.Stage != "" || store.progress.Message != "" || store.progress.Current != nil || store.progress.Total != nil || strings.Contains(string(store.progress.Details), "private") {
		t.Fatalf("progress copied into ordinary task metadata: %+v", store.progress)
	}
	loaded, err := service.GetProgress(ctx, run.TaskRunID)
	if err != nil || loaded.Message != "private-progress" || loaded.Current == nil || *loaded.Current != 1 {
		t.Fatalf("owner progress not restored: %+v %v", loaded, err)
	}
	next, err := port.SaveProgress(ctx, run, TaskRunProgress{TaskRunID: run.TaskRunID, Sequence: 2, Message: "new-private-progress", UpdatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := port.Progress(ctx, run, store.progress); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("stale progress borrowed latest source version: %v", err)
	}
	if actual, err := port.Progress(ctx, run, &next); err != nil || actual.Message != "new-private-progress" {
		t.Fatalf("latest owner progress failed: %+v %v", actual, err)
	}
	if _, err := port.SaveProgress(ctx, run, TaskRunProgress{TaskRunID: run.TaskRunID, Sequence: 1}); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("older sequence rewrote owner progress: %v", err)
	}
	data.wrongAck = true
	if _, err := port.SaveProgress(ctx, run, TaskRunProgress{TaskRunID: run.TaskRunID, Sequence: 3}); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("wrong owner progress ACK accepted: %v", err)
	}
}
