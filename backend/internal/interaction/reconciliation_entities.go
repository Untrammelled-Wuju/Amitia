package interaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/mindruntime"
)

func AgentSnapshotEntities(snapshot *AgentReconciliationSnapshot) []mindruntime.ReconciliationEntity {
	if snapshot == nil {
		return nil
	}
	entities := make([]mindruntime.ReconciliationEntity, 0, 64)
	for i := range snapshot.Goals {
		entities = append(entities, goalEntity(snapshot.Goals[i]))
	}
	for i := range snapshot.Observations {
		entities = append(entities, observationEntity(snapshot.Observations[i]))
	}
	for i := range snapshot.Tasks {
		entities = append(entities, taskEntity(snapshot.Tasks[i]))
	}
	for i := range snapshot.Workflows {
		entities = append(entities, workflowEntity(snapshot.Workflows[i]))
	}
	for i := range snapshot.Invocations {
		entities = append(entities, invocationEntity(snapshot.Invocations[i]))
	}
	return entities
}

func goalEntity(g decision.Goal) mindruntime.ReconciliationEntity {
	var status string
	if g.Status != "" {
		status = string(g.Status)
	}
	fields := map[string]string{
		"type":        string(g.Type),
		"priority":    string(g.Priority),
		"status":      status,
		"progress":    fmt.Sprintf("%.4f", g.Progress),
		"revision":    fmt.Sprintf("%d", g.Revision),
		"description": g.Description,
	}
	if g.LastObservationID != "" {
		fields["lastObservationId"] = g.LastObservationID
	}
	refs := map[string]string{}
	if g.UserID != "" {
		refs["userId"] = g.UserID
	}
	if g.CharacterID != "" {
		refs["characterId"] = g.CharacterID
	}
	if g.ConversationID != "" {
		refs["conversationId"] = g.ConversationID
	}
	return mindruntime.ReconciliationEntity{
		Store:      "agent_goal",
		Kind:       "goal",
		Key:        g.ID,
		Version:    fmt.Sprintf("%d", g.Revision),
		Status:     status,
		Hash:       stableHash(fields),
		Deleted:    false,
		Fields:     fields,
		References: refs,
	}
}

func observationEntity(o decision.Observation) mindruntime.ReconciliationEntity {
	fields := map[string]string{
		"kind":     string(o.Kind),
		"outcome":  string(o.Outcome),
		"targetKind": string(o.TargetKind),
	}
	if o.Version != "" {
		fields["version"] = string(o.Version)
	}
	if !o.ObservedAt.IsZero() {
		fields["observedAt"] = o.ObservedAt.UTC().Format("20060102T150405Z")
	}
	refs := map[string]string{}
	if o.PlanID != "" {
		refs["planId"] = o.PlanID
	}
	if o.ActionID != "" {
		refs["actionId"] = o.ActionID
	}
	if o.InteractionID != "" {
		refs["interactionId"] = o.InteractionID
	}
	if o.RequestID != "" {
		refs["requestId"] = o.RequestID
	}
	if o.UserID != "" {
		refs["userId"] = o.UserID
	}
	if o.CharacterID != "" {
		refs["characterId"] = o.CharacterID
	}
	if o.ConversationID != "" {
		refs["conversationId"] = o.ConversationID
	}
	if o.CandidateID != "" {
		refs["candidateId"] = o.CandidateID
	}
	if o.InvocationID != "" {
		refs["invocationId"] = o.InvocationID
	}
	if o.ToolID != "" {
		refs["toolId"] = o.ToolID
	}
	if o.ExternalCallID != "" {
		refs["externalCallId"] = o.ExternalCallID
	}
	if len(o.GoalIDs) > 0 {
		if b, err := json.Marshal(o.GoalIDs); err == nil {
			refs["goalIds"] = string(b)
		}
	}
	return mindruntime.ReconciliationEntity{
		Store:      "agent_observation",
		Kind:       "observation",
		Key:        o.ID,
		Version:    o.Version,
		Status:     string(o.Outcome),
		Hash:       stableHash(fields),
		Deleted:    false,
		Fields:     fields,
		References: refs,
	}
}

func taskEntity(t AgentTaskRef) mindruntime.ReconciliationEntity {
	fields := map[string]string{
		"status":     t.Status,
		"generation": fmt.Sprintf("%d", t.Generation),
		"completed":  fmt.Sprintf("%v", t.Completed),
	}
	refs := map[string]string{}
	if t.InvocationID != "" {
		refs["invocationId"] = t.InvocationID
	}
	return mindruntime.ReconciliationEntity{
		Store:      "agent_task",
		Kind:       "task",
		Key:        t.TaskRunID,
		Version:    fmt.Sprintf("%d", t.Generation),
		Status:     t.Status,
		Hash:       stableHash(fields),
		Deleted:    false,
		Fields:     fields,
		References: refs,
	}
}

func workflowEntity(w AgentWorkflowRef) mindruntime.ReconciliationEntity {
	fields := map[string]string{
		"status":   w.Status,
		"completed": fmt.Sprintf("%v", w.Completed),
		"attempts": fmt.Sprintf("%d", w.Attempts),
	}
	refs := map[string]string{}
	if w.WorkflowID != "" {
		refs["workflowId"] = w.WorkflowID
	}
	return mindruntime.ReconciliationEntity{
		Store:      "agent_workflow",
		Kind:       "workflow",
		Key:        w.ExecutionID,
		Version:    fmt.Sprintf("%d", w.Attempts),
		Status:     w.Status,
		Hash:       stableHash(fields),
		Deleted:    false,
		Fields:     fields,
		References: refs,
	}
}

func invocationEntity(inv AgentInvocationRef) mindruntime.ReconciliationEntity {
	fields := map[string]string{
		"status":    inv.Status,
		"completed": fmt.Sprintf("%v", inv.Completed),
	}
	refs := map[string]string{}
	if inv.CapabilityID != "" {
		refs["capabilityId"] = inv.CapabilityID
	}
	return mindruntime.ReconciliationEntity{
		Store:      "agent_invocation",
		Kind:       "invocation",
		Key:        inv.InvocationID,
		Version:    inv.Status,
		Status:     inv.Status,
		Hash:       stableHash(fields),
		Deleted:    false,
		Fields:     fields,
		References: refs,
	}
}

func stableHash(fields map[string]string) string {
	h := sha256.New()
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0x00})
		h.Write([]byte(fields[k]))
		h.Write([]byte{0x00})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))[:32]
}
