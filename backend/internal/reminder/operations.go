package reminder

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	amitialog "log"
)

func (s *Service) Update(ctx context.Context, caller Caller, id string, generation int64, request CreateReminderRequest) (*Reminder, error) {
	old, err := s.Find(ctx, caller, id)
	if err != nil {
		return nil, err
	}
	if old.Kind != request.Kind && request.Kind != "" {
		return nil, ErrForbidden
	}
	request.Kind = old.Kind
	request.RequestID = old.RequestID
	item, def, err := s.build(ctx, Caller{SpaceID: caller.SpaceID, DeviceID: caller.DeviceID, PluginID: old.Caller.PluginID, ModuleID: old.Caller.ModuleID}, request, id, old.Source, true)
	if err != nil {
		return nil, err
	}
	item.CreatedAt = old.CreatedAt
	item.CreatedBy = old.CreatedBy
	item.ArchiveHistory = old.ArchiveHistory
	existing, _, err := s.deps.Scheduler.GetSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	var stored struct {
		CreateHash string `json:"createHash"`
	}
	if err := json.Unmarshal(existing.Target.InputTemplate, &stored); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	def.Target.InputTemplate, err = json.Marshal(struct {
		*Reminder
		CreateHash string `json:"createHash"`
	}{item, stored.CreateHash})
	if err != nil {
		return nil, err
	}
	if err := s.deps.Scheduler.UpdateHostDefinitionState(ctx, id, generation, def, request.Enabled); err != nil {
		return nil, err
	}
	return s.Find(ctx, caller, id)
}

func (s *Service) Change(ctx context.Context, caller Caller, id string, generation int64, action string) (*Reminder, error) {
	item, err := s.Find(ctx, caller, id)
	if err != nil {
		return nil, err
	}
	if action == "resume" || action == "enable" {
		if err := s.authorize(ctx, caller, item); err != nil {
			return nil, err
		}
	}
	switch action {
	case "pause":
		err = s.deps.Scheduler.Pause(ctx, id, generation)
	case "resume":
		err = s.deps.Scheduler.Resume(ctx, id, generation)
	case "enable":
		err = s.deps.Scheduler.Enable(ctx, id, generation)
	case "disable":
		err = s.deps.Scheduler.Disable(ctx, id, generation)
	case "cancel":
		err = s.deps.Scheduler.CancelHostDefinition(ctx, id, generation)
	default:
		return nil, errors.New("任务操作无效")
	}
	if err != nil {
		return nil, err
	}
	return s.Find(ctx, caller, id)
}

func (s *Service) TriggerNow(ctx context.Context, caller Caller, id string, generation int64) (*schedule.ScheduleTriggerRecord, error) {
	item, err := s.Find(ctx, caller, id)
	if err != nil {
		return nil, err
	}
	if item.Generation != generation {
		return nil, schedule.ErrGenerationMismatch
	}
	if err := s.authorize(ctx, caller, item); err != nil {
		return nil, err
	}
	return s.deps.Scheduler.RunNowVersion(ctx, id, generation)
}

func (s *Service) Preview(ctx context.Context, caller Caller, id string) (map[string]any, error) {
	item, err := s.Find(ctx, caller, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, caller, item); err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "tested": true, "generated": false, "sent": false, "title": item.Title, "remindAt": item.RemindAt, "messageContent": item.Content, "channel": item.Channel, "channels": item.Channels, "conversationId": item.ConversationID, "ownerId": item.OwnerID, "generation": item.Generation, "authorizationCheck": map[string]any{"allowed": true, "reason": "配置与执行权限验证通过；预览不会生成或发送消息"}}, nil
}

func (s *Service) historyFor(ctx context.Context, item *Reminder, limit int) ([]TriggerHistory, error) {
	triggers, err := s.deps.Scheduler.GetAllTriggers(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	history := []TriggerHistory{}
	runs, err := s.deps.Scheduler.GetAllRuns(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	results := map[string]json.RawMessage{}
	for _, run := range runs {
		if len(run.ResultJSON) > 0 && len(results[run.TriggerID]) == 0 {
			results[run.TriggerID] = run.ResultJSON
		}
	}
	for _, trigger := range triggers {
		entry := TriggerHistory{ID: trigger.TriggerID, TriggerID: trigger.TriggerID, ReminderID: item.ID, TriggerType: item.Kind, Title: item.Title, Channel: item.Channel, State: string(trigger.Status), Priority: "normal", Reason: trigger.MisfireDecision, AttemptCount: trigger.Attempt + 1, ScheduledAt: trigger.ScheduledAt, CreatedAt: trigger.CreatedAt, UpdatedAt: trigger.UpdatedAt}
		if trigger.Status == schedule.RunStatusWaiting {
			entry.AttemptCount = 0
		}
		if trigger.Status == schedule.RunStatusTriggering || trigger.Status == schedule.RunStatusRunning || trigger.Status == schedule.RunStatusLeased {
			entry.AttemptCount = trigger.Attempt
		}
		if trigger.ErrorMessage != nil {
			entry.LastError = *trigger.ErrorMessage
		}
		entry.Result = results[trigger.TriggerID]
		if len(entry.Result) > 0 {
			var result struct {
				DeliveryResult
				State string `json:"state"`
			}
			if json.Unmarshal(entry.Result, &result) == nil && result.MessageSaved {
				result.State = "completed"
				for index, target := range result.Targets {
					current := target
					if s.deps.ResolveTarget != nil {
						current, err = s.deps.ResolveTarget(ctx, target)
						if err != nil {
							return nil, err
						}
					}
					result.Targets[index] = current
					if !targetCompleted(current.State) {
						result.State = "partial"
					}
				}
				entry.Result, err = json.Marshal(result)
				if err != nil {
					return nil, err
				}
			}
		}
		if len(entry.Result) > 0 {
			var result map[string]any
			if json.Unmarshal(entry.Result, &result) == nil {
				if state, ok := result["state"].(string); ok {
					entry.State = state
				}
				if reason, ok := result["reason"].(string); ok {
					entry.Reason = reason
				}
			}
		}
		history = append(history, entry)
	}
	return mergeArchivedHistory(item, history), nil
}

func (s *Service) History(ctx context.Context, caller Caller, kind, id string, page, size int, state string) ([]TriggerHistory, int, error) {
	if err := s.cleanupHistory(ctx, caller); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	items, err := s.list(ctx, caller, kind, true)
	if err != nil {
		return nil, 0, err
	}
	if id != "" {
		item, err := s.Find(ctx, caller, id)
		if err != nil {
			return nil, 0, err
		}
		if kind != "" && item.Kind != kind {
			return nil, 0, ErrForbidden
		}
		items = []Reminder{*item}
	}
	all := []TriggerHistory{}
	cutoff, err := s.historyCutoff(ctx, caller)
	if err != nil {
		return nil, 0, err
	}
	for index := range items {
		entries, err := s.historyFor(ctx, &items[index], 10000)
		if err != nil {
			return nil, 0, err
		}
		for _, entry := range entries {
			if cutoff != nil && entry.UpdatedAt.Before(*cutoff) && (entry.State == "completed" || entry.State == "cancelled" || entry.State == "expired" || entry.State == "skipped") {
				continue
			}
			if state == "" || state == entry.State {
				all = append(all, entry)
			}
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID > all[j].ID
		}
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	total := len(all)
	start := total
	if page-1 <= total/size {
		start = (page - 1) * size
	}
	end := start + size
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (s *Service) Status(ctx context.Context, caller Caller, kind string) (map[string]any, error) {
	items, err := s.List(ctx, caller, kind)
	if err != nil {
		return nil, err
	}
	enabled, due := 0, 0
	for _, item := range items {
		if item.Enabled == 1 {
			enabled++
			next, _ := time.Parse(time.RFC3339Nano, item.RemindAt)
			if !next.IsZero() && !next.After(time.Now()) {
				due++
			}
		}
	}
	queue, err := s.QueueSummary(ctx, caller, kind)
	if err != nil {
		return nil, err
	}
	sentToday, failedToday := 0, 0
	historicalItems, err := s.list(ctx, caller, kind, true)
	if err != nil {
		return nil, err
	}
	for index := range historicalItems {
		entries, err := s.historyFor(ctx, &historicalItems[index], -1)
		if err != nil {
			return nil, err
		}
		loc, err := time.LoadLocation(historicalItems[index].Timezone)
		if err != nil {
			return nil, err
		}
		date := time.Now().In(loc).Format("2006-01-02")
		for _, entry := range entries {
			if entry.UpdatedAt.In(loc).Format("2006-01-02") != date {
				continue
			}
			var result DeliveryResult
			if len(entry.Result) > 0 && json.Unmarshal(entry.Result, &result) == nil && result.MessageSaved {
				sentToday++
			}
			if entry.State == "failed" {
				failedToday++
			}
		}
	}
	return map[string]any{"schedulerRunning": s.Running(), "total": len(items), "enabled": enabled, "enabledRuleCount": enabled, "totalRuleCount": len(items), "dueNow": due, "queue": queue, "sentToday": sentToday, "failedToday": failedToday, "ownerId": s.deps.OwnerID, "executionMode": s.deps.Mode}, nil
}

func (s *Service) PolicyHistory(ctx context.Context, caller Caller) ([]TriggerHistory, error) {
	items, err := s.list(ctx, caller, ProactiveTask, true)
	if err != nil {
		return nil, err
	}
	history := []TriggerHistory{}
	for index := range items {
		entries, err := s.historyFor(ctx, &items[index], -1)
		if err != nil {
			return nil, err
		}
		history = append(history, entries...)
	}
	return history, nil
}

func (s *Service) QueueSummary(ctx context.Context, caller Caller, kind string) (map[string]any, error) {
	items, err := s.List(ctx, caller, kind)
	if err != nil {
		return nil, err
	}
	pending, failed, blocked, openCircuits := 0, 0, 0, 0
	for index := range items {
		circuit, err := s.deps.Scheduler.GetCircuit(ctx, items[index].ID)
		if err != nil {
			return nil, err
		}
		if circuit != nil && circuit.State != schedule.CircuitStateClosed {
			openCircuits++
		}
		triggers, err := s.deps.Scheduler.GetAllTriggers(ctx, items[index].ID)
		if err != nil {
			return nil, err
		}
		for _, trigger := range triggers {
			if trigger.Status.IsActive() {
				pending++
			}
			if trigger.Status == schedule.RunStatusFailed && time.Since(trigger.UpdatedAt) <= 24*time.Hour {
				failed++
			}
			if trigger.Status == schedule.RunStatusBlocked || trigger.Status == schedule.RunStatusQuarantined {
				blocked++
			}
		}
	}
	return map[string]any{"depth": pending, "pendingCount": pending, "recentFailures": failed, "blockedCount": blocked, "openCircuits": openCircuits, "backpressure": openCircuits > 0}, nil
}

func (s *Service) ClearBackpressure(ctx context.Context, caller Caller, kind string) (map[string]any, error) {
	items, err := s.List(ctx, caller, kind)
	if err != nil {
		return nil, err
	}
	cleared := 0
	for _, item := range items {
		circuit, err := s.deps.Scheduler.GetCircuit(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		if circuit != nil && circuit.State != schedule.CircuitStateClosed {
			if err := s.deps.Scheduler.ResetCircuit(ctx, item.ID); err != nil {
				return nil, err
			}
			cleared++
		}
	}
	return map[string]any{"clearedCount": cleared, "message": "已重置熔断；权限阻断和结果未知的任务须检查原因后再执行"}, nil
}

func (s *Service) Execute(ctx context.Context, def *schedule.ScheduleContributionDefinition, trigger *schedule.ScheduleTriggerRecord) (*schedule.TargetExecutionResult, error) {
	item, err := decode(def)
	if err != nil {
		return nil, err
	}
	item.Generation = trigger.Generation
	if err := s.authorize(ctx, item.Caller, item); err != nil {
		return &schedule.TargetExecutionResult{ErrorCode: schedule.ErrCodeScopeDenied, ErrorMessage: err.Error()}, nil
	}
	if item.Kind == ProactiveTask {
		history, err := s.historyFor(ctx, item, 10000)
		if err != nil {
			return nil, err
		}
		if s.deps.CheckPolicy == nil {
			return &schedule.TargetExecutionResult{ErrorCode: schedule.ErrCodeScopeDenied, ErrorMessage: "主动任务策略未接通"}, nil
		}
		reason, err := s.deps.CheckPolicy(ctx, item, history)
		if err != nil {
			return nil, err
		}
		if reason != "" {
			result, _ := json.Marshal(map[string]any{"state": "skipped", "reason": reason})
			return &schedule.TargetExecutionResult{Success: true, ResultJSON: result}, nil
		}
	}
	result, err := s.deps.Deliver(ctx, item, trigger.TriggerID)
	if err != nil {
		return nil, err
	}
	if !result.MessageSaved || result.MessageID == "" || result.ConversationID != item.ConversationID {
		return nil, errors.New("提醒消息未确认保存")
	}
	state := "completed"
	for _, target := range result.Targets {
		if !targetCompleted(target.State) {
			state = "partial"
		}
	}
	encoded, _ := json.Marshal(struct {
		DeliveryResult
		State string `json:"state"`
	}{result, state})
	return &schedule.TargetExecutionResult{Success: true, OperationID: result.MessageID, ResultJSON: encoded}, nil
}

func targetCompleted(state string) bool {
	return state == "delivered" || state == "saved" || state == "displayed" || state == "clicked"
}
