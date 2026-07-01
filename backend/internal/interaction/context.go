package interaction

import (
	"encoding/json"
	"time"
)

type ContextSnapshot struct {
	Version        string                             `json:"version"`
	RuntimeProfile SnapshotField[RuntimeProfile]      `json:"runtimeProfile"`
	Conversation   SnapshotField[ConversationState]   `json:"conversation"`
	Psyche         SnapshotField[PsycheState]         `json:"psyche"`
	Relationship   SnapshotField[RelationshipState]   `json:"relationship"`
	Beliefs        SnapshotField[BeliefSet]           `json:"beliefs"`
	Memories       SnapshotField[MemorySet]           `json:"memories"`
	Life           SnapshotField[LifeState]           `json:"life"`
	Channel        SnapshotField[ChannelCapabilities] `json:"channel"`
	AssembledAt    time.Time                          `json:"assembledAt"`
}

func (s ContextSnapshot) SnapshotVersion() string {
	if s.Version == "" {
		return "context-snapshot-v1"
	}
	return s.Version
}

type LoadStatus string

const (
	LoadStatusReady       LoadStatus = "ready"
	LoadStatusUnavailable LoadStatus = "unavailable"
	LoadStatusStale       LoadStatus = "stale"
	LoadStatusError       LoadStatus = "error"
)

type SnapshotField[T any] struct {
	Value   T          `json:"value,omitempty"`
	Source  string     `json:"source"`
	Status  LoadStatus `json:"status"`
	Version string     `json:"version,omitempty"`
}

func FieldReady[T any](v T, source, version string) SnapshotField[T] {
	return SnapshotField[T]{Value: v, Source: source, Status: LoadStatusReady, Version: version}
}

func FieldUnavailable[T any](source string) SnapshotField[T] {
	return SnapshotField[T]{Source: source, Status: LoadStatusUnavailable}
}

func FieldError[T any](source string) SnapshotField[T] {
	return SnapshotField[T]{Source: source, Status: LoadStatusError}
}

type RuntimeProfile struct {
	PersonalitySource string             `json:"personalitySource,omitempty"`
	BehaviorWeights   map[string]float64 `json:"behaviorWeights,omitempty"`
	SafetyLevel       string             `json:"safetyLevel,omitempty"`
	Budget            *TokenBudget       `json:"budget,omitempty"`
}

type TokenBudget struct {
	MaxInputTokens  int `json:"maxInputTokens"`
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type ConversationState struct {
	ConversationID string    `json:"conversationId,omitempty"`
	MessageCount   int       `json:"messageCount"`
	LastMessageAt  time.Time `json:"lastMessageAt,omitempty"`
	CurrentTopic           string              `json:"currentTopic,omitempty"`
	ActiveThreads          []string            `json:"activeThreads,omitempty"`
	LastInteractionSummary string              `json:"lastInteractionSummary,omitempty"`
	AttentionState         *AttentionState     `json:"attentionState,omitempty"`
	StateVersion           string              `json:"stateVersion,omitempty"`
	Scope                  *InteractionScope   `json:"scope,omitempty"`
	EmotionSnapshot        *EmotionSnapshot    `json:"emotionSnapshot,omitempty"`
	RelationshipSnapshot   *RelationshipSnapshot `json:"relationshipSnapshot,omitempty"`
}

type AttentionState struct {
	FocusTarget string    `json:"focusTarget,omitempty"`
	FocusType   string    `json:"focusType,omitempty"`
	LastShift   time.Time `json:"lastShift,omitempty"`
	Intensity   float64   `json:"intensity"`
}

type EmotionSnapshot struct {
	Primary   string             `json:"primary,omitempty"`
	Secondary string             `json:"secondary,omitempty"`
	Intensity float64            `json:"intensity"`
	Values    map[string]float64 `json:"values,omitempty"`
}

type RelationshipSnapshot struct {
	TargetID    string             `json:"targetId,omitempty"`
	Trust       float64            `json:"trust"`
	Familiarity float64            `json:"familiarity"`
	Values      map[string]float64 `json:"values,omitempty"`
}

type PsycheState struct {
	InternalModel map[string]float64 `json:"internalModel,omitempty"`
	Stress        float64            `json:"stress"`
	Fatigue       float64            `json:"fatigue"`
	Arousal       float64            `json:"arousal"`
	MoodPressure  float64            `json:"moodPressure"`
	SocialLoad    float64            `json:"socialLoad"`
	RecoveryHours float64            `json:"recoveryHours"`
	Needs         map[string]float64 `json:"needs,omitempty"`
}

type RelationshipState struct {
	Trust            float64 `json:"trust"`
	Familiarity      float64 `json:"familiarity"`
	Security         float64 `json:"security"`
	Tension          float64 `json:"tension"`
	RepairConfidence float64 `json:"repairConfidence"`
	Boundary         float64 `json:"boundary"`
}

type BeliefSet struct {
	Beliefs  []ResolvedBelief `json:"beliefs,omitempty"`
	Conflict *BeliefConflict  `json:"conflict,omitempty"`
}

type ResolvedBelief struct {
	Key        string  `json:"key"`
	Value      string  `json:"value,omitempty"`
	Confidence float64 `json:"confidence"`
}

type BeliefConflict struct {
	KeyA      string `json:"keyA"`
	ValueA    string `json:"valueA"`
	KeyB      string `json:"keyB"`
	ValueB    string `json:"valueB"`
	RiskLevel string `json:"riskLevel"`
}

type MemorySet struct {
	Memories []MemoryItem `json:"memories,omitempty"`
	Count    int          `json:"count"`
}

type MemoryItem struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Type       string `json:"type,omitempty"`
	Importance int    `json:"importance"`
	Confidence int    `json:"confidence"`
	Scope      string `json:"scope,omitempty"`
}

type LifeState struct {
	Mood  string        `json:"mood,omitempty"`
	Needs []NeedSummary `json:"needs,omitempty"`
}

type NeedSummary struct {
	Kind  string  `json:"kind"`
	Level float64 `json:"level"`
}

type ChannelCapabilities struct {
	Channel       string   `json:"channel"`
	SupportsText  bool     `json:"supportsText"`
	SupportsImage bool     `json:"supportsImage"`
	SupportsVoice bool     `json:"supportsVoice"`
	Features      []string `json:"features,omitempty"`
}

func (s ContextSnapshot) MarshalJSON() ([]byte, error) {
	type Alias ContextSnapshot
	return json.Marshal(Alias(s))
}

func (s *ContextSnapshot) UnmarshalJSON(data []byte) error {
	type Alias ContextSnapshot
	alias := &Alias{}
	if err := json.Unmarshal(data, alias); err != nil {
		return err
	}
	*s = ContextSnapshot(*alias)
	return nil
}

type VoiceContextSnapshot struct {
	SessionID      string           `json:"sessionId"`
	ConversationID string           `json:"conversationId"`
	CharacterID    string           `json:"characterId"`
	LastEventID    string           `json:"lastEventId"`
	StateVersion   int64            `json:"stateVersion"`
	FinalTurns     []VoiceTurnBrief `json:"finalTurns"`
	EndedAt        string           `json:"endedAt"`
}

type VoiceTurnBrief struct {
	TurnID    string `json:"turnId"`
	Text      string `json:"text"`
	IsFinal   bool   `json:"isFinal"`
	Cancelled bool   `json:"cancelled"`
}

func (s VoiceContextSnapshot) ToConversationState() ConversationState {
	return ConversationState{
		ConversationID: s.ConversationID,
		MessageCount:   len(s.FinalTurns),
	}
}

func NewVoiceContextFromSession(sessionID, conversationID, characterID string, stateVersion int64, turns []VoiceTurnBrief) VoiceContextSnapshot {
	return VoiceContextSnapshot{
		SessionID: sessionID, ConversationID: conversationID, CharacterID: characterID,
		StateVersion: stateVersion, FinalTurns: turns,
	}
}
