package plugin_boundary

import "testing"

func TestResourceDescriptorValidation(t *testing.T) {
	t.Run("missing display name", func(t *testing.T) {
		d := ResourceDescriptor{}
		if err := d.Validate(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid asset kind", func(t *testing.T) {
		d := ResourceDescriptor{DisplayName: "Avatar", AssetKind: "hack3d"}
		if err := d.Validate(); err == nil {
			t.Fatal("expected error for invalid asset kind")
		}
	})

	t.Run("valid kinds", func(t *testing.T) {
		for _, k := range []string{"sprite", "animation", "frame_set", "config_template", "manifest", "preview"} {
			d := ResourceDescriptor{DisplayName: "X", AssetKind: k}
			if err := d.Validate(); err != nil {
				t.Fatalf("kind %s should validate: %v", k, err)
			}
		}
	})

	t.Run("empty asset kind allowed", func(t *testing.T) {
		d := ResourceDescriptor{DisplayName: "X"}
		if err := d.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestActionDescriptorValidation(t *testing.T) {
	t.Run("missing action key", func(t *testing.T) {
		d := ActionDescriptor{DisplayName: "Wave"}
		if err := d.Validate(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing display name", func(t *testing.T) {
		d := ActionDescriptor{ActionKey: "wave"}
		if err := d.Validate(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid action key chars", func(t *testing.T) {
		d := ActionDescriptor{ActionKey: "wave!", DisplayName: "Wave"}
		if err := d.Validate(); err == nil {
			t.Fatal("expected error for invalid action key")
		}
	})

	t.Run("valid action key", func(t *testing.T) {
		for _, k := range []string{"wave", "wave_left", "wave.v2", "my-action"} {
			d := ActionDescriptor{ActionKey: k, DisplayName: "Wave"}
			if err := d.Validate(); err != nil {
				t.Fatalf("action key %q should validate: %v", k, err)
			}
		}
	})
}

func TestValidateResourceRef(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if err := validateResourceRef(""); err != nil {
			t.Errorf("err: %v", err)
		}
	})
	t.Run("relative", func(t *testing.T) {
		if err := validateResourceRef("assets/wave.png"); err != nil {
			t.Errorf("err: %v", err)
		}
	})
	t.Run("absolute denied", func(t *testing.T) {
		if err := validateResourceRef("/etc/passwd"); err == nil {
			t.Error("expected deny for absolute path")
		}
	})
	t.Run("traversal denied", func(t *testing.T) {
		if err := validateResourceRef("../../../../secret"); err == nil {
			t.Error("expected deny for traversal")
		}
	})
	t.Run("tilde denied", func(t *testing.T) {
		if err := validateResourceRef("~/something"); err == nil {
			t.Error("expected deny for tilde")
		}
	})
	t.Run("nested relative ok", func(t *testing.T) {
		if err := validateResourceRef("a/b/c/d.png"); err != nil {
			t.Errorf("err: %v", err)
		}
	})
}

func TestContributionRegistryView(t *testing.T) {
	regs := []ContributionRegistration{
		{Ref: ContributionRef{ExtensionID: "com.a/pet", PluginID: "p1", ContributionID: "r1"}, Kind: KindResource},
		{Ref: ContributionRef{ExtensionID: "com.a/pet", PluginID: "p1", ContributionID: "a1"}, Kind: KindAction},
		{Ref: ContributionRef{ExtensionID: "com.a/pet", PluginID: "p2", ContributionID: "r2"}, Kind: KindResource},
		{Ref: ContributionRef{ExtensionID: "com.b/pet", PluginID: "p3", ContributionID: "a2"}, Kind: KindAction},
	}
	view := NewContributionRegistryView(regs)

	if got := len(view.FindByExt("com.a/pet")); got != 3 {
		t.Errorf("ext 'com.a/pet' count=%d want 3", got)
	}
	if got := len(view.FindByExt("com.b/pet")); got != 1 {
		t.Errorf("ext 'com.b/pet' count=%d want 1", got)
	}
	if got := len(view.FindByExt("nope")); got != 0 {
		t.Errorf("ext 'nope' count=%d want 0", got)
	}
	if got := len(view.FindByPlugin("com.a/pet", "p1")); got != 2 {
		t.Errorf("plugin 'com.a/pet/p1' count=%d want 2", got)
	}
	if got := len(view.FindByPlugin("com.a/pet", "p2")); got != 1 {
		t.Errorf("plugin 'com.a/pet/p2' count=%d want 1", got)
	}

	lookup := ContributionRef{ExtensionID: "com.a/pet", PluginID: "p1", ContributionID: "a1"}
	reg, ok := view.FindByRef(lookup)
	if !ok {
		t.Fatal("expected to find a1")
	}
	if reg.Kind != KindAction {
		t.Errorf("kind=%v want KindAction", reg.Kind)
	}

	_, ok = view.FindByRef(ContributionRef{ExtensionID: "com.a/pet", PluginID: "p1", ContributionID: "zzz"})
	if ok {
		t.Fatal("expected not found for missing ref")
	}
}

func TestContributionRegistrationStates(t *testing.T) {
	t.Run("active is executable and not static", func(t *testing.T) {
		r := ContributionRegistration{Status: ContributionStatusActive}
		if !r.IsExecutable() {
			t.Error("active should be executable")
		}
		if r.IsStatic() {
			t.Error("active should not be static")
		}
	})
	t.Run("detached is not executable but static", func(t *testing.T) {
		r := ContributionRegistration{Status: ContributionStatusDetached}
		if r.IsExecutable() {
			t.Error("detached should not be executable")
		}
		if !r.IsStatic() {
			t.Error("detached should be static")
		}
	})
	t.Run("disabled is static", func(t *testing.T) {
		r := ContributionRegistration{Status: ContributionStatusDisabled}
		if !r.IsStatic() {
			t.Error("disabled should be static")
		}
	})
	t.Run("registered is not executable", func(t *testing.T) {
		r := ContributionRegistration{Status: ContributionStatusRegistered}
		if r.IsExecutable() {
			t.Error("registered should not be executable")
		}
	})
}
