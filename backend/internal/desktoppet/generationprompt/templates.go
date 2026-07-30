// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package generationprompt

import (
	"fmt"
	"strings"
)

const TemplateVersion = "1.0"

const defaultConsistencyConstraint = "保持参考图同一角色，脸部特征一致，发型发色一致，服装配饰一致，身体比例一致，绘画风格一致，固定镜头角度，固定角色缩放，角色完整可见，角色位于画布中心附近，不增加无关人物，不增加文字签名水印，不改变角色身份"

const defaultNegativeConstraint = "多余人物，多余肢体，手脚数量异常，脸部变形，服装变化，发型变化，角色裁切，角色超出画布，视角突变，比例突变，背景复杂，文字，水印，标志，分镜边框，漫画格，多个重复角色"

const (
	spriteSheetHeader = "Sprite Sheet 动画表生成"
	keyframeHeader    = "关键帧网格生成"
	singleFrameHeader = "单帧修复生成"
	legacyFrameHeader = "逐帧动画生成"
)

func buildIdentityBlock(doc PromptDocument) string {
	parts := make([]string, 0, 4)
	if v := strings.TrimSpace(doc.CharacterIdentity); v != "" {
		parts = append(parts, "角色身份："+v)
	}
	if v := strings.TrimSpace(doc.ArtStyle); v != "" {
		parts = append(parts, "画风："+v)
	}
	return strings.Join(parts, "\n")
}

func buildCameraBlock(doc PromptDocument) string {
	parts := make([]string, 0, 2)
	parts = append(parts, "固定相机，固定比例")
	if v := strings.TrimSpace(doc.CameraConstraint); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, "，")
}

func buildBackgroundBlock(doc PromptDocument) string {
	bg := strings.TrimSpace(doc.BackgroundStrategy)
	if bg == "" {
		bg = "透明背景"
	}
	return "背景：" + bg
}

func buildGridLayoutBlock(doc PromptDocument) string {
	g := doc.GridLayout
	if g.Rows <= 0 {
		g.Rows = 1
	}
	if g.Columns <= 0 {
		g.Columns = 1
	}
	if g.CellCount <= 0 {
		g.CellCount = g.Rows * g.Columns
	}
	readingOrder := strings.TrimSpace(g.ReadingOrder)
	if readingOrder == "" {
		readingOrder = "从左到右，从上到下"
	}
	return fmt.Sprintf("网格布局：%d行%d列，共%d格，阅读顺序：%s", g.Rows, g.Columns, g.CellCount, readingOrder)
}

func buildSpriteSheetRules() string {
	return strings.Join([]string{
		"每格只出现一个完整角色，角色完整可见，位于格内中心",
		"相邻格表示动作的连续进度，动作过渡自然",
		"禁止边框文字，禁止跨格重叠，禁止文字水印",
	}, "\n")
}

func buildSheetTemplate(doc PromptDocument) string {
	parts := make([]string, 0, 12)
	parts = append(parts, spriteSheetHeader)
	if b := buildIdentityBlock(doc); b != "" {
		parts = append(parts, b)
	}
	parts = append(parts, defaultConsistencyConstraint)
	parts = append(parts, buildCameraBlock(doc))
	parts = append(parts, buildBackgroundBlock(doc))
	parts = append(parts, buildGridLayoutBlock(doc))
	parts = append(parts, buildSpriteSheetRules())
	return strings.Join(parts, "\n")
}

func buildKeyframeTemplate(doc PromptDocument) string {
	parts := make([]string, 0, 12)
	parts = append(parts, keyframeHeader)
	if b := buildIdentityBlock(doc); b != "" {
		parts = append(parts, b)
	}
	parts = append(parts, defaultConsistencyConstraint)
	parts = append(parts, buildCameraBlock(doc))
	parts = append(parts, buildBackgroundBlock(doc))
	parts = append(parts, buildGridLayoutBlock(doc))
	parts = append(parts, "少量关键帧，每格为动作关键节点")
	parts = append(parts, buildSpriteSheetRules())
	return strings.Join(parts, "\n")
}

func buildSingleFrameTemplate(doc PromptDocument) string {
	parts := make([]string, 0, 10)
	parts = append(parts, singleFrameHeader)
	if b := buildIdentityBlock(doc); b != "" {
		parts = append(parts, b)
	}
	parts = append(parts, defaultConsistencyConstraint)
	parts = append(parts, buildCameraBlock(doc))
	parts = append(parts, buildBackgroundBlock(doc))
	parts = append(parts, "单帧修复，保持与参考帧一致")
	return strings.Join(parts, "\n")
}

func buildLegacyFrameTemplate(doc PromptDocument) string {
	parts := make([]string, 0, 10)
	parts = append(parts, legacyFrameHeader)
	if b := buildIdentityBlock(doc); b != "" {
		parts = append(parts, b)
	}
	parts = append(parts, defaultConsistencyConstraint)
	parts = append(parts, buildCameraBlock(doc))
	parts = append(parts, buildBackgroundBlock(doc))
	parts = append(parts, "逐帧动画，每张为独立帧")
	return strings.Join(parts, "\n")
}
