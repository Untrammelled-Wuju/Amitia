// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) getConfig() (baseURL, apiKey, modelName string) {
	var dbURL, dbKey, dbModel string
	err := s.db.Table("embedding_configs").
		Select("base_url, api_key, model_name").
		Where("is_active = 1").Limit(1).Row().
		Scan(&dbURL, &dbKey, &dbModel)
	if err == nil && dbURL != "" {
		baseURL = dbURL
		apiKey = dbKey
		modelName = dbModel
		return
	}
	ec := config.AppCfg.Embedding
	modelName = ec.ModelName
	baseURL = ec.BaseUrl
	apiKey = ec.ApiKey
	return
}

func (s *Service) Embed(text string) ([]float32, error) {
	baseURL, apiKey, modelName := s.getConfig()
	if baseURL == "" || apiKey == "" {
		return fallbackEmbedding(text), nil
	}

	baseURL = strings.TrimRight(baseURL, "/")

	reqBody := map[string]interface{}{
		"model": modelName,
		"input": []map[string]interface{}{{"type": "text", "text": text}},
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/embeddings/multimodal", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn("嵌入请求失败，使用本地向量:", err)
		return fallbackEmbedding(text), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Warn(fmt.Sprintf("嵌入API错误(%d)，使用本地向量: %s", resp.StatusCode, truncateStr(string(body), 300)))
		return fallbackEmbedding(text), nil
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析嵌入响应失败: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("嵌入API未返回向量数据")
	}

	log.Info(fmt.Sprintf("嵌入生成成功 维度:%d", len(result.Data[0].Embedding)))
	return result.Data[0].Embedding, nil
}

func (s *Service) BatchEmbed(texts []string) ([][]float32, error) {
	baseURL, apiKey, modelName := s.getConfig()
	if baseURL == "" || apiKey == "" {
		return fallbackEmbeddings(texts), nil
	}

	baseURL = strings.TrimRight(baseURL, "/")

	inputs := make([]interface{}, len(texts))
	for i, t := range texts {
		inputs[i] = t
	}

	reqBody := map[string]interface{}{
		"model": modelName,
		"input": s.buildMultimodalInputs(texts),
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/embeddings/multimodal", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn("批量嵌入请求失败，使用本地向量:", err)
		return fallbackEmbeddings(texts), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Warn(fmt.Sprintf("嵌入API错误(%d)，使用本地向量: %s", resp.StatusCode, truncateStr(string(body), 300)))
		return fallbackEmbeddings(texts), nil
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析嵌入响应失败: %w", err)
	}

	vectors := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		vectors[i] = d.Embedding
	}
	log.Info(fmt.Sprintf("批量嵌入生成成功 数量:%d", len(vectors)))
	return vectors, nil
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func (s *Service) buildMultimodalInputs(texts []string) []map[string]interface{} {
	inputs := make([]map[string]interface{}, len(texts))
	for i, t := range texts {
		inputs[i] = map[string]interface{}{"type": "text", "text": t}
	}
	return inputs
}
func fallbackEmbeddings(texts []string) [][]float32 {
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vectors[i] = fallbackEmbedding(text)
	}
	return vectors
}

func fallbackEmbedding(text string) []float32 {
	dim := 1536
	if config.AppCfg != nil && config.AppCfg.Qdrant.VectorDim > 0 {
		dim = config.AppCfg.Qdrant.VectorDim
	}
	vector := make([]float32, dim)
	tokens := embeddingTokens(text)
	if len(tokens) == 0 {
		tokens = []string{strings.TrimSpace(text)}
	}
	for _, token := range tokens {
		if token == "" {
			continue
		}
		h := fnv.New64a()
		_, _ = h.Write([]byte(token))
		sum := h.Sum64()
		idx := int(sum % uint64(dim))
		sign := float32(1)
		if (sum>>63)&1 == 1 {
			sign = -1
		}
		vector[idx] += sign
		h.Reset()
		_, _ = h.Write([]byte("bi:" + token))
		sum = h.Sum64()
		vector[int(sum%uint64(dim))] += 0.5 * sign
	}
	var norm float64
	for _, v := range vector {
		norm += float64(v * v)
	}
	if norm == 0 {
		vector[0] = 1
		return vector
	}
	scale := float32(1 / math.Sqrt(norm))
	for i := range vector {
		vector[i] *= scale
	}
	return vector
}

func embeddingTokens(text string) []string {
	var tokens []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			tokens = append(tokens, strings.ToLower(string(current)))
			current = current[:0]
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current = append(current, unicode.ToLower(r))
			continue
		}
		flush()
		if !unicode.IsSpace(r) && !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			tokens = append(tokens, string(r))
		}
	}
	flush()
	return tokens
}
