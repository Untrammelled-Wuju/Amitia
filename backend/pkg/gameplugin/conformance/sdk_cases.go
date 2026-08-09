package conformance

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
	sdk "github.com/u-ai/backend/pkg/gameplugin/sdk/go"
)

type MockTransport struct {
	sent     []protocol.Envelope
	receiveC chan protocol.Envelope
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		sent:     make([]protocol.Envelope, 0),
		receiveC: make(chan protocol.Envelope, 100),
	}
}

func (m *MockTransport) Send(ctx context.Context, message protocol.Envelope) error {
	m.sent = append(m.sent, message)
	return nil
}

func (m *MockTransport) Receive(ctx context.Context) (protocol.Envelope, error) {
	select {
	case msg := <-m.receiveC:
		return msg, nil
	case <-ctx.Done():
		return protocol.Envelope{}, ctx.Err()
	}
}

func (m *MockTransport) Close() error {
	return nil
}

func (m *MockTransport) GetSent() []protocol.Envelope {
	return m.sent
}

func (m *MockTransport) Queue(msg protocol.Envelope) {
	m.receiveC <- msg
}

type FixedIDGen struct {
	ids     []string
	current int
}

func NewFixedIDGen(ids ...string) *FixedIDGen {
	return &FixedIDGen{ids: ids, current: 0}
}

func (g *FixedIDGen) NewID() string {
	if g.current >= len(g.ids) {
		return "msg-overflow"
	}
	id := g.ids[g.current]
	g.current++
	return id
}

func SDKConformanceCases() []Case {
	cases := make([]Case, 0)

	cases = append(cases, Case{
		Name:          "sdk_build_valid_request",
		Description:   "Go SDK should build valid request",
		ExpectedValid: true,
		Validator:     &SDKBuildValidator{action: "build_request"},
	})

	cases = append(cases, Case{
		Name:          "sdk_build_valid_response",
		Description:   "Go SDK should build valid response",
		ExpectedValid: true,
		Validator:     &SDKBuildValidator{action: "build_response"},
	})

	cases = append(cases, Case{
		Name:          "sdk_build_valid_notification",
		Description:   "Go SDK should build valid notification",
		ExpectedValid: true,
		Validator:     &SDKBuildValidator{action: "build_notification"},
	})

	cases = append(cases, Case{
		Name:          "sdk_build_valid_error",
		Description:   "Go SDK should build valid error",
		ExpectedValid: true,
		Validator:     &SDKBuildValidator{action: "build_error"},
	})

	cases = append(cases, Case{
		Name:          "sdk_build_method_association",
		Description:   "Go SDK response should reference original request id",
		ExpectedValid: true,
		Validator:     &SDKBuildValidator{action: "check_association"},
	})

	cases = append(cases, Case{
		Name:          "sdk_encode_decode_roundtrip",
		Description:   "Go SDK encoded message should decode correctly",
		ExpectedValid: true,
		Validator:     &SDKBuildValidator{action: "roundtrip"},
	})

	return cases
}

type SDKBuildValidator struct {
	action string
}

func (v *SDKBuildValidator) Name() string {
	return "sdk_build_validator"
}

func (v *SDKBuildValidator) Validate(data []byte) error {
	switch v.action {
	case "build_request":
		return v.validateBuildRequest()
	case "build_response":
		return v.validateBuildResponse()
	case "build_notification":
		return v.validateBuildNotification()
	case "build_error":
		return v.validateBuildError()
	case "check_association":
		return v.validateAssociation()
	case "roundtrip":
		return v.validateRoundtrip()
	}
	return nil
}

func (v *SDKBuildValidator) validateBuildRequest() error {
	transport := NewMockTransport()
	idGen := NewFixedIDGen("req-001")
	client := sdk.NewClient(transport, sdk.WithIDGenerator(idGen))

	payload := map[string]any{"goal": "build shelter"}
	env, err := client.NewRequest("vendor.agent.execute", payload)
	if err != nil {
		return err
	}
	if env.Protocol != protocol.ProtocolVersion {
		return &ValidationError{Message: "wrong protocol"}
	}
	if env.Type != protocol.MessageTypeRequest {
		return &ValidationError{Message: "wrong type"}
	}
	if env.ID != "req-001" {
		return &ValidationError{Message: "wrong id"}
	}
	if env.Method != "vendor.agent.execute" {
		return &ValidationError{Message: "wrong method"}
	}
	return nil
}

func (v *SDKBuildValidator) validateBuildResponse() error {
	transport := NewMockTransport()
	idGen := NewFixedIDGen("req-001", "res-001")
	client := sdk.NewClient(transport, sdk.WithIDGenerator(idGen))

	request, err := client.NewRequest("vendor.agent.execute", nil)
	if err != nil {
		return err
	}
	response, err := client.NewResponse(request, map[string]any{"ok": true})
	if err != nil {
		return err
	}
	if response.RequestID != request.ID {
		return &ValidationError{Message: "requestId mismatch"}
	}
	return nil
}

func (v *SDKBuildValidator) validateBuildNotification() error {
	transport := NewMockTransport()
	idGen := NewFixedIDGen("note-001")
	client := sdk.NewClient(transport, sdk.WithIDGenerator(idGen))

	env, err := client.NewNotification("vendor.state.changed", map[string]any{"state": "x"})
	if err != nil {
		return err
	}
	if env.Type != protocol.MessageTypeNotification {
		return &ValidationError{Message: "wrong type"}
	}
	if env.RequestID != "" {
		return &ValidationError{Message: "notification should not have requestId"}
	}
	return nil
}

func (v *SDKBuildValidator) validateBuildError() error {
	transport := NewMockTransport()
	idGen := NewFixedIDGen("req-001", "err-001")
	client := sdk.NewClient(transport, sdk.WithIDGenerator(idGen))

	request, err := client.NewRequest("vendor.test", nil)
	if err != nil {
		return err
	}
	env, err := client.NewError(request, protocol.ErrorRuntimeUnavailable, "unavailable", true, nil)
	if err != nil {
		return err
	}
	if env.Error == nil || env.Error.Code != protocol.ErrorRuntimeUnavailable {
		return &ValidationError{Message: "wrong error code"}
	}
	return nil
}

func (v *SDKBuildValidator) validateAssociation() error {
	transport := NewMockTransport()
	idGen := NewFixedIDGen("req-001", "res-001")
	client := sdk.NewClient(transport, sdk.WithIDGenerator(idGen))

	request, _ := client.NewRequest("vendor.test", nil)
	response, err := client.NewResponse(request, nil)
	if err != nil {
		return err
	}
	if response.RequestID != request.ID {
		return &ValidationError{Message: "request-response association failed"}
	}
	return nil
}

func (v *SDKBuildValidator) validateRoundtrip() error {
	transport := NewMockTransport()
	idGen := NewFixedIDGen("req-001")
	client := sdk.NewClient(transport, sdk.WithIDGenerator(idGen))

	original := map[string]any{
		"nested": map[string]any{"value": float64(123)},
	}
	env, err := client.NewRequest("vendor.test", original)
	if err != nil {
		return err
	}

	data, err := json.Marshal(env)
	if err != nil {
		return err
	}

	var decoded protocol.Envelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	if err := decoded.Validate(); err != nil {
		return err
	}

	return nil
}

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
