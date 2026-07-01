// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

func (s *service) ToolRoute(body map[string]interface{}) map[string]interface{} {
	tool, _ := body["tool"].(string)
	if tool == "" {
		tool = "unknown"
	}
	return map[string]interface{}{"routed": true, "tool": tool}
}
