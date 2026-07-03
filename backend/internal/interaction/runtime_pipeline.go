package interaction

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/belief"
	"github.com/u-ai/backend/internal/personality"
	"github.com/u-ai/backend/internal/safety"

	"github.com/u-ai/backend/internal/psyche/appraisal"
	"github.com/u-ai/backend/internal/psyche/budget"
)

type RuntimeSafetyDecision struct {
	Level   string   `json:"level"`
	Blocked bool     `json:"blocked"`
	Reasons []string `json:"reasons,omitempty"`
}

type RuntimeDeliveryIntent struct {
	Channel       string   `json:"channel"`
	PeerID        string   `json:"peerId,omitempty"`
	RequiresText  bool     `json:"requiresText"`
	RequiresVoice bool     `json:"requiresVoice"`
	Media         []string `json:"media,omitempty"`
}

type RuntimeAssembly struct {
	Version     string                           `json:"version"`
	ExecutorID  string                           `json:"executorId,omitempty"`
	Context     ContextSnapshot                  `json:"context"`
	Path        PathType                         `json:"path"`
	Budget      []TokenBudgetPlan                `json:"budget"`
	Safety      RuntimeSafetyDecision            `json:"safety"`
	Delivery    RuntimeDeliveryIntent            `json:"delivery"`
	Transaction TransactionDefinition            `json:"transaction"`
	AssembledAt time.Time                        `json:"assembledAt"`
	Personality *personality.CompiledPersonality `json:"personality,omitempty"`
	Appraisal   *AppraisalResult                 `json:"appraisal,omitempty"`
}

type RuntimePipeline struct {
	registry            *ContextLoaderRegistry
	classifier          *PathClassifier
	budgetManager       *TokenBudgetManager
	transaction         TransactionDefinition
	snapshotVersion     string
	personalityCompiler *personality.Compiler
	safetyGovernor      *safety.Governor
	beliefResolver      BeliefResolverFunc
	appraisalEngine     *appraisal.Engine
	budgetController    *budget.BudgetController
}

func NewRuntimePipeline(registry *ContextLoaderRegistry, classifier *PathClassifier, budgetManager *TokenBudgetManager) *RuntimePipeline {
	if registry == nil {
		registry = NewContextLoaderRegistry()
	}
	if classifier == nil {
		classifier = NewPathClassifier()
	}
	if budgetManager == nil {
		budgetManager = NewTokenBudgetManager(2400)
	}
	return &RuntimePipeline{
		registry:        registry,
		classifier:      classifier,
		budgetManager:   budgetManager,
		transaction:     findTransactionDefinition(TransactionBoundaryAll),
		snapshotVersion: "context-snapshot-v1",
	}
}

func (p *RuntimePipeline) SetPersonalityCompiler(compiler *personality.Compiler) {
	p.personalityCompiler = compiler
}

func (p *RuntimePipeline) SetSafetyGovernor(governor *safety.Governor) {
	p.safetyGovernor = governor
}

type BeliefResolverFunc func(input belief.ResolveInput) belief.ResolveResult

func (p *RuntimePipeline) SetBeliefResolver(resolver BeliefResolverFunc) {
	p.beliefResolver = resolver
}

func (p *RuntimePipeline) SetAppraisalEngine(engine *appraisal.Engine) {
	p.appraisalEngine = engine
}

func (p *RuntimePipeline) SetBudgetController(controller *budget.BudgetController) {
	p.budgetController = controller
}

func (p *RuntimePipeline) Assemble(ctx context.Context, scope InteractionScope, req *ProcessRequest) RuntimeAssembly {
	snapshot := p.registry.LoadAll(ctx, scope, p.snapshotVersion)
	path := p.classifier.Classify(PathClassifyInput{
		MessageContent: req.Message,
		RoleState:      snapshot.Psyche.Value,
		Attachments:    attachmentCount(req),
		IsEmotional:    isEmotionalMessage(req.Message),
		HasCommands:    strings.Contains(req.Message, "/"),
		MessageLength:  len([]rune(req.Message)),
	})

	appraisalResult := p.runAppraisal(snapshot, scope, req, path)
	if p.beliefResolver != nil && snapshot.Beliefs.Status == LoadStatusReady && len(snapshot.Beliefs.Value.Beliefs) > 0 {
		resolved := make([]ResolvedBelief, 0, len(snapshot.Beliefs.Value.Beliefs))
		for _, b := range snapshot.Beliefs.Value.Beliefs {
			result := p.beliefResolver(belief.ResolveInput{
				Key:        b.Key,
				Candidates: []belief.Candidate{{Key: b.Key, Value: b.Value, Confidence: b.Confidence}},
				Policy:     belief.ResolverPolicy{MinimumConfidence: 0.3, ConflictGap: 0.15, MaxCandidates: 5},
				Now:        time.Now(),
			})
			resolved = append(resolved, ResolvedBelief{
				Key:        result.Belief.Key,
				Value:      result.Belief.Value,
				Confidence: result.Belief.Confidence,
			})
		}
		snapshot.Beliefs.Value.Beliefs = resolved
	}

	var compiledPersonality *personality.CompiledPersonality
	if p.personalityCompiler != nil && snapshot.RuntimeProfile.Status == LoadStatusReady {
		ifaceConfig := make(map[string]interface{})
		if snapshot.RuntimeProfile.Value.PersonalityConfig != nil {
			for k, v := range snapshot.RuntimeProfile.Value.PersonalityConfig {
				ifaceConfig[k] = v
			}
		}
		if snapshot.RuntimeProfile.Value.Identity != "" {
			ifaceConfig["identity"] = snapshot.RuntimeProfile.Value.Identity
		}
		if snapshot.RuntimeProfile.Value.Personality != "" {
			ifaceConfig["personality"] = snapshot.RuntimeProfile.Value.Personality
		}
		if snapshot.RuntimeProfile.Value.BoundaryRules != "" {
			ifaceConfig["coreBoundary"] = snapshot.RuntimeProfile.Value.BoundaryRules
		}
		cp := p.personalityCompiler.Compile(scope.CharacterID, ifaceConfig)
		compiledPersonality = &cp
	}

	safetyDecision := p.buildSafetyDecision(snapshot, scope)

	return RuntimeAssembly{
		Version:     "orchestrator-runtime-v1",
		Context:     snapshot,
		Path:        path,
		Budget:      p.budgetManager.Allocate(runtimeBudgetModules(snapshot, path)),
		Safety:      safetyDecision,
		Delivery:    runtimeDelivery(scope, req),
		Transaction: p.transaction,
		AssembledAt: time.Now(),
		Personality: compiledPersonality,
		Appraisal:   appraisalResult,
	}
}

func (p *RuntimePipeline) buildSafetyDecision(snapshot ContextSnapshot, scope InteractionScope) RuntimeSafetyDecision {
	if p.safetyGovernor != nil {
		preGenInput := safety.PreGenInput{
			CharacterID: scope.CharacterID,
			UserID:      scope.UserID,
			Scope:       scope.Channel,
			Context: map[string]string{
				"request_id": scope.RequestID,
			},
		}
		result := p.safetyGovernor.CheckPreGen(preGenInput)
		level := "normal"
		if !result.Allowed {
			level = "blocked"
		}
		return RuntimeSafetyDecision{
			Level:   level,
			Blocked: !result.Allowed,
			Reasons: result.Reasons,
		}
	}
	return runtimeSafety(snapshot)
}

func findTransactionDefinition(name TransactionBoundary) TransactionDefinition {
	for _, def := range DefaultTransactionBoundaries {
		if def.Name == name {
			return def
		}
	}
	return TransactionDefinition{Name: TransactionBoundaryNone}
}

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
	text := strings.ToLower(message)
	markers := []string{"难过", "生气", "焦虑", "害怕", "开心", "喜欢", "讨厌", "sad", "angry", "anxious", "happy", "love", "hate"}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func runtimeBudgetModules(snapshot ContextSnapshot, path PathType) []TokenBudgetModule {
	modules := []TokenBudgetModule{
		{Name: "safety", Tokens: 180, Priority: BudgetPrioritySafety},
		{Name: "current_intent", Tokens: 320, Priority: BudgetPriorityCurrentIntent},
	}
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

func runtimeDelivery(scope InteractionScope, req *ProcessRequest) RuntimeDeliveryIntent {
	intent := RuntimeDeliveryIntent{
		Channel:       scope.Channel,
		PeerID:        scope.PeerID,
		RequiresText:  true,
		RequiresVoice: req.VoiceMessage,
	}
	if strings.TrimSpace(req.ImageUrl) != "" {
		intent.Media = append(intent.Media, "image")
	}
	if strings.TrimSpace(req.AudioUrl) != "" {
		intent.Media = append(intent.Media, "audio")
	}
	if strings.TrimSpace(req.VideoUrl) != "" {
		intent.Media = append(intent.Media, "video")
	}
	return intent
}

type AppraisalResult struct {
	PsycheDelta       float64 `json:"psycheDelta"`
	RelationshipDelta float64 `json:"relationshipDelta"`
	Severity          float64 `json:"severity"`
	EventType         string  `json:"eventType"`
	BudgetAllocated   float64 `json:"budgetAllocated"`
}

func (p *RuntimePipeline) runAppraisal(snapshot ContextSnapshot, scope InteractionScope, req *ProcessRequest, path PathType) *AppraisalResult {
	if p.appraisalEngine == nil {
		return nil
	}
	if req.IsInternal {
		return nil
	}
	a := p.appraisalEngine.Evaluate(appraisal.AppraisalInput{
		EventType:         appraisalEventType(req.Message, path),
		RelatesToGoal:     strings.Contains(req.Message, "目标") || strings.Contains(req.Message, "goal"),
		GoalCongruent:     !strings.Contains(req.Message, "失败") && !strings.Contains(req.Message, "放弃"),
		IsExpected:        1.0,
		Controllable:      true,
		Responsibility:    0.5,
		Uncertainty:       0.3,
		InvolvesRelation:  snapshot.Relationship.Status == LoadStatusReady,
		NormViolated:      isEmotionalMessage(req.Message),
		BoundaryViolated:  false,
		SimilarPastEvents: 0,
	})

	severity := budget.ComputeEventSeverity(a.OverallSeverity, a.GoalRelevance, a.NormViolation, a.BoundaryViolation)
	result := &AppraisalResult{
		PsycheDelta:       a.GoalCongruence - 0.5,
		RelationshipDelta: a.RelationshipRelevance - 0.5,
		Severity:          severity,
		EventType:         string(a.EventType),
	}

	if p.budgetController != nil {
		candidates := []budget.CandidateDelta{
			{Module: "psyche", Delta: result.PsycheDelta, Priority: 1, Reason: "interaction_appraisal"},
			{Module: "relationship", Delta: result.RelationshipDelta, Priority: 2, Reason: "interaction_appraisal"},
		}
		budgetResult := p.budgetController.Allocate(severity, candidates)
		result.BudgetAllocated = budgetResult.TotalAllocated
		for _, final := range budgetResult.FinalDeltas {
			if final.Module == "psyche" {
				result.PsycheDelta = final.Delta
			} else if final.Module == "relationship" {
				result.RelationshipDelta = final.Delta
			}
		}
		for _, rejected := range budgetResult.Rejected {
			if rejected.Module == "psyche" {
				result.PsycheDelta = 0
			} else if rejected.Module == "relationship" {
				result.RelationshipDelta = 0
			}
		}
	}

	return result
}

func appraisalEventType(message string, path PathType) string {
	if path == PathTypeDeep {
		return "deep_interaction"
	}
	if isEmotionalMessage(message) {
		return "emotional"
	}
	return "chat"
}
