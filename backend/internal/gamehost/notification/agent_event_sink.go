package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/agentbridge"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

// PluginAgentEventPublishID is an opaque host-level event understood only as a
// wake-up hint. Event Type/Payload remain plugin-defined and untrusted.
const PluginAgentEventPublishID = "plugin.agent_event"

const (
	defaultAgentEventQueueSize      = 128
	defaultAgentEventWorkerCount    = 4
	defaultAgentEventRateMaxEntries = 4096
	defaultAgentEventRateEntryTTL   = 10 * time.Minute
	defaultAgentWakeMaxAttempts     = 3
	defaultAgentWakeAttemptTimeout  = 3 * time.Minute
	defaultAgentWakeRetryBaseDelay  = 250 * time.Millisecond
	defaultAgentWakeDeadLetterMax   = 128
)

var ErrAgentEventQueueFull = errors.New("notification: agent event queue full")

type AgentWakeRequest struct {
	Scope agentbridge.SessionScope
	Event protocol.PluginEvent
}

type AgentWakeupPort interface {
	WakePluginAgent(context.Context, AgentWakeRequest) error
}

type AgentWakeFailure struct {
	Request  AgentWakeRequest
	Error    string
	Attempts int
	FailedAt time.Time
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

	rateMaxEntries int
	rateEntryTTL   time.Duration
	maxAttempts    int
	attemptTimeout time.Duration
	retryBaseDelay time.Duration
	deadLetterMax  int
	deadLetters    []AgentWakeFailure
}

func NewAgentEventSink(sessions *agentbridge.SessionRegistry) *AgentEventSink {
	return &AgentEventSink{
		sessions:       sessions,
		queue:          make(chan AgentWakeRequest, defaultAgentEventQueueSize),
		last:           make(map[string]time.Time),
		minGap:         250 * time.Millisecond,
		rateMaxEntries: defaultAgentEventRateMaxEntries,
		rateEntryTTL:   defaultAgentEventRateEntryTTL,
		maxAttempts:    defaultAgentWakeMaxAttempts,
		attemptTimeout: defaultAgentWakeAttemptTimeout,
		retryBaseDelay: defaultAgentWakeRetryBaseDelay,
		deadLetterMax:  defaultAgentWakeDeadLetterMax,
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
	workers := defaultAgentEventWorkerCount
	if workers < 1 {
		workers = 1
	}
	s.wg.Add(workers)
	s.mu.Unlock()
	for i := 0; i < workers; i++ {
		go s.run(ctx)
	}
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
			s.deliverWithRetry(ctx, req)
		}
	}
}

func (s *AgentEventSink) deliverWithRetry(ctx context.Context, req AgentWakeRequest) {
	attempts := s.maxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		s.mu.RLock()
		port := s.port
		s.mu.RUnlock()
		if port == nil {
			lastErr = errors.New("notification: Agent wakeup port unavailable")
		} else {
			timeout := s.attemptTimeout
			if timeout <= 0 {
				timeout = defaultAgentWakeAttemptTimeout
			}
			wakeCtx, cancel := context.WithTimeout(ctx, timeout)
			lastErr = port.WakePluginAgent(wakeCtx, req)
			cancel()
			if lastErr == nil {
				return
			}
		}
		if attempt == attempts || ctx.Err() != nil {
			break
		}
		delay := s.retryBaseDelay << (attempt - 1)
		if delay <= 0 {
			delay = defaultAgentWakeRetryBaseDelay
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
	if ctx.Err() == nil {
		s.recordDeadLetter(req, attempts, lastErr)
	}
}

func (s *AgentEventSink) recordDeadLetter(req AgentWakeRequest, attempts int, err error) {
	if s == nil || s.deadLetterMax <= 0 {
		return
	}
	failure := AgentWakeFailure{Request: req, Attempts: attempts, FailedAt: time.Now().UTC()}
	if err != nil {
		failure.Error = err.Error()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.deadLetters) >= s.deadLetterMax {
		copy(s.deadLetters, s.deadLetters[len(s.deadLetters)-s.deadLetterMax+1:])
		s.deadLetters = s.deadLetters[:s.deadLetterMax-1]
	}
	s.deadLetters = append(s.deadLetters, failure)
}

// DeadLetters returns a bounded snapshot of wakeups that still failed after
// retry. This prevents silent loss and gives observability/management layers a
// deterministic recovery surface without allowing unbounded memory growth.
func (s *AgentEventSink) DeadLetters() []AgentWakeFailure {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AgentWakeFailure, len(s.deadLetters))
	copy(out, s.deadLetters)
	return out
}

func (s *AgentEventSink) DeadLetterCount() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.deadLetters)
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
	if s.sessions == nil {
		return fmt.Errorf("notification: Agent session registry unavailable")
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
	if n.RuntimeID == "" || n.ServiceID == "" || n.PluginID == "" || n.Generation <= 0 {
		return fmt.Errorf("notification: plugin event route is incomplete")
	}

	scope, ok := s.sessions.Resolve(n.RuntimeID, n.ServiceID, event.SessionID)
	if !ok {
		// Cold start: the game may publish an event before the Agent has ever
		// invoked a plugin tool. Preserve the authenticated notification route and
		// let the host wakeup adapter resolve the default/active Agent target.
		scope = agentbridge.SessionScope{
			PluginSessionID: event.SessionID,
			PluginID:        n.PluginID,
			RuntimeID:       n.RuntimeID,
			ServiceID:       n.ServiceID,
			Generation:      n.Generation,
		}
	} else {
		if scope.PluginID != n.PluginID || scope.ServiceID != n.ServiceID {
			return fmt.Errorf("notification: plugin event route does not match bound runtime context")
		}
		if scope.Generation != n.Generation {
			// Runtime generations are monotonic. Carry host context forward across a
			// restart, but never accept a late event from an older generation.
			if n.Generation <= scope.Generation {
				return fmt.Errorf("notification: stale plugin event generation %d (bound %d)", n.Generation, scope.Generation)
			}
			scope.Generation = n.Generation
		}
		if event.SessionID != "" && scope.PluginSessionID == "" {
			scope.PluginSessionID = event.SessionID
		}
	}
	now := time.Now().UTC()
	scope.UpdatedAt = now
	// Cache the route/context selected above. For cold-start scopes this records
	// only authenticated runtime identity; an explicit Agent context or the next
	// tool invocation can enrich it later.
	s.sessions.Bind(scope)
	if event.OccurredAt.IsZero() {
		event.OccurredAt = n.ReceivedAt
	}

	// Duplicate/coalescing identity includes the plugin event ID. Distinct game
	// events of the same type may legitimately occur within milliseconds and
	// must never be silently collapsed by a generic host-level policy.
	key := string(n.PluginID) + "\x00" + string(n.RuntimeID) + "\x00" + string(n.ServiceID) + "\x00" + fmt.Sprint(n.Generation) + "\x00" + event.SessionID + "\x00" + event.Type + "\x00" + event.ID
	s.mu.Lock()
	s.pruneRateStateLocked(now)
	if prev := s.last[key]; !prev.IsZero() && now.Sub(prev) < s.minGap {
		s.mu.Unlock()
		return nil
	}
	s.last[key] = now
	s.enforceRateCapacityLocked()
	s.mu.Unlock()

	req := AgentWakeRequest{Scope: scope, Event: event}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.queue <- req:
		return nil
	default:
		return ErrAgentEventQueueFull
	}
}

func (s *AgentEventSink) pruneRateStateLocked(now time.Time) {
	if s.rateEntryTTL <= 0 {
		return
	}
	cutoff := now.Add(-s.rateEntryTTL)
	for key, at := range s.last {
		if at.Before(cutoff) {
			delete(s.last, key)
		}
	}
}

func (s *AgentEventSink) enforceRateCapacityLocked() {
	if s.rateMaxEntries <= 0 || len(s.last) <= s.rateMaxEntries {
		return
	}
	type entry struct {
		key string
		at  time.Time
	}
	entries := make([]entry, 0, len(s.last))
	for key, at := range s.last {
		entries = append(entries, entry{key: key, at: at})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].at.Equal(entries[j].at) {
			return entries[i].key < entries[j].key
		}
		return entries[i].at.Before(entries[j].at)
	})
	remove := len(entries) - s.rateMaxEntries
	for i := 0; i < remove; i++ {
		delete(s.last, entries[i].key)
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
