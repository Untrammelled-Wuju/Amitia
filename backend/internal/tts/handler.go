// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
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
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
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
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
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
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
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

func (h *Handler) Test(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.Test(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	h.service.Update(id, map[string]interface{}{})
	util.SuccessMsgResponse(c, "连接测试成功", nil)
}

func (h *Handler) Synthesize(c *gin.Context) {
	var req SynthesizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	var result *SynthesizeResponse
	var err error
	if req.CharacterID != "" {
		result, err = h.service.SynthesizeForCharacter(req.CharacterID, req.Text)
	} else if req.VoiceID > 0 {
		result, err = h.service.Synthesize(req.VoiceID, req.Text)
	} else if req.SpeakerID != "" {
		result, err = h.service.SynthesizeWithSpeaker(req.SpeakerID, req.Text)
	} else {
		result, err = h.service.SynthesizeWithActive(req.Text)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) CloneVoice(c *gin.Context) {
	apiKey := c.GetHeader("X-Tts-Api-Key")
	if apiKey == "" {
		apiKey = c.Query("apiKey")
	}
	if apiKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少API Key", nil)
		return
	}

	name := c.PostForm("name")
	if name == "" {
		util.ErrorResponse(c, response.InvalidParams, "请填写音色名称", nil)
		return
	}

	langStr := c.PostForm("language")
	language := 0
	if langStr == "en" {
		language = 1
	} else if langStr == "ja" {
		language = 2
	}

	refText := c.PostForm("refText")

	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请上传音频文件", nil)
		return
	}
	defer file.Close()

	audioData, err := io.ReadAll(file)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取音频失败", nil)
		return
	}

	_ = strings.Split(header.Filename, ".")

	cfg, _ := h.service.GetActive()
	appKey := ""
	accessKey := ""
	if cfg != nil {
		appKey = cfg.RealtimeAppId
		accessKey = cfg.RealtimeAccessToken
	}
	var result *VoiceCloneResponse
	var cloneErr error
	if accessKey != "" && appKey != "" {
		result, cloneErr = CloneVoiceV1(accessKey, appKey, name, audioData, "", language, 5)
	} else {
		result, cloneErr = CloneVoice(apiKey, appKey, accessKey, audioData, "", name, language, refText)
	}
	if cloneErr != nil {
		util.ErrorResponse(c, response.OperationFailed, cloneErr.Error(), nil)
		return
	}

	util.SuccessMsgResponse(c, "音色复刻已提交", map[string]interface{}{
		"speakerId": result.SpeakerID,
		"name":      name,
	})
}

func (h *Handler) DeleteClonedVoice(c *gin.Context) {
	apiKey := c.GetHeader("X-Tts-Api-Key")
	if apiKey == "" {
		apiKey = c.Query("apiKey")
	}
	speakerID := c.Query("speakerId")
	if apiKey == "" || speakerID == "" {
		util.ErrorResponse(c, response.InvalidParams, "参数不全", nil)
		return
	}
	cfg2, _ := h.service.GetActive()
	appKey2 := ""
	accessKey2 := ""
	if cfg2 != nil {
		appKey2 = cfg2.RealtimeAppId
		accessKey2 = cfg2.RealtimeAccessToken
	}
	if err := DeleteClonedVoice(apiKey, appKey2, accessKey2, speakerID); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "已删除", nil)
}
