// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

type TtsConfig struct {
	ID                  int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name                string  `gorm:"column:name;not null" json:"name"`
	ApiKey              string  `gorm:"column:api_key" json:"apiKey"`
	ResourceId          string  `gorm:"column:resource_id;default:seed-tts-2.0" json:"resourceId"`
	VoiceType           string  `gorm:"column:voice_type;default:zh_female_vv_uranus_bigtts" json:"voiceType"`
	Emotion             string  `gorm:"column:emotion" json:"emotion"`
	Speed               float64 `gorm:"column:speed;default:1.0" json:"speed"`
	Pitch               float64 `gorm:"column:pitch;default:1.0" json:"pitch"`
	Volume              float64 `gorm:"column:volume;default:1.0" json:"volume"`
	IsActive            int     `gorm:"column:is_active;default:0" json:"isActive"`
	IsCustom            int     `gorm:"column:is_custom;default:0" json:"isCustom"`
	CustomVoiceID       string  `gorm:"column:custom_voice_id" json:"customVoiceId"`
	CloneResourceId     string  `gorm:"column:clone_resource_id;default:volc.megatts.timbre" json:"cloneResourceId"`
	RealtimeAppId       string  `gorm:"column:realtime_app_id" json:"realtimeAppId"`
	RealtimeAccessToken string  `gorm:"column:realtime_access_token" json:"realtimeAccessToken"`
	RealtimeSecretKey   string  `gorm:"column:realtime_secret_key" json:"realtimeSecretKey"`
	UpdatedAt           string  `gorm:"column:updated_at" json:"updatedAt"`
	EmotionScale        int     `gorm:"-" json:"emotionScale"`
	SilenceDuration     int     `gorm:"-" json:"silenceDuration"`
	LastTestResult      string  `gorm:"-" json:"lastTestResult"`
	HasApiKey           bool    `gorm:"-" json:"hasApiKey"`
	CreatedAt           string  `gorm:"column:created_at" json:"createdAt"`
}

func (TtsConfig) TableName() string { return "tts_configs" }

type CreateTtsConfigRequest struct {
	Name                string  `json:"name"`
	ApiKey              string  `json:"apiKey"`
	ResourceId          string  `json:"resourceId"`
	VoiceType           string  `json:"voiceType"`
	Emotion             string  `json:"emotion"`
	Speed               float64 `json:"speed"`
	Pitch               float64 `json:"pitch"`
	Volume              float64 `json:"volume"`
	RealtimeAppId       string  `json:"realtimeAppId"`
	RealtimeAccessToken string  `json:"realtimeAccessToken"`
	RealtimeSecretKey   string  `json:"realtimeSecretKey"`
	IsActive            int     `json:"isActive"`
}

type SynthesizeRequest struct {
	SpeakerID   string `json:"speakerId"`
	Text        string `json:"text"`
	VoiceID     int    `json:"voiceId"`
	CharacterID string `json:"characterId"`
}

type SynthesizeResponse struct {
	AudioURL string  `json:"audioUrl"`
	Duration float64 `json:"duration"`
}

type VoicePreset struct {
	Name            string `json:"name"`
	Label           string `json:"label"`
	Gender          string `json:"gender"`
	SupportsEmotion bool   `json:"supportsEmotion"`
}

type VoiceCloneRequest struct {
	SpeakerID       string     `json:"speaker_id"`
	CustomSpeakerID string     `json:"custom_speaker_id,omitempty"`
	Audio           cloneAudio `json:"audio"`
	Text            string     `json:"text,omitempty"`
	Language        int        `json:"language,omitempty"`
}

type cloneAudio struct {
	Data   string `json:"data"`
	Format string `json:"format,omitempty"`
}

type VoiceCloneResponse struct {
	SpeakerID       string `json:"speaker_id"`
	CustomSpeakerID string `json:"custom_speaker_id,omitempty"`
	Code            int    `json:"code"`
	Message         string `json:"message"`
}

type ClonedVoice struct {
	SpeakerID string `json:"speakerId"`
	Name      string `json:"name"`
	Language  int    `json:"language"`
	CreatedAt string `json:"createdAt"`
	Status    string `json:"status"`
}
