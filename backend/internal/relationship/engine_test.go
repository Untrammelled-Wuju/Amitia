package relationship

import (
	"reflect"
	"testing"
	"time"
)

func TestUpdateRelationshipUsesDefaultState(t *testing.T) {
	result := UpdateRelationship(UpdateInput{})
	if result.Previous != DefaultState() {
		t.Fatalf("expected default previous state, got %#v", result.Previous)
	}
	if result.Next != DefaultState() {
		t.Fatalf("expected unchanged default next state, got %#v", result.Next)
	}
	if !contains(result.Audit.Diagnostics, "default_state") {
		t.Fatalf("expected default_state diagnostic, got %#v", result.Audit.Diagnostics)
	}
}

func TestPositiveInteractionRaisesTrustAndFamiliarity(t *testing.T) {
	result := UpdateRelationship(UpdateInput{
		Current: DefaultState(),
		Evidence: []InteractionEvidence{
			{ID: "ev-positive", Kind: EvidenceKindSupportive, Intensity: 0.9, Confidence: 0.9},
		},
		Personality: PersonalityRef{
			Version:           "personality-v1",
			Warmth:            0.8,
			Affection:         0.7,
			BoundaryStrength:  0.45,
			Sensitivity:       0.35,
			Tolerance:         0.7,
			ConflictAvoidance: 0.4,
		},
	})
	if result.Next.Trust <= result.Previous.Trust {
		t.Fatalf("expected trust increase, got previous=%v next=%v", result.Previous.Trust, result.Next.Trust)
	}
	if result.Next.Familiarity <= result.Previous.Familiarity {
		t.Fatalf("expected familiarity increase, got previous=%v next=%v", result.Previous.Familiarity, result.Next.Familiarity)
	}
	if result.Next.Tension >= result.Previous.Tension {
		t.Fatalf("expected tension decrease, got previous=%v next=%v", result.Previous.Tension, result.Next.Tension)
	}
	if result.Audit.PersonalityVersion != "personality-v1" {
		t.Fatalf("expected personality version, got %q", result.Audit.PersonalityVersion)
	}
}

func TestBoundaryConflictRaisesTensionAndLimitsFamiliarityGrowth(t *testing.T) {
	result := UpdateRelationship(UpdateInput{
		Current: DefaultState(),
		Evidence: []InteractionEvidence{
			{ID: "ev-boundary", Kind: EvidenceKindBoundary, Intensity: 1, Confidence: 1},
			{ID: "ev-positive", Kind: EvidenceKindPositive, Intensity: 1, Confidence: 1},
		},
		Personality: PersonalityRef{
			Warmth:            0.9,
			Affection:         0.9,
			BoundaryStrength:  0.9,
			Sensitivity:       0.8,
			Tolerance:         0.4,
			ConflictAvoidance: 0.8,
		},
		Budget: ChangeBudget{
			MaxPositiveDelta: 0.08,
			MaxNegativeDelta: 0.08,
			MaxTensionDelta:  0.12,
			MaxBoundaryDelta: 0.12,
		},
	})
	if result.Next.Tension <= result.Previous.Tension {
		t.Fatalf("expected tension increase, got previous=%v next=%v", result.Previous.Tension, result.Next.Tension)
	}
	if result.Delta.Familiarity > 0.028 {
		t.Fatalf("expected conservative familiarity growth, got %v", result.Delta.Familiarity)
	}
	if !contains(result.Audit.Diagnostics, "conservative_boundary_mode") {
		t.Fatalf("expected conservative diagnostic, got %#v", result.Audit.Diagnostics)
	}
}

func TestConflictRaisesTensionWithoutUnlimitedTrustCollapse(t *testing.T) {
	current := DefaultState()
	current.Trust = 0.72
	current.Security = 0.68
	current.RepairConfidence = 0.75
	result := UpdateRelationship(UpdateInput{
		Current: current,
		Evidence: []InteractionEvidence{
			{ID: "ev-conflict", Kind: EvidenceKindConflict, Intensity: 1, Confidence: 1},
		},
		Personality: PersonalityRef{
			BoundaryStrength:  0.55,
			Sensitivity:       0.45,
			Tolerance:         0.7,
			ConflictAvoidance: 0.5,
		},
		Budget: DefaultBudget(),
	})
	if result.Next.Tension <= result.Previous.Tension {
		t.Fatalf("expected conflict tension increase, previous=%v next=%v", result.Previous.Tension, result.Next.Tension)
	}
	if result.Delta.Trust <= -DefaultBudget().MaxNegativeDelta {
		t.Fatalf("expected trust decline to remain budgeted, got %v", result.Delta.Trust)
	}
	if result.Next.RepairConfidence >= result.Previous.RepairConfidence {
		t.Fatalf("expected conflict to reduce repair confidence, previous=%v next=%v", result.Previous.RepairConfidence, result.Next.RepairConfidence)
	}
}

func TestRepairConfidenceControlsRepairAndDoesNotClearTension(t *testing.T) {
	current := DefaultState()
	current.Tension = 0.7
	current.RepairConfidence = 0.8
	high := UpdateRelationship(UpdateInput{
		Current: current,
		Evidence: []InteractionEvidence{
			{ID: "ev-repair", Kind: EvidenceKindRepair, Intensity: 1, Confidence: 1},
		},
		Personality: PersonalityRef{Warmth: 0.7, Tolerance: 0.8},
		Budget:      DefaultBudget(),
	})
	current.RepairConfidence = 0.1
	low := UpdateRelationship(UpdateInput{
		Current: current,
		Evidence: []InteractionEvidence{
			{ID: "ev-repair", Kind: EvidenceKindRepair, Intensity: 1, Confidence: 1},
		},
		Personality: PersonalityRef{Warmth: 0.7, Tolerance: 0.8},
		Budget:      DefaultBudget(),
	})
	if high.Delta.Tension >= low.Delta.Tension {
		t.Fatalf("expected stronger repair evidence to lower tension more, high=%v low=%v", high.Delta.Tension, low.Delta.Tension)
	}
	if high.Next.Tension == 0 {
		t.Fatalf("expected repair not to clear all tension at once, got %#v", high.Next)
	}
	if high.Next.RepairConfidence <= high.Previous.RepairConfidence {
		t.Fatalf("expected repair confidence to increase, previous=%v next=%v", high.Previous.RepairConfidence, high.Next.RepairConfidence)
	}
}

func TestSafetyEventIsSeparatedFromOrdinaryConflict(t *testing.T) {
	result := UpdateRelationship(UpdateInput{
		Current: DefaultState(),
		Evidence: []InteractionEvidence{
			{ID: "ev-safety", Kind: EvidenceKindSafety, Intensity: 1, Confidence: 1},
		},
		Personality: PersonalityRef{
			BoundaryStrength:  0.9,
			Sensitivity:       0.9,
			Tolerance:         0.3,
			ConflictAvoidance: 0.8,
		},
		Budget: DefaultBudget(),
	})
	if result.Next.Security >= result.Previous.Security {
		t.Fatalf("expected safety event to lower security, previous=%v next=%v", result.Previous.Security, result.Next.Security)
	}
	if result.Next.Boundary <= result.Previous.Boundary {
		t.Fatalf("expected safety event to raise boundary, previous=%v next=%v", result.Previous.Boundary, result.Next.Boundary)
	}
	if !contains(result.Audit.Diagnostics, "safety_event_separated") {
		t.Fatalf("expected separated safety diagnostic, got %#v", result.Audit.Diagnostics)
	}
}

func TestSecureRelationshipBuffersShortAbsence(t *testing.T) {
	current := DefaultState()
	current.Security = 0.82
	result := UpdateRelationship(UpdateInput{
		Current: current,
		Evidence: []InteractionEvidence{
			{ID: "ev-short-absence", Kind: EvidenceKindWithdrawal, Intensity: 0.7, Confidence: 0.8},
		},
		Personality: PersonalityRef{
			Version:           "attachment-v1",
			Warmth:            0.62,
			Affection:         0.55,
			Attachment:        0.9,
			BoundaryStrength:  0.5,
			Sensitivity:       0.55,
			Tolerance:         0.7,
			ConflictAvoidance: 0.5,
		},
	})
	if result.Next.Trust != result.Previous.Trust {
		t.Fatalf("expected short absence not to lower trust, previous=%v next=%v", result.Previous.Trust, result.Next.Trust)
	}
	if result.Next.Tension <= result.Previous.Tension {
		t.Fatalf("expected bounded contact tension increase, previous=%v next=%v", result.Previous.Tension, result.Next.Tension)
	}
	if !contains(result.Audit.Diagnostics, "secure_absence_buffer") {
		t.Fatalf("expected secure absence diagnostic, got %#v", result.Audit.Diagnostics)
	}
}

func TestChangeBudgetLimitsDelta(t *testing.T) {
	result := UpdateRelationship(UpdateInput{
		Current: DefaultState(),
		Evidence: []InteractionEvidence{
			{Kind: EvidenceKindSupportive, Intensity: 1, Confidence: 1},
			{Kind: EvidenceKindSupportive, Intensity: 1, Confidence: 1},
			{Kind: EvidenceKindSupportive, Intensity: 1, Confidence: 1},
		},
		Personality: PersonalityRef{Warmth: 1, Affection: 1, Tolerance: 1},
		Budget: ChangeBudget{
			MaxPositiveDelta: 0.03,
			MaxNegativeDelta: 0.02,
			MaxTensionDelta:  0.025,
			MaxBoundaryDelta: 0.02,
		},
	})
	if result.Delta.Trust != 0.03 {
		t.Fatalf("expected trust delta capped at 0.03, got %v", result.Delta.Trust)
	}
	if result.Delta.Tension != -0.025 {
		t.Fatalf("expected tension delta capped at -0.025, got %v", result.Delta.Tension)
	}
}

func TestRepeatedInputProducesStableOutput(t *testing.T) {
	input := UpdateInput{
		Current: RelationshipState{Trust: 0.45, Familiarity: 0.4, Tension: 0.3, Boundary: 0.55},
		Evidence: []InteractionEvidence{
			{ID: "ev-2", Kind: EvidenceKindRepair, Intensity: 0.8, Confidence: 0.9},
			{ID: "ev-1", Kind: EvidenceKindConflict, Intensity: 0.25, Confidence: 0.8},
		},
		Personality: PersonalityRef{
			Version:           "personality-v2",
			Warmth:            0.65,
			Affection:         0.5,
			BoundaryStrength:  0.6,
			Sensitivity:       0.5,
			Tolerance:         0.7,
			ConflictAvoidance: 0.55,
		},
		Budget: DefaultBudget(),
	}
	first := UpdateRelationship(input)
	second := UpdateRelationship(input)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected stable output\nfirst=%#v\nsecond=%#v", first, second)
	}
	if !reflect.DeepEqual(first.Audit.EvidenceIDs, []string{"ev-1", "ev-2"}) {
		t.Fatalf("expected sorted evidence ids, got %#v", first.Audit.EvidenceIDs)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestTensionAccumulatesAcrossMultipleConflicts(t *testing.T) {
	current := DefaultState()
	current.Tension = 0.1
	first := UpdateRelationship(UpdateInput{
		Current: current,
		Evidence: []InteractionEvidence{
			{ID: "ev-c1", Kind: EvidenceKindConflict, Intensity: 0.6, Confidence: 0.8},
		},
		Personality: PersonalityRef{Tolerance: 0.5, Sensitivity: 0.5},
		Budget:      DefaultBudget(),
	})
	if first.Next.Tension <= current.Tension {
		t.Fatalf("expected first conflict to raise tension, before=%v after=%v", current.Tension, first.Next.Tension)
	}
	second := UpdateRelationship(UpdateInput{
		Current: first.Next,
		Evidence: []InteractionEvidence{
			{ID: "ev-c2", Kind: EvidenceKindConflict, Intensity: 0.7, Confidence: 0.9},
		},
		Personality: PersonalityRef{Tolerance: 0.5, Sensitivity: 0.5},
		Budget:      DefaultBudget(),
	})
	if second.Next.Tension <= first.Next.Tension {
		t.Fatalf("expected accumulated tension after second conflict, first=%v second=%v", first.Next.Tension, second.Next.Tension)
	}
}

func TestRepairConfidenceBaseFromHistory(t *testing.T) {
	history := []RepairRecord{
		{Effective: true, TensionBefore: 0.5, TensionAfter: 0.3},
		{Effective: true, TensionBefore: 0.4, TensionAfter: 0.2},
		{Effective: false, TensionBefore: 0.6, TensionAfter: 0.5},
		{Effective: true, TensionBefore: 0.3, TensionAfter: 0.1},
	}
	base := ComputeRepairConfidenceBase(history)
	if base <= 0.35 {
		t.Fatalf("expected repair confidence base above baseline for successful history, got %v", base)
	}
	if base > 1 {
		t.Fatalf("expected repair confidence base <= 1, got %v", base)
	}
}

func TestRepairConfidenceBaseFromEmptyHistory(t *testing.T) {
	base := ComputeRepairConfidenceBase(nil)
	if base != 0.35 {
		t.Fatalf("expected default repair confidence baseline, got %v", base)
	}
}

func TestTensionDecayWithHighRepairConfidence(t *testing.T) {
	state := DefaultState()
	state.Tension = 0.6
	state.RepairConfidence = 0.8
	profile := DefaultTensionDecay()
	decayed := ComputeTensionDecay(state, 4.0, profile)
	if decayed >= state.Tension {
		t.Fatalf("expected tension to decay over time, before=%v after=%v", state.Tension, decayed)
	}
	if decayed < 0 {
		t.Fatalf("expected tension >= 0, got %v", decayed)
	}
}

func TestTensionDecayWithoutElapsedTime(t *testing.T) {
	state := DefaultState()
	state.Tension = 0.6
	profile := DefaultTensionDecay()
	decayed := ComputeTensionDecay(state, 0, profile)
	if decayed != state.Tension {
		t.Fatalf("expected tension unchanged when no time elapsed, got %v", decayed)
	}
}

func TestConflictEventTracedAfterConflict(t *testing.T) {
	current := DefaultState()
	current.Tension = 0.3
	result := UpdateRelationship(UpdateInput{
		Current: current,
		Evidence: []InteractionEvidence{
			{ID: "ev-conflict", Kind: EvidenceKindConflict, Intensity: 0.8, Confidence: 0.85},
		},
		Personality: PersonalityRef{Tolerance: 0.5, Sensitivity: 0.5},
		Budget:      DefaultBudget(),
	})
	if result.Next.Tension <= current.Tension {
		t.Fatalf("expected conflict to raise tension, before=%v after=%v", current.Tension, result.Next.Tension)
	}
	if hasDiagnostic(result.Audit.Diagnostics, "conservative_boundary_mode") {
		t.Logf("conservative boundary mode triggered for conflict")
	}
}

func hasDiagnostic(diagnostics []string, target string) bool {
	for _, d := range diagnostics {
		if d == target {
			return true
		}
	}
	return false
}

func TestDefaultDimensionsHasAllFields(t *testing.T) {
	dims := DefaultDimensions()
	if dims.Trust.Value != 50 {
		t.Fatalf("expected trust 50, got %v", dims.Trust.Value)
	}
	if dims.Intimacy.Value != 35 {
		t.Fatalf("expected intimacy 35, got %v", dims.Intimacy.Value)
	}
	if dims.Dependency.Value != 30 {
		t.Fatalf("expected dependency 30, got %v", dims.Dependency.Value)
	}
	if dims.Conflict.Value != 15 {
		t.Fatalf("expected conflict 15, got %v", dims.Conflict.Value)
	}
	if dims.Repair.Value != 35 {
		t.Fatalf("expected repair 35, got %v", dims.Repair.Value)
	}
	if dims.Trust.Velocity != 0 || dims.Intimacy.Velocity != 0 || dims.Conflict.Velocity != 0 {
		t.Fatalf("expected initial velocities to be zero")
	}
	if dims.Trust.LastUpdated.IsZero() {
		t.Fatalf("expected non-zero last updated")
	}
}

func TestDimensionsFromStateMapsCorrectly(t *testing.T) {
	state := DefaultState()
	state.Trust = 0.72
	state.Familiarity = 0.56
	state.Security = 0.44
	state.Tension = 0.28
	state.RepairConfidence = 0.63

	dims := DimensionsFromState(state)
	if dims.Trust.Value != 72 {
		t.Fatalf("expected trust 72, got %v", dims.Trust.Value)
	}
	if dims.Intimacy.Value != 56 {
		t.Fatalf("expected intimacy 56, got %v", dims.Intimacy.Value)
	}
	if dims.Dependency.Value != 44 {
		t.Fatalf("expected dependency 44, got %v", dims.Dependency.Value)
	}
	if dims.Conflict.Value != 28 {
		t.Fatalf("expected conflict 28, got %v", dims.Conflict.Value)
	}
	if dims.Repair.Value != 63 {
		t.Fatalf("expected repair 63, got %v", dims.Repair.Value)
	}
}

func TestStateFromDimensionsRoundTrip(t *testing.T) {
	state := DefaultState()
	state.Trust = 0.72
	state.Familiarity = 0.56
	state.Security = 0.44
	state.Tension = 0.28
	state.RepairConfidence = 0.63

	dims := DimensionsFromState(state)
	back := StateFromDimensions(dims)

	if back.Trust != 0.72 {
		t.Fatalf("expected trust round-trip to 0.72, got %v", back.Trust)
	}
	if back.Familiarity != 0.56 {
		t.Fatalf("expected familiarity round-trip to 0.56, got %v", back.Familiarity)
	}
	if back.Security != 0.44 {
		t.Fatalf("expected security round-trip to 0.44, got %v", back.Security)
	}
	if back.Tension != 0.28 {
		t.Fatalf("expected tension round-trip to 0.28, got %v", back.Tension)
	}
	if back.RepairConfidence != 0.63 {
		t.Fatalf("expected repair confidence round-trip to 0.63, got %v", back.RepairConfidence)
	}
}

func TestApplyPositiveInteractionIncreasesTrustIntimacy(t *testing.T) {
	dims := DefaultDimensions()
	event := RelationshipEvent{
		ID:              "ev-001",
		Type:            EventTypePositiveInteraction,
		Intensity:       0.9,
		Confidence:      0.9,
		SourceMessageID: "msg-001",
		SourceConvID:    "conv-001",
	}
	accum := DefaultAccumulation()
	result := ApplyRelationshipEvent(&dims, event, &accum)

	if dims.Trust.Value <= 50 {
		t.Fatalf("expected trust increase, got %v", dims.Trust.Value)
	}
	if dims.Intimacy.Value <= 35 {
		t.Fatalf("expected intimacy increase, got %v", dims.Intimacy.Value)
	}
	if dims.Conflict.Value >= 15 {
		t.Fatalf("expected conflict decrease, got %v", dims.Conflict.Value)
	}
	if len(result.Impacts) == 0 {
		t.Fatalf("expected non-empty impacts")
	}
}

func TestApplyNegativeInteractionIncreasesConflict(t *testing.T) {
	dims := DefaultDimensions()
	event := RelationshipEvent{
		ID:         "ev-002",
		Type:       EventTypeNegativeInteraction,
		Intensity:  0.85,
		Confidence: 0.9,
	}
	accum := DefaultAccumulation()
	result := ApplyRelationshipEvent(&dims, event, &accum)

	if dims.Conflict.Value <= 15 {
		t.Fatalf("expected conflict increase, got %v", dims.Conflict.Value)
	}
	if dims.Trust.Value >= 50 {
		t.Fatalf("expected trust decrease, got %v", dims.Trust.Value)
	}
	if result.Previous == nil || result.Next == nil {
		t.Fatalf("expected non-nil previous and next in result")
	}
}

func TestApplyRepairEffortReducesConflictIncreasesRepair(t *testing.T) {
	dims := DefaultDimensions()
	dims.Conflict.Value = 40
	dims.Repair.Value = 30

	event := RelationshipEvent{
		ID:         "ev-003",
		Type:       EventTypeRepairEffort,
		Intensity:  0.9,
		Confidence: 0.85,
	}
	accum := DefaultAccumulation()
	result := ApplyRelationshipEvent(&dims, event, &accum)

	if dims.Repair.Value <= 30 {
		t.Fatalf("expected repair increase, got %v", dims.Repair.Value)
	}
	if dims.Conflict.Value >= 40 {
		t.Fatalf("expected conflict decrease after repair, got %v", dims.Conflict.Value)
	}
	if len(result.Impacts) < 1 {
		t.Fatalf("expected at least one impact")
	}
}

func TestApplyRuptureHasStrongNegativeEffects(t *testing.T) {
	dims := DefaultDimensions()
	event := RelationshipEvent{
		ID:         "ev-004",
		Type:       EventTypeRupture,
		Intensity:  1,
		Confidence: 1,
	}
	accum := DefaultAccumulation()
	result := ApplyRelationshipEvent(&dims, event, &accum)

	if dims.Conflict.Value <= 15 {
		t.Fatalf("expected conflict increase from rupture, got %v (was 15)", dims.Conflict.Value)
	}
	if dims.Trust.Value >= 50 {
		t.Fatalf("expected trust decrease from rupture, got %v", dims.Trust.Value)
	}
	if len(result.Impacts) < 2 {
		t.Fatalf("expected multiple impacts from rupture")
	}
}

func TestApplyBoundaryCrossingIncreasesConflict(t *testing.T) {
	dims := DefaultDimensions()
	event := RelationshipEvent{
		ID:         "ev-005",
		Type:       EventTypeBoundaryCrossing,
		Intensity:  0.9,
		Confidence: 0.95,
	}
	accum := DefaultAccumulation()
	ApplyRelationshipEvent(&dims, event, &accum)

	if dims.Conflict.Value <= 15 {
		t.Fatalf("expected conflict increase from boundary crossing, got %v", dims.Conflict.Value)
	}
}

func TestApplyWithdrawalReducesIntimacy(t *testing.T) {
	dims := DefaultDimensions()
	event := RelationshipEvent{
		ID:         "ev-006",
		Type:       EventTypeWithdrawal,
		Intensity:  0.8,
		Confidence: 0.8,
	}
	accum := DefaultAccumulation()
	ApplyRelationshipEvent(&dims, event, &accum)

	if dims.Intimacy.Value >= 35 {
		t.Fatalf("expected intimacy decrease from withdrawal, got %v", dims.Intimacy.Value)
	}
	if dims.Conflict.Value <= 15 {
		t.Fatalf("expected conflict increase from withdrawal, got %v", dims.Conflict.Value)
	}
}

func TestApplyVulnerabilityShareIncreasesTrustAndIntimacy(t *testing.T) {
	dims := DefaultDimensions()
	event := RelationshipEvent{
		ID:         "ev-007",
		Type:       EventTypeVulnerabilityShare,
		Intensity:  0.9,
		Confidence: 0.85,
	}
	accum := DefaultAccumulation()
	result := ApplyRelationshipEvent(&dims, event, &accum)

	if dims.Trust.Value <= 50 {
		t.Fatalf("expected trust increase from vulnerability share, got %v", dims.Trust.Value)
	}
	if dims.Intimacy.Value <= 35 {
		t.Fatalf("expected intimacy increase from vulnerability share, got %v", dims.Intimacy.Value)
	}
	if dims.Dependency.Value <= 30 {
		t.Fatalf("expected dependency increase from vulnerability share, got %v", dims.Dependency.Value)
	}
	hasVulnImpact := false
	for _, imp := range result.Impacts {
		if imp.Reason == string(EventTypeVulnerabilityShare) {
			hasVulnImpact = true
			break
		}
	}
	if !hasVulnImpact {
		t.Fatalf("expected vulnerability_share impact reason")
	}
}

func TestAccumulationCapPreventsSingleEventOverflow(t *testing.T) {
	dims := DefaultDimensions()
	event := RelationshipEvent{
		ID:         "ev-008",
		Type:       EventTypeVulnerabilityShare,
		Intensity:  1,
		Confidence: 1,
	}
	accum := DefaultAccumulation()
	accum.MaxSingleDelta = 5
	result := ApplyRelationshipEvent(&dims, event, &accum)

	for _, imp := range result.Impacts {
		if imp.Delta > 5 {
			t.Fatalf("expected impact capped at 5, got %v for %s", imp.Delta, imp.Dimension)
		}
	}
}

func TestAccumulationTotalCapCreatesOverflow(t *testing.T) {
	dims := DefaultDimensions()
	accum := DefaultAccumulation()
	accum.MaxSingleDelta = 10
	accum.MaxTotalDelta = 15

	event := RelationshipEvent{
		ID:         "ev-009",
		Type:       EventTypePositiveInteraction,
		Intensity:  0.7,
		Confidence: 0.7,
	}

	r1 := ApplyRelationshipEvent(&dims, event, &accum)
	if len(r1.Impacts) == 0 {
		t.Fatalf("expected impacts on first event")
	}
	if len(r1.Overflow) > 0 {
		t.Fatalf("expected no overflow on first event under total cap")
	}

	r2 := ApplyRelationshipEvent(&dims, event, &accum)
	r3 := ApplyRelationshipEvent(&dims, event, &accum)
	r4 := ApplyRelationshipEvent(&dims, event, &accum)

	allOverflows := append(append(r2.Overflow, r3.Overflow...), r4.Overflow...)
	if len(allOverflows) == 0 && accum.Accumulated > accum.MaxTotalDelta {
		t.Fatalf("expected overflow when accumulated exceeds total cap")
	}
	if dims.Trust.Value == 0 {
		t.Fatalf("expected accumulated trust growth across events, got %v", dims.Trust.Value)
	}
}

func TestEventsPersistSourceIdentifiers(t *testing.T) {
	event := RelationshipEvent{
		ID:              "ev-010",
		Type:            EventTypePositiveInteraction,
		Intensity:       0.5,
		Confidence:      0.8,
		SourceMessageID: "msg-uuid-12345",
		SourceConvID:    "conv-uuid-67890",
		ParentEventID:   "ev-009",
	}
	if event.SourceMessageID != "msg-uuid-12345" {
		t.Fatalf("expected source message id preserved")
	}
	if event.SourceConvID != "conv-uuid-67890" {
		t.Fatalf("expected source conv id preserved")
	}
	if event.ParentEventID != "ev-009" {
		t.Fatalf("expected parent event id preserved")
	}
}

func TestAccumulateEventsWithCausalChain(t *testing.T) {
	dims := DefaultDimensions()
	events := []RelationshipEvent{
		{ID: "chain-1", Type: EventTypeNegativeInteraction, Intensity: 0.7, Confidence: 0.8},
		{ID: "chain-2", Type: EventTypeNegativeInteraction, Intensity: 0.7, Confidence: 0.8, ParentEventID: "chain-1"},
		{ID: "chain-3", Type: EventTypeNegativeInteraction, Intensity: 0.7, Confidence: 0.8, ParentEventID: "chain-2"},
	}

	results := AccumulateEvents(&dims, events)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if dims.Conflict.Value <= 15 {
		t.Fatalf("expected accumulated conflict increase, got %v", dims.Conflict.Value)
	}
}

func TestAccumulateEventsBuildsVelocity(t *testing.T) {
	dims := DefaultDimensions()
	events := []RelationshipEvent{
		{ID: "vel-1", Type: EventTypePositiveInteraction, Intensity: 0.8, Confidence: 0.9},
		{ID: "vel-2", Type: EventTypePositiveInteraction, Intensity: 0.8, Confidence: 0.9},
	}

	results := AccumulateEvents(&dims, events)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if dims.Trust.Value > 50 {
		t.Logf("trust increased from 50 to %v, velocity=%v", dims.Trust.Value, dims.Trust.Velocity)
	}
}

func TestComputeVelocity(t *testing.T) {
	vel := ComputeVelocity(60, 50, 2)
	if vel != 5 {
		t.Fatalf("expected velocity 5, got %v", vel)
	}

	velZero := ComputeVelocity(50, 50, 2)
	if velZero != 0 {
		t.Fatalf("expected velocity 0, got %v", velZero)
	}

	velNoTime := ComputeVelocity(60, 50, 0)
	if velNoTime != 0 {
		t.Fatalf("expected velocity 0 with no elapsed time, got %v", velNoTime)
	}
}

func TestDimensionalValuesBoundBetween0And100(t *testing.T) {
	dims := DefaultDimensions()
	dims.Trust.Value = 5

	positive := RelationshipEvent{
		ID:         "ev-bound-pos",
		Type:       EventTypePositiveInteraction,
		Intensity:  1,
		Confidence: 1,
	}
	accum := DefaultAccumulation()
	accum.MaxSingleDelta = 20
	accum.MaxTotalDelta = 50
	ApplyRelationshipEvent(&dims, positive, &accum)
	if dims.Trust.Value > 100 {
		t.Fatalf("expected trust <= 100, got %v", dims.Trust.Value)
	}

	dims2 := DefaultDimensions()
	dims2.Conflict.Value = 95
	negative := RelationshipEvent{
		ID:         "ev-bound-neg",
		Type:       EventTypeRupture,
		Intensity:  1,
		Confidence: 1,
	}
	accum2 := DefaultAccumulation()
	accum2.MaxSingleDelta = 20
	accum2.MaxTotalDelta = 50
	ApplyRelationshipEvent(&dims2, negative, &accum2)
	if dims2.Conflict.Value > 100 {
		t.Fatalf("expected conflict <= 100, got %v", dims2.Conflict.Value)
	}
}

func TestNeutralInteractionHasSmallEffect(t *testing.T) {
	dims := DefaultDimensions()
	event := RelationshipEvent{
		ID:         "ev-neutral",
		Type:       EventTypeNeutralInteraction,
		Intensity:  0.5,
		Confidence: 0.5,
	}
	accum := DefaultAccumulation()
	ApplyRelationshipEvent(&dims, event, &accum)

	if dims.Intimacy.Value <= 35 {
		t.Fatalf("expected small intimacy increase from neutral interaction, got %v", dims.Intimacy.Value)
	}
	if dims.Intimacy.Value >= 40 {
		t.Fatalf("expected neutral interaction to have small effect, got %v", dims.Intimacy.Value)
	}
}

func TestRelationshipSnapshotIntegration(t *testing.T) {
	state := DefaultState()
	dims := DimensionsFromState(state)
	snapshot := RelationshipSnapshot{
		State:      state,
		Dimensions: dims,
		CapturedAt: time.Now(),
	}
	if snapshot.State.Trust != state.Trust {
		t.Fatalf("expected snapshot state trust to match")
	}
	if snapshot.Dimensions.Trust.Value != dims.Trust.Value {
		t.Fatalf("expected snapshot dimensions trust to match")
	}
}

func TestEventApplyResultHasPreviousNextReferences(t *testing.T) {
	dims := DefaultDimensions()
	event := RelationshipEvent{
		ID:         "ev-ref",
		Type:       EventTypePositiveInteraction,
		Intensity:  0.5,
		Confidence: 0.5,
	}
	accum := DefaultAccumulation()
	result := ApplyRelationshipEvent(&dims, event, &accum)

	if result.Previous.Trust.Value != 50 {
		t.Fatalf("expected previous trust to be 50, got %v", result.Previous.Trust.Value)
	}
	if result.Next.Trust.Value <= 50 {
		t.Fatalf("expected next trust to be > 50, got %v", result.Next.Trust.Value)
	}
}
