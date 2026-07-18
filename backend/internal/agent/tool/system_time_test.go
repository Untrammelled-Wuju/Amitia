package tool

import (
	"strings"
	"testing"
)

func TestCurrentTimeUsesSystemFallbackWithoutTemporalService(t *testing.T) {
	SetTemporalService(nil)
	result, found := ExecuteWithContext(ToolExecutionContext{User: "user-1", CharacterID: "character-1", Channel: "web"}, "get_current_time", `{}`)
	if !found {
		t.Fatal("get_current_time not found")
	}
	if result.Status != ToolStatusSuccess || !strings.Contains(result.Content, "系统参考时间") || !strings.Contains(result.Content, "用户时区未确认") {
		t.Fatalf("unexpected fallback %#v", result)
	}
}
