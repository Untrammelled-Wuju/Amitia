package config

import (
	"encoding/json"
	"testing"
)

func TestComputeRevision_StableForSameInput(t *testing.T) {
	entries := []ConfigEntry{
		{Key: "a", Value: json.RawMessage("1"), Scope: ConfigScopePlugin},
		{Key: "b", Value: json.RawMessage("2"), Scope: ConfigScopeRuntime},
		{Key: "c", Value: json.RawMessage("3"), Scope: ConfigScopeService},
	}

	r1 := ComputeRevision(entries)
	r2 := ComputeRevision(entries)
	if r1 != r2 {
		t.Errorf("expected same revision, got %q vs %q", r1, r2)
	}
}

func TestComputeRevision_OrderIndependent(t *testing.T) {
	entries1 := []ConfigEntry{
		{Key: "z", Value: json.RawMessage("26")},
		{Key: "a", Value: json.RawMessage("1")},
		{Key: "m", Value: json.RawMessage("13")},
	}

	entries2 := []ConfigEntry{
		{Key: "a", Value: json.RawMessage("1")},
		{Key: "m", Value: json.RawMessage("13")},
		{Key: "z", Value: json.RawMessage("26")},
	}

	r1 := ComputeRevision(entries1)
	r2 := ComputeRevision(entries2)

	if r1 != r2 {
		t.Errorf("revision should be order-independent, got %q vs %q", r1, r2)
	}
}

func TestComputeRevision_IncludesScope(t *testing.T) {
	entriesPlugin := []ConfigEntry{
		{Key: "x", Value: json.RawMessage("1"), Scope: ConfigScopePlugin},
	}
	entriesService := []ConfigEntry{
		{Key: "x", Value: json.RawMessage("1"), Scope: ConfigScopeService},
	}

	rPlugin := ComputeRevision(entriesPlugin)
	rService := ComputeRevision(entriesService)

	if rPlugin == rService {
		t.Error("same key/value but different scopes should produce different revisions")
	}
}

func TestComputeRevision_IncludesValue(t *testing.T) {
	entries1 := []ConfigEntry{
		{Key: "x", Value: json.RawMessage("1")},
	}
	entries2 := []ConfigEntry{
		{Key: "x", Value: json.RawMessage("2")},
	}

	r1 := ComputeRevision(entries1)
	r2 := ComputeRevision(entries2)

	if r1 == r2 {
		t.Error("different values for the same key should produce different revisions")
	}
}

func TestComputeRevision_Empty(t *testing.T) {
	r := ComputeRevision(nil)
	if r == "" {
		t.Error("expected non-empty revision for empty entries")
	}

	r2 := ComputeRevision([]ConfigEntry{})
	if r2 == "" {
		t.Error("expected non-empty revision for empty slice")
	}
}

func TestComputeRevision_WithSecretRef(t *testing.T) {
	entries1 := []ConfigEntry{
		{
			Key:       "token",
			SecretRef: &SecretRef{Provider: "vault", Key: "secret/api"},
		},
	}

	entries2 := []ConfigEntry{
		{
			Key:       "token",
			SecretRef: &SecretRef{Provider: "vault", Key: "secret/other"},
		},
	}

	r1 := ComputeRevision(entries1)
	r2 := ComputeRevision(entries2)

	if r1 == r2 {
		t.Error("different secret refs should produce different revisions")
	}
}

func TestComputeRevision_Format(t *testing.T) {
	entries := []ConfigEntry{
		{Key: "x", Value: json.RawMessage("1")},
	}

	r := ComputeRevision(entries)
	if len(r) < 5 || r[:5] != "crev-" {
		t.Errorf("expected revision to start with 'crev-', got %q", r)
	}
}

