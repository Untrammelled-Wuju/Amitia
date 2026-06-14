package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
	ec := config.AppCfg.Embedding
	modelName = ec.ModelName
	baseURL = ec.BaseUrl
	apiKey = ec.ApiKey

	if baseURL == "" || apiKey == "" {
		var dbURL, dbKey, dbModel string
		err := s.db.Table("model_configs").
			Select("base_url, api_key, model_name").
			Where("is_active = 1").Limit(1).Row().
			Scan(&dbURL, &dbKey, &dbModel)
		if err == nil {
			if baseURL == "" {
				baseURL = dbURL
			}
			if apiKey == "" {
				apiKey = dbKey
			}
		}
	}
	return
}

func (s *Service) Embed(text string) ([]float32, error) {
	baseURL, apiKey, modelName := s.getConfig()
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("未配置嵌入模型")
	}

	baseURL = strings.TrimRight(baseURL, "/")

	reqBody := map[string]interface{}{
		"model": modelName,
		"input": text,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("嵌入请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("嵌入API错误(%d): %s", resp.StatusCode, truncateStr(string(body), 300))
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
		return nil, fmt.Errorf("未配置嵌入模型")
	}

	baseURL = strings.TrimRight(baseURL, "/")

	inputs := make([]interface{}, len(texts))
	for i, t := range texts {
		inputs[i] = t
	}

	reqBody := map[string]interface{}{
		"model": modelName,
		"input": inputs,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("批量嵌入请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("嵌入API错误(%d): %s", resp.StatusCode, truncateStr(string(body), 300))
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