package business

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type continuityMemoryPort struct {
	testDataPort
	engine      *Engine
	heartbeat   bool
	deferMemory bool
	triggered   bool
	err         error
}

func (p *continuityMemoryPort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if p.deferMemory && strings.HasSuffix(commit.Scope.RequestID, "|memory") && !p.triggered {
		p.triggered = true
		return coordination.Acknowledgement{}, errors.New("device temporarily unavailable")
	}
	ack, err := p.testDataPort.Commit(ctx, commit)
	if err != nil || !p.heartbeat || p.triggered {
		return ack, err
	}
	for _, mutation := range commit.Mutations {
		if mutation.Kind != "checkpoint" || !strings.HasPrefix(mutation.ID, "memory/") || mutation.Deleted {
			continue
		}
		p.triggered = true
		proof := coordination.CommitLease(ctx)
		if proof == nil {
			p.err = errors.New("memory plan missing lease proof")
			return ack, p.err
		}
		request := continuityRequest(commit.Scope)
		request.RequestID = "test/read-renewal"
		live, _, readErr := p.engine.Continuity(context.Background(), request, ContinuityMutation{ID: proof.ContinuityID, Action: "read"})
		if readErr != nil {
			p.err = readErr
			return ack, readErr
		}
		request.RequestID = "test/renewal"
		_, _, p.err = p.engine.Continuity(context.Background(), request, ContinuityMutation{ID: proof.ContinuityID, ExpectedRevision: live.Thread.Revision, Action: "heartbeat", LeaseID: proof.LeaseID})
		return ack, p.err
	}
	return ack, nil
}

func prepareContinuityMemoryWait(t *testing.T, engine *Engine) OwnedContinuity {
	t.Helper()
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "create-memory-task"}
	document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "保存工作进展"})
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().Add(-time.Second)
	request.RequestID = "add-memory-wait"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "add_wait", Wait: &continuity.Wait{WaitType: "time", DueAt: &due, AutoResume: true}})
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func TestContinuityHeartbeatDoesNotInvalidateMemoryPlan(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	port := &continuityMemoryPort{testDataPort: engine.data.(testDataPort), engine: engine, heartbeat: true}
	engine.data = port
	document := prepareContinuityMemoryWait(t, engine)
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "read-memory-result"}
	live, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, Action: "read"})
	if err != nil || port.err != nil || !port.triggered || live.Lease == nil || live.Lease.State != "completed" {
		t.Fatalf("lease=%+v triggered=%v err=%v hook=%v", live.Lease, port.triggered, err, port.err)
	}
	var count int
	err = db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_memory_jobs`).Scan(&count)
	if err != nil || count != 0 || model.calls.Load() != 1 || model.extractions.Load() != 1 {
		t.Fatalf("memory retry after normal heartbeat: jobs=%d err=%v", count, err)
	}
}

func TestContinuityMemoryRetryAfterCompletedLeaseDoesNotRegenerate(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	port := &continuityMemoryPort{testDataPort: engine.data.(testDataPort), engine: engine, deferMemory: true}
	engine.data = port
	prepareContinuityMemoryWait(t, engine)
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE kernel_device_memory_jobs SET next_attempt_at=0`); err != nil {
		t.Fatal(err)
	}
	jobs, err := engine.coordination.PendingMemoryJobs(t.Context())
	if err != nil || len(jobs) != 1 {
		t.Fatalf("missing deferred memory: %v %v", jobs, err)
	}
	response, err := engine.ResumeMemory(t.Context(), jobs[0])
	if err != nil || response.MemoryStatus != "saved" || model.calls.Load() != 1 || model.extractions.Load() != 1 {
		t.Fatalf("retry=%+v calls=%d extractions=%d err=%v", response, model.calls.Load(), model.extractions.Load(), err)
	}
}
