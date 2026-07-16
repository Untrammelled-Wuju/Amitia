package tts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"time"
)

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
		SpeakerID:       "custom_speaker_id",
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
