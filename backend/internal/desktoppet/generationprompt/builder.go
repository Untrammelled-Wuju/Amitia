// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package generationprompt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type templateFunc func(PromptDocument) string

func BuildSheetPrompt(doc PromptDocument) (PromptSnapshot, error) {
	return buildPrompt(doc, "sheet", buildSheetTemplate)
}

func BuildKeyframePrompt(doc PromptDocument) (PromptSnapshot, error) {
	return buildPrompt(doc, "keyframe", buildKeyframeTemplate)
}

func BuildSingleFramePrompt(doc PromptDocument) (PromptSnapshot, error) {
	return buildPrompt(doc, "single", buildSingleFrameTemplate)
}

func BuildLegacyFramePrompt(doc PromptDocument) (PromptSnapshot, error) {
	return buildPrompt(doc, "legacy", buildLegacyFrameTemplate)
}

func buildPrompt(doc PromptDocument, mode string, tmpl templateFunc) (PromptSnapshot, error) {
	doc.Mode = mode
	if doc.SchemaVersion == 0 {
		doc.SchemaVersion = 1
	}
	if err := ValidateUserPromptOverride(doc.UserPrompt); err != nil {
		return PromptSnapshot{}, err
	}
	docJSON, err := marshalDocument(doc)
	if err != nil {
		return PromptSnapshot{}, err
	}
	finalPrompt := assemblePrompt(doc, tmpl(doc))
	if err := ValidatePromptLength(finalPrompt); err != nil {
		return PromptSnapshot{}, err
	}
	negativePrompt := NormalizeNegativePrompt(doc)
	return PromptSnapshot{
		TemplateVersion:    TemplateVersion,
		DocumentJSON:       docJSON,
		FinalPrompt:        finalPrompt,
		NegativePrompt:     negativePrompt,
		PromptHash:         computeHash(finalPrompt),
		NegativePromptHash: computeHash(negativePrompt),
	}, nil
}

func assemblePrompt(doc PromptDocument, template string) string {
	parts := make([]string, 0, 12)
	parts = append(parts, template)
	if v := strings.TrimSpace(doc.PoseConstraint); v != "" {
		parts = append(parts, "动作约束："+v)
	}
	if v := strings.TrimSpace(doc.MotionDescription); v != "" {
		parts = append(parts, "动作描述："+v)
	}
	if v := strings.TrimSpace(doc.ContinuityConstraint); v != "" {
		parts = append(parts, "连续性约束："+v)
	}
	if len(doc.FramePhases) > 0 {
		if block := buildFramePhasesBlock(doc.FramePhases); block != "" {
			parts = append(parts, block)
		}
	}
	if v := strings.TrimSpace(doc.PromptFragment); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(doc.UserPrompt); v != "" {
		parts = append(parts, v)
	}
	parts = append(parts, "单张图片，不包含文字说明")
	return strings.Join(parts, "\n")
}

func buildFramePhasesBlock(phases []FramePhaseInput) string {
	parts := make([]string, 0, len(phases))
	for _, p := range phases {
		desc := strings.TrimSpace(p.Description)
		if desc == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("第%d格：%s", p.Index, desc))
	}
	if len(parts) == 0 {
		return ""
	}
	return "逐格动作：\n" + strings.Join(parts, "\n")
}

func marshalDocument(doc PromptDocument) (string, error) {
	data, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func computeHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
