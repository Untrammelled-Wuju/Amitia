package domain

import "testing"

func TestChannelDescriptorValid(t *testing.T) {
	cases := []ChannelDescriptor{
		{ID: ChannelID("events"), Kind: ChannelKindEvent},
		{ID: ChannelID("state"), Kind: ChannelKindState},
		{ID: ChannelID("logs"), Kind: ChannelKindLog},
		{ID: ChannelID("metrics"), Kind: ChannelKindMetric},
		{ID: ChannelID("frames"), Kind: ChannelKindBinary},
		{ID: ChannelID("custom"), Kind: ChannelKindCustom},
		{ID: ChannelID("schemaed"), Kind: ChannelKindState, SchemaID: "schema-v1"},
	}

	for _, ch := range cases {
		t.Run("valid_"+string(ch.ID), func(t *testing.T) {
			if err := ch.Validate(); err != nil {
				t.Errorf("expected valid, got: %v", err)
			}
		})
	}
}

func TestChannelDescriptorRejectsEmptyID(t *testing.T) {
	ch := ChannelDescriptor{
		ID:   ChannelID(""),
		Kind: ChannelKindEvent,
	}

	err := ch.Validate()
	if err == nil {
		t.Fatal("expected error for empty channel id")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestChannelDescriptorRejectsInvalidKind(t *testing.T) {
	ch := ChannelDescriptor{
		ID:   ChannelID("test"),
		Kind: ChannelKind("invalid_kind"),
	}

	err := ch.Validate()
	if err == nil {
		t.Fatal("expected error for invalid channel kind")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestChannelDescriptorRejectsControlCharactersInID(t *testing.T) {
	ch := ChannelDescriptor{
		ID:   ChannelID("bad\x00id"),
		Kind: ChannelKindEvent,
	}

	err := ch.Validate()
	if err == nil {
		t.Fatal("expected error for control character in channel id")
	}
}

func TestChannelDescriptorRejectsTooLongSchemaID(t *testing.T) {
	longSchema := make([]byte, 300)
	ch := ChannelDescriptor{
		ID:       ChannelID("test"),
		Kind:     ChannelKindState,
		SchemaID: string(longSchema),
	}

	err := ch.Validate()
	if err == nil {
		t.Fatal("expected error for too long schema id")
	}
}

func TestChannelDescriptorWithMetadata(t *testing.T) {
	ch := ChannelDescriptor{
		ID:   ChannelID("events"),
		Kind: ChannelKindEvent,
		Metadata: map[string]string{
			"rate": "100ms",
		},
	}

	if err := ch.Validate(); err != nil {
		t.Fatalf("expected metadata to be valid, got: %v", err)
	}
}

func TestChannelDescriptorRejectsInvalidMetadata(t *testing.T) {
	ch := ChannelDescriptor{
		ID:   ChannelID("events"),
		Kind: ChannelKindEvent,
		Metadata: map[string]string{
			"": "empty key",
		},
	}

	err := ch.Validate()
	if err == nil {
		t.Fatal("expected error for empty metadata key")
	}
}

func TestIsValidChannelKind(t *testing.T) {
	validKinds := []ChannelKind{
		ChannelKindEvent,
		ChannelKindState,
		ChannelKindLog,
		ChannelKindMetric,
		ChannelKindBinary,
		ChannelKindCustom,
	}

	for _, kind := range validKinds {
		if !IsValidChannelKind(kind) {
			t.Errorf("expected %s to be valid", kind)
		}
	}

	if IsValidChannelKind(ChannelKind("invalid")) {
		t.Error("expected invalid kind to be rejected")
	}
}
