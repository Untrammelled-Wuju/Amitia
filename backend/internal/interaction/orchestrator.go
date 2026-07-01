package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOrchestratorNotReady     = errors.New("orchestrator: not ready")
	ErrOrchestratorBusy         = errors.New("orchestrator: too many concurrent interactions")
	ErrOrchestratorCancelled    = errors.New("orchestrator: cancelled")
	ErrOrchestratorInvalidScope = errors.New("orchestrator: invalid scope")
)

type ProcessRequest struct {
	CharacterID    string  `json:"characterId,omitempty"`
	Message        string  `json:"message"`
	ConversationID string  `json:"conversationId,omitempty"`
	Channel        string  `json:"channel,omitempty"`
	Source         string  `json:"source,omitempty"`
	PeerID         string  `json:"peerId,omitempty"`
	UserID         string  `json:"userId,omitempty"`
	AudioUrl       string  `json:"audioUrl,omitempty"`
	AudioDuration  float64 `json:"audioDuration,omitempty"`
	VoiceMessage   bool    `json:"voiceMessage"`
	ImageUrl       string  `json:"imageUrl,omitempty"`
	VideoUrl       string  `json:"videoUrl,omitempty"`
	RequestID      string  `json:"requestId,omitempty"`
}

type ProcessResponse struct {
	ConversationID string   `json:"conversationId"`
	Reply          string   `json:"reply"`
	CharacterID    string   `json:"characterId"`
	CharacterName  string   `json:"characterName"`
	MessageIDs     []string `json:"messageIds"`
	ForceVoice     bool     `json:"forceVoice"`
	AudioUrls      []string `json:"audioUrls"`
	RequestID      string   `json:"requestId"`
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
	InteractionID string             `json:"interactionId"`
	Outcome       Outcome            `json:"outcome"`
	Response      *ProcessResponse   `json:"response,omitempty"`
	Error         string             `json:"error,omitempty"`
	Duration      time.Duration      `json:"duration"`
	Events        []OutboxRecord     `json:"events,omitempty"`
}

type OrchestratorConfig struct {
	MaxConcurrent int
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
	cfg       OrchestratorConfig
	processor MessageProcessor
	tracker   InteractionTracker
	outbox    OutboxStore
	resolver  *SupersedeResolver
	mu        sync.Mutex
	active    int
	ready     bool
}

func NewOrchestrator(cfg OrchestratorConfig, processor MessageProcessor) *Orchestrator {
	tracker := NewInMemoryTracker()
	return &Orchestrator{
		cfg:       cfg,
		processor: processor,
		tracker:   tracker,
		outbox:    NewInMemoryOutboxStore(),
		resolver:  NewSupersedeResolver(cfg.SupersedePolicy, tracker),
		ready:     true,
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

	scope := o.buildScope(req)
	if err := scope.Validate(); err != nil {
		return nil, err
	}

	record := NewInteractionRecord(scope)
	o.tracker.Track(record)

	resolution, err := o.resolver.Resolve(scope)
	if err != nil {
		record.Transition(InteractionStatusFailed)
		record.SetError(err.Error())
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}

	if resolution.RejectNew {
		record.Transition(InteractionStatusFailed)
		err := errors.New("orchestrator: too many queued interactions")
		record.SetError(err.Error())
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}

	if resolution.SupersedeTargetID != "" {
		o.supersedeTarget(resolution.SupersedeTargetID, record.ID)
		record.SupersedesID = resolution.SupersedeTargetID
	}

	if err := record.Transition(InteractionStatusProcessing); err != nil {
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}

	processCtx, cancel := context.WithTimeout(ctx, o.cfg.DefaultTimeout)
	defer cancel()
	record.SetCancel(cancel)

	start := time.Now()
	resp, err := o.processor.ProcessMessageCtx(processCtx, req)
	duration := time.Since(start)

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			record.Transition(InteractionStatusCancelled)
			record.CancelReason = err.Error()
			return o.buildResult(record, nil, OutcomeCancelled, err), err
		}
		record.Transition(InteractionStatusFailed)
		record.SetError(err.Error())
		return o.buildResult(record, nil, OutcomeFailed, err), err
	}

	if err := record.Transition(InteractionStatusCompleted); err != nil {
		log.Printf("[orchestrator] transition to completed failed: %v", err)
	}

	events := o.emitOutboxEvents(record, resp)
	result := o.buildResult(record, resp, OutcomeCompleted, nil)
	result.Duration = duration
	result.Events = events

	return result, nil
}

func (o *Orchestrator) Cancel(interactionID string) error {
	rec, ok := o.tracker.Get(interactionID)
	if !ok {
		return errors.New("orchestrator: interaction not found")
	}
	if rec.IsTerminal() {
		return nil
	}
	rec.Cancel()
	rec.Transition(InteractionStatusCancelled)
	return nil
}

func (o *Orchestrator) CancelByScope(scope InteractionScope) int {
	active := o.tracker.GetActiveByScope(scope)
	count := 0
	for _, rec := range active {
		if rec.Cancel() {
			count++
		}
		rec.Transition(InteractionStatusCancelled)
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
		Source:         req.Source,
		RequestID:      req.RequestID,
	}.Normalize()
}

func (o *Orchestrator) supersedeTarget(targetID, newID string) {
	rec, ok := o.tracker.Get(targetID)
	if !ok {
		return
	}
	rec.Cancel()
	rec.Transition(InteractionStatusSuperseded)
	rec.SetSupersededBy(newID)
}

func (o *Orchestrator) buildResult(record *InteractionRecord, resp *ProcessResponse, outcome Outcome, err error) *OrchestrationResult {
	r := &OrchestrationResult{
		InteractionID: record.ID,
		Outcome:       outcome,
		Response:      resp,
	}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

func (o *Orchestrator) emitOutboxEvents(record *InteractionRecord, resp *ProcessResponse) []OutboxRecord {
	now := time.Now()
	events := make([]OutboxRecord, 0, 2)

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

	return events
}
