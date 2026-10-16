package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/reminder"
)

type ReminderCallerResolver func(context.Context, host_api.CallRequest) (reminder.Caller, string, string, error)

func RegisterReminderHostRoutes(gateway *host_api.DefaultGateway, service *reminder.Service, resolve ReminderCallerResolver) error {
	if gateway == nil || service == nil || resolve == nil {
		return fmt.Errorf("reminder Host API dependencies unavailable")
	}
	for _, kind := range []string{reminder.UserReminder, reminder.ProactiveTask} {
		prefix, permission := "host.reminder.", "reminder."
		if kind == reminder.ProactiveTask {
			prefix = "host.proactive_task."
			permission = "proactive_task."
		}
		for _, action := range []string{"create", "list", "get", "update", "pause", "resume", "cancel", "execute", "history", "capabilities", "status", "queue", "preview", "clear"} {
			operation, taskKind := action, kind
			grant := "manage"
			side := host_api.SideEffectWrite
			risk := host_api.RiskMedium
			switch operation {
			case "list", "get", "history", "capabilities", "status", "queue", "preview":
				grant = "read"
				side = host_api.SideEffectReadOnly
				risk = host_api.RiskLow
			case "create":
				grant = "create"
			case "execute":
				grant = "execute"
				side = host_api.SideEffectExternal
			}
			route := host_api.Route{Method: host_api.Method(prefix + action), Version: 1, Permission: []host_api.PermissionRequirement{{Name: permission + grant, Resource: "schedule"}}, ScopePolicy: host_api.ScopePolicy{Namespaced: true}, RiskLevel: risk, SideEffectLevel: side, Timeout: 30 * time.Second, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":true}`), OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":true}`)}
			route.Handler = func(ctx context.Context, call host_api.CallRequest) (host_api.CallResult, error) {
				caller, role, conversation, err := resolve(ctx, call)
				if err != nil {
					return reminderHostFailure(err), nil
				}
				caller.RoleScope, caller.ConversationScope = role, conversation
				var payload struct {
					reminder.CreateReminderRequest
					ID         string `json:"id"`
					Generation int64  `json:"generation"`
					Page       int    `json:"page"`
					PageSize   int    `json:"pageSize"`
					State      string `json:"state"`
				}
				if len(call.Input) > 0 && json.Unmarshal(call.Input, &payload) != nil {
					return reminderHostFailure(fmt.Errorf("任务参数无效")), nil
				}
				payload.Kind = taskKind
				if role != "" && payload.CharacterID != "" && role != payload.CharacterID {
					return reminderHostFailure(reminder.ErrForbidden), nil
				}
				if conversation != "" && payload.ConversationID != "" && conversation != payload.ConversationID {
					return reminderHostFailure(reminder.ErrForbidden), nil
				}
				if role != "" {
					payload.CharacterID = role
				}
				if conversation != "" {
					payload.ConversationID = conversation
				}
				if payload.ID != "" {
					item, err := service.Find(ctx, caller, payload.ID)
					if err != nil {
						return reminderHostFailure(err), nil
					}
					if item.Kind != taskKind || role != "" && item.CharacterID != role || conversation != "" && item.ConversationID != conversation {
						return reminderHostFailure(reminder.ErrForbidden), nil
					}
				}
				var output any
				switch operation {
				case "create":
					output, err = service.Create(ctx, caller, payload.CreateReminderRequest, "plugin")
				case "get":
					output, err = service.Find(ctx, caller, payload.ID)
				case "list":
					items, listErr := service.List(ctx, caller, taskKind)
					err = listErr
					filtered := []reminder.Reminder{}
					for _, item := range items {
						if (role == "" || item.CharacterID == role) && (conversation == "" || item.ConversationID == conversation) {
							filtered = append(filtered, item)
						}
					}
					output = map[string]any{"items": filtered, "total": len(filtered)}
				case "update":
					output, err = service.Update(ctx, caller, payload.ID, payload.Generation, payload.CreateReminderRequest)
				case "pause", "resume", "cancel":
					output, err = service.Change(ctx, caller, payload.ID, payload.Generation, operation)
				case "execute":
					output, err = service.TriggerNow(ctx, caller, payload.ID, payload.Generation)
				case "history":
					items, total, historyErr := service.History(ctx, caller, taskKind, payload.ID, payload.Page, payload.PageSize, payload.State)
					err = historyErr
					output = map[string]any{"items": items, "total": total}
				case "capabilities":
					output = map[string]any{"ready": service.Ready(), "schedulerRunning": service.Running(), "rules": []string{"none", "daily", "weekly", "monthly", "weekdays", "interval", "cron"}, "management": []string{"create", "update", "pause", "resume", "cancel", "execute", "history"}}
				case "status":
					output, err = service.Status(ctx, caller, taskKind)
				case "queue":
					output, err = service.QueueSummary(ctx, caller, taskKind)
				case "preview":
					output, err = service.Preview(ctx, caller, payload.ID)
				case "clear":
					output, err = service.ClearBackpressure(ctx, caller, taskKind)
				}
				if err != nil {
					return reminderHostFailure(err), nil
				}
				encoded, err := json.Marshal(output)
				if err != nil {
					return reminderHostFailure(err), nil
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: encoded}, nil
			}
			if err := gateway.RegisterRoute(route); err != nil {
				return err
			}
		}
	}
	return nil
}

func reminderHostFailure(err error) host_api.CallResult {
	code := host_api.ErrorCodeInputInvalid
	switch {
	case errors.Is(err, reminder.ErrForbidden), errors.Is(err, schedule.ErrScopeDenied):
		code = host_api.ErrorCodeScopeDenied
	case errors.Is(err, reminder.ErrUnavailable):
		code = host_api.ErrorCodeHostUnavailable
	case errors.Is(err, schedule.ErrGenerationMismatch), errors.Is(err, schedule.ErrIdempotencyConflict):
		code = host_api.ErrorCodeStateConflict
	case errors.Is(err, schedule.ErrScheduleNotFound):
		code = host_api.ErrorCodeResourceNotFound
	case errors.Is(err, context.Canceled):
		code = host_api.ErrorCodeCancelled
	case errors.Is(err, context.DeadlineExceeded):
		code = host_api.ErrorCodeTimeout
	}
	return host_api.CallResult{Status: host_api.StatusFailed, Error: &host_api.Error{Code: code, Message: err.Error()}}
}
