package delivery

import (
	"bytes"
	"fmt"
	"encoding/json"
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
