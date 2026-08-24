package notification

import (
	"context"
	"encoding/json"
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
	envelope, err := json.Marshal(map[string]any{"channelId": event.SessionID, "eventId": PluginAgentEventPublishID, "payload": json.RawMessage(payload), "metadata": metadata})
	if err != nil {
		t.Fatal(err)
	}
	return Notification{PluginID: pluginID, RuntimeID: runtimeID, ServiceID: serviceID, Method: "plugin.event.publish", Payload: envelope, ReceivedAt: time.Now().UTC()}
}
func TestAgentEventSinkWakesBoundRuntimeContext(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1", UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web"})
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
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1"})
	sink := NewAgentEventSink(sessions)
	n := makePluginEventNotification(t, "other", "runtime-1", "service-1", gameprotocol.PluginEvent{ID: "evt", Type: "vendor.event"}, nil)
	if err := sink.Publish(context.Background(), n); err == nil {
		t.Fatal("expected route mismatch")
	}
}
func TestAgentEventSinkRespectsWakeAgentFalse(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1"})
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
