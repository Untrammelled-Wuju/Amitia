package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type ScheduleCreator func(context.Context, ToolExecutionContext, map[string]interface{}) (string, error)

var scheduleCreator struct {
	sync.RWMutex
	create ScheduleCreator
}

func SetScheduleCreator(create ScheduleCreator) {
	scheduleCreator.Lock()
	defer scheduleCreator.Unlock()
	scheduleCreator.create = create
}

func init() {
	var definition Tool
	err := json.Unmarshal([]byte(`{"type":"function","function":{"name":"create_schedule","description":"仅在用户明确要求定闹钟、安排日程或提醒时创建任务。保存到统一提醒系统，到期在当前角色对话提醒；不要把普通聊天中的意向自动安排为提醒。","parameters":{"type":"object","required":["title","due_time","timezone"],"additionalProperties":false,"properties":{"title":{"type":"string","minLength":1,"maxLength":512},"description":{"type":"string","maxLength":131072},"due_time":{"type":"string","description":"未来时间，优先使用带时区偏移的 ISO8601 时间"},"timezone":{"type":"string","description":"IANA 时区，例如 Asia/Shanghai"},"repeat":{"type":"string","enum":["none","daily","weekly","monthly","weekdays","interval"]},"recurrence":{"type":"object","properties":{"type":{"type":"string","enum":["none","daily","weekly","monthly","weekdays","interval"]},"weekdays":{"type":"array","items":{"type":"integer","minimum":1,"maximum":7}},"intervalSeconds":{"type":"integer","minimum":60,"maximum":31536000}},"additionalProperties":false},"end_at":{"type":"string"},"max_occurrences":{"type":"integer","minimum":0},"channel":{"type":"string","description":"web 为角色对话；其他渠道必须同时提供明确接收对象"},"channel_targets":{"type":"array","items":{"type":"object","required":["channelId","peerId"],"properties":{"channelId":{"type":"string"},"peerId":{"type":"string"}},"additionalProperties":false}}}}}}`), &definition)
	if err != nil {
		panic(err)
	}
	Register(definition, createSchedule)
}

func createSchedule(ctx context.Context, execution ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := ctx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scoped, scopeErr := requireScopedWrite(execution)
	if scopeErr != nil {
		return *scopeErr
	}
	scheduleCreator.RLock()
	create := scheduleCreator.create
	scheduleCreator.RUnlock()
	if create == nil {
		return ErrorResult("schedule_unavailable", "统一提醒服务未接通")
	}
	id, err := create(ctx, scoped, args)
	if err != nil {
		return ErrorResult("schedule_create_failed", err.Error())
	}
	if id == "" {
		return ErrorResult("schedule_unconfirmed", "提醒保存未确认")
	}
	result := TextResult(fmt.Sprintf("OK 已创建提醒，任务编号：%s。可在日程提醒页面查看和管理。", id))
	result.ExternalOperationID = id
	result.SideEffects = []ToolSideEffect{{Type: "schedule_create", TargetID: id, Confirmed: true}}
	result.Audit = map[string]interface{}{"character_id": scoped.CharacterID, "conversation_id": scoped.ConversationID, "request_id": scoped.RequestID}
	return result
}
