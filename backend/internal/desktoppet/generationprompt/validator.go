// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package generationprompt

import (
	"fmt"
	"strings"
)

const MaxPromptCharacters = 8000

var layoutOverridePatterns = []string{
	"忽略布局",
	"覆盖网格",
	"不要网格",
	"取消布局",
	"无视布局",
	"忽略网格",
	"覆盖布局",
	"ignore layout",
	"override grid",
	"no grid",
}

func ValidateUserPromptOverride(userPrompt string) error {
	trimmed := strings.TrimSpace(userPrompt)
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)
	for _, pattern := range layoutOverridePatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return fmt.Errorf("user prompt must not contain layout override directive: %s", pattern)
		}
	}
	return nil
}

func ValidatePromptLength(prompt string) error {
	n := len([]rune(prompt))
	if n > MaxPromptCharacters {
		return fmt.Errorf("prompt length %d exceeds max %d", n, MaxPromptCharacters)
	}
	return nil
}

func NormalizeNegativePrompt(doc PromptDocument) string {
	items := make([]string, 0, 32)
	items = append(items, splitNegativeItems(defaultNegativeConstraint)...)
	items = append(items, splitNegativeItems(doc.NegativePromptFragment)...)
	items = append(items, splitNegativeItems(doc.UserNegativePrompt)...)
	seen := make(map[string]struct{}, len(items))
	deduped := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		deduped = append(deduped, item)
	}
	return strings.Join(deduped, "，")
}

func splitNegativeItems(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	raw := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，'
	})
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
