package card

import (
	"strings"
)

func (card *CharacterCard) BuildPreview() *CharacterCardPreview {
	risks := card.DetectRisks()

	return &CharacterCardPreview{
		Format:             string(card.SourceFormat),
		SpecVersion:        card.specVersionString(),
		Name:               card.Name,
		Creator:            card.Creator,
		CharacterVersion:   card.CharacterVersion,
		DescriptionLength:  len(card.Description),
		PersonalityLength:  len(card.Personality),
		HasSystemPrompt:    card.SystemPrompt != "",
		HasPostHistory:     card.PostHistoryInstructions != "",
		GreetingCount:      len(card.AlternateGreetings) + boolToInt(card.FirstMessage != ""),
		LorebookEntryCount: card.lorebookCount(),
		AssetCount:         len(card.Assets),
		UnknownFieldCount:  len(card.Preserved),
		Risks:              risks,
	}
}

func (card *CharacterCard) DetectRisks() []CardImportRisk {
	var risks []CardImportRisk

	if len(card.SystemPrompt) > MaxSystemPromptBytes {
		risks = append(risks, CardImportRisk{
			Category: "system_prompt_size",
			Level:    "medium",
			Message:  "System prompt 超出建议长度",
		})
	}
	if len(card.PostHistoryInstructions) > MaxPostHistoryBytes {
		risks = append(risks, CardImportRisk{
			Category: "post_history_size",
			Level:    "medium",
			Message:  "Post-history instructions 超出建议长度",
		})
	}
	if len(card.Personality) > MaxPersonalityBytes {
		risks = append(risks, CardImportRisk{
			Category: "personality_size",
			Level:    "medium",
			Message:  "Personality 超出建议长度",
		})
	}

	if containsPromptInjectionPatterns(card.SystemPrompt) {
		risks = append(risks, CardImportRisk{
			Category: "prompt_injection",
			Level:    "high",
			Message:  "System prompt 包含可疑指令覆盖模式",
		})
	}

	if card.CharacterBook != nil && len(card.CharacterBook.Entries) > MaxLorebookEntries {
		risks = append(risks, CardImportRisk{
			Category: "lorebook_size",
			Level:    "medium",
			Message:  "Lorebook 条目超出限制",
		})
	}

	for _, asset := range card.Assets {
		if v, ok := asset.Metadata["uri"].(string); ok && (strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")) {
			risks = append(risks, CardImportRisk{
				Category: "remote_asset",
				Level:    "low",
				Message:  "卡片包含远程资产引用",
			})
			break
		}
	}

	if card.SourceFormat == FormatV3CHARX {
		risks = append(risks, CardImportRisk{
			Category: "charx_format",
			Level:    "low",
			Message:  "CHARX 格式中的代码资产不会被执行或加载",
		})
	}

	if len(card.Extensions) > 0 {
		extBytes := estimateMapSize(card.Extensions)
		if extBytes > MaxExtensionsBytes {
			risks = append(risks, CardImportRisk{
				Category: "extensions_size",
				Level:    "medium",
				Message:  "Extensions 超出建议大小",
			})
		}
	}

	if card.SourceFormat == FormatV2JSON || card.SourceFormat == FormatV2PNG {
		if card.unknownFieldCount() > 0 {
			risks = append(risks, CardImportRisk{
				Category: "unknown_fields",
				Level:    "low",
				Message:  "存在未来版本兼容字段将被保留",
			})
		}
	}

	return risks
}

func containsPromptInjectionPatterns(text string) bool {
	patterns := []string{
		"ignore all previous instructions",
		"ignore previous instructions",
		"you are now",
		"new system prompt",
		"system override",
		"jailbreak",
		"你现在是",
		"忽略之前的指令",
		"忽略所有指令",
	}
	lower := strings.ToLower(text)
	count := 0
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			count++
		}
	}
	return count >= 3
}

func estimateMapSize(m map[string]any) int {
	total := 0
	for k, v := range m {
		total += len(k)
		if s, ok := v.(string); ok {
			total += len(s)
		}
	}
	return total
}

func (card *CharacterCard) lorebookCount() int {
	if card.CharacterBook == nil {
		return 0
	}
	return len(card.CharacterBook.Entries)
}

func (card *CharacterCard) unknownFieldCount() int {
	return len(card.Preserved)
}

func (card *CharacterCard) specVersionString() string {
	switch card.SourceFormat {
	case FormatV2JSON, FormatV2PNG:
		return "2.0"
	case FormatV3JSON, FormatV3PNG, FormatV3CHARX:
		return "3.0"
	}
	return ""
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
