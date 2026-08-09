package config

import (
	"encoding/json"
	"testing"
)

func TestScopedConfig_HasSecrets_DetectsSecretKeys(t *testing.T) {
	sc := &ScopedConfig{
		Entries: []ConfigEntry{
			{Key: "name", Value: json.RawMessage(`"hello"`)},
			{Key: "token", SecretRef: &SecretRef{Provider: "vault", Key: "api"}},
		},
	}

	if !sc.HasSecrets() {
		t.Error("expected HasSecrets to return true when SecretRef is present")
	}

	if !stringContains(sc.SecretKeys(), "token") {
		t.Errorf("expected SecretKeys to include 'token', got %v", sc.SecretKeys())
	}

	if stringContains(sc.SecretKeys(), "name") {
		t.Errorf("should not include non-secret key 'name'")
	}
}

func TestScopedConfig_HasSecrets_EmptyRef(t *testing.T) {
	sc := &ScopedConfig{
		Entries: []ConfigEntry{
			{Key: "token", SecretRef: &SecretRef{Provider: "", Key: ""}},
		},
	}

	if sc.HasSecrets() {
		t.Error("expected HasSecrets to return false for empty SecretRef")
	}
}

func TestScopedConfig_HasSecrets_NoEntries(t *testing.T) {
	sc := &ScopedConfig{}
	if sc.HasSecrets() {
		t.Error("expected HasSecrets to return false for empty config")
	}
}

func stringContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
