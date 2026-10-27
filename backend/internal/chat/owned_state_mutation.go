package chat

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/need"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/relationship"
)

type ownedPsycheStateDocument struct {
	interaction.PsycheState
	UpdatedAt          string `json:"updatedAt"`
	SourceRequestID    string `json:"sourceRequestId"`
	SourceConversation string `json:"sourceConversationId,omitempty"`
	SourceTurnID       string `json:"sourceTurnId,omitempty"`
}

type ownedRelationshipStateDocument struct {
	interaction.RelationshipState
	UpdatedAt          string `json:"updatedAt"`
	SourceRequestID    string `json:"sourceRequestId"`
	SourceConversation string `json:"sourceConversationId,omitempty"`
	SourceTurnID       string `json:"sourceTurnId,omitempty"`
}

type ownedNeedStateDocument struct {
	interaction.NeedState
	UpdatedAt          string `json:"updatedAt"`
	SourceRequestID    string `json:"sourceRequestId"`
	SourceConversation string `json:"sourceConversationId,omitempty"`
	SourceTurnID       string `json:"sourceTurnId,omitempty"`
}

func (s *service) OwnedStateMutations(ctx context.Context, inference business.Inference, generation business.Generation) ([]coordination.Mutation, error) {
	if s == nil || len(generation.StateEffects) == 0 || string(generation.StateEffects) == "null" {
		return nil, nil
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	var runtime interaction.RuntimeAssembly
	if err := json.Unmarshal(generation.StateEffects, &runtime); err != nil {
		return nil, errors.New("角色运行状态快照格式无效")
	}
	appraisal := runtime.Appraisal
	if appraisal == nil {
		appraisal = &interaction.AppraisalResult{EventType: "chat"}
	}
	now := time.Now().UTC()
	psycheState, psycheRevision := currentOwnedPsycheState(inference.Snapshot.Resources, inference.Scope.RoleID)
	relationshipState, relationshipRevision := currentOwnedRelationshipState(inference.Snapshot.Resources, inference.Scope.RoleID)
	needState, needRevision := currentOwnedNeedState(inference.Snapshot.Resources, inference.Scope.RoleID)
	psycheState = nextOwnedPsycheState(psycheState, appraisal, now)
	relationshipState = nextOwnedRelationshipState(relationshipState, appraisal, now)
	needState = nextOwnedNeedState(needState, appraisal, now)
	metadata := func() (string, string, string) {
		return now.Format(time.RFC3339Nano), inference.Scope.RequestID, inference.Scope.TurnID
	}
	updatedAt, requestID, turnID := metadata()
	documents := []struct {
		Kind     string
		ID       string
		Revision int64
		Body     any
	}{
		{Kind: coordination.RuntimeStateKindPsyche, ID: coordination.RoleRuntimeStateID(coordination.RuntimeStateKindPsyche, inference.Scope.RoleID), Revision: psycheRevision, Body: ownedPsycheStateDocument{PsycheState: psycheState, UpdatedAt: updatedAt, SourceRequestID: requestID, SourceConversation: inference.ConversationID, SourceTurnID: turnID}},
		{Kind: coordination.RuntimeStateKindRelationship, ID: coordination.RoleRuntimeStateID(coordination.RuntimeStateKindRelationship, inference.Scope.RoleID), Revision: relationshipRevision, Body: ownedRelationshipStateDocument{RelationshipState: relationshipState, UpdatedAt: updatedAt, SourceRequestID: requestID, SourceConversation: inference.ConversationID, SourceTurnID: turnID}},
		{Kind: coordination.RuntimeStateKindNeeds, ID: coordination.RoleRuntimeStateID(coordination.RuntimeStateKindNeeds, inference.Scope.RoleID), Revision: needRevision, Body: ownedNeedStateDocument{NeedState: needState, UpdatedAt: updatedAt, SourceRequestID: requestID, SourceConversation: inference.ConversationID, SourceTurnID: turnID}},
	}
	mutations := make([]coordination.Mutation, 0, len(documents))
	for _, document := range documents {
		body, err := json.Marshal(document.Body)
		if err != nil {
			return nil, err
		}
		mutations = append(mutations, coordination.Mutation{Kind: document.Kind, ID: document.ID, RoleID: inference.Scope.RoleID, ExpectedRevision: document.Revision, Body: body})
	}
	return mutations, coordination.ValidateCurrent(ctx)
}

func currentOwnedPsycheState(resources []coordination.Resource, roleID string) (interaction.PsycheState, int64) {
	var value interaction.PsycheState
	revision := int64(0)
	qualifiedID := coordination.RoleRuntimeStateID(coordination.RuntimeStateKindPsyche, roleID)
	legacyID := coordination.LegacyRuntimeStateID(coordination.RuntimeStateKindPsyche)
	legacyFound := false
	for _, resource := range resources {
		if resource.Kind != coordination.RuntimeStateKindPsyche || resource.RoleID != roleID || resource.Deleted {
			continue
		}
		if resource.ID == qualifiedID {
			if json.Unmarshal(resource.Body, &value) == nil {
				revision = resource.Revision
			}
			break
		}
		if resource.ID == legacyID && json.Unmarshal(resource.Body, &value) == nil {
			legacyFound = true
		}
	}
	if revision == 0 {
		if !legacyFound {
			value = defaultOwnedPsycheState()
		}
	}
	return value, revision
}

func currentOwnedRelationshipState(resources []coordination.Resource, roleID string) (interaction.RelationshipState, int64) {
	var value interaction.RelationshipState
	revision := int64(0)
	qualifiedID := coordination.RoleRuntimeStateID(coordination.RuntimeStateKindRelationship, roleID)
	legacyID := coordination.LegacyRuntimeStateID(coordination.RuntimeStateKindRelationship)
	for _, resource := range resources {
		if resource.Kind != coordination.RuntimeStateKindRelationship || resource.RoleID != roleID || resource.Deleted {
			continue
		}
		if resource.ID == qualifiedID {
			if json.Unmarshal(resource.Body, &value) == nil {
				revision = resource.Revision
			}
			break
		}
		if resource.ID == legacyID {
			_ = json.Unmarshal(resource.Body, &value)
		}
	}
	if value == (interaction.RelationshipState{}) {
		value = interaction.RelationshipState{Trust: 0.5, Familiarity: 0, Security: 0.5, Tension: 0, RepairConfidence: 0.5, Boundary: 0.5}
	}
	return value, revision
}

func currentOwnedNeedState(resources []coordination.Resource, roleID string) (interaction.NeedState, int64) {
	var value interaction.NeedState
	revision := int64(0)
	qualifiedID := coordination.RoleRuntimeStateID(coordination.RuntimeStateKindNeeds, roleID)
	legacyID := coordination.LegacyRuntimeStateID(coordination.RuntimeStateKindNeeds)
	for _, resource := range resources {
		if resource.Kind != coordination.RuntimeStateKindNeeds || resource.RoleID != roleID || resource.Deleted {
			continue
		}
		if resource.ID == qualifiedID {
			if json.Unmarshal(resource.Body, &value) == nil {
				revision = resource.Revision
			}
			break
		}
		if resource.ID == legacyID {
			_ = json.Unmarshal(resource.Body, &value)
		}
	}
	if value.Count == 0 {
		value = defaultOwnedNeedState(time.Now().UTC())
	}
	return value, revision
}

func defaultOwnedPsycheState() interaction.PsycheState {
	return interaction.PsycheState{
		Valence:       0.5,
		Dominance:     0.5,
		MoodValence:   0.5,
		MoodArousal:   0.5,
		Stress:        0,
		Fatigue:       0.3,
		Arousal:       0.5,
		MoodPressure:  0.5,
		SocialLoad:    0.2,
		RecoveryHours: 0,
		Needs:         map[string]float64{},
	}
}

func defaultOwnedNeedState(now time.Time) interaction.NeedState {
	snapshot := need.DefaultSnapshot(now)
	result := interaction.NeedState{Needs: make([]interaction.NeedSummary, 0, len(snapshot.States))}
	for _, kind := range orderedOwnedNeedKinds() {
		state := snapshot.States[kind]
		result.Needs = append(result.Needs, interaction.NeedSummary{Kind: string(kind), Level: state.Level, Baseline: state.Baseline, UpdatedAt: state.UpdatedAt.Format(time.RFC3339)})
	}
	result.Count = len(result.Needs)
	return result
}

func nextOwnedPsycheState(current interaction.PsycheState, appraisal *interaction.AppraisalResult, now time.Time) interaction.PsycheState {
	state := psyche.PsycheState{
		Emotion: psyche.EmotionDimensions{Valence: current.Valence, Arousal: current.Arousal, Dominance: current.Dominance},
		Mood:    psyche.MoodDimensions{MoodValence: current.MoodValence, MoodArousal: current.MoodArousal},
		Stress:  current.Stress,
		Energy:  clampRelationshipValue(1 - current.Fatigue),
	}
	event := buildPsycheEvent("", state, &AppraisalResultBridge{
		EventType:         appraisal.EventType,
		PsycheDelta:       appraisal.PsycheDelta,
		RelationshipDelta: appraisal.RelationshipDelta,
		Severity:          appraisal.Severity,
	})
	event.Timestamp = now
	next := psyche.ApplyEvent(state, event)
	return interaction.PsycheState{
		Valence:       next.Emotion.Valence,
		Dominance:     next.Emotion.Dominance,
		Arousal:       next.Emotion.Arousal,
		MoodValence:   next.Mood.MoodValence,
		MoodArousal:   next.Mood.MoodArousal,
		Stress:        next.Stress,
		Fatigue:       clampRelationshipValue(1 - next.Energy),
		MoodPressure:  clampRelationshipValue(0.5 + (0.5-next.Mood.MoodValence)*0.5 + next.Stress*0.25),
		SocialLoad:    clampRelationshipValue(current.SocialLoad + math.Abs(appraisal.PsycheDelta)*0.08),
		RecoveryHours: math.Max(0, current.RecoveryHours),
		Needs:         cloneFloatMap(current.Needs),
	}
}

func nextOwnedRelationshipState(current interaction.RelationshipState, appraisal *interaction.AppraisalResult, now time.Time) interaction.RelationshipState {
	evidenceKind, intensity := relationshipEvidenceForAppraisal(appraisal)
	result := relationship.UpdateRelationship(relationship.UpdateInput{
		Current: relationship.RelationshipState{
			Trust:            current.Trust,
			Familiarity:      current.Familiarity,
			Security:         current.Security,
			Tension:          current.Tension,
			RepairConfidence: current.RepairConfidence,
			Boundary:         current.Boundary,
		},
		Evidence: []relationship.InteractionEvidence{{ID: appraisal.EventType, Kind: evidenceKind, Intensity: intensity, Confidence: 1}},
		Budget:   relationship.DefaultBudget(),
	})
	_ = now
	return interaction.RelationshipState{
		Trust:            result.Next.Trust,
		Familiarity:      result.Next.Familiarity,
		Security:         result.Next.Security,
		Tension:          result.Next.Tension,
		RepairConfidence: result.Next.RepairConfidence,
		Boundary:         result.Next.Boundary,
	}
}

func nextOwnedNeedState(current interaction.NeedState, appraisal *interaction.AppraisalResult, now time.Time) interaction.NeedState {
	snapshot := need.NeedSnapshot{Version: need.EngineVersionV1, States: map[need.NeedKind]need.NeedState{}, UpdatedAt: now}
	for _, item := range current.Needs {
		kind := need.NeedKind(item.Kind)
		updatedAt, err := time.Parse(time.RFC3339, item.UpdatedAt)
		if err != nil {
			updatedAt = now
		}
		snapshot.States[kind] = need.NeedState{Level: item.Level, Baseline: item.Baseline, UpdatedAt: updatedAt}
	}
	signals := make([]need.NeedSignal, 0, len(appraisal.NeedDeltas))
	keys := make([]string, 0, len(appraisal.NeedDeltas))
	for key := range appraisal.NeedDeltas {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		delta := appraisal.NeedDeltas[key]
		signals = append(signals, need.NeedSignal{Kind: need.NeedKind(key), Pressure: math.Max(delta, 0), Relief: math.Max(-delta, 0), Confidence: 1})
	}
	result := need.UpdateNeeds(need.UpdateInput{Current: snapshot, Signals: signals, Budget: need.DefaultBudget(), Now: now})
	value := interaction.NeedState{Needs: make([]interaction.NeedSummary, 0, len(result.After.States))}
	for _, kind := range orderedOwnedNeedKinds() {
		state := result.After.States[kind]
		value.Needs = append(value.Needs, interaction.NeedSummary{Kind: string(kind), Level: state.Level, Baseline: state.Baseline, UpdatedAt: state.UpdatedAt.Format(time.RFC3339)})
	}
	value.Count = len(value.Needs)
	return value
}

func relationshipEvidenceForAppraisal(appraisal *interaction.AppraisalResult) (relationship.EvidenceKind, float64) {
	intensity := clampRelationshipValue(0.2 + appraisal.Severity*0.8)
	switch appraisal.EventType {
	case "praise":
		return relationship.EvidenceKindSupportive, intensity
	case "apology":
		return relationship.EvidenceKindRepair, intensity
	case "help", "emotional":
		return relationship.EvidenceKindPositive, intensity
	case "complaint":
		return relationship.EvidenceKindConflict, intensity
	case "boundary_cross":
		return relationship.EvidenceKindBoundary, intensity
	case "cold":
		return relationship.EvidenceKindWithdrawal, intensity
	default:
		return relationship.EvidenceKindPositive, 0.2
	}
}

func orderedOwnedNeedKinds() []need.NeedKind {
	return []need.NeedKind{need.NeedKindAutonomy, need.NeedKindClarity, need.NeedKindConnection, need.NeedKindExpression, need.NeedKindNovelty, need.NeedKindReassurance, need.NeedKindRest}
}

func cloneFloatMap(source map[string]float64) map[string]float64 {
	if len(source) == 0 {
		return map[string]float64{}
	}
	result := make(map[string]float64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func clampRelationshipValue(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
