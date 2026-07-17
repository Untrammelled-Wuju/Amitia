package interaction

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (o *Orchestrator) Process(ctx context.Context, req *ProcessRequest) (*OrchestrationResult, error) {
	if !o.IsReady() {
		return nil, ErrOrchestratorNotReady
	}
	o.mu.Lock()
	if o.active >= o.cfg.MaxConcurrent {
		o.mu.Unlock()
		return nil, ErrOrchestratorBusy
	}
	o.active++
	o.mu.Unlock()
	defer func() { o.mu.Lock(); o.active--; o.mu.Unlock() }()
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	scope := o.buildScope(req)
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if existing, ok, err := o.tracker.GetByRequestID(ctx, scope.UserID, scope.RequestID); err != nil {
		return nil, err
	} else if ok {
		return o.handleIdempotentHit(existing)
	}
	record := NewInteractionRecord(scope)
	if err := o.tracker.Create(ctx, record); err != nil {
		if errors.Is(err, ErrDuplicateRequest) {
			if existing, ok, getErr := o.tracker.GetByRequestID(ctx, scope.UserID, scope.RequestID); getErr != nil {
				return nil, getErr
			} else if ok {
				return o.handleIdempotentHit(existing)
			}
		}
		return nil, err
	}
	resolution, err := o.resolver.ResolveExcluding(ctx, scope, record.ID)
	if err != nil {
		record.SetError(err.Error())
		if failed, failErr := o.tracker.Fail(ctx, record.ID, record.StatusVersion, "supersede_resolve_failed", err.Error()); failErr == nil {
			record = failed
		}
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	if resolution.RejectNew {
		err := errors.New("orchestrator: too many queued interactions")
		record.SetError(err.Error())
		if failed, failErr := o.tracker.Fail(ctx, record.ID, record.StatusVersion, "queue_full", err.Error()); failErr == nil {
			record = failed
		}
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	if resolution.SupersedeTargetID != "" && resolution.SupersedeTargetID != record.ID {
		if err := o.supersedeTarget(ctx, resolution.SupersedeTargetID, record.ID); err != nil {
			record.SetError(err.Error())
			if failed, failErr := o.tracker.Fail(ctx, record.ID, record.StatusVersion, "supersede_failed", err.Error()); failErr == nil {
				record = failed
			}
			return o.buildResult(record, nil, OutcomeFailed, err), err
		}
		if updated, err := o.tracker.UpdateMetadata(ctx, record.ID, InteractionMetadataUpdate{SupersedesID: &resolution.SupersedeTargetID}); err == nil {
			record = updated
		} else {
			if failed, failErr := o.tracker.Fail(ctx, record.ID, record.StatusVersion, "metadata_update_failed", err.Error()); failErr == nil {
				record = failed
			}
			return o.buildResult(record, nil, OutcomeFailed, err), err
		}
	}
	if resolution.Enqueue {
		record, err = o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusQueued)
		if err != nil {
			return o.buildResult(record, nil, OutcomeFailed, err), err
		}
	}
	if o.cfg.SupersedePolicy == SupersedePolicyQueue {
		releaseQueue := o.acquireQueueScope(scope)
		defer releaseQueue()
	}
	if resolution.Enqueue {
		record, err = o.waitForQueueTurn(ctx, scope, record)
		if err != nil {
			return o.buildResult(record, nil, outcomeFromQueueWaitError(err), err), err
		}
	}
	record, err = o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusProcessing)
	if err != nil {
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	var processCtx context.Context
	var cancel context.CancelFunc
	if o.deadlineFn != nil {
		processCtx, cancel = o.deadlineFn(ctx, req.RequestID)
	} else {
		processCtx, cancel = context.WithTimeout(ctx, o.cfg.DefaultTimeout)
	}
	defer cancel()
	o.cancels.Register(record.ID, cancel)
	defer o.cancels.Unregister(record.ID)
	runtime := o.pipeline.Assemble(processCtx, scope, req)
	runtime.ExecutorID = executorIDForProcessor(o.processor)
	if updated, err := o.tracker.UpdateMetadata(ctx, record.ID, metadataFromRuntime(runtime, processCtx)); err == nil {
		record = updated
	} else {
		o.tracker.Fail(ctx, record.ID, record.StatusVersion, "metadata_update_failed", err.Error())
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	req.InteractionID, req.Runtime = record.ID, &runtime
	if next, err := o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusContextReady); err == nil {
		record = next
	} else {
		o.tracker.Fail(ctx, record.ID, record.StatusVersion, "context_ready_failed", err.Error())
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	req.ExpectedStatusVersion = record.StatusVersion
	if runtime.Safety.Blocked {
		blockErr := error(ErrOrchestratorSafetyBlocked)
		if len(runtime.Safety.Reasons) > 0 {
			blockErr = fmt.Errorf("%w: %s", ErrOrchestratorSafetyBlocked, strings.Join(runtime.Safety.Reasons, ","))
		}
		if failed, failErr := o.tracker.Fail(ctx, record.ID, record.StatusVersion, "safety_blocked", blockErr.Error()); failErr == nil {
			record = failed
		} else {
			return o.buildResult(record, nil, OutcomeFailed, failErr), failErr
		}
		return o.buildResult(record, nil, OutcomeFailed, blockErr), blockErr
	}
	start := time.Now()
	resp, err := o.processor.ProcessMessageCtx(processCtx, req)
	duration := time.Since(start)
	if err != nil {
		return o.handleProcessorError(ctx, record, req, resp, duration, err)
	}
	if fresh, ok, getErr := o.tracker.Get(processCtx, record.ID); getErr == nil && ok {
		record = fresh
	}
	return o.finalizeProcessorSuccess(ctx, record, req, resp, runtime, duration)
}

func (o *Orchestrator) handleProcessorError(ctx context.Context, record *InteractionRecord, req *ProcessRequest, resp *ProcessResponse, duration time.Duration, procErr error) (*OrchestrationResult, error) {
	if fresh, ok, getErr := o.tracker.Get(ctx, record.ID); getErr == nil && ok {
		record = fresh
	}
	if record.Status == InteractionStatusCommitted || record.Status == InteractionStatusCompleted {
		log.Printf("[orchestrator] interaction %s committed despite processor error: %v", record.ID, procErr)
		if resp != nil {
			resp.RequestID, resp.ConversationID, resp.CharacterID = req.RequestID, req.ConversationID, req.CharacterID
		}
		result := o.buildResult(record, resp, OutcomeCompleted, nil)
		result.Duration = duration
		if resp != nil {
			result.Events = resp.Events
		}
		return result, nil
	}
	if errors.Is(procErr, context.Canceled) || errors.Is(procErr, context.DeadlineExceeded) {
		record.CancelReason = procErr.Error()
		if cancelErr := o.tracker.RequestCancel(ctx, record.ID, procErr.Error()); cancelErr != nil {
			return o.buildResult(record, nil, OutcomeFailed, cancelErr), cancelErr
		}
		if fresh, ok, getErr := o.tracker.Get(ctx, record.ID); getErr != nil {
			return o.buildResult(record, nil, OutcomeFailed, getErr), getErr
		} else if ok {
			record = fresh
		}
		if cancelled, failErr := o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusCancelled); failErr == nil {
			record = cancelled
		}
		return o.buildResult(record, nil, OutcomeCancelled, procErr), procErr
	}
	record.SetError(procErr.Error())
	if failed, failErr := o.tracker.Fail(ctx, record.ID, record.StatusVersion, "processor_failed", procErr.Error()); failErr == nil {
		record = failed
	}
	return o.buildResult(record, nil, OutcomeFailed, procErr), procErr
}
