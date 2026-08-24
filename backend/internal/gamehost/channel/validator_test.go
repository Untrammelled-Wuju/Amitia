package channel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestValidateDirection_PluginToHost_AllowsPluginToHost(t *testing.T) {
	ch := RuntimeChannel{
		Direction: protocol.ChannelDirectionPluginToHost,
		Kind:      domain.ChannelKindEvent,
	}
	if err := ValidateDirection(ch, protocol.ChannelDirectionPluginToHost); err != nil {
		t.Fatalf("should allow plugin_to_host: %v", err)
	}
}

func TestValidateDirection_V1RejectsOutboundFlow(t *testing.T) {
	ch := RuntimeChannel{
		Direction: protocol.ChannelDirectionPluginToHost,
		Kind:      domain.ChannelKindEvent,
	}
	if err := ValidateDirection(ch, protocol.ChannelDirection("host_to_plugin")); err == nil {
		t.Fatal("protocol v1 must reject host_to_plugin channel flow")
	}
}

func TestValidateDirection_V1RejectsLegacyOutboundDirections(t *testing.T) {
	for _, direction := range []protocol.ChannelDirection{"host_to_plugin", "bidirectional"} {
		ch := RuntimeChannel{Direction: direction, Kind: domain.ChannelKindEvent}
		if err := ValidateDirection(ch, protocol.ChannelDirectionPluginToHost); err == nil {
			t.Fatalf("protocol v1 must reject direction %q", direction)
		}
	}
}

func TestValidateDirection_DefaultsToPluginToHost(t *testing.T) {
	ch := RuntimeChannel{Direction: "", Kind: domain.ChannelKindEvent}
	if err := ValidateDirection(ch, protocol.ChannelDirectionPluginToHost); err != nil {
		t.Fatalf("empty direction should mean plugin_to_host in v1: %v", err)
	}
}

func TestValidator_ValidateRegistration_Valid(t *testing.T) {
	v := NewValidator(Options{
		ServiceResolver: &mockServiceResolver{services: map[domain.ServiceID]bool{
			"s": true,
		}},
	})
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err != nil {
		t.Fatalf("should succeed: %v", err)
	}
}

func TestValidator_ValidateRegistration_UnknownKind(t *testing.T) {
	v := NewValidator(Options{})
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKind("invalid"),
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err == nil {
		t.Fatal("should reject unknown kind")
	}
}

func TestValidator_ValidateRegistration_UnknownDirection(t *testing.T) {
	v := NewValidator(Options{})
	dir := protocol.ChannelDirection("duplex")
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
		Direction: dir,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err == nil {
		t.Fatal("should reject unknown direction")
	}
}

func TestValidator_ValidateRegistration_UnknownFrequency(t *testing.T) {
	v := NewValidator(Options{})
	freq := protocol.FrequencyHint("ultra")
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
		Frequency: &freq,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err == nil {
		t.Fatal("should reject unknown frequency")
	}
}

func TestValidator_ValidateRegistration_OwnerNotFound(t *testing.T) {
	v := NewValidator(Options{
		ServiceResolver: &mockServiceResolver{services: map[domain.ServiceID]bool{}},
	})
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "unknown",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err == nil {
		t.Fatal("should reject when service does not exist")
	}
}

func TestValidator_ValidateRegistration_RuntimeLimit(t *testing.T) {
	v := NewValidator(Options{MaxChannelsPerRuntime: 2})
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 2, 0); err == nil {
		t.Fatal("should reject when runtime limit reached")
	}
}

func TestValidator_ValidateRegistration_ServiceLimit(t *testing.T) {
	v := NewValidator(Options{MaxChannelsPerService: 5})
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 5); err == nil {
		t.Fatal("should reject when service limit reached")
	}
}

func TestValidator_ValidateRegistration_EmptyPluginID(t *testing.T) {
	v := NewValidator(Options{})
	ch := RuntimeChannel{
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err == nil {
		t.Fatal("should reject empty plugin id")
	}
}

func TestValidator_ValidateRegistration_EmptyRuntimeID(t *testing.T) {
	v := NewValidator(Options{})
	ch := RuntimeChannel{
		PluginID:  "p",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err == nil {
		t.Fatal("should reject empty runtime id")
	}
}

func TestValidator_ValidateRegistration_EmptyServiceID(t *testing.T) {
	v := NewValidator(Options{})
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err == nil {
		t.Fatal("should reject empty service id")
	}
}

func TestValidator_ValidateRegistration_EmptyChannelID(t *testing.T) {
	v := NewValidator(Options{})
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Kind:      domain.ChannelKindEvent,
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err == nil {
		t.Fatal("should reject empty channel id")
	}
}

func TestValidator_ValidateRegistration_MetadataTooLarge(t *testing.T) {
	v := NewValidator(Options{MaxMetadataBytes: 10})
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
		Metadata: map[string]json.RawMessage{
			"k": []byte("value-over-limit"),
		},
	}
	if err := v.ValidateRegistration(context.Background(), ch, 0, 0); err == nil {
		t.Fatal("should reject large metadata")
	}
}

func TestRuntimeChannel_Clone_MetadataCopied(t *testing.T) {
	ch := RuntimeChannel{
		ID:        "r/s/c",
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
		Metadata: map[string]json.RawMessage{
			"key": []byte(`"value"`),
		},
	}

	cloned := ch.Clone()
	cloned.Metadata["key"] = []byte(`"changed"`)

	if string(ch.Metadata["key"]) != `"value"` {
		t.Fatalf("original should be unchanged, got %s", ch.Metadata["key"])
	}
}

func TestRuntimeChannel_Clone_FrequencyCopied(t *testing.T) {
	freq := protocol.FrequencyHintHigh
	ch := RuntimeChannel{
		ID:        "r/s/c",
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "c",
		Kind:      domain.ChannelKindEvent,
		Frequency: &freq,
	}

	cloned := ch.Clone()
	if cloned.Frequency == nil {
		t.Fatal("frequency should be copied")
	}
	if *cloned.Frequency != protocol.FrequencyHintHigh {
		t.Fatalf("expected high, got %s", *cloned.Frequency)
	}
}

func TestRuntimeChannel_Validate_Valid(t *testing.T) {
	ch := RuntimeChannel{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "events",
		Kind:      domain.ChannelKindEvent,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	if err := ch.Validate(); err != nil {
		t.Fatalf("should be valid: %v", err)
	}
}

func TestRuntimeChannelID_Parse(t *testing.T) {
	id := NewRuntimeChannelID("rt-1", "svc-a", "events")
	if id.RuntimeID() != "rt-1" {
		t.Fatalf("expected rt-1, got %s", id.RuntimeID())
	}
	if id.ServiceID() != "svc-a" {
		t.Fatalf("expected svc-a, got %s", id.ServiceID())
	}
	if id.ChannelID() != "events" {
		t.Fatalf("expected events, got %s", id.ChannelID())
	}
}

func TestMapper_Map_Valid(t *testing.T) {
	m := NewMapper()
	input := ChannelMappingInput{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Descriptors: []protocol.ChannelDescriptor{
			{ID: "events", Kind: "event", Direction: "plugin_to_host", SchemaID: "vendor.events/v1"},
			{ID: "state", Kind: "state"},
		},
	}
	result, err := m.Map(context.Background(), input)
	if err != nil {
		t.Fatalf("map should succeed: %v", err)
	}
	if len(result.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(result.Channels))
	}
	if result.Channels[0].Kind != domain.ChannelKindEvent {
		t.Fatalf("expected event, got %s", result.Channels[0].Kind)
	}
}

func TestMapper_BuildForRuntime(t *testing.T) {
	m := NewMapper()
	freq := protocol.FrequencyHintNormal
	descriptors := []protocol.ChannelDescriptor{
		{ID: "events", Kind: "event", Direction: "plugin_to_host", FrequencyHint: &freq},
	}
	result, err := m.BuildForRuntime(context.Background(), "p", "r", "s", descriptors)
	if err != nil {
		t.Fatalf("should succeed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(result))
	}
	if result[0].Frequency == nil || *result[0].Frequency != protocol.FrequencyHintNormal {
		t.Fatal("frequency should be preserved")
	}
}

func TestReconciler_EmptyDesired_RemovesAll(t *testing.T) {
	reg := NewMemoryRegistry(Options{})
	mapper := NewMapper()
	rec := NewReconciler(reg, mapper)

	ch := RuntimeChannel{
		ID:        NewRuntimeChannelID("r", "s", "events"),
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "events",
		Kind:      domain.ChannelKindEvent,
	}
	reg.Register(context.Background(), ch)

	result, err := rec.ReconcileRuntimeChannels(context.Background(), "r", nil)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if len(result.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(result.Removed))
	}
}

func TestReconciler_Idempotent(t *testing.T) {
	reg := NewMemoryRegistry(Options{})
	mapper := NewMapper()
	rec := NewReconciler(reg, mapper)

	input := []ChannelMappingInput{{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Descriptors: []protocol.ChannelDescriptor{
			{ID: "events", Kind: "event"},
		},
	}}

	_, err := rec.ReconcileRuntimeChannels(context.Background(), "r", input)
	if err != nil {
		t.Fatalf("first reconcile failed: %v", err)
	}

	result2, err := rec.ReconcileRuntimeChannels(context.Background(), "r", input)
	if err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}
	if len(result2.Added) != 0 || len(result2.Removed) != 0 || len(result2.Updated) != 0 {
		t.Fatalf("second reconcile should be noop, got %+v", result2)
	}
}

func TestReconciler_AddChannel(t *testing.T) {
	reg := NewMemoryRegistry(Options{})
	mapper := NewMapper()
	rec := NewReconciler(reg, mapper)

	result1, _ := rec.ReconcileRuntimeChannels(context.Background(), "r", []ChannelMappingInput{{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Descriptors: []protocol.ChannelDescriptor{
			{ID: "events", Kind: "event"},
		},
	}})
	if len(result1.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(result1.Added))
	}

	result2, _ := rec.ReconcileRuntimeChannels(context.Background(), "r", []ChannelMappingInput{{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Descriptors: []protocol.ChannelDescriptor{
			{ID: "events", Kind: "event"},
			{ID: "state", Kind: "state"},
		},
	}})
	if len(result2.Added) != 1 {
		t.Fatalf("expected 1 added in second pass, got %d", len(result2.Added))
	}
}

func TestReconciler_RemoveChannel(t *testing.T) {
	reg := NewMemoryRegistry(Options{})
	mapper := NewMapper()
	rec := NewReconciler(reg, mapper)

	input1 := []ChannelMappingInput{{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Descriptors: []protocol.ChannelDescriptor{
			{ID: "events", Kind: "event"},
			{ID: "state", Kind: "state"},
		},
	}}
	rec.ReconcileRuntimeChannels(context.Background(), "r", input1)

	input2 := []ChannelMappingInput{{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Descriptors: []protocol.ChannelDescriptor{
			{ID: "events", Kind: "event"},
		},
	}}
	result, _ := rec.ReconcileRuntimeChannels(context.Background(), "r", input2)

	if len(result.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(result.Removed))
	}
}

func TestReconciler_UpdateChannel(t *testing.T) {
	reg := NewMemoryRegistry(Options{})
	mapper := NewMapper()
	rec := NewReconciler(reg, mapper)

	input1 := []ChannelMappingInput{{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Descriptors: []protocol.ChannelDescriptor{
			{ID: "events", Kind: "event", Direction: "plugin_to_host", SchemaID: "vendor.events/v1"},
		},
	}}
	rec.ReconcileRuntimeChannels(context.Background(), "r", input1)

	input2 := []ChannelMappingInput{{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Descriptors: []protocol.ChannelDescriptor{
			{ID: "events", Kind: "event", Direction: "plugin_to_host", SchemaID: "vendor.events/v2"},
		},
	}}
	result, _ := rec.ReconcileRuntimeChannels(context.Background(), "r", input2)

	if len(result.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(result.Updated))
	}
}

func TestReconciler_FullValidation_FailsOnBadChannel(t *testing.T) {
	reg := NewMemoryRegistry(Options{})
	mapper := NewMapper()
	rec := NewReconciler(reg, mapper)

	input := []ChannelMappingInput{{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		Descriptors: []protocol.ChannelDescriptor{
			{ID: "events", Kind: "event"},
			{ID: "bad", Kind: "invalid_kind"},
		},
	}}
	_, err := rec.ReconcileRuntimeChannels(context.Background(), "r", input)
	if err == nil {
		t.Fatal("expected reconciliation to fail on bad channel")
	}

	if reg.Count() != 0 {
		t.Fatalf("expected 0 channels after failed reconcile, got %d", reg.Count())
	}
}

func mockServiceExists(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (bool, error) {
	return serviceID == "s", nil
}
