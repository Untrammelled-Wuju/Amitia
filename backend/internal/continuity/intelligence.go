package continuity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type WorkshopJSONGenerator interface {
	GenerateWorkshopJSON(context.Context, string, string) (string, string, string, error)
}

type Intelligence struct {
	generator WorkshopJSONGenerator
}

func NewIntelligence(generator WorkshopJSONGenerator) *Intelligence {
	return &Intelligence{generator: generator}
}

func (i *Intelligence) Available() bool { return i != nil && i.generator != nil }

func (i *Intelligence) Extract(ctx context.Context, thread *Thread, waits []Wait, payload ObservePayload) (ThreadPatch, error) {
	if !i.Available() || thread == nil {
		return ThreadPatch{}, errors.New("continuity: structured intelligence unavailable")
	}
	current := map[string]any{
		"thread": map[string]any{
			"id": thread.ID, "title": thread.Title, "goal": thread.Goal, "status": thread.Status,
			"summary": thread.Summary, "currentState": thread.CurrentState, "nextAction": thread.NextAction,
		},
		"openWaits": waits,
		"turn": map[string]any{
			"user": payload.UserMessage, "assistant": payload.AssistantReply, "source": payload.Source,
			"conversationId": payload.ConversationID, "requestId": payload.RequestID, "executionId": payload.ExecutionID,
		},
		"now": time.Now().UTC().Format(time.RFC3339),
	}
	input, _ := json.Marshal(current)
	system := `You update a persistent continuity thread after one completed assistant turn.
Return exactly one JSON object and no prose.
Only record changes supported by this turn. Never invent completion, deadlines, devices, approvals, external conditions, or execution IDs.
A thread is the long-lived real-world/work item; it is not the conversation itself.
Keep strings compact and operational. Do not copy the full assistant reply.
Valid status: active, waiting, blocked, paused, completed, cancelled. Use completed/cancelled only when the user or execution result clearly establishes it.
Wait types: user, time, device, external, approval, dependency.
Create device/external/approval/dependency waits only when a concrete selector/identifier is present in the evidence. A user wait is valid when the assistant genuinely requires user input/confirmation. A time wait needs an exact or clearly derivable due time.
For a wait resolution/cancellation, prefer the existing waitId.
Schema:
{"stateChanged":boolean,"title":"","goal":"","status":"","summary":"","currentState":"","nextAction":"","confidence":0.0,"eventType":"","eventSummary":"","waits":[{"action":"create|resolve|cancel","waitId":"","type":"user|time|device|external|approval|dependency","description":"","condition":{},"resumeHint":"","dueAt":"RFC3339 or empty","autoResume":true,"resolution":{}}]}`
	user := "Current continuity state and completed turn:\n" + string(input)
	raw, _, _, err := i.generator.GenerateWorkshopJSON(ctx, system, user)
	if err != nil {
		return ThreadPatch{}, err
	}
	var patch ThreadPatch
	if err := json.Unmarshal([]byte(extractJSONObject(raw)), &patch); err != nil {
		return ThreadPatch{}, fmt.Errorf("continuity: invalid structured thread patch: %w", err)
	}
	normalizeThreadPatch(&patch)
	return patch, nil
}

func normalizeThreadPatch(patch *ThreadPatch) {
	if patch == nil {
		return
	}
	patch.Title = shortText(strings.TrimSpace(patch.Title), 120)
	patch.Goal = shortText(strings.TrimSpace(patch.Goal), 600)
	patch.Summary = shortText(strings.TrimSpace(patch.Summary), 1200)
	patch.CurrentState = shortText(strings.TrimSpace(patch.CurrentState), 700)
	patch.NextAction = shortText(strings.TrimSpace(patch.NextAction), 500)
	patch.EventType = normalizeEventType(patch.EventType)
	patch.EventSummary = shortText(strings.TrimSpace(patch.EventSummary), 400)
	if patch.Status != "" && !ValidThreadStatus(patch.Status) {
		patch.Status = ""
	}
	if patch.Confidence < 0 {
		patch.Confidence = 0
	}
	if patch.Confidence > 1 {
		patch.Confidence = 1
	}
	if len(patch.Waits) > 12 {
		patch.Waits = patch.Waits[:12]
	}
	cleaned := make([]WaitPatch, 0, len(patch.Waits))
	for _, wait := range patch.Waits {
		wait.Action = strings.ToLower(strings.TrimSpace(wait.Action))
		if wait.Action != "create" && wait.Action != "resolve" && wait.Action != "cancel" {
			continue
		}
		wait.WaitID = strings.TrimSpace(wait.WaitID)
		wait.Type = strings.TrimSpace(wait.Type)
		wait.Description = shortText(wait.Description, 300)
		wait.ResumeHint = shortText(wait.ResumeHint, 300)
		wait.DueAt = strings.TrimSpace(wait.DueAt)
		if wait.Action == "create" && !ValidWaitType(wait.Type) {
			continue
		}
		if wait.Action != "create" && wait.WaitID == "" && !ValidWaitType(wait.Type) {
			continue
		}
		cleaned = append(cleaned, wait)
	}
	patch.Waits = cleaned
}

func normalizeEventType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "turn.state_changed"
	}
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			return r
		}
		return -1
	}, value)
	if value == "" {
		return "turn.state_changed"
	}
	return shortText(value, 80)
}

func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return raw
}
