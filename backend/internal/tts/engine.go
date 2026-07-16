// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const volcanoSSEUri = "https://openspeech.bytedance.com/api/v3/tts/unidirectional/sse"

type v3ReqParams struct {
	Text        string      `json:"text"`
	Speaker     string      `json:"speaker"`
	AudioParams audioParams `json:"audio_params"`
	Additions   string      `json:"additions,omitempty"`
}

type audioParams struct {
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
}

type additions struct {
	SilenceDuration int         `json:"silence_duration,omitempty"`
	SpeechRate      int         `json:"speech_rate,omitempty"`
	LoudnessRate    int         `json:"loudness_rate,omitempty"`
	Emotion         string      `json:"emotion,omitempty"`
	EmotionScale    int         `json:"emotion_scale,omitempty"`
	PostProcess     postProcess `json:"post_process,omitempty"`
	CustomSpeakerID string      `json:"custom_speaker_id,omitempty"`
}

type postProcess struct {
	Pitch int `json:"pitch,omitempty"`
}

type v3SSEReq struct {
	User      v3User      `json:"user"`
	Event     int         `json:"event"`
	ReqParams v3ReqParams `json:"req_params"`
}

type v3User struct {
	UID string `json:"uid"`
}

type v3SSEData struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func getCacheDir() string {
	dir := filepath.Join("data", "tts_cache")
	os.MkdirAll(dir, 0755)
	return dir
}

func cacheKey(cfg *TtsConfig, text string) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s|%s|%.1f|%.1f|%.1f|%s|%d|%d", cfg.VoiceType, text, cfg.Speed, cfg.Pitch, cfg.Volume, cfg.Emotion, cfg.EmotionScale, cfg.SilenceDuration)))
	return fmt.Sprintf("%x.mp3", h.Sum(nil))
}

func Synthesize(cfg *TtsConfig, text string) (*SynthesizeResponse, error) {
	if cfg.ApiKey == "" {
		return nil, fmt.Errorf("API Key 未配置")
	}
	if text == "" {
		return nil, fmt.Errorf("文本为空")
	}

	cacheFile := cacheKey(cfg, text)
	cachePath := filepath.Join(getCacheDir(), cacheFile)
	if _, err := os.Stat(cachePath); err == nil {
		return &SynthesizeResponse{AudioURL: "/audio/" + cacheFile, Duration: 0}, nil
	}

	resourceId := cfg.ResourceId
	if resourceId == "" {
		resourceId = "seed-tts-2.0"
	}

	reqBody := v3SSEReq{
		User:  v3User{UID: "u-ai-user"},
		Event: 100,
		ReqParams: v3ReqParams{
			Text:    text,
			Speaker: cfg.VoiceType,
			AudioParams: audioParams{
				Format:     "mp3",
				SampleRate: 24000,
			},
			Additions: buildAdditions(cfg),
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", volcanoSSEUri, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", cfg.ApiKey)
	req.Header.Set("X-Api-Resource-Id", resourceId)
	req.Header.Set("X-Api-Connect-Id", uuid.New().String())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("火山引擎 TTS 返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300))
	}

	var audioBuffer bytes.Buffer
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			raw := strings.TrimPrefix(line, "data:")
			raw = strings.TrimSpace(raw)
			if raw == "" || raw == "[DONE]" {
				continue
			}
			var sseData v3SSEData
			if err := json.Unmarshal([]byte(raw), &sseData); err != nil {
				continue
			}
			if sseData.Code != 0 && sseData.Code != 20000000 {
				return nil, fmt.Errorf("TTS 错误 [code:%d]: %s", sseData.Code, sseData.Message)
			}
			if sseData.Data != "" {
				chunk, err := base64.StdEncoding.DecodeString(sseData.Data)
				if err != nil {
					continue
				}
				audioBuffer.Write(chunk)
			}
		}
	}

	if audioBuffer.Len() == 0 {
		return nil, fmt.Errorf("未收到音频数据")
	}

	audioBytes := audioBuffer.Bytes()
	if err := os.WriteFile(cachePath, audioBytes, 0644); err != nil {
		return nil, fmt.Errorf("缓存音频失败: %w", err)
	}

	duration := float64(len(audioBytes)) / 24000.0
	return &SynthesizeResponse{AudioURL: "/audio/" + cacheFile, Duration: duration}, nil
}

func TestConnection(cfg *TtsConfig) error {
	if cfg.ApiKey == "" {
		return fmt.Errorf("API Key 未填写")
	}
	_, err := Synthesize(cfg, "测试")
	return err
}

func buildAdditions(cfg *TtsConfig) string {
	a := additions{}
	hasContent := false
	if cfg.SilenceDuration > 0 {
		a.SilenceDuration = cfg.SilenceDuration
		hasContent = true
	}
	speechRate := int((cfg.Speed - 1.0) * 100)
	if speechRate != 0 {
		a.SpeechRate = speechRate
		hasContent = true
	}
	loudnessRate := int((cfg.Volume - 1.0) * 100)
	if loudnessRate != 0 {
		a.LoudnessRate = loudnessRate
		hasContent = true
	}
	if cfg.Emotion != "" {
		a.Emotion = cfg.Emotion
		hasContent = true
		if cfg.EmotionScale > 0 {
			a.EmotionScale = cfg.EmotionScale
		}
	}
	pitch := int(cfg.Pitch)
	if pitch != 0 {
		a.PostProcess = postProcess{Pitch: pitch}
		hasContent = true
	}
	if !hasContent {
		return ""
	}
	b, _ := json.Marshal(a)
	return string(b)
}

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
