// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/modelerror"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type Handler struct {
	service Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) List(c *gin.Context) {
	configs, err := h.service.List()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, configs)
}

func (h *Handler) ListSummaries(c *gin.Context) {
	configs, err := h.service.List()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	summaries := make([]TtsConfigSummary, 0, len(configs))
	for _, cfg := range configs {
		summaries = append(summaries, TtsConfigSummary{
			ID:         cfg.ID,
			Name:       cfg.Name,
			ApiType:    cfg.ApiType,
			ResourceId: cfg.ResourceId,
			VoiceType:  cfg.VoiceType,
			IsActive:   cfg.IsActive,
			IsCustom:   cfg.IsCustom,
			HasApiKey:  cfg.HasApiKey,
		})
	}
	util.SuccessResponse(c, summaries)
}

func (h *Handler) Get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	cfg, err := h.service.GetByID(id)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, cfg)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateTtsConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	cfg, err := h.service.Create(&req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "音色配置已创建", cfg)
}

func (h *Handler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	cfg, err := h.service.Update(id, updates)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "音色配置已更新", cfg)
}

func (h *Handler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.Delete(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "音色配置已删除", nil)
}

func (h *Handler) Activate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	cfg, err := h.service.Activate(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "已设为默认音色", cfg)
}

func (h *Handler) GetVoices(c *gin.Context) {
	voices := h.service.GetAvailableVoices()
	util.SuccessResponse(c, voices)
}

func (h *Handler) GetEmotions(c *gin.Context) {
	emotions := h.service.GetEmotions()
	util.SuccessResponse(c, emotions)
}

func (h *Handler) ListProviders(c *gin.Context) {
	providers := h.service.ListProviders()
	util.SuccessResponse(c, providers)
}

func (h *Handler) Test(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.Test(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	h.service.Update(id, map[string]interface{}{})
	util.SuccessMsgResponse(c, "连接测试成功", nil)
}

func (h *Handler) TestConnectionStandalone(c *gin.Context) {
	var body struct {
		ApiKey     string `json:"apiKey"`
		ApiType    string `json:"apiType"`
		BaseURL    string `json:"baseUrl"`
		ResourceId string `json:"resource"`
		VoiceType  string `json:"voiceType"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	cfg := &TtsConfig{
		ApiKey:     body.ApiKey,
		ApiType:    body.ApiType,
		BaseURL:    body.BaseURL,
		ResourceId: body.ResourceId,
		VoiceType:  body.VoiceType,
	}
	if err := TestConnection(cfg); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "语音服务测试成功", nil)
}
func requestSpaceID(c *gin.Context) string {
	return requestidentity.NormalizeSpaceID(requestidentity.ResolveGin(c))
}

func (h *Handler) Preview(c *gin.Context) {
	var req SynthesizePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	result, err := h.service.SynthesizePreview(&req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) Synthesize(c *gin.Context) {
	var req SynthesizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	var result *SynthesizeResponse
	var err error
	if req.CharacterID != "" {
		result, err = h.service.SynthesizeForCharacter(requestSpaceID(c), req.CharacterID, req.Text)
	} else if req.VoiceID > 0 {
		result, err = h.service.Synthesize(req.VoiceID, req.Text)
	} else if req.SpeakerID != "" {
		result, err = h.service.SynthesizeWithSpeaker(requestSpaceID(c), req.SpeakerID, req.Text)
	} else {
		result, err = h.service.SynthesizeWithActive(req.Text)
	}
	if err != nil {
		modelerror.Report(modelerror.Event{ModelType: "voice", ConversationID: req.ConversationID, RequestID: req.RequestID, Channel: "web", RawError: err.Error()})
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ListClonedVoices(c *gin.Context) {
	voices, err := h.service.ListClonedVoices(requestSpaceID(c))
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询复刻音色失败", nil)
		return
	}
	util.SuccessResponse(c, voices)
}

func cloneAudioFormat(filename string) (string, error) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(filename))), ".")
	switch ext {
	case "mp3", "wav", "m4a", "aac", "ogg", "pcm", "webm":
		return ext, nil
	default:
		return "", fmt.Errorf("不支持的音频格式，请上传 mp3/wav/m4a/aac/ogg/pcm 音频")
	}
}

func isProviderSpeakerID(candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || len(candidate) > 64 {
		return false
	}
	for _, r := range candidate {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func safeProviderSpeakerID(candidate string) string {
	if isProviderSpeakerID(candidate) {
		return strings.TrimSpace(candidate)
	}
	return "uai_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func cloneProviderCredentials(cfg *TtsConfig) (apiKey, appKey, accessKey string, err error) {
	if cfg == nil {
		return "", "", "", fmt.Errorf("没有可用的 TTS 配置")
	}
	apiType := strings.ToLower(strings.TrimSpace(cfg.ApiType))
	if apiType != "" && apiType != "volcengine" {
		return "", "", "", fmt.Errorf("声音复刻需要使用火山引擎 TTS 配置")
	}
	apiKey = strings.TrimSpace(cfg.ApiKey)
	appKey = strings.TrimSpace(cfg.RealtimeAppId)
	accessKey = strings.TrimSpace(cfg.RealtimeAccessToken)
	if apiKey == "" && (appKey == "" || accessKey == "") {
		return "", "", "", fmt.Errorf("TTS 配置缺少声音复刻凭据")
	}
	return apiKey, appKey, accessKey, nil
}

func (h *Handler) CloneVoice(c *gin.Context) {
	// Voice cloning currently uses Volcengine clone APIs. Provider credentials
	// stay server-side; clients only submit clone metadata and audio.
	displayName := strings.TrimSpace(c.PostForm("name"))
	if displayName == "" {
		util.ErrorResponse(c, response.InvalidParams, "请填写音色名称", nil)
		return
	}
	requestedSpeakerID := strings.TrimSpace(c.PostForm("speakerId"))
	requestedConfigID := strings.TrimSpace(c.PostForm("voiceConfigId"))
	spaceID := requestSpaceID(c)

	var cfg *TtsConfig
	if requestedSpeakerID != "" {
		if _, ownerErr := h.service.GetClonedVoice(spaceID, requestedSpeakerID); ownerErr == nil {
			boundCfg, _, resolveErr := h.service.ResolveClonedVoiceProviderConfig(spaceID, requestedSpeakerID)
			if resolveErr != nil {
				util.ErrorResponse(c, response.OperationFailed, resolveErr.Error(), nil)
				return
			}
			cfg = boundCfg
		} else if errors.Is(ownerErr, gorm.ErrRecordNotFound) {
			if existing, globalErr := h.service.GetClonedVoiceBySpeakerID(requestedSpeakerID); globalErr == nil && existing != nil {
				util.ErrorResponse(c, response.OperationFailed, "该 speakerId 已属于其他用户", nil)
				return
			} else if globalErr != nil && !errors.Is(globalErr, gorm.ErrRecordNotFound) {
				util.ErrorResponse(c, response.InternalError, "检查 speakerId 所有权失败", nil)
				return
			}
		} else {
			util.ErrorResponse(c, response.InternalError, "检查 speakerId 所有权失败", nil)
			return
		}
	}
	if cfg == nil && requestedConfigID != "" {
		id, parseErr := strconv.Atoi(requestedConfigID)
		if parseErr != nil || id <= 0 {
			util.ErrorResponse(c, response.InvalidParams, "voiceConfigId 无效", nil)
			return
		}
		selected, getErr := h.service.GetProviderConfigByID(id)
		if getErr != nil || selected == nil {
			util.ErrorResponse(c, response.InvalidParams, "指定的 TTS 配置不存在", nil)
			return
		}
		cfg = selected
	}
	if cfg == nil {
		var activeErr error
		cfg, activeErr = h.service.GetActive()
		if activeErr != nil || cfg == nil {
			util.ErrorResponse(c, response.InvalidParams, "请先配置并启用 TTS 服务", nil)
			return
		}
	}
	apiKey, appKey, accessKey, credentialErr := cloneProviderCredentials(cfg)
	if credentialErr != nil {
		util.ErrorResponse(c, response.InvalidParams, credentialErr.Error(), nil)
		return
	}

	langStr := c.PostForm("language")
	language := 0
	if langStr == "en" {
		language = 1
	} else if langStr == "ja" {
		language = 2
	}
	refText := strings.TrimSpace(c.PostForm("refText"))

	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请上传音频文件", nil)
		return
	}
	defer file.Close()

	audioData, err := io.ReadAll(file)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	audioFormat, err := cloneAudioFormat(header.Filename)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}

	providerSpeakerID := requestedSpeakerID
	if providerSpeakerID == "" {
		// V1 MegaTTS cloning addresses a purchased provider slot and therefore
		// requires an explicit speaker ID. Older clients put that slot ID in the
		// name field, so keep accepting an identifier-shaped name for compatibility.
		if accessKey != "" && appKey != "" {
			if !isProviderSpeakerID(displayName) {
				util.ErrorResponse(c, response.InvalidParams, "当前声音复刻配置需要填写服务商 speakerId/槽位 ID", nil)
				return
			}
			providerSpeakerID = strings.TrimSpace(displayName)
		} else {
			// V3 custom-speaker cloning can use an opaque client-generated ID. Human
			// display names are never sent directly as provider identifiers.
			providerSpeakerID = safeProviderSpeakerID(displayName)
		}
	}

	var result *VoiceCloneResponse
	var cloneErr error
	if accessKey != "" && appKey != "" {
		result, cloneErr = CloneVoiceV1(accessKey, appKey, providerSpeakerID, audioData, audioFormat, language, 5)
	} else {
		result, cloneErr = CloneVoice(apiKey, appKey, accessKey, audioData, audioFormat, providerSpeakerID, language, refText)
	}
	if cloneErr != nil {
		util.ErrorResponse(c, response.OperationFailed, cloneErr.Error(), nil)
		return
	}
	if result == nil || strings.TrimSpace(result.SpeakerID) == "" {
		util.ErrorResponse(c, response.OperationFailed, "服务商未返回有效 speakerId", nil)
		return
	}

	result.SpeakerID = strings.TrimSpace(result.SpeakerID)
	resultWasOwned := false
	if _, ownerErr := h.service.GetClonedVoice(spaceID, result.SpeakerID); ownerErr == nil {
		resultWasOwned = true
	} else if errors.Is(ownerErr, gorm.ErrRecordNotFound) {
		if existing, globalErr := h.service.GetClonedVoiceBySpeakerID(result.SpeakerID); globalErr == nil && existing != nil {
			util.ErrorResponse(c, response.OperationFailed, "服务商返回的 speakerId 已属于其他用户", nil)
			return
		} else if globalErr != nil && !errors.Is(globalErr, gorm.ErrRecordNotFound) {
			util.ErrorResponse(c, response.InternalError, "检查复刻音色所有权失败", nil)
			return
		}
	} else {
		util.ErrorResponse(c, response.InternalError, "检查复刻音色所有权失败", nil)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	voice := &ClonedVoice{
		SpaceID:     spaceID,
		SpeakerID:   result.SpeakerID,
		Name:        displayName,
		TtsConfigID: cfg.ID,
		Language:    language,
		Status:      "ready",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.service.SaveClonedVoice(voice); err != nil {
		// Only roll back a newly created provider voice. Retraining an already
		// owned slot must never delete the previously valid provider resource.
		if !resultWasOwned {
			_ = DeleteClonedVoice(apiKey, appKey, accessKey, voice.SpeakerID)
		}
		util.ErrorResponse(c, response.InternalError, "保存复刻音色元数据失败", nil)
		return
	}

	util.SuccessMsgResponse(c, "音色复刻已提交", voice)
}

func (h *Handler) DeleteClonedVoice(c *gin.Context) {
	speakerID := strings.TrimSpace(c.Query("speakerId"))
	if speakerID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少 speakerId", nil)
		return
	}
	spaceID := requestSpaceID(c)
	if _, err := h.service.GetClonedVoice(spaceID, speakerID); err != nil {
		util.ErrorResponse(c, response.NotFound, "复刻音色不存在或不属于当前用户", nil)
		return
	}

	cfg, _, err := h.service.ResolveClonedVoiceProviderConfig(spaceID, speakerID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	apiKey, appKey, accessKey, credentialErr := cloneProviderCredentials(cfg)
	if credentialErr != nil {
		util.ErrorResponse(c, response.InvalidParams, credentialErr.Error(), nil)
		return
	}
	if err := DeleteClonedVoice(apiKey, appKey, accessKey, speakerID); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	if err := h.service.DeleteClonedVoiceMetadata(spaceID, speakerID); err != nil {
		util.ErrorResponse(c, response.InternalError, "服务商音色已删除，但清理 Core 元数据失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "已删除", nil)
}
