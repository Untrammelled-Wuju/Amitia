package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestKernelEventAdapter_Publish_NoPublisher(t *testing.T) {
	adapter := NewKernelEventAdapter(NewStaticEventTypeProvider("t", 1), nil)
	err := adapter.Publish(context.Background(), Notification{})
	if err == nil {
		t.Fatal("expected error when publisher is nil")
	}
}

func TestKernelEventAdapter_Publish_CustomType(t *testing.T) {
	provider := NewStaticEventTypeProvider("gamehost.notification", 1)
	provider.WithOverride("minecraft.player.moved", "game.player.event")

	var capturedTypeID string
	var capturedVersion int
	var capturedOpts map[string]json.RawMessage

	publisher := &fakeEventPublisher{
		publish: func(ctx context.Context, typeID string, version int, payload json.RawMessage, opts map[string]json.RawMessage) error {
			capturedTypeID = typeID
			capturedVersion = version
			capturedOpts = opts
			return nil
		},
	}

	adapter := NewKernelEventAdapter(provider, publisher)
	err := adapter.Publish(context.Background(), Notification{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
		Method:    "minecraft.player.moved",
		Payload:   json.RawMessage(`{"x":1}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedTypeID != "game.player.event" {
		t.Errorf("expected game.player.event, got %s", capturedTypeID)
	}
	if capturedVersion != 1 {
		t.Errorf("expected version 1, got %d", capturedVersion)
	}
	if capturedOpts == nil {
		t.Fatal("expected opts")
	}
	if string(capturedOpts["method"]) != `"minecraft.player.moved"` {
		t.Errorf("expected method in opts, got %s", capturedOpts["method"])
	}
	if string(capturedOpts["producerId"]) != `"plugin"` {
		t.Errorf("expected producerId plugin, got %s", capturedOpts["producerId"])
	}
	if string(capturedOpts["producerRuntimeId"]) != `"runtime"` {
		t.Errorf("expected producerRuntimeId runtime, got %s", capturedOpts["producerRuntimeId"])
	}
	if string(capturedOpts["producerServiceId"]) != `"svc"` {
		t.Errorf("expected producerServiceId svc, got %s", capturedOpts["producerServiceId"])
	}
	if string(capturedOpts["producerType"]) != `"gamehost_plugin"` {
		t.Errorf("expected producerType gamehost_plugin, got %s", capturedOpts["producerType"])
	}
}

func TestKernelEventAdapter_Publish_DefaultType(t *testing.T) {
	provider := NewStaticEventTypeProvider("gamehost.notification", 2)

	var capturedTypeID string
	publisher := &fakeEventPublisher{
		publish: func(ctx context.Context, typeID string, version int, payload json.RawMessage, opts map[string]json.RawMessage) error {
			capturedTypeID = typeID
			return nil
		},
	}

	adapter := NewKernelEventAdapter(provider, publisher)
	err := adapter.Publish(context.Background(), Notification{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
		Method:    "vendor.custom.event",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedTypeID != "gamehost.notification" {
		t.Errorf("expected gamehost.notification, got %s", capturedTypeID)
	}
}

func TestKernelEventAdapter_Publish_PublisherError(t *testing.T) {
	provider := NewStaticEventTypeProvider("t", 1)
	expectedErr := errors.New("publish failed")
	publisher := &fakeEventPublisher{
		publish: func(ctx context.Context, typeID string, version int, payload json.RawMessage, opts map[string]json.RawMessage) error {
			return expectedErr
		},
	}

	adapter := NewKernelEventAdapter(provider, publisher)
	err := adapter.Publish(context.Background(), Notification{PluginID: "p"})
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestStaticEventTypeProvider_Override(t *testing.T) {
	provider := NewStaticEventTypeProvider("default", 1).WithOverride("custom.a", "custom.type.id")

	typeID, _ := provider.EventTypeID("custom.a")
	if typeID != "custom.type.id" {
		t.Errorf("expected custom.type.id, got %s", typeID)
	}

	typeID, _ = provider.EventTypeID("other.method")
	if typeID != "default" {
		t.Errorf("expected default, got %s", typeID)
	}

	typeID, version := provider.DefaultEventTypeID()
	if typeID != "default" {
		t.Errorf("expected default, got %s", typeID)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}
}

func TestBuildGameHostProvider(t *testing.T) {
	provider := BuildGameHostProvider()
	typeID, version := provider.DefaultEventTypeID()
	if typeID != "gamehost.notification" {
		t.Errorf("expected gamehost.notification, got %s", typeID)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}
}

func TestKernelEventAdapter_MergedMetadata(t *testing.T) {
	provider := NewStaticEventTypeProvider("t", 1)

	var capturedOpts map[string]json.RawMessage
	publisher := &fakeEventPublisher{
		publish: func(ctx context.Context, typeID string, version int, payload json.RawMessage, opts map[string]json.RawMessage) error {
			capturedOpts = opts
			return nil
		},
	}

	adapter := NewKernelEventAdapter(provider, publisher)
	metadata := map[string]json.RawMessage{
		"extra": []byte(`"added"`),
	}
	err := adapter.Publish(context.Background(), Notification{PluginID: "p", Metadata: metadata})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(capturedOpts["extra"]) != `"added"` {
		t.Error("expected extra merged from metadata")
	}
	if string(capturedOpts["producerId"]) != `"p"` {
		t.Error("expected producerId present")
	}
}

type fakeEventPublisher struct {
	publish func(ctx context.Context, typeID string, version int, payload json.RawMessage, opts map[string]json.RawMessage) error
}

func (f *fakeEventPublisher) Publish(ctx context.Context, typeID string, version int, payload json.RawMessage, opts map[string]json.RawMessage) error {
	return f.publish(ctx, typeID, version, payload, opts)
}

func _unused_imports() {
	fmt.Println("noop")
}
