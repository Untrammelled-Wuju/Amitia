package interaction

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type asyncUnifiedEntryWorkerRunner struct {
	entry   *UnifiedEntry
	tracker InteractionTracker
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	active  map[string]context.CancelFunc
	closed  bool
	wg      sync.WaitGroup
}

func NewAsyncUnifiedEntryWorkerRunner(entry *UnifiedEntry, tracker InteractionTracker) AgentWorkerRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &asyncUnifiedEntryWorkerRunner{
		entry: entry, tracker: tracker, ctx: ctx, cancel: cancel, active: make(map[string]context.CancelFunc),
	}
}

func (r *asyncUnifiedEntryWorkerRunner) WorkerInteractionID(req WorkerRunRequest) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("amitia-multi-agent:"+req.AssignmentID)).String()
}

func (r *asyncUnifiedEntryWorkerRunner) StartWorker(ctx context.Context, req WorkerRunRequest) (string, error) {
	if r == nil || r.entry == nil || r.tracker == nil || req.AssignmentID == "" {
		return "", fmt.Errorf("multi_agent: async worker requires an entry, tracker and assignment ID")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	childID := r.WorkerInteractionID(req)
	input := &UnifiedEntryRequest{
		Channel: "web", Message: req.Objective, SpaceID: req.SpaceID, CharacterID: req.CharacterID,
		ConversationID: workerConversationID(req), WorkspaceID: req.WorkspaceID,
		PermissionMode: req.PermissionMode, WorkspaceDeviceID: req.WorkspaceDeviceID,
		WorkspaceName: req.WorkspaceName, WorkspaceKind: req.WorkspaceKind,
		WorkspaceRootURI: req.WorkspaceRootURI, Source: req.Source,
		RequestID: "ma-" + req.AssignmentID, IsInternal: true, ReservedInteractionID: childID,
	}
	resolved, err := r.entry.ResolveScope(ctx, input)
	if err != nil {
		return "", fmt.Errorf("multi_agent: resolve reserved worker scope: %w", err)
	}
	if resolved.Scope.SpaceID == "" || resolved.Scope.ConversationID != input.ConversationID ||
		resolved.Scope.CharacterID != input.CharacterID {
		return "", fmt.Errorf("multi_agent: resolved worker identity does not match assigned workspace scope")
	}
	scope := resolved.Scope.Normalize()
	if scope.RequestID == "" {
		return "", fmt.Errorf("multi_agent: worker request identity is missing")
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return "", fmt.Errorf("multi_agent: worker runtime has shut down")
	}
	if _, active := r.active[childID]; active {
		r.mu.Unlock()
		return childID, nil
	}
	rec, exists, getErr := r.tracker.GetByRequestID(ctx, scope.SpaceID, scope.RequestID)
	if getErr != nil {
		r.mu.Unlock()
		return "", getErr
	}
	if !exists {
		record := NewInteractionRecord(scope)
		record.ID = childID
		if createErr := r.tracker.Create(ctx, record); createErr != nil {
			if !errors.Is(createErr, ErrDuplicateRequest) {
				r.mu.Unlock()
				return "", createErr
			}
			rec, exists, getErr = r.tracker.GetByRequestID(ctx, scope.SpaceID, scope.RequestID)
			if getErr != nil || !exists {
				r.mu.Unlock()
				return "", fmt.Errorf("multi_agent: failed to resolve reserved interaction collision: %v", getErr)
			}
		} else {
			rec = record
		}
	}
	if rec == nil || rec.ID != childID || !sameSupersedeScope(rec.Scope, scope) ||
		rec.Scope.RequestID != scope.RequestID {
		r.mu.Unlock()
		return "", fmt.Errorf("multi_agent: child reservation identity collision")
	}
	if rec.Status != InteractionStatusReceived {
		r.mu.Unlock()
		return childID, nil
	}
	runCtx, cancel := context.WithTimeout(r.ctx, 30*time.Minute)
	r.active[childID] = cancel
	r.wg.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		defer func() {
			r.mu.Lock()
			delete(r.active, childID)
			r.mu.Unlock()
		}()
		defer cancel()
		_, runErr := r.entry.Handle(runCtx, input)
		if runErr != nil {
			if latest, found, readErr := r.tracker.Get(context.Background(), childID); readErr == nil && found &&
				latest != nil && latest.Status == InteractionStatusReceived {
				_, _ = r.tracker.Fail(context.Background(), childID, latest.StatusVersion, "worker_launch_failed", runErr.Error())
			}
		}
	}()
	return childID, nil
}

func (r *asyncUnifiedEntryWorkerRunner) CancelWorker(ctx context.Context, childID string) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	cancel := r.active[childID]
	if cancel != nil {
		cancel()
	}
	r.mu.Unlock()
	if r.tracker == nil {
		return nil
	}
	if err := r.tracker.RequestCancel(ctx, childID, "coordination_cancelled"); err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		record, found, err := r.tracker.Get(ctx, childID)
		if err != nil {
			return err
		}
		if !found {
			return ErrInteractionNotFound
		}
		if record.IsTerminal() {
			return nil
		}
		_, err = r.tracker.TransitionCAS(ctx, childID, record.StatusVersion, InteractionStatusCancelled)
		if !errors.Is(err, ErrVersionConflict) {
			return err
		}
	}
	return ErrVersionConflict
}

func (r *asyncUnifiedEntryWorkerRunner) WorkerIsActive(childID string) bool {
	if r == nil || childID == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, active := r.active[childID]
	return active
}

func (r *asyncUnifiedEntryWorkerRunner) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	r.closed = true
	r.cancel()
	r.mu.Unlock()
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
