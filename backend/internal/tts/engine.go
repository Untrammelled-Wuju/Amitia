package tts

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const volcanoSSEUri = "https://openspeech.bytedance.com/api/v3/tts/unidirectional/sse"

type v3ReqParams struct {
	Text        string          `json:"text"`
	Speaker     string          `json:"speaker"`
	AudioParams audioParams     `json:"audio_params"`
	Additions   string          `json:"additions,omitempty"`
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
	h.Write([]byte(fmt.Sprintf("%s|%s|%.1f|%.1f|%.1f|%s", cfg.VoiceType, text, cfg.Speed, cfg.Pitch, cfg.Volume, cfg.Emotion, cfg.EmotionScale, cfg.SilenceDuration)))
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


const volcanoCloneUri = "https://openspeech.bytedance.com/api/v3/tts/voice_clone"
const volcanoV1CloneUri = "https://openspeech.bytedance.com/api/v1/mega_tts/audio/upload"
const volcanoV1StatusUri = "https://openspeech.bytedance.com/api/v1/mega_tts/status"

func CloneVoice(apiKey string, appKey string, accessKey string, audioData []byte, audioFormat string, customName string, language int, refText string) (*VoiceCloneResponse, error) {
	if apiKey == "" && (appKey == "" || accessKey == "") {
		return nil, fmt.Errorf("API Key 未配置")
	}
	if len(audioData) == 0 {
		return nil, fmt.Errorf("音频数据为空")
	}
	if len(audioData) > 10*1024*1024 {
		return nil, fmt.Errorf("音频文件不能超过10MB")
	}

	if audioFormat == "" {
		audioFormat = "mp3"
	}

	reqBody := VoiceCloneRequest{
		SpeakerID: "custom_speaker_id",
		CustomSpeakerID: customName,
		Audio: cloneAudio{
			Data:   base64.StdEncoding.EncodeToString(audioData),
			Format: audioFormat,
		},
		Language: language,
	}
	if refText != "" {
		reqBody.Text = refText
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", volcanoCloneUri, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	} else {
		req.Header.Set("X-Api-App-Key", appKey)
		req.Header.Set("X-Api-Access-Key", accessKey)
	}
	req.Header.Set("X-Api-Request-Id", uuid.New().String())

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("音色复刻请求失败: %w", err)
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("音色复刻返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300))
	}

	var result VoiceCloneResponse
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 3000 {
		return nil, fmt.Errorf("音色复刻失败 [code:%d]: %s", result.Code, result.Message)
	}

	result.CustomSpeakerID = customName
	return &result, nil
}

func DeleteClonedVoice(apiKey string, appKey string, accessKey string, speakerID string) error {
	if (apiKey == "" && (appKey == "" || accessKey == "")) || speakerID == "" {
		return fmt.Errorf("参数不全")
	}
	deleteBody := map[string]string{
		"speaker_id": speakerID,
	}
	jsonBody, _ := json.Marshal(deleteBody)

	req, _ := http.NewRequest("POST", "https://openspeech.bytedance.com/api/v3/tts/voice_clone/delete", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	} else {
		req.Header.Set("X-Api-App-Key", appKey)
		req.Header.Set("X-Api-Access-Key", accessKey)
	}
	req.Header.Set("X-Api-Request-Id", uuid.New().String())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("删除请求失败: %w", err)
	}
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("删除返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 200))
	}
	return nil
}

func CloneVoiceV1(accessToken string, appId string, speakerId string, audioData []byte, audioFormat string, language int, modelType int) (*VoiceCloneResponse, error) {
	if accessToken == "" || appId == "" {
		return nil, fmt.Errorf("Access Token 或 APP ID 未配置")
	}
	if speakerId == "" {
		return nil, fmt.Errorf("音色ID不能为空")
	}
	if len(audioData) == 0 {
		return nil, fmt.Errorf("音频数据为空")
	}
	if len(audioData) > 10*1024*1024 {
		return nil, fmt.Errorf("音频文件不能超过10MB")
	}
	if modelType == 0 {
		modelType = 5
	}
	if audioFormat == "" {
		audioFormat = "mp3"
	}

	body := map[string]interface{}{
		"appid":      appId,
		"speaker_id": speakerId,
		"audios": []map[string]string{{
			"audio_bytes":  base64.StdEncoding.EncodeToString(audioData),
			"audio_format": audioFormat,
		}},
		"source":     2,
		"language":   language,
		"model_type": modelType,
	}
	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", volcanoV1CloneUri, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer;"+accessToken)
	req.Header.Set("Resource-Id", "seed-icl-2.0")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("V1 复刻请求失败: %w", err)
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("V1 复刻返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300))
	}

	return &VoiceCloneResponse{
		SpeakerID:       speakerId,
		CustomSpeakerID: speakerId,
		Code:            3000,
		Message:         "训练已提交",
	}, nil
}

func GetAvailableVoices() []VoicePreset {
	return []VoicePreset{
		{Name: "zh_female_vv_uranus_bigtts", Label: "Vivi 2.0 (活泼灵动)", Gender: "female", SupportsEmotion: true},
		{Name: "zh_female_cancan_uranus_bigtts", Label: "知性灿灿 2.0", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_xiaohe_uranus_bigtts", Label: "小何 2.0 (甜美)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_m191_uranus_bigtts", Label: "云舟 2.0 (沉稳男声)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_taocheng_uranus_bigtts", Label: "小天 2.0 (磁性男声)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_sophie_uranus_bigtts", Label: "魅力苏菲 2.0", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_qingxinnvsheng_uranus_bigtts", Label: "清新女声 2.0", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_sajiaoxuemei_uranus_bigtts", Label: "撒娇学妹 2.0", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_tianmeixiaoyuan_uranus_bigtts", Label: "甜美小源 2.0", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_shuangkuaisisi_uranus_bigtts", Label: "爽快思思 2.0", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_linjianvhai_uranus_bigtts", Label: "邻家女孩 2.0", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_shaonianzixin_uranus_bigtts", Label: "少年梓辛 2.0", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_meilinvyou_uranus_bigtts", Label: "魅力女友 2.0", Gender: "female", SupportsEmotion: false},
		{Name: "en_male_tim_uranus_bigtts", Label: "Tim (美式英语)", Gender: "male", SupportsEmotion: false},
		{Name: "en_female_dacey_uranus_bigtts", Label: "Dacey (美式英语)", Gender: "female", SupportsEmotion: false},
	}
}

func GetEmotions() []string {
	return []string{"", "happy", "sad", "angry", "fearful", "surprised", "neutral"}
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
	if len(runes) <= n { return s }
	return string(runes[:n]) + "..."
}
