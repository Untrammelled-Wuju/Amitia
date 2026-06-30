// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

func init() {
	Register(Tool{
		Type: "function",
		Function: Function{
			Name:        "force_voice_reply",
			Description: "当用户明确要求用语音回复、发语音、语音回答、说语音、讲语音时调用此工具。调用后本次回复将以语音消息形式发送给用户。",
			Parameters: Parameters{
				Type:       "object",
				Properties: map[string]Property{},
				Required:   []string{},
			},
		},
	}, func(args map[string]interface{}) string {
		SetForceVoice()
		return "OK"
	})
}

var forceVoiceFlag bool

func SetForceVoice() {
	forceVoiceFlag = true
}

func GetForceVoice() bool {
	v := forceVoiceFlag
	forceVoiceFlag = false
	return v
}
