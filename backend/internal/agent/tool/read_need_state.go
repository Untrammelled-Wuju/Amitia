package tool

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func init() {
	RegisterMemory(Tool{
		Type: "function",
		Function: Function{
			Name:        "read_need_state",
			Description: "读取角色当前的内在需求状态，包括联系、休息、表达、自主、确定性、新鲜感等维度的当前值和基线。用于在需要理解自身驱动力或评估行为动机时调用。",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"character_id": {
						Type:        "string",
						Description: "角色ID，为空时使用当前角色",
					},
					"include_history": {
						Type:        "boolean",
						Description: "是否同时返回最近的变化记录，默认 false",
					},
				},
				Required: []string{},
			},
		},
	}, readNeedState)
}

type needStateOutput struct {
	Needs     []needBlock `json:"needs"`
	UpdatedAt string      `json:"updatedAt"`
}

type needBlock struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	CurrentValue float64 `json:"currentValue"`
	Baseline     float64 `json:"baseline"`
	Deviation    float64 `json:"deviation"`
}

func readNeedState(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}

	characterID, _ := args["character_id"].(string)
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		characterID = execCtx.CharacterID
	}
	if characterID == "" {
		return ErrorResult("missing_character_scope", "ERROR: character_id is required")
	}

	includeHistory, _ := args["include_history"].(bool)

	if toolDB == nil {
		return ErrorResult("database_not_initialized", "ERROR: database not initialized")
	}

	output := needStateOutput{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	rows, err := toolDB.Query(
		"SELECT need_key, current_value, baseline, updated_at FROM need_states WHERE character_id = ? ORDER BY need_key",
		characterID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			var cur, base float64
			var updated sql.NullString
			if err := rows.Scan(&key, &cur, &base, &updated); err != nil {
				continue
			}
			block := needBlock{
				Key:          key,
				Label:        needLabel(key),
				CurrentValue: cur,
				Baseline:     base,
				Deviation:    cur - base,
			}
			output.Needs = append(output.Needs, block)
			if updated.Valid && updated.String > output.UpdatedAt {
				output.UpdatedAt = updated.String
			}
		}
	}

	if len(output.Needs) == 0 {
		return TextResult("当前角色暂无需求状态数据")
	}

	raw, err := json.Marshal(output)
	if err != nil {
		return TextResult("序列化需求状态失败: " + err.Error())
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("内在需求状态 (%d项):", len(output.Needs)))
	for _, n := range output.Needs {
		arrow := "→"
		if n.Deviation > 0.1 {
			arrow = "↑"
		} else if n.Deviation < -0.1 {
			arrow = "↓"
		}
		lines = append(lines, fmt.Sprintf("  %s [%s] 当前: %.2f (基线: %.2f) %s", n.Label, n.Key, n.CurrentValue, n.Baseline, arrow))
	}
	if includeHistory {
		lines = append(lines, "（历史变化记录功能待扩展）")
	}

	result := TextResult(strings.Join(lines, "\n"))
	result.Audit = map[string]interface{}{
		"character_id":    characterID,
		"need_count":      len(output.Needs),
		"include_history": includeHistory,
		"raw":             string(raw),
	}
	return result
}

func needLabel(key string) string {
	labels := map[string]string{
		"companionship": "联系需求",
		"rest":          "休息需求",
		"expression":    "表达需求",
		"autonomy":      "自主需求",
		"certainty":     "确定性需求",
		"novelty":       "新鲜感需求",
		"achievement":   "成就需求",
		"belonging":     "归属需求",
	}
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}
