package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/agentbridge"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const GameEventPublishID = "game.event"

type AgentWakeRequest struct {
	Scope agentbridge.SessionScope
	Event protocol.GameEvent
}

type AgentWakeupPort interface {
	WakeGameAgent(context.Context, AgentWakeRequest) error
}

type AgentEventSink struct {
	sessions *agentbridge.SessionRegistry
	mu       sync.RWMutex
	port     AgentWakeupPort
	queue    chan AgentWakeRequest
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	last     map[string]time.Time
	minGap   time.Duration
}

func NewAgentEventSink(sessions *agentbridge.SessionRegistry) *AgentEventSink {
	return &AgentEventSink{
		sessions: sessions,
		queue:    make(chan AgentWakeRequest, 128),
		last:     make(map[string]time.Time),
		minGap:   250 * time.Millisecond,
	}
}

func (s *AgentEventSink) SetPort(port AgentWakeupPort) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.port = port
}

func (s *AgentEventSink) Start(parent context.Context) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.wg.Add(1)
	s.mu.Unlock()
	go s.run(ctx)
}

func (s *AgentEventSink) Shutdown() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.wg.Wait()
	}
}

func (s *AgentEventSink) run(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-s.queue:
			s.mu.RLock()
			port := s.port
			s.mu.RUnlock()
			if port != nil {
				wakeCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
				_ = port.WakeGameAgent(wakeCtx, req)
				cancel()
			}
		}
	}
}

type eventPublishEnvelope struct {
	ChannelID string                     `json:"channelId"`
	EventID   string                     `json:"eventId"`
	Payload   json.RawMessage            `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (s *AgentEventSink) Publish(ctx context.Context, n Notification) error {
	if s == nil || n.Method != "plugin.event.publish" {
		return nil
	}
	var published eventPublishEnvelope
	if err := json.Unmarshal(n.Payload, &published); err != nil {
		return fmt.Errorf("notification: decode plugin event: %w", err)
	}
	if published.EventID != GameEventPublishID {
		return nil
	}
	if wakeDisabled(published.Metadata) {
		return nil
	}
	var gameEvent protocol.GameEvent
	if err := json.Unmarshal(published.Payload, &gameEvent); err != nil {
		return fmt.Errorf("notification: decode game event: %w", err)
	}
	if gameEvent.ID == "" || gameEvent.SessionID == "" || gameEvent.Type == "" {
		return fmt.Errorf("notification: game event requires id, sessionId and type")
	}
	if len(gameEvent.ID) > 256 || len(gameEvent.SessionID) > 256 || len(gameEvent.Type) > 128 {
		return fmt.Errorf("notification: game event identity fields exceed limits")
	}
	if len(gameEvent.Payload) > 512*1024 {
		return fmt.Errorf("notification: game event payload exceeds 512 KiB")
	}
	scope, ok := s.sessions.Resolve(n.RuntimeID, gameEvent.SessionID)
	if !ok {
		return nil
	}
	if scope.PluginID != n.PluginID || scope.ServiceID != n.ServiceID {
		return fmt.Errorf("notification: game event route does not match bound game session")
	}
	if gameEvent.OccurredAt.IsZero() {
		gameEvent.OccurredAt = n.ReceivedAt
	}
	key := string(n.RuntimeID) + "\x00" + gameEvent.SessionID + "\x00" + gameEvent.Type
	now := time.Now().UTC()
	s.mu.Lock()
	if previous := s.last[key]; !previous.IsZero() && now.Sub(previous) < s.minGap {
		s.mu.Unlock()
		return nil
	}
	s.last[key] = now
	s.mu.Unlock()

	request := AgentWakeRequest{Scope: scope, Event: gameEvent}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.queue <- request:
		return nil
	default:
		// Backpressure is intentional: durable event publication still records the
		// notification while the real-time wake queue sheds duplicates/overflow.
		return nil
	}
}

func wakeDisabled(metadata map[string]json.RawMessage) bool {
	raw := metadata["wakeAgent"]
	if len(raw) == 0 {
		return false
	}
	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false
	}
	return !enabled
}
