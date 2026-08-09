package sdk

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func loadFixture(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse fixture %s: %v", path, err)
	}
	return result
}

func TestWireCompatibleRequest(t *testing.T) {
	fixture := loadFixture(t, "../testdata/request.json")

	if fixture["protocol"] != protocol.ProtocolVersion {
		t.Fatalf("expected protocol '%s', got '%v'", protocol.ProtocolVersion, fixture["protocol"])
	}
	if fixture["type"] != string(protocol.MessageTypeRequest) {
		t.Fatalf("expected type 'request', got '%v'", fixture["type"])
	}
	if fixture["method"] != "minecraft.agent.submit_goal" {
		t.Fatalf("expected method 'minecraft.agent.submit_goal', got '%v'", fixture["method"])
	}

	payload, ok := fixture["payload"].(map[string]any)
	if !ok {
		t.Fatal("expected payload to be a map")
	}
	if payload["goal"] != "build a shelter" {
		t.Fatalf("expected payload.goal 'build a shelter', got '%v'", payload["goal"])
	}
}

func TestWireCompatibleResponse(t *testing.T) {
	fixture := loadFixture(t, "../testdata/response.json")

	if fixture["protocol"] != protocol.ProtocolVersion {
		t.Fatalf("expected protocol '%s', got '%v'", protocol.ProtocolVersion, fixture["protocol"])
	}
	if fixture["type"] != string(protocol.MessageTypeResponse) {
		t.Fatalf("expected type 'response', got '%v'", fixture["type"])
	}
	if fixture["requestId"] != "msg-001" {
		t.Fatalf("expected requestId 'msg-001', got '%v'", fixture["requestId"])
	}
}

func TestWireCompatibleNotification(t *testing.T) {
	fixture := loadFixture(t, "../testdata/notification.json")

	if fixture["protocol"] != protocol.ProtocolVersion {
		t.Fatalf("expected protocol '%s', got '%v'", protocol.ProtocolVersion, fixture["protocol"])
	}
	if fixture["type"] != string(protocol.MessageTypeNotification) {
		t.Fatalf("expected type 'notification', got '%v'", fixture["type"])
	}
	if fixture["method"] != "vendor.runtime.state_changed" {
		t.Fatalf("expected method 'vendor.runtime.state_changed', got '%v'", fixture["method"])
	}

	if _, hasRequestId := fixture["requestId"]; hasRequestId {
		t.Fatal("notification should not have requestId")
	}
}

func TestWireCompatibleError(t *testing.T) {
	fixture := loadFixture(t, "../testdata/error.json")

	if fixture["protocol"] != protocol.ProtocolVersion {
		t.Fatalf("expected protocol '%s', got '%v'", protocol.ProtocolVersion, fixture["protocol"])
	}
	if fixture["type"] != string(protocol.MessageTypeError) {
		t.Fatalf("expected type 'error', got '%v'", fixture["type"])
	}

	errorObj, ok := fixture["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error to be a map")
	}
	if errorObj["code"] != "resource_exhausted" {
		t.Fatalf("expected error.code 'resource_exhausted', got '%v'", errorObj["code"])
	}
	if errorObj["message"] != "channel quota exceeded" {
		t.Fatalf("expected error.message 'channel quota exceeded', got '%v'", errorObj["message"])
	}
}

func TestWireCompatibleDescriptor(t *testing.T) {
	fixture := loadFixture(t, "../testdata/descriptor.json")

	if fixture["protocolVersion"] != protocol.ProtocolVersion {
		t.Fatalf("expected protocolVersion '%s', got '%v'", protocol.ProtocolVersion, fixture["protocolVersion"])
	}

	services, ok := fixture["services"].([]any)
	if !ok {
		t.Fatal("expected services to be an array")
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	caps, ok := fixture["capabilities"].([]any)
	if !ok {
		t.Fatal("expected capabilities to be an array")
	}
	if len(caps) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(caps))
	}
}
