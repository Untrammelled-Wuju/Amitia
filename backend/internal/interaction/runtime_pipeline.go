package interaction

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/belief"
	"github.com/u-ai/backend/internal/personality"
	"github.com/u-ai/backend/internal/safety"

	"github.com/u-ai/backend/internal/decision"

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
	BehaviorPlan   *decision.BehaviorPlan    `json:"behaviorPlan,omitempty"`
	ExpressionPlan *decision.ExpressionPlan  `json:"expressionPlan,omitempty"`
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
	candidateRegistry   *decision.CandidateRegistry
	arbitrationLayer    decision.ArbitrationLayer
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

func (p *RuntimePipeline) SetDecisionLayer(registry *decision.CandidateRegistry, layer decision.ArbitrationLayer) {
	if registry == nil {
		registry = decision.DefaultCandidateRegistry()
	}
	p.candidateRegistry = registry
	p.arbitrationLayer = layer
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

	var behaviorPlan *decision.BehaviorPlan
	var expressionPlan *decision.ExpressionPlan
	if p.candidateRegistry != nil && !req.IsInternal {
		bp, ep := p.runDecision(ctx, scope, snapshot, appraisalResult, compiledPersonality)
		behaviorPlan = bp
		expressionPlan = ep
	}

	return RuntimeAssembly{
		Version:        "orchestrator-runtime-v1",
		Context:        snapshot,
		Path:           path,
		Budget:         p.budgetManager.Allocate(runtimeBudgetModules(snapshot, path)),
		Safety:         safetyDecision,
		Delivery:       runtimeDelivery(scope, req),
		Transaction:    p.transaction,
		AssembledAt:    time.Now(),
		Personality:    compiledPersonality,
		Appraisal:      appraisalResult,
		BehaviorPlan:   behaviorPlan,
		ExpressionPlan: expressionPlan,
	}
}

func (p *RuntimePipeline) runDecision(ctx context.Context, scope InteractionScope, snapshot ContextSnapshot, appraisal *AppraisalResult, compiledPersonality *personality.CompiledPersonality) (*decision.BehaviorPlan, *decision.ExpressionPlan) {
	if p.candidateRegistry == nil {
		return nil, nil
	}

	now := time.Now()

	var psycheSignals decision.PsycheSignalSet
	if snapshot.Psyche.Status == LoadStatusReady {
		psycheSignals = decision.PsycheSignalSet{
			Mood:          decision.ScalarSignal{Value: snapshot.Psyche.Value.MoodPressure},
			Stress:        decision.ScalarSignal{Value: snapshot.Psyche.Value.Stress},
			CognitiveLoad: decision.ScalarSignal{Value: snapshot.Psyche.Value.Fatigue},
		}
	} else {
		psycheSignals = decision.PsycheSignalSet{
			Mood:   decision.ScalarSignal{Value: 0.5},
			Stress: decision.ScalarSignal{Value: 0.0},
		}
	}

	personalityWeights := derivePersonalityWeights(compiledPersonality)
	personalityStyle := derivePersonalityStyle(compiledPersonality)

	var relSnapshot decision.RelationshipSnapshot
	if snapshot.Relationship.Status == LoadStatusReady {
		relSnapshot = decision.RelationshipSnapshot{
			UserID:      scope.UserID,
			CharacterID: scope.CharacterID,
			Dimensions: map[decision.RelationshipDimension]decision.RelationshipDimensionValue{
				decision.RelationshipTrust:       {Value: snapshot.Relationship.Value.Trust},
				decision.RelationshipFamiliarity: {Value: snapshot.Relationship.Value.Familiarity},
				decision.RelationshipSafety:      {Value: snapshot.Relationship.Value.Security},
			},
		}
	}

	var lifeSnapshot decision.LifeSnapshot
	if snapshot.Life.Status == LoadStatusReady {
		lifeSnapshot = decision.LifeSnapshot{
			Energy: snapshot.Life.Value.Energy,
			Busy:   0,
		}
		if snapshot.Life.Value.Busy {
			lifeSnapshot.Busy = 0.8
		}
	} else {
		lifeSnapshot = decision.LifeSnapshot{Energy: 0.7}
	}

	ctx_ := decision.CandidateGenerationContext{
		UserID:       scope.UserID,
		CharacterID:  scope.CharacterID,
		Psyche:       psycheSignals,
		Relationship: relSnapshot,
		Life:         lifeSnapshot,
		PersonalityWeights: personalityWeights,
		Now:          now,
	}

	candidates := decision.GenerateCandidates(ctx_, p.candidateRegistry)

	arbitrationInput := decision.ArbitrationInput{
		Candidates:   candidates,
		Relationship: relSnapshot,
		Psyche:       psycheSignals,
		Life:         lifeSnapshot,
		Filter:       decision.DefaultHardConstraintFilter(),
		Now:          now,
	}
	arbitrationResult := p.arbitrationLayer.Arbitrate(arbitrationInput)

	builder := decision.NewBehaviorPlanBuilder(now)
	plan := builder.Build(arbitrationResult.Selected, arbitrationInput)
	plan.CharacterID = scope.CharacterID
	plan.UserID = scope.UserID
	if compiledPersonality != nil {
		plan.Personality = decision.CompiledPersonalityRef{
			Version:           compiledPersonality.Version,
			SourceCharacterID: compiledPersonality.CharacterID,
			BehaviorWeights:   personalityWeights,
		}
	}

	emotionIntensity := 0.5
	if compiledPersonality != nil {
		if v, ok := compiledPersonality.ExpressionStyle["emotionalExpression"]; ok {
			emotionIntensity = v
		}
	}

	exprCtrl := decision.ExpressionControlInput{
		EmotionIntensity:   emotionIntensity,
		StressLevel:        psycheSignals.Stress.Value,
		RelationshipSafety: 0.5,
	}
	if relSnapshot.Dimensions != nil {
		if v, ok := relSnapshot.Dimensions[decision.RelationshipSafety]; ok {
			exprCtrl.RelationshipSafety = v.Value
		}
	}
	exprInput := decision.ExpressionPlanInput{
		BehaviorPlan:   plan,
		Psyche:         psycheSignals,
		ExpressionCtrl: exprCtrl,
		PersonalityExpressionStyle: personalityStyle,
		Now:            now,
	}
	exprPlan := decision.GenerateExpressionPlan(exprInput)

	return &plan, &exprPlan
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
	NeedDeltas        map[string]float64 `json:"needDeltas,omitempty"`
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

	relatesToGoal := semanticRelatesToGoal(req.Message)
	goalCongruent := semanticGoalCongruent(req.Message)
	isExpected := semanticIsExpected(req.Message, snapshot)
	controllable := semanticControllable(req.Message)
	responsibility := semanticResponsibility(req.Message)
	uncertainty := semanticUncertainty(req.Message)
	involvesRelation := snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Familiarity > 0.1
	normViolated := semanticNormViolated(req.Message)
	boundaryViolated := semanticBoundaryViolated(req.Message, snapshot)
	similarPastCount := semanticSimilarPastCount(req.Message, snapshot)

	a := p.appraisalEngine.Evaluate(appraisal.AppraisalInput{
		EventType:         appraisalEventType(req.Message, path),
		RelatesToGoal:     relatesToGoal,
		GoalCongruent:     goalCongruent,
		IsExpected:        isExpected,
		Controllable:      controllable,
		Responsibility:    responsibility,
		Uncertainty:       uncertainty,
		InvolvesRelation:  involvesRelation,
		NormViolated:      normViolated,
		BoundaryViolated:  boundaryViolated,
		SimilarPastEvents: similarPastCount,
	})

	severity := budget.ComputeEventSeverity(a.OverallSeverity, a.GoalRelevance, a.NormViolation, a.BoundaryViolation)
	result := &AppraisalResult{
		PsycheDelta:       a.GoalCongruence - 0.5,
		RelationshipDelta: a.RelationshipRelevance - 0.5,
		Severity:          severity,
		EventType:         string(a.EventType),
	}

	result.NeedDeltas = map[string]float64{
		"reassurance": (a.GoalCongruence - 0.5) * 0.1,
		"connection":  (a.RelationshipRelevance - 0.5) * 0.1,
		"autonomy":    (a.Controllability - 0.5) * 0.1,
		"clarity":     ((1.0 - a.CausalUncertainty) - 0.5) * 0.1,
		"novelty":     (a.Novelty - 0.5) * 0.1,
		"expression":  (a.Responsibility - 0.5) * 0.1,
		"rest":        0,
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


func semanticRelatesToGoal(message string) bool {
	msg := strings.ToLower(message)
	goalMarkers := []string{"目标", "goal", "计划", "plan", "想要", "want", "需要", "need", "希望", "hope", "决定", "decide"}
	for _, m := range goalMarkers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

func semanticGoalCongruent(message string) bool {
	msg := strings.ToLower(message)
	negativeMarkers := []string{"失败", "fail", "放弃", "give up", "做不到", "can't", "不行", "impossible", "绝望", "hopeless"}
	for _, m := range negativeMarkers {
		if strings.Contains(msg, m) {
			return false
		}
	}
	positiveMarkers := []string{"成功", "success", "做到了", "did it", "开心", "happy", "感谢", "thanks", "很棒", "great"}
	for _, m := range positiveMarkers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return true
}

func semanticIsExpected(message string, snapshot ContextSnapshot) float64 {
	if snapshot.Relationship.Status != LoadStatusReady {
		return 0.5
	}
	familiarity := snapshot.Relationship.Value.Familiarity
	if familiarity > 0.6 {
		return 0.75
	}
	if familiarity > 0.3 {
		return 0.6
	}
	return 0.4
}

func semanticControllable(message string) bool {
	msg := strings.ToLower(message)
	uncontrollableMarkers := []string{"地震", "earthquake", "意外", "accident", "突然", "sudden", "死亡", "death", "灾难", "disaster"}
	for _, m := range uncontrollableMarkers {
		if strings.Contains(msg, m) {
			return false
		}
	}
	return true
}

func semanticResponsibility(message string) float64 {
	msg := strings.ToLower(message)
	selfBlame := []string{"我错了", "my fault", "怪我", "blame me", "对不起", "sorry", "是我的错", "I was wrong"}
	for _, m := range selfBlame {
		if strings.Contains(msg, m) {
			return 0.85
		}
	}
	otherBlame := []string{"你错了", "your fault", "怪你", "blame you", "是你的问题", "your problem"}
	for _, m := range otherBlame {
		if strings.Contains(msg, m) {
			return 0.15
		}
	}
	return 0.5
}

func semanticUncertainty(message string) float64 {
	msg := strings.ToLower(message)
	uncertainMarkers := []string{"可能", "maybe", "也许", "perhaps", "不知道", "don't know", "不确定", "unsure", "好像", "似乎", "大概"}
	uncertainCount := 0
	for _, m := range uncertainMarkers {
		if strings.Contains(msg, m) {
			uncertainCount++
		}
	}
	if uncertainCount >= 3 {
		return 0.75
	}
	if uncertainCount >= 1 {
		return 0.5
	}
	return 0.2
}

func semanticNormViolated(message string) bool {
	msg := strings.ToLower(message)
	violationMarkers := []string{"骂", "侮辱", "insult", "人身攻击", "personal attack", "威胁", "threat", "骚扰", "harass"}
	for _, m := range violationMarkers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return isEmotionalMessage(message)
}

func semanticBoundaryViolated(message string, snapshot ContextSnapshot) bool {
	msg := strings.ToLower(message)
	boundaryMarkers := []string{"爱", "love", "喜欢", "like", "想见", "want to meet", "私聊", "private", "加好友", "add friend"}
	for _, m := range boundaryMarkers {
		if strings.Contains(msg, m) {
			if snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Familiarity < 0.3 {
				return true
			}
		}
	}
	return false
}

func semanticSimilarPastCount(message string, snapshot ContextSnapshot) int {
	if snapshot.Memories.Status != LoadStatusReady {
		return 0
	}
	count := 0
	for _, mem := range snapshot.Memories.Value.Memories {
		if strings.Contains(strings.ToLower(mem.Value), strings.ToLower(message[:minInt(len(message), 20)])) {
			count++
		}
	}
	return count
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func derivePersonalityWeights(cp *personality.CompiledPersonality) map[decision.BehaviorTag]float64 {
	if cp == nil {
		return nil
	}
	b := cp.BehaviorBias
	weights := make(map[decision.BehaviorTag]float64)

	mapBiasToTag := func(tag decision.BehaviorTag, key string, factor float64) {
		if v, ok := b[key]; ok {
			weights[tag] = (v - 0.5) * factor
		}
	}

	mapBiasToTag(decision.BehaviorTagReply, "warmth", 0.3)
	mapBiasToTag(decision.BehaviorTagReply, "initiative", 0.2)
	mapBiasToTag(decision.BehaviorTagOfferSupport, "warmth", 0.4)
	mapBiasToTag(decision.BehaviorTagOfferSupport, "companionship", 0.3)
	mapBiasToTag(decision.BehaviorTagOfferSupport, "affection", 0.2)
	mapBiasToTag(decision.BehaviorTagAskClarify, "clarification", 0.6)
	mapBiasToTag(decision.BehaviorTagAskClarify, "directness", 0.2)
	mapBiasToTag(decision.BehaviorTagSetBoundary, "boundary", 0.5)
	mapBiasToTag(decision.BehaviorTagSetBoundary, "conflictAvoidance", -0.3)
	mapBiasToTag(decision.BehaviorTagSetBoundary, "directness", 0.3)
	mapBiasToTag(decision.BehaviorTagProactiveCheck, "initiative", 0.5)
	mapBiasToTag(decision.BehaviorTagProactiveCheck, "warmth", 0.3)
	mapBiasToTag(decision.BehaviorTagProactiveCheck, "companionship", 0.2)
	mapBiasToTag(decision.BehaviorTagDelay, "initiative", -0.4)
	mapBiasToTag(decision.BehaviorTagDelay, "patience", 0.3)
	mapBiasToTag(decision.BehaviorTagRepair, "affection", 0.4)
	mapBiasToTag(decision.BehaviorTagRepair, "warmth", 0.3)

	return weights
}

func derivePersonalityStyle(cp *personality.CompiledPersonality) map[string]float64 {
	if cp == nil {
		return nil
	}
	s := cp.ExpressionStyle
	if s == nil {
		return nil
	}
	result := make(map[string]float64, len(s))
	for k, v := range s {
		result[k] = v
	}
	return result
}
