// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

import (
	"fmt"
	"time"
)

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
	}, func(args map[string]interface{}) string {
		local := time.Now().Format("2006-01-02 15:04:05")
		utc := time.Now().UTC().Format("2006-01-02 15:04:05")
		weekday := time.Now().Weekday().String()
		return fmt.Sprintf("本地时间: %s (周%s) | UTC: %s", local, weekday, utc)
	})
}
