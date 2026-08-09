package conformance

func InvalidCases() []Case {
	cases := make([]Case, 0)

	invalidFixtures := []struct {
		name     string
		path     string
		validator Validator
	}{
		{"wrong_protocol", "invalid/wrong_protocol.json", EnvelopeValidator{}},
		{"request_without_id", "invalid/request_without_id.json", EnvelopeValidator{}},
		{"request_without_method", "invalid/request_without_method.json", EnvelopeValidator{}},
		{"response_without_request_id", "invalid/response_without_request_id.json", EnvelopeValidator{}},
		{"invalid_error_missing_code", "invalid/invalid_error.json", EnvelopeValidator{}},
		{"duplicate_service", "invalid/duplicate_service.json", ServiceDescriptorValidator{}},
		{"invalid_channel_kind", "invalid/invalid_channel.json", ChannelDescriptorValidator{}},
		{"invalid_capability_empty", "invalid/invalid_capability.json", CapabilityValidator{}},
	}

	for _, f := range invalidFixtures {
		data, err := loadFixture(f.path)
		if err != nil {
			continue
		}
		cases = append(cases, NewCase(
			"invalid_"+f.name,
			"Invalid fixture "+f.name+" should be rejected",
			data,
			false,
			f.validator,
		))
	}

	cases = append(cases, NewCase(
		"malformed_json",
		"Malformed JSON should be rejected",
		[]byte(`{"protocol": "broken"`),
		false,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"unknown_type",
		"Unknown message type should be rejected",
		[]byte(`{"protocol":"amitia-game-host/1","type":"unknown","id":"r1"}`),
		false,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"empty_id",
		"Empty message id should be rejected",
		[]byte(`{"protocol":"amitia-game-host/1","type":"request","id":"","method":"vendor.test"}`),
		false,
		EnvelopeValidator{},
	))

	cases = append(cases, NewCase(
		"empty_method",
		"Empty method should be rejected",
		[]byte(`{"protocol":"amitia-game-host/1","type":"request","id":"r1","method":""}`),
		false,
		EnvelopeValidator{},
	))

	return cases
}
