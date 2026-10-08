package tts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"gorm.io/gorm"
)

const privateSpeechAudioLimit = 1 << 20

type OwnedVoiceSnapshot struct {
	VoiceConfigID   string  `json:"voiceConfigId"`
	VoiceType       string  `json:"voiceType"`
	VoiceSpeed      float64 `json:"voiceSpeed"`
	VoicePitch      float64 `json:"voicePitch"`
	VoiceVolume     float64 `json:"voiceVolume"`
	CustomVoiceID   string  `json:"customVoiceId"`
	VoiceMode       string  `json:"voiceMode"`
	Emotion         string  `json:"emotion"`
	EmotionScale    int     `json:"emotionScale"`
	SilenceDuration int     `json:"silenceDuration"`
}

func SynthesizePrivateRole(ctx context.Context, db *gorm.DB, space string, profile json.RawMessage, coreOwned bool, text string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(text) > 8192 || strings.TrimSpace(text) == "" {
		return nil, errors.New("朗读文本为空或超过大小上限")
	}
	var snapshot struct {
		Voice *OwnedVoiceSnapshot `json:"voice"`
	}
	if json.Unmarshal(profile, &snapshot) != nil || snapshot.Voice == nil {
		return nil, errors.New("角色缺少当前声学配置，请刷新角色后重试")
	}
	voice := snapshot.Voice
	if voice.VoiceSpeed < 0 || voice.VoiceSpeed > 3 || voice.VoicePitch < 0 || voice.VoicePitch > 3 || voice.VoiceVolume < 0 || voice.VoiceVolume > 3 || voice.EmotionScale < 0 || voice.EmotionScale > 5 || voice.SilenceDuration < 0 || voice.SilenceDuration > 10000 || len(voice.VoiceType) > 256 || len(voice.Emotion) > 128 {
		return nil, errors.New("角色声学配置超过允许范围")
	}
	repo := NewRepository(db.WithContext(ctx))
	active, err := repo.GetActive()
	if err != nil || active == nil {
		return nil, errors.New("当前 Core 没有可用的语音服务配置")
	}
	cfg := *active
	if !coreOwned && voice.VoiceMode != "clone" && voice.CustomVoiceID == "" && strings.TrimSpace(voice.VoiceType) != "" && !publicSourceVoice(active, strings.TrimSpace(voice.VoiceType)) {
		return nil, errors.New("设备音色不是当前 Core 支持的公共预设，请选择兼容音色")
	}
	if strings.TrimSpace(voice.VoiceType) != "" {
		cfg.VoiceType = strings.TrimSpace(voice.VoiceType)
	}
	if !coreOwned {
		if strings.TrimSpace(voice.VoiceType) == "" && (active.IsCustom != 0 || strings.TrimSpace(active.CustomVoiceID) != "") {
			return nil, errors.New("设备角色未指定公共音色，禁止继承当前 Core 的复刻音色")
		}
		if clone, err := repo.GetClonedVoiceBySpeakerID(cfg.VoiceType); err == nil && clone != nil {
			return nil, errors.New("设备角色不能通过预设参数调用当前 Core 的复刻音色")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("当前 Core 暂时无法验证音色归属")
		}
	}
	if voice.VoiceSpeed > 0 {
		cfg.Speed = voice.VoiceSpeed
	}
	if voice.VoicePitch > 0 {
		cfg.Pitch = voice.VoicePitch
	}
	if voice.VoiceVolume > 0 {
		cfg.Volume = voice.VoiceVolume
	}
	cfg.Emotion, cfg.EmotionScale, cfg.SilenceDuration = voice.Emotion, voice.EmotionScale, voice.SilenceDuration
	if voice.VoiceMode == "clone" || strings.TrimSpace(voice.CustomVoiceID) != "" {
		if !coreOwned || strings.TrimSpace(voice.CustomVoiceID) == "" {
			return nil, errors.New("设备复刻音色未获当前 Core 授权，无法调用")
		}
		clone, err := repo.GetClonedVoice(space, voice.CustomVoiceID)
		if err != nil || clone == nil || clone.TtsConfigID != active.ID {
			return nil, errors.New("角色复刻音色不属于当前 Core 的活动语音服务")
		}
		if protocolForApiType(cfg.ApiType) != "volcengine" {
			return nil, errors.New("当前 Core 服务不支持该复刻音色")
		}
		cfg.VoiceType, cfg.ResourceId = clone.SpeakerID, "seed-icl-2.0"
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := synthesizeBytes(ctx, &cfg, text)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(data) == 0 || len(data) > privateSpeechAudioLimit {
		return nil, errors.New("合成音频为空或超过大小上限")
	}
	return data, nil
}

func publicSourceVoice(active *TtsConfig, voice string) bool {
	if active.IsCustom == 0 && active.CustomVoiceID == "" && voice == active.VoiceType {
		return true
	}
	if protocolForApiType(active.ApiType) == "volcengine" {
		for _, preset := range GetAvailableVoices() {
			if preset.Name == voice {
				return true
			}
		}
	}
	if protocolForApiType(active.ApiType) == "openai" {
		for _, preset := range []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "nova", "onyx", "sage", "shimmer", "verse"} {
			if preset == voice {
				return true
			}
		}
	}
	return false
}

func synthesizeBytes(ctx context.Context, cfg *TtsConfig, text string) ([]byte, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("文本为空")
	}
	if cfg.ApiKey == "" && cfg.ApiType != "edge" && cfg.ApiType != "cosyvoice" {
		return nil, errors.New("语音服务未配置凭据")
	}
	switch protocolForApiType(cfg.ApiType) {
	case "openai":
		return synthesizeOpenAI(ctx, cfg, text)
	case "azure":
		return synthesizeAzure(ctx, cfg, text)
	case "edge":
		return synthesizeEdge(ctx, cfg, text)
	case "elevenlabs":
		return synthesizeElevenLabs(ctx, cfg, text)
	case "minimax":
		return synthesizeMiniMax(ctx, cfg, text)
	case "aliyun":
		return synthesizeAliyun(ctx, cfg, text)
	case "cosyvoice":
		return synthesizeCosyVoice(ctx, cfg, text)
	default:
		return synthesizeVolcengine(ctx, cfg, text)
	}
}

type speechBoundedBody struct {
	io.ReadCloser
	remaining int64
}

func (b *speechBoundedBody) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if b.remaining == 0 {
		var probe [1]byte
		count, err := b.ReadCloser.Read(probe[:])
		if count > 0 {
			return 0, fmt.Errorf("语音服务响应超过大小上限")
		}
		return 0, err
	}
	if int64(len(data)) > b.remaining {
		data = data[:b.remaining]
	}
	count, err := b.ReadCloser.Read(data)
	b.remaining -= int64(count)
	return count, err
}
