package v2

import (
	"testing"
)

func TestEnvelopeValidate(t *testing.T) {
	env := &Envelope{
		EnvelopeVersion:      2,
		Protocol:             "amitia.desktop-pet.runtime",
		MessageType:          MessageTypeHello,
		MessageName:          "hello",
		MessageID:            "test-msg-1",
		UserID:               "user-1",
		DeviceID:             "device-1",
		RuntimeID:            "runtime-1",
		RuntimeSessionID:     "session-1",
		ConnectionGeneration: 1,
		Sequence:             0,
		PayloadSchemaVersion: 1,
		PayloadHash:          "sha256:abc",
	}

	if err := env.Validate(); err != nil {
		t.Errorf("valid envelope should pass: %v", err)
	}

	env.EnvelopeVersion = 1
	if err := env.Validate(); err == nil {
		t.Error("invalid envelope version should fail")
	}
}

func TestComputePayloadHash(t *testing.T) {
	payload := []byte(`{"key":"value"}`)
	hash1 := ComputePayloadHash(payload)
	hash2 := ComputePayloadHash(payload)

	if hash1 != hash2 {
		t.Error("same payload should produce same hash")
	}

	if len(hash1) < 10 {
		t.Error("hash should have reasonable length")
	}

	otherPayload := []byte(`{"key":"other"}`)
	hash3 := ComputePayloadHash(otherPayload)
	if hash1 == hash3 {
		t.Error("different payload should produce different hash")
	}
}

func TestCommandStatusTransitions(t *testing.T) {
	tests := []struct {
		status   CommandStatus
		terminal bool
		running  bool
	}{
		{CommandStatusCreated, false, false},
		{CommandStatusQueued, false, false},
		{CommandStatusDispatching, false, true},
		{CommandStatusTransportDispatched, false, true},
		{CommandStatusRuntimeReceived, false, true},
		{CommandStatusRuntimeAccepted, false, true},
		{CommandStatusRendererAccepted, false, true},
		{CommandStatusPlaybackStarted, false, true},
		{CommandStatusCompleted, true, false},
		{CommandStatusFailedTerminal, true, false},
		{CommandStatusExpired, true, false},
		{CommandStatusCancelled, true, false},
		{CommandStatusSuperseded, true, false},
	}

	for _, tt := range tests {
		if tt.status.IsTerminal() != tt.terminal {
			t.Errorf("%s terminal=%v, got %v", tt.status, tt.terminal, tt.status.IsTerminal())
		}
		if tt.status.IsRunning() != tt.running {
			t.Errorf("%s running=%v, got %v", tt.status, tt.running, tt.status.IsRunning())
		}
	}
}

func TestCommandTypeClassification(t *testing.T) {
	durableTypes := []CommandType{
		CommandTypeSyncDesiredState,
		CommandTypeEnsureAbsent,
		CommandTypeReloadRelease,
	}

	ephemeralTypes := []CommandType{
		CommandTypePlayAction,
		CommandTypeStopAction,
		CommandTypePauseAction,
		CommandTypeResumeAction,
		CommandTypeRecenterOnce,
	}

	for _, ct := range durableTypes {
		if !ct.IsDurable() {
			t.Errorf("%s should be durable", ct)
		}
		if ct.IsEphemeral() {
			t.Errorf("%s should not be ephemeral", ct)
		}
	}

	for _, ct := range ephemeralTypes {
		if ct.IsDurable() {
			t.Errorf("%s should not be durable", ct)
		}
		if !ct.IsEphemeral() {
			t.Errorf("%s should be ephemeral", ct)
		}
		if !ct.IsKnown() {
			t.Errorf("%s should be known", ct)
		}
	}

	unknown := CommandType("runtime.command.typo")
	if unknown.IsDurable() || unknown.IsEphemeral() || unknown.IsKnown() {
		t.Fatalf("unknown command type must fail closed, got durable=%v ephemeral=%v known=%v",
			unknown.IsDurable(), unknown.IsEphemeral(), unknown.IsKnown())
	}

	validDurable := &RuntimeCommand{CommandType: string(CommandTypeSyncDesiredState), Durability: "durable"}
	validEphemeral := &RuntimeCommand{CommandType: string(CommandTypeRecenterOnce), Durability: "ephemeral"}
	invalidMismatch := &RuntimeCommand{CommandType: string(CommandTypeSyncDesiredState), Durability: "ephemeral"}
	invalidUnknown := &RuntimeCommand{CommandType: string(unknown), Durability: "ephemeral"}
	if !validDurable.HasValidClassification() || !validEphemeral.HasValidClassification() {
		t.Fatal("known command classifications must be valid")
	}
	if invalidMismatch.HasValidClassification() || invalidUnknown.HasValidClassification() {
		t.Fatal("mismatched or unknown stored command classification must fail closed")
	}
}

func TestSessionStatus(t *testing.T) {
	active := []SessionStatus{
		SessionStatusRegistering,
		SessionStatusSyncing,
		SessionStatusReady,
		SessionStatusDegraded,
	}
	terminal := []SessionStatus{
		SessionStatusClosed,
		SessionStatusSuperseded,
	}

	for _, s := range active {
		if !s.IsActive() {
			t.Errorf("%s should be active", s)
		}
		if s.IsTerminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}

	for _, s := range terminal {
		if s.IsActive() {
			t.Errorf("%s should not be active", s)
		}
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
}

func TestCanUpdateProtection(t *testing.T) {
	state := &RuntimeActualState{
		ConnectionGeneration: 5,
		LastEventSequence:    100,
	}

	if !state.CanUpdate(6, 99) {
		t.Error("higher generation should allow update")
	}
	if !state.CanUpdate(5, 101) {
		t.Error("same gen, higher seq should allow update")
	}
	if state.CanUpdate(5, 100) {
		t.Error("same gen, same seq should reject")
	}
	if state.CanUpdate(5, 99) {
		t.Error("same gen, lower seq should reject")
	}
	if state.CanUpdate(4, 200) {
		t.Error("lower generation should reject")
	}
}
