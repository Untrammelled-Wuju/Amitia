package chat

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func ownedVisionMessages(count int) []map[string]interface{} {
	parts := []map[string]any{{"type": "text", "text": "请看画面"}}
	for range count {
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64,cGljdHVyZQ=="}})
	}
	return []map[string]interface{}{{"role": "user", "content": parts}}
}

func TestOwnedIndependentVisionUsesCoreResultsAndStopsOnCancellation(t *testing.T) {
	messages := ownedVisionMessages(2)
	calls := 0
	generate := func(ctx context.Context, images []string, prompt string) (string, error) {
		calls++
		if len(images) != 2 || !strings.Contains(prompt, "请看画面") {
			t.Fatal("vision input lost original frames or text")
		}
		return "两个可见画面", nil
	}
	if err := prepareOwnedIndependentVision(t.Context(), messages, generate); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !strings.Contains(messages[0]["content"].(string), "两个可见画面") {
		t.Fatal("main text model did not receive Core vision output")
	}
	if err := prepareOwnedIndependentVision(t.Context(), ownedVisionMessages(33), generate); err == nil || calls != 1 {
		t.Fatal("unbounded visual inputs reached provider")
	}
	ctx, cancel := context.WithCancel(t.Context())
	unchanged := ownedVisionMessages(1)
	err := prepareOwnedIndependentVision(ctx, unchanged, func(context.Context, []string, string) (string, error) { cancel(); return "旧Core迟到结果", nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled Core result remained usable")
	}
	if _, ok := unchanged[0]["content"].(string); ok {
		t.Fatal("late Core vision output changed the prompt")
	}
}
