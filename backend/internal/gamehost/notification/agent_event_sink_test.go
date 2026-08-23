package notification

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/agentbridge"
	"github.com/u-ai/backend/internal/gamehost/domain"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type captureAgentWakePort struct{ ch chan AgentWakeRequest }

func (p *captureAgentWakePort) WakeGameAgent(ctx context.Context, request AgentWakeRequest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.ch <- request:
		return nil
	}
}

func makeGameEventNotification(t *testing.T, pluginID domain.PluginID, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, event gameprotocol.GameEvent, metadata map[string]any) Notification {
	t.Helper()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(map[string]any{
		"channelId": event.SessionID,
		"eventId":   GameEventPublishID,
		"payload":   json.RawMessage(payload),
		"metadata":  metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return Notification{PluginID: pluginID, RuntimeID: runtimeID, ServiceID: serviceID, Method: "plugin.event.publish", Payload: envelope, ReceivedAt: time.Now().UTC()}
}

func TestAgentEventSinkWakesBoundGameSession(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{
		GameSessionID:  "game-1",
		PluginID:       "plugin-1",
		RuntimeID:      "runtime-1",
		ServiceID:      "service-1",
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
	})
	sink := NewAgentEventSink(sessions)
	port := &captureAgentWakePort{ch: make(chan AgentWakeRequest, 1)}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()

	n := makeGameEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.GameEvent{ID: "evt-1", SessionID: "game-1", Type: "hostile.nearby"}, nil)
	if err := sink.Publish(context.Background(), n); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	select {
	case req := <-port.ch:
		if req.Scope.CharacterID != "char-1" || req.Event.ID != "evt-1" {
			t.Fatalf("unexpected wake request: %+v", req)
		}
	case <-time.After(time.Second):
		t.Fatal("agent wake request was not delivered")
	}
}

func TestAgentEventSinkRejectsRouteMismatch(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{GameSessionID: "game-1", PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1"})
	sink := NewAgentEventSink(sessions)
	n := makeGameEventNotification(t, "plugin-other", "runtime-1", "service-1", gameprotocol.GameEvent{ID: "evt-1", SessionID: "game-1", Type: "damage"}, nil)
	if err := sink.Publish(context.Background(), n); err == nil {
		t.Fatal("Publish() error = nil, want route mismatch error")
	}
}

func TestAgentEventSinkRespectsWakeAgentFalse(t *testing.T) {
	sessions := agentbridge.NewSessionRegistry()
	sessions.Bind(agentbridge.SessionScope{GameSessionID: "game-1", PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1"})
	sink := NewAgentEventSink(sessions)
	port := &captureAgentWakePort{ch: make(chan AgentWakeRequest, 1)}
	sink.SetPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)
	defer sink.Shutdown()

	n := makeGameEventNotification(t, "plugin-1", "runtime-1", "service-1", gameprotocol.GameEvent{ID: "evt-1", SessionID: "game-1", Type: "ambient"}, map[string]any{"wakeAgent": false})
	if err := sink.Publish(context.Background(), n); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	select {
	case <-port.ch:
		t.Fatal("wakeAgent=false should suppress real-time wake")
	case <-time.After(50 * time.Millisecond):
	}
}
