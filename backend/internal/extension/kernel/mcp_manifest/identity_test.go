package mcp_manifest

import (
	"strings"
	"testing"
)

func TestBuildServerID_Deterministic(t *testing.T) {
	id1 := BuildServerID("com.example/tools", "mcp", "filesystem")
	id2 := BuildServerID("com.example/tools", "mcp", "filesystem")
	if id1 != id2 {
		t.Errorf("ServerID not deterministic: %s != %s", id1, id2)
	}
}

func TestBuildServerID_DifferentContribution(t *testing.T) {
	id1 := BuildServerID("com.example/tools", "mcp", "filesystem")
	id2 := BuildServerID("com.example/tools", "mcp", "browser")
	if id1 == id2 {
		t.Error("expected different IDs for different contributions")
	}
}

func TestBuildServerID_DifferentExtension(t *testing.T) {
	id1 := BuildServerID("com.example/tools", "mcp", "filesystem")
	id2 := BuildServerID("com.other/tools", "mcp", "filesystem")
	if id1 == id2 {
		t.Error("expected different IDs for different extensions")
	}
}

func TestBuildServerID_DifferentModule(t *testing.T) {
	id1 := BuildServerID("com.example/tools", "mcp", "filesystem")
	id2 := BuildServerID("com.example/tools", "mcp2", "filesystem")
	if id1 == id2 {
		t.Error("expected different IDs for different modules")
	}
}

func TestBuildServerID_PathSafe(t *testing.T) {
	id := BuildServerID("com.example/tools", "mcp", "filesystem")
	if !IsServerIDPathSafe(id) {
		t.Errorf("ServerID not path-safe: %s", id)
	}
}

func TestBuildServerID_NoReservedChars(t *testing.T) {
	id := BuildServerID("com.example/tools", "mcp", "filesystem")
	reserved := []string{"/", "?", "#", "%"}
	for _, ch := range reserved {
		if strings.Contains(id, ch) {
			t.Errorf("ServerID contains reserved char %q: %s", ch, id)
		}
	}
}

func TestBuildServerID_BoundedLength(t *testing.T) {
	id := BuildServerID("com.example/tools", "mcp", "filesystem")
	if len(id) > MaxServerIDLength {
		t.Errorf("ServerID too long: %d > %d", len(id), MaxServerIDLength)
	}
}

func TestBuildServerID_StartsWithMCP(t *testing.T) {
	id := BuildServerID("com.example/tools", "mcp", "filesystem")
	if !strings.HasPrefix(id, "mcp_") {
		t.Errorf("ServerID should start with mcp_, got: %s", id)
	}
}

func TestIsServerIDPathSafe_Empty(t *testing.T) {
	if IsServerIDPathSafe("") {
		t.Error("empty ID should not be path-safe")
	}
}

func TestIsServerIDPathSafe_Slash(t *testing.T) {
	if IsServerIDPathSafe("mcp/test/name") {
		t.Error("ID with / should not be path-safe")
	}
}

func TestIsServerIDPathSafe_Question(t *testing.T) {
	if IsServerIDPathSafe("mcp?name") {
		t.Error("ID with ? should not be path-safe")
	}
}

func TestIsServerIDPathSafe_Hash(t *testing.T) {
	if IsServerIDPathSafe("mcp#name") {
		t.Error("ID with # should not be path-safe")
	}
}

func TestIsServerIDPathSafe_Control(t *testing.T) {
	if IsServerIDPathSafe("mcp\x01name") {
		t.Error("ID with control char should not be path-safe")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"com.example", "com_example"},
		{"my-server", "my-server"},
		{"UPPER", "upper"},
		{"a/b/c", "a_b_c"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.expected {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
