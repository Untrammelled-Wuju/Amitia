package localmodel

import (
	"strings"
	"testing"
)

func TestRuntimeProcessIDNormalizesModelAndProvider(t *testing.T) {
	id := RuntimeProcessID("llama.cpp", "Story15M Q4_0", "ABC123")
	if id != "llama.cpp-abc123-story15m-q4_0" {
		t.Fatalf("RuntimeProcessID()=%q", id)
	}
}

func TestRuntimeProcessIDBoundsLength(t *testing.T) {
	id := RuntimeProcessID("provider", strings.Repeat("Model Name ", 40), strings.Repeat("f", 40))
	if len(id) > 128 {
		t.Fatalf("RuntimeProcessID() length=%d", len(id))
	}
	if strings.ContainsAny(id, " ") || strings.ToLower(id) != id {
		t.Fatalf("RuntimeProcessID() is not canonical: %q", id)
	}
}
