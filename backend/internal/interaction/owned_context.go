package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedContextKey struct{}

func WithOwnedContext(ctx context.Context, scope coordination.ExecutionScope, data coordination.DataSnapshot, conversationID, channel string) (context.Context, error) {
	if err := coordination.ValidateSnapshot(scope, data); err != nil {
		return ctx, err
	}
	var profile character.RoleRuntimeProfile
	if err := json.Unmarshal(data.Role.Profile, &profile); err != nil {
		return ctx, err
	}
	if profile.CharacterID != scope.RoleID {
		return ctx, coordination.ErrWrongOwner
	}
	version := "owned-context-v1"
	snapshot := ContextSnapshot{Version: version, AssembledAt: time.Now().UTC(), RuntimeProfile: FieldReady(runtimeProfileFromRole(&profile), "owned-role", version)}
	fields := map[string]any{"psyche": &snapshot.Psyche, "relationship": &snapshot.Relationship, "beliefs": &snapshot.Beliefs, "memories": &snapshot.Memories, "life": &snapshot.Life, "needs": &snapshot.Needs, "unresolvedThreads": &snapshot.UnresolvedThreads, "temporal": &snapshot.Temporal, "continuity": &snapshot.Continuity}
	for name, raw := range data.RuntimeContext {
		target, ok := fields[name]
		if !ok {
			return ctx, errors.New("Owner 运行上下文包含未知字段")
		}
		var header struct {
			Status LoadStatus `json:"status"`
			Source string     `json:"source"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			return ctx, err
		}
		if header.Source == "" || header.Status != LoadStatusReady && header.Status != LoadStatusUnavailable && header.Status != LoadStatusStale && header.Status != LoadStatusError {
			return ctx, errors.New("Owner 运行上下文状态无效")
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return ctx, err
		}
	}
	conversation := ConversationState{ConversationID: conversationID, StateVersion: version}
	memories, beliefs := []MemoryItem{}, []ResolvedBelief{}
	conversation.MessageCount = 0
	for _, row := range data.Resources {
		if row.Kind == "message" {
			conversation.MessageCount++
			continue
		}
		switch row.Kind {
		case "memory", "fact", "profile", "episodic":
			entry := business.MemoryConsumerEntry{Resource: row, Scope: scope}
			if !business.MemoryActive(entry) {
				continue
			}
			document, content, err := business.MemoryContent(entry)
			if err != nil {
				return ctx, err
			}
			key, value := business.MemoryText(document, "key"), business.MemoryValue(content)
			var importance, confidence float64
			_ = json.Unmarshal(content["importance"], &importance)
			_ = json.Unmarshal(content["confidence"], &confidence)
			memories = append(memories, MemoryItem{Key: key, Value: value, Type: row.Kind, Importance: int(importance), Confidence: int(confidence), Scope: row.RoleID})
			if row.Kind == "fact" {
				beliefs = append(beliefs, ResolvedBelief{Key: key, Value: value, Confidence: confidence})
			}
		case "continuity":
			ingress, _ := business.IngressFromContext(ctx)
			var document business.OwnedContinuity
			if err := json.Unmarshal(row.Body, &document); err != nil {
				return ctx, err
			}
			if ingress.ThreadID == "" || document.Thread.ID != ingress.ThreadID {
				continue
			}
			thread := document.Thread
			value := continuity.Context{ThreadID: thread.ID, Title: thread.Title, Goal: thread.Goal, Status: thread.Status, Summary: thread.Summary, CurrentState: thread.CurrentState, NextAction: thread.NextAction, Revision: thread.Revision}
			for _, wait := range document.Waits {
				value.Waits = append(value.Waits, continuity.WaitSummary{ID: wait.ID, Type: wait.WaitType, Description: wait.Description, ResumeHint: wait.ResumeHint, DueAt: wait.DueAt})
			}
			snapshot.Continuity = FieldReady(value, "owned-continuity", version)
		case "psyche":
			if err := json.Unmarshal(row.Body, &snapshot.Psyche.Value); err != nil {
				return ctx, err
			}
			snapshot.Psyche.Source, snapshot.Psyche.Status, snapshot.Psyche.Version = "owned-psyche", LoadStatusReady, "owned-psyche:"+strconv.FormatInt(row.Revision, 10)
		case "relationship":
			if err := json.Unmarshal(row.Body, &snapshot.Relationship.Value); err != nil {
				return ctx, err
			}
			snapshot.Relationship.Source, snapshot.Relationship.Status, snapshot.Relationship.Version = "owned-relationship", LoadStatusReady, "owned-relationship:"+strconv.FormatInt(row.Revision, 10)
		case "beliefs":
			if err := json.Unmarshal(row.Body, &snapshot.Beliefs.Value); err != nil {
				return ctx, err
			}
			snapshot.Beliefs.Source, snapshot.Beliefs.Status, snapshot.Beliefs.Version = "owned-beliefs", LoadStatusReady, version
		case "life":
			if err := json.Unmarshal(row.Body, &snapshot.Life.Value); err != nil {
				return ctx, err
			}
			snapshot.Life.Source, snapshot.Life.Status, snapshot.Life.Version = "owned-life", LoadStatusReady, version
		case "needs":
			if err := json.Unmarshal(row.Body, &snapshot.Needs.Value); err != nil {
				return ctx, err
			}
			snapshot.Needs.Source, snapshot.Needs.Status, snapshot.Needs.Version = "owned-needs", LoadStatusReady, "owned-needs:"+strconv.FormatInt(row.Revision, 10)
		}
	}
	snapshot.Conversation = FieldReady(conversation, "owned-conversation", version)
	if len(memories) > 0 {
		snapshot.Memories = FieldReady(MemorySet{Memories: memories, Count: len(memories)}, "owned-memories", version)
	}
	if len(beliefs) > 0 {
		snapshot.Beliefs = FieldReady(BeliefSet{Beliefs: beliefs}, "owned-beliefs", version)
	}
	snapshot.Channel = FieldReady(ChannelCapabilities{Channel: channel, SupportsText: true, SupportsImage: true, SupportsVoice: true}, "owned-channel", version)
	return context.WithValue(ctx, ownedContextKey{}, snapshot), nil
}

func ownedContextFromContext(ctx context.Context) (ContextSnapshot, bool) {
	snapshot, ok := ctx.Value(ownedContextKey{}).(ContextSnapshot)
	return snapshot, ok
}
