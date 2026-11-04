package knowledge

import (
	"testing"

	"github.com/u-ai/backend/config"
)

func TestResolveChatRouteUsesActivePolicy(t *testing.T) {
	features := config.DefaultKnowledgeRuntimeConfig().Features
	active := ResolveChatRoute("核对 PDF 第37页原文", features)
	if active.EffectiveLevel != RetrievalLevelAgentic || active.PolicyMode != PolicyModeActive || active.Degraded {
		t.Fatalf("unexpected active decision: %#v", active)
	}
}

func TestFeatureGatesReturnEffectiveLevelAndReason(t *testing.T) {
	features := config.DefaultKnowledgeRuntimeConfig().Features
	features.AgenticInvestigationEnabled = false
	decision := ResolveDiagnosticRoute(RetrievalLevelAgentic, RetrievalLevelAgentic, features)
	if decision.EffectiveLevel != RetrievalLevelDeep || !decision.Degraded || decision.RouteDegradationReason != "agentic_investigation_disabled" {
		t.Fatalf("unexpected agentic gate: %#v", decision)
	}

}

func TestChatRouteSkipsNonDocumentConversation(t *testing.T) {
	decision := ResolveChatRoute("今天陪我聊聊", config.DefaultKnowledgeRuntimeConfig().Features)
	if decision.EffectiveLevel != RetrievalLevelNone || decision.RouteReason != RouteReasonChatNoIntent {
		t.Fatalf("unexpected social decision: %#v", decision)
	}
}
