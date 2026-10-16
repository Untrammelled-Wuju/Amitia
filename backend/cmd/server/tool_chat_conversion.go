package main

import (
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/extension/kernel"
)

func kernelSkillsToChat(items []kernel.ActivatedSkill) []chat.ActivatedSkill {
	result := make([]chat.ActivatedSkill, 0, len(items))
	for _, item := range items {
		result = append(result, chat.ActivatedSkill{
			ActivationID:        item.ActivationID,
			ExtensionID:         item.ExtensionID,
			Name:                item.Name,
			Source:              item.Source,
			Scope:               item.Scope,
			CompatibilityStatus: item.CompatibilityStatus,
			Prompt:              item.Prompt,
			BodyTokens:          item.BodyTokens,
			Explicit:            item.Explicit,
			ToolMappings:        item.ToolMappings,
		})
	}
	return result
}

func kernelContributionsToChat(items []kernel.ContextContribution) []chat.ContextContribution {
	result := make([]chat.ContextContribution, 0, len(items))
	for _, c := range items {
		result = append(result, chat.ContextContribution{
			Source:     c.Source,
			Priority:   c.Priority,
			Content:    c.Content,
			TokenLimit: c.TokenLimit,
			ExpiresAt:  c.ExpiresAt,
			Metadata:   c.Metadata,
		})
	}
	return result
}

func kernelToolResultToChat(r kernel.ToolDispatchResult) chat.ToolResult {
	result := chat.ToolResult{
		RunID:       r.RunID,
		Status:      r.Status,
		Output:      r.Output,
		DurationMS:  r.DurationMS,
		VisibleText: r.VisibleText,
		ForceVoice:  r.ForceVoice,
	}
	if r.Error != nil {
		result.Error = &chat.ToolError{
			Code:      r.Error.Code,
			Message:   r.Error.Message,
			Detail:    r.Error.Detail,
			Retryable: r.Error.Retryable,
		}
	}
	return result
}
