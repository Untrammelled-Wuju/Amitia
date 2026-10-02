package chat

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/vision"
)

func TestMainVisionCapabilityMerge(t *testing.T) {
	json := mergeReasoningCapabilities(map[string]interface{}{"supportsVision": true}, `{"supportsReasoning":true,"defaultReasoningEffort":"high","supportsToolUse":true}`)
	cfg := &ModelConfig{CapabilitiesJSON: json, APIKey: "test-key"}
	redactModelConfigForResponse(cfg)
	if !cfg.SupportsVision || !cfg.SupportsReasoning || cfg.DefaultReasoningEffort != "high" || cfg.APIKey != "" {
		t.Fatalf("capabilities lost: %v", cfg)
	}
	cfg.CapabilitiesJSON = mergeReasoningCapabilities(map[string]interface{}{"supportsVision": false}, json)
	redactModelConfigForResponse(cfg)
	if cfg.SupportsVision || !cfg.SupportsReasoning {
		t.Fatalf("switch off lost reasoning: %v", cfg)
	}
}

func TestMainVisionNativeHistoryAndLocalParts(t *testing.T) {
	visionModelConfigProviderMu.RLock()
	original := visionModelConfigProvider
	visionModelConfigProviderMu.RUnlock()
	t.Cleanup(func() { SetVisionModelConfigProvider(original) })
	SetVisionModelConfigProvider(func() (*vision.VisionConfig, error) {
		return &vision.VisionConfig{ID: 7, ApiKey: "test-key", BaseUrl: "http://test", ModelName: "main", FromMainModel: true}, nil
	})
	if !UseNativeMainVision(0) || !UseNativeMainVision(7) || UseNativeMainVision(8) {
		t.Fatal("wrong selected model routing")
	}
	messages := []map[string]interface{}{{"role": "user", "content": "compare these pictures"}}
	history := []map[string]string{{"role": "user", "content": "previous", "imageUrl": "data:image/png;base64,aGlzdG9yeQ=="}}
	if err := attachMainVisionImages(context.Background(), messages, history, "space", "data:image/png;base64,Y3VycmVudA==", 7); err != nil {
		t.Fatal(err)
	}
	req := messagesToModelRequest(&ModelConfig{ModelName: "main"}, messages, nil, false)
	count := 0
	for _, part := range req.Messages[0].Parts {
		if part.Type == ContentTypeImage {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("history/current image count=%d", count)
	}
	local := toLocalModelRequest(req, messages)
	if len(local.Messages[0].Parts) != len(req.Messages[0].Parts) {
		t.Fatal("local multimodal parts lost")
	}
	otherMessages := []map[string]interface{}{{"role": "user", "content": "secondary"}}
	if err := attachMainVisionImages(context.Background(), otherMessages, history, "space", "data:image/png;base64,Y3VycmVudA==", 8); err != nil {
		t.Fatal(err)
	}
	if otherMessages[0]["parts"] != nil {
		t.Fatal("secondary model must not receive main model images")
	}
}
