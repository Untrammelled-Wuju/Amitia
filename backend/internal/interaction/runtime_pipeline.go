package interaction

import (
	"context"
	"strings"
	"time"
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
	Version     string                `json:"version"`
	Context     ContextSnapshot       `json:"context"`
	Path        PathType              `json:"path"`
	Budget      []TokenBudgetPlan     `json:"budget"`
	Safety      RuntimeSafetyDecision `json:"safety"`
	Delivery    RuntimeDeliveryIntent `json:"delivery"`
	Transaction TransactionDefinition `json:"transaction"`
	AssembledAt time.Time             `json:"assembledAt"`
}

type RuntimePipeline struct {
	registry        *ContextLoaderRegistry
	classifier      *PathClassifier
	budgetManager   *TokenBudgetManager
	transaction     TransactionDefinition
	snapshotVersion string
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
	return RuntimeAssembly{
		Version:     "orchestrator-runtime-v1",
		Context:     snapshot,
		Path:        path,
		Budget:      p.budgetManager.Allocate(runtimeBudgetModules(snapshot, path)),
		Safety:      runtimeSafety(snapshot),
		Delivery:    runtimeDelivery(scope, req),
		Transaction: p.transaction,
		AssembledAt: time.Now(),
	}
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
