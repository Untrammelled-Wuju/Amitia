package behavior

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/u-ai/backend/log"
)

type EngineConfig struct {
	ShadowMode            bool
	RuntimeCommandEnabled bool
	MailboxCapacity       int
	MaxCASRetries         int
}

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

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

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
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return nil
	}
	e.running = true
	e.stopCh = make(chan struct{})

	e.wg.Add(1)
	go e.inboxWorker(ctx)

	log.Info("behavior engine started", map[string]interface{}{
		"shadowMode":            e.config.ShadowMode,
		"runtimeCommandEnabled": e.config.RuntimeCommandEnabled,
	})
	return nil
}

func (e *BehaviorEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return nil
	}
	e.running = false
	close(e.stopCh)
	e.wg.Wait()
	log.Info("behavior engine stopped")
	return nil
}

func (e *BehaviorEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

func (e *BehaviorEngine) SubmitEvent(ctx context.Context, event BehaviorEventEnvelope) error {
	if !e.IsRunning() {
		return NewBehaviorError(ErrCodeRulesetInvalid, "engine not running")
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

	now := e.clock.Now()
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
	payload, _ := json.Marshal(map[string]interface{}{
		"commandId":  feedback.CommandID,
		"decisionId": feedback.DecisionID,
		"actionKey":  feedback.ActionKey,
		"errorClass": feedback.ErrorClass,
	})

	eventType := "runtime.playback.action_" + string(feedback.Phase)
	event := BehaviorEventEnvelope{
		EventID:       "pb_" + feedback.CommandID + "_" + string(feedback.Phase),
		EventType:     eventType,
		SchemaVersion: 1,
		OccurredAt:    feedback.OccurredAt,
		ReceivedAt:    e.clock.Now(),
		Payload:       payload,
		Origin:        OriginPlayback,
		DedupKey:      feedback.CommandID + ":" + string(feedback.Phase) + ":" + intToStr(feedback.Sequence),
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

	activePet, err := e.activePetPort.ResolveActivePet(ctx, event.UserID, event.CharacterID)
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

func (e *BehaviorEngine) SetBindingEvaluator(fn func(scope interface{}, eventType string, origin EventOrigin, payload map[string]interface{}) []interface{}) {
	e.resolver.SetBindingEvaluator(fn)
}

func (e *BehaviorEngine) processEvent(ctx context.Context, event BehaviorEventEnvelope) {
	currentCtx, err := e.repo.LoadContext(ctx, event.UserID, event.CharacterID)
	if err != nil {
		log.Warn("behavior engine: failed to load context", map[string]interface{}{
			"error":       err.Error(),
			"characterId": event.CharacterID,
		})
		return
	}

	nextCtx, reduceResult, err := e.reducer.Reduce(*currentCtx, event)
	if err != nil {
		log.Warn("behavior engine: reducer error", map[string]interface{}{
			"error":     err.Error(),
			"eventType": event.EventType,
		})
		return
	}

	if reduceResult.IsDuplicate {
		e.metrics.IncDeduped()
		return
	}
	if reduceResult.IsExpired {
		e.metrics.IncExpired()
		return
	}
	if !reduceResult.NeedsDecision {
		if reduceResult.ContextChanged {
			e.saveContextCAS(ctx, currentCtx.Revision, nextCtx)
		}
		return
	}

	saved := e.saveContextCAS(ctx, currentCtx.Revision, nextCtx)
	if !saved {
		e.metrics.IncCASConflict()
		for retry := 0; retry < e.config.MaxCASRetries; retry++ {
			currentCtx, err = e.repo.LoadContext(ctx, event.UserID, event.CharacterID)
			if err != nil {
				return
			}
			nextCtx, reduceResult, err = e.reducer.Reduce(*currentCtx, event)
			if err != nil || !reduceResult.NeedsDecision {
				return
			}
			if e.saveContextCAS(ctx, currentCtx.Revision, nextCtx) {
				saved = true
				break
			}
			e.metrics.IncCASConflict()
		}
		if !saved {
			log.Warn("behavior engine: CAS conflict exhausted", map[string]interface{}{
				"characterId": event.CharacterID,
				"eventType":   event.EventType,
			})
			return
		}
	}

	activePet, err := e.activePetPort.ResolveActivePet(ctx, event.UserID, event.CharacterID)
	if err != nil || activePet == nil {
		e.metrics.IncIgnored()
		return
	}

	candidates, err := e.resolver.Resolve(&nextCtx, event, activePet)
	if err != nil {
		log.Warn("behavior engine: resolver error", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	decision, err := e.arbiter.Arbitrate(&nextCtx, candidates, activePet, activePet.RuntimeOnline)
	if err != nil {
		log.Warn("behavior engine: arbiter error", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	e.metrics.IncDecision()
	if decision.Status == DecisionStatusNoAction {
		e.metrics.IncNoAction()
	}

	audit := BehaviorDecisionAudit{
		BehaviorDecision: *decision,
		ContextHash:      hashContext(&nextCtx),
	}
	if err := e.repo.AppendDecision(ctx, audit); err != nil {
		log.Error("behavior engine: failed to append decision audit", map[string]interface{}{
			"error":      err.Error(),
			"decisionId": decision.DecisionID,
		})
		return
	}

	if !e.config.ShadowMode && e.config.RuntimeCommandEnabled && decision.Status == DecisionStatusSelected {
		e.submitRuntimeCommand(ctx, decision, activePet, &nextCtx)
	}
}

func (e *BehaviorEngine) submitRuntimeCommand(ctx context.Context, decision *BehaviorDecision, activePet *ActivePetSnapshot, ctxSnapshot *BehaviorContextSnapshot) {
	if e.runtimePort == nil {
		return
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
		ContextRevision:      ctxSnapshot.Revision,
		ActionKey:            decision.ActionKey,
		Semantic:             decision.Semantic,
		Priority:             decision.Priority,
		InterruptPolicy:      decision.InterruptPolicy,
		ReasonCode:           decision.ReasonCode,
		MinimumPlayMS:        decision.MinimumPlayMS,
		MaximumPlayMS:        decision.MaximumPlayMS,
		ReturnPolicy:         decision.ReturnPolicy,
		Durable:              decision.ReturnPolicy != "",
	}

	receipt, err := e.runtimePort.SubmitBehaviorCommand(ctx, cmd)
	if err != nil {
		e.metrics.IncRuntimeFail()
		log.Warn("behavior engine: runtime command failed", map[string]interface{}{
			"error":      err.Error(),
			"actionKey":  decision.ActionKey,
			"decisionId": decision.DecisionID,
		})
		decision.Status = DecisionStatusFailed
		decision.ReasonCode = ErrCodeRuntimeOffline
		return
	}

	e.metrics.IncRuntimeCmd()

	if receipt != nil && receipt.Accepted {
		decision.Status = DecisionStatusCommandSubmitted
		if receipt.CommandID != "" {
			decision.RuntimeCommandID = receipt.CommandID
		} else {
			decision.RuntimeCommandID = cmd.CommandID
		}
	} else if receipt != nil {
		decision.Status = DecisionStatusFailed
		decision.ReasonCode = receipt.PendingReason
	}
}

func (e *BehaviorEngine) saveContextCAS(ctx context.Context, expectedRevision int64, next BehaviorContextSnapshot) bool {
	ok, err := e.repo.SaveContextCAS(ctx, expectedRevision, next)
	if err != nil {
		log.Warn("behavior engine: save context error", map[string]interface{}{
			"error": err.Error(),
		})
		return false
	}
	return ok
}

func (e *BehaviorEngine) inboxWorker(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.processInboxBatch(ctx)
		}
	}
}

func (e *BehaviorEngine) processInboxBatch(ctx context.Context) {
	leaseToken := e.idGen.NewID()
	records, err := e.repo.LeaseInbox(ctx, 16, leaseToken)
	if err != nil || len(records) == 0 {
		return
	}

	for _, record := range records {
		env := BehaviorEventEnvelope{
			EventID:       record.EventID,
			EventType:     record.EventType,
			SchemaVersion: record.SchemaVersion,
			OccurredAt:    record.OccurredAt,
			ReceivedAt:    record.ReceivedAt,
			ExpiresAt:     record.ExpiresAt,
			UserID:        record.UserID,
			CharacterID:   record.CharacterID,
			Origin:        record.Origin,
			CorrelationID: record.CorrelationID,
			CausationID:   record.CausationID,
			Payload:       record.Payload,
			DedupKey:      record.DedupKey,
		}

		e.processEvent(ctx, env)

		if err := e.repo.MarkInboxStatus(ctx, record.EventID, InboxProcessed, ""); err != nil {
			log.Warn("behavior engine: failed to mark inbox processed", map[string]interface{}{
				"eventId": record.EventID,
				"error":   err.Error(),
			})
		}
	}
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
