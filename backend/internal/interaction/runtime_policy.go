package interaction

import "strings"

import "github.com/u-ai/backend/internal/safety"

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

func (p *RuntimePipeline) buildSafetyDecision(snapshot ContextSnapshot, scope InteractionScope) RuntimeSafetyDecision {
	if p.safetyGovernor == nil {
		return runtimeSafety(snapshot)
	}
	result := p.safetyGovernor.CheckPreGen(safety.PreGenInput{CharacterID: scope.CharacterID, UserID: scope.UserID, Scope: scope.Channel, Context: map[string]string{"request_id": scope.RequestID}})
	level := "normal"
	if !result.Allowed {
		level = "blocked"
	}
	return RuntimeSafetyDecision{Level: level, Blocked: !result.Allowed, Reasons: result.Reasons}
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
