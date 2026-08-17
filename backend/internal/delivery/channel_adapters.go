package delivery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	applog "github.com/u-ai/backend/log"
)

const (
	ProviderInstanceIDWebChannel    = "builtin.channel.web"
	ProviderInstanceIDQQChannel     = "builtin.channel.qq"
	ProviderInstanceIDWechatChannel = "builtin.channel.wechat"
)

type QQChannelAdapter struct {
	sidecarURL string
}

func NewQQChannelAdapter(sidecarURL string) *QQChannelAdapter {
	return &QQChannelAdapter{sidecarURL: sidecarURL}
}

func (a *QQChannelAdapter) Name() string {
	return "qq"
}

func (a *QQChannelAdapter) ProviderInstanceID() string {
	return ProviderInstanceIDQQChannel
}

func (a *QQChannelAdapter) Deliver(intent DeliveryIntent) error {
	if intent.ContentType == "emote" {
		return deliverEmoteHTTP(a.sidecarURL+"/api/send-image", intent, false)
	}
	content := extractContentFromPayload(intent.Payload)
	body, _ := json.Marshal(map[string]string{
		"toUserId": intent.PeerID,
		"text":     content,
	})
	req, _ := http.NewRequest("POST", a.sidecarURL+"/api/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		applog.Error("QQ delivery failed", "peerId", intent.PeerID, "error", err)
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		applog.Error("QQ delivery failed", "peerId", intent.PeerID, "status", resp.StatusCode)
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	applog.Info("QQ delivered", "peerId", intent.PeerID)
	return nil
}

type WechatChannelAdapter struct {
	sidecarURL string
}

func NewWechatChannelAdapter(sidecarURL string) *WechatChannelAdapter {
	return &WechatChannelAdapter{sidecarURL: sidecarURL}
}

func (a *WechatChannelAdapter) Name() string {
	return "wechat"
}

func (a *WechatChannelAdapter) ProviderInstanceID() string {
	return ProviderInstanceIDWechatChannel
}

func (a *WechatChannelAdapter) Deliver(intent DeliveryIntent) error {
	if intent.ContentType == "emote" {
		return deliverEmoteHTTP(a.sidecarURL+"/api/send-image", intent, true)
	}
	content := extractContentFromPayload(intent.Payload)
	body, _ := json.Marshal(map[string]string{
		"toUserId":    intent.PeerID,
		"text":        content,
		"deliveryKey": intent.ID,
	})
	req, _ := http.NewRequest("POST", a.sidecarURL+"/api/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", intent.ID)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		applog.Error("Wechat delivery failed", "deliveryId", intent.ID, "error", err)
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		applog.Error("Wechat delivery failed", "deliveryId", intent.ID, "status", resp.StatusCode)
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	applog.Info("Wechat delivered", "deliveryId", intent.ID)
	return nil
}

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
	if intent.ContentType == "emote" {
		if _, hasAsset := payload["originalPath"]; !hasAsset {
			if _, hasFallback := payload["fallbackPath"]; !hasFallback {
				return fmt.Errorf("web delivery: emote missing asset")
			}
		}
	}
	return nil
}

func deliverEmoteHTTP(url string, intent DeliveryIntent, wechat bool) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(intent.Payload, &payload); err != nil {
		return err
	}
	original, _ := payload["originalPath"].(string)
	fallback, _ := payload["fallbackPath"].(string)
	asset := original
	if wechat || asset == "" {
		asset = fallback
	}
	if asset == "" {
		return fmt.Errorf("emote asset missing")
	}
	bodyMap := map[string]interface{}{"toUserId": intent.PeerID, "assetUrl": asset, "fallbackUrl": fallback, "animated": payload["isAnimated"], "altText": payload["altText"]}
	if wechat {
		bodyMap["deliveryKey"] = intent.ID
	}
	body, _ := json.Marshal(bodyMap)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if wechat {
		req.Header.Set("Idempotency-Key", intent.ID)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
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
