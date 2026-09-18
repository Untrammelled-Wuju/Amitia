package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/internal/temporal"
)

func TestWebhookRejectsNilContextBeforeUnifiedEntry(t *testing.T) {
	processor := &agentTestProcessor{}
	orch := interaction.NewOrchestratorWithStores(interaction.DefaultOrchestratorConfig(), processor, interaction.NewInMemoryTracker(), nil)
	orch.SetReady(true)
	svc := &service{unifiedEntry: interaction.NewUnifiedEntry(orch, interaction.NewScopeResolver(nil), temporal.SystemClock{})}

	_, err := svc.Webhook(nil, WebhookRequest{
		Channel:        "web",
		ConversationID: "conv-1",
		SenderID:       "peer-1",
		SpaceID:        "user-1",
		Source:         "web",
		Text:           "hello",
		SkipTiming:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "请求上下文不能为空") {
		t.Fatalf("expected nil context error, got %v", err)
	}
	if processor.called {
		t.Fatal("unified entry processor should not be called")
	}
}

type agentTestProcessor struct {
	called bool
}

func (p *agentTestProcessor) ProcessMessageCtx(ctx context.Context, req *interaction.ProcessRequest) (*interaction.ProcessResponse, error) {
	p.called = true
	return nil, errors.New("unexpected call")
}

func TestStableWebhookSpaceIDUsesAuthenticatedScope(t *testing.T) {
	canonical := requestidentity.CanonicalSpaceID()
	if got := stableWebhookSpaceID(WebhookRequest{}); got != canonical {
		t.Fatalf("expected %q, got %q", canonical, got)
	}
	if got := stableWebhookSpaceID(WebhookRequest{SpaceID: "web-user"}); got != "web-user" {
		t.Fatalf("expected %q, got %q", "web-user", got)
	}
	if got := stableWebhookSpaceID(WebhookRequest{SenderID: "wechat-openid"}); got != canonical {
		t.Fatalf("expected %q, got %q", canonical, got)
	}
	if got := stableWebhookSpaceID(WebhookRequest{AccountID: "qq-account"}); got != canonical {
		t.Fatalf("expected %q, got %q", canonical, got)
	}
}
