package builtin

import "testing"

func TestFeaturePluginsAreOptionalWithExpectedDefaults(t *testing.T) {
	cases := []struct {
		def     Definition
		enabled bool
	}{
		{def: BuildEmotionExtension("1.0.0")},
		{def: BuildLifestyleExtension("1.0.0")},
	}
	for _, item := range cases {
		def := item.def
		if def.Required {
			t.Fatalf("%s must not be required", def.Extension.ID)
		}
		if !def.DisableAllowed {
			t.Fatalf("%s must be disableable", def.Extension.ID)
		}
		if !def.SystemManaged {
			t.Fatalf("%s must be system managed", def.Extension.ID)
		}
		if def.ShouldEnable() != item.enabled {
			t.Fatalf("%s default enabled=%v, want %v", def.Extension.ID, def.ShouldEnable(), item.enabled)
		}
		if len(def.Extension.Modules) != 1 || len(def.Extension.Modules[0].Contributions) != 1 {
			t.Fatalf("%s must expose one character detail tab", def.Extension.ID)
		}
		contribution := def.Extension.Modules[0].Contributions[0]
		if contribution.Kind != "ui_page" {
			t.Fatalf("%s unexpected contribution kind %s", def.Extension.ID, contribution.Kind)
		}
		slot, _ := contribution.Definition["slot"].(map[string]any)
		if slot["slot_id"] != "character.detail.tab" {
			t.Fatalf("%s must target character.detail.tab", def.Extension.ID)
		}
	}
}
