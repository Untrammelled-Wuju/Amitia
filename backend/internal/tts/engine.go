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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

func GetAvailableVoices() []VoicePreset {
	return []VoicePreset{
		{Name: "zh_female_vv_uranus_bigtts", Label: "Vivi 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "saturn_zh_female_cancan_tob", Label: "知性灿灿 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "saturn_zh_female_keainvsheng_tob", Label: "可爱女生 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "saturn_zh_female_tiaopigongzhu_tob", Label: "调皮公主 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "saturn_zh_male_shuanglangshaonian_tob", Label: "爽朗少年 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "saturn_zh_male_tiancaitongzhuo_tob", Label: "天才同桌 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_xiaohe_uranus_bigtts", Label: "小何 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_m191_uranus_bigtts", Label: "云舟 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_taocheng_uranus_bigtts", Label: "小天 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "en_male_tim_uranus_bigtts", Label: "Tim (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "en_female_dacey_uranus_bigtts", Label: "Dacey (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "en_female_stokie_uranus_bigtts", Label: "Stokie (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_liufei_uranus_bigtts", Label: "刘飞 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_qingxinnvsheng_uranus_bigtts", Label: "清新女声 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_cancan_uranus_bigtts", Label: "知性灿灿 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_sajiaoxuemei_uranus_bigtts", Label: "撒娇学妹 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_tianmeixiaoyuan_uranus_bigtts", Label: "甜美小源 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_tianmeitaozi_uranus_bigtts", Label: "甜美桃子 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_shuangkuaisisi_uranus_bigtts", Label: "爽快思思 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_peiqi_uranus_bigtts", Label: "佩奇猪 2.0 (视频配音)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_linjianvhai_uranus_bigtts", Label: "邻家女孩 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_shaonianzixin_uranus_bigtts", Label: "少年梓辛/Brayan 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_sunwukong_uranus_bigtts", Label: "猴哥 2.0 (视频配音)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_yingyujiaoxue_uranus_bigtts", Label: "Tina老师 2.0 (教育)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_kefunvsheng_uranus_bigtts", Label: "暖阳女声 2.0 (客服)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_xiaoxue_uranus_bigtts", Label: "儿童绘本 2.0 (有声阅读)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_dayi_uranus_bigtts", Label: "大壹 2.0 (视频配音)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_mizai_uranus_bigtts", Label: "黑猫侦探社咪仔 2.0 (视频配音)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_jitangnv_uranus_bigtts", Label: "鸡汤女 2.0 (视频配音)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_meilinvyou_uranus_bigtts", Label: "魅力女友 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_liuchangnv_uranus_bigtts", Label: "流畅女声 2.0 (视频配音)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_ruyayichen_uranus_bigtts", Label: "儒雅逸辰 2.0 (视频配音)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_wenroumama_uranus_bigtts", Label: "温柔妈妈 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_jieshuoxiaoming_uranus_bigtts", Label: "解说小明 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_tvbnv_uranus_bigtts", Label: "TVB女声 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_yizhipiannan_uranus_bigtts", Label: "译制片男 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_qiaopinv_uranus_bigtts", Label: "俏皮女声 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_zhishuaiyingzi_uranus_bigtts", Label: "直率英子 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_linjiananhai_uranus_bigtts", Label: "邻家男孩 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_silang_uranus_bigtts", Label: "四郎 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_ruyaqingnian_uranus_bigtts", Label: "儒雅青年 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_qingcang_uranus_bigtts", Label: "擎苍 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_xionger_uranus_bigtts", Label: "熊二 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_yingtaowanzi_uranus_bigtts", Label: "樱桃丸子 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_wennuanahu_uranus_bigtts", Label: "温暖阿虎/Alvin 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_naiqimengwa_uranus_bigtts", Label: "奶气萌娃 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_popo_uranus_bigtts", Label: "婆婆 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_gaolengyujie_uranus_bigtts", Label: "高冷御姐 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_aojiaobazong_uranus_bigtts", Label: "傲娇霸总 2.0 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_lanyinmianbao_uranus_bigtts", Label: "懒音绵宝 2.0 (有声阅读)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_fanjuanqingnian_uranus_bigtts", Label: "反卷青年 2.0 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_wenroushunv_uranus_bigtts", Label: "温柔淑女 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_gufengshaoyu_uranus_bigtts", Label: "古风少御 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_huolixiaoge_uranus_bigtts", Label: "活力小哥 2.0 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_baqiqingshu_uranus_bigtts", Label: "霸气青叔 2.0 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_xuanyijieshuo_uranus_bigtts", Label: "悬疑解说 2.0 (有声阅读)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_mengyatou_uranus_bigtts", Label: "萌丫头/Cutey 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_tiexinnvsheng_uranus_bigtts", Label: "贴心女声/Candy 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_jitangmei_uranus_bigtts", Label: "鸡汤妹妹/Hope 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_cixingjieshuonan_uranus_bigtts", Label: "磁性解说男声/Morgan 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_liangsangmengzai_uranus_bigtts", Label: "亮嗓萌仔 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_kailangjiejie_uranus_bigtts", Label: "开朗姐姐 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_gaolengchenwen_uranus_bigtts", Label: "高冷沉稳 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_shenyeboke_uranus_bigtts", Label: "深夜播客 2.0 (多情感)", Gender: "male", SupportsEmotion: true},
		{Name: "zh_male_lubanqihao_uranus_bigtts", Label: "鲁班七号 2.0 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_jiaochuannv_uranus_bigtts", Label: "娇喘女声 2.0 (视频配音)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_linxiao_uranus_bigtts", Label: "林潇 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_lingling_uranus_bigtts", Label: "玲玲姐姐 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_chunribu_uranus_bigtts", Label: "春日部姐姐 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_tangseng_uranus_bigtts", Label: "唐僧 2.0 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_zhuangzhou_uranus_bigtts", Label: "庄周 2.0 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_kailangdidi_uranus_bigtts", Label: "开朗弟弟 2.0 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_zhubajie_uranus_bigtts", Label: "猪八戒 2.0 (角色扮演)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_ganmaodianyin_uranus_bigtts", Label: "感冒电音姐姐 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_chanmeinv_uranus_bigtts", Label: "谄媚女声 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_nvleishen_uranus_bigtts", Label: "女雷神 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_qinqienv_uranus_bigtts", Label: "亲切女声 2.0 (有声阅读)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_kuailexiaodong_uranus_bigtts", Label: "快乐小东 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_kailangxuezhang_uranus_bigtts", Label: "开朗学长 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_youyoujunzi_uranus_bigtts", Label: "悠悠君子 2.0 (有声阅读)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_wenjingmaomao_uranus_bigtts", Label: "文静毛毛 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_zhixingnv_uranus_bigtts", Label: "知性女声 2.0 (有声阅读)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_qingshuangnanda_uranus_bigtts", Label: "清爽男大 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_yuanboxiaoshu_uranus_bigtts", Label: "渊博小叔 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_male_yangguangqingnian_uranus_bigtts", Label: "阳光青年 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_qingchezizi_uranus_bigtts", Label: "清澈梓梓 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_tianmeiyueyue_uranus_bigtts", Label: "甜美悦悦 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_xinlingjitang_uranus_bigtts", Label: "心灵鸡汤 2.0 (有声阅读)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_wenrouxiaoge_uranus_bigtts", Label: "温柔小哥 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_roumeinvyou_uranus_bigtts", Label: "柔美女友 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_dongfanghaoran_uranus_bigtts", Label: "东方浩然 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_wenrouxiaoya_uranus_bigtts", Label: "温柔小雅 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_tiancaitongsheng_uranus_bigtts", Label: "天才童声 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_wuzetian_uranus_bigtts", Label: "武则天 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_gujie_uranus_bigtts", Label: "顾姐 2.0 (角色扮演)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_male_guanggaojieshuo_uranus_bigtts", Label: "广告解说 2.0 (通用)", Gender: "male", SupportsEmotion: false},
		{Name: "zh_female_shaoergushi_uranus_bigtts", Label: "少儿故事 2.0 (有声阅读)", Gender: "female", SupportsEmotion: false},
		{Name: "zh_female_sophie_uranus_bigtts", Label: "魅力苏菲 2.0 (通用)", Gender: "female", SupportsEmotion: false},
		{Name: "saturn_zh_male_qingxinmumu_cs_tob", Label: "清新沐沐 2.0 (客服)", Gender: "male", SupportsEmotion: false},
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
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func HandlePlayMessage(c *gin.Context, db interface{}) {
	msgID := c.Param("messageId")
	if msgID == "" {
		c.JSON(400, gin.H{"error": "missing messageId"})
		return
	}
	type Msg struct {
		Content string
		MsgType string
	}
	var msg Msg
	gdb := db.(*gorm.DB)
	if err := gdb.Table("messages").Select("content, msg_type").Where("id = ?", msgID).Row().Scan(&msg.Content, &msg.MsgType); err != nil {
		c.JSON(404, gin.H{"error": "message not found"})
		return
	}
	if msg.Content == "" {
		c.JSON(404, gin.H{"error": "empty message"})
		return
	}
	repo := NewRepository(gdb)
	svc := NewService(repo)
	result, err := svc.SynthesizeWithActive(msg.Content)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	audioPath := "data/tts_cache/" + strings.TrimPrefix(result.AudioURL, "/audio/")
	c.File(audioPath)
}

func GetActiveConfig(db *gorm.DB) (*TtsConfig, error) {
	var cfg TtsConfig
	if err := db.Table("tts_configs").Where("is_active = 1").Limit(1).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}
