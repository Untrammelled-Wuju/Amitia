package config

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type fakeConfigStore struct {
	pluginConfig  *ConfigBlob
	runtimeConfig *ConfigBlob
	serviceConfig *ConfigBlob
	loadErr       error
}

func (s *fakeConfigStore) LoadPluginConfig(ctx context.Context, pluginID string) (*ConfigBlob, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.pluginConfig, nil
}

func (s *fakeConfigStore) SavePluginConfig(ctx context.Context, pluginID string, entries []ConfigEntry) error {
	s.pluginConfig = &ConfigBlob{Scope: ConfigScopePlugin, Entries: entries}
	return nil
}

func (s *fakeConfigStore) LoadRuntimeConfig(ctx context.Context, runtimeID string) (*ConfigBlob, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.runtimeConfig, nil
}

func (s *fakeConfigStore) SaveRuntimeConfig(ctx context.Context, runtimeID string, entries []ConfigEntry) error {
	s.runtimeConfig = &ConfigBlob{Scope: ConfigScopeRuntime, Entries: entries}
	return nil
}

func (s *fakeConfigStore) LoadServiceConfig(ctx context.Context, runtimeID, serviceID string) (*ConfigBlob, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.serviceConfig, nil
}

func (s *fakeConfigStore) SaveServiceConfig(ctx context.Context, runtimeID, serviceID string, entries []ConfigEntry) error {
	s.serviceConfig = &ConfigBlob{Scope: ConfigScopeService, Entries: entries}
	return nil
}

func TestResolver_RespectsOverridePriority(t *testing.T) {
	minLen := 1

	store := &fakeConfigStore{
		pluginConfig: &ConfigBlob{
			Scope: ConfigScopePlugin,
			Entries: []ConfigEntry{
				{Key: "timeout", Value: json.RawMessage("30")},
				{Key: "minLen", Value: json.RawMessage("5")},
			},
		},
		runtimeConfig: &ConfigBlob{
			Scope: ConfigScopeRuntime,
			Entries: []ConfigEntry{
				{Key: "timeout", Value: json.RawMessage("60")},
			},
		},
		serviceConfig: &ConfigBlob{
			Scope: ConfigScopeService,
			Entries: []ConfigEntry{
				{Key: "timeout", Value: json.RawMessage("120")},
			},
		},
	}

	schema := NewSchema([]ConfigField{
		{Key: "timeout", Type: ConfigTypeInteger},
		{Key: "minLen", Type: ConfigTypeInteger, MinLength: &minLen},
	})

	resolver := NewResolver(store, schema, nil)
	cfg, errs := resolver.Resolve(context.Background(), "plugin-x", "rt-1", "svc-a")

	if len(errs) > 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	timeoutEntry := findEntry(cfg, "timeout")
	if timeoutEntry == nil {
		t.Fatal("timeout entry not found")
	}

	var timeoutVal int
	if err := json.Unmarshal(timeoutEntry.Value, &timeoutVal); err != nil {
		t.Fatalf("cannot unmarshal timeout value: %v", err)
	}

	if timeoutVal != 120 {
		t.Errorf("expected service-level override (120), got %d", timeoutVal)
	}

	if timeoutEntry.Scope != ConfigScopeService {
		t.Errorf("expected service scope, got %s", timeoutEntry.Scope)
	}
}

func TestResolver_PluginLevelFallback(t *testing.T) {
	store := &fakeConfigStore{
		pluginConfig: &ConfigBlob{
			Scope: ConfigScopePlugin,
			Entries: []ConfigEntry{
				{Key: "name", Value: json.RawMessage(`"plugin-name"`)},
			},
		},
	}

	schema := NewSchema([]ConfigField{
		{Key: "name", Type: ConfigTypeString},
	})

	resolver := NewResolver(store, schema, nil)
	cfg, errs := resolver.Resolve(context.Background(), "plugin-x", "rt-1", "svc-a")

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	nameEntry := findEntry(cfg, "name")
	if nameEntry == nil {
		t.Fatal("name entry not found")
	}

	if nameEntry.Scope != ConfigScopePlugin {
		t.Errorf("expected plugin scope, got %s", nameEntry.Scope)
	}
}

func TestResolver_AppliesSchemaDefaults(t *testing.T) {
	store := &fakeConfigStore{
		pluginConfig: &ConfigBlob{
			Scope:   ConfigScopePlugin,
			Entries: []ConfigEntry{},
		},
	}

	schema := NewSchema([]ConfigField{
		{
			Key:     "mode",
			Type:    ConfigTypeString,
			Default: json.RawMessage(`"auto"`),
		},
	})

	resolver := NewResolver(store, schema, nil)
	cfg, errs := resolver.Resolve(context.Background(), "plugin-x", "", "")

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	modeEntry := findEntry(cfg, "mode")
	if modeEntry == nil {
		t.Fatal("mode entry not found (default was not applied)")
	}

	var mode string
	if err := json.Unmarshal(modeEntry.Value, &mode); err != nil {
		t.Fatalf("cannot unmarshal mode: %v", err)
	}

	if mode != "auto" {
		t.Errorf("expected default 'auto', got %q", mode)
	}
}

func TestResolver_ReportsValidationErrors(t *testing.T) {
	store := &fakeConfigStore{
		pluginConfig: &ConfigBlob{
			Scope: ConfigScopePlugin,
			Entries: []ConfigEntry{
				{Key: "port", Value: json.RawMessage("99999")},
			},
		},
	}

	max := 65535.0
	schema := NewSchema([]ConfigField{
		{Key: "port", Type: ConfigTypeInteger, Maximum: &max},
	})

	resolver := NewResolver(store, schema, nil)
	_, errs := resolver.Resolve(context.Background(), "plugin-x", "", "")

	if len(errs) == 0 {
		t.Error("expected validation errors for out-of-range port")
	}
}

func TestResolver_IgnoresUnknownKeys_WhenNotInSchema(t *testing.T) {
	store := &fakeConfigStore{
		pluginConfig: &ConfigBlob{
			Scope: ConfigScopePlugin,
			Entries: []ConfigEntry{
				{Key: "known", Value: json.RawMessage(`"ok"`)},
				{Key: "unknown", Value: json.RawMessage(`"whatever"`)},
			},
		},
	}

	schema := NewSchema([]ConfigField{
		{Key: "known", Type: ConfigTypeString},
	})

	resolver := NewResolver(store, schema, nil)
	cfg, errs := resolver.Resolve(context.Background(), "plugin-x", "", "")

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if findEntry(cfg, "unknown") == nil {
		t.Error("unknown key should still be present (passthrough)")
	}
}

func TestResolver_SecretRef_DoesNotExposeValues(t *testing.T) {
	store := &fakeConfigStore{
		pluginConfig: &ConfigBlob{
			Scope: ConfigScopePlugin,
			Entries: []ConfigEntry{
				{
					Key:       "api_key",
					SecretRef: &SecretRef{Provider: "vault", Key: "secret/api"},
				},
			},
		},
	}

	schema := NewSchema([]ConfigField{
		{Key: "api_key", Type: ConfigTypeString, Secret: true},
	})

	resolver := NewResolver(store, schema, fakeSecretRegistry{providers: []string{"vault"}})
	cfg, errs := resolver.Resolve(context.Background(), "plugin-x", "", "")

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	entry := findEntry(cfg, "api_key")
	if entry == nil {
		t.Fatal("api_key entry not found")
	}

	if entry.SecretRef == nil {
		t.Fatal("expected SecretRef to be preserved")
	}

	if entry.SecretRef.Provider != "vault" || entry.SecretRef.Key != "secret/api" {
		t.Errorf("secretRef mismatch: %+v", entry.SecretRef)
	}
}

func TestResolver_RejectsUnknownProvider(t *testing.T) {
	store := &fakeConfigStore{
		pluginConfig: &ConfigBlob{
			Scope: ConfigScopePlugin,
			Entries: []ConfigEntry{
				{
					Key:       "api_key",
					SecretRef: &SecretRef{Provider: "unknown", Key: "x"},
				},
			},
		},
	}

	schema := NewSchema([]ConfigField{
		{Key: "api_key", Type: ConfigTypeString, Secret: true},
	})

	resolver := NewResolver(store, schema, fakeSecretRegistry{providers: []string{"vault"}})
	_, errs := resolver.Resolve(context.Background(), "plugin-x", "", "")

	if len(errs) == 0 {
		t.Error("expected validation error for unknown provider")
	}
}

func TestResolver_GeneratesRevision(t *testing.T) {
	store := &fakeConfigStore{
		pluginConfig: &ConfigBlob{
			Scope: ConfigScopePlugin,
			Entries: []ConfigEntry{
				{Key: "a", Value: json.RawMessage("1")},
				{Key: "b", Value: json.RawMessage("2")},
			},
		},
	}

	schema := NewSchema([]ConfigField{
		{Key: "a", Type: ConfigTypeInteger},
		{Key: "b", Type: ConfigTypeInteger},
	})

	resolver := NewResolver(store, schema, nil)
	cfg, _ := resolver.Resolve(context.Background(), "plugin-x", "", "")

	if cfg.Revision == "" {
		t.Error("expected non-empty revision")
	}
}

func TestResolver_RevisionIsStable(t *testing.T) {
	store := &fakeConfigStore{
		pluginConfig: &ConfigBlob{
			Scope: ConfigScopePlugin,
			Entries: []ConfigEntry{
				{Key: "z", Value: json.RawMessage("26")},
				{Key: "a", Value: json.RawMessage("1")},
				{Key: "m", Value: json.RawMessage("13")},
			},
		},
	}

	schema := NewSchema([]ConfigField{
		{Key: "a", Type: ConfigTypeInteger},
		{Key: "m", Type: ConfigTypeInteger},
		{Key: "z", Type: ConfigTypeInteger},
	})

	resolver := NewResolver(store, schema, nil)

	revisions := make(map[string]int)
	for i := 0; i < 5; i++ {
		cfg, _ := resolver.Resolve(context.Background(), "plugin-x", "", "")
		revisions[cfg.Revision]++
		time.Sleep(time.Millisecond)
	}

	if len(revisions) != 1 {
		t.Errorf("expected same revision every time, got: %v", revisions)
	}
}

func TestResolver_EmptyStore(t *testing.T) {
	store := &fakeConfigStore{}
	schema := NewSchema([]ConfigField{
		{Key: "x", Type: ConfigTypeInteger, Default: json.RawMessage("10")},
	})

	resolver := NewResolver(store, schema, nil)
	cfg, errs := resolver.Resolve(context.Background(), "plugin-any", "", "")

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	x := findEntry(cfg, "x")
	if x == nil {
		t.Fatal("default value not applied")
	}
}

func TestResolver_NilStore(t *testing.T) {
	schema := NewSchema([]ConfigField{
		{Key: "x", Type: ConfigTypeInteger, Default: json.RawMessage("10")},
	})

	resolver := NewResolver(nil, schema, nil)
	cfg, errs := resolver.Resolve(context.Background(), "plugin-any", "", "")

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func findEntry(cfg *ScopedConfig, key string) *ConfigEntry {
	if cfg == nil {
		return nil
	}
	for i := range cfg.Entries {
		if cfg.Entries[i].Key == key {
			return &cfg.Entries[i]
		}
	}
	return nil
}
