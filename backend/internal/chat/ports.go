// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	visioncfg "github.com/u-ai/backend/internal/vision"
)

type WorldBookPort interface {
	ToSystemPromptForSpace(spaceID, characterID, userMessage, assistantReply string) string
}

type VisionPort interface {
	GetActive() (*visioncfg.VisionConfig, error)
}

type worldBookPortAdapter struct {
	ToSystemPromptForSpaceFunc func(spaceID, characterID, userMessage, assistantReply string) string
}

func (a worldBookPortAdapter) ToSystemPromptForSpace(spaceID, characterID, userMessage, assistantReply string) string {
	return a.ToSystemPromptForSpaceFunc(spaceID, characterID, userMessage, assistantReply)
}

type visionPortAdapter struct {
	GetActiveFunc func() (*visioncfg.VisionConfig, error)
}

func (a visionPortAdapter) GetActive() (*visioncfg.VisionConfig, error) {
	return a.GetActiveFunc()
}
