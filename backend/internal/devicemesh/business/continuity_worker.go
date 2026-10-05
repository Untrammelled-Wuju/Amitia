package business

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func continuityRequest(scope coordination.ExecutionScope) Request {
	return Request{SpaceID: scope.SpaceID, DeviceID: scope.InitiatorDeviceID, CoreID: scope.CoreID, TargetDeviceID: scope.TargetDeviceID, RoleID: scope.RoleID, RequestID: uuid.NewString()}
}

func (e *Engine) TickContinuity(ctx context.Context) error {
	locations, err := e.coordination.PendingContinuity(ctx)
	if err != nil {
		return err
	}
	var workers sync.WaitGroup
	defer workers.Wait()
	for _, location := range locations {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e.continuitySlots <- struct{}{}:
		}
		workers.Add(1)
		go func(location coordination.ContinuityLocation) {
			defer workers.Done()
			defer func() { <-e.continuitySlots }()
			e.tickContinuityLocation(ctx, location)
		}(location)
	}
	return nil
}

func (e *Engine) tickContinuityLocation(ctx context.Context, location coordination.ContinuityLocation) {
	if err := e.coordination.Validate(ctx, location.Scope); err != nil {
		if errors.Is(err, coordination.ErrScopeExpired) || errors.Is(err, coordination.ErrWrongOwner) {
			_ = e.coordination.ForgetContinuity(ctx, location)
		} else {
			_ = e.coordination.DeferContinuity(ctx, location, time.Minute)
		}
		return
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, location.Scope); err != nil {
		_ = e.coordination.DeferContinuity(ctx, location, time.Minute)
		return
	}
	request := continuityRequest(location.Scope)
	document, _, err := e.Continuity(ctx, request, ContinuityMutation{ID: location.ID, Action: "read", rawRead: true})
	if err != nil {
		_ = e.coordination.DeferContinuity(ctx, location, time.Minute)
		return
	}
	if document.Thread.Revision < location.Revision {
		_ = e.coordination.DeferContinuity(ctx, location, time.Minute)
		return
	}
	if document.Thread.Status.IsTerminal() || document.Thread.Status == continuity.ThreadStatusPaused {
		_ = e.coordination.ForgetContinuity(ctx, location)
		return
	}
	if document.Lease != nil && document.Lease.State == "running" {
		if !document.Lease.ExpiresAt.After(time.Now()) {
			request.RequestID = uuid.NewString()
			_, _, _ = e.Continuity(ctx, request, ContinuityMutation{ID: location.ID, ExpectedRevision: document.Thread.Revision, Action: "mark_unknown"})
		}
		_ = e.coordination.DeferContinuity(ctx, location, 15*time.Second)
		return
	}
	claimed := false
	for _, wait := range document.Waits {
		timeReady := wait.Status == continuity.WaitStatusWaiting && wait.WaitType == continuity.WaitTypeTime && wait.DueAt != nil && !wait.DueAt.After(time.Now())
		resolvedReady := wait.Status == continuity.WaitStatusResolved && wait.WakeState == continuity.WakeStatePending
		if !wait.AutoResume || (!timeReady && !resolvedReady) {
			continue
		}
		request.RequestID = uuid.NewString()
		lease, ack, claimErr := e.Continuity(ctx, request, ContinuityMutation{ID: location.ID, ExpectedRevision: document.Thread.Revision, Action: "claim", WaitID: wait.ID})
		if claimErr == nil && lease.Lease != nil && ack.Versions["continuity/"+location.ID] == lease.Thread.Revision {
			claimed = true
			_ = e.executeContinuity(ctx, lease, wait)
		}
		break
	}
	if !claimed {
		_ = e.coordination.DeferContinuity(ctx, location, 15*time.Second)
	}
}

func (e *Engine) executeContinuity(parent context.Context, document OwnedContinuity, wait continuity.Wait) error {
	parent, timeoutCancel := context.WithTimeout(parent, 3*time.Minute)
	defer timeoutCancel()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return errors.New("持续事项数据端口未就绪")
	}
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	leaseResource := func(current context.Context) (*coordination.Resource, error) {
		resource, err := port.Resource(coordination.WithoutAdditionalGuard(current), document.Scope, "continuity", document.Thread.ID)
		if err != nil {
			return nil, err
		}
		var live OwnedContinuity
		if resource == nil || resource.Deleted || json.Unmarshal(resource.Body, &live) != nil || live.Lease == nil || live.Lease.ID != document.Lease.ID || live.Lease.State != "running" || !live.Lease.ExpiresAt.After(time.Now()) || live.Thread.Status == continuity.ThreadStatusPaused || live.Thread.Status.IsTerminal() || live.CoreID != document.CoreID {
			return nil, ErrUncertainExecution
		}
		return resource, nil
	}
	check := func(current context.Context) error { _, err := leaseResource(current); return err }
	ctx = coordination.WithAdditionalGuard(ctx, check)
	ctx = coordination.WithCommitLease(ctx, coordination.CommitLeaseProof{ContinuityID: document.Thread.ID, LeaseID: document.Lease.ID, RequestID: document.Lease.RequestID})
	if err := check(ctx); err != nil {
		return err
	}
	heartbeatStop := make(chan struct{})
	heartbeatDone := make(chan struct{})
	var stopOnce sync.Once
	stopHeartbeat := func() { stopOnce.Do(func() { close(heartbeatStop) }); <-heartbeatDone }
	defer stopHeartbeat()
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatStop:
				return
			case <-ticker.C:
				if err := check(ctx); err != nil {
					cancel(err)
					return
				}
				request := continuityRequest(document.Scope)
				live, _, err := e.Continuity(ctx, request, ContinuityMutation{ID: document.Thread.ID, Action: "read"})
				if err != nil {
					cancel(err)
					return
				}
				_, _, err = e.Continuity(ctx, request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: live.Thread.Revision, Action: "heartbeat", LeaseID: document.Lease.ID})
				if err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	request := continuityRequest(document.Scope)
	request.RequestID = document.Lease.RequestID
	request.ConversationID = document.Thread.ID
	request.Message = "继续处理持续事项：" + document.Thread.Title + "\n目标：" + document.Thread.Goal + "\n当前摘要：" + document.Thread.Summary + "\n等待已到期：" + wait.Description + "\n继续提示：" + wait.ResumeHint
	response, err := e.Run(ctx, request)
	stopHeartbeat()
	if err != nil || !response.Saved {
		if err == nil {
			err = ErrUncertainExecution
		}
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		request.RequestID = uuid.NewString()
		live, _, err := e.Continuity(ctx, request, ContinuityMutation{ID: document.Thread.ID, Action: "read"})
		if err != nil {
			return err
		}
		_, _, err = e.Continuity(ctx, request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: live.Thread.Revision, Action: "finish", LeaseID: document.Lease.ID, Result: response.Text})
		if !errors.Is(err, coordination.ErrResourceVersion) {
			return err
		}
	}
	return coordination.ErrResourceVersion
}

func (e *Engine) RunContinuityWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = e.TickContinuity(ctx)
		}
	}
}
