package notification

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/gamehost/agentbridge"
	"github.com/u-ai/backend/internal/gamehost/domain"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
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
	bound, ok := sessions.Resolve("runtime-1", "")
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
