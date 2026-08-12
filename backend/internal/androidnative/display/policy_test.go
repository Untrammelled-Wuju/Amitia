package display

import "testing"

func TestDefaultSelectionPolicy(t *testing.T) {
	p := DefaultSelectionPolicy
	if !p.PreferExplicit {
		t.Error("expected PreferExplicit=true")
	}
	if p.AllowDefaultFallback {
		t.Error("expected AllowDefaultFallback=false")
	}
	if !p.RejectAmbiguous {
		t.Error("expected RejectAmbiguous=true")
	}
}

func TestRuntimeBinding(t *testing.T) {
	if RuntimeID != "android_native_display" {
		t.Errorf("expected RuntimeID android_native_display, got %s", RuntimeID)
	}
	if RuntimeBinding.RuntimeType != "android_native" {
		t.Errorf("expected RuntimeType android_native, got %s", RuntimeBinding.RuntimeType)
	}
}

func TestMaxDisplays(t *testing.T) {
	if MaxDisplays != 32 {
		t.Errorf("expected MaxDisplays=32, got %d", MaxDisplays)
	}
}

func TestSelectionPolicyFromConfig(t *testing.T) {
	p := SelectionPolicyFromConfig(true, true)
	if !p.AllowDefaultFallback {
		t.Error("expected AllowDefaultFallback=true")
	}
	if !p.RejectAmbiguous {
		t.Error("expected RejectAmbiguous=true")
	}
	if !p.PreferExplicit {
		t.Error("expected PreferExplicit=true")
	}
}
