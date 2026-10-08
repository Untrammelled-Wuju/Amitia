package notificationruntime

import (
	"context"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/nativebridge"
)

const nativeRuntimeDeliverOperation = "notification.runtime_deliver"

type NativeBridgeProvider struct {
	bridge nativebridge.Bridge
}

func NewNativeBridgeProvider(bridge nativebridge.Bridge) *NativeBridgeProvider {
	if bridge == nil {
		return nil
	}
	return &NativeBridgeProvider{bridge: bridge}
}

func (p *NativeBridgeProvider) Name() string { return "native" }

func (p *NativeBridgeProvider) Available(ctx context.Context) bool {
	if p == nil || p.bridge == nil {
		return false
	}
	health := p.bridge.Health(ctx)
	return health == nativebridge.HealthReady || health == nativebridge.HealthUnknown
}

func (p *NativeBridgeProvider) Send(
	ctx context.Context,
	endpoint DeviceEndpoint,
	envelope PushEnvelope,
) ProviderResult {
	result := ProviderResult{Provider: "native"}
	if p == nil || p.bridge == nil {
		result.ErrorCode = "native_bridge_unavailable"
		result.ErrorMessage = "native bridge is unavailable"
		return result
	}
	payload := map[string]any{
		"notificationId": envelope.NotificationID,
		"type":           envelope.Type,
		"spaceId":        envelope.SpaceID,
		"conversationId": envelope.ConversationID,
		"characterId":    envelope.CharacterID,
		"messageId":      envelope.MessageID,
		"runId":          envelope.RunID,
		"revision":       envelope.Revision,
		"title":          envelope.Title,
		"body":           envelope.Body,
		"deepLink":       envelope.DeepLink,
		"priority":       envelope.Priority,
		"sound":          envelope.Sound,
	}
	for key, value := range envelope.Data {
		payload[key] = value
	}
	response, err := p.bridge.Execute(ctx, nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       envelope.NotificationID,
		Platform:        strings.ToLower(strings.TrimSpace(endpoint.Platform)),
		Operation:       nativeRuntimeDeliverOperation,
		Payload:         payload,
	})
	if err != nil {
		result.ErrorCode = "native_bridge_error"
		result.ErrorMessage = err.Error()
		return result
	}
	if response.Status != "success" && response.Status != "ok" {
		result.ErrorCode = "native_delivery_failed"
		if response.Error != nil {
			result.ErrorCode = response.Error.Code
			result.ErrorMessage = response.Error.Message
		}
		return result
	}
	result.Accepted = true
	result.ProviderMessageID = envelope.NotificationID
	if response.Result != nil {
		if id, ok := response.Result["notificationRef"].(string); ok && strings.TrimSpace(id) != "" {
			result.ProviderMessageID = strings.TrimSpace(id)
		} else if posted, ok := response.Result["posted"].(bool); ok && !posted {
			result.ProviderMessageID = envelope.NotificationID + ":suppressed"
		} else if revision, ok := response.Result["revision"].(float64); ok {
			result.ProviderMessageID = envelope.NotificationID + ":" + strconv.FormatInt(int64(revision), 10)
		}
	}
	return result
}
