package delivery

import (
	"encoding/json"
	"fmt"
)

const (
	ProviderInstanceIDWebChannel = "builtin.channel.web"
)

type WebChannelAdapter struct{}

func NewWebChannelAdapter() *WebChannelAdapter { return &WebChannelAdapter{} }

func (a *WebChannelAdapter) Name() string { return "web" }

func (a *WebChannelAdapter) ProviderInstanceID() string {
	return ProviderInstanceIDWebChannel
}

func (a *WebChannelAdapter) Deliver(intent DeliveryIntent) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(intent.Payload, &payload); err != nil {
		return fmt.Errorf("web delivery: invalid payload: %w", err)
	}
	messageID, ok := payload["messageId"].(string)
	if !ok || messageID == "" {
		return fmt.Errorf("web delivery: missing messageId")
	}
	switch intent.ContentType {
	case "text", "image":
	default:
		return fmt.Errorf("web delivery: unsupported content type %s", intent.ContentType)
	}
	if intent.ContentType == "image" {
		if _, hasAsset := payload["originalPath"]; !hasAsset {
			if _, hasFallback := payload["fallbackPath"]; !hasFallback {
				return fmt.Errorf("web delivery: image missing asset")
			}
		}
	}
	return nil
}

func extractContentFromPayload(payload []byte) string {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return string(payload)
	}
	if content, ok := data["content"].(string); ok {
		return content
	}
	return string(payload)
}
