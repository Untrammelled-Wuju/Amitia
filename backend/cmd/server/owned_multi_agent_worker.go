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
	"github.com/u-ai/backend/internal/mindruntime"
	amitialog "log"
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

func classifyOwnedWorkerSettlement(cancelRequested bool, response business.Response, runErr error) interaction.InteractionStatus {
	if runErr == nil && response.Saved && !response.Partial && !response.Interrupted {
		return interaction.InteractionStatusCompleted
	}
	if cancelRequested || response.Interrupted || errors.Is(runErr, business.ErrInterrupted) {
		return interaction.InteractionStatusInterrupted
	}
	return interaction.InteractionStatusFailed
}

type ownedMultiAgentWorkerRunner struct {
	engine     ownedWorkerEngine
	tracker    interaction.InteractionTracker
	usageStore *mindruntime.ModelUsageStore
	mu         sync.Mutex
	active     map[string]context.CancelFunc
	closed     bool
	wg         sync.WaitGroup
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
	parent, err := r.verifyWorkerParent(ctx, req)
	if err != nil {
		r.mu.Unlock()
		return "", err
	}
	var durableDeadline *time.Time
	var ownedToolBudget *mindruntime.OwnedToolBudget
	var ownedModelBudget *mindruntime.OwnedModelBudget
	if parent.RecoveryDescriptor != nil && parent.RecoveryDescriptor.MultiAgent != nil {
		ref := parent.RecoveryDescriptor.MultiAgent
		durableDeadline = ref.DeadlineAt
		if ref.MaxToolCalls != nil {
			if r.usageStore == nil || *ref.MaxToolCalls < 1 || *ref.MaxToolCalls > 256 {
				r.mu.Unlock()
				return "", errors.New("multi_agent: durable tool-budget admission unavailable")
			}
			ownedToolBudget = &mindruntime.OwnedToolBudget{SpaceID: owned.SpaceID, RoleID: owned.RoleID, OwnerID: owned.ResourceOwnerID, ParentInteractionID: req.ParentInteractionID, CoordinationID: req.CoordinationID, AssignmentID: req.AssignmentID, Limit: *ref.MaxToolCalls}
		}
		if ref.MaxTotalTokens != nil || ref.MaxCostMicroUSD != nil {
			if r.usageStore == nil || (ref.MaxTotalTokens != nil && (*ref.MaxTotalTokens < 1 || *ref.MaxTotalTokens > 100000000)) || (ref.MaxCostMicroUSD != nil && (*ref.MaxCostMicroUSD < 1 || *ref.MaxCostMicroUSD > 100000000000)) {
				r.mu.Unlock()
				return "", errors.New("multi_agent: durable token-budget admission unavailable")
			}
			ownedModelBudget = &mindruntime.OwnedModelBudget{SpaceID: owned.SpaceID, RoleID: owned.RoleID, OwnerID: owned.ResourceOwnerID, ParentInteractionID: req.ParentInteractionID, CoordinationID: req.CoordinationID, AssignmentID: req.AssignmentID}
			if ref.MaxTotalTokens != nil {
				ownedModelBudget.TokenLimit = *ref.MaxTotalTokens
			}
			if ref.MaxCostMicroUSD != nil {
				ownedModelBudget.CostLimitMicroUSD = *ref.MaxCostMicroUSD
			}
		}
	}
	runDeadline, err := ownedWorkerEffectiveDeadline(time.Now().UTC(), durableDeadline)
	if err != nil {
		r.mu.Unlock()
		return "", err
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
		if saved.IsTerminal() {
			return childID, nil
		}
		return "", fmt.Errorf("multi_agent: worker %s has persisted status %s without a live executor; reconcile its existing results before resuming", childID, saved.Status)
	}
	runCtx, cancel := context.WithDeadline(context.Background(), runDeadline)
	runCtx = scope.WithContext(runCtx)
	runCtx = coordination.WithAdditionalGuard(runCtx, r.workerBudgetAdmissionGuard(childID, req, parent.RecoveryDescriptor.MultiAgent))
	if ownedToolBudget != nil || ownedModelBudget != nil {
		runCtx = mindruntime.WithDurableOwnedBudgetParent(runCtx, mindruntime.OwnedBudgetParentLimits{
			ToolCalls:          snapshotOwnedWorkerBudget(parent.RecoveryDescriptor.MultiAgent).toolLimit,
			Tokens:             snapshotOwnedWorkerBudget(parent.RecoveryDescriptor.MultiAgent).tokenLimit,
			CostMicroUSD:       snapshotOwnedWorkerBudget(parent.RecoveryDescriptor.MultiAgent).costLimit,
			ChildInteractionID: childID,
		})
	}
	if ownedToolBudget != nil {
		runCtx = mindruntime.WithOwnedToolBudget(runCtx, *ownedToolBudget)
	}
	if ownedModelBudget != nil {
		runCtx = mindruntime.WithOwnedModelBudget(runCtx, *ownedModelBudget)
	}
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
				if _, err := r.tracker.TransitionCAS(context.Background(), childID, latest.StatusVersion, interaction.InteractionStatusInterrupted); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
			}
		}
	}()
	stored, found, err := r.tracker.Get(ctx, childID)
	if err != nil || !found || stored == nil {
		return
	}
	if !stored.CancelRequestedAt.IsZero() {
		if err := r.cancelRecord(context.Background(), childID); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return
	}
	if _, err = r.tracker.TransitionCAS(ctx, childID, stored.StatusVersion, interaction.InteractionStatusProcessing); err != nil {
		return
	}
	cancelled, probeErr := r.workerCancellationRequested(ctx, childID, req)
	if probeErr != nil {
		r.settleUnverifiedWorker(childID)
		return
	}
	if cancelled {
		if err := r.cancelRecord(context.Background(), childID); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return
	}
	watchDone := make(chan struct{})
	watchStopped := make(chan struct{})
	go func() {
		defer close(watchStopped)
		r.watchWorkerCancellation(ctx, cancel, childID, req, watchDone)
	}()
	defer func() {
		close(watchDone)
		<-watchStopped
	}()
	workspaces := []business.ProjectWorkspace(nil)
	if req.WorkspaceID != "" {
		workspaces = []business.ProjectWorkspace{{ID: req.WorkspaceID, DeviceID: req.WorkspaceDeviceID, Name: req.WorkspaceName, Kind: req.WorkspaceKind, RootURI: req.WorkspaceRootURI}}
	}
	message := req.Objective
	if strings.TrimSpace(req.ExpectedOutcome) != "" {
		message += "\n\n预期结果：" + req.ExpectedOutcome
	}
	response, runErr := r.engine.RunWorker(ctx, owned, business.Request{SpaceID: owned.SpaceID, DeviceID: owned.InitiatorDeviceID, TargetDeviceID: owned.TargetDeviceID, CoreID: owned.CoreID, RoleID: owned.RoleID, ConversationID: conversationID, RequestID: "ma-" + req.AssignmentID, Message: message, PermissionMode: req.PermissionMode}, workspaces, req.WorkspaceID)
	r.settleWorker(ctx, childID, req, response, runErr)
}

func (r *ownedMultiAgentWorkerRunner) CancelWorker(ctx context.Context, childID string) error {
	if err := r.tracker.RequestCancel(ctx, childID, "owned worker cancellation requested"); err != nil {
		return err
	}
	r.mu.Lock()
	cancel, owned := r.active[childID]
	if owned {
		cancel()
	}
	r.mu.Unlock()
	if owned {

		return nil
	}

	return r.cancelUnstartedRecord(ctx, childID)
}

func (r *ownedMultiAgentWorkerRunner) cancelUnstartedRecord(ctx context.Context, childID string) error {
	record, found, err := r.tracker.Get(ctx, childID)
	if err != nil {
		return err
	}
	if !found || record == nil {
		return interaction.ErrInteractionNotFound
	}
	if record.IsTerminal() || record.Status != interaction.InteractionStatusReceived {
		return nil
	}
	_, err = r.tracker.TransitionCAS(ctx, childID, record.StatusVersion, interaction.InteractionStatusCancelled)
	if errors.Is(err, interaction.ErrVersionConflict) {
		return nil
	}
	return err
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
