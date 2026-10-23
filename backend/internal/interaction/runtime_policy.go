package interaction

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/safety"
)

func attachmentCount(req *ProcessRequest) int {
	count := 0
	if strings.TrimSpace(req.ImageUrl) != "" || strings.TrimSpace(req.ImageContext) != "" {
		count++
	}
	if strings.TrimSpace(req.AudioUrl) != "" {
		count++
	}
	if strings.TrimSpace(req.VideoUrl) != "" {
		count++
	}
	return count
}

func isEmotionalMessage(message string) bool {
	message = strings.ToLower(message)
	for _, marker := range []string{"难过", "生气", "焦虑", "害怕", "开心", "喜欢", "讨厌", "sad", "angry", "anxious", "happy", "love", "hate"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func runtimeRequestID(ctx context.Context, scope InteractionScope, req *ProcessRequest) string {
	if execution, ok := coordination.FromContext(ctx); ok {
		if requestID := strings.TrimSpace(execution.RequestID); requestID != "" {
			return requestID
		}
	}
	if req != nil {
		if requestID := strings.TrimSpace(req.RequestID); requestID != "" {
			return requestID
		}
	}
	return strings.TrimSpace(scope.RequestID)
}

func (p *RuntimePipeline) buildSafetyDecision(ctx context.Context, snapshot ContextSnapshot, scope InteractionScope, req *ProcessRequest) (RuntimeSafetyDecision, safety.RuntimePolicy, *safety.Governor) {
	policy := safety.DefaultPolicy()
	governor := p.safetyGovernor
	requestID := runtimeRequestID(ctx, scope, req)
	if p.safetyPolicyReader != nil {
		ownerID := scope.SpaceID
		if execution, ok := coordination.FromContext(ctx); ok && execution.ResourceOwnerID != "" {
			ownerID = execution.ResourceOwnerID
		}
		loaded, err := p.safetyPolicyReader.RuntimePolicy(ctx, scope.SpaceID, ownerID)
		if err != nil {
			return RuntimeSafetyDecision{Level: "blocked", Blocked: true, Reasons: []string{"policy_unavailable"}}, policy, nil
		}
		policy = loaded
		governor = safety.NewPolicyGovernor(loaded)
	}
	if governor == nil {
		decision := runtimeSafety(snapshot)
		decision.PolicyRevision = policy.Revision
		return decision, policy, nil
	}
	result := governor.CheckPreGen(safety.PreGenInput{CharacterID: scope.CharacterID, SpaceID: scope.SpaceID, Scope: scope.Channel, Text: req.Message, Context: map[string]string{"request_id": requestID}})
	level := "normal"
	if !result.Allowed {
		level = "blocked"
		if p.safetyRecorder != nil {
			ownerID := scope.SpaceID
			coreID := scope.SpaceID
			if execution, ok := coordination.FromContext(ctx); ok {
				ownerID, coreID = execution.ResourceOwnerID, execution.CoreID
			}
			details, _ := json.Marshal(map[string]any{"reasons": result.Reasons, "action": policy.Config.ViolationAction})
			if err := p.safetyRecorder.RecordIncident(ctx, safety.Incident{SpaceID: scope.SpaceID, OwnerID: ownerID, CoreID: coreID, RoleID: scope.CharacterID, ConversationID: scope.ConversationID, RequestID: requestID, EventType: "policy.violation", Severity: "high", Description: strings.Join(result.Reasons, ","), Status: "open", PolicyRevision: policy.Revision, Revision: 1, DetailsJSON: string(details)}); err != nil {
				level = "blocked"
				result.Reasons = append(result.Reasons, "safety_incident_store_unavailable")
			}
		}
	}
	return RuntimeSafetyDecision{Level: level, Blocked: !result.Allowed, Reasons: result.Reasons, PolicyRevision: policy.Revision, Action: policy.Config.ViolationAction}, policy, governor
}

func findTransactionDefinition(name TransactionBoundary) TransactionDefinition {
	for _, definition := range DefaultTransactionBoundaries {
		if definition.Name == name {
			return definition
		}
	}
	return TransactionDefinition{Name: TransactionBoundaryNone}
}

func runtimeSafety(snapshot ContextSnapshot) RuntimeSafetyDecision {
	decision := RuntimeSafetyDecision{Level: "normal"}
	if snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Tension > 0.75 {
		decision.Level = "conservative"
		decision.Reasons = append(decision.Reasons, "relationship_tension")
	}
	if snapshot.Psyche.Status == LoadStatusReady && snapshot.Psyche.Value.Stress > 0.85 {
		decision.Level = "conservative"
		decision.Reasons = append(decision.Reasons, "high_stress")
	}
	if snapshot.Beliefs.Status == LoadStatusReady && snapshot.Beliefs.Value.Conflict != nil && snapshot.Beliefs.Value.Conflict.RiskLevel == "blocked" {
		decision.Level = "blocked"
		decision.Blocked = true
		decision.Reasons = append(decision.Reasons, "belief_conflict")
	}
	if snapshot.UnresolvedThreads.Status == LoadStatusReady && snapshot.UnresolvedThreads.Value.Count > 0 && decision.Level == "normal" {
		decision.Level = "conservative"
		decision.Reasons = append(decision.Reasons, "unresolved_threads")
	}
	return decision
}

func runtimeDelivery(scope InteractionScope, request *ProcessRequest) RuntimeDeliveryIntent {
	intent := RuntimeDeliveryIntent{Channel: scope.Channel, PeerID: scope.PeerID, RequiresText: true, RequiresVoice: request.VoiceMessage}
	if strings.TrimSpace(request.ImageUrl) != "" {
		intent.Media = append(intent.Media, "image")
	}
	if strings.TrimSpace(request.AudioUrl) != "" {
		intent.Media = append(intent.Media, "audio")
	}
	if strings.TrimSpace(request.VideoUrl) != "" {
		intent.Media = append(intent.Media, "video")
	}
	return intent
}

func runtimeBudgetModules(snapshot ContextSnapshot, path PathType) []TokenBudgetModule {
	modules := []TokenBudgetModule{{Name: "safety", Tokens: 180, Priority: BudgetPrioritySafety}, {Name: "current_intent", Tokens: 320, Priority: BudgetPriorityCurrentIntent}}
	if snapshot.RuntimeProfile.Status == LoadStatusReady {
		modules = append(modules, TokenBudgetModule{Name: "runtime_profile", Tokens: 260, Priority: BudgetPriorityHighAuthority})
	}
	if snapshot.Psyche.Status == LoadStatusReady {
		modules = append(modules, TokenBudgetModule{Name: "psyche", Tokens: 220, Priority: BudgetPriorityHighAuthority})
	}
	if snapshot.Relationship.Status == LoadStatusReady {
		modules = append(modules, TokenBudgetModule{Name: "relationship", Tokens: 220, Priority: BudgetPriorityHighAuthority})
	}
	if snapshot.Beliefs.Status == LoadStatusReady {
		modules = append(modules, TokenBudgetModule{Name: "beliefs", Tokens: 280, Priority: BudgetPriorityHighAuthority})
	}
	if snapshot.Memories.Status == LoadStatusReady {
		modules = append(modules, TokenBudgetModule{Name: "memories", Tokens: 420, Priority: BudgetPriorityLowAuthority})
	}
	if snapshot.Life.Status == LoadStatusReady {
		modules = append(modules, TokenBudgetModule{Name: "life", Tokens: 180, Priority: BudgetPriorityHighAuthority})
	}
	if snapshot.Needs.Status == LoadStatusReady {
		modules = append(modules, TokenBudgetModule{Name: "needs", Tokens: 180, Priority: BudgetPriorityHighAuthority})
	}
	if snapshot.UnresolvedThreads.Status == LoadStatusReady && snapshot.UnresolvedThreads.Value.Count > 0 {
		modules = append(modules, TokenBudgetModule{Name: "unresolved_threads", Tokens: 220, Priority: BudgetPriorityHighAuthority})
	}
	if path == PathTypeDeep {
		modules = append(modules, TokenBudgetModule{Name: "deep_reasoning", Tokens: 360, Priority: BudgetPriorityCurrentIntent})
	}
	return modules
}
