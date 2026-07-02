package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeWebhookService struct {
	ctx context.Context
	req WebhookRequest
}

func (f *fakeWebhookService) Test(characterID, message string) (map[string]interface{}, error) {
	return nil, nil
}

func (f *fakeWebhookService) ContextPreview(convID string) (map[string]interface{}, error) {
	return nil, nil
}

func (f *fakeWebhookService) Webhook(ctx context.Context, req WebhookRequest) (map[string]interface{}, error) {
	f.ctx = ctx
	f.req = req
	return map[string]interface{}{
		"outgoingMessage": map[string]interface{}{"text": "ok"},
		"requestId":       req.RequestID,
		"sessionId":       req.SessionID,
	}, nil
}

func TestWebhookHandlerPassesEnvelopeAndContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeWebhookService{}
	h := NewHandler(svc)
	body := map[string]interface{}{
		"channel":        "qq",
		"accountId":      "account-1",
		"conversationId": "conv-1",
		"senderId":       "peer-1",
		"userId":         "user-1",
		"messageId":      "message-1",
		"requestId":      "request-1",
		"sessionId":      "session-1",
		"text":           "hello",
		"voiceMessage":   true,
		"imageUrl":       "image.png",
		"videoUrl":       "video.mp4",
		"audioBase64":    "YQ==",
		"skipTiming":     true,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/agent/webhook", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	h.Webhook(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.ctx != req.Context() {
		t.Fatal("request context was not passed to service")
	}
	if svc.req.Channel != "qq" || svc.req.AccountID != "account-1" || svc.req.ConversationID != "conv-1" {
		t.Fatalf("unexpected channel envelope: %#v", svc.req)
	}
	if svc.req.SenderID != "peer-1" || svc.req.UserID != "user-1" {
		t.Fatalf("unexpected user envelope: %#v", svc.req)
	}
	if svc.req.MessageID != "message-1" || svc.req.RequestID != "request-1" || svc.req.SessionID != "session-1" {
		t.Fatalf("unexpected id envelope: %#v", svc.req)
	}
	if svc.req.Text != "hello" || !svc.req.VoiceMessage || svc.req.ImageUrl != "image.png" || svc.req.VideoUrl != "video.mp4" || svc.req.AudioBase64 != "YQ==" || !svc.req.SkipTiming {
		t.Fatalf("unexpected payload envelope: %#v", svc.req)
	}
}

func TestStableWebhookSourcePrefersRequestSource(t *testing.T) {
	req := WebhookRequest{Channel: "wechat", Source: "qq"}
	if got := stableWebhookSource(req); got != "qq" {
		t.Fatalf("unexpected source: %s", got)
	}

	req = WebhookRequest{Channel: "wechat"}
	if got := stableWebhookSource(req); got != "wechat" {
		t.Fatalf("unexpected channel fallback source: %s", got)
	}

	req = WebhookRequest{}
	if got := stableWebhookSource(req); got != "webhook" {
		t.Fatalf("unexpected default source: %s", got)
	}
}
