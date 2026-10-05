package business

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestContinuityOwnerLeaseAndNoReplay(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "create"}
	created, ack, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "检查进展", Goal: "查看设备任务"})
	if err != nil || created.OwnerID != "a" || ack.OwnerID != "a" {
		t.Fatalf("create=%+v ack=%+v err=%v", created, ack, err)
	}
	replayed, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "检查进展", Goal: "查看设备任务"})
	if err != nil || replayed.Thread.Revision != 1 {
		t.Fatalf("create replay=%+v err=%v", replayed, err)
	}
	if _, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "另一操作"}); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("conflicting request accepted: %v", err)
	}
	due := time.Now().Add(-time.Minute)
	request.RequestID = "wait"
	added, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: created.Thread.ID, ExpectedRevision: 1, Action: "add_wait", Wait: &continuity.Wait{WaitType: "time", DueAt: &due, AutoResume: true, ResumeHint: "检查状态"}})
	if err != nil || len(added.Waits) != 1 {
		t.Fatal(err)
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "read"
	finished, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: created.Thread.ID, Action: "read"})
	if err != nil || finished.Lease == nil || finished.Lease.State != "completed" || finished.Waits[0].Status != continuity.WaitStatusResolved || model.calls.Load() != 1 {
		t.Fatalf("finished=%+v calls=%d err=%v", finished, model.calls.Load(), err)
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != 1 {
		t.Fatal("completed wake was replayed")
	}
	mirrored, err := coordination.NewOwnershipStore(db, "core").List(t.Context(), "continuity", "role", false)
	if err != nil || len(mirrored) != 0 {
		t.Fatal("device ongoing body mirrored to Core", err)
	}
}

func TestPausingContinuityInterruptsGenerationAndPreventsReplay(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	started := make(chan struct{})
	model.generate = func(ctx context.Context, _ Inference) (Generation, error) {
		close(started)
		<-ctx.Done()
		return Generation{Text: "已接受的部分内容", Partial: true}, ctx.Err()
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "create"}
	document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "查看任务"})
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().Add(-time.Second)
	request.RequestID = "wait"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: 1, Action: "add_wait", Wait: &continuity.Wait{WaitType: "time", DueAt: &due, AutoResume: true}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- engine.TickContinuity(t.Context()) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("ongoing generation did not start")
	}
	request.RequestID = "read"
	live, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, Action: "read"})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "pause"
	paused, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: live.Thread.Revision, Action: "pause"})
	if err != nil || paused.Lease.State != "unknown" || paused.Thread.Status != continuity.ThreadStatusPaused {
		t.Fatalf("paused=%+v err=%v", paused, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pause did not interrupt ongoing conversation")
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != 1 {
		t.Fatal("paused execution was replayed")
	}
}

func TestContinuityCoreChangeDoesNotRecreateOldCoreData(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "create"}
	policy, err := service.Get(t.Context(), "core", "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", policy.ModeRevision, true, ""); err != nil {
		t.Fatal(err)
	}
	document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "旧云端事项"})
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().Add(-time.Second)
	request.RequestID = "wait"
	_, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: 1, Action: "add_wait", Wait: &continuity.Wait{WaitType: "time", DueAt: &due, AutoResume: true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.BindProvider(t.Context(), "new-core"); err != nil {
		t.Fatal(err)
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != 0 {
		t.Fatal("old Core ongoing work executed after handoff")
	}
	rows, err := coordination.NewOwnershipStore(db, "new-core").List(t.Context(), "continuity", "role", false)
	if err != nil || len(rows) != 0 {
		t.Fatal("old cloud data was migrated or recreated", err)
	}
	locations, err := service.PendingContinuity(t.Context())
	if err != nil || len(locations) != 0 {
		t.Fatal("old scheduler registration remained active", err)
	}
}

func TestExpiredContinuityLeasePausesWithoutModelReplay(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "create"}
	document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "定时动作"})
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().Add(-time.Minute)
	request.RequestID = "wait"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: 1, Action: "add_wait", Wait: &continuity.Wait{WaitType: "time", DueAt: &due, AutoResume: true}})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "claim"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: 2, Action: "claim", WaitID: document.Waits[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	document.Lease.ExpiresAt = time.Now().Add(-time.Minute)
	if _, err := db.Exec(`UPDATE kernel_device_owned_resources SET body=? WHERE owner_id='a' AND kind='continuity' AND resource_id=?`, []byte(body(document)), document.Thread.ID); err != nil {
		t.Fatal(err)
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "read"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, Action: "read"})
	if err != nil || document.Lease.State != "unknown" || document.Thread.Status != continuity.ThreadStatusPaused || model.calls.Load() != 0 {
		t.Fatalf("expired=%+v err=%v", document, err)
	}
	request.RequestID = "resume"
	if _, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "resume"}); !errors.Is(err, ErrUncertainExecution) {
		t.Fatalf("unknown replay allowed: %v", err)
	}
	request.RequestID = "wrong-confirm"
	if _, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "confirm_execution", LeaseID: "other-lease", Outcome: "completed"}); !errors.Is(err, ErrUncertainExecution) {
		t.Fatalf("wrong lease confirmation accepted: %v", err)
	}
	request.RequestID = "confirmed"
	confirmed, ack, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "confirm_execution", LeaseID: document.Lease.ID, Outcome: "completed", Result: "已核实原设备完成"})
	if err != nil || confirmed.Lease.State != "completed" || confirmed.Thread.Status != continuity.ThreadStatusPaused || ack.Versions["continuity/"+document.Thread.ID] != confirmed.Thread.Revision {
		t.Fatalf("confirmation=%+v ack=%+v err=%v", confirmed, ack, err)
	}
	request.RequestID = "confirmed-resume"
	if _, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: confirmed.Thread.Revision, Action: "resume"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != 0 {
		t.Fatal("manually confirmed execution was replayed")
	}
}
