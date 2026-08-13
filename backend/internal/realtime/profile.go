// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"sync"
)

type VoiceProfile struct {
	ID                 string `json:"id" gorm:"primaryKey"`
	Name               string `json:"name"`
	ASRConfigID        int    `json:"asrConfigId"`
	TTSConfigID        int    `json:"ttsConfigId"`
	RealtimeProviderID string `json:"realtimeProviderId"`
	WakeConfigID       string `json:"wakeConfigId"`
	VADPreset          string `json:"vadPreset"`
	InterruptPolicy    string `json:"interruptPolicy"`
	PrivacyMode        string `json:"privacyMode"`
	IsDefault          bool   `json:"isDefault"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type WakeConfig struct {
	ID               string  `json:"id" gorm:"primaryKey"`
	Name             string  `json:"name"`
	Enabled          bool    `json:"enabled"`
	Backend          string  `json:"backend"`
	ModelResourceURI string  `json:"modelResourceUri"`
	Phrases          string  `json:"phrases"`
	Threshold        float64 `json:"threshold"`
	CooldownMS       int64   `json:"cooldownMs"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

var activeVoiceProfiles sync.Map

func GetDefaultProfile() *VoiceProfile {
	return &VoiceProfile{
		ID:              "default",
		Name:            "Default",
		VADPreset:       "default",
		InterruptPolicy: "immediate",
		PrivacyMode:     "standard",
	}
}

func (p *VoiceProfile) ValidateVoice() error {
	if p.VADPreset == "" {
		p.VADPreset = "default"
	}
	if p.InterruptPolicy == "" {
		p.InterruptPolicy = "immediate"
	}
	if p.PrivacyMode == "" {
		p.PrivacyMode = "standard"
	}
	return nil
}

func (p *VoiceProfile) IsLocalOnly() bool {
	return p.PrivacyMode == "local_only"
}

func (p *VoiceProfile) AllowsBackgroundCapture() bool {
	return p.PrivacyMode != "local_only"
}

