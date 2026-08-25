package conformance

import (
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func ServiceSchemaCases() []Case {
	cases := make([]Case, 0)

	validSvc1, _ := loadFixture("valid/service.json")
	cases = append(cases, NewCase(
		"valid_service",
		"Standard service descriptor should pass",
		validSvc1,
		true,
		ServiceDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"service_external_kind",
		"External service kind is not part of amitia-game-host/1",
		[]byte(`{"id":"external-svc","kind":"external"}`),
		false,
		ServiceDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"service_with_dependencies",
		"Service with dependencies should be valid",
		[]byte(`{"id":"svc-1","kind":"process","dependsOn":["svc-2","svc-3"]}`),
		true,
		ServiceDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"empty_service_id",
		"Service with empty id should be rejected",
		[]byte(`{"id":"","kind":"process"}`),
		false,
		ServiceDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"invalid_service_kind",
		"Service with invalid kind should be rejected",
		[]byte(`{"id":"svc-1","kind":"invalid"}`),
		false,
		ServiceDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"self_dependency",
		"Service depending on itself should be rejected",
		[]byte(`{"id":"svc-1","kind":"process","dependsOn":["svc-1"]}`),
		false,
		ServiceDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"duplicate_dependency",
		"Service with duplicate dependency should be rejected",
		[]byte(`{"id":"svc-1","kind":"process","dependsOn":["svc-2","svc-2"]}`),
		false,
		ServiceDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"duplicate_capability_in_service",
		"Service with duplicate capability should be rejected",
		[]byte(`{"id":"svc-1","kind":"process","capabilities":["custom_rpc","custom_rpc"]}`),
		false,
		ServiceDescriptorValidator{},
	))

	return cases
}

func ChannelSchemaCases() []Case {
	cases := make([]Case, 0)

	validCh, _ := loadFixture("valid/channel.json")
	cases = append(cases, NewCase(
		"valid_channel",
		"Standard channel descriptor should pass",
		validCh,
		true,
		ChannelDescriptorValidator{},
	))

	channelKinds := []string{"event", "state", "log", "metric", "custom", "binary"}
	for _, k := range channelKinds {
		data := []byte(`{"id":"ch-1","kind":"` + k + `"}`)
		cases = append(cases, NewCase(
			"channel_kind_"+k,
			"Channel kind '"+k+"' should be valid",
			data,
			true,
			ChannelDescriptorValidator{},
		))
	}

	directions := []string{"plugin_to_host"}
	for _, d := range directions {
		data := []byte(`{"id":"ch-1","kind":"event","direction":"` + d + `"}`)
		cases = append(cases, NewCase(
			"channel_direction_"+d,
			"Channel direction '"+d+"' should be valid",
			data,
			true,
			ChannelDescriptorValidator{},
		))
	}

	hints := []string{"low", "normal", "high", "realtime"}
	for _, h := range hints {
		data := []byte(`{"id":"ch-1","kind":"state","frequencyHint":"` + h + `"}`)
		cases = append(cases, NewCase(
			"channel_frequency_"+h,
			"Channel frequency hint '"+h+"' should be valid",
			data,
			true,
			ChannelDescriptorValidator{},
		))
	}

	customSchemaData := []byte(`{"id":"custom-state","kind":"state","schemaId":"example.game.state/v1"}`)
	cases = append(cases, NewCase(
		"custom_schema_id",
		"Custom schemaId like 'example.game.state/v1' should be preserved",
		customSchemaData,
		true,
		ChannelDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"empty_channel_id",
		"Channel with empty id should be rejected",
		[]byte(`{"id":"","kind":"event"}`),
		false,
		ChannelDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"invalid_channel_kind",
		"Channel with invalid kind should be rejected",
		[]byte(`{"id":"ch-1","kind":"invalid"}`),
		false,
		ChannelDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"invalid_channel_direction",
		"Channel with invalid direction should be rejected",
		[]byte(`{"id":"ch-1","kind":"event","direction":"invalid"}`),
		false,
		ChannelDescriptorValidator{},
	))

	cases = append(cases, NewCase(
		"invalid_channel_frequency",
		"Channel with invalid frequency hint should be rejected",
		[]byte(`{"id":"ch-1","kind":"state","frequencyHint":"invalid"}`),
		false,
		ChannelDescriptorValidator{},
	))

	return cases
}

func CapabilityCases() []Case {
	cases := make([]Case, 0)

	knownCaps := []protocol.Capability{
		protocol.CapabilityRealtimeControl,
		protocol.CapabilityStateStreaming,
		protocol.CapabilityEventStreaming,
		protocol.CapabilityCustomRPC,
		protocol.CapabilityHostAPI,
		protocol.CapabilitySharedControl,
		protocol.CapabilityMultiService,
	}
	for _, cap := range knownCaps {
		data := []byte(`"` + string(cap) + `"`)
		cases = append(cases, NewCase(
			"known_capability_"+string(cap),
			"Known capability '"+string(cap)+"' should be valid and known",
			data,
			true,
			CapabilityValidator{},
		))
	}

	customCaps := []string{
		"example.navigation",
		"vendor.visual-agent",
		"game.foo",
	}
	for _, cap := range customCaps {
		data := []byte(`"` + cap + `"`)
		cases = append(cases, NewCase(
			"custom_capability_"+cap,
			"Game/tool capability '"+cap+"' must not be a GameHost feature",
			data,
			false,
			CapabilityValidator{},
		))
	}

	invalidCaps, _ := loadFixture("invalid/invalid_capability.json")
	cases = append(cases, NewCase(
		"invalid_capability_empty",
		"Capability array with empty string should be rejected",
		invalidCaps,
		false,
		CapabilityValidator{},
	))

	cases = append(cases, NewCase(
		"duplicate_capabilities",
		"Duplicate capabilities should be rejected",
		[]byte(`["custom_rpc","custom_rpc"]`),
		false,
		CapabilityValidator{},
	))

	return cases
}

func PluginErrorCases() []Case {
	cases := make([]Case, 0)

	validErr, _ := loadFixture("valid/error.json")
	cases = append(cases, NewCase(
		"valid_error_envelope",
		"Valid error envelope should pass",
		validErr,
		true,
		EnvelopeValidator{},
	))

	errorCodes := []string{
		"invalid_request", "invalid_argument", "not_found", "already_exists", "unsupported",
		"protocol_mismatch", "capability_unsupported", "runtime_unavailable", "service_unavailable",
		"invalid_runtime_state", "permission_denied", "resource_exhausted", "timeout", "cancelled", "internal",
	}
	for _, code := range errorCodes {
		data := []byte(`{"protocol":"amitia-game-host/1","type":"error","id":"e1","error":{"code":"` + code + `","message":"test"}}`)
		cases = append(cases, NewCase(
			"error_code_"+code,
			"Standard error code '"+code+"' should be valid",
			data,
			true,
			EnvelopeValidator{},
		))
	}

	customCodes := []string{
		"vendor.game.connection_failed",
		"vendor.agent_crashed",
	}
	for _, code := range customCodes {
		data := []byte(`{"protocol":"amitia-game-host/1","type":"error","id":"e1","error":{"code":"` + code + `","message":"test"}}`)
		cases = append(cases, NewCase(
			"custom_error_code_"+code,
			"Custom error code '"+code+"' should be valid",
			data,
			true,
			EnvelopeValidator{},
		))
	}

	cases = append(cases, NewCase(
		"invalid_bare_error_code",
		"Bare unknown error code should be rejected",
		[]byte(`{"protocol":"amitia-game-host/1","type":"error","id":"e1","error":{"code":"some_random_name","message":"test"}}`),
		false,
		EnvelopeValidator{},
	))

	return cases
}

func DescriptorConformanceCases() []Case {
	cases := make([]Case, 0)

	validDesc, _ := loadFixture("valid/descriptor.json")
	cases = append(cases, NewCase(
		"valid_descriptor",
		"Standard descriptor should pass",
		validDesc,
		true,
		PluginSchemaValidator{},
	))

	cases = append(cases, NewCase(
		"empty_descriptor",
		"Empty descriptor with only id should pass",
		[]byte(`{"id":"test"}`),
		true,
		PluginSchemaValidator{},
	))

	cases = append(cases, NewCase(
		"missing_id_descriptor",
		"Descriptor without id should be rejected",
		[]byte(`{"name":"test"}`),
		false,
		DescriptorValidator{},
	))

	return cases
}
