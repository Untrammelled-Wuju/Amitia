package business

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type RuntimePsycheState struct {
	Valence       float64            `json:"valence"`
	Dominance     float64            `json:"dominance"`
	Arousal       float64            `json:"arousal"`
	MoodValence   float64            `json:"moodValence"`
	MoodArousal   float64            `json:"moodArousal"`
	Stress        float64            `json:"stress"`
	Fatigue       float64            `json:"fatigue"`
	MoodPressure  float64            `json:"moodPressure"`
	SocialLoad    float64            `json:"socialLoad"`
	RecoveryHours float64            `json:"recoveryHours"`
	Needs         map[string]float64 `json:"needs,omitempty"`
}

type RuntimeRelationshipState struct {
	Trust            float64 `json:"trust"`
	Familiarity      float64 `json:"familiarity"`
	Security         float64 `json:"security"`
	Tension          float64 `json:"tension"`
	RepairConfidence float64 `json:"repairConfidence"`
	Boundary         float64 `json:"boundary"`
}

type RuntimeNeedItem struct {
	Kind      string  `json:"kind"`
	Level     float64 `json:"level"`
	Baseline  float64 `json:"baseline,omitempty"`
	UpdatedAt string  `json:"updatedAt,omitempty"`
}

type RuntimeStateView struct {
	OwnerID              string                      `json:"ownerId"`
	RoleID               string                      `json:"roleId"`
	Execution            coordination.ExecutionScope `json:"executionScope"`
	Psyche               RuntimePsycheState          `json:"psyche"`
	Relationship         RuntimeRelationshipState    `json:"relationship"`
	Needs                []RuntimeNeedItem           `json:"needs"`
	Beliefs              []map[string]any            `json:"beliefs"`
	PsycheRevision       int64                       `json:"psycheRevision"`
	RelationshipRevision int64                       `json:"relationshipRevision"`
	NeedsRevision        int64                       `json:"needsRevision"`
	UpdatedAt            string                      `json:"updatedAt"`
}

type RuntimeStateEffect struct {
	NeedDeltas        map[string]float64 `json:"needDeltas,omitempty"`
	EventType         string             `json:"eventType"`
	PsycheDelta       float64            `json:"psycheDelta"`
	RelationshipDelta float64            `json:"relationshipDelta"`
	Severity          float64            `json:"severity"`
}

func defaultRuntimePsycheState() RuntimePsycheState {
	return RuntimePsycheState{Valence: 0.5, Dominance: 0.5, Arousal: 0.5, MoodValence: 0.5, MoodArousal: 0.5, Fatigue: 0.3, MoodPressure: 0.5, SocialLoad: 0.2, Needs: map[string]float64{}}
}

func defaultRuntimeRelationshipState() RuntimeRelationshipState {
	return RuntimeRelationshipState{Trust: 0.5, Security: 0.5, RepairConfidence: 0.5, Boundary: 0.5}
}

func defaultRuntimeNeeds(now time.Time) []RuntimeNeedItem {
	defaults := []struct {
		Kind     string
		Level    float64
		Baseline float64
	}{
		{"autonomy", 0.3, 0.3},
		{"clarity", 0.26, 0.24},
		{"connection", 0.38, 0.34},
		{"expression", 0.28, 0.25},
		{"novelty", 0.24, 0.22},
		{"reassurance", 0.32, 0.28},
		{"rest", 0.22, 0.2},
	}
	result := make([]RuntimeNeedItem, 0, len(defaults))
	for _, item := range defaults {
		result = append(result, RuntimeNeedItem{Kind: item.Kind, Level: item.Level, Baseline: item.Baseline, UpdatedAt: now.Format(time.RFC3339)})
	}
	return result
}

func (e *Engine) RuntimeState(ctx context.Context, request Request) (RuntimeStateView, error) {
	now := time.Now().UTC()
	result := RuntimeStateView{
		Psyche:       defaultRuntimePsycheState(),
		Relationship: defaultRuntimeRelationshipState(),
		Needs:        defaultRuntimeNeeds(now),
		Beliefs:      []map[string]any{},
		UpdatedAt:    now.Format(time.RFC3339),
	}
	for _, kind := range coordination.RuntimeStateKinds {
		view, err := e.Query(ctx, request, coordination.DataQuery{ResourceKind: kind, Management: true, Limit: 1})
		if err != nil {
			return RuntimeStateView{}, err
		}
		result.OwnerID, result.RoleID, result.Execution = view.Snapshot.OwnerID, view.Snapshot.Role.ID, view.Scope
		for _, resource := range view.Snapshot.Resources {
			switch resource.Kind {
			case coordination.RuntimeStateKindPsyche:
				var state struct {
					RuntimePsycheState
					UpdatedAt string `json:"updatedAt"`
				}
				if json.Unmarshal(resource.Body, &state) != nil {
					return RuntimeStateView{}, coordination.ErrResourceVersion
				}
				result.Psyche, result.PsycheRevision, result.UpdatedAt = state.RuntimePsycheState, resource.Revision, state.UpdatedAt
			case coordination.RuntimeStateKindRelationship:
				var state struct {
					RuntimeRelationshipState
					UpdatedAt string `json:"updatedAt"`
				}
				if json.Unmarshal(resource.Body, &state) != nil {
					return RuntimeStateView{}, coordination.ErrResourceVersion
				}
				result.Relationship, result.RelationshipRevision, result.UpdatedAt = state.RuntimeRelationshipState, resource.Revision, state.UpdatedAt
			case coordination.RuntimeStateKindNeeds:
				var state struct {
					Needs []RuntimeNeedItem `json:"needs"`
				}
				if json.Unmarshal(resource.Body, &state) != nil {
					return RuntimeStateView{}, coordination.ErrResourceVersion
				}
				if len(state.Needs) > 0 {
					result.Needs = state.Needs
				}
				result.NeedsRevision = resource.Revision
			}
		}
	}
	facts, err := e.Query(ctx, request, coordination.DataQuery{ResourceKind: "fact", Management: true, Limit: 10})
	if err == nil {
		for _, resource := range facts.Snapshot.Resources {
			entry := MemoryConsumerEntry{Resource: resource, Scope: facts.Scope}
			if !MemoryActive(entry) {
				continue
			}
			document, content, contentErr := MemoryContent(entry)
			if contentErr != nil {
				continue
			}
			var confidence float64
			_ = json.Unmarshal(content["confidence"], &confidence)
			result.Beliefs = append(result.Beliefs, map[string]any{"key": MemoryText(document, "key"), "value": MemoryValue(content), "confidence": confidence, "conflicted": false})
		}
	}
	return result, nil
}

func (e *Engine) ApplyRuntimeState(ctx context.Context, request Request, effect RuntimeStateEffect) error {
	if e == nil || e.coordination == nil || e.data == nil || e.model == nil || request.RequestID == "" {
		return errors.New("角色运行状态提交尚未就绪")
	}
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, request.RequestID)
	if err != nil {
		return err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return err
	}
	role, err := coordination.ResolveRole(request.RoleID, roles)
	if err != nil {
		return err
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	if err := coordination.ResolveMemoryScope(&scope, request.MemoryView); err != nil {
		return err
	}
	scope.TurnID, scope.ExecutionID = request.RequestID+"/turn", request.RequestID+"/execution"
	ctx = coordination.WithScope(ctx, scope)
	snapshot := coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID, Role: role}
	if port, ok := e.data.(coordination.ResourcePort); ok {
		for _, document := range []struct {
			Kind string
			ID   string
		}{
			{Kind: coordination.RuntimeStateKindPsyche, ID: "role"},
			{Kind: coordination.RuntimeStateKindRelationship, ID: "user_character"},
			{Kind: coordination.RuntimeStateKindNeeds, ID: "role"},
		} {
			resource, resourceErr := port.Resource(ctx, scope, document.Kind, document.ID)
			if resourceErr != nil {
				return resourceErr
			}
			if resource != nil && !resource.Deleted {
				snapshot.Resources = append(snapshot.Resources, *resource)
			}
		}
	} else {
		snapshot, err = e.data.Snapshot(ctx, scope, coordination.DataQuery{Limit: 1})
		if err != nil {
			return err
		}
	}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		return err
	}
	mutator, ok := e.model.(OwnedStateMutator)
	if !ok {
		return errors.New("计算服务不支持角色运行状态提交")
	}
	runtime := map[string]any{"appraisal": effect}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		return err
	}
	mutations, err := mutator.OwnedStateMutations(ctx, Inference{Scope: scope, Snapshot: snapshot, Emit: func(Event) error { return nil }}, Generation{StateEffects: encoded})
	if err != nil {
		return err
	}
	if len(mutations) == 0 {
		return errors.New("角色运行状态没有生成可提交变更")
	}
	if err := e.submit(ctx, scope, "runtime-state", mutations); err != nil {
		return err
	}
	if err := e.coordination.CompleteRequest(ctx, scope); err != nil {
		return err
	}
	return coordination.ValidateCurrent(ctx)
}
