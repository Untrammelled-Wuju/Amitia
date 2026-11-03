package business

import (
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/conversationstream"
)

func TestAppendRuntimeError(t *testing.T) {
	runtime := &conversationstream.ActiveTurnState{TurnID: "turn-1"}
	appendRuntimeError(runtime, errors.New("模型请求超时"), "failed")
	appendRuntimeError(runtime, errors.New("重复错误"), "failed")
	if len(runtime.Blocks) != 1 || runtime.Blocks[0].Type != "error" || runtime.Blocks[0].Content != "重复错误" {
		t.Fatalf("unexpected runtime error blocks: %+v", runtime.Blocks)
	}
	result, _ := runtime.Blocks[0].Payload["result"].(string)
	if !strings.Contains(result, "重复错误") || !strings.Contains(result, `"rawError":"重复错误"`) {
		t.Fatalf("error payload not preserved: %+v", runtime.Blocks[0].Payload)
	}
}
