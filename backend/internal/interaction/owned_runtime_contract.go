package interaction

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/outbox"
)

var (
	ErrOrchestratorProcessing     = errors.New("orchestrator: request still processing")
	ErrOrchestratorNotReady       = errors.New("orchestrator: not ready")
	ErrOrchestratorBusy           = errors.New("orchestrator: too many concurrent interactions")
	ErrOrchestratorCancelled      = errors.New("orchestrator: cancelled")
	ErrOrchestratorSuperseded     = errors.New("orchestrator: superseded")
	ErrOrchestratorDuplicate      = errors.New("orchestrator: duplicate request")
	ErrOrchestratorInvalidScope   = errors.New("orchestrator: invalid scope")
	ErrOrchestratorSafetyBlocked  = errors.New("orchestrator: safety blocked")
	ErrToolReconciliationRequired = errors.New("orchestrator: tool side effect requires reconciliation")
)

type ProcessRequest struct {
	CharacterID              string                     `json:"characterId,omitempty"`
	Message                  string                     `json:"message"`
	ConversationID           string                     `json:"conversationId,omitempty"`
	ThreadID                 string                     `json:"threadId,omitempty"`
	Channel                  string                     `json:"channel,omitempty"`
	Source                   string                     `json:"source,omitempty"`
	ProactiveTaskInstruction string                     `json:"-"`
	ProactiveTimeContext     string                     `json:"-"`
	ProactiveRecentContext   string                     `json:"-"`
	ProactiveRelationship    string                     `json:"-"`
	ProactiveEmotion         string                     `json:"-"`
	ProactiveMemory          string                     `json:"-"`
	PeerID                   string                     `json:"peerId,omitempty"`
	SpaceID                  string                     `json:"spaceId,omitempty"`
	DeviceTimezone           string                     `json:"deviceTimezone,omitempty"`
	SessionID                string                     `json:"sessionId,omitempty"`
	AudioUrl                 string                     `json:"audioUrl,omitempty"`
	AudioDuration            float64                    `json:"audioDuration,omitempty"`
	VoiceMessage             bool                       `json:"voiceMessage"`
	ExpressionPlan           *ExpressionPlan            `json:"expressionPlan,omitempty"`
	ImageUrl                 string                     `json:"imageUrl,omitempty"`
	VideoUrl                 string                     `json:"videoUrl,omitempty"`
	ImageContext             string                     `json:"imageContext,omitempty"`
	ReplyToMessageID         *string                    `json:"replyToMessageId,omitempty"`
	ModelConfigID            int                        `json:"modelConfigId,omitempty"`
	ReasoningEffort          string                     `json:"reasoningEffort,omitempty"`
	MessageStyle             string                     `json:"messageStyle,omitempty"`
	ReasoningEnabled         *bool                      `json:"reasoningEnabled,omitempty"`
	PermissionMode           string                     `json:"permissionMode,omitempty"`
	RequestID                string                     `json:"requestId,omitempty"`
	TurnID                   string                     `json:"turnId,omitempty"`
	ExecutionID              string                     `json:"executionId,omitempty"`
	InteractionID            string                     `json:"-"`
	ReservedInteractionID    string                     `json:"-"`
	RecoverExistingTurn      bool                       `json:"-"`
	ExpectedStatusVersion    int64                      `json:"-"`
	Runtime                  *RuntimeAssembly           `json:"-"`
	ExecContext              *coreexec.ExecutionContext `json:"-"`
	IsInternal               bool                       `json:"-"`
	SuppressReplyPersistence bool                       `json:"-"`
	ForceRegenerate          bool                       `json:"-"`
}

type ProcessResponse struct {
	ConversationID      string                `json:"conversationId"`
	ThreadID            string                `json:"threadId,omitempty"`
	TurnID              string                `json:"turnId,omitempty"`
	Sequence            int64                 `json:"sequence"`
	Reply               string                `json:"reply"`
	Reasoning           string                `json:"reasoning,omitempty"`
	ReasoningDurationMS int64                 `json:"reasoningDurationMs"`
	CharacterID         string                `json:"characterId"`
	CharacterName       string                `json:"characterName"`
	MessageIDs          []string              `json:"messageIds"`
	ForceVoice          bool                  `json:"forceVoice"`
	AudioUrls           []string              `json:"audioUrls"`
	RequestID           string                `json:"requestId"`
	MessagePlan         *MessagePlan          `json:"messagePlan,omitempty"`
	Events              []outbox.OutboxRecord `json:"-"`
}

type MessagePlan struct {
	DeliveryGroupID string            `json:"deliveryGroupId"`
	Managed         bool              `json:"managed"`
	Items           []MessagePlanItem `json:"items"`
}

type MessagePlanItem struct {
	MessageID              string `json:"messageId"`
	Sequence               int    `json:"sequence"`
	Type                   string `json:"type"`
	ExtensionType          string `json:"extensionType,omitempty"`
	Content                string `json:"content,omitempty"`
	AltText                string `json:"altText,omitempty"`
	MIMEType               string `json:"mimeType,omitempty"`
	IsAnimated             bool   `json:"isAnimated,omitempty"`
	Width                  int    `json:"width,omitempty"`
	Height                 int    `json:"height,omitempty"`
	OriginalAssetReference string `json:"originalAssetReference,omitempty"`
	FallbackAssetReference string `json:"fallbackAssetReference,omitempty"`
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
	InteractionID string                `json:"interactionId"`
	Outcome       Outcome               `json:"outcome"`
	Response      *ProcessResponse      `json:"response,omitempty"`
	Error         string                `json:"error,omitempty"`
	Duration      time.Duration         `json:"duration"`
	Events        []outbox.OutboxRecord `json:"events,omitempty"`
}

type RuntimeContextService struct {
	tracker  InteractionTracker
	pipeline *RuntimePipeline
	mu       sync.Mutex
	ready    bool
	slots    chan struct{}
}

func NewRuntimeContextService(tracker InteractionTracker, parallelTurns int) *RuntimeContextService {
	if tracker == nil {
		tracker = NewInMemoryTracker()
	}
	if parallelTurns <= 0 {
		parallelTurns = 16
	}
	return &RuntimeContextService{slots: make(chan struct{}, parallelTurns), tracker: tracker, pipeline: NewRuntimePipeline(nil, nil, nil)}
}
func (e *UnifiedEntry) SetReady(ready bool) {
	if ready {
		e.runtimeContext.SetReady(true)
	} else {
		e.runtimeContext.SetReady(false)
	}
}

func (o *RuntimeContextService) SetReady(ready bool) {
	o.mu.Lock()
	o.ready = ready
	o.mu.Unlock()
}

func (o *RuntimeContextService) IsReady() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.ready
}

func (o *RuntimeContextService) GetTracker() InteractionTracker {
	return o.tracker
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

func (o *RuntimeContextService) SetRuntimePipeline(pipeline *RuntimePipeline) {
	if pipeline == nil {
		pipeline = NewRuntimePipeline(nil, nil, nil)
	}
	o.pipeline = pipeline
}
