// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *service) generateLLMReply(prompt string) string {
	var baseURL, apiKey, modelName string
	err := s.db.Table("model_configs").Select("base_url, api_key, model_name").Where("is_active = 1").Limit(1).Row().Scan(&baseURL, &apiKey, &modelName)
	if err != nil || baseURL == "" || apiKey == "" {
		return ""
	}
	var charName, identity string
	s.db.Table("characters").Select("name, COALESCE(identity,'')").Where("is_active = 1").Limit(1).Row().Scan(&charName, &identity)
	if charName == "" {
		charName = "AI助手"
	}
	if identity == "" {
		identity = "一个AI伙伴"
	}
	now := time.Now()
	sys := fmt.Sprintf("你是%s，%s。\n当前时间：%s，周%s。\n你的语气自然、口语化。字数控制在8-40字。不要调用工具，直接输出纯文本。不要使用emoji。", charName, identity, now.Format("15:04"), now.Weekday().String())
	msgs := []map[string]interface{}{{"role": "system", "content": sys}, {"role": "user", "content": prompt}}
	reqBody, _ := json.Marshal(map[string]interface{}{"model": modelName, "messages": msgs, "temperature": 0.9, "max_tokens": 200, "stream": false})
	baseURL = strings.TrimRight(baseURL, "/")
	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	var r struct {
		Choices []struct{ Message struct{ Content string } }
	}
	json.Unmarshal(rb, &r)
	if len(r.Choices) > 0 {
		return strings.TrimSpace(r.Choices[0].Message.Content)
	}
	return ""
}
