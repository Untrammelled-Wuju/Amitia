package config

import (
	"testing"
)

func TestNewSchema_BuildsFieldIndex(t *testing.T) {
	fields := []ConfigField{
		{Key: "timeout", Type: ConfigTypeInteger},
		{Key: "name", Type: ConfigTypeString},
		{Key: "debug", Type: ConfigTypeBoolean},
	}

	schema := NewSchema(fields)

	if schema.SchemaVersion != 1 {
		t.Errorf("expected schema version 1, got %d", schema.SchemaVersion)
	}

	if len(schema.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(schema.Fields))
	}

	for _, f := range fields {
		got, ok := schema.Field(f.Key)
		if !ok {
			t.Errorf("expected field %q to exist", f.Key)
			continue
		}
		if got.Key != f.Key {
			t.Errorf("field key mismatch: got %q, want %q", got.Key, f.Key)
		}
	}
}

func TestConfigSchema_FieldNotFound(t *testing.T) {
	schema := NewSchema([]ConfigField{
		{Key: "known", Type: ConfigTypeString},
	})

	if _, ok := schema.Field("unknown"); ok {
		t.Error("expected unknown field to return false")
	}
}

func TestConfigSchema_HasField(t *testing.T) {
	schema := NewSchema([]ConfigField{
		{Key: "alpha", Type: ConfigTypeString},
	})

	if !schema.HasField("alpha") {
		t.Error("expected HasField(alpha) to be true")
	}
	if schema.HasField("beta") {
		t.Error("expected HasField(beta) to be false")
	}
}

func TestConfigSchema_Clone(t *testing.T) {
	original := NewSchema([]ConfigField{
		{Key: "x", Type: ConfigTypeInteger},
		{Key: "y", Type: ConfigTypeInteger},
	})

	cloned := original.Clone()

	if cloned == original {
		t.Error("clone should be a different pointer")
	}

	if len(cloned.Fields) != len(original.Fields) {
		t.Errorf("clone field count mismatch: got %d, want %d", len(cloned.Fields), len(original.Fields))
	}

	cloned.Fields[0].Key = "modified"
	if original.Fields[0].Key == "modified" {
		t.Error("mutating clone should not affect original")
	}
}

func TestScopePriority(t *testing.T) {
	cases := []struct {
		scope ConfigScope
		want  int
	}{
		{ConfigScopePlugin, 0},
		{ConfigScopeRuntime, 1},
		{ConfigScopeService, 2},
		{ConfigScope("unknown"), -1},
	}

	for _, tc := range cases {
		got := ScopePriority(tc.scope)
		if got != tc.want {
			t.Errorf("ScopePriority(%q) = %d, want %d", tc.scope, got, tc.want)
		}
	}

	if ScopePriority(ConfigScopeService) <= ScopePriority(ConfigScopeRuntime) {
		t.Error("service priority should be higher than runtime")
	}
	if ScopePriority(ConfigScopeRuntime) <= ScopePriority(ConfigScopePlugin) {
		t.Error("runtime priority should be higher than plugin")
	}
}
