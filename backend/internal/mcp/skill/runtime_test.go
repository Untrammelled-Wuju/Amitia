package skill

import (
	"strings"
	"testing"
)

func TestNormalizeHandlesAlphanumericAndSeparators(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello_world"},
		{"MCP-Server_123", "mcp_server_123"},
		{"  spaces  ", "spaces"},
		{"!!!special@@@", "special"},
		{"", ""},
	}
	for _, c := range cases {
		result := normalize(c.input)
		if result != c.expected {
			t.Fatalf("normalize(%q) = %q, want %q", c.input, result, c.expected)
		}
	}
}

func TestSkillSegmentReplacesUnderscoresWithHyphens(t *testing.T) {
	result := skillSegment("My_MCP_Server")
	if result != "my-mcp-server" {
		t.Fatalf("skillSegment = %q, want %q", result, "my-mcp-server")
	}
}

func TestSkillSegmentHandlesEmptyAndSpecialChars(t *testing.T) {
	result := skillSegment("!!!")
	if result != "" {
		t.Fatalf("skillSegment with special chars only = %q, want empty", result)
	}
}

func TestNormalizeDoesNotProduceLeadingTrailingUnderscores(t *testing.T) {
	result := normalize("---test---")
	if strings.HasPrefix(result, "_") || strings.HasSuffix(result, "_") {
		t.Fatalf("normalize should not produce leading/trailing underscores, got %q", result)
	}
}
