// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/asr"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) VoiceUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少音频文件", nil)
		return
	}
	defer file.Close()

	voiceDir := filepath.Join("data", "voice_msg")
	if err := os.MkdirAll(voiceDir, 0755); err != nil {
		util.ErrorResponse(c, response.InternalError, "创建目录失败", nil)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".webm"
	}
	filename := uuid.New().String() + ext
	savePath := filepath.Join(voiceDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "保存文件失败", nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		util.ErrorResponse(c, response.InternalError, "写入文件失败", nil)
		return
	}

	audioUrl := "/voice/" + filename
	util.SuccessResponse(c, gin.H{"audioUrl": audioUrl, "duration": 0})
}

func (h *Handler) ImageUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少图片文件", nil)
		return
	}
	defer file.Close()

	imageDir := filepath.Join("data", "images")
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		util.ErrorResponse(c, response.InternalError, "创建目录失败", nil)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".png"
	}
	filename := uuid.New().String() + ext
	savePath := filepath.Join(imageDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "保存文件失败", nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		util.ErrorResponse(c, response.InternalError, "写入文件失败", nil)
		return
	}

	imageUrl := "/images/" + filename
	util.SuccessResponse(c, gin.H{"imageUrl": imageUrl})
}

func (h *Handler) VideoUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("video")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少视频文件", nil)
		return
	}
	defer file.Close()

	videoDir := filepath.Join("data", "videos")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		util.ErrorResponse(c, response.InternalError, "创建目录失败", nil)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".mp4"
	}
	filename := uuid.New().String() + ext
	savePath := filepath.Join(videoDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "保存文件失败", nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		util.ErrorResponse(c, response.InternalError, "写入文件失败", nil)
		return
	}

	videoUrl := "/videos/" + filename
	util.SuccessResponse(c, gin.H{"videoUrl": videoUrl})
}

func (h *Handler) VoiceTranscribe(c *gin.Context) {
	var body struct {
		AudioUrl string `json:"audioUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.AudioUrl == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少audioUrl", nil)
		return
	}

	asrRepo := asr.NewRepository(h.db)
	activeCfg, cfgErr := asrRepo.GetActive()
	if cfgErr != nil || activeCfg.ApiKey == "" {
		util.SuccessResponse(c, gin.H{"text": "", "status": "no_asr_key"})
		return
	}

	fullAudioUrl := "http://127.0.0.1:18080" + body.AudioUrl

	taskID, submitErr := asr.SubmitTask(activeCfg, fullAudioUrl, "zh-CN")
	if submitErr != nil {
		util.SuccessResponse(c, gin.H{"text": "", "status": "asr_failed"})
		return
	}

	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		result, queryErr := asr.QueryTask(activeCfg, taskID)
		if queryErr != nil {
			continue
		}
		if result.Status == "done" || result.Status == "success" {
			util.SuccessResponse(c, gin.H{"text": result.Result, "status": "ok"})
			return
		}
		if result.Status == "failed" {
			util.SuccessResponse(c, gin.H{"text": "", "status": "asr_failed"})
			return
		}
	}

	util.SuccessResponse(c, gin.H{"text": "", "status": "timeout"})
}
