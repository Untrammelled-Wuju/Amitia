package ui_provider

import "testing"

func TestBuiltinProvidersValidate(t *testing.T) {
	registry := NewRegistryWithBuiltins()
	if _, ok := registry.Get("builtin.amitia.workspace-sidebar"); !ok {
		t.Fatal("workspace sidebar builtin was rejected")
	}
}
