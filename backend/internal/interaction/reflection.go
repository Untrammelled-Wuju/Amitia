package interaction

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/outbox"
)

type ReflectionProcessInput struct {
	Scope        InteractionScope
	Plan         *decision.BehaviorPlan
	Observation  *decision.Observation
	GoalProgress decision.GoalProgressBatchResult
	Continuation decision.ContinuationDecision
	Now          time.Time
}

type ReflectionProcessResult struct {
	Triggered     bool
	TriggerKinds  []mindruntime.ReflectionTriggerKind
	CandidateID   string
	Significant   bool
	Approved      bool
	Escalated     bool
	OutboxEventID string
}

type ReflectionProcessor interface {
	ProcessReflection(
		ctx context.Context,
		input ReflectionProcessInput,
	) (ReflectionProcessResult, error)
}

type ReflectionOutbox interface {
	Append(record outbox.OutboxRecord) error
}

type reflectionScopeState struct {
	Trigger          mindruntime.ReflectionTriggerState
	Evidence         *ReflectionEvidenceWindow
	SeenObservations *boundedIDSet
	LastCandidateID  string
}

type ReflectionService struct {
	mu               sync.Mutex
	triggerConfig    mindruntime.ReflectionTriggerConfig
	runConfig        mindruntime.ReflectionRunConfig
	approvalConfig   mindruntime.ReflectionApprovalConfig
	supervisorConfig mindruntime.SupervisorConfig
	states           map[string]*reflectionScopeState
	outbox           ReflectionOutbox
	evidenceReader   ReflectionEvidenceReader
	selector         *ReflectionEvidenceSelector
	now              func() time.Time
}

type ReflectionServiceOption func(*ReflectionService)

func WithReflectionTriggerConfig(cfg mindruntime.ReflectionTriggerConfig) ReflectionServiceOption {
	return func(s *ReflectionService) { s.triggerConfig = cfg }
}

func WithReflectionRunConfig(cfg mindruntime.ReflectionRunConfig) ReflectionServiceOption {
	return func(s *ReflectionService) { s.runConfig = cfg }
}

func WithReflectionApprovalConfig(cfg mindruntime.ReflectionApprovalConfig) ReflectionServiceOption {
	return func(s *ReflectionService) { s.approvalConfig = cfg }
}

func WithReflectionSupervisorConfig(cfg mindruntime.SupervisorConfig) ReflectionServiceOption {
	return func(s *ReflectionService) { s.supervisorConfig = cfg }
}

func WithReflectionOutbox(ob ReflectionOutbox) ReflectionServiceOption {
	return func(s *ReflectionService) { s.outbox = ob }
}

func WithReflectionEvidenceReader(r ReflectionEvidenceReader) ReflectionServiceOption {
	return func(s *ReflectionService) { s.evidenceReader = r }
}

func WithReflectionNowFunc(now func() time.Time) ReflectionServiceOption {
	return func(s *ReflectionService) { s.now = now }
}

func NewReflectionService(opts ...ReflectionServiceOption) *ReflectionService {
	s := &ReflectionService{
		triggerConfig:    mindruntime.DefaultReflectionTriggerConfig(),
		runConfig:        mindruntime.DefaultReflectionRunConfig(),
		approvalConfig:   mindruntime.DefaultReflectionApprovalConfig(),
		supervisorConfig: mindruntime.DefaultSupervisorConfig(),
		states:           make(map[string]*reflectionScopeState),
		selector:         NewReflectionEvidenceSelector(),
		now:              func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *ReflectionService) ProcessReflection(
	ctx context.Context,
	input ReflectionProcessInput,
) (ReflectionProcessResult, error) {
	result := ReflectionProcessResult{}

	if ctx == nil || ctx.Err() != nil {
		return result, nil
	}
	if input.Now.IsZero() {
		input.Now = s.now()
	}
	if strings.TrimSpace(input.Scope.CharacterID) == "" || strings.TrimSpace(input.Observation.ID) == "" {
		return result, nil
	}

	scopeKey := NewReflectionScopeKey(input.Scope).Normalize()

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.getOrCreateScopeState(scopeKey)

	if !state.SeenObservations.Add(input.Observation.ID) {
		return result, nil
	}

	evt, ok := ObservationToVerifiedEvent(*input.Observation, input.GoalProgress, input.Continuation)
	if !ok {
		return result, nil
	}
	state.Evidence.AddEvents(evt)

	triggerResult := mindruntime.EvaluateTrigger(
		state.Trigger,
		s.triggerConfig,
		input.Now,
		1,
		0,
		nil,
	)
	state.Trigger = triggerResult.State

	if !triggerResult.Fired {
		result.TriggerKinds = triggerResult.Kinds
		return result, nil
	}

	result.Triggered = true
	result.TriggerKinds = triggerResult.Kinds

	externalEvidence := s.loadExternalEvidence(ctx, scopeKey, input.Now)
	combinedWindow := s.combineEvidence(state, externalEvidence)
	selected := s.selector.Select(combinedWindow)

	evidenceCount := len(selected.Events) + len(selected.Relations) + len(selected.Memories)
	if evidenceCount == 0 {
		state.Trigger = mindruntime.ResetTriggerStateAt(input.Now)
		return result, nil
	}

	candidate := mindruntime.RunReflection(
		scopeKey.CharacterID,
		selected,
		s.runConfig,
		input.Now,
	)
	state.LastCandidateID = candidate.ID

	result.CandidateID = candidate.ID
	result.Significant = mindruntime.IsReflectionCandidateSignificant(candidate, s.runConfig)

	if !result.Significant {
		state.Trigger = mindruntime.ResetTriggerStateAt(input.Now)
		return result, nil
	}

	supervisor := s.getOrCreateSupervisor(scopeKey.CharacterID)
	approvalResult := supervisor.ApproveReflectionCandidateAt(candidate, evidenceCount, input.Now)

	result.Approved = approvalResult.Approved
	result.Escalated = approvalResult.Escalated

	if !approvalResult.Approved || approvalResult.Escalated {
		return result, nil
	}

	outboxID, err := s.appendApprovedEvent(candidate, scopeKey, input, evidenceCount)
	if err != nil {
		log.Printf("reflection: failed to append approved event: scope=%s candidate=%s err=%v", scopeKey.String(), candidate.ID, err)
		return result, fmt.Errorf("reflection outbox append failed: %w", err)
	}

	result.OutboxEventID = outboxID
	state.Trigger = mindruntime.ResetTriggerStateAt(input.Now)

	return result, nil
}

func (s *ReflectionService) getOrCreateScopeState(scopeKey ReflectionScopeKey) *reflectionScopeState {
	key := scopeKey.String()
	if st, ok := s.states[key]; ok {
		return st
	}
	st := &reflectionScopeState{
		Trigger:          mindruntime.ReflectionTriggerState{},
		Evidence:         NewReflectionEvidenceWindow(),
		SeenObservations: newBoundedIDSet(MaxSeenObservation),
	}
	s.states[key] = st
	return st
}

var reflectionSupervisors sync.Map

func (s *ReflectionService) getOrCreateSupervisor(characterID string) *mindruntime.ReflectionSupervisor {
	trimmed := strings.TrimSpace(characterID)
	if v, ok := reflectionSupervisors.Load(trimmed); ok {
		return v.(*mindruntime.ReflectionSupervisor)
	}
	sv := mindruntime.NewReflectionSupervisor(trimmed, s.approvalConfig, s.supervisorConfig)
	reflectionSupervisors.Store(trimmed, &sv)
	if v, ok := reflectionSupervisors.Load(trimmed); ok {
		return v.(*mindruntime.ReflectionSupervisor)
	}
	return &sv
}

func (s *ReflectionService) loadExternalEvidence(
	ctx context.Context,
	scopeKey ReflectionScopeKey,
	now time.Time,
) ReflectionExternalEvidence {
	if s.evidenceReader == nil {
		return ReflectionExternalEvidence{}
	}
	ext, err := s.evidenceReader.LoadVerifiedEvidence(ctx, scopeKey, now, MaxEvidenceMemories)
	if err != nil {
		log.Printf("reflection: external evidence reader error: scope=%s err=%v", scopeKey.String(), err)
		return ReflectionExternalEvidence{}
	}
	return ext
}

func (s *ReflectionService) combineEvidence(
	state *reflectionScopeState,
	external ReflectionExternalEvidence,
) *ReflectionEvidenceWindow {
	combined := NewReflectionEvidenceWindow()
	allEvents := make([]mindruntime.VerifiedEvent, 0, len(state.Evidence.Events)+8)
	allEvents = append(allEvents, state.Evidence.Events...)
	combined.AddEvents(allEvents...)
	combined.AddRelations(state.Evidence.Relations...)
	combined.AddRelations(external.Relations...)
	combined.AddMemories(state.Evidence.Memories...)
	combined.AddMemories(external.Memories...)
	return combined
}

func (s *ReflectionService) appendApprovedEvent(
	candidate mindruntime.ReflectionCandidate,
	scopeKey ReflectionScopeKey,
	input ReflectionProcessInput,
	evidenceCount int,
) (string, error) {
	if s.outbox == nil {
		return "", fmt.Errorf("reflection outbox not configured")
	}
	payload := reflectionCandidateApprovedPayload{
		CandidateID:        candidate.ID,
		CharacterID:        scopeKey.CharacterID,
		ConversationID:     scopeKey.ConversationID,
		RequestID:          input.Scope.RequestID,
		Confidence:         candidate.Confidence,
		MemoryAbstractions: make([]reflectionMemoryAbstractionPayload, 0, len(candidate.MemoryAbstractions)),
	}
	for _, ab := range candidate.MemoryAbstractions {
		payload.MemoryAbstractions = append(payload.MemoryAbstractions, reflectionMemoryAbstractionPayload{
			SourceIDs: ab.SourceIDs,
			Topic:     ab.Topic,
			Abstract:  ab.Abstract,
		})
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	eventID := "reflection-event:" + candidate.ID
	idempotencyKey := "reflection:candidate:" + candidate.ID

	record := outbox.OutboxRecord{
		ID:             eventID,
		AggregateID:    scopeKey.CharacterID,
		EventType:      ReflectionCandidateApprovedEventType,
		Payload:        payloadBytes,
		Status:         outbox.OutboxStatusPending,
		AvailableAt:    input.Now,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      input.Now,
	}

	if err := s.outbox.Append(record); err != nil {
		return "", err
	}
	return eventID, nil
}

type reflectionCandidateApprovedPayload struct {
	CandidateID        string                               `json:"candidateId"`
	CharacterID        string                               `json:"characterId"`
	ConversationID     string                               `json:"conversationId"`
	RequestID          string                               `json:"requestId"`
	Confidence         float64                              `json:"confidence"`
	MemoryAbstractions []reflectionMemoryAbstractionPayload `json:"memoryAbstractions"`
}

type reflectionMemoryAbstractionPayload struct {
	SourceIDs []string `json:"sourceIds"`
	Topic     string   `json:"topic"`
	Abstract  string   `json:"abstract"`
}
