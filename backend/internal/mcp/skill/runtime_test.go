package skill

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/mcp"
)

func TestNormalizeResultSupportsStructuredMediaAndRemoteError(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"text","text":"hello"},{"type":"image","data":"aGVsbG8=","mimeType":"image/png"},{"type":"audio","data":"aGVsbG8=","mimeType":"audio/wav"}],"structuredContent":{"value":1}}`)
	result, remoteError, err := normalizeResult(raw)
	if err != nil || remoteError || result.VisibleText != "hello" || string(result.Output) != `{"value":1}` {
		t.Fatalf("unexpected result=%#v remote=%v err=%v", result, remoteError, err)
	}
	errorRaw := json.RawMessage(`{"content":[{"type":"text","text":"failed"}],"isError":true}`)
	_, remoteError, err = normalizeResult(errorRaw)
	if err != nil || !remoteError {
		t.Fatalf("expected remote error, got %v %v", remoteError, err)
	}
}

func TestNormalizeResultRejectsInvalidAndOversizedContent(t *testing.T) {
	if _, _, err := normalizeResult(json.RawMessage(`{"content":[{"type":"video","data":"x"}]}`)); err == nil {
		t.Fatal("expected invalid type error")
	}
	raw, _ := json.Marshal(map[string]any{"content": []any{map[string]any{"type": "text", "text": strings.Repeat("x", (256<<10)+1)}}})
	if _, _, err := normalizeResult(raw); err == nil {
		t.Fatal("expected oversized content error")
	}
}

func TestNormalizeResultRedactsSecrets(t *testing.T) {
	result, _, err := normalizeResult(json.RawMessage(`{"content":[{"type":"text","text":"password=secretvalue"}],"structuredContent":{"api_key":"abcdefghijk"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.VisibleText, "secretvalue") || string(result.Output) != `{"redacted":true}` {
		t.Fatalf("secret not redacted: %#v", result)
	}
}

func TestModelNameLongValuesKeepStableUniqueHash(t *testing.T) {
	prefix := strings.Repeat("long-tool-name-", 10)
	left := modelName("server", prefix+"left")
	right := modelName("server", prefix+"right")
	if left == right || len(left) > 64 || len(right) > 64 || left != modelName("server", prefix+"left") {
		t.Fatalf("unexpected model names %q %q", left, right)
	}
}

func TestCapabilitiesClassifyRiskAndTransport(t *testing.T) {
	values, sideEffects, idempotent := capabilities(mcp.Server{ID: "remote", Transport: "streamable_http"}, mcp.ToolDefinition{RemoteName: "delete_message", Description: "delete", RiskLevel: "high", CapabilityHintsJSON: `["idempotent"]`})
	joined := strings.Join(values, ",")
	if !sideEffects || !idempotent || !strings.Contains(joined, "network.remote") || !strings.Contains(joined, "data.delete") {
		t.Fatalf("unexpected capabilities=%v sideEffects=%v idempotent=%v", values, sideEffects, idempotent)
	}
}
