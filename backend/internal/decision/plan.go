package decision

import "time"

type PlanVersion string

const (
	PlanVersionV1 PlanVersion = "behavior-plan-v1"
	PlanVersionV2 PlanVersion = "behavior-plan-v2"
)

type BehaviorPlanIntent string

const (
	PlanIntentReply     BehaviorPlanIntent = "reply"
	PlanIntentClarify   BehaviorPlanIntent = "clarify"
	PlanIntentSupport   BehaviorPlanIntent = "support"
	PlanIntentBoundary  BehaviorPlanIntent = "boundary"
	PlanIntentRepair    BehaviorPlanIntent = "repair"
	PlanIntentProactive BehaviorPlanIntent = "proactive"
	PlanIntentObserve   BehaviorPlanIntent = "observe"
	PlanIntentTool      BehaviorPlanIntent = "tool"
)

type BehaviorPlanStrategy string

const (
	StrategyRespondNaturally       BehaviorPlanStrategy = "respond_naturally"
	StrategyRequestClarification   BehaviorPlanStrategy = "request_clarification"
	StrategyProvideSupport         BehaviorPlanStrategy = "provide_support"
	StrategySetBoundary            BehaviorPlanStrategy = "set_boundary"
	StrategyRepairRelationship     BehaviorPlanStrategy = "repair_relationship"
	StrategyProactiveCheck         BehaviorPlanStrategy = "proactive_check"
	StrategyObserveWithoutResponse BehaviorPlanStrategy = "observe_without_response"
	StrategyResolveViaTool         BehaviorPlanStrategy = "resolve_via_tool"
)

type PlanContentPolicy struct {
	AllowedTopics   []string `json:"allowedTopics,omitempty"`
	ForbiddenTopics []string `json:"forbiddenTopics,omitempty"`
}

type PlanSafetyContext struct {
	Level   BehaviorSafetyLevel
	Blocked bool
	Reasons []string
}

type BehaviorTag string

const (
	BehaviorTagReply          BehaviorTag = "reply"
	BehaviorTagAskClarify     BehaviorTag = "ask_clarify"
	BehaviorTagOfferSupport   BehaviorTag = "offer_support"
	BehaviorTagSetBoundary    BehaviorTag = "set_boundary"
	BehaviorTagRepair         BehaviorTag = "repair"
	BehaviorTagProactiveCheck BehaviorTag = "proactive_check"
	BehaviorTagDelay          BehaviorTag = "delay"
)

type BehaviorChannel string

const (
	BehaviorChannelChat      BehaviorChannel = "chat"
	BehaviorChannelProactive BehaviorChannel = "proactive"
	BehaviorChannelSystem    BehaviorChannel = "system"
)

type BehaviorPriority string

const (
	BehaviorPriorityLow      BehaviorPriority = "low"
	BehaviorPriorityNormal   BehaviorPriority = "normal"
	BehaviorPriorityHigh     BehaviorPriority = "high"
	BehaviorPriorityCritical BehaviorPriority = "critical"
)

type BehaviorSafetyLevel string

const (
	BehaviorSafetyLevelNormal       BehaviorSafetyLevel = "normal"
	BehaviorSafetyLevelConservative BehaviorSafetyLevel = "conservative"
	BehaviorSafetyLevelBlocked      BehaviorSafetyLevel = "blocked"
)

type CompiledPersonalityRef struct {
	Version             string                  `json:"version"`
	SourceCharacterID   string                  `json:"sourceCharacterId,omitempty"`
	RawConfig           map[string]any          `json:"rawConfig,omitempty"`
	BehaviorWeights     map[BehaviorTag]float64 `json:"behaviorWeights,omitempty"`
	ExpressionPolicyKey string                  `json:"expressionPolicyKey,omitempty"`
}

type BehaviorCandidate struct {
	ID                  string               `json:"id"`
	ActionType          CandidateActionType  `json:"actionType"`
	Tag                 BehaviorTag          `json:"tag"`
	Channel             BehaviorChannel      `json:"channel"`
	BaseScore           float64              `json:"baseScore"`
	PersonalityScore    float64              `json:"personalityScore"`
	NeedScore           float64              `json:"needScore"`
	RelationshipScore   float64              `json:"relationshipScore"`
	AffectScore         float64              `json:"affectScore"`
	UserPreferenceScore float64              `json:"userPreferenceScore"`
	RiskScore           float64              `json:"riskScore"`
	RepeatPenalty       float64              `json:"repeatPenalty"`
	FatiguePenalty      float64              `json:"fatiguePenalty"`
	FinalScore          float64              `json:"finalScore"`
	Reasons             []BehaviorReason     `json:"reasons,omitempty"`
	Constraints         []BehaviorConstraint `json:"constraints,omitempty"`
	Overrides           []string             `json:"overrides,omitempty"`
	ScoringVersion      string               `json:"scoringVersion,omitempty"`
}

type BehaviorReason struct {
	Source string  `json:"source"`
	Key    string  `json:"key"`
	Delta  float64 `json:"delta"`
}

type BehaviorConstraint struct {
	Kind     string  `json:"kind"`
	Limit    float64 `json:"limit,omitempty"`
	Observed float64 `json:"observed,omitempty"`
	Hard     bool    `json:"hard"`
}

type BehaviorPlan struct {
	Version              PlanVersion               `json:"version"`
	ID                   string                    `json:"id,omitempty"`
	UserID               string                    `json:"userId,omitempty"`
	CharacterID          string                    `json:"characterId,omitempty"`
	ConversationID       string                    `json:"conversationId,omitempty"`
	InteractionID        string                    `json:"interactionId,omitempty"`
	RequestID            string                    `json:"requestId,omitempty"`
	CreatedAt            time.Time                 `json:"createdAt"`
	Selected             BehaviorCandidate         `json:"selected"`
	Alternatives         []BehaviorCandidate       `json:"alternatives,omitempty"`
	SelectionDisposition ArbitrationDisposition    `json:"selectionDisposition"`
	Priority             BehaviorPriority          `json:"priority"`
	SafetyLevel          BehaviorSafetyLevel       `json:"safetyLevel"`
	DoNotSend            bool                      `json:"doNotSend"`
	NeedsExpression      bool                      `json:"needsExpression"`
	ExpressionPlanID     string                    `json:"expressionPlanId,omitempty"`
	Personality          CompiledPersonalityRef    `json:"personality"`
	Psyche               PsycheSignalSet           `json:"psyche"`
	Relationship         RelationshipSnapshot      `json:"relationship"`
	Life                 LifeSnapshot              `json:"life"`
	Audit                BehaviorAudit             `json:"audit"`
	Intent               BehaviorPlanIntent        `json:"intent"`
	Strategy             BehaviorPlanStrategy      `json:"strategy"`
	PlanContentPolicy    PlanContentPolicy         `json:"planContentPolicy,omitempty"`
	ResponseGoal         string                    `json:"responseGoal"`
	ToneHint             ExpressionTone            `json:"toneHint,omitempty"`
	GoalIDs              []string                  `json:"goalIds,omitempty"`
	GoalRefs             []GoalRef                 `json:"goalRefs,omitempty"`
	GoalProgress         []GoalProgressExpectation `json:"goalProgress,omitempty"`
	IntentionIDs         []string                  `json:"intentionIds,omitempty"`
}

type PsycheSignalSet struct {
	Emotions      []EmotionSignal  `json:"emotions,omitempty"`
	Mood          ScalarSignal     `json:"mood"`
	Valence       ScalarSignal     `json:"valence"`
	Arousal       ScalarSignal     `json:"arousal"`
	Dominance     ScalarSignal     `json:"dominance"`
	MoodValence   ScalarSignal     `json:"moodValence"`
	MoodArousal   ScalarSignal     `json:"moodArousal"`
	Stress        ScalarSignal     `json:"stress"`
	CognitiveLoad ScalarSignal     `json:"cognitiveLoad"`
	Needs         []NeedSignal     `json:"needs,omitempty"`
	Regulation    RegulationSignal `json:"regulation"`
	LastUpdatedAt time.Time        `json:"lastUpdatedAt,omitempty"`
}

type EmotionSignal struct {
	Kind         string    `json:"kind"`
	Intensity    float64   `json:"intensity"`
	SourceEvent  string    `json:"sourceEvent,omitempty"`
	OnsetAt      time.Time `json:"onsetAt,omitempty"`
	DecayProfile string    `json:"decayProfile,omitempty"`
}

type NeedSignal struct {
	Kind      string  `json:"kind"`
	Level     float64 `json:"level"`
	Baseline  float64 `json:"baseline"`
	Trend     float64 `json:"trend,omitempty"`
	Saturated bool    `json:"saturated"`
}

type ScalarSignal struct {
	Value      float64 `json:"value"`
	Baseline   float64 `json:"baseline,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type RegulationSignal struct {
	Strategy       string   `json:"strategy,omitempty"`
	ExpressionMode string   `json:"expressionMode,omitempty"`
	AppraisalID    string   `json:"appraisalId,omitempty"`
	RevisionID     string   `json:"revisionId,omitempty"`
	DelayedReasons []string `json:"delayedReasons,omitempty"`
}

type RelationshipSnapshot struct {
	UserID        string                                               `json:"userId,omitempty"`
	CharacterID   string                                               `json:"characterId,omitempty"`
	Dimensions    map[RelationshipDimension]RelationshipDimensionValue `json:"dimensions,omitempty"`
	LastChangedAt time.Time                                            `json:"lastChangedAt,omitempty"`
}

type RelationshipDimension string

const (
	RelationshipFamiliarity      RelationshipDimension = "familiarity"
	RelationshipTrust            RelationshipDimension = "trust"
	RelationshipIntimacy         RelationshipDimension = "intimacy"
	RelationshipSafety           RelationshipDimension = "safety"
	RelationshipReciprocity      RelationshipDimension = "reciprocity"
	RelationshipRespect          RelationshipDimension = "respect"
	RelationshipConflict         RelationshipDimension = "conflict"
	RelationshipRepairConfidence RelationshipDimension = "repair_confidence"
)

type RelationshipDimensionValue struct {
	Value         float64   `json:"value"`
	Baseline      float64   `json:"baseline"`
	EvidenceIDs   []string  `json:"evidenceIds,omitempty"`
	LastChangedAt time.Time `json:"lastChangedAt,omitempty"`
}

type LifeSnapshot struct {
	Energy       float64   `json:"energy"`
	Fatigue      float64   `json:"fatigue"`
	Busy         float64   `json:"busy"`
	Activity     string    `json:"activity,omitempty"`
	Availability string    `json:"availability,omitempty"`
	Source       string    `json:"source,omitempty"`
	ValidUntil   time.Time `json:"validUntil,omitempty"`
}

type BehaviorAudit struct {
	FormulaVersion   string   `json:"formulaVersion,omitempty"`
	ParameterVersion string   `json:"parameterVersion,omitempty"`
	ConflictIDs      []string `json:"conflictIds,omitempty"`
	SnapshotID       string   `json:"snapshotId,omitempty"`
	ReplayEventID    string   `json:"replayEventId,omitempty"`
	Diagnostics      []string `json:"diagnostics,omitempty"`
}
