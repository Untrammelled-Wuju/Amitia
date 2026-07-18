package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func emoteIntent(t *testing.T, animated bool) DeliveryIntent {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{"messageId": "m1", "originalPath": "/emote-assets/e/original.gif", "fallbackPath": "/emote-assets/e/fallback.png", "isAnimated": animated, "altText": "[表情：开心]"})
	if err != nil {
		t.Fatal(err)
	}
	return DeliveryIntent{Channel: "web", PeerID: "peer", ContentType: "emote", Payload: payload}
}

func TestWebEmoteAdapterAndFailures(t *testing.T) {
	adapter := NewWebChannelAdapter()
	if err := adapter.Deliver(emoteIntent(t, true)); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Deliver(DeliveryIntent{ContentType: "emote", Payload: []byte(`{"altText":"missing"}`)}); err == nil {
		t.Fatal("缺少 messageId 时不应假成功")
	}
	if err := adapter.Deliver(DeliveryIntent{ContentType: "video", Payload: []byte(`{"messageId":"m1"}`)}); err == nil {
		t.Fatal("不支持的 Web 类型应失败")
	}
}

func TestQQAndWechatEmoteAssetSelection(t *testing.T) {
	requests := make(chan map[string]interface{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/send-image" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	intent := emoteIntent(t, true)
	if err := NewQQChannelAdapter(server.URL).Deliver(intent); err != nil {
		t.Fatal(err)
	}
	if err := NewWechatChannelAdapter(server.URL).Deliver(intent); err != nil {
		t.Fatal(err)
	}
	qq := <-requests
	wechat := <-requests
	if qq["assetUrl"] != "/emote-assets/e/original.gif" {
		t.Fatalf("QQ 应优先发送原动图: %#v", qq)
	}
	if wechat["assetUrl"] != "/emote-assets/e/fallback.png" {
		t.Fatalf("微信应使用降级图: %#v", wechat)
	}
}

func TestQQAndWechatPropagateSendFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	intent := emoteIntent(t, false)
	if err := NewQQChannelAdapter(server.URL).Deliver(intent); err == nil {
		t.Fatal("QQ 发送失败必须返回错误")
	}
	if err := NewWechatChannelAdapter(server.URL).Deliver(intent); err == nil {
		t.Fatal("微信发送失败必须返回错误")
	}
}
