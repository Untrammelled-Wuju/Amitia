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
