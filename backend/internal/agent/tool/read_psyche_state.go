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
			Description: "读取角色当前的心理状态，包括情感（正向/负向情绪强度、唤醒度、支配感）、心境（PAD、紧张度）、压力水平和关键信念。用于在需要理解自身状态或向用户解释感受时调用。",
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
	Emotion   emotionBlock   `json:"emotion"`
	Mood      moodBlock      `json:"mood"`
	Stress    float64        `json:"stress"`
	Beliefs   []beliefBlock  `json:"beliefs,omitempty"`
	UpdatedAt string         `json:"updatedAt"`
}

type emotionBlock struct {
	Positive  float64 `json:"positive"`
	Negative  float64 `json:"negative"`
	Arousal   float64 `json:"arousal"`
	Dominance float64 `json:"dominance"`
}

type moodBlock struct {
	Valence float64 `json:"valence"`
	Tension float64 `json:"tension"`
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

	emotionRow := toolDB.QueryRow(
		"SELECT positive, negative, arousal, dominance, updated_at FROM affect_states WHERE character_id = ? ORDER BY updated_at DESC LIMIT 1",
		characterID,
	)
	var pos, neg, arou, dom float64
	var emoUpdated sql.NullString
	if err := emotionRow.Scan(&pos, &neg, &arou, &dom, &emoUpdated); err == nil {
		output.Emotion = emotionBlock{Positive: pos, Negative: neg, Arousal: arou, Dominance: dom}
		if emoUpdated.Valid {
			output.UpdatedAt = emoUpdated.String
		}
	}

	moodRow := toolDB.QueryRow(
		"SELECT valence, tension, updated_at FROM mood_states WHERE character_id = ? ORDER BY updated_at DESC LIMIT 1",
		characterID,
	)
	var moodVal, moodTen float64
	var moodUpdated sql.NullString
	if err := moodRow.Scan(&moodVal, &moodTen, &moodUpdated); err == nil {
		output.Mood = moodBlock{Valence: moodVal, Tension: moodTen}
		if moodUpdated.Valid && moodUpdated.String > output.UpdatedAt {
			output.UpdatedAt = moodUpdated.String
		}
	}

	stressRow := toolDB.QueryRow(
		"SELECT stress, updated_at FROM stress_states WHERE character_id = ? ORDER BY updated_at DESC LIMIT 1",
		characterID,
	)
	var stress float64
	var stressUpdated sql.NullString
	if err := stressRow.Scan(&stress, &stressUpdated); err == nil {
		output.Stress = stress
		if stressUpdated.Valid && stressUpdated.String > output.UpdatedAt {
			output.UpdatedAt = stressUpdated.String
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

	raw, _ := json.Marshal(output)

	var lines []string
	lines = append(lines, fmt.Sprintf("情感状态 — 正向: %.2f, 负向: %.2f, 唤醒度: %.2f, 支配感: %.2f",
		output.Emotion.Positive, output.Emotion.Negative, output.Emotion.Arousal, output.Emotion.Dominance))
	lines = append(lines, fmt.Sprintf("心境状态 — PAD效价: %.2f, 紧张度: %.2f", output.Mood.Valence, output.Mood.Tension))
	lines = append(lines, fmt.Sprintf("压力水平: %.2f", output.Stress))
	if len(output.Beliefs) > 0 {
		var beliefLines []string
		for _, b := range output.Beliefs {
			beliefLines = append(beliefLines, fmt.Sprintf("[%.0f%%] %s = %s", b.Confidence*100, b.Key, b.Value))
		}
		lines = append(lines, fmt.Sprintf("关键信念 (%d条):\n%s", len(output.Beliefs), strings.Join(beliefLines, "\n")))
	}

	result := TextResult(strings.Join(lines, "\n"))
	result.Audit = map[string]interface{}{
		"character_id":   characterID,
		"include_beliefs": includeBeliefs,
		"raw":            string(raw),
	}
	return result
}
