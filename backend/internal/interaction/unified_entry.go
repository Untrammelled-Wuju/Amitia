package interaction

import (
	"context"
	"errors"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type EntrySource string

const (
	EntrySourceWeb     EntrySource = "web"
	EntrySourceWeChat  EntrySource = "wechat"
	EntrySourceQQ      EntrySource = "qq"
	EntrySourceVoice   EntrySource = "voice"
	EntrySourceUnknown EntrySource = "unknown"
)

func ParseEntrySource(source string) EntrySource {
	s := EntrySource(strings.ToLower(strings.TrimSpace(source)))
	switch s {
	case EntrySourceWeb, EntrySourceWeChat, EntrySourceQQ, EntrySourceVoice:
		return s
	default:
		return EntrySourceUnknown
	}
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

func (s *BackpressureState) applyCooldownLocked(cfg BackpressureConfig) time.Duration {
	ratio := queueRatio(s.QueueDepth, cfg.MaxQueueDepth)
	duration := time.Duration(float64(cfg.CooldownBase) * (1.0 + ratio))
	if duration > cfg.CooldownMax {
		duration = cfg.CooldownMax
	}
	if cfg.RecoveryTimeout > 0 && duration > cfg.RecoveryTimeout {
		duration = cfg.RecoveryTimeout
	}
	s.CooldownUntil = time.Now().Add(duration)
	return duration
}

var (
	ErrBackpressureShedding  = errors.New("unified_entry: shedding due to backpressure")
	ErrBackpressureCooldown  = errors.New("unified_entry: in cooldown period")
	ErrInvalidChannel        = errors.New("unified_entry: invalid channel")
	ErrScopeResolutionFailed = errors.New("unified_entry: scope resolution failed")
)

type UnifiedEntryRequest struct {
	Channel        string          `json:"channel"`
	Message        string          `json:"message"`
	PeerID         string          `json:"peerId,omitempty"`
	UserID         string          `json:"userId,omitempty"`
	Source         string          `json:"source,omitempty"`
	ProactiveTimeContext  string `json:"-"`
	ProactiveRecentContext string `json:"-"`
	ProactiveRelationship  string `json:"-"`
	ProactiveEmotion      string `json:"-"`
	ProactiveMemory       string `json:"-"`
	CharacterID    string          `json:"characterId,omitempty"`
	ConversationID string          `json:"conversationId,omitempty"`
	AudioUrl       string          `json:"audioUrl,omitempty"`
	AudioDuration  float64         `json:"audioDuration,omitempty"`
	VoiceMessage   bool            `json:"voiceMessage"`
	ExpressionPlan *ExpressionPlan `json:"expressionPlan,omitempty"`
	ImageUrl       string          `json:"imageUrl,omitempty"`
	VideoUrl       string          `json:"videoUrl,omitempty"`
	ImageContext   string          `json:"imageContext,omitempty"`
	RequestID      string          `json:"requestId,omitempty"`
	SessionID      string          `json:"sessionId,omitempty"`
	IsInternal     bool            `json:"-"`
}

type UnifiedEntry struct {
	orchestrator *Orchestrator
	resolver     ScopeResolver
	bpCfg        BackpressureConfig
	bpState      *BackpressureState
	mu           sync.Mutex
}

func NewUnifiedEntry(orchestrator *Orchestrator, resolver ScopeResolver) *UnifiedEntry {
	return &UnifiedEntry{
		orchestrator: orchestrator,
		resolver:     resolver,
		bpCfg:        DefaultBackpressureConfig(),
		bpState:      &BackpressureState{Status: BackpressureNormal},
	}
}

func (e *UnifiedEntry) Handle(ctx context.Context, req *UnifiedEntryRequest) (*OrchestrationResult, error) {
	e.mu.Lock()
	now := time.Now()
	if now.Before(e.bpState.CooldownUntil) {
		e.mu.Unlock()
		return nil, ErrBackpressureCooldown
	}
	e.bpState.QueueDepth++
	e.bpState.updateStatusLocked(e.bpCfg)
	status := e.bpState.Status
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.bpState.QueueDepth--
		e.bpState.updateStatusLocked(e.bpCfg)
		e.mu.Unlock()
	}()

	if status == BackpressureShedding {
		e.mu.Lock()
		e.bpState.applyCooldownLocked(e.bpCfg)
		e.mu.Unlock()
		return nil, ErrBackpressureShedding
	}

	requestID := stableRequestID(req.RequestID)
	source := parseOptionalEntrySource(req.Source)
	scopeInput := ScopeResolveInput{
		UserID:         req.UserID,
		CharacterID:    req.CharacterID,
		ConversationID: req.ConversationID,
		Channel:        req.Channel,
		PeerID:         req.PeerID,
		SessionID:      req.SessionID,
		RequestID:      requestID,
		Source:         source,
	}

	resolution, err := e.resolver.Resolve(ctx, scopeInput)
	if err != nil {
		return nil, err
	}

	procReq := &ProcessRequest{
		CharacterID:    resolution.Scope.CharacterID,
		ConversationID: resolution.Scope.ConversationID,
		Message:        req.Message,
		Channel:        resolution.Scope.Channel,
		Source:         resolution.Source,
		PeerID:         resolution.Scope.PeerID,
		UserID:         resolution.Scope.UserID,
		SessionID:      resolution.Scope.SessionID,
		RequestID:      requestID,
		AudioUrl:       req.AudioUrl,
		AudioDuration:  req.AudioDuration,
		VoiceMessage:   req.VoiceMessage,
		ExpressionPlan: req.ExpressionPlan,
		ImageUrl:       req.ImageUrl,
		VideoUrl:       req.VideoUrl,
		ImageContext:   req.ImageContext,
		IsInternal:     req.IsInternal,
		ProactiveTimeContext:  req.ProactiveTimeContext,
		ProactiveRecentContext: req.ProactiveRecentContext,
		ProactiveRelationship:  req.ProactiveRelationship,
		ProactiveEmotion:      req.ProactiveEmotion,
		ProactiveMemory:       req.ProactiveMemory,
	}

	return e.orchestrator.Process(ctx, procReq)
}

func (e *UnifiedEntry) ResolveScope(ctx context.Context, req *UnifiedEntryRequest) (ScopeResolution, error) {
	source := parseOptionalEntrySource(req.Source)
	scopeInput := ScopeResolveInput{
		UserID:         req.UserID,
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

func (e *UnifiedEntry) IsOrchestratorReady() bool {
	if e == nil || e.orchestrator == nil {
		return false
	}
	return e.orchestrator.IsReady()
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
	count := e.orchestrator.CancelByScope(scope)
	log.Printf("[unified_entry] cancelled %d interactions for channel=%s peer=%s", count, channel, peerID)
	return count
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
