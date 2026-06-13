package asr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func SubmitTask(apiKey string, audioURL string, language string) (string, error) {
	if apiKey == "" { return "", fmt.Errorf("API Key 未配置") }
	if audioURL == "" { return "", fmt.Errorf("音频URL不能为空") }
	reqBody := AsrSubmitReq{Audio: AsrAudio{URL: audioURL, Language: language}, User: AsrUser{UID: "u-ai-user"}}
	jsonBody, _ := json.Marshal(reqBody)
	taskID := uuid.New().String()
	req, _ := http.NewRequest("POST", asrSubmitUri, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("X-Api-Resource-Id", "volc.seedasr.auc")
	req.Header.Set("X-Api-Request-Id", taskID)
	req.Header.Set("X-Api-Sequence", "-1")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil { return "", fmt.Errorf("提交ASR任务失败: %w", err) }
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 { return "", fmt.Errorf("ASR提交返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300)) }
	var result AsrSubmitResp
	if err := json.Unmarshal(rawBody, &result); err != nil { return "", fmt.Errorf("解析响应失败: %w", err) }
	if result.Code != 3000 { return "", fmt.Errorf("ASR提交失败 [code:%d]: %s", result.Code, result.Message) }
	return taskID, nil
}

func QueryTask(apiKey string, taskID string) (*AsrQueryResp, error) {
	if apiKey == "" || taskID == "" { return nil, fmt.Errorf("参数不全") }
	req, _ := http.NewRequest("GET", asrQueryUri+"?task_id="+taskID, nil)
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("X-Api-Resource-Id", "volc.seedasr.auc")
	req.Header.Set("X-Api-Request-Id", taskID)
	req.Header.Set("X-Api-Sequence", "-1")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil { return nil, fmt.Errorf("查询ASR失败: %w", err) }
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 { return nil, fmt.Errorf("ASR查询返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300)) }
	var result AsrQueryResp
	if err := json.Unmarshal(rawBody, &result); err != nil { return nil, fmt.Errorf("解析响应失败: %w", err) }
	if result.Code != 3000 { return nil, fmt.Errorf("ASR查询失败 [code:%d]: %s", result.Code, result.Message) }
	return &result, nil
}

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n { return s }
	return string(runes[:n]) + "..."
}

func resolveApiKey(explicitKey string) string {
	if explicitKey != "" { return explicitKey }
	if asrService != nil {
		key, err := asrService.GetActiveApiKey()
		if err == nil { return key }
	}
	return ""
}

func RegisterAsrRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx.DB)
	asrService = NewService(repo)
	handler := NewHandler(asrService)

	asrGroup := r.Group("/asr")
	{
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
	if apiKey == "" { apiKey = c.Query("apiKey") }
	apiKey = resolveApiKey(apiKey)
	if apiKey == "" { util.ErrorResponse(c, response.InvalidParams, "请先在模型配置中设置语音识别API Key", nil); return }
	audioURL := c.PostForm("audioUrl")
	if audioURL == "" { util.ErrorResponse(c, response.InvalidParams, "缺少音频URL", nil); return }
	language := c.PostForm("language")
	taskID, err := SubmitTask(apiKey, audioURL, language)
	if err != nil { util.ErrorResponse(c, response.OperationFailed, err.Error(), nil); return }
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
		util.ErrorResponse(c, response.InternalError, "保存文件失败", nil)
		return
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "写入文件失败", nil)
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
	if apiKey == "" { apiKey = c.Query("apiKey") }
	apiKey = resolveApiKey(apiKey)
	if apiKey == "" { util.ErrorResponse(c, response.InvalidParams, "请先在模型配置中设置语音识别API Key", nil); return }
	taskID := c.Query("taskId")
	if taskID == "" { util.ErrorResponse(c, response.InvalidParams, "缺少taskId", nil); return }
	result, err := QueryTask(apiKey, taskID)
	if err != nil { util.ErrorResponse(c, response.OperationFailed, err.Error(), nil); return }
	util.SuccessResponse(c, map[string]interface{}{"status": result.Status, "result": result.Result})
}
