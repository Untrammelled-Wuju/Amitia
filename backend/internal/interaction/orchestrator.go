package interaction

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOrchestratorProcessing    = errors.New("orchestrator: request still processing")
	ErrOrchestratorNotReady      = errors.New("orchestrator: not ready")
	ErrOrchestratorBusy          = errors.New("orchestrator: too many concurrent interactions")
	ErrOrchestratorCancelled     = errors.New("orchestrator: cancelled")
	ErrOrchestratorSuperseded    = errors.New("orchestrator: superseded")
	ErrOrchestratorDuplicate     = errors.New("orchestrator: duplicate request")
	ErrOrchestratorInvalidScope  = errors.New("orchestrator: invalid scope")
	ErrOrchestratorSafetyBlocked = errors.New("orchestrator: safety blocked")
)

type ProcessRequest struct {
	CharacterID           string           `json:"characterId,omitempty"`
	Message               string           `json:"message"`
	ConversationID        string           `json:"conversationId,omitempty"`
	Channel               string           `json:"channel,omitempty"`
	Source                string           `json:"source,omitempty"`
	ProactiveTimeContext  string           `json:"-"`
	ProactiveRecentContext string          `json:"-"`
	ProactiveRelationship  string           `json:"-"`
	ProactiveEmotion      string           `json:"-"`
	ProactiveMemory       string           `json:"-"`
	PeerID                string           `json:"peerId,omitempty"`
	UserID                string           `json:"userId,omitempty"`
	SessionID             string           `json:"sessionId,omitempty"`
	AudioUrl              string           `json:"audioUrl,omitempty"`
	AudioDuration         float64          `json:"audioDuration,omitempty"`
	VoiceMessage          bool             `json:"voiceMessage"`
	ExpressionPlan        *ExpressionPlan  `json:"expressionPlan,omitempty"`
	ImageUrl              string           `json:"imageUrl,omitempty"`
	VideoUrl              string           `json:"videoUrl,omitempty"`
	ImageContext          string           `json:"imageContext,omitempty"`
	RequestID             string           `json:"requestId,omitempty"`
	InteractionID         string           `json:"-"`
	ExpectedStatusVersion int64            `json:"-"`
	Runtime               *RuntimeAssembly `json:"-"`
	IsInternal            bool             `json:"-"`
}

type ProcessResponse struct {
	ConversationID string         `json:"conversationId"`
	Sequence       int64          `json:"sequence"`
	Reply          string         `json:"reply"`
	Lines          []string       `json:"lines"`
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
	OutcomeCompleted       Outcome = "completed"
	OutcomeFailed          Outcome = "failed"
	OutcomeCancelled       Outcome = "cancelled"
	OutcomeSuperseded      Outcome = "superseded"
	OutcomeDeliveryUnknown Outcome = "delivery_unknown"
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
	queueMu    sync.Mutex
	queueLocks map[string]*sync.Mutex
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
		cfg:        cfg,
		processor:  processor,
		tracker:    tracker,
		outbox:     outbox,
		resolver:   NewSupersedeResolver(cfg.SupersedePolicy, tracker),
		pipeline:   NewRuntimePipeline(nil, nil, nil),
		cancels:    NewCancellationRegistry(),
		queueLocks: map[string]*sync.Mutex{},
		ready:      false,
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

	if resolution.SupersedeTargetID != "" {
		if resolution.SupersedeTargetID != record.ID {
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
	}

	if resolution.Enqueue {
		record, err = o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusQueued)
		if err != nil {
			return o.buildResult(record, nil, OutcomeFailed, err), err
		}
	}

	var releaseQueue func()
	if o.cfg.SupersedePolicy == SupersedePolicyQueue {
		releaseQueue = o.acquireQueueScope(scope)
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
	req.InteractionID = record.ID
	req.Runtime = &runtime
	if next, err := o.tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusContextReady); err == nil {
		record = next
	} else {
		o.tracker.Fail(ctx, record.ID, record.StatusVersion, "context_ready_failed", err.Error())
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}
	req.ExpectedStatusVersion = record.StatusVersion
	if runtime.Safety.Blocked {
		blockErr := ErrOrchestratorSafetyBlocked
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
	if record.Status == InteractionStatusCommitted || record.Status == InteractionStatusCompleted {
		resp.RequestID = req.RequestID
		resp.ConversationID = req.ConversationID
		resp.CharacterID = req.CharacterID
		result := o.buildResult(record, resp, OutcomeCompleted, nil)
		result.Duration = duration
		result.Events = resp.Events
		return result, nil
	}
	return nil, fmt.Errorf("orchestrator: processor did not commit interaction status=%s", record.Status)
}

func (o *Orchestrator) handleProcessorError(ctx context.Context, record *InteractionRecord, req *ProcessRequest, resp *ProcessResponse, duration time.Duration, procErr error) (*OrchestrationResult, error) {
	if fresh, ok, getErr := o.tracker.Get(ctx, record.ID); getErr == nil && ok {
		record = fresh
	}
	if record.Status == InteractionStatusCommitted || record.Status == InteractionStatusCompleted {
		log.Printf("[orchestrator] interaction %s committed despite processor error: %v", record.ID, procErr)
		if resp != nil {
			resp.RequestID = req.RequestID
			resp.ConversationID = req.ConversationID
			resp.CharacterID = req.CharacterID
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

func (o *Orchestrator) Cancel(interactionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
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
	if rec.Status == InteractionStatusCommitted || rec.Status == InteractionStatusDeliveryPending || rec.Status == InteractionStatusDelivered {
		return o.writeCompensationEvent(ctx, rec, "cancel_after_commit")
	}
	if err := o.tracker.RequestCancel(ctx, interactionID, "cancel_requested"); err != nil {
		return o.resolveCancelConflict(ctx, interactionID, err)
	}
	o.cancels.Cancel(interactionID)
	fresh, ok, err := o.tracker.Get(ctx, interactionID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("orchestrator: interaction not found")
	}
	_, err = o.tracker.TransitionCAS(ctx, fresh.ID, fresh.StatusVersion, InteractionStatusCancelled)
	if err != nil {
		return o.resolveCancelConflict(ctx, interactionID, err)
	}
	return nil
}

func (o *Orchestrator) CancelByScope(scope InteractionScope) int {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
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
		fresh, ok, err := o.tracker.Get(ctx, rec.ID)
		if err != nil {
			log.Printf("[orchestrator] reload after cancel request failed: %v", err)
			continue
		}
		if !ok {
			log.Printf("[orchestrator] cancelled record disappeared: %s", rec.ID)
			continue
		}
		if _, err := o.tracker.TransitionCAS(ctx, fresh.ID, fresh.StatusVersion, InteractionStatusCancelled); err != nil {
			if resolveErr := o.resolveCancelConflict(ctx, rec.ID, err); resolveErr != nil {
				log.Printf("[orchestrator] cancel transition failed: %v", resolveErr)
			}
		}
	}
	return count
}

func (o *Orchestrator) writeCompensationEvent(ctx context.Context, rec *InteractionRecord, reason string) error {
	log.Printf("[orchestrator] compensation event for committed interaction %s: %s (compensation outbox deferred)", rec.ID, reason)
	return nil
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
	if executorID := strings.TrimSpace(runtime.ExecutorID); executorID != "" {
		update.ExecutorID = &executorID
	}
	if deadline, ok := ctx.Deadline(); ok {
		update.DeadlineAt = &deadline
	}
	return update
}

func executorIDForProcessor(processor MessageProcessor) string {
	if processor == nil {
		return ""
	}
	t := reflect.TypeOf(processor)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.PkgPath() == "" || t.Name() == "" {
		return t.String()
	}
	return t.PkgPath() + "." + t.Name()
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
	rec, ok, err := o.tracker.Get(ctx, targetID)
	if err != nil {
		return err
	}
	if ok && (rec.Status == InteractionStatusCommitted || rec.Status == InteractionStatusDeliveryPending || rec.Status == InteractionStatusDelivered) {
		return o.writeCompensationEvent(ctx, rec, "supersede_after_commit:"+newID)
	}
	if err := o.tracker.MarkSuperseded(ctx, targetID, newID); err != nil {
		return err
	}
	o.cancels.Cancel(targetID)
	return nil
}

func (o *Orchestrator) waitForQueueTurn(ctx context.Context, scope InteractionScope, record *InteractionRecord) (*InteractionRecord, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		fresh, _, err := o.ensureFresh(ctx, record.ID)
		if err != nil {
			return fresh, err
		}
		record = fresh
		older, err := o.hasOlderActive(ctx, scope, record)
		if err != nil {
			return record, err
		}
		if !older {
			return record, nil
		}
		select {
		case <-ctx.Done():
			return record, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (o *Orchestrator) hasOlderActive(ctx context.Context, scope InteractionScope, record *InteractionRecord) (bool, error) {
	active, err := o.tracker.ListActive(ctx, scope)
	if err != nil {
		return false, err
	}
	for _, rec := range active {
		if rec.ID == record.ID {
			continue
		}
		if !sameSupersedeScope(scope, rec.Scope) {
			continue
		}
		if rec.CreatedAt.Before(record.CreatedAt) || (rec.CreatedAt.Equal(record.CreatedAt) && rec.ID < record.ID) {
			return true, nil
		}
	}
	return false, nil
}

func (o *Orchestrator) acquireQueueScope(scope InteractionScope) func() {
	key := queueScopeKey(scope)
	o.queueMu.Lock()
	lock := o.queueLocks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		o.queueLocks[key] = lock
	}
	o.queueMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func queueScopeKey(scope InteractionScope) string {
	scope = scope.Normalize()
	return strings.Join([]string{
		scope.UserID,
		scope.CharacterID,
		scope.ConversationID,
		scope.Channel,
		scope.PeerID,
	}, "\x00")
}

func outcomeFromQueueWaitError(err error) Outcome {
	switch {
	case errors.Is(err, ErrOrchestratorCancelled), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return OutcomeCancelled
	case errors.Is(err, ErrOrchestratorSuperseded):
		return OutcomeSuperseded
	default:
		return OutcomeFailed
	}
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

func (o *Orchestrator) ensureFreshAtVersion(ctx context.Context, id string, expectedVersion int64) (*InteractionRecord, Outcome, error) {
	rec, outcome, err := o.ensureFresh(ctx, id)
	if err != nil {
		return rec, outcome, err
	}
	if rec.StatusVersion != expectedVersion {
		return rec, OutcomeFailed, ErrVersionConflict
	}
	if err := ctx.Err(); err != nil {
		return rec, OutcomeCancelled, fmt.Errorf("context expired before version check: %w", err)
	}
	return rec, outcome, nil
}

func (o *Orchestrator) resolveCompletionConflict(ctx context.Context, id string, fallback *InteractionRecord, err error) (*InteractionRecord, Outcome, error) {
	if !isInteractionConflictError(err) {
		return fallback, OutcomeFailed, err
	}
	rec, ok, getErr := o.tracker.Get(ctx, id)
	if getErr != nil {
		return fallback, OutcomeFailed, getErr
	}
	if !ok {
		return fallback, OutcomeFailed, ErrInteractionNotFound
	}
	switch rec.Status {
	case InteractionStatusCancelled:
		return rec, OutcomeCancelled, ErrOrchestratorCancelled
	case InteractionStatusSuperseded:
		return rec, OutcomeSuperseded, ErrOrchestratorSuperseded
	case InteractionStatusCompleted:
		return rec, OutcomeCompleted, nil
	}
	if !rec.CancelRequestedAt.IsZero() {
		cancelled, cancelErr := o.tracker.TransitionCAS(ctx, rec.ID, rec.StatusVersion, InteractionStatusCancelled)
		if cancelErr == nil {
			return cancelled, OutcomeCancelled, ErrOrchestratorCancelled
		}
		if isInteractionConflictError(cancelErr) {
			return o.resolveCompletionConflict(ctx, id, rec, cancelErr)
		}
		return rec, OutcomeFailed, cancelErr
	}
	return rec, OutcomeFailed, err
}

func (o *Orchestrator) resolveCancelConflict(ctx context.Context, id string, err error) error {
	if !isInteractionConflictError(err) {
		return err
	}
	rec, ok, getErr := o.tracker.Get(ctx, id)
	if getErr != nil {
		return getErr
	}
	if !ok {
		return ErrInteractionNotFound
	}
	if rec.IsTerminal() || !canSupersedeStatus(rec.Status) {
		return nil
	}
	return err
}

func isInteractionConflictError(err error) bool {
	return errors.Is(err, ErrVersionConflict) ||
		errors.Is(err, ErrAlreadyTerminal) ||
		errors.Is(err, ErrInvalidTransition)
}

func (o *Orchestrator) handleIdempotentHit(existing *InteractionRecord) (*OrchestrationResult, error) {
	outcome := outcomeForRecord(existing)
	switch existing.Status {
	case InteractionStatusCompleted, InteractionStatusCommitted, InteractionStatusDeliveryPending, InteractionStatusDelivered:
		var resp *ProcessResponse
		if existing.ResultRef != "" {
			resp = &ProcessResponse{
				RequestID:      existing.Scope.RequestID,
				ConversationID: existing.Scope.ConversationID,
				CharacterID:    existing.Scope.CharacterID,
				Reply:          existing.ResultRef,
			}
		}
		return o.buildResult(existing, resp, outcome, nil), nil
	case InteractionStatusReceived, InteractionStatusNormalized, InteractionStatusQueued, InteractionStatusProcessing, InteractionStatusContextReady, InteractionStatusDecided, InteractionStatusGenerated:
		return o.buildResult(existing, nil, outcome, nil), ErrOrchestratorProcessing
	case InteractionStatusFailed:
		return o.buildResult(existing, nil, outcome, nil), ErrOrchestratorDuplicate
	case InteractionStatusCancelled, InteractionStatusSuperseded:
		return o.buildResult(existing, nil, outcome, nil), ErrOrchestratorDuplicate
	default:
		return o.buildResult(existing, nil, outcome, nil), ErrOrchestratorDuplicate
	}
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
	case InteractionStatusCompleted:
		return OutcomeCompleted
	case InteractionStatusCommitted, InteractionStatusDeliveryPending, InteractionStatusDelivered:
		return OutcomeDeliveryUnknown
	case InteractionStatusCancelled:
		return OutcomeCancelled
	case InteractionStatusSuperseded:
		return OutcomeSuperseded
	default:
		return OutcomeFailed
	}
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
