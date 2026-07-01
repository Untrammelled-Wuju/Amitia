// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/u-ai/backend/config")

func analyzeImageInternal(imageUrl string) (string, string) {
	cfg, err := getVisionModelConfig()
	if err != nil {
		return "", err.Error()
	}
	imageData := imageUrl
	if strings.HasPrefix(imageUrl, "/images/") {
		ext := filepath.Ext(imageUrl)
		mimeType := "image/png"
		switch ext {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case ".webp":
			mimeType = "image/webp"
		case ".bmp":
			mimeType = "image/bmp"
		}
		filePath := filepath.Join(config.AppCfg.Storage.DataDir, "images", filepath.Base(imageUrl))
		data, err := os.ReadFile(filePath)
		if err == nil {
			imageData = "data:" + mimeType + ";base64," + base64Encode(data)
		}
	}
	content := []map[string]interface{}{
		{"type": "input_image", "image_url": imageData},
		{"type": "input_text", "text": "请详细描述这张图片的内容，包括场景、物体、人物、文字、表情、氛围等所有可见信息，严禁描述不存在于图片中的信息"},
	}
	return callDoubaoVision(cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, content)
}

func analyzeVideoInternal(videoUrl string) (string, string) {
	cfg, err := getVisionModelConfig()
	if err != nil {
		return "", err.Error()
	}
	if strings.HasPrefix(videoUrl, "data:video/") {
		content := []map[string]interface{}{
			{"type": "input_video", "video_url": videoUrl},
			{"type": "input_text", "text": "请详细描述这段视频的内容，包括场景、人物动作、事件发展、关键画面等所有可见信息，严禁描述不存在于视频中的信息"},
		}
		return callDoubaoVision(cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, content)
	}
	if strings.HasPrefix(videoUrl, "/videos/") {
		filePath := filepath.Join(config.AppCfg.Storage.DataDir, "videos", filepath.Base(videoUrl))
		fileID, err := uploadFileToArk(cfg.BaseUrl, cfg.ApiKey, filePath)
		time.Sleep(5 * time.Second)
		if err != nil {
			return "", fmt.Sprintf("视频上传失败: %s", err.Error())
		}
		content := []map[string]interface{}{
			{"type": "input_video", "file_id": fileID},
			{"type": "input_text", "text": "请详细描述这段视频的内容，包括场景、人物动作、事件发展、关键画面等所有可见信息，严禁描述不存在于视频中的信息"},
		}
		return callDoubaoVision(cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, content)
	}
	return "", fmt.Sprintf("不支持的视频URL格式: %s", videoUrl[:min(len(videoUrl), 100)])
}

func callDoubaoVision(baseURL, apiKey, modelName string, content []map[string]interface{}) (string, string) {
	reqBody := map[string]interface{}{
		"model": modelName,
		"input": []map[string]interface{}{{
			"role":    "user",
			"content": content,
		}},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", strings.TrimRight(baseURL, "/")+"/responses", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", string(body)
	}
	rawBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return "", string(rawBody)
	}
	output, _ := result["output"].([]interface{})
	for _, item := range output {
		m, _ := item.(map[string]interface{})
		if m["type"] == "message" {
			contentArr, _ := m["content"].([]interface{})
			var texts []string
			for _, c := range contentArr {
				cm, _ := c.(map[string]interface{})
				if cm["type"] == "output_text" {
					texts = append(texts, fmt.Sprint(cm["text"]))
				}
			}
			resultText := strings.Join(texts, "")
			return resultText, ""
		}
	}
	return "", string(rawBody)
}
