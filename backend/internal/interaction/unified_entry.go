package interaction

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/agentpermission"
	"github.com/u-ai/backend/internal/auth"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/temporal"
)

type EntrySource string

const (
	EntrySourceWeb       EntrySource = "web"
	EntrySourceVoice     EntrySource = "voice"
	EntrySourceProactive EntrySource = "proactive"
	EntrySourceRuntime   EntrySource = "runtime"
	EntrySourceUnknown   EntrySource = "unknown"
)

func ParseEntrySource(source string) EntrySource {
	s := EntrySource(strings.ToLower(strings.TrimSpace(source)))
	if s == "" {
		return EntrySourceUnknown
	}
	return s
}

type BackpressureStatus string

const (
	BackpressureNormal   BackpressureStatus = "normal"
	BackpressureWarning  BackpressureStatus = "warning"
	BackpressureShedding BackpressureStatus = "shedding"
)

type BackpressureConfig struct {
	MaxQueueDepth   int           `json:"maxQueueDepth"`
	WarningRatio    float64       `json:"warningRatio"`
	SheddingRatio   float64       `json:"sheddingRatio"`
	CooldownBase    time.Duration `json:"cooldownBase"`
	CooldownMax     time.Duration `json:"cooldownMax"`
	RecoveryTimeout time.Duration `json:"recoveryTimeout"`
}

func DefaultBackpressureConfig() BackpressureConfig {
	return BackpressureConfig{
		MaxQueueDepth:   50,
		WarningRatio:    0.6,
		SheddingRatio:   0.9,
		CooldownBase:    1 * time.Second,
		CooldownMax:     30 * time.Second,
		RecoveryTimeout: 15 * time.Second,
	}
}

type BackpressureState struct {
	Status        BackpressureStatus `json:"status"`
	QueueDepth    int                `json:"queueDepth"`
	CooldownUntil time.Time          `json:"cooldownUntil"`
}

func (s *BackpressureState) updateStatusLocked(cfg BackpressureConfig) {
	ratio := queueRatio(s.QueueDepth, cfg.MaxQueueDepth)
	switch {
	case ratio >= cfg.SheddingRatio:
		s.Status = BackpressureShedding
	case ratio >= cfg.WarningRatio:
		s.Status = BackpressureWarning
	default:
		s.Status = BackpressureNormal
	}
}

func (s *BackpressureState) applyCooldownLocked(cfg BackpressureConfig, now time.Time) time.Duration {
	ratio := queueRatio(s.QueueDepth, cfg.MaxQueueDepth)
	duration := time.Duration(float64(cfg.CooldownBase) * (1.0 + ratio))
	if duration > cfg.CooldownMax {
		duration = cfg.CooldownMax
	}
	if cfg.RecoveryTimeout > 0 && duration > cfg.RecoveryTimeout {
		duration = cfg.RecoveryTimeout
	}
	s.CooldownUntil = now.Add(duration)
	return duration
}

var (
	ErrBackpressureShedding  = errors.New("unified_entry: shedding due to backpressure")
	ErrBackpressureCooldown  = errors.New("unified_entry: in cooldown period")
	ErrInvalidChannel        = errors.New("unified_entry: invalid channel")
	ErrScopeResolutionFailed = errors.New("unified_entry: scope resolution failed")
	ErrSourceAccessDenied    = errors.New("unified_entry: source access denied")
)

type UnifiedEntryRequest struct {
	Channel                  string             `json:"channel"`
	Message                  string             `json:"message"`
	PeerID                   string             `json:"peerId,omitempty"`
	SpaceID                  string             `json:"spaceId,omitempty"`
	DeviceTimezone           string             `json:"deviceTimezone,omitempty"`
	Source                   string             `json:"source,omitempty"`
	AdmissionTrigger         string             `json:"admissionTrigger,omitempty"`
	AdmissionExplicit        bool               `json:"admissionExplicit,omitempty"`
	ProactiveTaskInstruction string             `json:"-"`
	ProactiveTimeContext     string             `json:"-"`
	ProactiveRecentContext   string             `json:"-"`
	ProactiveRelationship    string             `json:"-"`
	ProactiveEmotion         string             `json:"-"`
	ProactiveMemory          string             `json:"-"`
	CharacterID              string             `json:"characterId,omitempty"`
	AgentID                  string             `json:"agentId,omitempty"`
	TeamID                   string             `json:"teamId,omitempty"`
	ConversationID           string             `json:"conversationId,omitempty"`
	ThreadID                 string             `json:"threadId,omitempty"`
	WorkspaceID              string             `json:"workspaceId,omitempty"`
	WorkspaceDeviceID        string             `json:"workspaceDeviceId,omitempty"`
	WorkspaceName            string             `json:"workspaceName,omitempty"`
	WorkspaceKind            string             `json:"workspaceKind,omitempty"`
	WorkspaceRootURI         string             `json:"workspaceRootUri,omitempty"`
	AudioUrl                 string             `json:"audioUrl,omitempty"`
	AudioDuration            float64            `json:"audioDuration,omitempty"`
	VoiceMessage             bool               `json:"voiceMessage"`
	ExpressionPlan           *ExpressionPlan    `json:"expressionPlan,omitempty"`
	ImageUrl                 string             `json:"imageUrl,omitempty"`
	VideoUrl                 string             `json:"videoUrl,omitempty"`
	ImageContext             string             `json:"imageContext,omitempty"`
	ReplyToMessageID         *string            `json:"replyToMessageId,omitempty"`
	ReplyExcerpt             string             `json:"replyExcerpt,omitempty"`
	SenderName               string             `json:"senderName,omitempty"`
	SenderID                 string             `json:"senderId,omitempty"`
	MentionNames             []string           `json:"mentionNames,omitempty"`
	PeerKind                 string             `json:"peerKind,omitempty"`
	ConversationTitle        string             `json:"conversationTitle,omitempty"`
	SandboxWorkspaceID       string             `json:"sandboxWorkspaceId,omitempty"`
	SandboxRoot              string             `json:"sandboxRoot,omitempty"`
	SandboxAllowNetwork      bool               `json:"sandboxAllowNetwork,omitempty"`
	FileUrl                  string             `json:"fileUrl,omitempty"`
	FileName                 string             `json:"fileName,omitempty"`
	ModelConfigID            int                `json:"modelConfigId,omitempty"`
	ReasoningEffort          string             `json:"reasoningEffort,omitempty"`
	MessageStyle             string             `json:"messageStyle,omitempty"`
	ReasoningEnabled         *bool              `json:"reasoningEnabled,omitempty"`
	PermissionMode           string             `json:"permissionMode,omitempty"`
	RequestID                string             `json:"requestId,omitempty"`
	TurnID                   string             `json:"turnId,omitempty"`
	ExecutionID              string             `json:"executionId,omitempty"`
	TaskRunID                string             `json:"taskRunId,omitempty"`
	WorkflowRunID            string             `json:"workflowRunId,omitempty"`
	ApprovalID               string             `json:"approvalId,omitempty"`
	SessionID                string             `json:"sessionId,omitempty"`
	IsInternal               bool               `json:"-"`
	SuppressReplyPersistence bool               `json:"-"`
	ForceRegenerate          bool               `json:"-"`
	ReservedInteractionID    string             `json:"-"`
	RecoverExistingTurn      bool               `json:"-"`
	OriginActor              *auth.ActorContext `json:"-"`
}

type UnifiedEntry struct {
	canonical      EntryExecutor
	runtimeContext *RuntimeContextService
	resolver       ScopeResolver
	execService    *coreexec.ExecutionService
	bpCfg          BackpressureConfig
	bpState        *BackpressureState
	access         *agentpermission.Service
	agentRuntime   AgentRuntimeProvider
	mu             sync.Mutex
	clock          temporal.Clock
}

type AgentRuntimeProfile struct {
	AgentID          string   `json:"agentId"`
	TeamID           string   `json:"teamId"`
	RuntimeID        string   `json:"runtimeId"`
	ModelConfigID    int      `json:"modelConfigId"`
	ReasoningEffort  string   `json:"reasoningEffort"`
	PermissionMode   string   `json:"permissionMode"`
	WorkspaceID      string   `json:"workspaceId"`
	WorkspaceRootURI string   `json:"workspaceRootUri"`
	Skills           []string `json:"skills"`
	Tools            []string `json:"tools"`
}

type AgentRuntimeProvider interface {
	ResolveAgentRuntime(context.Context, string, string) (AgentRuntimeProfile, bool, error)
}

type EntryExecutor interface {
	ExecuteEntry(context.Context, *UnifiedEntryRequest) (*OrchestrationResult, error)
	CancelEntryScope(InteractionScope) int
	EntryInteractionActive(string) bool
	EntryReady() bool
}

func (e *UnifiedEntry) SetEntryExecutor(executor EntryExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.canonical = executor
}

func NewUnifiedEntry(runtimeContext *RuntimeContextService, resolver ScopeResolver, clock temporal.Clock) *UnifiedEntry {
	return &UnifiedEntry{
		runtimeContext: runtimeContext,
		resolver:       resolver,
		bpCfg:          DefaultBackpressureConfig(),
		bpState:        &BackpressureState{Status: BackpressureNormal},
		clock:          clock,
	}
}

func (e *UnifiedEntry) SetExecutionService(svc *coreexec.ExecutionService) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.execService = svc
}

func (e *UnifiedEntry) SetAccessService(svc *agentpermission.Service) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.access = svc
}

func (e *UnifiedEntry) SetAgentRuntimeProvider(provider AgentRuntimeProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.agentRuntime = provider
}

func admissionInput(req *UnifiedEntryRequest) agentpermission.AdmissionInput {
	trigger := strings.TrimSpace(req.AdmissionTrigger)
	if trigger == "" {
		switch {
		case strings.EqualFold(req.Source, "proactive"):
			trigger = "inferred"
		case strings.EqualFold(req.PeerKind, "group"):
			if len(req.MentionNames) > 0 {
				trigger = "mention"
			} else if req.ReplyToMessageID != nil && strings.TrimSpace(*req.ReplyToMessageID) != "" {
				trigger = "reply"
			} else {
				trigger = "all"
			}
		default:
			trigger = "direct"
		}
	}
	explicit := req.AdmissionExplicit
	if !strings.EqualFold(req.PeerKind, "group") || strings.EqualFold(req.Channel, "web") || strings.EqualFold(req.Channel, "voice") {
		explicit = true
	}
	if trigger == "mention" || trigger == "reply" || trigger == "prefix" || trigger == "direct" {
		explicit = true
	}
	if strings.EqualFold(req.Source, "proactive") || strings.EqualFold(req.Source, "runtime") {
		explicit = false
	}
	return agentpermission.AdmissionInput{
		SpaceID:             req.SpaceID,
		RequestID:           req.RequestID,
		ChannelID:           req.Channel,
		AccountID:           req.SessionID,
		ConversationID:      req.ConversationID,
		SenderID:            req.SenderID,
		PeerKind:            req.PeerKind,
		Source:              req.Source,
		TriggerMode:         trigger,
		Explicit:            explicit,
		RequestedPermission: req.PermissionMode,
	}
}

func (e *UnifiedEntry) Handle(ctx context.Context, req *UnifiedEntryRequest) (*OrchestrationResult, error) {
	if req == nil {
		return nil, ErrOrchestratorInvalidScope
	}
	e.mu.Lock()
	now := e.clock.Now()
	if now.Before(e.bpState.CooldownUntil) {
		e.mu.Unlock()
		return nil, ErrBackpressureCooldown
	}
	e.bpState.QueueDepth++
	e.bpState.updateStatusLocked(e.bpCfg)
	status := e.bpState.Status
	canonical := e.canonical
	access := e.access
	agentRuntime := e.agentRuntime
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.bpState.QueueDepth--
		e.bpState.updateStatusLocked(e.bpCfg)
		e.mu.Unlock()
	}()

	if status == BackpressureShedding {
		e.mu.Lock()
		e.bpState.applyCooldownLocked(e.bpCfg, now)
		e.mu.Unlock()
		return nil, ErrBackpressureShedding
	}
	originActor := req.OriginActor
	if originActor == nil {
		originActor, _ = auth.FromContext(ctx)
	}
	if originActor != nil {
		ctx = auth.WithActor(ctx, originActor)
		if strings.TrimSpace(req.SpaceID) == "" && originActor.SpaceID.String() != "" {
			req.SpaceID = originActor.SpaceID.String()
		}
	}
	if access != nil && access.Ready() && !req.IsInternal && !strings.EqualFold(req.Source, "proactive") && !strings.EqualFold(req.Source, "runtime") {
		admission, err := access.Evaluate(ctx, admissionInput(req))
		if err != nil {
			return nil, err
		}
		if admission.Observed {
			return &OrchestrationResult{Outcome: OutcomeObserved, Duration: 0}, nil
		}
		if !admission.Allowed {
			return nil, fmt.Errorf("%w: %s", ErrSourceAccessDenied, admission.Reason)
		}
		req.PermissionMode = admission.PermissionMode
	}
	if agentRuntime != nil && strings.TrimSpace(req.SpaceID) != "" && strings.TrimSpace(req.CharacterID) != "" {
		profile, found, err := agentRuntime.ResolveAgentRuntime(ctx, req.SpaceID, req.CharacterID)
		if err != nil {
			return nil, err
		}
		if found {
			req.AgentID = profile.AgentID
			req.TeamID = profile.TeamID
			if req.ModelConfigID == 0 {
				req.ModelConfigID = profile.ModelConfigID
			}
			if strings.TrimSpace(req.ReasoningEffort) == "" {
				req.ReasoningEffort = profile.ReasoningEffort
			}
			if strings.TrimSpace(req.PermissionMode) == "" {
				req.PermissionMode = profile.PermissionMode
			}
			if strings.TrimSpace(req.WorkspaceID) == "" && profile.WorkspaceID != "" {
				req.WorkspaceID = profile.WorkspaceID
				req.WorkspaceRootURI = profile.WorkspaceRootURI
				req.WorkspaceName = "Agent 电脑"
				req.WorkspaceKind = "local"
			}
		}
	}
	if canonical != nil {
		copied := *req
		copied.RequestID = stableRequestID(req.RequestID)
		return canonical.ExecuteEntry(ctx, &copied)
	}

	return nil, ErrOrchestratorNotReady
}

func (e *UnifiedEntry) ResolveScope(ctx context.Context, req *UnifiedEntryRequest) (ScopeResolution, error) {
	source := parseOptionalEntrySource(req.Source)
	scopeInput := ScopeResolveInput{
		SpaceID:        req.SpaceID,
		CharacterID:    req.CharacterID,
		ConversationID: req.ConversationID,
		Channel:        req.Channel,
		PeerID:         req.PeerID,
		SessionID:      req.SessionID,
		RequestID:      req.RequestID,
		Source:         source,
	}
	return e.resolver.Resolve(ctx, scopeInput)
}

func (e *UnifiedEntry) GetBackpressureStatus() BackpressureStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.bpState.Status
}

func (e *UnifiedEntry) IsReady() bool {
	if e == nil {
		return false
	}
	e.mu.Lock()
	canonical := e.canonical
	e.mu.Unlock()
	if canonical != nil {
		return canonical.EntryReady()
	}
	return false
}

func (e *UnifiedEntry) IsInteractionActive(interactionID string) bool {
	if e == nil {
		return false
	}
	e.mu.Lock()
	canonical := e.canonical
	e.mu.Unlock()
	if canonical != nil {
		return canonical.EntryInteractionActive(interactionID)
	}
	return false
}

func (e *UnifiedEntry) SetBackpressureConfig(cfg BackpressureConfig) {
	e.mu.Lock()
	cfg = normalizeBackpressureConfig(cfg)
	e.bpCfg = cfg
	e.bpState.updateStatusLocked(cfg)
	e.mu.Unlock()
}

func (e *UnifiedEntry) CancelByPeer(channel, peerID string) int {
	scope := InteractionScope{
		Channel: channel,
		PeerID:  peerID,
	}.Normalize()
	count := e.CancelByScope(scope)
	log.Printf("[unified_entry] cancelled %d interactions for channel=%s peer=%s", count, channel, peerID)
	return count
}

func (e *UnifiedEntry) CancelByScope(scope InteractionScope) int {
	e.mu.Lock()
	canonical := e.canonical
	e.mu.Unlock()
	if canonical != nil {
		return canonical.CancelEntryScope(scope)
	}
	return 0
}

func normalizeBackpressureConfig(cfg BackpressureConfig) BackpressureConfig {
	defaults := DefaultBackpressureConfig()
	if cfg.MaxQueueDepth <= 0 {
		cfg.MaxQueueDepth = defaults.MaxQueueDepth
	}
	if cfg.WarningRatio <= 0 || cfg.WarningRatio > 1 || math.IsNaN(cfg.WarningRatio) || math.IsInf(cfg.WarningRatio, 0) {
		cfg.WarningRatio = defaults.WarningRatio
	}
	if cfg.SheddingRatio <= 0 || cfg.SheddingRatio > 1 || math.IsNaN(cfg.SheddingRatio) || math.IsInf(cfg.SheddingRatio, 0) {
		cfg.SheddingRatio = defaults.SheddingRatio
	}
	if cfg.WarningRatio > cfg.SheddingRatio {
		cfg.WarningRatio = defaults.WarningRatio
		cfg.SheddingRatio = defaults.SheddingRatio
	}
	if cfg.CooldownBase <= 0 {
		cfg.CooldownBase = defaults.CooldownBase
	}
	if cfg.CooldownMax <= 0 || cfg.CooldownMax < cfg.CooldownBase {
		cfg.CooldownMax = defaults.CooldownMax
	}
	if cfg.RecoveryTimeout <= 0 {
		cfg.RecoveryTimeout = defaults.RecoveryTimeout
	}
	return cfg
}

func queueRatio(depth int, maxDepth int) float64 {
	if maxDepth <= 0 {
		return 0
	}
	if depth < 0 {
		depth = 0
	}
	return float64(depth) / float64(maxDepth)
}

func stableRequestID(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID != "" {
		return requestID
	}
	return uuid.New().String()
}

func parseOptionalEntrySource(source string) string {
	if strings.TrimSpace(source) == "" {
		return ""
	}
	return string(ParseEntrySource(source))
}
