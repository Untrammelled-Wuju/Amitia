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
	Version        string                           `json:"version"`
	ExecutorID     string                           `json:"executorId,omitempty"`
	Context        ContextSnapshot                  `json:"context"`
	Path           PathType                         `json:"path"`
	Budget         []TokenBudgetPlan                `json:"budget"`
	Safety         RuntimeSafetyDecision            `json:"safety"`
	Delivery       RuntimeDeliveryIntent            `json:"delivery"`
	Transaction    TransactionDefinition            `json:"transaction"`
	AssembledAt    time.Time                        `json:"assembledAt"`
	Personality    *personality.CompiledPersonality `json:"personality,omitempty"`
	Appraisal      *AppraisalResult                 `json:"appraisal,omitempty"`
	BehaviorPlan   *decision.BehaviorPlan           `json:"behaviorPlan,omitempty"`
	ExpressionPlan *decision.ExpressionPlan         `json:"expressionPlan,omitempty"`
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
			Valence:       decision.ScalarSignal{Value: snapshot.Psyche.Value.Valence},
			Arousal:       decision.ScalarSignal{Value: snapshot.Psyche.Value.Arousal},
			Dominance:     decision.ScalarSignal{Value: snapshot.Psyche.Value.Dominance},
			MoodValence:   decision.ScalarSignal{Value: snapshot.Psyche.Value.MoodValence},
			MoodArousal:   decision.ScalarSignal{Value: snapshot.Psyche.Value.MoodArousal},
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
		UserID:             scope.UserID,
		CharacterID:        scope.CharacterID,
		Psyche:             psycheSignals,
		Relationship:       relSnapshot,
		Life:               lifeSnapshot,
		PersonalityWeights: personalityWeights,
		Now:                now,
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
		BehaviorPlan:               plan,
		Psyche:                     psycheSignals,
		ExpressionCtrl:             exprCtrl,
		PersonalityExpressionStyle: personalityStyle,
		Now:                        now,
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
	PsycheDelta       float64            `json:"psycheDelta"`
	RelationshipDelta float64            `json:"relationshipDelta"`
	NeedDeltas        map[string]float64 `json:"needDeltas,omitempty"`
	Severity          float64            `json:"severity"`
	EventType         string             `json:"eventType"`
	BudgetAllocated   float64            `json:"budgetAllocated"`
}

type AppraisalEventCategory string

const (
	AppraisalCatPraise        AppraisalEventCategory = "praise"
	AppraisalCatCold          AppraisalEventCategory = "cold"
	AppraisalCatHelp          AppraisalEventCategory = "help"
	AppraisalCatBoundaryCross AppraisalEventCategory = "boundary_cross"
	AppraisalCatApology       AppraisalEventCategory = "apology"
	AppraisalCatComplaint     AppraisalEventCategory = "complaint"
	AppraisalCatEmotional     AppraisalEventCategory = "emotional"
	AppraisalCatChat          AppraisalEventCategory = "chat"
)

type appraisalSensitivities struct {
	boundaryStrength  float64
	warmth            float64
	rejectionSens     float64
	affection         float64
	conflictAvoidance float64
}

func extractAppraisalSensitivities(snapshot ContextSnapshot) appraisalSensitivities {
	s := appraisalSensitivities{
		boundaryStrength:  0.7,
		warmth:            0.5,
		rejectionSens:     0.5,
		affection:         0.45,
		conflictAvoidance: 0.5,
	}
	if snapshot.RuntimeProfile.Status != LoadStatusReady {
		return s
	}
	cfg := snapshot.RuntimeProfile.Value.PersonalityConfig
	if cfg == nil {
		return s
	}
	s.boundaryStrength = extractSensFloat(cfg, "boundary", s.boundaryStrength)
	s.warmth = extractSensFloat(cfg, "warmth", s.warmth)
	s.affection = extractSensFloat(cfg, "affection", s.affection)
	s.conflictAvoidance = extractSensFloat(cfg, "conflictAvoidance", s.conflictAvoidance)
	directness := extractSensFloat(cfg, "directness", 0.5)
	s.rejectionSens = (s.conflictAvoidance*0.6 + (1.0-directness)*0.4)
	return s
}

func extractSensFloat(cfg map[string]interface{}, key string, defaultVal float64) float64 {
	v, ok := cfg[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		if val > 1 {
			return clampFloat(val/100, 0, 1)
		}
		return clampFloat(val, 0, 1)
	case int:
		f := float64(val)
		if f > 1 {
			return clampFloat(f/100, 0, 1)
		}
		return clampFloat(f, 0, 1)
	case int64:
		f := float64(val)
		if f > 1 {
			return clampFloat(f/100, 0, 1)
		}
		return clampFloat(f, 0, 1)
	default:
		return defaultVal
	}
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func classifyAppraisalEvent(message string, path PathType) AppraisalEventCategory {
	if path == PathTypeDeep {
		return AppraisalCatEmotional
	}
	msg := strings.ToLower(message)
	if containsAny(msg, praiseMarkers) {
		return AppraisalCatPraise
	}
	if containsAny(msg, apologyMarkers) {
		return AppraisalCatApology
	}
	if containsAny(msg, boundaryCrossMarkers) {
		return AppraisalCatBoundaryCross
	}
	if containsAny(msg, complaintMarkers) {
		return AppraisalCatComplaint
	}
	if containsAny(msg, helpMarkers) {
		return AppraisalCatHelp
	}
	if containsAny(msg, coldMarkers) {
		return AppraisalCatCold
	}
	if isEmotionalMessage(message) {
		return AppraisalCatEmotional
	}
	return AppraisalCatChat
}

var praiseMarkers = []string{"谢谢", "感谢", "很棒", "厉害", "优秀", "佩服", "真好", "太棒", "多谢", "thank", "great", "awesome", "amazing", "wonderful", "称赞", "表扬"}
var apologyMarkers = []string{"对不起", "抱歉", "我的错", "怪我", "不好意思", "sorry", "apologize", "原谅", "forgive", "我错了", "悔", "道歉"}
var boundaryCrossMarkers = []string{"爱你", "想见你", "私聊", "加好友", "私人", "私下", "单独", "love you", "private", "personal", "电话号码", "地址"}
var complaintMarkers = []string{"不满", "失望", "讨厌", "烦", "生气", "无语", "糟糕", "差劲", "hate", "disappointed", "angry", "upset", "抱怨", "投诉"}
var helpMarkers = []string{"帮我", "求助", "怎么办", "不知道", "教", "请问", "help", "need", "需要", "帮忙", "能不能", "可以吗", "建议", "advice"}
var coldMarkers = []string{"哦", "嗯", "行吧", "随便", "无所谓", "fine", "whatever", "k", "呵呵", "好吧"}

func (p *RuntimePipeline) runAppraisal(snapshot ContextSnapshot, scope InteractionScope, req *ProcessRequest, path PathType) *AppraisalResult {
	if p.appraisalEngine == nil {
		return nil
	}
	if req.IsInternal {
		return nil
	}

	sens := extractAppraisalSensitivities(snapshot)
	eventCat := classifyAppraisalEvent(req.Message, path)

	relatesToGoal := semanticRelatesToGoal(req.Message, snapshot, eventCat)
	goalCongruent := semanticGoalCongruent(req.Message, snapshot, eventCat, sens)
	isExpected := semanticIsExpected(req.Message, snapshot, eventCat, sens)
	controllable := semanticControllable(req.Message, eventCat)
	responsibility := semanticResponsibility(req.Message, snapshot, eventCat, sens)
	uncertainty := semanticUncertainty(req.Message, snapshot, eventCat, sens)
	involvesRelation := snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Familiarity > 0.1
	normViolated := semanticNormViolated(req.Message, snapshot, eventCat, sens)
	boundaryViolated := semanticBoundaryViolated(req.Message, snapshot, eventCat, sens)
	similarPastCount := semanticSimilarPastCount(req.Message, snapshot, eventCat)
	hasAlternativeExplanation := semanticHasAlternativeExplanation(req.Message, eventCat)

	a := p.appraisalEngine.Evaluate(appraisal.AppraisalInput{
		EventType:                 string(eventCat),
		Source:                    "user_message",
		IsUserInitiated:           true,
		RelatesToGoal:             relatesToGoal,
		GoalCongruent:             goalCongruent,
		IsExpected:                isExpected,
		Controllable:              controllable,
		Responsibility:            responsibility,
		Uncertainty:               uncertainty,
		InvolvesRelation:          involvesRelation,
		NormViolated:              normViolated,
		BoundaryViolated:          boundaryViolated,
		HasAlternativeExplanation: hasAlternativeExplanation,
		SimilarPastEvents:         similarPastCount,
	})

	severity := budget.ComputeEventSeverity(a.OverallSeverity, a.GoalRelevance, a.NormViolation, a.BoundaryViolation)

	psyScale, relScale, needScales := eventCategoryScales(eventCat, sens)
	result := &AppraisalResult{
		PsycheDelta:       (a.GoalCongruence - 0.5) * psyScale,
		RelationshipDelta: (a.RelationshipRelevance - 0.5) * relScale,
		Severity:          severity,
		EventType:         string(eventCat),
	}

	result.NeedDeltas = map[string]float64{
		"reassurance": (a.GoalCongruence - 0.5) * needScales["reassurance"],
		"connection":  (a.RelationshipRelevance - 0.5) * needScales["connection"],
		"autonomy":    (a.Controllability - 0.5) * needScales["autonomy"],
		"clarity":     ((1.0 - a.CausalUncertainty) - 0.5) * needScales["clarity"],
		"novelty":     (a.Novelty - 0.5) * needScales["novelty"],
		"expression":  (a.Responsibility - 0.5) * needScales["expression"],
		"rest":        -severity * 0.05,
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

func eventCategoryScales(cat AppraisalEventCategory, sens appraisalSensitivities) (float64, float64, map[string]float64) {
	switch cat {
	case AppraisalCatPraise:
		return 0.35, 0.40, map[string]float64{
			"reassurance": 0.35, "connection": 0.40, "autonomy": 0.15,
			"clarity": 0.20, "novelty": 0.10, "expression": 0.25,
		}
	case AppraisalCatApology:
		return 0.45, 0.50, map[string]float64{
			"reassurance": 0.45, "connection": 0.50, "autonomy": 0.20,
			"clarity": 0.35, "novelty": 0.15, "expression": 0.40,
		}
	case AppraisalCatComplaint:
		return 0.35, -0.40, map[string]float64{
			"reassurance": -0.35, "connection": -0.40, "autonomy": -0.10,
			"clarity": -0.25, "novelty": -0.10, "expression": -0.30,
		}
	case AppraisalCatBoundaryCross:
		scale := 0.4 + sens.boundaryStrength*0.4
		return scale, -scale, map[string]float64{
			"reassurance": -scale, "connection": -scale, "autonomy": -scale * 0.5,
			"clarity": -scale * 0.6, "novelty": -scale * 0.4, "expression": -scale * 0.5,
		}
	case AppraisalCatCold:
		rejScale := 0.25 + sens.rejectionSens*0.5
		return rejScale, -rejScale, map[string]float64{
			"reassurance": -rejScale, "connection": -rejScale, "autonomy": -0.10,
			"clarity": -0.15, "novelty": -0.05, "expression": -rejScale * 0.6,
		}
	case AppraisalCatHelp:
		return 0.20, 0.25, map[string]float64{
			"reassurance": 0.20, "connection": 0.25, "autonomy": 0.10,
			"clarity": 0.30, "novelty": 0.15, "expression": 0.15,
		}
	case AppraisalCatEmotional:
		emoScale := 0.30 + sens.warmth*0.3
		return emoScale, emoScale * 0.8, map[string]float64{
			"reassurance": emoScale, "connection": emoScale * 0.8, "autonomy": 0.10,
			"clarity": 0.15, "novelty": emoScale * 0.5, "expression": emoScale,
		}
	default:
		return 0.10, 0.10, map[string]float64{
			"reassurance": 0.10, "connection": 0.10, "autonomy": 0.05,
			"clarity": 0.08, "novelty": 0.08, "expression": 0.10,
		}
	}
}

func containsAny(msg string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

func semanticRelatesToGoal(message string, snapshot ContextSnapshot, cat AppraisalEventCategory) bool {
	if cat == AppraisalCatHelp || cat == AppraisalCatApology || cat == AppraisalCatComplaint || cat == AppraisalCatCold || cat == AppraisalCatBoundaryCross {
		return true
	}
	msg := strings.ToLower(message)
	goalMarkers := []string{"目标", "goal", "计划", "plan", "想要", "want", "需要", "need", "希望", "hope", "决定", "decide"}
	for _, m := range goalMarkers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return cat == AppraisalCatPraise
}

func semanticGoalCongruent(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) bool {
	switch cat {
	case AppraisalCatPraise:
		return true
	case AppraisalCatComplaint, AppraisalCatBoundaryCross, AppraisalCatCold:
		return false
	case AppraisalCatApology:
		return snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Tension < 0.4
	case AppraisalCatHelp:
		return true
	}
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
	if snapshot.Memories.Status == LoadStatusReady {
		recentPositive := 0
		recentNegative := 0
		for _, mem := range snapshot.Memories.Value.Memories {
			mv := strings.ToLower(mem.Value)
			if containsAny(mv, positiveMarkers) {
				recentPositive++
			}
			if containsAny(mv, negativeMarkers) {
				recentNegative++
			}
		}
		if recentPositive > recentNegative {
			return true
		}
		if recentNegative > recentPositive {
			return false
		}
	}
	return sens.warmth > 0.3
}

func semanticIsExpected(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) float64 {
	base := 0.5
	if snapshot.Relationship.Status == LoadStatusReady {
		base = snapshot.Relationship.Value.Familiarity*0.4 + 0.3
	}

	switch cat {
	case AppraisalCatPraise, AppraisalCatChat:
		base += 0.15
	case AppraisalCatBoundaryCross:
		base -= 0.30
	case AppraisalCatApology:
		base -= 0.20
	case AppraisalCatComplaint:
		base -= 0.15
	case AppraisalCatCold:
		base += 0.10
	case AppraisalCatEmotional:
		base -= 0.05
	}

	msg := strings.ToLower(message)
	surpriseCount := 0
	surpriseMarkers := []string{"居然", "没想到", "竟然", "突然", "surprise", "unexpected", "震惊", "怎么会"}
	for _, m := range surpriseMarkers {
		if strings.Contains(msg, m) {
			surpriseCount++
		}
	}
	base -= float64(surpriseCount) * 0.10

	boundaryMod := (1.0 - sens.boundaryStrength) * 0.15
	base -= boundaryMod

	return clampFloat(base, 0.05, 0.95)
}

func semanticControllable(message string, cat AppraisalEventCategory) bool {
	switch cat {
	case AppraisalCatBoundaryCross, AppraisalCatApology:
		return false
	case AppraisalCatHelp, AppraisalCatEmotional:
		return true
	}
	msg := strings.ToLower(message)
	uncontrollableMarkers := []string{"地震", "earthquake", "意外", "accident", "突然", "sudden", "死亡", "death", "灾难", "disaster", "生病", "sick", "被迫", "forced"}
	for _, m := range uncontrollableMarkers {
		if strings.Contains(msg, m) {
			return false
		}
	}
	return true
}

func semanticResponsibility(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) float64 {
	base := 0.5
	switch cat {
	case AppraisalCatApology:
		base = 0.85
	case AppraisalCatComplaint:
		base = 0.25
	case AppraisalCatPraise:
		base = 0.60
	case AppraisalCatBoundaryCross:
		base = 0.30
	}
	msg := strings.ToLower(message)
	selfBlame := []string{"我错了", "my fault", "怪我", "blame me", "对不起", "sorry", "是我的错", "I was wrong"}
	for _, m := range selfBlame {
		if strings.Contains(msg, m) {
			base = 0.90
			break
		}
	}
	otherBlame := []string{"你错了", "your fault", "怪你", "blame you", "是你的问题", "your problem"}
	for _, m := range otherBlame {
		if strings.Contains(msg, m) {
			base = 0.10
			break
		}
	}
	if snapshot.Relationship.Status == LoadStatusReady {
		tensionMod := snapshot.Relationship.Value.Tension * 0.30
		base -= tensionMod
	}
	base += (sens.affection - 0.5) * 0.20
	return clampFloat(base, 0.05, 0.95)
}

func semanticUncertainty(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) float64 {
	base := 0.35
	switch cat {
	case AppraisalCatHelp:
		base = 0.65
	case AppraisalCatApology:
		base = 0.50
	case AppraisalCatBoundaryCross:
		base = 0.20
	case AppraisalCatComplaint:
		base = 0.30
	case AppraisalCatCold:
		base = 0.45
	}
	msg := strings.ToLower(message)
	uncertainMarkers := []string{"可能", "maybe", "也许", "perhaps", "不知道", "don't know", "不确定", "unsure", "好像", "似乎", "大概"}
	uncertainCount := 0
	for _, m := range uncertainMarkers {
		if strings.Contains(msg, m) {
			uncertainCount++
		}
	}
	base += float64(uncertainCount) * 0.12
	if snapshot.Relationship.Status == LoadStatusReady {
		securityMod := (1.0 - snapshot.Relationship.Value.Security) * 0.20
		base += securityMod
	}
	base += (sens.rejectionSens - 0.5) * 0.15
	return clampFloat(base, 0.05, 0.95)
}

func semanticNormViolated(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) bool {
	if cat == AppraisalCatComplaint {
		msg := strings.ToLower(message)
		violationMarkers := []string{"骂", "侮辱", "insult", "人身攻击", "personal attack", "威胁", "threat", "骚扰", "harass"}
		for _, m := range violationMarkers {
			if strings.Contains(msg, m) {
				return true
			}
		}
	}
	if cat == AppraisalCatBoundaryCross && sens.boundaryStrength > 0.6 {
		return true
	}
	return cat == AppraisalCatBoundaryCross || cat == AppraisalCatComplaint && isEmotionalMessage(message)
}

func semanticBoundaryViolated(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) bool {
	if cat == AppraisalCatBoundaryCross {
		return true
	}
	msg := strings.ToLower(message)
	boundaryMarkers := []string{"爱", "love", "喜欢", "like", "想见", "want to meet", "私聊", "private", "加好友", "add friend"}
	for _, m := range boundaryMarkers {
		if strings.Contains(msg, m) {
			if snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Familiarity < 0.3 {
				return true
			}
		}
	}
	if snapshot.Relationship.Status == LoadStatusReady {
		boundaryThreshold := 0.15 + sens.boundaryStrength*0.25
		if snapshot.Relationship.Value.Familiarity < boundaryThreshold && isEmotionalMessage(message) {
			return true
		}
	}
	return false
}

func semanticSimilarPastCount(message string, snapshot ContextSnapshot, cat AppraisalEventCategory) int {
	if snapshot.Memories.Status != LoadStatusReady {
		return 0
	}
	msgLower := strings.ToLower(message)
	tokens := tokenizeForMemory(msgLower)
	count := 0
	for _, mem := range snapshot.Memories.Value.Memories {
		memLower := strings.ToLower(mem.Value)
		matchCount := 0
		for _, t := range tokens {
			if len(t) < 2 {
				continue
			}
			if strings.Contains(memLower, t) {
				matchCount++
			}
		}
		matchRatio := float64(matchCount) / float64(maxInt(1, len(tokens)))
		if matchRatio > 0.3 {
			count++
		}
	}
	return count
}

func semanticHasAlternativeExplanation(message string, cat AppraisalEventCategory) bool {
	switch cat {
	case AppraisalCatApology:
		return true
	case AppraisalCatComplaint:
		return true
	case AppraisalCatCold:
		return true
	default:
		return false
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tokenizeForMemory(text string) []string {
	parts := strings.Fields(text)
	result := make([]string, 0, len(parts)*2)
	for _, p := range parts {
		runes := []rune(p)
		if len(runes) <= 1 {
			continue
		}
		result = append(result, p)
		if len(runes) >= 4 {
			result = append(result, string(runes[:len(runes)/2]))
		}
	}
	return result
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
