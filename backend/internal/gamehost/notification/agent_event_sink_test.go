package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/u-ai/backend/internal/gamehost/agentbridge"
	"github.com/u-ai/backend/internal/gamehost/domain"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
	"sync"
	"testing"
	"time"
)

type captureAgentWakePort struct{ ch chan AgentWakeRequest }

func (p *captureAgentWakePort) WakePluginAgent(ctx context.Context, request AgentWakeRequest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.ch <- request:
		return nil
	}
}
func makePluginEventNotification(t *testing.T, pluginID domain.PluginID, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, event gameprotocol.PluginEvent, metadata map[string]any) Notification {
	t.Helper()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(map[string]any{"eventId": PluginAgentEventPublishID, "payload": json.RawMessage(payload), "metadata": metadata})
	if err != nil {
		t.Fatal(err)
	}
	return Notification{PluginID: pluginID, RuntimeID: runtimeID, ServiceID: serviceID, Generation: 1, Method: "plugin.event.publish", Payload: envelope, ReceivedAt: time.Now().UTC()}
}
func TestAgentEventSinkWakesBoundRuntimeContext(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1", Generation: 1, UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"})
	sink := NewAgentEventSink(sessions)
	port := &captureAgentWakePort{ch: make(chan AgentWakeRequest, 1)}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()
	n := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt-1", Type: "vendor.hostile.nearby"}, nil)
	if err := sink.Publish(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	select {
	case req := <-port.ch:
		if req.Scope.CharacterID != "char-1" || req.Event.ID != "evt-1" {
			t.Fatalf("unexpected %+v", req)
		}
	case <-time.After(time.Second):
		t.Fatal("wake not delivered")
	}
}
func TestAgentEventSinkRejectsRouteMismatch(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1", Generation: 1})
	sink := NewAgentEventSink(sessions)
	n := makePluginEventNotification(t, "other", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt", Type: "vendor.event"}, nil)
	if err := sink.Publish(context.Background(), n); err == nil {
		t.Fatal("expected route mismatch")
	}
}
func TestAgentEventSinkRespectsWakeAgentFalse(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1", Generation: 1})
	sink := NewAgentEventSink(sessions)
	port := &captureAgentWakePort{ch: make(chan AgentWakeRequest, 1)}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()
	n := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt", Type: "vendor.event"}, map[string]any{"wakeAgent": false})
	if err := sink.Publish(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	select {
	case <-port.ch:
		t.Fatal("wakeAgent=false woke agent")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAgentEventSinkRejectsStaleGeneration(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1", Generation: 2})
	sink := NewAgentEventSink(sessions)
	n := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt", Type: "vendor.event"}, nil)
	if err := sink.Publish(context.Background(), n); err == nil {
		t.Fatal("expected generation mismatch")
	}
}

func TestAgentEventSinkColdStartWakesWithoutPriorToolInvocation(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sink := NewAgentEventSink(sessions)
	port := &captureAgentWakePort{ch: make(chan AgentWakeRequest, 1)}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()

	n := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt-cold", SessionID: "plugin-session", Type: "vendor.ready"}, nil)
	if err := sink.Publish(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	select {
	case req := <-port.ch:
		if req.Scope.PluginID != "plugin-1" || req.Scope.RuntimeID != "runtime-1" || req.Scope.ServiceID != "service-1" {
			t.Fatalf("unexpected route scope: %+v", req.Scope)
		}
		if req.Scope.PluginSessionID != "plugin-session" {
			t.Fatalf("expected plugin session to be preserved, got %+v", req.Scope)
		}
	case <-time.After(time.Second):
		t.Fatal("cold-start event was not delivered")
	}
}

func TestAgentEventSinkCarriesContextForwardAcrossNewGeneration(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1", Generation: 1, CharacterID: "char-1", ConversationID: "conv-1"})
	sink := NewAgentEventSink(sessions)
	port := &captureAgentWakePort{ch: make(chan AgentWakeRequest, 1)}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()

	n := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt-restart", Type: "vendor.runtime.ready"}, nil)
	n.Generation = 2
	if err := sink.Publish(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	select {
	case req := <-port.ch:
		if req.Scope.Generation != 2 || req.Scope.CharacterID != "char-1" || req.Scope.ConversationID != "conv-1" {
			t.Fatalf("context was not carried forward: %+v", req.Scope)
		}
	case <-time.After(time.Second):
		t.Fatal("new-generation event was not delivered")
	}
	bound, ok := sessions.Resolve("runtime-1", "service-1", "")
	if !ok || bound.Generation != 2 {
		t.Fatalf("registry generation was not refreshed: %+v ok=%v", bound, ok)
	}
}

func TestAgentEventSinkQueueBackpressureIsExplicit(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sink := NewAgentEventSink(sessions)
	sink.queue = make(chan AgentWakeRequest, 1)
	first := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt-1", Type: "vendor.one"}, nil)
	second := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt-2", Type: "vendor.two"}, nil)
	if err := sink.Publish(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := sink.Publish(context.Background(), second); !errors.Is(err, ErrAgentEventQueueFull) {
		t.Fatalf("expected ErrAgentEventQueueFull, got %v", err)
	}
}

type failingAgentWakePort struct {
	mu    sync.Mutex
	calls int
}

func (p *failingAgentWakePort) WakePluginAgent(context.Context, AgentWakeRequest) error {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	return errors.New("agent unavailable")
}

func (p *failingAgentWakePort) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func TestAgentEventSinkSeparatesSameSessionAcrossServices(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-a", Generation: 1, PluginSessionID: "player", CharacterID: "char-a"})
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-b", Generation: 1, PluginSessionID: "player", CharacterID: "char-b"})
	sink := NewAgentEventSink(sessions)
	port := &captureAgentWakePort{ch: make(chan AgentWakeRequest, 2)}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()

	for _, tc := range []struct {
		service domain.ServiceID
		eventID string
	}{
		{service: "service-a", eventID: "evt-a"},
		{service: "service-b", eventID: "evt-b"},
	} {
		n := makePluginEventNotification(t, "plugin-1", "runtime-1", tc.service, gameprotocol.PluginEvent{ID: tc.eventID, SessionID: "player", Type: "vendor.same"}, nil)
		if err := sink.Publish(context.Background(), n); err != nil {
			t.Fatal(err)
		}
	}

	got := map[domain.ServiceID]string{}
	for i := 0; i < 2; i++ {
		select {
		case req := <-port.ch:
			got[req.Scope.ServiceID] = req.Scope.CharacterID
		case <-time.After(time.Second):
			t.Fatal("wake not delivered")
		}
	}
	if got["service-a"] != "char-a" || got["service-b"] != "char-b" {
		t.Fatalf("service contexts crossed: %+v", got)
	}
}

func TestAgentEventSinkRetriesAndRecordsBoundedDeadLetter(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sink := NewAgentEventSink(sessions)
	sink.maxAttempts = 2
	sink.retryBaseDelay = time.Millisecond
	sink.deadLetterMax = 1
	port := &failingAgentWakePort{}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()

	n := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt-fail", Type: "vendor.fail"}, nil)
	if err := sink.Publish(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && len(sink.DeadLetters()) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	failures := sink.DeadLetters()
	if len(failures) != 1 || failures[0].Attempts != 2 || failures[0].Error == "" {
		t.Fatalf("unexpected dead letters: %+v", failures)
	}
	if port.Calls() != 2 {
		t.Fatalf("expected exactly 2 wake attempts, got %d", port.Calls())
	}
}

func TestAgentEventSinkRateStateIsBounded(t *testing.T) {
	sink := NewAgentEventSink(agentbridge.NewSessionRegistry())
	sink.rateMaxEntries = 2
	sink.rateEntryTTL = time.Hour
	for i := 0; i < 3; i++ {
		n := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: fmt.Sprintf("evt-%d", i), Type: fmt.Sprintf("vendor.type.%d", i)}, nil)
		if err := sink.Publish(context.Background(), n); err != nil && !errors.Is(err, ErrAgentEventQueueFull) {
			t.Fatal(err)
		}
	}
	sink.mu.RLock()
	got := len(sink.last)
	sink.mu.RUnlock()
	if got != 2 {
		t.Fatalf("expected bounded rate state size=2, got %d", got)
	}
}

func TestAgentEventSinkDoesNotCollapseDistinctSameTypeEvents(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sink := NewAgentEventSink(sessions)
	port := &captureAgentWakePort{ch: make(chan AgentWakeRequest, 2)}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()

	for _, eventID := range []string{"evt-rapid-1", "evt-rapid-2"} {
		n := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: eventID, Type: "vendor.damage"}, nil)
		if err := sink.Publish(context.Background(), n); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case req := <-port.ch:
			seen[req.Event.ID] = true
		case <-time.After(time.Second):
			t.Fatal("distinct same-type event was collapsed")
		}
	}
	if !seen["evt-rapid-1"] || !seen["evt-rapid-2"] {
		t.Fatalf("missing rapid events: %+v", seen)
	}
}

type selectiveBlockingWakePort struct {
	ch chan AgentWakeRequest
}

func (p *selectiveBlockingWakePort) WakePluginAgent(ctx context.Context, req AgentWakeRequest) error {
	if req.Event.ID == "evt-block" {
		<-ctx.Done()
		return ctx.Err()
	}
	select {
	case p.ch <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestAgentEventSinkSlowWakeDoesNotBlockIndependentEvents(t *testing.T) {
	sink := NewAgentEventSink(agentbridge.NewSessionRegistry())
	sink.attemptTimeout = 250 * time.Millisecond
	sink.maxAttempts = 1
	port := &selectiveBlockingWakePort{ch: make(chan AgentWakeRequest, 1)}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()

	blocked := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt-block", Type: "vendor.block"}, nil)
	fast := makePluginEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt-fast", Type: "vendor.fast"}, nil)
	if err := sink.Publish(context.Background(), blocked); err != nil {
		t.Fatal(err)
	}
	if err := sink.Publish(context.Background(), fast); err != nil {
		t.Fatal(err)
	}
	select {
	case req := <-port.ch:
		if req.Event.ID != "evt-fast" {
			t.Fatalf("unexpected event delivered first: %s", req.Event.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("slow wake caused head-of-line blocking")
	}
}
