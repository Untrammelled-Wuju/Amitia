package business

import (
	"encoding/json"
	"errors"
	"math"
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
		for _, key := range []string{"retentionLevel", "memoryStrength", "pinned", "reinforceCount", "strengthUpdatedAt", "lastReinforcedAt", "decayState", "decayPolicy"} {
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
			assigned := memorypolicy.NewLifecycleWithPolicy(state.DecayPolicy, 0, false, content.Importance, now)
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
		if resource.Kind != "memory" || resource.Deleted || seen[resource.ID] || !coordination.ResourceRecallable(resource.Body, now) {
			continue
		}
		seen[resource.ID] = true
		var document map[string]json.RawMessage
		if err := json.Unmarshal(resource.Body, &document); err != nil {
			amitialog.Printf("[original-error] %v", err)
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
			reinforced := state.Reinforce(now)
			memorypolicy.WriteLifecycle(document, reinforced)
			storeLastRetrievalFeatures(document, reinforced, now)
			touchMemory(document, now)
			mutations[index].Body = body(document)
		} else {
			reinforced := state.Reinforce(now)
			memorypolicy.WriteLifecycle(document, reinforced)
			storeLastRetrievalFeatures(document, reinforced, now)
			touchMemory(document, now)
			mutations = append(mutations, coordination.Mutation{Kind: "memory", ID: resource.ID, RoleID: resource.RoleID, ExpectedRevision: resource.Revision, Body: body(document)})
		}
	}
	return mutations
}

func storeLastRetrievalFeatures(document map[string]json.RawMessage, state memorypolicy.Lifecycle, now time.Time) {
	var content map[string]json.RawMessage
	if err := json.Unmarshal(document["content"], &content); err != nil {
		amitialog.Printf("[original-error] %v", err)
		content = map[string]json.RawMessage{}
	}
	helpful, unhelpful := memoryInt(document, "helpfulRetrievalCount"), memoryInt(document, "unhelpfulRetrievalCount")
	features := memoryRetrievalFeatures{
		Semantic:      .5,
		Keyword:       .5,
		Importance:    math.Max(0, math.Min(10, memoryNumber(content["importance"], 0))) / 10,
		Confidence:    math.Max(0, math.Min(100, memoryNumber(content["confidence"], 50))) / 100,
		Strength:      math.Max(0, math.Min(1, state.MemoryStrength)),
		Recency:       memoryRecency(document, content, now),
		Verification:  .55,
		Reinforcement: math.Min(5, math.Max(0, memoryNumber(document["reinforceCount"], 0))) / 5,
		Utility:       float64(helpful+1) / float64(helpful+unhelpful+2),
		Validity:      memoryValidityFactor(content, now),
	}
	document["lastRetrievalFeatures"] = body(features)
}

func snapshotForRecallEvidence(snapshot coordination.DataSnapshot, evidence []coordination.ResourceVersion) coordination.DataSnapshot {
	if len(evidence) == 0 {
		return coordination.DataSnapshot{OwnerID: snapshot.OwnerID, Role: snapshot.Role}
	}
	expected := map[string]int64{}
	for _, item := range evidence {
		if item.Kind == "memory" && item.ID != "" && item.Revision > 0 {
			expected[item.ID] = item.Revision
		}
	}
	recalled := coordination.DataSnapshot{OwnerID: snapshot.OwnerID, Role: snapshot.Role}
	for _, resource := range snapshot.Resources {
		if resource.Kind == "memory" && expected[resource.ID] == resource.Revision {
			recalled.Resources = append(recalled.Resources, resource)
			delete(expected, resource.ID)
		}
	}
	return recalled
}

func touchMemory(document map[string]json.RawMessage, now time.Time) {
	document["retrievedCount"] = body(memoryInt(document, "retrievedCount") + 1)
	document["injectedCount"] = body(memoryInt(document, "injectedCount") + 1)
	document["useCount"] = body(memoryInt(document, "useCount") + 1)
	document["lastUsedAt"] = body(now.UTC().Format(time.RFC3339Nano))
}

func memoryInt(document map[string]json.RawMessage, key string) int {
	raw, exists := document[key]
	if !exists || len(raw) == 0 {
		return 0
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	return value
}
