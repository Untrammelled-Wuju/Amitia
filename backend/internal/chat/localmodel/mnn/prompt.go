// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mnn

import (
	"strings"

	"github.com/u-ai/backend/internal/chat/localmodel"
)

type ToolCallMode string

const (
	ToolCallModeNative ToolCallMode = "native_template"
	ToolCallModePrompt ToolCallMode = "prompt_json"
	ToolCallModeDisabled ToolCallMode = "disabled"
)

func BuildPrompt(messages []localmodel.LocalModelMessage, config MNNProviderConfig) string {
	var sb strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			sb.WriteString("[SYSTEM]\n")
			sb.WriteString(contentText(msg.Parts))
			sb.WriteString("\n")
		case "user":
			sb.WriteString("[USER]\n")
			sb.WriteString(contentText(msg.Parts))
			sb.WriteString("\n")
		case "assistant":
			sb.WriteString("[ASSISTANT]\n")
			sb.WriteString(contentText(msg.Parts))
			sb.WriteString("\n")
		}
	}
	sb.WriteString("[ASSISTANT]\n")
	return sb.String()
}

func contentText(parts []localmodel.LocalModelContent) string {
	var sb strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

func ParseToolCalls(text string) []localmodel.LocalModelToolCall {
	return nil
}
