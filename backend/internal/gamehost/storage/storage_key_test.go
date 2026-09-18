package storage

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestStorageKeyForPluginID_Empty(t *testing.T) {
	_, err := StorageKeyForPluginID("")
	if err == nil {
		t.Fatal("expected error for empty plugin id")
	}
}

func TestStorageKeyForRuntimeID_Empty(t *testing.T) {
	_, err := StorageKeyForRuntimeID("")
	if err == nil {
		t.Fatal("expected error for empty runtime id")
	}
}

func TestStorageKeyForServiceID_Empty(t *testing.T) {
	_, err := StorageKeyForServiceID("", "svc")
	if err == nil {
		t.Fatal("expected error for empty runtime id")
	}

	_, err = StorageKeyForServiceID("rt", "")
	if err == nil {
		t.Fatal("expected error for empty service id")
	}
}

func TestStorageKey_Stable(t *testing.T) {
	id := domain.PluginID("com.example.game")
	first, err := StorageKeyForPluginID(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 100; i++ {
		next, err := StorageKeyForPluginID(id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if first != next {
			t.Fatalf("storage key not stable: %s vs %s", first, next)
		}
	}
}

func TestStorageKey_UniqueForDifferentIDs(t *testing.T) {
	ids := []domain.PluginID{
		"com.example.plugin-a",
		"com.example.plugin-b",
		"org.test.alpha",
		"org.test.beta",
	}
	seen := make(map[StorageKey]domain.PluginID)
	for _, id := range ids {
		key, err := StorageKeyForPluginID(id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prev, exists := seen[key]; exists {
			t.Fatalf("duplicate key %s for ids %s and %s", key, prev, id)
		}
		seen[key] = id
	}
}

func TestStorageKey_UniquenessUnderRuntimeID(t *testing.T) {
	runtimeA := domain.RuntimeInstanceID("runtime-a")
	runtimeB := domain.RuntimeInstanceID("runtime-b")
	serviceID := domain.ServiceID("bridge")

	keyA, err := StorageKeyForServiceID(runtimeA, serviceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	keyB, err := StorageKeyForServiceID(runtimeB, serviceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keyA == keyB {
		t.Fatalf("same service key for different runtimes: %s", keyA)
	}
}

func TestStorageKey_CaseCollision(t *testing.T) {
	upper, err := StorageKeyForPluginID("PluginA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lower, err := StorageKeyForPluginID("plugina")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upper == lower {
		t.Fatalf("case collision: %s == %s", upper, lower)
	}
}

func TestStorageKey_RuntimeKeyDerivedFromRuntimeID(t *testing.T) {
	pluginKey, err := StorageKeyForPluginID("com.example.app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	runtimeKey, err := StorageKeyForRuntimeID("runtime-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(pluginKey) == string(runtimeKey) {
		t.Fatalf("plugin and runtime keys should differ")
	}
}

func TestStorageKey_Validate_Empty(t *testing.T) {
	err := ValidateStorageKey("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestStorageKey_Validate_AbsolutePath(t *testing.T) {
	err := ValidateStorageKey(StorageKey("/etc/passwd"))
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestStorageKey_Validate_DoubleDot(t *testing.T) {
	err := ValidateStorageKey(StorageKey("../../../etc/passwd"))
	if err == nil {
		t.Fatal("expected error for double dot")
	}
}

func TestStorageKey_Validate_Backslash(t *testing.T) {
	err := ValidateStorageKey(StorageKey("a\\b\\c"))
	if err == nil {
		t.Fatal("expected error for backslash")
	}
}

func TestStorageKey_Validate_NullByte(t *testing.T) {
	err := ValidateStorageKey(StorageKey("a\x00b"))
	if err == nil {
		t.Fatal("expected error for null byte")
	}
}

func TestStorageKey_HasPrefix(t *testing.T) {
	pluginKey, _ := StorageKeyForPluginID("com.example.test")
	if !strings.HasPrefix(string(pluginKey), "plg-") {
		t.Fatalf("plugin key should have plg- prefix: %s", pluginKey)
	}

	runtimeKey, _ := StorageKeyForRuntimeID("rt-1")
	if !strings.HasPrefix(string(runtimeKey), "run-") {
		t.Fatalf("runtime key should have run- prefix: %s", runtimeKey)
	}

	serviceKey, _ := StorageKeyForServiceID("rt-1", "svc-1")
	if !strings.HasPrefix(string(serviceKey), "svc-") {
		t.Fatalf("service key should have svc- prefix: %s", serviceKey)
	}
}

func TestStorageKey_NoRandomElements(t *testing.T) {
	id := domain.PluginID("stable-test-id")
	keys := make(map[StorageKey]int)
	for i := 0; i < 50; i++ {
		key, err := StorageKeyForPluginID(id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		keys[key]++
	}
	if len(keys) != 1 {
		t.Fatalf("expected exactly 1 unique key, got %d", len(keys))
	}
}

func TestStorageKey_SafeCharacters(t *testing.T) {
	ids := []domain.PluginID{
		"simple-id",
		"id_with_underscore",
		"id.with.dots",
		"id-with-dashes",
		"mixed_case-ID",
		"123numeric",
	}
	for _, id := range ids {
		key, err := StorageKeyForPluginID(id)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", id, err)
		}
		if err := ValidateStorageKey(key); err != nil {
			t.Fatalf("invalid storage key %s for %s: %v", key, id, err)
		}
	}
}
