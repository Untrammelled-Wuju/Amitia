package capability

import (
	"testing"
)

func TestEffectiveResourceLimits_ForDimension(t *testing.T) {
	limits := EffectiveResourceLimits{
		Limits: []EffectiveResourceLimit{
			{Dimension: ResourceMemoryBytes, Limit: 256, Mode: ResourceEnforcementEnforced, Source: ResourceLimitSourceRuntime},
			{Dimension: ResourceCPUPercent, Limit: 50, Mode: ResourceEnforcementDeclaredOnly, Source: ResourceLimitSourceToolPolicy},
		},
	}

	if l, ok := limits.ForDimension(ResourceMemoryBytes); !ok || l.Limit != 256 {
		t.Fatalf("expected memory 256, got %v ok=%v", l, ok)
	}
	if _, ok := limits.ForDimension(ResourceProcesses); ok {
		t.Fatal("expected no processes limit")
	}
}

func TestEffectiveResourceLimits_StrictestLimit(t *testing.T) {
	limits := EffectiveResourceLimits{
		Limits: []EffectiveResourceLimit{
			{Dimension: ResourceMemoryBytes, Limit: 512, Mode: ResourceEnforcementDeclaredOnly, Source: ResourceLimitSourceToolPolicy},
			{Dimension: ResourceMemoryBytes, Limit: 256, Mode: ResourceEnforcementEnforced, Source: ResourceLimitSourceRuntime},
		},
	}

	best, ok := limits.StrictestLimit(ResourceMemoryBytes)
	if !ok {
		t.Fatal("expected to find strictest limit")
	}
	if best.Limit != 256 {
		t.Fatalf("expected 256 strictest, got %d", best.Limit)
	}
	if best.Mode != ResourceEnforcementEnforced {
		t.Fatalf("expected enforced mode, got %s", best.Mode)
	}
}

func TestEffectiveResourceLimits_StrictestLimit_PrefersEnforcedWhenEqual(t *testing.T) {
	limits := EffectiveResourceLimits{
		Limits: []EffectiveResourceLimit{
			{Dimension: ResourceMemoryBytes, Limit: 256, Mode: ResourceEnforcementEnforced, Source: ResourceLimitSourceRuntime},
			{Dimension: ResourceMemoryBytes, Limit: 512, Mode: ResourceEnforcementDeclaredOnly, Source: ResourceLimitSourceToolPolicy},
		},
	}

	best, ok := limits.StrictestLimit(ResourceMemoryBytes)
	if !ok || best.Limit != 256 {
		t.Fatalf("expected 256 enforced, got %v ok=%v", best, ok)
	}
}

func TestEffectiveResourceLimits_SkipsUnsupported(t *testing.T) {
	limits := EffectiveResourceLimits{
		Limits: []EffectiveResourceLimit{
			{Dimension: ResourceMemoryBytes, Limit: 128, Mode: ResourceEnforcementUnsupported, Source: ResourceLimitSourceRuntime},
			{Dimension: ResourceMemoryBytes, Limit: 512, Mode: ResourceEnforcementDeclaredOnly, Source: ResourceLimitSourceToolPolicy},
		},
	}

	best, ok := limits.StrictestLimit(ResourceMemoryBytes)
	if !ok || best.Limit != 512 {
		t.Fatalf("expected 512 (declared), got %v ok=%v", best, ok)
	}
}

func TestEffectiveResourceLimits_SkipsZeroOrNegative(t *testing.T) {
	limits := EffectiveResourceLimits{
		Limits: []EffectiveResourceLimit{
			{Dimension: ResourceMemoryBytes, Limit: 0, Mode: ResourceEnforcementEnforced, Source: ResourceLimitSourceToolPolicy},
			{Dimension: ResourceMemoryBytes, Limit: -1, Mode: ResourceEnforcementEnforced, Source: ResourceLimitSourceRuntime},
		},
	}

	if _, ok := limits.StrictestLimit(ResourceMemoryBytes); ok {
		t.Fatal("expected no valid limit for zero/negative")
	}
}

func TestEnforcementModeValues(t *testing.T) {
	if string(ResourceEnforcementUnsupported) != "unsupported" {
		t.Fatalf("expected unsupported, got %s", ResourceEnforcementUnsupported)
	}
	if string(ResourceEnforcementEnforced) != "enforced" {
		t.Fatalf("expected enforced, got %s", ResourceEnforcementEnforced)
	}
}

func TestResourceUsage_UnknownNotZero(t *testing.T) {
	usage := ResourceUsage{}
	if len(usage.MeasuredDimensions) != 0 {
		t.Fatal("expected no measured dimensions")
	}

	usage2 := ResourceUsage{
		PeakMemoryBytes:    256,
		MeasuredDimensions: []ResourceDimension{ResourceMemoryBytes},
	}
	if usage2.PeakMemoryBytes != 256 {
		t.Fatalf("expected 256, got %d", usage2.PeakMemoryBytes)
	}
}

func TestResourceLimitSourceValues(t *testing.T) {
	sources := []ResourceLimitSource{
		ResourceLimitSourceToolPolicy,
		ResourceLimitSourceRuntime,
		ResourceLimitSourceRuntimeSupervisor,
		ResourceLimitSourceProvider,
		ResourceLimitSourcePlatform,
	}
	if len(sources) != 5 {
		t.Fatal("expected 5 sources")
	}
}

func TestResourceLimitRequirementValues(t *testing.T) {
	if string(ResourceLimitBestEffort) != "best_effort" {
		t.Fatalf("expected best_effort, got %s", ResourceLimitBestEffort)
	}
	if string(ResourceLimitRequireEnforced) != "require_enforced" {
		t.Fatalf("expected require_enforced, got %s", ResourceLimitRequireEnforced)
	}
}
