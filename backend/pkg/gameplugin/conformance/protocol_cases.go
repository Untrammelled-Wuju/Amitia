package conformance

import (
	"encoding/json"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func ProtocolCases() []Case {
	cases := make([]Case, 0)

	validRequest, _ := loadFixture("valid/request.json")
	cases = append(cases, NewCase(
		"valid_request",
		"Standard valid request envelope should pass",
		validRequest,
		true,
		EnvelopeValidator{},
	))

	validResponse, _ := loadFixture("valid/response.json")
	cases = append(cases, NewCase(
		"valid_response",
		"Standard valid response envelope should pass",
		validResponse,
		true,
		EnvelopeValidator{},
	))

	validNotification, _ := loadFixture("valid/notification.json")
	cases = append(cases, NewCase(
		"valid_notification",
		"Standard valid notification envelope should pass",
		validNotification,
		true,
		EnvelopeValidator{},
	))

	validError, _ := loadFixture("valid/error.json")
	cases = append(cases, NewCase(
		"valid_error",
		"Standard valid error envelope should pass",
		validError,
		true,
		EnvelopeValidator{},
	))

	wrongProtocol, _ := loadFixture("invalid/wrong_protocol.json")
	cases = append(cases, NewCase(
		"wrong_protocol",
		"Wrong protocol version should be rejected",
		wrongProtocol,
		false,
		EnvelopeValidator{},
	))

	reqWithoutID, _ := loadFixture("invalid/request_without_id.json")
	cases = append(cases, NewCase(
		"request_without_id",
		"Request without id should be rejected",
		reqWithoutID,
		false,
		EnvelopeValidator{},
	))

	reqWithoutMethod, _ := loadFixture("invalid/request_without_method.json")
	cases = append(cases, NewCase(
		"request_without_method",
		"Request without method should be rejected",
		reqWithoutMethod,
		false,
		EnvelopeValidator{},
	))

	respWithoutReqID, _ := loadFixture("invalid/response_without_request_id.json")
	cases = append(cases, NewCase(
		"response_without_request_id",
		"Response without requestId should be rejected",
		respWithoutReqID,
		false,
		EnvelopeValidator{},
	))

	invalidErr, _ := loadFixture("invalid/invalid_error.json")
	cases = append(cases, NewCase(
		"invalid_error_missing_code",
		"Error without code should be rejected",
		invalidErr,
		false,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"request_with_request_id",
		"Request carrying requestId should be rejected",
		[]byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","requestId":"x","method":"vendor.test"}`),
		false,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"request_with_error",
		"Request carrying error should be rejected",
		[]byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"vendor.test","error":{"code":"x","message":"y"}}`),
		false,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"response_with_error",
		"Response carrying error should be rejected",
		[]byte(`{"protocol":"amitia-game-host/1","type":"response","id":"r1","requestId":"q1","error":{"code":"x","message":"y"}}`),
		false,
		EnvelopeValidator{},
	))

	return cases
}

func ProtocolVersionCases() []Case {
	cases := make([]Case, 0)

	validVersions := []string{"amitia-game-host/1"}
	for _, v := range validVersions {
		data := []byte(`{"protocol":"` + v + `","type":"request","id":"r1","method":"vendor.test"}`)
		cases = append(cases, NewCase(
			"protocol_"+v,
			"Protocol version "+v+" should be accepted",
			data,
			true,
			EnvelopeValidator{},
		))
	}

	invalidVersions := []string{"amitia-game-host/2", "amitia-game/1", "v1", ""}
	for _, v := range invalidVersions {
		data := []byte(`{"protocol":"` + v + `","type":"request","id":"r1","method":"vendor.test"}`)
		cases = append(cases, NewCase(
			"invalid_protocol_"+v,
			"Protocol version '"+v+"' should be rejected",
			data,
			false,
			EnvelopeValidator{},
		))
	}

	return cases
}

func RequestResponseCorrelationCases() []Case {
	cases := make([]Case, 0)

	cases = append(cases, NewCase(
		"correlation_basic",
		"Response requestId should reference an existing request id",
		[]byte(`{"protocol":"amitia-game-host/1","type":"response","id":"res-001","requestId":"req-001"}`),
		true,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"correlation_missing_request_id",
		"Response with empty requestId should be rejected",
		[]byte(`{"protocol":"amitia-game-host/1","type":"response","id":"res-001","requestId":""}`),
		false,
		EnvelopeValidator{},
	))

	return cases
}

func OpaquePayloadCases() []Case {
	cases := make([]Case, 0)

	complexPayload := map[string]interface{}{
		"player": map[string]interface{}{
			"anything": []interface{}{float64(1), float64(2), float64(3)},
		},
		"vendorExtension": map[string]interface{}{
			"nested": map[string]interface{}{
				"unknown": true,
			},
		},
	}
	payloadBytes, _ := json.Marshal(complexPayload)

	envelope := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "opaque-001",
		Method:   "vendor.custom.action",
		Payload:  payloadBytes,
	}
	envBytes, _ := json.Marshal(envelope)

	cases = append(cases, NewCase(
		"opaque_complex_payload",
		"Complex payload with arbitrary structure should pass",
		envBytes,
		true,
		EnvelopeValidator{},
	))

	largeIntPayload := map[string]interface{}{
		"largeInteger": float64(9007199254740993),
	}
	largeIntBytes, _ := json.Marshal(largeIntPayload)
	envelope2 := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "opaque-002",
		Method:   "vendor.test",
		Payload:  largeIntBytes,
	}
	envBytes2, _ := json.Marshal(envelope2)

	cases = append(cases, NewCase(
		"opaque_large_integer",
		"Large integer in opaque payload should be preserved",
		envBytes2,
		true,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"opaque_null_payload",
		"Null payload should be accepted",
		[]byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"vendor.test","payload":null}`),
		true,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"opaque_empty_object",
		"Empty object payload should be accepted",
		[]byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"vendor.test","payload":{}}`),
		true,
		EnvelopeValidator{},
	))

	return cases
}

func MetadataForwardCompatibilityCases() []Case {
	cases := make([]Case, 0)

	cases = append(cases, NewCase(
		"metadata_with_unknown_keys",
		"Unknown metadata keys should be preserved",
		[]byte(`{"protocol":"amitia-game-host/1","type":"notification","id":"n1","method":"vendor.test","metadata":{"traceId":"abc","vendor.foo":{"unknown":true}}}`),
		true,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"request_metadata",
		"Request with metadata should pass",
		[]byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"vendor.test","metadata":{"traceId":"trace-001"}}`),
		true,
		EnvelopeValidator{},
	))

	return cases
}

func PluginMethodValidationCases() []Case {
	cases := make([]Case, 0)

	validMethods := []string{
		"example.game.operation.execute",
		"vendor.control.run",
		"custom.foo.bar",
		"example.game.state.query",
	}
	for _, m := range validMethods {
		data := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"` + m + `"}`)
		cases = append(cases, NewCase(
			"plugin_method_"+m,
			"Plugin custom method '"+m+"' should be valid",
			data,
			true,
			PluginMethodValidator{},
		))
	}

	reservedMethods := []string{
		"host.secret.read",
		"host.runtime.health",
		"runtime.internal.state",
		"service.register",
		"channel.create",
		"control.stop",
		"permission.check",
	}
	for _, m := range reservedMethods {
		data := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"` + m + `"}`)
		cases = append(cases, NewCase(
			"reserved_method_"+m,
			"Reserved method '"+m+"' should be rejected for plugins",
			data,
			false,
			PluginMethodValidator{},
		))
	}

	invalidMethods := []string{
		"single",
		"UPPERCASE.Method",
		"double..dot",
		".leading.dot",
	}
	for _, m := range invalidMethods {
		data := []byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":"` + m + `"}`)
		cases = append(cases, NewCase(
			"invalid_method_format_"+m,
			"Invalid method format '"+m+"' should be rejected",
			data,
			false,
			PluginMethodValidator{},
		))
	}

	return cases
}
