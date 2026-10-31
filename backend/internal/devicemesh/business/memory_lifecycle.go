package business

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memorypolicy"
	amitialog "log"
)

func modelMemoryDerived(derived []DerivedMemory) []DerivedMemory {
	result := append([]DerivedMemory(nil), derived...)
	for index, item := range result {
		var content map[string]json.RawMessage
		if json.Unmarshal(item.Body, &content) != nil || content == nil {
			continue
		}
		for _, key := range []string{"retentionLevel", "memoryStrength", "pinned", "reinforceCount", "strengthUpdatedAt", "lastReinforcedAt", "decayState"} {
			delete(content, key)
		}
		if item.Kind == "fact" || item.Kind == "profile" || item.Kind == "episodic" {
			content["source"] = body("extraction")
		}
		result[index].Body = body(content)
	}
	return result
}

func editMemoryLifecycle(document map[string]json.RawMessage, changes map[string]json.RawMessage, now time.Time) error {
	state := memorypolicy.ReadLifecycle(document, now).Effective(now)
	changed := false
	if raw, exists := changes["retentionLevel"]; exists {
		if json.Unmarshal(raw, &state.RetentionLevel) != nil || state.RetentionLevel < 1 || state.RetentionLevel > 5 {
			return errors.New("记忆等级必须为 L1—L5")
		}
		changed = true
	}
	if raw, exists := changes["memoryStrength"]; exists {
		if json.Unmarshal(raw, &state.MemoryStrength) != nil {
			return errors.New("记忆强度无效")
		}
		if err := state.Validate(); err != nil {
			return err
		}
		changed = true
	}
	if raw, exists := changes["pinned"]; exists {
		previous := state.Pinned
		if json.Unmarshal(raw, &state.Pinned) != nil {
			return errors.New("固定记忆开关无效")
		}
		if state.Pinned {
			document["archivedAt"] = body(nil)
		}
		if !state.Pinned && previous && changes["retentionLevel"] == nil {
			var content struct {
				Importance int `json:"importance"`
			}
			if err := json.Unmarshal(document["content"], &content); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			assigned := memorypolicy.NewLifecycle(0, false, content.Importance, now)
			state.RetentionLevel, state.MemoryStrength = assigned.RetentionLevel, assigned.MemoryStrength
		}
		changed = true
	}
	if changed {
		if state.Pinned || state.RetentionLevel == 1 {
			state.RetentionLevel, state.MemoryStrength = 1, 1
		}
		if err := state.Validate(); err != nil {
			return err
		}
		state.StrengthUpdatedAt = now.UTC().Format(time.RFC3339Nano)
		state = state.Effective(now)
		memorypolicy.WriteLifecycle(document, state)
	}
	return nil
}

func reinforceMemoryMutations(mutations []coordination.Mutation, snapshot coordination.DataSnapshot, scope coordination.ExecutionScope, now time.Time) []coordination.Mutation {
	indices := map[string]int{}
	for index, mutation := range mutations {
		if mutation.Kind == "memory" {
			indices[mutation.ID] = index
		}
	}
	seen := map[string]bool{}
	for _, resource := range snapshot.Resources {
		if resource.Kind != "memory" || resource.Deleted || seen[resource.ID] || !coordination.ResourceUsable(resource.Body, now) {
			continue
		}
		seen[resource.ID] = true
		var document map[string]json.RawMessage
		if json.Unmarshal(resource.Body, &document) != nil {
			continue
		}
		state := memorypolicy.ReadLifecycle(document, now)
		if state.Pinned || state.RetentionLevel == 1 {
			continue
		}
		if index, exists := indices[resource.ID]; exists {
			if err := json.Unmarshal(mutations[index].Body, &document); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			memorypolicy.WriteLifecycle(document, state.Reinforce(now))
			mutations[index].Body = body(document)
		} else {
			memorypolicy.WriteLifecycle(document, state.Reinforce(now))
			mutations = append(mutations, coordination.Mutation{Kind: "memory", ID: resource.ID, RoleID: resource.RoleID, ExpectedRevision: resource.Revision, Body: body(document)})
		}
	}
	return mutations
}
