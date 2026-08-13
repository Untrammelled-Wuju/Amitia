package javascript_main

import (
	"context"
	"testing"
)

func TestDefaultCapabilities(t *testing.T) {
	caps := DefaultCapabilities()
	if caps.Backend != "node-process" {
		t.Fatalf("expected backend node-process, got %s", caps.Backend)
	}
	if len(caps.SupportedFormats) == 0 {
		t.Fatal("expected supported formats")
	}
	foundESM := false
	foundCJS := false
	for _, f := range caps.SupportedFormats {
		if f == ".mjs" {
			foundESM = true
		}
		if f == ".cjs" {
			foundCJS = true
		}
	}
	if !foundESM || !foundCJS {
		t.Fatalf("expected .mjs and .cjs in supported formats, got %v", caps.SupportedFormats)
	}
	if !caps.NetworkDisabled {
		t.Fatal("expected network disabled by default")
	}
	if caps.Platform == "" {
		t.Fatal("expected platform to be set")
	}
	if caps.Architecture == "" {
		t.Fatal("expected architecture to be set")
	}
	if caps.MaxMemoryMB != 512 {
		t.Fatalf("expected max memory 512, got %d", caps.MaxMemoryMB)
	}
	if caps.MaxConcurrent != 4 {
		t.Fatalf("expected max concurrent 4, got %d", caps.MaxConcurrent)
	}
}

func TestNodeProcessBackendCapabilities(t *testing.T) {
	factory := NewRuntimeFactory()
	backend := NewNodeProcessBackend(factory, nil, nil)
	caps := backend.Capabilities()
	if caps.Backend != "node-process" {
		t.Fatalf("expected node-process, got %s", caps.Backend)
	}
}

func TestNodeProcessBackendStartValidation(t *testing.T) {
	factory := NewRuntimeFactory()
	backend := NewNodeProcessBackend(factory, nil, nil)
	_, err := backend.Start(context.Background(), JavaScriptRuntimeSpec{})
	if err == nil {
		t.Fatal("expected error for empty extension id")
	}
}
