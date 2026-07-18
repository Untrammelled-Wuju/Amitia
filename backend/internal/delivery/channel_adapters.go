package delivery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	applog "github.com/u-ai/backend/log"
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

func (a *WechatChannelAdapter) Deliver(intent DeliveryIntent) error {
	if intent.ContentType == "emote" {
		return deliverEmoteHTTP(a.sidecarURL+"/api/send-image", intent, true)
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
		applog.Error("Wechat delivery failed", "peerId", intent.PeerID, "error", err)
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		applog.Error("Wechat delivery failed", "peerId", intent.PeerID, "status", resp.StatusCode)
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	applog.Info("Wechat delivered", "peerId", intent.PeerID)
	return nil
}

type WebChannelAdapter struct{}

func NewWebChannelAdapter() *WebChannelAdapter { return &WebChannelAdapter{} }

func (a *WebChannelAdapter) Name() string { return "web" }

func (a *WebChannelAdapter) Deliver(intent DeliveryIntent) error {
	if intent.ContentType != "text" && intent.ContentType != "emote" && intent.ContentType != "audio" {
		return fmt.Errorf("unsupported web content type %s", intent.ContentType)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(intent.Payload, &payload); err != nil {
		return err
	}
	if _, ok := payload["messageId"].(string); !ok {
		return fmt.Errorf("web delivery missing messageId")
	}
	return nil
}

func deliverEmoteHTTP(url string, intent DeliveryIntent, forceFallback bool) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(intent.Payload, &payload); err != nil {
		return err
	}
	original, _ := payload["originalPath"].(string)
	fallback, _ := payload["fallbackPath"].(string)
	asset := original
	if forceFallback || asset == "" {
		asset = fallback
	}
	if asset == "" {
		return fmt.Errorf("emote asset missing")
	}
	body, _ := json.Marshal(map[string]interface{}{"toUserId": intent.PeerID, "assetUrl": asset, "fallbackUrl": fallback, "animated": payload["isAnimated"], "altText": payload["altText"]})
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
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
