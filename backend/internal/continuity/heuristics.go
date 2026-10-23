package continuity

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

func BuildHeuristicThreadPatch(thread *Thread, payload ObservePayload) ThreadPatch {
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
		autoResume := false
		patch.Waits = append(patch.Waits, WaitPatch{Action: "create", Type: wait.WaitType, Description: wait.Description, Condition: json.RawMessage(wait.ConditionJSON), ResumeHint: wait.ResumeHint, AutoResume: &autoResume})
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
	if thread.Status == ThreadStatusPaused && IsContinuationCue(user) {
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
	value := strings.ToLower(strings.TrimSpace(message))
	markers := []string{"这个完成了", "这个做完了", "已经搞定", "已经完成", "这件事结束", "任务完成", "可以关闭这个", "mark this done", "this is done", "completed this"}
	for _, marker := range markers {
		if strings.Contains(value, marker) {
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

func sentenceContaining(text, marker string) string {
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '。' || r == '！' || r == '？' || r == '!' || r == '?' })
	for _, part := range parts {
		if strings.Contains(part, marker) {
			return strings.TrimSpace(part)
		}
	}
	return text
}

func shortText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:maxRunes])) + "…"
}

func safeWaitCondition(waitType string, raw json.RawMessage) bool {
	if waitType == WaitTypeUser || waitType == WaitTypeTime {
		return true
	}
	var condition map[string]interface{}
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

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	raw, _ := json.Marshal(value)
	return string(raw)
}

func WaitMatchesSignal(wait Wait, signal Signal) bool {
	if wait.WaitType != signal.WaitType {
		return false
	}
	condition := map[string]interface{}{}
	if strings.TrimSpace(wait.ConditionJSON) != "" && strings.TrimSpace(wait.ConditionJSON) != "{}" {
		if err := json.Unmarshal([]byte(wait.ConditionJSON), &condition); err != nil {
			return false
		}
	}
	if wait.WaitType == WaitTypeTime {
		return wait.DueAt != nil && !wait.DueAt.After(signal.OccurredAt)
	}
	attributes := signal.Attributes
	if attributes == nil {
		attributes = map[string]interface{}{}
	}
	matchKeys := []string{}
	switch wait.WaitType {
	case WaitTypeDevice:
		matchKeys = []string{"deviceId"}
	case WaitTypeApproval:
		matchKeys = []string{"approvalId", "requestId", "toolCallId"}
	case WaitTypeDependency:
		matchKeys = []string{"executionId", "workflowRunId", "taskRunId", "operationId", "invocationId"}
	case WaitTypeExternal:
		for key := range condition {
			if key != "kind" && key != "autoResume" && key != "resumeHint" {
				matchKeys = append(matchKeys, key)
			}
		}
		if len(matchKeys) == 0 {
			return false
		}
	case WaitTypeUser:
		return false
	}
	matched := false
	for _, key := range matchKeys {
		expected, ok := condition[key]
		if !ok || strings.TrimSpace(fmt.Sprint(expected)) == "" {
			continue
		}
		matched = true
		actual, ok := attributes[key]
		if !ok && key == "sourceId" {
			actual, ok = signal.SourceID, signal.SourceID != ""
		}
		if !ok || fmt.Sprint(actual) != fmt.Sprint(expected) {
			return false
		}
	}
	return matched
}
