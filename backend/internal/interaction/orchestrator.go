package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOrchestratorNotReady      = errors.New("orchestrator: not ready")
	ErrOrchestratorBusy          = errors.New("orchestrator: too many concurrent interactions")
	ErrOrchestratorCancelled     = errors.New("orchestrator: cancelled")
	ErrOrchestratorSuperseded    = errors.New("orchestrator: superseded")
	ErrOrchestratorDuplicate     = errors.New("orchestrator: duplicate request")
	ErrOrchestratorInvalidScope  = errors.New("orchestrator: invalid scope")
	ErrOrchestratorSafetyBlocked = errors.New("orchestrator: safety blocked")
)

type ProcessRequest struct {
	CharacterID    string           `json:"characterId,omitempty"`
	Message        string           `json:"message"`
	ConversationID string           `json:"conversationId,omitempty"`
	Channel        string           `json:"channel,omitempty"`
	Source         string           `json:"source,omitempty"`
	PeerID         string           `json:"peerId,omitempty"`
	UserID         string           `json:"userId,omitempty"`
	SessionID      string           `json:"sessionId,omitempty"`
	AudioUrl       string           `json:"audioUrl,omitempty"`
	AudioDuration  float64          `json:"audioDuration,omitempty"`
	VoiceMessage   bool             `json:"voiceMessage"`
	ImageUrl       string           `json:"imageUrl,omitempty"`
	VideoUrl       string           `json:"videoUrl,omitempty"`
	ImageContext   string           `json:"imageContext,omitempty"`
	RequestID      string           `json:"requestId,omitempty"`
	InteractionID  string           `json:"-"`
	Runtime        *RuntimeAssembly `json:"-"`
}

type ProcessResponse struct {
	ConversationID string         `json:"conversationId"`
	Reply          string         `json:"reply"`
	CharacterID    string         `json:"characterId"`
	CharacterName  string         `json:"characterName"`
	MessageIDs     []string       `json:"messageIds"`
	ForceVoice     bool           `json:"forceVoice"`
	AudioUrls      []string       `json:"audioUrls"`
	RequestID      string         `json:"requestId"`
	Events         []OutboxRecord `json:"-"`
}

type MessageProcessor interface {
	ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error)
}

type Outcome string

const (
	OutcomeCompleted  Outcome = "completed"
	OutcomeFailed     Outcome = "failed"
	OutcomeCancelled  Outcome = "cancelled"
	OutcomeSuperseded Outcome = "superseded"
)

type OrchestrationResult struct {
	InteractionID string           `json:"interactionId"`
	Outcome       Outcome          `json:"outcome"`
	Response      *ProcessResponse `json:"response,omitempty"`
	Error         string           `json:"error,omitempty"`
	Duration      time.Duration    `json:"duration"`
	Events        []OutboxRecord   `json:"events,omitempty"`
}

type OrchestratorConfig struct {
	MaxConcurrent   int
	SupersedePolicy SupersedePolicy
	DefaultTimeout  time.Duration
}

func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		MaxConcurrent:   10,
		SupersedePolicy: SupersedePolicyLatest,
		DefaultTimeout:  180 * time.Second,
	}
}

type Orchestrator struct {
	cfg        OrchestratorConfig
	processor  MessageProcessor
	tracker    InteractionTracker
	outbox     OutboxStore
	resolver   *SupersedeResolver
	pipeline   *RuntimePipeline
	cancels    *CancellationRegistry
	deadlineFn func(ctx context.Context, requestID string) (context.Context, context.CancelFunc)
	mu         sync.Mutex
	active     int
	ready      bool
}

func NewOrchestrator(cfg OrchestratorConfig, processor MessageProcessor) *Orchestrator {
	tracker := NewInMemoryTracker()
	return newOrchestratorWithStores(cfg, processor, tracker, NewInMemoryOutboxStore())
}

func NewOrchestratorWithStores(cfg OrchestratorConfig, processor MessageProcessor, tracker InteractionTracker, outbox OutboxStore) *Orchestrator {
	return newOrchestratorWithStores(cfg, processor, tracker, outbox)
}

func newOrchestratorWithStores(cfg OrchestratorConfig, processor MessageProcessor, tracker InteractionTracker, outbox OutboxStore) *Orchestrator {
	cfg = normalizeOrchestratorConfig(cfg)
	return &Orchestrator{
		cfg:       cfg,
		processor: processor,
		tracker:   tracker,
		outbox:    outbox,
		resolver:  NewSupersedeResolver(cfg.SupersedePolicy, tracker),
		pipeline:  NewRuntimePipeline(nil, nil, nil),
		cancels:   NewCancellationRegistry(),
		ready:     false,
	}
}

func normalizeOrchestratorConfig(cfg OrchestratorConfig) OrchestratorConfig {
	defaults := DefaultOrchestratorConfig()
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = defaults.MaxConcurrent
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = defaults.DefaultTimeout
	}
	if cfg.SupersedePolicy == "" {
		cfg.SupersedePolicy = defaults.SupersedePolicy
	}
	return cfg
}

func (e *UnifiedEntry) SetOrchestratorReady(ready bool) {
	if ready {
		e.orchestrator.SetReady(true)
	} else {
		e.orchestrator.SetReady(false)
	}
}

func (o *Orchestrator) SetReady(ready bool) {
	o.mu.Lock()
	o.ready = ready
	o.mu.Unlock()
}

func (o *Orchestrator) IsReady() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.ready
}

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

	defer func() {
		o.mu.Lock()
		o.active--
		o.mu.Unlock()
	}()

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
		return o.buildResult(existing, nil, outcomeForRecord(existing), ErrOrchestratorDuplicate), ErrOrchestratorDuplicate
	}

	record := NewInteractionRecord(scope)
	if err := o.tracker.Create(ctx, record); err != nil {
		if errors.Is(err, ErrDuplicateRequest) {
			if existing, ok, getErr := o.tracker.GetByRequestID(ctx, scope.UserID, scope.RequestID); getErr != nil {
				return nil, getErr
			} else if ok {
				return o.buildResult(existing, nil, outcomeForRecord(existing), ErrOrchestratorDuplicate), ErrOrchestratorDuplicate
			}
		}
		return nil, err
	}

	resolution, err := o.resolver.ResolveExcluding(ctx, scope, record.ID)
	if err != nil {
		record.SetError(err.Error())
		if failed, failErr := o.tracker.Fail(ctx, record.ID, "supersede_resolve_failed", err.Error()); failErr == nil {
			record = failed
		}
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}

	if resolution.RejectNew {
		err := errors.New("orchestrator: too many queued interactions")
		record.SetError(err.Error())
		if failed, failErr := o.tracker.Fail(ctx, record.ID, "queue_full", err.Error()); failErr == nil {
			record = failed
		}
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}

	if resolution.SupersedeTargetID != "" {
		if resolution.SupersedeTargetID != record.ID {
			if err := o.supersedeTarget(ctx, resolution.SupersedeTargetID, record.ID); err != nil {
				record.SetError(err.Error())
				if failed, failErr := o.tracker.Fail(ctx, record.ID, "supersede_failed", err.Error()); failErr == nil {
					record = failed
				}
				return o.buildResult(record, nil, OutcomeFailed, err), err
			}
			if updated, err := o.tracker.UpdateMetadata(ctx, record.ID, InteractionMetadataUpdate{SupersedesID: &resolution.SupersedeTargetID}); err == nil {
				record = updated
			} else {
				if failed, failErr := o.tracker.Fail(ctx, record.ID, "metadata_update_failed", err.Error()); failErr == nil {
					record = failed
				}
				return o.buildResult(record, nil, OutcomeFailed, err), err
			}
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
	if updated, err := o.tracker.UpdateMetadata(ctx, record.ID, metadataFromRuntime(runtime, processCtx)); err == nil {
		record = updated
	} else {
		o.tracker.Fail(ctx, record.ID, "metadata_update_failed", err.Error())
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	req.InteractionID = record.ID
	req.Runtime = &runtime
	if next, err := o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusContextReady); err == nil {
		record = next
	} else {
		o.tracker.Fail(ctx, record.ID, "context_ready_failed", err.Error())
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	if runtime.Safety.Blocked {
		blockErr := ErrOrchestratorSafetyBlocked
		if len(runtime.Safety.Reasons) > 0 {
			blockErr = fmt.Errorf("%w: %s", ErrOrchestratorSafetyBlocked, strings.Join(runtime.Safety.Reasons, ","))
		}
		if failed, failErr := o.tracker.Fail(ctx, record.ID, "safety_blocked", blockErr.Error()); failErr == nil {
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
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			record.CancelReason = err.Error()
			if cancelErr := o.tracker.RequestCancel(ctx, record.ID, err.Error()); cancelErr != nil {
				return o.buildResult(record, nil, OutcomeFailed, cancelErr), cancelErr
			}
			if cancelled, failErr := o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusCancelled); failErr == nil {
				record = cancelled
			}
			return o.buildResult(record, nil, OutcomeCancelled, err), err
		}
		record.SetError(err.Error())
		if failed, failErr := o.tracker.Fail(ctx, record.ID, "processor_failed", err.Error()); failErr == nil {
			record = failed
		}
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}

	if fresh, outcome, freshErr := o.ensureFresh(ctx, record.ID); freshErr != nil {
		record = fresh
		return o.buildResult(record, nil, outcome, freshErr), freshErr
	} else {
		record = fresh
	}
	if updated, err := o.tracker.UpdateMetadata(ctx, record.ID, metadataFromResponse(resp)); err == nil {
		record = updated
	} else {
		log.Printf("[orchestrator] response metadata update failed: %v", err)
		if failed, failErr := o.tracker.Fail(ctx, record.ID, "metadata_update_failed", err.Error()); failErr == nil {
			record = failed
		}
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}

	if next, err := o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusGenerated); err == nil {
		record = next
	} else {
		log.Printf("[orchestrator] transition to generated failed: %v", err)
		if fresh, outcome, freshErr := o.ensureFresh(ctx, record.ID); freshErr != nil {
			return o.buildResult(fresh, nil, outcome, freshErr), freshErr
		}
		if failed, failErr := o.tracker.Fail(ctx, record.ID, "generated_transition_failed", err.Error()); failErr == nil {
			record = failed
		}
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	if fresh, outcome, freshErr := o.ensureFresh(ctx, record.ID); freshErr != nil {
		record = fresh
		return o.buildResult(record, nil, outcome, freshErr), freshErr
	} else {
		record = fresh
	}
	if next, err := o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusCommitted); err == nil {
		record = next
	} else {
		log.Printf("[orchestrator] transition to committed failed: %v", err)
		if fresh, outcome, freshErr := o.ensureFresh(ctx, record.ID); freshErr != nil {
			return o.buildResult(fresh, nil, outcome, freshErr), freshErr
		}
		if failed, failErr := o.tracker.Fail(ctx, record.ID, "commit_transition_failed", err.Error()); failErr == nil {
			record = failed
		}
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	if fresh, outcome, freshErr := o.ensureFresh(ctx, record.ID); freshErr != nil {
		record = fresh
		return o.buildResult(record, nil, outcome, freshErr), freshErr
	} else {
		record = fresh
	}
	if completed, err := o.tracker.Complete(ctx, record.ID, "processor_response"); err == nil {
		record = completed
	} else {
		log.Printf("[orchestrator] transition to completed failed: %v", err)
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}

	events := resp.Events
	if len(events) == 0 {
		events = o.emitOutboxEvents(record, resp, runtime)
	}
	result := o.buildResult(record, resp, OutcomeCompleted, nil)
	result.Duration = duration
	result.Events = events

	return result, nil
}

func (o *Orchestrator) Cancel(interactionID string) error {
	ctx := context.Background()
	rec, ok, err := o.tracker.Get(ctx, interactionID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("orchestrator: interaction not found")
	}
	if rec.IsTerminal() {
		return nil
	}
	if err := o.tracker.RequestCancel(ctx, interactionID, "cancel_requested"); err != nil {
		return err
	}
	o.cancels.Cancel(interactionID)
	_, err = o.tracker.TransitionCAS(ctx, rec.ID, rec.StatusVersion, InteractionStatusCancelled)
	return err
}

func (o *Orchestrator) CancelByScope(scope InteractionScope) int {
	ctx := context.Background()
	active, err := o.tracker.ListActive(ctx, scope)
	if err != nil {
		log.Printf("[orchestrator] list active failed during cancel: %v", err)
		return 0
	}
	count := 0
	for _, rec := range active {
		if err := o.tracker.RequestCancel(ctx, rec.ID, "scope_cancel_requested"); err != nil {
			log.Printf("[orchestrator] request cancel failed: %v", err)
			continue
		}
		if o.cancels.Cancel(rec.ID) {
			count++
		}
		if _, err := o.tracker.TransitionCAS(ctx, rec.ID, rec.StatusVersion, InteractionStatusCancelled); err != nil {
			log.Printf("[orchestrator] cancel transition failed: %v", err)
		}
	}
	return count
}

func (o *Orchestrator) GetTracker() InteractionTracker {
	return o.tracker
}

func (o *Orchestrator) GetOutbox() OutboxStore {
	return o.outbox
}

func (o *Orchestrator) buildScope(req *ProcessRequest) InteractionScope {
	return InteractionScope{
		UserID:         req.UserID,
		CharacterID:    req.CharacterID,
		ConversationID: req.ConversationID,
		Channel:        req.Channel,
		PeerID:         req.PeerID,
		SessionID:      req.SessionID,
		Source:         req.Source,
		RequestID:      req.RequestID,
	}.Normalize()
}

func metadataFromRuntime(runtime RuntimeAssembly, ctx context.Context) InteractionMetadataUpdate {
	path := string(runtime.Path)
	priority := priorityForPath(runtime.Path)
	update := InteractionMetadataUpdate{
		PathType: &path,
		Priority: &priority,
	}
	if deadline, ok := ctx.Deadline(); ok {
		update.DeadlineAt = &deadline
	}
	return update
}

func metadataFromResponse(resp *ProcessResponse) InteractionMetadataUpdate {
	commitID := ""
	if resp != nil {
		if len(resp.MessageIDs) > 0 {
			commitID = strings.Join(resp.MessageIDs, ",")
		} else {
			commitID = resp.RequestID
		}
	}
	return InteractionMetadataUpdate{CommitID: &commitID}
}

func priorityForPath(path PathType) int {
	switch path {
	case PathTypeDeep:
		return 1
	case PathTypeStandard:
		return 2
	default:
		return 3
	}
}

func (o *Orchestrator) supersedeTarget(ctx context.Context, targetID, newID string) error {
	if err := o.tracker.MarkSuperseded(ctx, targetID, newID); err != nil {
		return err
	}
	o.cancels.Cancel(targetID)
	return nil
}

func (o *Orchestrator) ensureFresh(ctx context.Context, id string) (*InteractionRecord, Outcome, error) {
	rec, ok, err := o.tracker.Get(ctx, id)
	if err != nil {
		return nil, OutcomeFailed, err
	}
	if !ok {
		return nil, OutcomeFailed, ErrInteractionNotFound
	}
	switch rec.Status {
	case InteractionStatusCancelled:
		return rec, OutcomeCancelled, ErrOrchestratorCancelled
	case InteractionStatusSuperseded:
		return rec, OutcomeSuperseded, ErrOrchestratorSuperseded
	}
	if !rec.CancelRequestedAt.IsZero() {
		cancelled, err := o.tracker.TransitionCAS(ctx, rec.ID, rec.StatusVersion, InteractionStatusCancelled)
		if err != nil {
			return rec, OutcomeFailed, err
		}
		return cancelled, OutcomeCancelled, ErrOrchestratorCancelled
	}
	return rec, OutcomeCompleted, nil
}

func (o *Orchestrator) buildResult(record *InteractionRecord, resp *ProcessResponse, outcome Outcome, err error) *OrchestrationResult {
	r := &OrchestrationResult{Outcome: outcome, Response: resp}
	if record != nil {
		r.InteractionID = record.ID
	}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

func outcomeForRecord(record *InteractionRecord) Outcome {
	if record == nil {
		return OutcomeFailed
	}
	switch record.Status {
	case InteractionStatusCompleted, InteractionStatusCommitted, InteractionStatusDeliveryPending, InteractionStatusDelivered:
		return OutcomeCompleted
	case InteractionStatusCancelled:
		return OutcomeCancelled
	case InteractionStatusSuperseded:
		return OutcomeSuperseded
	default:
		return OutcomeFailed
	}
}

func (o *Orchestrator) emitOutboxEvents(record *InteractionRecord, resp *ProcessResponse, runtime RuntimeAssembly) []OutboxRecord {
	now := time.Now()
	events := make([]OutboxRecord, 0, 3)

	msgPayload, _ := json.Marshal(map[string]interface{}{
		"interactionId": record.ID,
		"scope":         record.Scope,
		"reply":         resp.Reply,
		"messageIds":    resp.MessageIDs,
	})
	msgEvent := OutboxRecord{
		ID:          uuid.New().String(),
		AggregateID: record.ID,
		EventType:   "interaction.completed",
		Payload:     msgPayload,
		Status:      OutboxStatusPending,
		MaxRetries:  DefaultMaxRetries,
		CreatedAt:   now,
		NextRetryAt: now,
	}
	o.outbox.Append(&msgEvent)
	events = append(events, msgEvent)

	statePayload, _ := json.Marshal(map[string]interface{}{
		"interactionId":  record.ID,
		"conversationId": record.Scope.ConversationID,
		"characterId":    record.Scope.CharacterID,
		"channel":        record.Scope.Channel,
		"status":         "completed",
		"timestamp":      now,
	})
	stateEvent := OutboxRecord{
		ID:          uuid.New().String(),
		AggregateID: record.ID,
		EventType:   "interaction.state_changed",
		Payload:     statePayload,
		Status:      OutboxStatusPending,
		MaxRetries:  DefaultMaxRetries,
		CreatedAt:   now,
		NextRetryAt: now,
	}
	o.outbox.Append(&stateEvent)
	events = append(events, stateEvent)

	runtimePayload, _ := json.Marshal(map[string]interface{}{
		"interactionId": record.ID,
		"scope":         record.Scope,
		"path":          runtime.Path,
		"safety":        runtime.Safety,
		"delivery":      runtime.Delivery,
		"timestamp":     now,
	})
	runtimeEvent := OutboxRecord{
		ID:          uuid.New().String(),
		AggregateID: record.ID,
		EventType:   "interaction.runtime_assembled",
		Payload:     runtimePayload,
		Status:      OutboxStatusPending,
		MaxRetries:  DefaultMaxRetries,
		CreatedAt:   now,
		NextRetryAt: now,
	}
	o.outbox.Append(&runtimeEvent)
	events = append(events, runtimeEvent)

	return events
}
func (o *Orchestrator) SetDeadlineProvider(fn func(ctx context.Context, requestID string) (context.Context, context.CancelFunc)) {
	o.deadlineFn = fn
}

func (o *Orchestrator) SetRuntimePipeline(pipeline *RuntimePipeline) {
	if pipeline == nil {
		pipeline = NewRuntimePipeline(nil, nil, nil)
	}
	o.pipeline = pipeline
}
