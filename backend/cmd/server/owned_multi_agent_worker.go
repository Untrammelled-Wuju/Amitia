package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type ownedWorkerEngine interface {
	RunWorker(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error)
}

type ownedWorkerEnginePort struct {
	engine *business.Engine
}

func (p *ownedWorkerEnginePort) RunWorker(ctx context.Context, scope coordination.ExecutionScope, request business.Request, workspaces []business.ProjectWorkspace, primary string) (business.Response, error) {
	if p == nil || p.engine == nil {
		return business.Response{}, errors.New("owned worker engine unavailable")
	}
	return p.engine.RunWorker(ctx, scope, request, workspaces, primary)
}

type ownedMultiAgentWorkerRunner struct {
	engine  ownedWorkerEngine
	tracker interaction.InteractionTracker
	mu      sync.Mutex
	active  map[string]context.CancelFunc
	closed  bool
	wg      sync.WaitGroup
}

func newOwnedMultiAgentWorkerRunner(engine ownedWorkerEngine, tracker interaction.InteractionTracker) *ownedMultiAgentWorkerRunner {
	return &ownedMultiAgentWorkerRunner{engine: engine, tracker: tracker, active: make(map[string]context.CancelFunc)}
}

func (r *ownedMultiAgentWorkerRunner) WorkerInteractionID(req interaction.WorkerRunRequest) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("amitia-multi-agent:"+req.AssignmentID)).String()
}

func (r *ownedMultiAgentWorkerRunner) StartWorker(ctx context.Context, req interaction.WorkerRunRequest) (string, error) {
	if req.OwnedScope == nil {
		return "", errors.New("multi_agent: worker缺少经过验证的数据归属")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	owned := *req.OwnedScope
	if r.engine == nil || r.tracker == nil || req.AssignmentID == "" || req.Source != "multi_agent" || owned.SpaceID == "" || owned.SpaceID != req.SpaceID || owned.RoleID != req.CharacterID || owned.CoreID == "" || owned.InitiatorDeviceID == "" || owned.TargetDeviceID == "" || owned.ResourceOwnerID == "" {
		return "", errors.New("multi_agent: invalid owned worker scope")
	}
	childID := r.WorkerInteractionID(req)
	workerConversationID := "multi-agent-worker-" + req.AssignmentID
	scope := interaction.InteractionScope{SpaceID: owned.SpaceID, CharacterID: owned.RoleID, ConversationID: workerConversationID, Channel: "web", Source: "multi_agent", RequestID: "ma-" + req.AssignmentID}
	record := interaction.NewInteractionRecord(scope)
	record.ID = childID
	record.ExecutorID = owned.CoreID
	record.CausationID = req.ParentInteractionID
	record.CorrelationID = req.CoordinationID
	encoded, _ := json.Marshal(req)
	fingerprint := sha256.Sum256(encoded)
	record.RecoveryDescriptor = &interaction.RecoveryDescriptor{
		SchemaVersion: interaction.RecoveryDescriptorSchemaVersion,
		Requirement:   interaction.RecoveryRequired,
		State:         interaction.RecoveryDescriptorActive,
		Interaction:   interaction.RecoveryInteractionRef{InteractionID: childID, RequestID: scope.RequestID, Status: record.Status},
		Scope:         interaction.RecoveryScopeRef{SpaceID: scope.SpaceID, CharacterID: scope.CharacterID, ConversationID: scope.ConversationID, Channel: scope.Channel},
		OwnedTurn:     &interaction.OwnedTurnRecoveryRef{Scope: owned, RequestID: scope.RequestID, InputFingerprint: hex.EncodeToString(fingerprint[:])},
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return "", errors.New("multi_agent: owned worker runtime has shut down")
	}
	saved, _, err := interaction.RegisterInteraction(ctx, r.tracker, record)
	if err != nil {
		r.mu.Unlock()
		return "", err
	}
	if saved == nil || saved.ID != childID || saved.Scope.Normalize() != scope.Normalize() || saved.CausationID != req.ParentInteractionID || saved.CorrelationID != req.CoordinationID || saved.ExecutorID != owned.CoreID || saved.RecoveryDescriptor == nil || saved.RecoveryDescriptor.OwnedTurn == nil || *saved.RecoveryDescriptor.OwnedTurn != *record.RecoveryDescriptor.OwnedTurn {
		r.mu.Unlock()
		return "", errors.New("multi_agent: owned worker reservation identity collision")
	}
	if _, ok := r.active[childID]; ok {
		r.mu.Unlock()
		return childID, nil
	}
	if saved.Status != interaction.InteractionStatusReceived {
		r.mu.Unlock()
		return childID, nil
	}
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	runCtx = scope.WithContext(runCtx)
	r.active[childID] = cancel
	r.wg.Add(1)
	r.mu.Unlock()
	go r.run(runCtx, cancel, req, owned, childID, workerConversationID)
	return childID, nil
}

func (r *ownedMultiAgentWorkerRunner) run(ctx context.Context, cancel context.CancelFunc, req interaction.WorkerRunRequest, owned coordination.ExecutionScope, childID, conversationID string) {
	defer r.wg.Done()
	defer cancel()
	defer func() { r.mu.Lock(); delete(r.active, childID); r.mu.Unlock() }()
	defer func() {
		if recovered := recover(); recovered != nil {
			if latest, found, err := r.tracker.Get(context.Background(), childID); err == nil && found && !latest.IsTerminal() {
				_, _ = r.tracker.Fail(context.Background(), childID, latest.StatusVersion, "owned_worker_panic", fmt.Sprint(recovered))
			}
		}
	}()
	stored, found, err := r.tracker.Get(ctx, childID)
	if err != nil || !found || stored == nil {
		return
	}
	if _, err = r.tracker.TransitionCAS(ctx, childID, stored.StatusVersion, interaction.InteractionStatusProcessing); err != nil {
		return
	}
	workspaces := []business.ProjectWorkspace(nil)
	if req.WorkspaceID != "" {
		workspaces = []business.ProjectWorkspace{{ID: req.WorkspaceID, DeviceID: req.WorkspaceDeviceID, Name: req.WorkspaceName, Kind: req.WorkspaceKind, RootURI: req.WorkspaceRootURI}}
	}
	message := req.Objective
	if strings.TrimSpace(req.ExpectedOutcome) != "" {
		message += "\n\n预期结果：" + req.ExpectedOutcome
	}
	response, runErr := r.engine.RunWorker(ctx, owned, business.Request{SpaceID: owned.SpaceID, DeviceID: owned.InitiatorDeviceID, TargetDeviceID: owned.TargetDeviceID, CoreID: owned.CoreID, RoleID: owned.RoleID, ConversationID: conversationID, RequestID: "ma-" + req.AssignmentID, Message: message, PermissionMode: req.PermissionMode}, workspaces, req.WorkspaceID)
	latest, exists, readErr := r.tracker.Get(context.Background(), childID)
	if readErr != nil || !exists || latest == nil || latest.IsTerminal() {
		return
	}
	if ctx.Err() != nil || !latest.CancelRequestedAt.IsZero() || response.Interrupted || errors.Is(runErr, business.ErrInterrupted) {
		_ = r.cancelRecord(context.Background(), childID)
		return
	}
	if runErr == nil && (!response.Saved || response.Partial) {
		runErr = errors.New("owned worker result was not fully committed")
	}
	if runErr != nil {
		_, _ = r.tracker.Fail(context.Background(), childID, latest.StatusVersion, "owned_worker_failed", runErr.Error())
		return
	}
	_, _ = r.tracker.Complete(context.Background(), childID, latest.StatusVersion, response.Text)
}

func (r *ownedMultiAgentWorkerRunner) CancelWorker(ctx context.Context, childID string) error {
	r.mu.Lock()
	cancel, owned := r.active[childID]
	if owned {
		cancel()
	}
	r.mu.Unlock()
	if owned {
		return r.cancelRecord(ctx, childID)
	}
	return r.cancelRecord(ctx, childID)
}

func (r *ownedMultiAgentWorkerRunner) cancelRecord(ctx context.Context, childID string) error {
	if err := r.tracker.RequestCancel(ctx, childID, "owned worker cancelled"); err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		record, found, err := r.tracker.Get(ctx, childID)
		if err != nil {
			return err
		}
		if !found {
			return interaction.ErrInteractionNotFound
		}
		if record.IsTerminal() {
			return nil
		}
		_, err = r.tracker.TransitionCAS(ctx, childID, record.StatusVersion, interaction.InteractionStatusCancelled)
		if !errors.Is(err, interaction.ErrVersionConflict) {
			return err
		}
	}
	return interaction.ErrVersionConflict
}

func (r *ownedMultiAgentWorkerRunner) WorkerIsActive(childID string) bool {
	r.mu.Lock()
	_, active := r.active[childID]
	r.mu.Unlock()
	if active {
		return true
	}
	return false
}

func (r *ownedMultiAgentWorkerRunner) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	r.closed = true
	for _, cancel := range r.active {
		cancel()
	}
	r.mu.Unlock()
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

var _ interaction.AgentWorkerRunner = (*ownedMultiAgentWorkerRunner)(nil)
