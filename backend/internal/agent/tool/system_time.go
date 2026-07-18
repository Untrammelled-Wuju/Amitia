// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/temporal"
)

var temporalResolver struct {
	sync.RWMutex
	service interface {
		ResolveSnapshot(context.Context, temporal.SnapshotInput) (temporal.Snapshot, error)
	}
}

func SetTemporalService(service interface {
	ResolveSnapshot(context.Context, temporal.SnapshotInput) (temporal.Snapshot, error)
}) {
	temporalResolver.Lock()
	temporalResolver.service = service
	temporalResolver.Unlock()
}

func init() {
	Register(Tool{
		Type: "function",
		Function: Function{
			Name:        "get_current_time",
			Description: "获取当前本地时间和UTC时间，无需参数",
			Parameters: Parameters{
				Type:       "object",
				Properties: map[string]Property{},
				Required:   []string{},
			},
		},
	}, func(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
		if err := callCtx.Err(); err != nil {
			return CancelledResult(err.Error())
		}
		temporalResolver.RLock()
		service := temporalResolver.service
		temporalResolver.RUnlock()
		if service == nil {
			now := time.Now()
			result := TextResult(fmt.Sprintf("系统参考时间: %s | UTC: %s | 用户时区未确认", now.Format("2006-01-02 15:04:05 MST"), now.UTC().Format("2006-01-02 15:04:05Z07:00")))
			result.Audit = map[string]interface{}{"clockSource": "system_fallback", "userTimezoneConfirmed": false}
			return result
		}
		snapshot, err := service.ResolveSnapshot(callCtx, temporal.SnapshotInput{UserID: execCtx.User, CharacterID: execCtx.CharacterID, Channel: execCtx.Channel})
		if err != nil {
			return ErrorResult("temporal_snapshot_failed", "时间上下文解析失败")
		}
		result := TextResult(fmt.Sprintf("用户当地时间: %s (%s, %s) | 角色当地时间: %s (%s, %s) | UTC: %s", snapshot.UserTime.LocalTime.Format("2006-01-02 15:04:05"), snapshot.UserTime.Timezone, snapshot.UserTime.Weekday, snapshot.CharacterTime.LocalTime.Format("2006-01-02 15:04:05"), snapshot.CharacterTime.Timezone, snapshot.CharacterTime.Weekday, snapshot.NowUTC.Format("2006-01-02 15:04:05Z07:00")))
		result.Audit = map[string]interface{}{"channel": execCtx.Channel, "snapshotVersion": snapshot.Version, "userTimezone": snapshot.UserTime.Timezone, "characterTimezone": snapshot.CharacterTime.Timezone}
		return result
	})
}
