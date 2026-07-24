// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/specs"
	"github.com/u-ai/backend/internal/imageprovider"
)

const baseConsistencyConstraint = "保持参考图同一角色，脸部特征一致，发型发色一致，服装配饰一致，身体比例一致，绘画风格一致，固定镜头角度，固定角色缩放，角色完整可见，角色位于画布中心附近，不增加无关人物，不增加文字签名水印，不改变角色身份"

const defaultNegativeConstraint = "多余人物，多余肢体，手脚数量异常，脸部变形，服装变化，发型变化，角色裁切，角色超出画布，视角突变，比例突变，背景复杂，文字，水印，标志，分镜边框，漫画格，多个重复角色"

func BuildFramePrompt(spec specs.ActionGenerationSpec, frameIndex int, userPrompt string) string {
	parts := make([]string, 0, 8)
	parts = append(parts, baseConsistencyConstraint)
	if trimmed := strings.TrimSpace(userPrompt); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(spec.PromptFragment); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if phaseDesc := selectFramePhaseDescription(spec.FramePhases, frameIndex); phaseDesc != "" {
		parts = append(parts, phaseDesc)
	}
	if trimmed := strings.TrimSpace(spec.CameraConstraint); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(spec.PoseConstraint); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(spec.ContinuityConstraint); trimmed != "" {
		parts = append(parts, trimmed)
	}
	parts = append(parts, "单张图片，不包含文字说明")
	return strings.Join(parts, "\n")
}

func selectFramePhaseDescription(phases []specs.FramePhase, frameIndex int) string {
	if len(phases) == 0 {
		return ""
	}
	if frameIndex < 0 || frameIndex >= len(phases) {
		return strings.TrimSpace(phases[len(phases)-1].Description)
	}
	return strings.TrimSpace(phases[frameIndex].Description)
}

func BuildNegativePrompt(spec specs.ActionGenerationSpec, userNegativePrompt string) string {
	items := make([]string, 0, 32)
	items = append(items, splitAndTrim(defaultNegativeConstraint)...)
	items = append(items, splitAndTrim(spec.NegativePromptFragment)...)
	items = append(items, splitAndTrim(userNegativePrompt)...)
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

func splitAndTrim(value string) []string {
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

func SelectReferenceImages(sourceImagePath string, previousFramePath string, supportsMultiple bool) []imageprovider.ImageInput {
	inputs := make([]imageprovider.ImageInput, 0, 2)
	if sourceImagePath == "" {
		return inputs
	}
	if src, ok := readImageInput(sourceImagePath); ok {
		inputs = append(inputs, src)
	}
	if !supportsMultiple {
		return inputs
	}
	if strings.TrimSpace(previousFramePath) == "" || previousFramePath == sourceImagePath {
		return inputs
	}
	if prev, ok := readImageInput(previousFramePath); ok {
		inputs = append(inputs, prev)
	}
	return inputs
}

func readImageInput(path string) (imageprovider.ImageInput, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return imageprovider.ImageInput{}, false
	}
	return imageprovider.ImageInput{
		Path:     path,
		Bytes:    data,
		MimeType: inferImageMime(path),
	}, true
}

func inferImageMime(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}
