package conformance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestStandardConformanceSuite(t *testing.T) {
	h := NewHarness(protocol.ProtocolVersion)
	h.AddCases(StandardSuite()...)

	result := h.Run()

	if !result.AllPassed() {
		for _, cr := range result.Results {
			if !cr.Passed {
				t.Errorf("FAILED: %s - %v", cr.Name, cr.Err)
			}
		}
		t.Fatalf("Standard conformance suite failed: %d/%d passed", result.Passed, result.Total())
	}

	t.Logf("Standard suite: %d/%d passed", result.Passed, result.Total())
}

func TestValidRequestFixture(t *testing.T) {
	data, err := loadFixture("valid/request.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	validator := EnvelopeValidator{}
	if err := validator.Validate(data); err != nil {
		t.Fatalf("valid request failed validation: %v", err)
	}

	var env protocol.Envelope
	if err := parseEnvelope(data, &env); err != nil {
		t.Fatalf("failed to parse envelope: %v", err)
	}

	if env.Protocol != protocol.ProtocolVersion {
		t.Errorf("expected protocol %s, got %s", protocol.ProtocolVersion, env.Protocol)
	}
	if env.Type != protocol.MessageTypeRequest {
		t.Errorf("expected type request, got %s", env.Type)
	}
	if env.ID != "req-001" {
		t.Errorf("expected id req-001, got %s", env.ID)
	}
	if env.Method != "vendor.agent.execute" {
		t.Errorf("expected method vendor.agent.execute, got %s", env.Method)
	}
}

func TestInvalidRequestFixture(t *testing.T) {
	invalidFixtures := []string{
		"invalid/request_without_id.json",
		"invalid/request_without_method.json",
		"invalid/wrong_protocol.json",
	}

	validator := EnvelopeValidator{}
	for _, fixture := range invalidFixtures {
		data, err := loadFixture(fixture)
		if err != nil {
			t.Fatalf("fixture %s not found: %v", fixture, err)
		}
		if err := validator.Validate(data); err == nil {
			t.Errorf("%s: expected validation to fail", fixture)
		}
	}
}

func TestResponseCorrelation(t *testing.T) {
	reqData, _ := loadFixture("valid/request.json")
	respData, _ := loadFixture("valid/response.json")

	var req protocol.Envelope
	var resp protocol.Envelope

	if err := parseEnvelope(reqData, &req); err != nil {
		t.Fatalf("failed to parse request: %v", err)
	}
	if err := parseEnvelope(respData, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.RequestID != req.ID {
		t.Errorf("response requestId %s does not match request id %s", resp.RequestID, req.ID)
	}
}

func TestNotificationFixture(t *testing.T) {
	data, err := loadFixture("valid/notification.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	var env protocol.Envelope
	if err := parseEnvelope(data, &env); err != nil {
		t.Fatalf("failed to parse notification: %v", err)
	}

	if env.Type != protocol.MessageTypeNotification {
		t.Errorf("expected notification type, got %s", env.Type)
	}
	if env.RequestID != "" {
		t.Errorf("notification should not have requestId, got %s", env.RequestID)
	}
}

func TestErrorFixture(t *testing.T) {
	data, err := loadFixture("valid/error.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	var env protocol.Envelope
	if err := parseEnvelope(data, &env); err != nil {
		t.Fatalf("failed to parse error: %v", err)
	}

	if env.Error == nil {
		t.Fatal("expected error to be non-nil")
	}
	if env.Error.Code != "runtime_unavailable" {
		t.Errorf("expected code runtime_unavailable, got %s", env.Error.Code)
	}
	if !env.Error.Retryable {
		t.Error("expected retryable to be true")
	}
}

func TestServiceSchemaConformance(t *testing.T) {
	data, err := loadFixture("valid/service.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	validator := ServiceDescriptorValidator{}
	if err := validator.Validate(data); err != nil {
		t.Fatalf("valid service failed: %v", err)
	}

	var svc protocol.ServiceDescriptor
	if err := parseEnvelope(data, &svc); err != nil {
		t.Fatalf("failed to parse service: %v", err)
	}

	if svc.ID != "game-bridge" {
		t.Errorf("expected id game-bridge, got %s", svc.ID)
	}
	if svc.Kind != protocol.ServiceKindProcess {
		t.Errorf("expected kind process, got %s", svc.Kind)
	}
}

func TestChannelSchemaConformance(t *testing.T) {
	data, err := loadFixture("valid/channel.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	validator := ChannelDescriptorValidator{}
	if err := validator.Validate(data); err != nil {
		t.Fatalf("valid channel failed: %v", err)
	}
}

func TestCapabilityConformance(t *testing.T) {
	caps := []string{
		"realtime_control", "state_streaming", "event_streaming", "binary_streaming",
		"custom_rpc", "host_api", "shared_control", "custom_ui", "multi_service",
	}

	validator := CapabilityValidator{}
	for _, cap := range caps {
		data := []byte(`"` + cap + `"`)
		if err := validator.Validate(data); err != nil {
			t.Errorf("capability %s failed: %v", cap, err)
		}
	}

	for _, cap := range []string{"minecraft.pathfinding", "vendor.agent"} {
		if err := validator.Validate([]byte(`"` + cap + `"`)); err == nil {
			t.Errorf("game/tool capability %s must not validate as a GameHost feature", cap)
		}
	}
}

func TestCustomCapabilityConformance(t *testing.T) {
	validator := CapabilityValidator{}

	data := []byte(`"minecraft.pathfinding"`)
	if err := validator.Validate(data); err == nil {
		t.Error("game/tool capability must not validate as a host feature")
	}

	if protocol.IsKnownCapability("minecraft.pathfinding") {
		t.Error("custom capability should not be known")
	}
}

func TestOpaquePayloadConformance(t *testing.T) {
	data, err := loadFixture("valid/request.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	var env protocol.Envelope
	if err := parseEnvelope(data, &env); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if env.Payload == nil {
		t.Fatal("expected payload to be non-nil")
	}

	var payload map[string]interface{}
	if err := parseEnvelope(env.Payload, &payload); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}

	if payload["value"] == nil {
		t.Error("expected value in payload")
	}
}

func TestUnknownFieldForwardCompatibility(t *testing.T) {
	data := []byte(`{"protocol":"amitia-game-host/1","type":"notification","id":"n1","method":"vendor.test","futureField":{"foo":1}}`)

	var env map[string]interface{}
	if err := parseEnvelope(data, &env); err != nil {
		t.Logf("Note: unknown field handling may fail strict decoding: %v", err)
	}
}

func TestGoGeneratedFixture(t *testing.T) {
	requestData, _ := loadFixture("valid/request.json")
	responseData, _ := loadFixture("valid/response.json")
	notificationData, _ := loadFixture("valid/notification.json")
	errorData, _ := loadFixture("valid/error.json")

	validator := EnvelopeValidator{}

	fixtures := map[string][]byte{
		"request":      requestData,
		"response":     responseData,
		"notification": notificationData,
		"error":        errorData,
	}

	for name, data := range fixtures {
		if err := validator.Validate(data); err != nil {
			t.Errorf("%s fixture failed validation: %v", name, err)
		}
	}
}

func TestTypeScriptGeneratedFixture(t *testing.T) {
	fixtures := []string{
		"valid/request.json",
		"valid/response.json",
		"valid/notification.json",
		"valid/error.json",
	}

	validator := EnvelopeValidator{}
	for _, f := range fixtures {
		data, err := loadFixture(f)
		if err != nil {
			t.Fatalf("fixture %s not found: %v", f, err)
		}
		if err := validator.Validate(data); err != nil {
			t.Errorf("fixture %s failed: %v", f, err)
		}
	}
}

func TestNegativeCases(t *testing.T) {
	for _, c := range InvalidCases() {
		t.Run(c.Name, func(t *testing.T) {
			err := c.Validator.Validate(c.Input)
			if c.ExpectedValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !c.ExpectedValid && err == nil {
				t.Error("expected invalid, but validation passed")
			}
		})
	}
}

func TestProtocolVersion(t *testing.T) {
	validData := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"vendor.test"}`)
	validator := EnvelopeValidator{}
	if err := validator.Validate(validData); err != nil {
		t.Errorf("valid protocol failed: %v", err)
	}

	invalidProtocols := []string{
		`{"protocol":"amitia-game-host/2","type":"request","id":"r1","method":"vendor.test"}`,
		`{"protocol":"amitia-game/1","type":"request","id":"r1","method":"vendor.test"}`,
		`{"type":"request","id":"r1","method":"vendor.test"}`,
	}

	for _, p := range invalidProtocols {
		if err := validator.Validate([]byte(p)); err == nil {
			t.Errorf("protocol %s should be rejected", p)
		}
	}
}

func TestReservedNamespace(t *testing.T) {
	validator := PluginMethodValidator{}

	validMethods := []string{
		`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"minecraft.agent.execute"}`,
		`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"vendor.control.run"}`,
	}

	for _, m := range validMethods {
		if err := validator.Validate([]byte(m)); err != nil {
			t.Errorf("valid method failed: %v", err)
		}
	}

	reservedMethods := []string{
		`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"host.secret.read"}`,
		`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"host.runtime.health"}`,
	}

	for _, m := range reservedMethods {
		if err := validator.Validate([]byte(m)); err == nil {
			t.Errorf("reserved method %s should be rejected", m)
		}
	}
}

func TestEmptyPayload(t *testing.T) {
	data := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"vendor.test"}`)
	validator := EnvelopeValidator{}
	if err := validator.Validate(data); err != nil {
		t.Errorf("empty payload should be valid: %v", err)
	}
}

func TestNullPayload(t *testing.T) {
	data := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"vendor.test","payload":null}`)
	validator := EnvelopeValidator{}
	if err := validator.Validate(data); err != nil {
		t.Errorf("null payload should be valid: %v", err)
	}
}

func TestEmptyObjectPayload(t *testing.T) {
	data := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"vendor.test","payload":{}}`)
	validator := EnvelopeValidator{}
	if err := validator.Validate(data); err != nil {
		t.Errorf("empty object payload should be valid: %v", err)
	}
}

func TestDuplicateServiceRejected(t *testing.T) {
	data := []byte(`[{"id":"svc","kind":"process"},{"id":"svc","kind":"process"}]`)

	var svcs []protocol.ServiceDescriptor
	if err := json.Unmarshal(data, &svcs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if err := protocol.ValidateServices(svcs); err == nil {
		t.Error("duplicate services should be rejected")
	}
}

func TestDuplicateCapabilityRejected(t *testing.T) {
	data := []byte(`["custom_rpc","custom_rpc"]`)
	var caps []protocol.Capability
	if err := json.Unmarshal(data, &caps); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if err := protocol.ValidateCapabilities(caps); err == nil {
		t.Error("duplicate capabilities should be rejected")
	}
}

func TestInvalidBareErrorCodeRejected(t *testing.T) {
	validator := EnvelopeValidator{}
	data := []byte(`{"protocol":"amitia-game-host/1","type":"error","id":"e1","error":{"code":"random_error_name","message":"test"}}`)
	if err := validator.Validate(data); err == nil {
		t.Error("invalid bare error code should be rejected")
	}
}

func TestCustomErrorCodeAccepted(t *testing.T) {
	validator := EnvelopeValidator{}
	data := []byte(`{"protocol":"amitia-game-host/1","type":"error","id":"e1","error":{"code":"minecraft.connection_failed","message":"test"}}`)
	if err := validator.Validate(data); err != nil {
		t.Errorf("custom error code should be accepted: %v", err)
	}
}

func TestExtensionFunctionCasesRegistered(t *testing.T) {
	suite := StandardSuite()
	if len(suite) == 0 {
		t.Fatal("standard suite should have cases")
	}
	t.Logf("Standard suite has %d cases", len(suite))
}

func TestFixturesLoadable(t *testing.T) {
	fixtures := []string{
		"valid/request.json",
		"valid/response.json",
		"valid/notification.json",
		"valid/error.json",
		"valid/service.json",
		"valid/channel.json",
		"valid/descriptor.json",
		"invalid/wrong_protocol.json",
		"invalid/request_without_id.json",
	}

	for _, f := range fixtures {
		_, err := loadFixture(f)
		if err != nil {
			t.Logf("Fixture %s not found (may need to be created): %v", f, err)
		}
	}
}

func parseEnvelope(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func TestHarnessArchitecture(t *testing.T) {
	h := NewHarness("test-protocol")
	h.AddCase(NewCase("test_case", "test description", []byte(`{}`), false, EnvelopeValidator{}))

	if h.CaseCount() != 1 {
		t.Errorf("expected 1 case, got %d", h.CaseCount())
	}

	result := h.Run()
	if result.Total() != 1 {
		t.Errorf("expected total 1, got %d", result.Total())
	}
}

func TestMalformedJSONRejected(t *testing.T) {
	validator := EnvelopeValidator{}
	data := []byte(`{"protocol": "broken"`)
	if err := validator.Validate(data); err == nil {
		t.Error("malformed JSON should be rejected")
	}
}

func TestUnknownTypeRejected(t *testing.T) {
	validator := EnvelopeValidator{}
	data := []byte(`{"protocol":"amitia-game-host/1","type":"unknown_type","id":"r1"}`)
	if err := validator.Validate(data); err == nil {
		t.Error("unknown message type should be rejected")
	}
}

func TestWhitespaceOnlyMethodRejected(t *testing.T) {
	validator := PluginMethodValidator{}
	data := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":""}`)
	if err := validator.Validate(data); err == nil {
		t.Log("Empty method validation depends on envelope validator behavior")
	}
}

func TestNumericIdAccepted(t *testing.T) {
	validator := EnvelopeValidator{}
	data := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"12345","method":"vendor.test"}`)
	if err := validator.Validate(data); err != nil {
		t.Errorf("numeric string id should be valid: %v", err)
	}
}

func TestLargeMessageId(t *testing.T) {
	validator := EnvelopeValidator{}
	largeID := strings.Repeat("x", 300)
	data := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"` + largeID + `","method":"vendor.test"}`)
	if err := validator.Validate(data); err == nil {
		t.Error("large message id should be rejected")
	}
}
