package interaction

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"time"
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
	CharacterID              string           `json:"characterId,omitempty"`
	Message                  string           `json:"message"`
	ConversationID           string           `json:"conversationId,omitempty"`
	Channel                  string           `json:"channel,omitempty"`
	Source                   string           `json:"source,omitempty"`
	ProactiveTaskInstruction string           `json:"-"`
	ProactiveTimeContext     string           `json:"-"`
	ProactiveRecentContext   string           `json:"-"`
	ProactiveRelationship    string           `json:"-"`
	ProactiveEmotion         string           `json:"-"`
	ProactiveMemory          string           `json:"-"`
	PeerID                   string           `json:"peerId,omitempty"`
	UserID                   string           `json:"userId,omitempty"`
	SessionID                string           `json:"sessionId,omitempty"`
	AudioUrl                 string           `json:"audioUrl,omitempty"`
	AudioDuration            float64          `json:"audioDuration,omitempty"`
	VoiceMessage             bool             `json:"voiceMessage"`
	ExpressionPlan           *ExpressionPlan  `json:"expressionPlan,omitempty"`
	ImageUrl                 string           `json:"imageUrl,omitempty"`
	VideoUrl                 string           `json:"videoUrl,omitempty"`
	ImageContext             string           `json:"imageContext,omitempty"`
	ReplyToMessageID         *string          `json:"replyToMessageId,omitempty"`
	RequestID                string           `json:"requestId,omitempty"`
	InteractionID            string           `json:"-"`
	ExpectedStatusVersion    int64            `json:"-"`
	Runtime                  *RuntimeAssembly `json:"-"`
	IsInternal               bool             `json:"-"`
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
	MessagePlan    *MessagePlan   `json:"messagePlan,omitempty"`
	Events         []OutboxRecord `json:"-"`
}

type MessagePlan struct {
	ResponseGroupID string            `json:"responseGroupId"`
	Managed         bool              `json:"managed"`
	Items           []MessagePlanItem `json:"items"`
}

type MessagePlanItem struct {
	MessageID              string `json:"messageId"`
	Sequence               int    `json:"sequence"`
	Type                   string `json:"type"`
	Content                string `json:"content,omitempty"`
	EmoteID                string `json:"emoteId,omitempty"`
	AltText                string `json:"altText,omitempty"`
	IsAnimated             bool   `json:"isAnimated,omitempty"`
	Width                  int    `json:"width,omitempty"`
	Height                 int    `json:"height,omitempty"`
	OriginalAssetReference string `json:"originalAssetReference,omitempty"`
	FallbackAssetReference string `json:"fallbackAssetReference,omitempty"`
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

func (o *Orchestrator) SetDeadlineProvider(fn func(ctx context.Context, requestID string) (context.Context, context.CancelFunc)) {
	o.deadlineFn = fn
}

func (o *Orchestrator) SetRuntimePipeline(pipeline *RuntimePipeline) {
	if pipeline == nil {
		pipeline = NewRuntimePipeline(nil, nil, nil)
	}
	o.pipeline = pipeline
}
