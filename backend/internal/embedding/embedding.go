// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package embedding

import (
	"bytes"
	"database/sql"
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

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) getConfig() (baseURL, apiKey, modelName, apiType, providerConfigJSON string) {
	var dbURL, dbKey, dbModel, dbApiType sql.NullString
	var dbProviderConfigJSON sql.NullString
	err := s.db.Table("embedding_configs").
		Select("base_url, api_key, model_name, api_type, provider_config_json").
		Where("is_active = 1").Limit(1).Row().
		Scan(&dbURL, &dbKey, &dbModel, &dbApiType, &dbProviderConfigJSON)
	if err == nil && dbURL.Valid && dbURL.String != "" {
		baseURL = dbURL.String
		apiKey = dbKey.String
		modelName = dbModel.String
		apiType = dbApiType.String
		providerConfigJSON = dbProviderConfigJSON.String
		return
	}
	if err == nil && dbApiType.Valid && dbApiType.String == "llama_cpp" {
		apiType = dbApiType.String
		modelName = dbModel.String
		providerConfigJSON = dbProviderConfigJSON.String
		return
	}
	if config.AppCfg != nil {
		ec := config.AppCfg.Embedding
		modelName = ec.ModelName
		baseURL = ec.BaseUrl
		apiKey = ec.ApiKey
		apiType = "volcengine"
	}
	return
}

func (s *Service) Embed(text string) ([]float32, error) {
	vector, _, err := s.EmbedWithRawError(text)
	return vector, err
}

func (s *Service) EmbedWithRawError(text string) ([]float32, string, error) {
	baseURL, apiKey, modelName, apiType, providerConfigJSON := s.getConfig()
	if apiType == "llama_cpp" {
		return s.embedLocal(text, modelName, providerConfigJSON)
	}
	if baseURL == "" || apiKey == "" {
		return nil, "", fmt.Errorf("嵌入服务未配置: baseURL/apiKey 为空")
	}

	baseURL = strings.TrimRight(baseURL, "/")
	protocol := protocolForApiType(apiType)

	var reqBody map[string]interface{}
	var endpoint string

	if protocol == "volcengine" {
		reqBody = map[string]interface{}{
			"model": modelName,
			"input": []map[string]interface{}{{"type": "text", "text": text}},
		}
		endpoint = baseURL + "/embeddings/multimodal"
	} else {
		reqBody = map[string]interface{}{
			"model": modelName,
			"input": text,
		}
		endpoint = baseURL + "/embeddings"
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err.Error(), err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err.Error(), fmt.Errorf("嵌入请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		rawError := fmt.Sprintf("嵌入API返回 %d: %s", resp.StatusCode, string(body))
		return nil, rawError, fmt.Errorf("%s", rawError)
	}

	vectors, err := parseEmbeddingVectors(body)
	if err != nil {
		rawError := fmt.Sprintf("解析嵌入响应失败: %v; 原始响应: %s", err, string(body))
		return nil, rawError, fmt.Errorf("%s", rawError)
	}

	if len(vectors) == 0 {
		rawError := "嵌入API未返回向量数据，原始响应: " + string(body)
		return nil, rawError, fmt.Errorf("%s", rawError)
	}

	vector, err := fitEmbeddingDimension(vectors[0])
	if err != nil {
		return nil, err.Error(), err
	}
	log.Info(fmt.Sprintf("嵌入生成成功 维度:%d", len(vector)))
	return vector, "", nil
}

func (s *Service) BatchEmbed(texts []string) ([][]float32, error) {
	baseURL, apiKey, modelName, apiType, providerConfigJSON := s.getConfig()
	if apiType == "llama_cpp" {
		return s.batchEmbedLocal(texts, modelName, providerConfigJSON)
	}
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("嵌入服务未配置: baseURL/apiKey 为空")
	}

	baseURL = strings.TrimRight(baseURL, "/")
	protocol := protocolForApiType(apiType)

	var reqBody map[string]interface{}
	var endpoint string

	if protocol == "volcengine" {
		reqBody = map[string]interface{}{
			"model": modelName,
			"input": s.buildMultimodalInputs(texts),
		}
		endpoint = baseURL + "/embeddings/multimodal"
	} else {
		reqBody = map[string]interface{}{
			"model": modelName,
			"input": texts,
		}
		endpoint = baseURL + "/embeddings"
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonBody))
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
		return nil, fmt.Errorf("批量嵌入API返回 %d: %s", resp.StatusCode, truncateStr(string(body), 300))
	}

	vectors, err := parseEmbeddingVectors(body)
	if err != nil {
		return nil, fmt.Errorf("解析嵌入响应失败: %w", err)
	}
	for i := range vectors {
		fitted, err := fitEmbeddingDimension(vectors[i])
		if err != nil {
			return nil, fmt.Errorf("嵌入维度不匹配: provider=%d configured=%d", len(vectors[i]), config.AppCfg.Providers.VectorStore.Qdrant.VectorDim)
		}
		vectors[i] = fitted
	}

	log.Info(fmt.Sprintf("批量嵌入生成成功 数量:%d", len(vectors)))
	return vectors, nil
}

func parseEmbeddingVectors(body []byte) ([][]float32, error) {
	var response struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	data := bytes.TrimSpace(response.Data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, fmt.Errorf("data字段为空")
	}
	var items []embeddingData
	switch data[0] {
	case '[':
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, err
		}
	case '{':
		var item embeddingData
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, err
		}
		items = []embeddingData{item}
	default:
		return nil, fmt.Errorf("data字段格式不支持")
	}
	vectors := make([][]float32, 0, len(items))
	for _, item := range items {
		if len(item.Embedding) > 0 {
			vectors = append(vectors, item.Embedding)
		}
	}
	return vectors, nil
}

func fitEmbeddingDimension(vector []float32) ([]float32, error) {
	if config.AppCfg == nil || config.AppCfg.Providers.VectorStore.Qdrant.VectorDim <= 0 || len(vector) == config.AppCfg.Providers.VectorStore.Qdrant.VectorDim {
		return vector, nil
	}
	return nil, fmt.Errorf("嵌入维度不匹配: provider=%d configured=%d", len(vector), config.AppCfg.Providers.VectorStore.Qdrant.VectorDim)
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func protocolForApiType(apiType string) string {
	switch apiType {
	case "volcengine":
		return "volcengine"
	default:
		return "openai"
	}
}

func (s *Service) buildMultimodalInputs(texts []string) []map[string]interface{} {
	inputs := make([]map[string]interface{}, len(texts))
	for i, t := range texts {
		inputs[i] = map[string]interface{}{"type": "text", "text": t}
	}
	return inputs
}
type localEmbeddingProvider interface {
	EmbedSingle(text string, purpose string) ([]float32, error)
	EmbedBatch(texts []string, purpose string) ([][]float32, error)
}

var localEmbeddingProviders = make(map[string]func(configJSON string) (localEmbeddingProvider, error))

func registerLocalEmbeddingProvider(providerType string, factory func(configJSON string) (localEmbeddingProvider, error)) {
	localEmbeddingProviders[providerType] = factory
}

func (s *Service) embedLocal(text string, modelName string, configJSON string) ([]float32, string, error) {
	provider, err := getLocalEmbeddingProvider("llama_cpp", configJSON)
	if err != nil {
		return nil, err.Error(), err
	}
	vector, err := provider.EmbedSingle(text, "document")
	if err != nil {
		return nil, err.Error(), err
	}
	return vector, "", nil
}

func (s *Service) batchEmbedLocal(texts []string, modelName string, configJSON string) ([][]float32, error) {
	provider, err := getLocalEmbeddingProvider("llama_cpp", configJSON)
	if err != nil {
		return nil, err
	}
	vectors, err := provider.EmbedBatch(texts, "document")
	if err != nil {
		return nil, err
	}
	return vectors, nil
}

func getLocalEmbeddingProvider(providerType string, configJSON string) (localEmbeddingProvider, error) {
	factory, exists := localEmbeddingProviders[providerType]
	if !exists {
		return nil, fmt.Errorf("local embedding provider %s not registered", providerType)
	}
	return factory(configJSON)
}
