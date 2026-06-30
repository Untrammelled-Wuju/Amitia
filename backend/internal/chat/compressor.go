// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ConversationSummary struct {
	ID              string `gorm:"column:id;primaryKey" json:"id"`
	ConversationID  string `gorm:"column:conversation_id;not null" json:"conversationId"`
	RoundStart      int    `gorm:"column:round_start;not null" json:"roundStart"`
	RoundEnd        int    `gorm:"column:round_end;not null" json:"roundEnd"`
	SummaryText     string `gorm:"column:summary_text;not null" json:"summaryText"`
	ParentSummaryID string `gorm:"column:parent_summary_id" json:"parentSummaryId"`
	CompressedAt    string `gorm:"column:compressed_at" json:"compressedAt"`
}

func (ConversationSummary) TableName() string { return "conversation_summaries" }

type Compressor struct {
	db *gorm.DB
}

func NewCompressor(db *gorm.DB) *Compressor {
	return &Compressor{db: db}
}

func (c *Compressor) MaybeCompress(convID string) {
	var totalMsgCount int64
	c.db.Table("messages").Where("conversation_id = ? AND role IN ('user','assistant') AND include_in_context = 1", convID).Count(&totalMsgCount)

	totalRounds := int(totalMsgCount / 2)

	var lastSummary ConversationSummary
	err := c.db.Where("conversation_id = ?", convID).Order("round_end DESC").First(&lastSummary).Error

	var nextRoundStart int
	if err == nil {
		nextRoundStart = lastSummary.RoundEnd + 1
	} else {
		nextRoundStart = 1
	}

	roundsSinceLast := totalRounds - nextRoundStart + 1
	if roundsSinceLast < 16 {
		return
	}

	var messages []Message
	c.db.Where("conversation_id = ? AND role IN ('user','assistant') AND include_in_context = 1", convID).
		Order("created_at ASC").Limit(8).Find(&messages)

	if len(messages) < 4 {
		return
	}

	var convText strings.Builder
	var msgIDs []string
	count := 0
	for _, m := range messages {
		if count >= 8 {
			break
		}
		convText.WriteString(m.Role + ": " + m.Content + "\n")
		msgIDs = append(msgIDs, m.ID)
		count++
	}

	var parentSummaryText string
	var parentSummaryID string
	if err == nil && lastSummary.SummaryText != "" {
		parentSummaryText = lastSummary.SummaryText
		parentSummaryID = lastSummary.ID
	}

	summary := c.generateSummary(convText.String(), parentSummaryText)
	if summary == "" {
		return
	}

	roundEnd := nextRoundStart + (count / 2) - 1
	cs := &ConversationSummary{
		ID:              uuid.New().String(),
		ConversationID:  convID,
		RoundStart:      nextRoundStart,
		RoundEnd:        roundEnd,
		SummaryText:     summary,
		ParentSummaryID: parentSummaryID,
		CompressedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}
	c.db.Create(cs)

	c.db.Model(&Message{}).Where("id IN ?", msgIDs).Update("include_in_context", 0)
}

func (c *Compressor) generateSummary(conversationText, parentSummary string) string {
	var baseURL, apiKey, modelName string
	var temperature, maxTokens float64
	err := c.db.Table("model_configs").
		Select("base_url, api_key, model_name, temperature, max_tokens").
		Where("is_active = 1").Limit(1).Row().
		Scan(&baseURL, &apiKey, &modelName, &temperature, &maxTokens)
	if err != nil {
		return ""
	}

	systemPrompt := "你是一个对话压缩器。将对话内容压缩为结构化摘要，包含：关键决策、用户偏好、待办事项。用中文输出，不超过300字。"

	userPrompt := conversationText
	if parentSummary != "" {
		userPrompt = "前次摘要：\n" + parentSummary + "\n\n新对话：\n" + conversationText
		userPrompt += "\n请将前次摘要和新对话合并为一个新的结构化摘要。"
	}

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}

	baseURL = strings.TrimRight(baseURL, "/")
	reqBody := map[string]interface{}{
		"model":       modelName,
		"messages":    messages,
		"temperature": temperature,
		"max_tokens":  int(maxTokens),
		"stream":      false,
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return ""
	}

	var result struct {
		Choices []struct{ Message struct{ Content string } }
	}
	json.Unmarshal(rb, &result)
	if len(result.Choices) == 0 {
		return ""
	}
	return strings.TrimSpace(result.Choices[0].Message.Content)
}

func (c *Compressor) GetCompressionStatus(convID string) map[string]interface{} {
	var totalRounds int64
	c.db.Table("messages").Where("conversation_id = ? AND role IN ('user','assistant')", convID).Count(&totalRounds)

	var summaries []ConversationSummary
	c.db.Where("conversation_id = ?", convID).Order("round_end DESC").Find(&summaries)

	var compressedRounds int
	for _, s := range summaries {
		compressedRounds += s.RoundEnd - s.RoundStart + 1
	}

	var lastCompressedAt string
	if len(summaries) > 0 {
		lastCompressedAt = summaries[0].CompressedAt
	}

	var latestSummary string
	if len(summaries) > 0 {
		latestSummary = summaries[0].SummaryText
	}

	return map[string]interface{}{
		"totalRounds":      totalRounds,
		"compressedRounds": compressedRounds,
		"lastCompressedAt": lastCompressedAt,
		"latestSummary":    latestSummary,
		"summaryCount":     len(summaries),
	}
}
