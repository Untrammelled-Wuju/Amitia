package deepsearch

import (
	"strings"
	"testing"
)

func TestBuildTaskDefinition_Defaults(t *testing.T) {
	def := BuildTaskDefinition("")
	if def.TaskID != DeepSearchTaskID {
		t.Fatalf("expected TaskID %q, got %q", DeepSearchTaskID, def.TaskID)
	}
	if def.ExtensionID != DeepSearchExtensionID {
		t.Fatalf("expected ExtensionID %q, got %q", DeepSearchExtensionID, def.ExtensionID)
	}
	if def.ModuleID != DeepSearchModuleID {
		t.Fatalf("expected ModuleID %q, got %q", DeepSearchModuleID, def.ModuleID)
	}
	if def.Entry != DeepSearchEntry {
		t.Fatalf("expected Entry %q, got %q", DeepSearchEntry, def.Entry)
	}
	if !def.Checkpoint {
		t.Fatal("expected Checkpoint=true")
	}
	if def.Idempotency != "conditionally_idempotent" {
		t.Fatalf("expected Idempotency=conditionally_idempotent, got %q", def.Idempotency)
	}
	if def.Recoverability != "checkpoint_recoverable" {
		t.Fatalf("expected Recoverability=checkpoint_recoverable, got %q", def.Recoverability)
	}
	if def.RetryPolicy.MaxAttempts != 1 {
		t.Fatalf("expected MaxAttempts=1, got %d", def.RetryPolicy.MaxAttempts)
	}
	if def.TimeoutPolicy.DefaultTimeout != 5*60*1000000000 {
		t.Fatalf("expected DefaultTimeout=5min, got %v", def.TimeoutPolicy.DefaultTimeout)
	}
	if def.TimeoutPolicy.MaxTimeout != 15*60*1000000000 {
		t.Fatalf("expected MaxTimeout=15min, got %v", def.TimeoutPolicy.MaxTimeout)
	}
	if def.ResultPolicy != "auto" {
		t.Fatalf("expected ResultPolicy=auto, got %q", def.ResultPolicy)
	}
	if def.ResourceLimits.MaxMemoryMB != 256 {
		t.Fatalf("expected MaxMemoryMB=256, got %d", def.ResourceLimits.MaxMemoryMB)
	}
	if def.ResourceLimits.MaxConcurrentTasks != 2 {
		t.Fatalf("expected MaxConcurrentTasks=2, got %d", def.ResourceLimits.MaxConcurrentTasks)
	}
}

func TestBuildTaskDefinition_CustomEntry(t *testing.T) {
	def := BuildTaskDefinition("custom/entry.js")
	if def.Entry != "custom/entry.js" {
		t.Fatalf("expected custom entry, got %q", def.Entry)
	}
}

func TestBuildTaskDefinition_InputSchema(t *testing.T) {
	def := BuildTaskDefinition("")
	if len(def.InputSchema) == 0 {
		t.Fatal("expected non-empty InputSchema")
	}
	if !strings.Contains(string(def.InputSchema), "query") {
		t.Fatal("expected InputSchema to contain 'query' property")
	}
}

func TestDefinitionHash_Deterministic(t *testing.T) {
	def := BuildTaskDefinition("tasks/deep-search/index.js")
	h1 := DefinitionHash(def)
	h2 := DefinitionHash(def)
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %q vs %q", h1, h2)
	}
	if !strings.HasPrefix(h1, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %q", h1)
	}
}

func TestDefinitionHash_DifferentEntries(t *testing.T) {
	def1 := BuildTaskDefinition("entry1.js")
	def2 := BuildTaskDefinition("entry2.js")
	h1 := DefinitionHash(def1)
	h2 := DefinitionHash(def2)
	if h1 == h2 {
		t.Fatal("different entries should produce different hashes")
	}
}

func TestConstants(t *testing.T) {
	if DeepSearchTaskID != "system.search.deep" {
		t.Fatalf("unexpected TaskID: %q", DeepSearchTaskID)
	}
	if DeepSearchExtensionID != "amitia.system.search" {
		t.Fatalf("unexpected ExtensionID: %q", DeepSearchExtensionID)
	}
	if DeepSearchModuleID != "deep_search" {
		t.Fatalf("unexpected ModuleID: %q", DeepSearchModuleID)
	}
	if DeepSearchEntry != "tasks/deep-search/index.js" {
		t.Fatalf("unexpected Entry: %q", DeepSearchEntry)
	}
}
