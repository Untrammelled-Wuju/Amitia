package plugin_boundary

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestContributionRef(t *testing.T) {
	t.Run("validate non-empty passes", func(t *testing.T) {
		ref := ContributionRef{
			ExtensionID:    "com.example/pet",
			PluginID:       "plugin_1",
			ContributionID: "wave_action",
		}
		if err := ref.Validate(); err != nil {
			t.Fatalf("unexpected validate error: %v", err)
		}
	})

	t.Run("validate empty extension fails", func(t *testing.T) {
		ref := ContributionRef{PluginID: "plugin_1", ContributionID: "wave_action"}
		if err := ref.Validate(); err == nil {
			t.Fatal("expected validate error for empty extension")
		}
	})

	t.Run("validate empty plugin fails", func(t *testing.T) {
		ref := ContributionRef{ExtensionID: "com.example/pet", ContributionID: "wave_action"}
		if err := ref.Validate(); err == nil {
			t.Fatal("expected validate error for empty plugin")
		}
	})

	t.Run("validate empty contribution fails", func(t *testing.T) {
		ref := ContributionRef{ExtensionID: "com.example/pet", PluginID: "plugin_1"}
		if err := ref.Validate(); err == nil {
			t.Fatal("expected validate error for empty contribution")
		}
	})
}

func TestContributionRefKey(t *testing.T) {
	ref := ContributionRef{
		ExtensionID:    "com.example/pet",
		PluginID:       "plugin_1",
		ContributionID: "wave_action",
	}
	want := "com.example/pet/plugin_1/wave_action"
	if got := ref.Key(); got != want {
		t.Fatalf("Key()=%q want %q", got, want)
	}
}

func TestContributionRefFromDefinition(t *testing.T) {
	contrib := domain.ContributionDefinition{
		ID:          "wave_action",
		ModuleID:    "module_pet",
		ExtensionID: "com.example/pet",
	}
	ref := ContributionRefFromDefinition(contrib)
	if ref.PluginID != "wave_action" {
		t.Errorf("PluginID=%q want wave_action", ref.PluginID)
	}
	if ref.ExtensionID != "com.example/pet" {
		t.Errorf("ExtensionID=%q want com.example/pet", ref.ExtensionID)
	}
}

func TestParseContributionRef(t *testing.T) {
	t.Run("valid triple", func(t *testing.T) {
		ref, err := ParseContributionRef("com.example/pet/plugin_1/wave_action")
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if ref.ExtensionID != "com.example/pet" || ref.PluginID != "plugin_1" || ref.ContributionID != "wave_action" {
			t.Fatalf("parsed ref mismatch: %+v", ref)
		}
	})

	t.Run("too few parts", func(t *testing.T) {
		if _, err := ParseContributionRef("com.example/pet"); err == nil {
			t.Fatal("expected error for too few parts")
		}
	})

	t.Run("extra parts preserved in contributionId", func(t *testing.T) {
		ref, err := ParseContributionRef("com.example/pet/plugin_1/with/slash")
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if ref.ContributionID != "with/slash" {
			t.Errorf("ContributionID=%q want with/slash", ref.ContributionID)
		}
	})
}

func TestContributionKindParsing(t *testing.T) {
	cases := map[string]ContributionKind{
		"pet_resource":                   KindResource,
		"pet_action":                     KindAction,
		"pet_runtime_capability":         KindRuntime,
		"pet_floating_window_capability": KindFloatingWindow,
		"":                               KindUnknown,
		"garbage":                        KindUnknown,
	}
	for input, want := range cases {
		if got := ParseContributionKind(input); got != want {
			t.Errorf("ParseContributionKind(%q)=%q want %q", input, got, want)
		}
	}
}

func TestContributionOwnership(t *testing.T) {
	owner := domain.ExtensionID("com.example/pet")
	o := ContributionOwnership{
		Ref:        ContributionRef{ExtensionID: owner, PluginID: "p1", ContributionID: "a1"},
		OwnerExtID: owner,
	}
	if !o.BelongsTo("com.example/pet") {
		t.Fatal("expected to belong")
	}
	if o.BelongsTo("com.other/pet") {
		t.Fatal("expected not to belong")
	}
}
