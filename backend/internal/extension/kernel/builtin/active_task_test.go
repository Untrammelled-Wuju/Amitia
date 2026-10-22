package builtin

import "testing"

func TestActiveTaskBuiltinDefinition(t *testing.T) {
	def := BuildActiveTaskExtension("1.0.0")
	if def.Extension.ID != "com.amitia.builtin.active-task" {
		t.Fatalf("unexpected extension id %s", def.Extension.ID)
	}
	if !def.ShouldEnable() || !def.SystemManaged || def.Required {
		t.Fatalf("active task must be an optional enabled system extension")
	}
	if len(def.Extension.Modules) != 1 {
		t.Fatalf("active task modules=%d", len(def.Extension.Modules))
	}
	module := def.Extension.Modules[0]
	if module.Runtime == nil || module.Runtime.EntryPoint != "host.character.active-task" {
		t.Fatalf("unexpected runtime %#v", module.Runtime)
	}
	if len(module.Contributions) != 1 || module.Contributions[0].Kind != "ui_page" {
		t.Fatalf("active task contribution missing")
	}
}
