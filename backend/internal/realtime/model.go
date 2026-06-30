// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

const volcanoRealtimeUri = "wss://openspeech.bytedance.com/api/v3/realtime/dialogue"

type RealtimeEvent struct {
	EventType string      `json:"event_type"`
	Data      interface{} `json:"data,omitempty"`
}

type StartSessionData struct {
	Asr struct {
		AudioInfo struct {
			Format     string `json:"format"`
			SampleRate int    `json:"sample_rate"`
			Channel    int    `json:"channel"`
		} `json:"audio_info"`
	} `json:"asr"`
	Tts struct {
		Speaker     string `json:"speaker,omitempty"`
		AudioConfig struct {
			Channel    int    `json:"channel"`
			Format     string `json:"format"`
			SampleRate int    `json:"sample_rate"`
		} `json:"audio_config,omitempty"`
	} `json:"tts"`
	BotName       string `json:"bot_name,omitempty"`
	SystemRole    string `json:"system_role,omitempty"`
	SpeakingStyle string `json:"speaking_style,omitempty"`
	AuditResponse string `json:"audit_response,omitempty"`
}

type AudioQueryData struct {
	Audio string `json:"audio"`
}

type TextQueryData struct {
	Text string `json:"text"`
}

type AsrEndedData struct {
	QuestionID string `json:"question_id"`
}

type TtsSentenceEndData struct {
	SentenceID string `json:"sentence_id"`
}

type SessionEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data,omitempty"`
}

type RealtimeConfig struct {
	ApiKey     string `json:"apiKey"`
	ResourceId string `json:"resourceId"`
	VoiceType  string `json:"voiceType"`
	BotName    string `json:"botName"`
	SystemRole string `json:"systemRole"`
}
