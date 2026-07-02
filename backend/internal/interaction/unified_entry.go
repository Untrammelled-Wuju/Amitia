package interaction

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
)

type EntrySource string

const (
	EntrySourceWeb     EntrySource = "web"
	EntrySourceWeChat  EntrySource = "wechat"
	EntrySourceQQ      EntrySource = "qq"
	EntrySourceUnknown EntrySource = "unknown"
)

func ParseEntrySource(source string) EntrySource {
	s := EntrySource(strings.ToLower(strings.TrimSpace(source)))
	switch s {
	case EntrySourceWeb, EntrySourceWeChat, EntrySourceQQ:
		return s
	default:
		return EntrySourceUnknown
	}
}

type BackpressureStatus string

const (
	BackpressureNormal     BackpressureStatus = "normal"
	BackpressureWarning    BackpressureStatus = "warning"
	BackpressureShedding   BackpressureStatus = "shedding"
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
	Status       BackpressureStatus `json:"status"`
	QueueDepth   int                `json:"queueDepth"`
	CooldownUntil time.Time         `json:"cooldownUntil"`
	mu           sync.RWMutex
}

func (s *BackpressureState) updateStatus(cfg BackpressureConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ratio := float64(s.QueueDepth) / float64(cfg.MaxQueueDepth)
	switch {
	case ratio >= cfg.SheddingRatio:
		s.Status = BackpressureShedding
	case ratio >= cfg.WarningRatio:
		s.Status = BackpressureWarning
	default:
		s.Status = BackpressureNormal
	}
}

func (s *BackpressureState) applyCooldown(cfg BackpressureConfig) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	ratio := float64(s.QueueDepth) / float64(cfg.MaxQueueDepth)
	duration := time.Duration(float64(cfg.CooldownBase) * (1.0 + ratio))
	if duration > cfg.CooldownMax {
		duration = cfg.CooldownMax
	}
	s.CooldownUntil = time.Now().Add(duration)
	return duration
}

var (
	ErrBackpressureShedding  = errors.New("unified_entry: shedding due to backpressure")
	ErrBackpressureCooldown  = errors.New("unified_entry: in cooldown period")
	ErrInvalidChannel        = errors.New("unified_entry: invalid channel")
	ErrScopeResolutionFailed  = errors.New("unified_entry: scope resolution failed")
)

type UnifiedEntryRequest struct {
	Channel        string  `json:"channel"`
	Message        string  `json:"message"`
	PeerID         string  `json:"peerId,omitempty"`
	UserID         string  `json:"userId,omitempty"`
	Source         string  `json:"source,omitempty"`
	CharacterID    string  `json:"characterId,omitempty"`
	ConversationID string  `json:"conversationId,omitempty"`
	AudioUrl       string  `json:"audioUrl,omitempty"`
	AudioDuration  float64 `json:"audioDuration,omitempty"`
	VoiceMessage   bool    `json:"voiceMessage"`
	ImageUrl       string  `json:"imageUrl,omitempty"`
	VideoUrl       string  `json:"videoUrl,omitempty"`
	ImageContext   string  `json:"imageContext,omitempty"`
}

type UnifiedEntry struct {
	orchestrator   *Orchestrator
	resolver       ScopeResolver
	bpCfg          BackpressureConfig
	bpState        *BackpressureState
	mu             sync.Mutex
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
	e.bpState.updateStatus(e.bpCfg)
	status := e.bpState.Status
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.bpState.QueueDepth--
		e.bpState.updateStatus(e.bpCfg)
		e.mu.Unlock()
	}()

	if status == BackpressureShedding {
		e.bpState.applyCooldown(e.bpCfg)
		return nil, ErrBackpressureShedding
	}

	source := ParseEntrySource(req.Source)
	scopeInput := ScopeResolveInput{
		UserID:         req.UserID,
		CharacterID:    req.CharacterID,
		ConversationID: req.ConversationID,
		Channel:        req.Channel,
		PeerID:         req.PeerID,
		Source:         string(source),
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
		Source:         string(source),
		PeerID:         req.PeerID,
		UserID:         req.UserID,
		AudioUrl:       req.AudioUrl,
		AudioDuration:  req.AudioDuration,
		VoiceMessage:   req.VoiceMessage,
		ImageUrl:       req.ImageUrl,
		VideoUrl:       req.VideoUrl,
		ImageContext:   req.ImageContext,
	}

	return e.orchestrator.Process(ctx, procReq)
}

func (e *UnifiedEntry) ResolveScope(ctx context.Context, req *UnifiedEntryRequest) (ScopeResolution, error) {
	source := ParseEntrySource(req.Source)
	scopeInput := ScopeResolveInput{
		UserID:         req.UserID,
		CharacterID:    req.CharacterID,
		ConversationID: req.ConversationID,
		Channel:        req.Channel,
		PeerID:         req.PeerID,
		Source:         string(source),
	}
	return e.resolver.Resolve(ctx, scopeInput)
}

func (e *UnifiedEntry) GetBackpressureStatus() BackpressureStatus {
	e.bpState.mu.RLock()
	defer e.bpState.mu.RUnlock()
	return e.bpState.Status
}

func (e *UnifiedEntry) SetBackpressureConfig(cfg BackpressureConfig) {
	e.mu.Lock()
	e.bpCfg = cfg
	e.mu.Unlock()
}

func (e *UnifiedEntry) CancelByPeer(channel, peerID string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	scope := InteractionScope{
		Channel: channel,
		PeerID:  peerID,
	}.Normalize()
	count := e.orchestrator.CancelByScope(scope)
	log.Printf("[unified_entry] cancelled %d interactions for channel=%s peer=%s", count, channel, peerID)
	return count
}
