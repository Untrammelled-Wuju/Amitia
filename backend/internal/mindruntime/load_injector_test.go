package mindruntime

import (
	"testing"
	"time"
)

func TestInjectLoadBurst(t *testing.T) {
	config := DefaultLoadInjectorConfig()
	config.Profile = LoadProfileBurst
	config.Duration = 30 * time.Second
	config.BurstRate = 50
	config.BurstInterval = 5 * time.Second

	result := InjectLoad(config)

	if result.Profile != LoadProfileBurst {
		t.Fatalf("expected burst profile, got %s", result.Profile)
	}
	if result.TotalMessages == 0 {
		t.Fatal("expected some messages")
	}
	if len(result.TimeSeries) == 0 {
		t.Fatal("expected time series data")
	}
}

func TestInjectLoadSustained(t *testing.T) {
	config := DefaultLoadInjectorConfig()
	config.Profile = LoadProfileSustained
	config.Duration = 10 * time.Second
	config.SustainedRPS = 30

	result := InjectLoad(config)

	if result.Profile != LoadProfileSustained {
		t.Fatalf("expected sustained profile, got %s", result.Profile)
	}
	if result.TotalMessages == 0 {
		t.Fatal("expected some messages")
	}
}

func TestInjectLoadWithFaults(t *testing.T) {
	now := time.Now().UTC()
	config := DefaultLoadInjectorConfig()
	config.Profile = LoadProfileBurst
	config.Duration = 30 * time.Second
	config.Faults = BuildBurstFaults(now)

	result := InjectLoad(config)

	if result.TotalMessages == 0 {
		t.Fatal("expected some messages")
	}
	if len(result.TimeSeries) != 30 {
		t.Fatalf("expected 30 time series points, got %d", len(result.TimeSeries))
	}
}

func TestInjectLoadStep(t *testing.T) {
	config := DefaultLoadInjectorConfig()
	config.Profile = LoadProfileStep
	config.Duration = 10 * time.Second
	config.SustainedRPS = 10
	config.StepIncrement = 5
	config.StepInterval = 3 * time.Second

	result := InjectLoad(config)

	if result.TotalMessages == 0 {
		t.Fatal("expected some messages")
	}
	if result.PeakRPS == 0 {
		t.Fatal("expected peak RPS > 0")
	}
}

func TestBuildBurstFaults(t *testing.T) {
	now := time.Now().UTC()
	faults := BuildBurstFaults(now)

	if len(faults) != 4 {
		t.Fatalf("expected 4 faults, got %d", len(faults))
	}
	if faults[0].FaultType != FaultChannelOffline {
		t.Fatalf("expected channel_offline, got %s", faults[0].FaultType)
	}
}

func TestSortFaultsByStart(t *testing.T) {
	now := time.Now().UTC()
	faults := BuildBurstFaults(now)
	reversed := []InjectedFault{faults[3], faults[2], faults[1], faults[0]}
	sorted := SortFaultsByStart(reversed)

	if sorted[0].FaultType != FaultChannelOffline {
		t.Fatalf("expected channel_offline first, got %s", sorted[0].FaultType)
	}
}
