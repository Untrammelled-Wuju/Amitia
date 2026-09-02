// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package asr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

const asrSubmitUri = "https://openspeech.bytedance.com/api/v3/auc/bigmodel/submit"
const asrQueryUri = "https://openspeech.bytedance.com/api/v3/auc/bigmodel/query"

type AsrSubmitReq struct {
	Audio AsrAudio `json:"audio"`
	User  AsrUser  `json:"user,omitempty"`
}

type AsrAudio struct {
	URL      string `json:"url"`
	Language string `json:"language,omitempty"`
}

type AsrUser struct {
	UID string `json:"uid"`
}

type AsrSubmitResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	TaskID  string `json:"task_id"`
}

type AsrQueryResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
	Result  string `json:"result"`
}

var asrService Service
var syncResults sync.Map

func protocolForApiType(apiType string) string {
	switch apiType {
	case "openai":
		return "openai"
	case "azure":
		return "azure"
	case "aliyun":
		return "aliyun"
	default:
		return "volcengine"
	}
}

func SubmitTask(cfg *AsrConfig, audioURL string, language string) (string, error) {
	if cfg.ApiType == "" {
		cfg.ApiType = "volcengine"
	}
	if cfg.ApiKey == "" {
		return "", fmt.Errorf("API Key 未配置")
	}
	if audioURL == "" {
		return "", fmt.Errorf("音频URL不能为空")
	}

	switch protocolForApiType(cfg.ApiType) {
	case "openai":
		return submitOpenAI(cfg, audioURL, language)
	case "azure":
		return submitAzure(cfg, audioURL, language)
	case "aliyun":
		return submitAliyun(cfg, audioURL, language)
	default:
		return submitVolcengine(cfg, audioURL, language)
	}
}

func QueryTask(cfg *AsrConfig, taskID string) (*AsrQueryResp, error) {
	if cfg.ApiType == "" {
		cfg.ApiType = "volcengine"
	}
	if cfg.ApiKey == "" {
		return nil, fmt.Errorf("参数不全")
	}
	if taskID == "" {
		return nil, fmt.Errorf("taskId不能为空")
	}

	if strings.HasPrefix(taskID, "sync:") {
		val, ok := syncResults.Load(taskID)
		if !ok {
			return nil, fmt.Errorf("任务结果不存在")
		}
		text := val.(string)
		syncResults.Delete(taskID)
		return &AsrQueryResp{
			Code:    3000,
			Message: "success",
			Status:  "success",
			Result:  text,
		}, nil
	}

	switch protocolForApiType(cfg.ApiType) {
	case "aliyun":
		return queryAliyun(cfg, taskID)
	default:
		return queryVolcengine(cfg, taskID)
	}
}

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func resolveConfig(explicitKey string) *AsrConfig {
	if explicitKey != "" {
		return &AsrConfig{ApiKey: explicitKey, ApiType: "volcengine", ResourceId: "volc.seedasr.auc"}
	}
	if asrService != nil {
		cfg, err := asrService.GetActiveConfig()
		if err == nil && cfg != nil {
			return cfg
		}
	}
	return &AsrConfig{ApiKey: "", ApiType: "volcengine", ResourceId: "volc.seedasr.auc"}
}

func fetchAudioData(audioURL string) ([]byte, string, error) {
	if strings.HasPrefix(audioURL, "/api/asr/uploads/") {
		filename := filepath.Base(audioURL)
		filePath := filepath.Join("data", "asr_uploads", filename)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("读取本地音频失败: %w", err)
		}
		return data, filename, nil
	}

	// Segment/continuous voice runtimes can pass a private temporary WAV through
	// the file:// form. This path is deliberately constrained to files generated
	// by SegmentASRAdapter under os.TempDir(); never turn the public ASR submit
	// surface into a generic local-file reader.
	if strings.HasPrefix(audioURL, "file://") {
		localPath := filepath.Clean(strings.TrimPrefix(audioURL, "file://"))
		tempRoot := filepath.Clean(os.TempDir())
		base := filepath.Base(localPath)
		if filepath.Dir(localPath) != tempRoot || !strings.HasPrefix(base, "amitia_segment_") || !strings.EqualFold(filepath.Ext(base), ".wav") {
			return nil, "", fmt.Errorf("拒绝读取非 Segment ASR 临时音频")
		}
		info, err := os.Lstat(localPath)
		if err != nil {
			return nil, "", fmt.Errorf("检查 Segment ASR 临时音频失败: %w", err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, "", fmt.Errorf("拒绝读取非普通 Segment ASR 临时音频")
		}
		const maxSegmentBytes int64 = 32 << 20
		if info.Size() < 0 || info.Size() > maxSegmentBytes {
			return nil, "", fmt.Errorf("Segment ASR 临时音频超过大小限制")
		}
		data, err := os.ReadFile(localPath)
		if err != nil {
			return nil, "", fmt.Errorf("读取 Segment ASR 临时音频失败: %w", err)
		}
		return data, base, nil
	}

	resp, err := http.Get(audioURL)
	if err != nil {
		return nil, "", fmt.Errorf("下载音频失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取音频失败: %w", err)
	}
	filename := filepath.Base(audioURL)
	if filename == "" || filename == "/" {
		filename = "audio.mp3"
	}
	return data, filename, nil
}

func submitVolcengine(cfg *AsrConfig, audioURL string, language string) (string, error) {
	reqBody := AsrSubmitReq{Audio: AsrAudio{URL: audioURL, Language: language}, User: AsrUser{UID: "u-ai-user"}}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal ASR request: %w", err)
	}
	taskID := uuid.New().String()
	req, err := http.NewRequest("POST", asrSubmitUri, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create ASR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", cfg.ApiKey)
	resourceId := cfg.ResourceId
	if resourceId == "" {
		resourceId = "volc.seedasr.auc"
	}
	req.Header.Set("X-Api-Resource-Id", resourceId)
	req.Header.Set("X-Api-Request-Id", taskID)
	req.Header.Set("X-Api-Sequence", "-1")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("提交ASR任务失败: %w", err)
	}
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("ASR提交返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300))
	}
	var result AsrSubmitResp
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}
	if result.Code != 3000 {
		return "", fmt.Errorf("ASR提交失败 [code:%d]: %s", result.Code, result.Message)
	}
	return taskID, nil
}

func queryVolcengine(cfg *AsrConfig, taskID string) (*AsrQueryResp, error) {
	req, _ := http.NewRequest("GET", asrQueryUri+"?task_id="+taskID, nil)
	req.Header.Set("X-Api-Key", cfg.ApiKey)
	resourceId := cfg.ResourceId
	if resourceId == "" {
		resourceId = "volc.seedasr.auc"
	}
	req.Header.Set("X-Api-Resource-Id", resourceId)
	req.Header.Set("X-Api-Request-Id", taskID)
	req.Header.Set("X-Api-Sequence", "-1")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询ASR失败: %w", err)
	}
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ASR查询返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300))
	}
	var result AsrQueryResp
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if result.Code != 3000 {
		return nil, fmt.Errorf("ASR查询失败 [code:%d]: %s", result.Code, result.Message)
	}
	return &result, nil
}

func submitOpenAI(cfg *AsrConfig, audioURL string, language string) (string, error) {
	audioData, filename, err := fetchAudioData(audioURL)
	if err != nil {
		return "", err
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := cfg.ResourceId
	if model == "" {
		model = "whisper-1"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("创建表单失败: %w", err)
	}
	part.Write(audioData)

	writer.WriteField("model", model)
	if language != "" {
		writer.WriteField("language", language)
	}
	writer.Close()

	req, err := http.NewRequest("POST", baseURL+"/audio/transcriptions", body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI ASR 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI ASR 返回 %d: %s", resp.StatusCode, string(rawBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	taskID := "sync:" + uuid.New().String()
	syncResults.Store(taskID, result.Text)
	return taskID, nil
}

func buildAzureShortAudioURL(baseURL string, language string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("Azure Speech BaseURL 必须包含 region 或 resource hostname")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("Azure Speech BaseURL 无效")
	}
	if strings.EqualFold(parsed.Hostname(), "stt.speech.microsoft.com") {
		return "", fmt.Errorf("Azure Speech BaseURL 必须包含 region，例如 https://eastus.stt.speech.microsoft.com")
	}

	path := strings.TrimRight(parsed.Path, "/")
	const shortAudioPath = "/speech/recognition/conversation/cognitiveservices"
	switch {
	case path == "":
		path = shortAudioPath + "/v1"
	case strings.HasSuffix(path, shortAudioPath):
		path += "/v1"
	case strings.HasSuffix(path, shortAudioPath+"/v1"):
	default:
		path += shortAudioPath + "/v1"
	}
	parsed.Path = path

	lang := strings.TrimSpace(language)
	if lang == "" {
		lang = "zh-CN"
	}
	query := parsed.Query()
	query.Set("language", lang)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func submitAzure(cfg *AsrConfig, audioURL string, language string) (string, error) {
	audioData, _, err := fetchAudioData(audioURL)
	if err != nil {
		return "", err
	}

	endpoint, err := buildAzureShortAudioURL(cfg.BaseURL, language)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(audioData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	// Azure resource keys use Ocp-Apim-Subscription-Key. Authorization: Bearer
	// is reserved for an STS access token and cannot be populated with the raw key.
	req.Header.Set("Ocp-Apim-Subscription-Key", cfg.ApiKey)
	req.Header.Set("Content-Type", "audio/wav; codecs=audio/pcm; samplerate=16000")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Azure ASR 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Azure ASR 返回 %d: %s", resp.StatusCode, string(rawBody))
	}

	var result struct {
		DisplayText string `json:"DisplayText"`
		Text        string `json:"text"`
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	text := result.DisplayText
	if text == "" {
		text = result.Text
	}

	taskID := "sync:" + uuid.New().String()
	syncResults.Store(taskID, text)
	return taskID, nil
}

func submitAliyun(cfg *AsrConfig, audioURL string, language string) (string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://nls-gateway.cn-shanghai.aliyuncs.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	reqBody := map[string]interface{}{
		"audio_url": audioURL,
	}
	if language != "" {
		reqBody["language"] = language
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal ASR request: %w", err)
	}

	url := fmt.Sprintf("%s/rest/v1/auc/submit", baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	req.Header.Set("X-NLS-Token", cfg.ApiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("阿里云ASR 提交失败: %w", err)
	}
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("阿里云ASR 提交返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300))
	}

	var result struct {
		TaskID string `json:"task_id"`
		Status int    `json:"status"`
		ErrMsg string `json:"err_msg"`
	}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}
	if result.TaskID == "" {
		return "", fmt.Errorf("阿里云ASR未返回任务ID")
	}
	return result.TaskID, nil
}

func queryAliyun(cfg *AsrConfig, taskID string) (*AsrQueryResp, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://nls-gateway.cn-shanghai.aliyuncs.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	url := fmt.Sprintf("%s/rest/v1/auc/query?task_id=%s", baseURL, taskID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	req.Header.Set("X-NLS-Token", cfg.ApiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询阿里云ASR失败: %w", err)
	}
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("阿里云ASR查询返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300))
	}

	var raw struct {
		Status int    `json:"status"`
		Result string `json:"result"`
		ErrMsg string `json:"err_msg"`
	}
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	statusStr := "processing"
	if raw.Status == 1 || raw.Status == 2 {
		statusStr = "success"
	}

	return &AsrQueryResp{
		Code:    3000,
		Message: "success",
		Status:  statusStr,
		Result:  raw.Result,
	}, nil
}

func RegisterAsrRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx.DB)
	asrService = NewService(repo)
	handler := NewHandler(asrService)

	asrGroup := r.Group("/asr")
	{
		asrGroup.GET("/providers", handler.ListProviders)
		asrGroup.POST("/upload", handleUpload)
		asrGroup.GET("/uploads/:file", handleServeUpload)
		asrGroup.POST("/submit", handleSubmit)
		asrGroup.GET("/query", handleQuery)

		asrGroup.GET("/configs", handler.List)
		asrGroup.GET("/configs/:id", handler.Get)
		asrGroup.POST("/configs", handler.Create)
		asrGroup.PUT("/configs/:id", handler.Update)
		asrGroup.DELETE("/configs/:id", handler.Delete)
		asrGroup.POST("/configs/:id/activate", handler.Activate)
		asrGroup.POST("/configs/:id/test", handler.Test)
	}
}

func handleSubmit(c *gin.Context) {
	apiKey := c.GetHeader("X-Tts-Api-Key")
	if apiKey == "" {
		apiKey = c.Query("apiKey")
	}
	cfg := resolveConfig(apiKey)
	if cfg.ApiKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "请先在模型配置中设置语音识别API Key", nil)
		return
	}
	audioURL := c.PostForm("audioUrl")
	if audioURL == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少音频URL", nil)
		return
	}
	language := c.PostForm("language")
	taskID, err := SubmitTask(cfg, audioURL, language)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "ASR任务已提交", map[string]string{"taskId": taskID})
}

func handleUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请上传音频文件", nil)
		return
	}
	defer file.Close()
	uploadDir := filepath.Join("data", "asr_uploads")
	os.MkdirAll(uploadDir, 0755)
	savePath := filepath.Join(uploadDir, header.Filename)
	out, err := os.Create(savePath)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, map[string]string{
		"filename": header.Filename,
		"url":      "/api/asr/uploads/" + header.Filename,
	})
}

func handleServeUpload(c *gin.Context) {
	filename := c.Param("file")
	filePath := filepath.Join("data", "asr_uploads", filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		util.ErrorResponse(c, response.NotFound, "文件不存在", nil)
		return
	}
	c.File(filePath)
}

func handleQuery(c *gin.Context) {
	apiKey := c.GetHeader("X-Tts-Api-Key")
	if apiKey == "" {
		apiKey = c.Query("apiKey")
	}
	cfg := resolveConfig(apiKey)
	if cfg.ApiKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "请先在模型配置中设置语音识别API Key", nil)
		return
	}
	taskID := c.Query("taskId")
	if taskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少taskId", nil)
		return
	}
	result, err := QueryTask(cfg, taskID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"status": result.Status, "result": result.Result})
}
