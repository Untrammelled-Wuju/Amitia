package system

import "testing"

func TestClassifyWebChatRuntimeError(t *testing.T) {
	errorCode, userMessage, errorType := classifyWebChatRuntimeError(
		"generation_failed",
		"Agent 执行失败",
		"database is locked (5) (SQLITE_BUSY)",
	)
	if errorCode != "storage_error" {
		t.Fatalf("errorCode=%q, want storage_error", errorCode)
	}
	if userMessage != "本地数据库繁忙，请重试" {
		t.Fatalf("userMessage=%q", userMessage)
	}
	if errorType != "storage" {
		t.Fatalf("errorType=%q, want storage", errorType)
	}
}

func TestClassifyWebChatRuntimeErrorKeepsProviderFailure(t *testing.T) {
	errorCode, userMessage, errorType := classifyWebChatRuntimeError(
		"generation_failed",
		"Agent 执行失败",
		"provider returned 503",
	)
	if errorCode != "generation_failed" {
		t.Fatalf("errorCode=%q, want generation_failed", errorCode)
	}
	if userMessage != "Agent 执行失败" {
		t.Fatalf("userMessage=%q", userMessage)
	}
	if errorType != "runtime" {
		t.Fatalf("errorType=%q, want runtime", errorType)
	}
}
