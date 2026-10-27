package business

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/conversationstream"
)

func TestAppendRuntimeError(t *testing.T) {
	runtime := &conversationstream.ActiveTurnState{TurnID: "turn-1"}
	appendRuntimeError(runtime, errors.New("模型请求超时"))
	appendRuntimeError(runtime, errors.New("重复错误"))
	if len(runtime.Blocks) != 1 || runtime.Blocks[0].Type != "error" || runtime.Blocks[0].Content != "模型请求超时" {
		t.Fatalf("unexpected runtime error blocks: %+v", runtime.Blocks)
	}
	if runtime.Blocks[0].Payload["result"] != `{"userMessage":"模型请求超时"}` {
		t.Fatalf("error payload not preserved: %+v", runtime.Blocks[0].Payload)
	}
}
