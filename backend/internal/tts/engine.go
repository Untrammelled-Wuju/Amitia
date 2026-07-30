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

func protocolForApiType(apiType string) string {
	switch apiType {
	case "openai":
		return "openai"
	case "azure":
		return "azure"
	case "edge":
		return "edge"
	case "elevenlabs":
		return "elevenlabs"
	case "minimax":
		return "minimax"
	case "aliyun":
		return "aliyun"
	case "cosyvoice":
		return "cosyvoice"
	default:
		return "volcengine"
	}
}

func getCacheDir() string {
	dir := filepath.Join("data", "tts_cache")
	os.MkdirAll(dir, 0755)
	return dir
}

func cacheKey(cfg *TtsConfig, text string) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s|%s|%s|%.1f|%.1f|%.1f|%s|%d|%d", cfg.ApiType, cfg.VoiceType, text, cfg.Speed, cfg.Pitch, cfg.Volume, cfg.Emotion, cfg.EmotionScale, cfg.SilenceDuration)))
	return fmt.Sprintf("%x.mp3", h.Sum(nil))
}

func Synthesize(cfg *TtsConfig, text string) (*SynthesizeResponse, error) {
	if cfg.ApiType == "" {
		cfg.ApiType = "volcengine"
	}
	if cfg.ApiKey == "" && cfg.ApiType != "edge" && cfg.ApiType != "cosyvoice" {
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

	var audioBytes []byte
	var err error
	switch protocolForApiType(cfg.ApiType) {
	case "openai":
		audioBytes, err = synthesizeOpenAI(cfg, text)
	case "azure":
		audioBytes, err = synthesizeAzure(cfg, text)
	case "edge":
		audioBytes, err = synthesizeEdge(cfg, text)
	case "elevenlabs":
		audioBytes, err = synthesizeElevenLabs(cfg, text)
	case "minimax":
		audioBytes, err = synthesizeMiniMax(cfg, text)
	case "aliyun":
		audioBytes, err = synthesizeAliyun(cfg, text)
	case "cosyvoice":
		audioBytes, err = synthesizeCosyVoice(cfg, text)
	default:
		audioBytes, err = synthesizeVolcengine(cfg, text)
	}
	if err != nil {
		return nil, err
	}

	if len(audioBytes) == 0 {
		return nil, fmt.Errorf("未收到音频数据")
	}

	if err := os.WriteFile(cachePath, audioBytes, 0644); err != nil {
		return nil, fmt.Errorf("缓存音频失败: %w", err)
	}

	duration := float64(len(audioBytes)) / 24000.0
	return &SynthesizeResponse{AudioURL: "/audio/" + cacheFile, Duration: duration}, nil
}

func TestConnection(cfg *TtsConfig) error {
	if cfg.ApiType == "" {
		cfg.ApiType = "volcengine"
	}
	if cfg.ApiKey == "" && cfg.ApiType != "edge" && cfg.ApiType != "cosyvoice" {
		return fmt.Errorf("API Key 未填写")
	}
	_, err := Synthesize(cfg, "测试")
	return err
}

func synthesizeVolcengine(cfg *TtsConfig, text string) ([]byte, error) {
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
		return nil, fmt.Errorf("火山引擎 TTS 返回 %d: %s", resp.StatusCode, string(rawBody))
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

	return audioBuffer.Bytes(), nil
}

func synthesizeOpenAI(cfg *TtsConfig, text string) ([]byte, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := cfg.ResourceId
	if model == "" {
		model = "tts-1"
	}

	voice := cfg.VoiceType
	if voice == "" {
		voice = "alloy"
	}

	reqBody := map[string]interface{}{
		"model":           model,
		"input":           text,
		"voice":           voice,
		"response_format": "mp3",
	}
	if cfg.Speed > 0 {
		reqBody["speed"] = cfg.Speed
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/audio/speech", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI TTS 返回 %d: %s", resp.StatusCode, string(rawBody))
	}

	return io.ReadAll(resp.Body)
}

func synthesizeAzure(cfg *TtsConfig, text string) ([]byte, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://tts.speech.microsoft.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	voice := cfg.VoiceType
	if voice == "" {
		voice = "zh-CN-XiaoxiaoNeural"
	}

	rate := "0%"
	if cfg.Speed != 1.0 {
		rate = fmt.Sprintf("%.0f%%", (cfg.Speed-1.0)*100)
	}
	pitch := "0%"
	if cfg.Pitch != 1.0 {
		pitch = fmt.Sprintf("%.0f%%", (cfg.Pitch-1.0)*100)
	}
	volume := "0%"
	if cfg.Volume != 1.0 {
		volume = fmt.Sprintf("%.0f%%", (cfg.Volume-1.0)*100)
	}

	ssml := fmt.Sprintf(`<speak version='1.0' xml:lang='zh-CN'><voice name='%s'><prosody rate='%s' pitch='%s' volume='%s'>%s</prosody></voice></speak>`, voice, rate, pitch, volume, text)

	req, err := http.NewRequest("POST", baseURL+"/cognitiveservices/v1", strings.NewReader(ssml))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	req.Header.Set("X-Microsoft-OutputFormat", "audio-16khz-128kbitrate-mono-mp3")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Azure TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure TTS 返回 %d: %s", resp.StatusCode, string(rawBody))
	}

	return io.ReadAll(resp.Body)
}

func synthesizeEdge(cfg *TtsConfig, text string) ([]byte, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://speech.platform.bing.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	voice := cfg.VoiceType
	if voice == "" {
		voice = "zh-CN-XiaoxiaoNeural"
	}

	rate := "+0%"
	if cfg.Speed != 1.0 {
		rate = fmt.Sprintf("%+.0f%%", (cfg.Speed-1.0)*100)
	}
	pitch := "+0Hz"
	if cfg.Pitch != 1.0 {
		pitch = fmt.Sprintf("%+.0fHz", (cfg.Pitch-1.0)*50)
	}
	volume := "+0%"
	if cfg.Volume != 1.0 {
		volume = fmt.Sprintf("%+.0f%%", (cfg.Volume-1.0)*100)
	}

	ssml := fmt.Sprintf(`<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='zh-CN'><voice name='%s'><prosody rate='%s' pitch='%s' volume='%s'>%s</prosody></voice></speak>`, voice, rate, pitch, volume, text)

	reqBody := map[string]string{
		"ssml": ssml,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/api/tts", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Edge TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Edge TTS 返回 %d: %s", resp.StatusCode, string(rawBody))
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		body, _ := io.ReadAll(resp.Body)
		var result struct {
			Audio string `json:"audio"`
			Data  string `json:"data"`
		}
		if err := json.Unmarshal(body, &result); err == nil {
			audioB64 := result.Audio
			if audioB64 == "" {
				audioB64 = result.Data
			}
			if audioB64 != "" {
				return base64.StdEncoding.DecodeString(audioB64)
			}
		}
		return nil, fmt.Errorf("Edge TTS 未返回有效音频数据")
	}

	return io.ReadAll(resp.Body)
}

func synthesizeElevenLabs(cfg *TtsConfig, text string) ([]byte, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.elevenlabs.io/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	voice := cfg.VoiceType
	if voice == "" {
		voice = "21m00Tcm4TlvDq8ikWAM"
	}

	model := cfg.ResourceId
	if model == "" {
		model = "eleven_multilingual_v2"
	}

	reqBody := map[string]interface{}{
		"text":     text,
		"model_id": model,
		"voice_settings": map[string]interface{}{
			"stability":        0.5,
			"similarity_boost": 0.5,
		},
	}
	if cfg.Speed > 0 {
		reqBody["voice_settings"].(map[string]interface{})["speed"] = cfg.Speed
	}
	jsonBody, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/text-to-speech/%s", baseURL, voice)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", cfg.ApiKey)
	req.Header.Set("Accept", "audio/mpeg")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ElevenLabs TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ElevenLabs TTS 返回 %d: %s", resp.StatusCode, string(rawBody))
	}

	return io.ReadAll(resp.Body)
}

func synthesizeMiniMax(cfg *TtsConfig, text string) ([]byte, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.minimax.chat/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := cfg.ResourceId
	if model == "" {
		model = "speech-01-hd"
	}

	voice := cfg.VoiceType
	if voice == "" {
		voice = "male-qn-qingse"
	}

	voiceSetting := map[string]interface{}{
		"voice_id": voice,
	}
	if cfg.Speed > 0 {
		voiceSetting["speed"] = cfg.Speed
	}
	if cfg.Volume > 0 {
		voiceSetting["vol"] = cfg.Volume
	}
	if cfg.Pitch > 0 {
		voiceSetting["pitch"] = cfg.Pitch
	}

	reqBody := map[string]interface{}{
		"model":         model,
		"text":          text,
		"stream":        false,
		"voice_setting": voiceSetting,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/text_to_speech", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MiniMax TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("MiniMax TTS 返回 %d: %s", resp.StatusCode, string(rawBody))
	}

	var result struct {
		Data  string `json:"data"`
		Audio string `json:"audio"`
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析MiniMax响应失败: %w", err)
	}

	audioB64 := result.Data
	if audioB64 == "" {
		audioB64 = result.Audio
	}
	if audioB64 == "" {
		return nil, fmt.Errorf("MiniMax未返回音频数据")
	}

	return base64.StdEncoding.DecodeString(audioB64)
}

func synthesizeAliyun(cfg *TtsConfig, text string) ([]byte, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://nls-gateway.cn-shanghai.aliyuncs.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	voice := cfg.VoiceType
	if voice == "" {
		voice = "xiaoyun"
	}

	reqBody := map[string]interface{}{
		"text":        text,
		"voice":       voice,
		"format":      "mp3",
		"sample_rate": 24000,
	}
	if cfg.Speed > 0 {
		reqBody["speech_rate"] = int((cfg.Speed - 1.0) * 100)
	}
	if cfg.Pitch > 0 {
		reqBody["pitch_rate"] = int((cfg.Pitch - 1.0) * 100)
	}
	if cfg.Volume > 0 {
		reqBody["volume"] = int((cfg.Volume - 1.0) * 100)
	}
	jsonBody, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/rest/v1/services/aigc/text2speech", baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	req.Header.Set("X-NLS-Token", cfg.ApiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("阿里云 TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("阿里云 TTS 返回 %d: %s", resp.StatusCode, string(rawBody))
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "audio/") {
		return io.ReadAll(resp.Body)
	}

	var result struct {
		Output struct {
			Audio string `json:"audio"`
		} `json:"output"`
		Data string `json:"data"`
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析阿里云响应失败: %w", err)
	}

	audioB64 := result.Output.Audio
	if audioB64 == "" {
		audioB64 = result.Data
	}
	if audioB64 == "" {
		return nil, fmt.Errorf("阿里云未返回音频数据")
	}

	return base64.StdEncoding.DecodeString(audioB64)
}

func synthesizeCosyVoice(cfg *TtsConfig, text string) ([]byte, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://127.0.0.1:5000"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	voice := cfg.VoiceType
	if voice == "" {
		voice = "default"
	}

	reqBody := map[string]interface{}{
		"text":  text,
		"voice": voice,
	}
	if cfg.Speed > 0 {
		reqBody["speed"] = cfg.Speed
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/api/cosyvoice/tts", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CosyVoice TTS 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rawBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CosyVoice TTS 返回 %d: %s", resp.StatusCode, string(rawBody))
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "application/octet-stream") {
		return io.ReadAll(resp.Body)
	}

	var result struct {
		Audio string `json:"audio"`
		Data  string `json:"data"`
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return bodyBytes, nil
	}

	audioB64 := result.Audio
	if audioB64 == "" {
		audioB64 = result.Data
	}
	if audioB64 == "" {
		return bodyBytes, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		return bodyBytes, nil
	}
	return decoded, nil
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
