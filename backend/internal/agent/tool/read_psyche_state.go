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
			Name:        "read_psyche_state",
			Description: "读取角色当前的心理状态，包括情感（效价、唤醒度、支配感）、心境（心境效价、心境唤醒度）、压力水平和精力水平。用于在需要理解自身状态或向用户解释感受时调用。",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"character_id": {
						Type:        "string",
						Description: "角色ID，为空时使用当前角色",
					},
					"include_beliefs": {
						Type:        "boolean",
						Description: "是否同时返回关键信念摘要，默认 false",
					},
				},
				Required: []string{},
			},
		},
	}, readPsycheState)
}

type psycheStateOutput struct {
	Emotion   emotionBlock  `json:"emotion"`
	Mood      moodBlock     `json:"mood"`
	Stress    float64       `json:"stress"`
	Energy    float64       `json:"energy"`
	Beliefs   []beliefBlock `json:"beliefs,omitempty"`
	UpdatedAt string        `json:"updatedAt"`
}

type emotionBlock struct {
	Valence   float64 `json:"valence"`
	Arousal   float64 `json:"arousal"`
	Dominance float64 `json:"dominance"`
}

type moodBlock struct {
	MoodValence float64 `json:"moodValence"`
	MoodArousal float64 `json:"moodArousal"`
}

type beliefBlock struct {
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
}

func readPsycheState(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
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

	includeBeliefs, _ := args["include_beliefs"].(bool)

	if toolDB == nil {
		return ErrorResult("database_not_initialized", "ERROR: database not initialized")
	}

	output := psycheStateOutput{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	var emotionJSON, moodJSON sql.NullString
	var stress, energy float64
	var updatedAt sql.NullString

	psycheRow := toolDB.QueryRow(
		"SELECT emotion, mood, stress, energy, updated_at FROM psyche_states WHERE character_id = ? ORDER BY updated_at DESC LIMIT 1",
		characterID,
	)
	if err := psycheRow.Scan(&emotionJSON, &moodJSON, &stress, &energy, &updatedAt); err == nil {
		output.Stress = stress
		output.Energy = energy
		if updatedAt.Valid {
			output.UpdatedAt = updatedAt.String
		}

		if emotionJSON.Valid && emotionJSON.String != "" && emotionJSON.String != "{}" {
			var emo struct {
				Valence   float64 `json:"valence"`
				Arousal   float64 `json:"arousal"`
				Dominance float64 `json:"dominance"`
			}
			if json.Unmarshal([]byte(emotionJSON.String), &emo) == nil {
				output.Emotion = emotionBlock{Valence: emo.Valence, Arousal: emo.Arousal, Dominance: emo.Dominance}
			}
		}

		if moodJSON.Valid && moodJSON.String != "" && moodJSON.String != "{}" {
			var m struct {
				MoodValence float64 `json:"moodValence"`
				MoodArousal float64 `json:"moodArousal"`
			}
			if json.Unmarshal([]byte(moodJSON.String), &m) == nil {
				output.Mood = moodBlock{MoodValence: m.MoodValence, MoodArousal: m.MoodArousal}
			}
		}
	}

	if includeBeliefs {
		beliefRows, err := toolDB.Query(
			"SELECT key, value, confidence FROM resolved_beliefs WHERE character_id = ? ORDER BY confidence DESC LIMIT 20",
			characterID,
		)
		if err == nil {
			defer beliefRows.Close()
			for beliefRows.Next() {
				var bk, bv string
				var bc float64
				if err := beliefRows.Scan(&bk, &bv, &bc); err == nil {
					output.Beliefs = append(output.Beliefs, beliefBlock{Key: bk, Value: bv, Confidence: bc})
				}
			}
		}
	}

	raw, err := json.Marshal(output)
	if err != nil {
		return TextResult("序列化心灵状态失败: " + err.Error())
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("情感状态 — 效价: %.2f, 唤醒度: %.2f, 支配感: %.2f",
		output.Emotion.Valence, output.Emotion.Arousal, output.Emotion.Dominance))
	lines = append(lines, fmt.Sprintf("心境状态 — 心境效价: %.2f, 心境唤醒度: %.2f", output.Mood.MoodValence, output.Mood.MoodArousal))
	lines = append(lines, fmt.Sprintf("压力水平: %.2f", output.Stress))
	lines = append(lines, fmt.Sprintf("精力水平: %.2f", output.Energy))
	if len(output.Beliefs) > 0 {
		var beliefLines []string
		for _, b := range output.Beliefs {
			beliefLines = append(beliefLines, fmt.Sprintf("[%.0f%%] %s = %s", b.Confidence*100, b.Key, b.Value))
		}
		lines = append(lines, fmt.Sprintf("关键信念 (%d条):\n%s", len(output.Beliefs), strings.Join(beliefLines, "\n")))
	}

	result := TextResult(strings.Join(lines, "\n"))
	result.Audit = map[string]interface{}{
		"character_id":    characterID,
		"include_beliefs": includeBeliefs,
		"raw":             string(raw),
	}
	return result
}
