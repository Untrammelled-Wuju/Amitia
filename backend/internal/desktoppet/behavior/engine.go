package behavior

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior/bindings"
	"github.com/u-ai/backend/log"
)

type EngineConfig struct {
	ShadowMode            bool
	RuntimeCommandEnabled bool
	MailboxCapacity       int
	MaxCASRetries         int
}

const (
	maxInboxAttempts       = 8
	maxInboxBackoff        = 30 * time.Second
	inboxLeaseDuration     = 30 * time.Second
	inboxHeartbeatInterval = 10 * time.Second
)

func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		ShadowMode:            true,
		RuntimeCommandEnabled: false,
		MailboxCapacity:       MailboxCapacity,
		MaxCASRetries:         MaxCASRetries,
	}
}

type BehaviorEngine struct {
	config        EngineConfig
	clock         Clock
	idGen         IDGenerator
	repo          BehaviorStateRepository
	activePetPort ActivePetPort
	runtimePort   RuntimeActionPort
	reducer       *Reducer
	resolver      *Resolver
	arbiter       *Arbiter
	reconciler    *Reconciler
	coordinator   *Coordinator

	lifecycleMu sync.Mutex
	mu          sync.RWMutex
	running     bool
	alive       atomic.Bool
	stopCh      chan struct{}
	runCancel   context.CancelFunc
	wg          sync.WaitGroup

	metrics *EngineMetrics
}

type EngineMetrics struct {
	mu                sync.RWMutex
	eventsReceived    map[string]int64
	eventsDeduped     int64
	eventsExpired     int64
	eventsIgnored     int64
	decisionsTotal    int64
	decisionsNoAction int64
	mailboxOverflow   int64
	casConflicts      int64
	runtimeCommands   int64
	runtimeFailures   int64
}

func NewEngineMetrics() *EngineMetrics {
	return &EngineMetrics{
		eventsReceived: make(map[string]int64),
	}
}

func (m *EngineMetrics) IncEventReceived(eventType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventsReceived[eventType]++
}

func (m *EngineMetrics) IncDeduped()     { m.mu.Lock(); m.eventsDeduped++; m.mu.Unlock() }
func (m *EngineMetrics) IncExpired()     { m.mu.Lock(); m.eventsExpired++; m.mu.Unlock() }
func (m *EngineMetrics) IncIgnored()     { m.mu.Lock(); m.eventsIgnored++; m.mu.Unlock() }
func (m *EngineMetrics) IncDecision()    { m.mu.Lock(); m.decisionsTotal++; m.mu.Unlock() }
func (m *EngineMetrics) IncNoAction()    { m.mu.Lock(); m.decisionsNoAction++; m.mu.Unlock() }
func (m *EngineMetrics) IncOverflow()    { m.mu.Lock(); m.mailboxOverflow++; m.mu.Unlock() }
func (m *EngineMetrics) IncCASConflict() { m.mu.Lock(); m.casConflicts++; m.mu.Unlock() }
func (m *EngineMetrics) IncRuntimeCmd()  { m.mu.Lock(); m.runtimeCommands++; m.mu.Unlock() }
func (m *EngineMetrics) IncRuntimeFail() { m.mu.Lock(); m.runtimeFailures++; m.mu.Unlock() }

func (m *EngineMetrics) Snapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := map[string]interface{}{
		"eventsDeduped":     m.eventsDeduped,
		"eventsExpired":     m.eventsExpired,
		"eventsIgnored":     m.eventsIgnored,
		"decisionsTotal":    m.decisionsTotal,
		"decisionsNoAction": m.decisionsNoAction,
		"mailboxOverflow":   m.mailboxOverflow,
		"casConflicts":      m.casConflicts,
		"runtimeCommands":   m.runtimeCommands,
		"runtimeFailures":   m.runtimeFailures,
	}
	eventsByType := make(map[string]int64)
	for k, v := range m.eventsReceived {
		eventsByType[k] = v
	}
	result["eventsByType"] = eventsByType
	return result
}

func NewBehaviorEngine(
	config EngineConfig,
	clock Clock,
	repo BehaviorStateRepository,
	activePetPort ActivePetPort,
	runtimePort RuntimeActionPort,
	reconciler *Reconciler,
) *BehaviorEngine {
	if clock == nil {
		clock = NewRealClock()
	}
	fallback := DefaultFallbackGraph()
	engine := &BehaviorEngine{
		config:        config,
		clock:         clock,
		idGen:         NewUUIDIDGen(),
		repo:          repo,
		activePetPort: activePetPort,
		runtimePort:   runtimePort,
		reducer:       NewReducer(clock),
		resolver:      NewResolver(clock, fallback),
		arbiter:       NewArbiter(clock, fallback),
		reconciler:    reconciler,
		metrics:       NewEngineMetrics(),
		stopCh:        make(chan struct{}),
	}

	engine.coordinator = NewCoordinator(config.MailboxCapacity, engine.processEvent)

	return engine
}

func (e *BehaviorEngine) Start(ctx context.Context) error {
	e.lifecycleMu.Lock()
	defer e.lifecycleMu.Unlock()

	e.mu.Lock()
	if e.running && e.alive.Load() {
		e.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	e.running = true
	e.alive.Store(true)
	e.stopCh = make(chan struct{})
	e.runCancel = cancel
	// Coordinator.Stop is terminal; every start gets a fresh coordinator.
	coordinator := NewCoordinator(e.config.MailboxCapacity, e.processEvent)
	e.coordinator = coordinator
	shadowMode := e.config.ShadowMode
	runtimeCommandEnabled := e.config.RuntimeCommandEnabled
	e.wg.Add(1)
	go e.inboxWorker(runCtx, coordinator)
	e.mu.Unlock()

	log.Info("behavior engine started", map[string]interface{}{
		"shadowMode":            shadowMode,
		"runtimeCommandEnabled": runtimeCommandEnabled,
	})
	return nil
}

func (e *BehaviorEngine) Stop() error {
	e.lifecycleMu.Lock()
	defer e.lifecycleMu.Unlock()

	e.mu.Lock()
	if !e.running && !e.alive.Load() {
		e.mu.Unlock()
		return nil
	}
	e.running = false
	stopCh := e.stopCh
	cancel := e.runCancel
	coordinator := e.coordinator
	e.runCancel = nil
	e.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if stopCh != nil {
		select {
		case <-stopCh:
		default:
			close(stopCh)
		}
	}
	if coordinator != nil {
		coordinator.Stop()
	}
	e.wg.Wait()
	e.alive.Store(false)
	log.Info("behavior engine stopped")
	return nil
}

func (e *BehaviorEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running && e.alive.Load()
}

func (e *BehaviorEngine) SubmitEvent(ctx context.Context, event BehaviorEventEnvelope) error {
	if !e.IsRunning() {
		return NewBehaviorError(ErrCodeRulesetInvalid, "engine not running")
	}

	now := e.clock.Now()
	if event.EventID == "" {
		event.EventID = e.idGen.NewID()
	}
	if event.DedupKey == "" {
		event.DedupKey = event.EventID
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = now
	}

	e.metrics.IncEventReceived(event.EventType)

	if event.CharacterID == "" {
		e.metrics.IncIgnored()
		return NewBehaviorError(ErrCodeEventSchemaInvalid, "event missing characterId")
	}

	if event.UserID == "" {
		e.metrics.IncIgnored()
		return NewBehaviorError(ErrCodeEventSchemaInvalid, "event missing userId")
	}

	validatedPayload, err := ValidatePayload(event.EventType, event.Payload)
	if err != nil {
		e.metrics.IncIgnored()
		return NewBehaviorErrorWithCause(ErrCodeEventSchemaInvalid, "payload validation failed", err)
	}
	event.Payload = validatedPayload

	if event.ExpiresAt == nil {
		expiresAt := ComputeExpiresAt(event.EventType, event.OccurredAt)
		if expiresAt != nil {
			event.ExpiresAt = expiresAt
		}
	}

	if event.ExpiresAt != nil && now.After(*event.ExpiresAt) {
		e.metrics.IncExpired()
		return nil
	}

	reliability := GetReliability(event.EventType)
	switch reliability {
	case ReliabilityDurable:
		inserted, err := e.repo.InsertInboxIfAbsent(ctx, event)
		if err != nil {
			return err
		}
		if !inserted {
			e.metrics.IncDeduped()
			return nil
		}
		return nil

	case ReliabilityRecoverable:
		inserted, err := e.repo.InsertInboxIfAbsent(ctx, event)
		if err != nil {
			return err
		}
		if !inserted {
			e.metrics.IncDeduped()
			return nil
		}
		return nil

	case ReliabilityEphemeral:
		if e.coordinator.TryEnqueue(event.UserID, event.CharacterID, event) {
			return nil
		}
		e.metrics.IncOverflow()
		return nil
	}

	return nil
}

func (e *BehaviorEngine) HandlePlaybackFeedback(ctx context.Context, feedback PlaybackFeedback) error {
	if feedback.DecisionID == "" {
		return NewBehaviorError(ErrCodeEventSchemaInvalid, "playback feedback missing decisionId")
	}
	decision, err := e.repo.FindDecisionByID(ctx, feedback.DecisionID)
	if err != nil {
		return err
	}
	if decision == nil {
		return NewBehaviorError(ErrCodeEventSchemaInvalid, "playback feedback decision not found")
	}

	payload, err := json.Marshal(map[string]interface{}{
		"commandId":  feedback.CommandID,
		"decisionId": feedback.DecisionID,
		"actionKey":  feedback.ActionKey,
		"errorClass": feedback.ErrorClass,
	})
	if err != nil {
		return err
	}

	occurredAt := feedback.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = e.clock.Now()
	}
	eventType := "runtime.playback.action_" + string(feedback.Phase)
	event := BehaviorEventEnvelope{
		EventID:        "pb_" + feedback.CommandID + "_" + string(feedback.Phase) + "_" + intToStr(feedback.Sequence),
		EventType:      eventType,
		SchemaVersion:  1,
		OccurredAt:     occurredAt,
		ReceivedAt:     e.clock.Now(),
		UserID:         decision.UserID,
		CharacterID:    decision.CharacterID,
		InstallationID: decision.InstallationID,
		PetInstanceID:  feedback.PetInstanceID,
		Sequence:       feedback.Sequence,
		Payload:        payload,
		Origin:         OriginPlayback,
		DedupKey:       feedback.CommandID + ":" + string(feedback.Phase) + ":" + intToStr(feedback.Sequence),
	}
	return e.SubmitEvent(ctx, event)
}

func (e *BehaviorEngine) Reconcile(ctx context.Context, userID, characterID string) error {
	if e.reconciler == nil {
		return nil
	}
	currentCtx, err := e.repo.LoadContext(ctx, userID, characterID)
	if err != nil {
		return err
	}
	_, err = e.reconciler.ReconcileCharacter(ctx, userID, characterID, currentCtx)
	return err
}

func (e *BehaviorEngine) Simulate(ctx context.Context, event BehaviorEventEnvelope) (*BehaviorDecision, error) {
	currentCtx, err := e.repo.LoadContext(ctx, event.UserID, event.CharacterID)
	if err != nil {
		return nil, err
	}

	nextCtx, reduceResult, err := e.reducer.Reduce(*currentCtx, event)
	if err != nil {
		return nil, err
	}
	if !reduceResult.NeedsDecision {
		return &BehaviorDecision{
			Status:     DecisionStatusIgnored,
			ReasonCode: reduceResult.Reason,
			CreatedAt:  e.clock.Now(),
		}, nil
	}

	activePet, err := e.resolveActivePet(ctx, event)
	if err != nil && !IsErrorCode(err, ErrCodeNoActiveInstallation) {
		return nil, err
	}
	if err != nil || activePet == nil {
		return &BehaviorDecision{
			Status:     DecisionStatusIgnored,
			ReasonCode: ErrCodeNoActiveInstallation,
			CreatedAt:  e.clock.Now(),
		}, nil
	}

	runtimeOnline := activePet.RuntimeOnline
	candidates, err := e.resolver.Resolve(&nextCtx, event, activePet)
	if err != nil {
		return nil, err
	}

	decision, err := e.arbiter.Arbitrate(&nextCtx, candidates, activePet, runtimeOnline)
	if err != nil {
		return nil, err
	}

	return decision, nil
}

func (e *BehaviorEngine) GetState(ctx context.Context, userID, characterID string) (*BehaviorContextSnapshot, error) {
	return e.repo.LoadContext(ctx, userID, characterID)
}

func (e *BehaviorEngine) GetMetrics() map[string]interface{} {
	return e.metrics.Snapshot()
}

func (e *BehaviorEngine) SetShadowMode(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config.ShadowMode = enabled
}

func (e *BehaviorEngine) SetRuntimeCommandEnabled(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config.RuntimeCommandEnabled = enabled
}

func (e *BehaviorEngine) SetBindingEvaluator(fn func(scope bindings.EvaluatorScope, eventType string, origin EventOrigin, payload map[string]interface{}) []interface{}) {
	e.resolver.SetBindingEvaluator(fn)
}

func (e *BehaviorEngine) runtimeExecutionConfig() (shadowMode, runtimeCommandEnabled bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config.ShadowMode, e.config.RuntimeCommandEnabled
}

func (e *BehaviorEngine) preparePlaybackEvent(ctx context.Context, event BehaviorEventEnvelope) (BehaviorEventEnvelope, error) {
	switch event.EventType {
	case "runtime.playback.action_started",
		"runtime.playback.action_completed",
		"runtime.playback.action_interrupted",
		"runtime.playback.action_failed":
	default:
		return event, nil
	}

	payload := parsePayload(event.Payload)
	decisionID, _ := payload["decisionId"].(string)
	if decisionID == "" {
		return event, nil
	}

	decision, err := e.repo.FindDecisionByID(ctx, decisionID)
	if err != nil {
		return event, fmt.Errorf("load playback decision: %w", err)
	}
	if decision == nil {
		return event, NewBehaviorError(ErrCodeEventSchemaInvalid, "playback event references an unknown decision")
	}
	if event.UserID != "" && decision.UserID != "" && event.UserID != decision.UserID {
		return event, NewBehaviorError(ErrCodeEventSchemaInvalid, "playback decision user mismatch")
	}
	if event.CharacterID != "" && decision.CharacterID != "" && event.CharacterID != decision.CharacterID {
		return event, NewBehaviorError(ErrCodeEventSchemaInvalid, "playback decision character mismatch")
	}
	if event.InstallationID != "" && decision.InstallationID != "" && event.InstallationID != decision.InstallationID {
		return event, NewBehaviorError(ErrCodeEventSchemaInvalid, "playback decision installation mismatch")
	}
	commandID, _ := payload["commandId"].(string)
	if commandID != "" && decision.RuntimeCommandID != "" && commandID != decision.RuntimeCommandID {
		return event, NewBehaviorError(ErrCodeEventSchemaInvalid, "playback runtime command mismatch")
	}

	if event.EventType != "runtime.playback.action_started" {
		return event, nil
	}

	// Runtime playback feedback intentionally stays compact. Rehydrate the
	// arbitration metadata from the committed decision so ForegroundActionState
	// accurately reflects the action that is physically playing. Without this,
	// the zero-value Interruptible=false made normal actions effectively
	// uninterruptible after action_started.
	payload["semantic"] = decision.Semantic
	payload["interruptible"] = decision.InterruptPolicy != "uninterruptible"
	payload["minimumPlayMs"] = decision.MinimumPlayMS
	payload["maximumPlayMs"] = decision.MaximumPlayMS
	encoded, err := json.Marshal(payload)
	if err != nil {
		return event, fmt.Errorf("encode enriched playback payload: %w", err)
	}
	event.Payload = encoded
	return event, nil
}

func (e *BehaviorEngine) processEvent(ctx context.Context, event BehaviorEventEnvelope) {
	status, err := e.processEventOnce(ctx, event, "")
	if err != nil {
		log.Warn("behavior engine: event processing failed", map[string]interface{}{
			"eventId":     event.EventID,
			"eventType":   event.EventType,
			"characterId": event.CharacterID,
			"status":      status,
			"error":       err.Error(),
		})
	}
}

func (e *BehaviorEngine) processEventOnce(ctx context.Context, event BehaviorEventEnvelope, leaseToken string) (InboxStatus, error) {
	preparedEvent, err := e.preparePlaybackEvent(ctx, event)
	if err != nil {
		return InboxRetry, err
	}
	event = preparedEvent
	// Reliable inbox events use the decision row as their durable processing
	// checkpoint. Ephemeral mailbox events deliberately skip this database lookup.
	if leaseToken != "" {
		existing, err := e.repo.FindDecisionByEventID(ctx, event.EventID)
		if err != nil {
			return InboxRetry, fmt.Errorf("load committed decision: %w", err)
		}
		if existing != nil {
			if err := e.resumeCommittedDecision(ctx, event, &existing.BehaviorDecision); err != nil {
				return InboxRetry, err
			}
			if err := e.applyPlaybackDecisionStatus(ctx, event); err != nil {
				return InboxRetry, err
			}
			return InboxProcessed, nil
		}
	}

	maxRetries := e.config.MaxCASRetries
	if maxRetries <= 0 {
		maxRetries = MaxCASRetries
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return InboxRetry, err
		}
		currentCtx, err := e.repo.LoadContext(ctx, event.UserID, event.CharacterID)
		if err != nil {
			return InboxRetry, fmt.Errorf("load context: %w", err)
		}

		nextCtx, reduceResult, err := e.reducer.Reduce(*currentCtx, event)
		if err != nil {
			return InboxRetry, fmt.Errorf("reduce event: %w", err)
		}
		if reduceResult.NeedsSnapshotSync && e.reconciler != nil {
			syncedCtx, syncErr := e.reconciler.buildReconciledContext(
				ctx, event.UserID, event.CharacterID, &nextCtx, e.clock.Now(),
			)
			if syncErr != nil {
				return InboxRetry, fmt.Errorf("rebuild behavior snapshot: %w", syncErr)
			}
			nextCtx = syncedCtx
			reduceResult.ContextChanged = true
			// Snapshot synchronization exists to restore physical behavior after a
			// reconnect. It must flow through normal resolution/arbitration instead
			// of being persisted as metadata-only state.
			reduceResult.NeedsDecision = true
		}
		if reduceResult.IsDuplicate {
			e.metrics.IncDeduped()
			return InboxIgnored, nil
		}
		if reduceResult.IsExpired {
			e.metrics.IncExpired()
			return InboxIgnored, nil
		}
		if reduceResult.IsOutOfOrder {
			e.metrics.IncIgnored()
			return InboxIgnored, nil
		}
		if !reduceResult.NeedsDecision {
			// Playback status updates are idempotent and monotonic. Apply them
			// before the context/inbox atomic commit so a crash can never leave
			// an acknowledged durable event with an unapplied decision status.
			if err := e.applyPlaybackDecisionStatus(ctx, event); err != nil {
				return InboxRetry, err
			}

			if reduceResult.ContextChanged {
				var committed bool
				if leaseToken != "" {
					committed, err = e.repo.CommitLeasedContextAndInboxCAS(
						ctx, currentCtx.Revision, nextCtx, event.EventID, leaseToken, InboxProcessed,
					)
				} else {
					committed, err = e.repo.SaveContextCAS(ctx, currentCtx.Revision, nextCtx)
				}
				if err != nil {
					return InboxRetry, fmt.Errorf("save context: %w", err)
				}
				if !committed {
					e.metrics.IncCASConflict()
					continue
				}
			}
			return InboxProcessed, nil
		}

		activePet, resolveErr := e.resolveActivePet(ctx, event)
		if resolveErr != nil && !IsErrorCode(resolveErr, ErrCodeNoActiveInstallation) {
			return InboxRetry, fmt.Errorf("resolve active pet: %w", resolveErr)
		}
		if reduceResult.NeedsSnapshotSync && e.reconciler != nil {
			if syncErr := e.reconciler.alignForegroundWithRuntime(ctx, &nextCtx, activePet); syncErr != nil {
				return InboxRetry, syncErr
			}
		}

		var decision *BehaviorDecision
		if activePet == nil || resolveErr != nil {
			e.metrics.IncIgnored()
			decision = &BehaviorDecision{
				Status:     DecisionStatusIgnored,
				ReasonCode: ErrCodeNoActiveInstallation,
				CreatedAt:  e.clock.Now(),
			}
		} else {
			candidates, err := e.resolver.Resolve(&nextCtx, event, activePet)
			if err != nil {
				return InboxRetry, fmt.Errorf("resolve candidates: %w", err)
			}

			decision, err = e.arbiter.Arbitrate(&nextCtx, candidates, activePet, activePet.RuntimeOnline)
			if err != nil {
				return InboxRetry, fmt.Errorf("arbitrate candidates: %w", err)
			}
		}

		if decision == nil {
			return InboxRetry, NewBehaviorError(ErrCodeRulesetInvalid, "arbiter returned nil decision")
		}
		e.normalizeDecision(decision, event, activePet, nextCtx.Revision)
		e.metrics.IncDecision()
		if decision.Status == DecisionStatusNoAction {
			e.metrics.IncNoAction()
		}

		audit := BehaviorDecisionAudit{
			BehaviorDecision: *decision,
			ContextHash:      hashContext(&nextCtx),
		}
		var committed bool
		if leaseToken != "" {
			committed, err = e.repo.CommitLeasedContextAndDecisionCAS(ctx, currentCtx.Revision, nextCtx, audit, event.EventID, leaseToken)
		} else {
			committed, err = e.repo.CommitContextAndDecisionCAS(ctx, currentCtx.Revision, nextCtx, audit)
		}
		if err != nil {
			return InboxRetry, fmt.Errorf("commit context and decision: %w", err)
		}
		if !committed {
			e.metrics.IncCASConflict()
			continue
		}

		if err := e.resumeCommittedDecision(ctx, event, decision); err != nil {
			return InboxRetry, err
		}
		if err := e.applyPlaybackDecisionStatus(ctx, event); err != nil {
			return InboxRetry, err
		}
		return InboxProcessed, nil
	}

	return InboxRetry, NewBehaviorError(ErrCodeContextConflict, "behavior context CAS retries exhausted")
}

func (e *BehaviorEngine) resolveActivePet(ctx context.Context, event BehaviorEventEnvelope) (*ActivePetSnapshot, error) {
	if e.activePetPort == nil {
		return nil, NewBehaviorError(ErrCodeNoActiveInstallation, "active pet port unavailable")
	}
	if targeted, ok := e.activePetPort.(EventTargetedActivePetPort); ok {
		return targeted.ResolveActivePetForEvent(ctx, event)
	}
	return e.activePetPort.ResolveActivePet(ctx, event.UserID, event.CharacterID)
}

func (e *BehaviorEngine) normalizeDecision(decision *BehaviorDecision, event BehaviorEventEnvelope, activePet *ActivePetSnapshot, contextRevision int64) {
	if decision.DecisionID == "" {
		decision.DecisionID = e.idGen.NewID()
	}
	if decision.EventID == "" {
		decision.EventID = event.EventID
	}
	if decision.UserID == "" {
		decision.UserID = event.UserID
	}
	if decision.CharacterID == "" {
		decision.CharacterID = event.CharacterID
	}
	if decision.InstallationID == "" && activePet != nil {
		decision.InstallationID = activePet.InstallationID
	}
	if decision.ContextRevision == 0 {
		decision.ContextRevision = contextRevision
	}
	if decision.RulesetVersion == 0 {
		decision.RulesetVersion = int(CurrentRulesetVersion)
	}
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = e.clock.Now()
	}
}

func (e *BehaviorEngine) resumeCommittedDecision(ctx context.Context, event BehaviorEventEnvelope, decision *BehaviorDecision) error {
	if decision == nil || decision.Status != DecisionStatusSelected {
		return nil
	}
	shadowMode, runtimeCommandEnabled := e.runtimeExecutionConfig()
	if shadowMode || !runtimeCommandEnabled {
		return nil
	}

	targetEvent := event
	if targetEvent.InstallationID == "" {
		targetEvent.InstallationID = decision.InstallationID
	}
	activePet, err := e.resolveActivePet(ctx, targetEvent)
	if err != nil || activePet == nil {
		if err != nil && !IsErrorCode(err, ErrCodeNoActiveInstallation) {
			return fmt.Errorf("resume active pet resolution: %w", err)
		}
		// The decision is already durably committed. Keep it selected and retry
		// until the original installation/runtime returns or the inbox reaches
		// its dead-letter threshold; finalizing it here would lose recovery.
		return NewBehaviorError(ErrCodeNoActiveInstallation, "committed behavior target is temporarily unavailable")
	}
	return e.submitRuntimeCommand(ctx, decision, activePet)
}

func (e *BehaviorEngine) submitRuntimeCommand(ctx context.Context, decision *BehaviorDecision, activePet *ActivePetSnapshot) error {
	if e.runtimePort == nil {
		return NewBehaviorError(ErrCodeRuntimeOffline, "runtime action port unavailable")
	}

	cmd := BehaviorRuntimeCommand{
		CommandID:            e.idGen.NewID(),
		DecisionID:           decision.DecisionID,
		IdempotencyKey:       decision.DecisionID,
		UserID:               activePet.UserID,
		DeviceID:             activePet.DeviceID,
		CharacterID:          activePet.CharacterID,
		RuntimeID:            activePet.RuntimeID,
		PetInstanceID:        activePet.PetInstanceID,
		InstallationID:       activePet.InstallationID,
		InstallationRevision: activePet.StateRevision,
		ContextRevision:      decision.ContextRevision,
		ActionKey:            decision.ActionKey,
		Semantic:             decision.Semantic,
		Priority:             decision.Priority,
		InterruptPolicy:      decision.InterruptPolicy,
		ReasonCode:           decision.ReasonCode,
		MinimumPlayMS:        decision.MinimumPlayMS,
		MaximumPlayMS:        decision.MaximumPlayMS,
		ExpiresAt:            decision.ExpiresAt,
		ReturnPolicy:         decision.ReturnPolicy,
		Durable:              decision.ReturnPolicy != "",
	}

	receipt, runtimeErr := e.runtimePort.SubmitBehaviorCommand(ctx, cmd)
	if runtimeErr != nil {
		e.metrics.IncRuntimeFail()
		log.Warn("behavior engine: runtime command failed", map[string]interface{}{
			"error":      runtimeErr.Error(),
			"actionKey":  decision.ActionKey,
			"decisionId": decision.DecisionID,
		})
		// Keep the committed decision in selected state. The inbox retry will
		// resume this exact decision, and Runtime V2 idempotency prevents a
		// duplicate command if transport acceptance happened before the error.
		return runtimeErr
	}
	if receipt == nil {
		e.metrics.IncRuntimeFail()
		return NewBehaviorError(ErrCodeRuntimeCommandFailed, "runtime returned an empty receipt")
	}

	now := e.clock.Now()
	if receipt.Accepted {
		e.metrics.IncRuntimeCmd()
		decision.Status = DecisionStatusCommandSubmitted
		decision.StartedAt = &now
		if receipt.CommandID != "" {
			decision.RuntimeCommandID = receipt.CommandID
		} else {
			decision.RuntimeCommandID = cmd.CommandID
		}
		return e.repo.UpdateDecisionOutcome(ctx, *decision)
	}

	if receipt.Status == CmdOffline {
		e.metrics.IncRuntimeFail()
		return NewBehaviorError(ErrCodeRuntimeOffline, "target desktop pet runtime is offline")
	}

	e.metrics.IncRuntimeFail()
	decision.Status = DecisionStatusFailed
	decision.CompletedAt = &now
	decision.ReasonCode = receipt.PendingReason
	if decision.ReasonCode == "" {
		decision.ReasonCode = ErrCodeRuntimeCommandFailed
	}
	return e.repo.UpdateDecisionOutcome(ctx, *decision)
}

func (e *BehaviorEngine) applyPlaybackDecisionStatus(ctx context.Context, event BehaviorEventEnvelope) error {
	var status DecisionStatus
	switch event.EventType {
	case "runtime.playback.action_started":
		status = DecisionStatusPlaying
	case "runtime.playback.action_completed":
		status = DecisionStatusCompleted
	case "runtime.playback.action_interrupted":
		status = DecisionStatusInterrupted
	case "runtime.playback.action_failed":
		status = DecisionStatusFailed
	default:
		return nil
	}

	payload := parsePayload(event.Payload)
	decisionID, _ := payload["decisionId"].(string)
	if decisionID == "" {
		return nil
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = e.clock.Now()
	}
	if err := e.repo.UpdateDecisionStatus(ctx, decisionID, status, occurredAt); err != nil {
		return fmt.Errorf("update playback decision status: %w", err)
	}
	return nil
}

func (e *BehaviorEngine) inboxWorker(ctx context.Context, coordinator *Coordinator) {
	defer e.wg.Done()
	defer func() {
		if coordinator != nil {
			coordinator.Stop()
		}
		e.mu.Lock()
		if e.coordinator == coordinator {
			e.running = false
			e.alive.Store(false)
		}
		e.mu.Unlock()
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.processInboxBatch(ctx)
		}
	}
}

func (e *BehaviorEngine) processInboxBatch(ctx context.Context) {
	leaseToken := e.idGen.NewID()
	records, err := e.repo.LeaseInbox(ctx, 16, leaseToken)
	if err != nil {
		log.Warn("behavior engine: lease inbox failed", map[string]interface{}{"error": err.Error()})
		return
	}

	for _, record := range records {
		if ctx.Err() != nil {
			return
		}
		renewed, err := e.repo.RenewInboxLease(ctx, record.EventID, leaseToken, time.Now().UTC().Add(inboxLeaseDuration))
		if err != nil {
			log.Warn("behavior engine: renew inbox lease failed", map[string]interface{}{"eventId": record.EventID, "error": err.Error()})
			continue
		}
		if !renewed {
			continue
		}

		leaseCtx, cancel := context.WithCancel(ctx)
		heartbeatDone := make(chan struct{})
		go e.inboxLeaseHeartbeat(leaseCtx, cancel, record.EventID, leaseToken, heartbeatDone)
		status, processErr := e.safeProcessInboxEvent(leaseCtx, inboxRecordEnvelope(record), leaseToken)
		cancel()
		<-heartbeatDone

		if processErr != nil {
			errorCode := behaviorProcessingErrorCode(processErr)
			if record.AttemptCount >= maxInboxAttempts {
				// Finalize the inbox row and any still-selected decision in one DB
				// transaction. This prevents an orphaned selected decision after the
				// event has already become terminal. Lease fencing remains mandatory.
				if err := e.repo.MarkInboxDeadLetter(ctx, record.EventID, leaseToken, errorCode, truncateBehaviorError(processErr.Error()), e.clock.Now()); err != nil {
					if err != context.Canceled {
						log.Warn("behavior engine: failed to dead-letter inbox event", map[string]interface{}{"eventId": record.EventID, "error": err.Error()})
					}
				}
				continue
			}
			retryAt := time.Now().UTC().Add(inboxRetryBackoff(record.AttemptCount))
			if err := e.repo.MarkInboxRetry(ctx, record.EventID, leaseToken, errorCode, truncateBehaviorError(processErr.Error()), retryAt); err != nil {
				log.Warn("behavior engine: failed to requeue inbox event", map[string]interface{}{"eventId": record.EventID, "error": err.Error()})
			}
			continue
		}

		if status == "" || status == InboxRetry || status == InboxLeased || status == InboxPending {
			status = InboxProcessed
		}
		if err := e.repo.MarkInboxStatus(ctx, record.EventID, leaseToken, status, "", ""); err != nil {
			// A lost lease means another worker is authoritative. Any duplicated
			// Runtime V2 submission is fenced by the decision idempotency key.
			log.Warn("behavior engine: failed to acknowledge inbox event", map[string]interface{}{"eventId": record.EventID, "status": status, "error": err.Error()})
		}
	}
}

func (e *BehaviorEngine) inboxLeaseHeartbeat(ctx context.Context, cancel context.CancelFunc, eventID, leaseToken string, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(inboxHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, err := e.repo.RenewInboxLease(ctx, eventID, leaseToken, time.Now().UTC().Add(inboxLeaseDuration))
			if err != nil || !ok {
				cancel()
				return
			}
		}
	}
}

func (e *BehaviorEngine) safeProcessInboxEvent(ctx context.Context, event BehaviorEventEnvelope, leaseToken string) (status InboxStatus, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			status = InboxRetry
			err = fmt.Errorf("panic while processing behavior event: %v", recovered)
		}
	}()
	return e.processEventOnce(ctx, event, leaseToken)
}

func inboxRecordEnvelope(record InboxRecord) BehaviorEventEnvelope {
	if len(record.EventEnvelopeJSON) > 0 {
		var env BehaviorEventEnvelope
		if err := json.Unmarshal(record.EventEnvelopeJSON, &env); err == nil && env.EventID != "" {
			// Payload JSON is persisted separately and is the canonical filtered
			// payload used by the reducer.
			if len(record.Payload) > 0 {
				env.Payload = record.Payload
			}
			return env
		}
	}
	return BehaviorEventEnvelope{
		EventID:         record.EventID,
		EventType:       record.EventType,
		SchemaVersion:   record.SchemaVersion,
		OccurredAt:      record.OccurredAt,
		ReceivedAt:      record.ReceivedAt,
		ExpiresAt:       record.ExpiresAt,
		UserID:          record.UserID,
		CharacterID:     record.CharacterID,
		ConversationID:  record.ConversationID,
		InteractionID:   record.InteractionID,
		SessionID:       record.SessionID,
		ToolOperationID: record.ToolOperationID,
		InstallationID:  record.InstallationID,
		PetInstanceID:   record.PetInstanceID,
		ReleaseID:       record.ReleaseID,
		Origin:          record.Origin,
		CorrelationID:   record.CorrelationID,
		CausationID:     record.CausationID,
		Sequence:        record.Sequence,
		Payload:         record.Payload,
		DedupKey:        record.DedupKey,
	}
}

func inboxRetryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 5 {
		shift = 5
	}
	backoff := time.Second * time.Duration(1<<shift)
	if backoff > maxInboxBackoff {
		return maxInboxBackoff
	}
	return backoff
}

func behaviorProcessingErrorCode(err error) string {
	if be, ok := IsBehaviorError(err); ok && be.Code != "" {
		return be.Code
	}
	return "behavior_processing_failed"
}

func truncateBehaviorError(message string) string {
	const maxLen = 1024
	if len(message) <= maxLen {
		return message
	}
	return message[:maxLen]
}

func hashContext(ctx *BehaviorContextSnapshot) string {
	if ctx == nil {
		return ""
	}
	data, _ := json.Marshal(ctx)
	return simpleHash(string(data))
}

func simpleHash(s string) string {
	hash := uint64(14695981039346656037)
	for _, c := range s {
		hash ^= uint64(c)
		hash *= 1099511628211
	}
	return intToStr(int64(hash))
}
