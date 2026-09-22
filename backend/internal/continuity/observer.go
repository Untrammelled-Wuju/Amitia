package continuity

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Observer struct {
	repo         *Repository
	intelligence *Intelligence
	waits        *WaitCoordinator
}

func NewObserver(repo *Repository) *Observer                        { return &Observer{repo: repo} }
func (o *Observer) SetIntelligence(intelligence *Intelligence)      { o.intelligence = intelligence }
func (o *Observer) SetWaitCoordinator(coordinator *WaitCoordinator) { o.waits = coordinator }

func (o *Observer) Observe(ctx context.Context, payload ObservePayload) error {
	if o == nil || o.repo == nil || strings.TrimSpace(payload.ThreadID) == "" {
		return nil
	}
	thread, err := o.repo.GetThread(payload.ThreadID, payload.SpaceID)
	if err != nil || thread == nil {
		return err
	}
	if thread.Status.IsTerminal() {
		return nil
	}
	idempotencyKey := strings.TrimSpace(payload.RequestID) + "|continuity.observe"
	if strings.TrimSpace(payload.RequestID) == "" {
		idempotencyKey = payload.ThreadID + "|" + payload.ConversationID + "|" + shortText(payload.UserMessage, 80) + "|" + shortText(payload.AssistantReply, 80)
	}
	existing, err := o.repo.GetEventByIdempotencyKey(idempotencyKey)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	eventPayload, _ := json.Marshal(map[string]string{"summary": eventSummary(payload.UserMessage, payload.AssistantReply), "userMessage": shortText(payload.UserMessage, 500), "assistantReply": shortText(payload.AssistantReply, 1000)})
	openWaits, _ := o.repo.ListOpenWaits(thread.ID, 30)
	patch := ThreadPatch{}
	if o.intelligence != nil && o.intelligence.Available() {
		if modelPatch, modelErr := o.intelligence.Extract(ctx, thread, openWaits, payload); modelErr == nil {
			patch = modelPatch
		}
	}
	if !patch.StateChanged && len(patch.Waits) == 0 && patch.EventSummary == "" {
		patch = heuristicThreadPatch(thread, payload)
	}
	return o.repo.WithTransaction(func(repo *Repository) error {
		inserted, insertErr := repo.AppendEvent(&ThreadEvent{ID: uuid.New().String(), ThreadID: thread.ID, EventType: "turn.observed", SourceType: normalizedSource(payload.Source), ConversationID: payload.ConversationID, RequestID: payload.RequestID, ExecutionID: payload.ExecutionID, PayloadJSON: string(eventPayload), IdempotencyKey: idempotencyKey, OccurredAt: time.Now().UTC()})
		if insertErr != nil || !inserted {
			return insertErr
		}
		latest, getErr := repo.GetThread(thread.ID, payload.SpaceID)
		if getErr != nil || latest == nil {
			return getErr
		}
		if latest.Status.IsTerminal() {
			return nil
		}
		if strings.TrimSpace(payload.UserMessage) != "" {
			open, listErr := repo.ListOpenWaits(latest.ID, 50)
			if listErr != nil {
				return listErr
			}
			userWaits := make([]Wait, 0, 1)
			for i := range open {
				if open[i].WaitType == WaitTypeUser {
					userWaits = append(userWaits, open[i])
				}
			}
			if len(userWaits) == 1 {
				if o.waits != nil {
					if _, resolveErr := o.waits.resolveWait(ctx, repo, userWaits[0].ID, "user_message", map[string]any{"requestId": payload.RequestID}, false); resolveErr != nil {
						return resolveErr
					}
				} else if resolveErr := repo.ResolveOpenWaits(latest.ID, WaitTypeUser); resolveErr != nil {
					return resolveErr
				}
			}
		}
		latest, getErr = repo.GetThread(thread.ID, payload.SpaceID)
		if getErr != nil || latest == nil {
			return getErr
		}
		if latest.Status.IsTerminal() {
			return nil
		}
		if applyErr := o.applyPatch(ctx, repo, latest, payload, patch); applyErr != nil {
			return applyErr
		}
		return repo.Bind(thread.ID, "conversation", payload.ConversationID, "context", "observer", 1)
	})
}

func (o *Observer) applyPatch(ctx context.Context, repo *Repository, thread *Thread, payload ObservePayload, patch ThreadPatch) error {
	if thread == nil {
		return nil
	}
	updates := map[string]interface{}{}
	if patch.StateChanged {
		if patch.Title != "" {
			updates["title"] = patch.Title
		}
		if patch.Goal != "" {
			updates["goal"] = patch.Goal
		}
		if patch.Summary != "" {
			updates["summary"] = patch.Summary
		}
		if patch.CurrentState != "" {
			updates["current_state"] = patch.CurrentState
		}
		if patch.NextAction != "" || patch.Status.IsTerminal() {
			updates["next_action"] = patch.NextAction
		}
		if patch.Confidence > 0 {
			updates["confidence"] = patch.Confidence
		}
		if patch.Status != "" && ValidThreadStatus(patch.Status) {
			updates["status"] = patch.Status
			if patch.Status.IsTerminal() {
				now := time.Now().UTC()
				updates["completed_at"] = &now
			}
		}
		updates["last_active_at"] = time.Now().UTC()
	}
	if len(updates) > 0 {
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			latest, getErr := repo.GetThread(thread.ID, payload.SpaceID)
			if getErr != nil || latest == nil {
				return getErr
			}
			_, err = repo.UpdateThreadCAS(latest.ID, latest.Revision, updates)
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
		}
		if err != nil {
			return err
		}
	}

	if patch.EventSummary != "" {
		payloadJSON, _ := json.Marshal(map[string]any{"summary": patch.EventSummary, "confidence": patch.Confidence})
		key := firstNonEmpty(payload.RequestID, payload.ExecutionID, thread.ID) + "|continuity.state|" + patch.EventType
		_, _ = repo.AppendEvent(&ThreadEvent{ThreadID: thread.ID, EventType: firstNonEmpty(patch.EventType, "turn.state_changed"), SourceType: "continuity_intelligence", ConversationID: payload.ConversationID, RequestID: payload.RequestID, ExecutionID: payload.ExecutionID, PayloadJSON: string(payloadJSON), IdempotencyKey: key, OccurredAt: time.Now().UTC()})
	}

	for _, waitPatch := range patch.Waits {
		if err := o.applyWaitPatch(ctx, repo, thread.ID, payload, waitPatch); err != nil {
			return err
		}
	}
	latest, _ := repo.GetThread(thread.ID, "")
	if latest != nil && latest.Status.IsTerminal() {
		return repo.CancelOpenWaits(latest.ID, "thread_terminal")
	}
	open, _ := repo.ListOpenWaits(thread.ID, 1)
	if len(open) > 0 && latest != nil && latest.Status == ThreadStatusActive {
		_, _ = repo.UpdateThreadCAS(latest.ID, latest.Revision, map[string]interface{}{"status": ThreadStatusWaiting})
	}
	return nil
}

func (o *Observer) applyWaitPatch(ctx context.Context, repo *Repository, threadID string, payload ObservePayload, patch WaitPatch) error {
	switch patch.Action {
	case "resolve":
		wait := o.findWaitForPatch(repo, threadID, patch)
		if wait == nil {
			return nil
		}
		resolution := rawMessageMap(patch.Resolution)
		if o.waits != nil {
			_, err := o.waits.resolveWait(ctx, repo, wait.ID, "continuity_intelligence", resolution, false)
			return err
		}
		_, err := (&WaitCoordinator{}).resolveWait(ctx, repo, wait.ID, "continuity_intelligence", resolution, false)
		return err
	case "cancel":
		wait := o.findWaitForPatch(repo, threadID, patch)
		if wait == nil {
			return nil
		}
		if o.waits != nil {
			_, err := o.waits.cancelWait(repo, wait.ID, "continuity_intelligence")
			return err
		}
		_, err := (&WaitCoordinator{}).cancelWait(repo, wait.ID, "continuity_intelligence")
		return err
	case "create":
		condition := patch.Condition
		if len(condition) == 0 {
			condition = json.RawMessage(`{}`)
		}
		if !safeWaitCondition(patch.Type, condition) {
			return nil
		}
		autoResume := patch.Type != WaitTypeUser
		if patch.AutoResume != nil {
			autoResume = *patch.AutoResume
		}
		var dueAt *time.Time
		if patch.DueAt != "" {
			if parsed, err := time.Parse(time.RFC3339, patch.DueAt); err == nil {
				parsed = parsed.UTC()
				dueAt = &parsed
			}
		}
		if patch.Type == WaitTypeTime && dueAt == nil {
			return nil
		}
		open, _ := repo.ListOpenWaits(threadID, 100)
		for i := range open {
			if open[i].WaitType == patch.Type && strings.TrimSpace(open[i].Description) == strings.TrimSpace(patch.Description) && open[i].ConditionJSON == string(condition) {
				return nil
			}
		}
		wait := &Wait{ThreadID: threadID, WaitType: patch.Type, Status: WaitStatusWaiting, Description: patch.Description, ConditionJSON: string(condition), ResumeHint: patch.ResumeHint, DueAt: dueAt, SourceExecutionID: payload.ExecutionID, AutoResume: autoResume}
		return repo.CreateWait(wait)
	}
	return nil
}

func (o *Observer) findWaitForPatch(repo *Repository, threadID string, patch WaitPatch) *Wait {
	if patch.WaitID != "" {
		wait, _ := repo.GetWait(patch.WaitID)
		if wait != nil && wait.ThreadID == threadID && wait.Status == WaitStatusWaiting {
			return wait
		}
		return nil
	}
	open, _ := repo.ListOpenWaits(threadID, 50)
	var matched *Wait
	for i := range open {
		if patch.Type == "" || open[i].WaitType == patch.Type {
			if matched != nil {
				return nil
			}
			matched = &open[i]
		}
	}
	return matched
}

func safeWaitCondition(waitType string, raw json.RawMessage) bool {
	if waitType == WaitTypeUser || waitType == WaitTypeTime {
		return true
	}
	var condition map[string]any
	if json.Unmarshal(raw, &condition) != nil {
		return false
	}
	keys := []string{}
	switch waitType {
	case WaitTypeDevice:
		keys = []string{"deviceId"}
	case WaitTypeApproval:
		keys = []string{"approvalId", "requestId", "toolCallId"}
	case WaitTypeDependency:
		keys = []string{"executionId", "workflowRunId", "taskRunId", "operationId", "invocationId"}
	case WaitTypeExternal:
		for key, value := range condition {
			if key != "kind" && key != "autoResume" && strings.TrimSpace(toString(value)) != "" {
				return true
			}
		}
		return false
	}
	for _, key := range keys {
		if strings.TrimSpace(toString(condition[key])) != "" {
			return true
		}
	}
	return false
}

func heuristicThreadPatch(thread *Thread, payload ObservePayload) ThreadPatch {
	updates := deriveThreadUpdates(thread, payload)
	patch := ThreadPatch{StateChanged: len(updates) > 0, Confidence: 0.55, EventType: "turn.heuristic", EventSummary: eventSummary(payload.UserMessage, payload.AssistantReply)}
	if value, ok := updates["status"].(ThreadStatus); ok {
		patch.Status = value
	}
	if value, ok := updates["summary"].(string); ok {
		patch.Summary = value
	}
	if value, ok := updates["current_state"].(string); ok {
		patch.CurrentState = value
	}
	if value, ok := updates["next_action"].(string); ok {
		patch.NextAction = value
	}
	if wait := deriveUserWait(thread.ID, payload); wait != nil {
		ar := false
		patch.Waits = append(patch.Waits, WaitPatch{Action: "create", Type: wait.WaitType, Description: wait.Description, Condition: json.RawMessage(wait.ConditionJSON), ResumeHint: wait.ResumeHint, AutoResume: &ar})
	}
	return patch
}

func deriveThreadUpdates(thread *Thread, payload ObservePayload) map[string]interface{} {
	updates := map[string]interface{}{}
	user := strings.TrimSpace(payload.UserMessage)
	reply := strings.TrimSpace(payload.AssistantReply)
	if reply != "" {
		updates["current_state"] = shortText(reply, 420)
		updates["summary"] = shortText("用户："+user+"\nAI："+reply, 900)
		if next := extractNextAction(reply); next != "" {
			updates["next_action"] = next
		}
	}
	if userExplicitlyCompletes(user) {
		updates["status"] = ThreadStatusCompleted
		updates["next_action"] = ""
		return updates
	}
	if thread.Status == ThreadStatusWaiting && user != "" {
		updates["status"] = ThreadStatusActive
	}
	if thread.Status == ThreadStatusPaused && isContinuationCue(user) {
		updates["status"] = ThreadStatusActive
	}
	return updates
}

func deriveUserWait(threadID string, payload ObservePayload) *Wait {
	reply := strings.TrimSpace(payload.AssistantReply)
	if reply == "" {
		return nil
	}
	markers := []string{"请提供", "需要你提供", "需要你确认", "请确认", "等你确认", "等你提供", "等待你的回复", "你确认后", "你提供后"}
	matched := ""
	for _, marker := range markers {
		if strings.Contains(reply, marker) {
			matched = marker
			break
		}
	}
	if matched == "" {
		return nil
	}
	return &Wait{ThreadID: threadID, WaitType: WaitTypeUser, Status: WaitStatusWaiting, Description: shortText(sentenceContaining(reply, matched), 240), ConditionJSON: `{"kind":"user_reply"}`, ResumeHint: "收到用户补充或确认后重新评估并继续", SourceExecutionID: payload.ExecutionID, AutoResume: false}
}

func extractNextAction(reply string) string {
	lines := strings.Split(strings.ReplaceAll(reply, "。", "。\n"), "\n")
	markers := []string{"下一步", "接下来", "然后", "之后需要", "下一项"}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, marker := range markers {
			if strings.Contains(trimmed, marker) {
				return shortText(trimmed, 240)
			}
		}
	}
	return ""
}

func userExplicitlyCompletes(message string) bool {
	v := strings.ToLower(strings.TrimSpace(message))
	markers := []string{"这个完成了", "这个做完了", "已经搞定", "已经完成", "这件事结束", "任务完成", "可以关闭这个", "mark this done", "this is done", "completed this"}
	for _, marker := range markers {
		if strings.Contains(v, marker) {
			return true
		}
	}
	return false
}

func eventSummary(user, reply string) string {
	if strings.TrimSpace(reply) == "" {
		return shortText(user, 180)
	}
	return shortText(reply, 180)
}

func normalizedSource(source string) string {
	if v := strings.TrimSpace(source); v != "" {
		return v
	}
	return "conversation"
}

func sentenceContaining(text, marker string) string {
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '。' || r == '！' || r == '？' || r == '!' || r == '?' })
	for _, part := range parts {
		if strings.Contains(part, marker) {
			return strings.TrimSpace(part)
		}
	}
	return text
}

func rawMessageMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if value, ok := value.(string); ok {
		return value
	}
	bytes, _ := json.Marshal(value)
	return string(bytes)
}

func shortText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:maxRunes])) + "…"
}
