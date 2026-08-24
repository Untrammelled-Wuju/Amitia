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

// PluginAgentEventPublishID is an opaque host-level event understood only as a
// wake-up hint. Event Type/Payload remain plugin-defined and untrusted.
const PluginAgentEventPublishID = "plugin.agent_event"

type AgentWakeRequest struct {
	Scope agentbridge.SessionScope
	Event protocol.PluginEvent
}
type AgentWakeupPort interface {
	WakePluginAgent(context.Context, AgentWakeRequest) error
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
	return &AgentEventSink{sessions: sessions, queue: make(chan AgentWakeRequest, 128), last: make(map[string]time.Time), minGap: 250 * time.Millisecond}
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
				_ = port.WakePluginAgent(wakeCtx, req)
				cancel()
			}
		}
	}
}

type eventPublishEnvelope struct {
	EventID  string                     `json:"eventId"`
	Payload  json.RawMessage            `json:"payload"`
	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (s *AgentEventSink) Publish(ctx context.Context, n Notification) error {
	if s == nil || n.Method != "plugin.event.publish" {
		return nil
	}
	var published eventPublishEnvelope
	if err := json.Unmarshal(n.Payload, &published); err != nil {
		return fmt.Errorf("notification: decode plugin event envelope: %w", err)
	}
	if published.EventID != PluginAgentEventPublishID || wakeDisabled(published.Metadata) {
		return nil
	}
	var event protocol.PluginEvent
	if err := json.Unmarshal(published.Payload, &event); err != nil {
		return fmt.Errorf("notification: decode plugin event: %w", err)
	}
	if event.ID == "" || event.Type == "" {
		return fmt.Errorf("notification: plugin event requires id and type")
	}
	if len(event.ID) > 256 || len(event.SessionID) > 256 || len(event.Type) > 128 {
		return fmt.Errorf("notification: plugin event identity fields exceed limits")
	}
	if len(event.Payload) > 512*1024 {
		return fmt.Errorf("notification: plugin event payload exceeds 512 KiB")
	}
	scope, ok := s.sessions.Resolve(n.RuntimeID, event.SessionID)
	if !ok {
		return nil
	}
	if scope.PluginID != n.PluginID || scope.ServiceID != n.ServiceID || scope.Generation != n.Generation {
		return fmt.Errorf("notification: plugin event route does not match bound runtime context")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = n.ReceivedAt
	}
	key := string(n.RuntimeID) + "\x00" + event.SessionID + "\x00" + event.Type
	now := time.Now().UTC()
	s.mu.Lock()
	if prev := s.last[key]; !prev.IsZero() && now.Sub(prev) < s.minGap {
		s.mu.Unlock()
		return nil
	}
	s.last[key] = now
	s.mu.Unlock()
	req := AgentWakeRequest{Scope: scope, Event: event}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.queue <- req:
		return nil
	default:
		return nil
	}
}
func wakeDisabled(metadata map[string]json.RawMessage) bool {
	raw := metadata["wakeAgent"]
	if len(raw) == 0 {
		return false
	}
	var enabled bool
	if json.Unmarshal(raw, &enabled) != nil {
		return false
	}
	return !enabled
}
