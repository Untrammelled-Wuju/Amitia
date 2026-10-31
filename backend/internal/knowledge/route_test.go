package knowledge

import (
	"testing"

	"github.com/u-ai/backend/config"
)

func TestResolveChatRouteMatchesActiveAndLegacyPolicies(t *testing.T) {
	features := config.DefaultKnowledgeRuntimeConfig().Features
	active := ResolveChatRoute("核对 PDF 第37页原文", features)
	if active.EffectiveLevel != RetrievalLevelAgentic || active.PolicyMode != PolicyModeActive || active.Degraded {
		t.Fatalf("unexpected active decision: %#v", active)
	}

	features.RetrievalPolicyEnabled = false
	features.RetrievalPolicyShadow = true
	legacy := ResolveChatRoute("核对 PDF 第37页原文", features)
	if legacy.EffectiveLevel != RetrievalLevelFast || legacy.PolicyMode != PolicyModeLegacyShadow || legacy.ShadowLevel != RetrievalLevelAgentic {
		t.Fatalf("unexpected legacy shadow decision: %#v", legacy)
	}
}

func TestFeatureGatesReturnEffectiveLevelAndReason(t *testing.T) {
	features := config.DefaultKnowledgeRuntimeConfig().Features
	features.AgenticInvestigationEnabled = false
	decision := ResolveDiagnosticRoute(RetrievalLevelAgentic, RetrievalLevelAgentic, features)
	if decision.EffectiveLevel != RetrievalLevelDeep || !decision.Degraded || decision.RouteDegradationReason != "agentic_investigation_disabled" {
		t.Fatalf("unexpected agentic gate: %#v", decision)
	}

	features = config.DefaultKnowledgeRuntimeConfig().Features
	features.RetrievalPolicyEnabled = false
	decision = ResolveDiagnosticRoute(RetrievalLevelDeep, RetrievalLevelDeep, features)
	if decision.EffectiveLevel != RetrievalLevelFast || !decision.Degraded || decision.RouteDegradationReason != "retrieval_policy_disabled" {
		t.Fatalf("unexpected policy gate: %#v", decision)
	}
}

func TestChatRouteSkipsNonDocumentConversation(t *testing.T) {
	decision := ResolveChatRoute("今天陪我聊聊", config.DefaultKnowledgeRuntimeConfig().Features)
	if decision.EffectiveLevel != RetrievalLevelNone || decision.RouteReason != RouteReasonChatNoIntent {
		t.Fatalf("unexpected social decision: %#v", decision)
	}
}
