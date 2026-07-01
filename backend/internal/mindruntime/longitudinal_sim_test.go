package mindruntime

import (
	"testing"
	"time"
)

func TestRunLongitudinalSim(t *testing.T) {
	config := LongitudinalSimConfig{
		SimDuration:      6 * time.Hour,
		StepInterval:     time.Hour,
		BehaviorsPerStep: 5,
		Seed:             42,
		Roles: []SimRoleConfig{
			{RoleID: "r1", CharacterID: "c1", Frequency: SimFreqHigh, PersonalityKind: "warm", Enabled: true, SafetyCap: 0.8},
			{RoleID: "r2", CharacterID: "c2", Frequency: SimFreqMedium, PersonalityKind: "cool", Enabled: true, SafetyCap: 0.9},
			{RoleID: "r3", CharacterID: "c3", Frequency: SimFreqLow, PersonalityKind: "neutral", Enabled: true, SafetyCap: 0.85},
		},
	}

	result := RunLongitudinalSim(config)

	if result.TotalSteps != 6 {
		t.Fatalf("expected 6 steps, got %d", result.TotalSteps)
	}
	if len(result.Steps) != 6 {
		t.Fatalf("expected 6 step results, got %d", len(result.Steps))
	}
	if result.Aggregations.TotalMessages == 0 {
		t.Fatal("expected some messages")
	}
	if result.Parameters.Version < 1 {
		t.Fatal("expected calibrated params")
	}
}

func TestLongitudinalSimEmptyRoles(t *testing.T) {
	config := DefaultLongitudinalSimConfig()
	result := RunLongitudinalSim(config)
	if result.TotalSteps != 24 {
		t.Fatalf("expected 24 steps, got %d", result.TotalSteps)
	}
	if result.Aggregations.TotalMessages != 0 {
		t.Fatalf("expected 0 messages with no roles, got %d", result.Aggregations.TotalMessages)
	}
}

func TestComparePersonalityTemplates(t *testing.T) {
	templates := []SimRoleConfig{
		{RoleID: "r1", CharacterID: "c1", Frequency: SimFreqHigh, PersonalityKind: "warm", Enabled: true, SafetyCap: 0.8},
		{RoleID: "r2", CharacterID: "c2", Frequency: SimFreqHigh, PersonalityKind: "cool", Enabled: true, SafetyCap: 0.9},
	}
	config := LongitudinalSimConfig{
		SimDuration:      2 * time.Hour,
		StepInterval:     time.Hour,
		BehaviorsPerStep: 3,
		Seed:             7,
	}

	results := ComparePersonalityTemplates(templates, config)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].TotalSteps != 2 {
		t.Fatalf("expected 2 steps, got %d", results[0].TotalSteps)
	}
}
