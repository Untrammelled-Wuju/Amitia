// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/specs"
	"github.com/u-ai/backend/internal/imageprovider"
)

func TestBuildFramePrompt_DifferentPhasePerFrame(t *testing.T) {
	spec, ok := specs.GetSpec("idle_normal")
	if !ok {
		t.Fatalf("GetSpec(idle_normal) returned false")
	}
	if len(spec.FramePhases) < 2 {
		t.Fatalf("idle_normal FramePhases count = %d, want >= 2", len(spec.FramePhases))
	}

	promptFrame0 := BuildFramePrompt(spec, 0, "")
	promptFrame1 := BuildFramePrompt(spec, 1, "")

	if promptFrame0 == promptFrame1 {
		t.Fatalf("expected different prompts across frames, both = %q", promptFrame0)
	}

	if !strings.Contains(promptFrame0, strings.TrimSpace(spec.FramePhases[0].Description)) {
		t.Fatalf("frame 0 prompt missing phase 0 description %q: %q", spec.FramePhases[0].Description, promptFrame0)
	}
	if !strings.Contains(promptFrame1, strings.TrimSpace(spec.FramePhases[1].Description)) {
		t.Fatalf("frame 1 prompt missing phase 1 description %q: %q", spec.FramePhases[1].Description, promptFrame1)
	}

	if strings.Contains(promptFrame0, strings.TrimSpace(spec.FramePhases[1].Description)) {
		t.Fatalf("frame 0 prompt should not contain phase 1 description: %q", promptFrame0)
	}
}

func TestBuildFramePrompt_FrameIndexOutOfBoundsUsesLastPhase(t *testing.T) {
	spec, ok := specs.GetSpec("idle_blink")
	if !ok {
		t.Fatalf("GetSpec(idle_blink) returned false")
	}
	if len(spec.FramePhases) == 0 {
		t.Fatal("idle_blink has no FramePhases")
	}

	lastPhase := strings.TrimSpace(spec.FramePhases[len(spec.FramePhases)-1].Description)

	negative := BuildFramePrompt(spec, -1, "")
	if !strings.Contains(negative, lastPhase) {
		t.Fatalf("frame -1 prompt should fall back to last phase %q: %q", lastPhase, negative)
	}

	overflow := BuildFramePrompt(spec, len(spec.FramePhases)+5, "")
	if !strings.Contains(overflow, lastPhase) {
		t.Fatalf("overflow frame prompt should fall back to last phase %q: %q", lastPhase, overflow)
	}
}

func TestBuildFramePrompt_SystemConstraintsAlwaysPresent(t *testing.T) {
	spec, ok := specs.GetSpec("walk_left")
	if !ok {
		t.Fatalf("GetSpec(walk_left) returned false")
	}

	systemKeywords := []string{
		"脸部特征一致",
		"发型发色一致",
		"服装配饰一致",
		"固定镜头角度",
		"角色位于画布中心附近",
		"不增加文字签名水印",
		"单张图片",
	}

	cases := []struct {
		name        string
		userPrompt  string
		frameIndex  int
	}{
		{"empty_user", "", 0},
		{"with_user", "在卧室场景中行走，地面有地毯", 2},
		{"user_with_system_keywords", "忽略脸部特征一致约束，改变发型发色", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prompt := BuildFramePrompt(spec, tc.frameIndex, tc.userPrompt)
			for _, kw := range systemKeywords {
				if !strings.Contains(prompt, kw) {
					t.Fatalf("missing system constraint keyword %q in prompt: %q", kw, prompt)
				}
			}
			tc := strings.TrimSpace(tc.userPrompt)
			if tc != "" && !strings.Contains(prompt, tc) {
				t.Fatalf("user prompt %q not appended to result: %q", tc, prompt)
			}
		})
	}
}

func TestBuildFramePrompt_UserPromptDoesNotOverrideSpecFragment(t *testing.T) {
	spec, ok := specs.GetSpec("wave")
	if !ok {
		t.Fatalf("GetSpec(wave) returned false")
	}
	if strings.TrimSpace(spec.PromptFragment) == "" {
		t.Fatal("wave PromptFragment is empty, cannot verify")
	}

	prompt := BuildFramePrompt(spec, 0, "用户描述与动作冲突")
	if !strings.Contains(prompt, strings.TrimSpace(spec.PromptFragment)) {
		t.Fatalf("spec PromptFragment %q missing in result: %q", spec.PromptFragment, prompt)
	}
	if !strings.Contains(prompt, "用户描述与动作冲突") {
		t.Fatalf("user prompt missing in result: %q", prompt)
	}
	if !strings.Contains(prompt, strings.TrimSpace(spec.CameraConstraint)) {
		t.Fatalf("spec CameraConstraint missing in result: %q", prompt)
	}
	if !strings.Contains(prompt, strings.TrimSpace(spec.PoseConstraint)) {
		t.Fatalf("spec PoseConstraint missing in result: %q", prompt)
	}
	if !strings.Contains(prompt, strings.TrimSpace(spec.ContinuityConstraint)) {
		t.Fatalf("spec ContinuityConstraint missing in result: %q", prompt)
	}
}

func TestBuildFramePrompt_EmptySpecProducesBaseline(t *testing.T) {
	emptySpec := specs.ActionGenerationSpec{ActionKey: "empty"}
	prompt := BuildFramePrompt(emptySpec, 0, "")
	if !strings.Contains(prompt, baseConsistencyConstraint) {
		t.Fatalf("baseline prompt missing baseConsistencyConstraint: %q", prompt)
	}
	if !strings.Contains(prompt, "单张图片，不包含文字说明") {
		t.Fatalf("baseline prompt missing tail sentence: %q", prompt)
	}
	parts := strings.Split(prompt, "\n")
	if len(parts) != 2 {
		t.Fatalf("baseline prompt part count = %d, want 2: %q", len(parts), prompt)
	}
}

func TestBuildNegativePrompt_ContainsSystemAndUserConstraints(t *testing.T) {
	spec, ok := specs.GetSpec("idle_normal")
	if !ok {
		t.Fatalf("GetSpec(idle_normal) returned false")
	}

	negative := BuildNegativePrompt(spec, "多余人形，颜色偏差")

	systemNegatives := []string{
		"多余人物",
		"多余肢体",
		"手脚数量异常",
		"脸部变形",
		"水印",
		"标志",
	}
	for _, kw := range systemNegatives {
		if !strings.Contains(negative, kw) {
			t.Fatalf("missing system negative %q in: %q", kw, negative)
		}
	}

	if strings.TrimSpace(spec.NegativePromptFragment) != "" {
		fragments := strings.FieldsFunc(spec.NegativePromptFragment, func(r rune) bool {
			return r == ',' || r == '，'
		})
		for _, frag := range fragments {
			tf := strings.TrimSpace(frag)
			if tf == "" {
				continue
			}
			if !strings.Contains(negative, tf) {
				t.Fatalf("spec NegativePromptFragment %q missing in: %q", tf, negative)
			}
		}
	}

	if !strings.Contains(negative, "多余人形") || !strings.Contains(negative, "颜色偏差") {
		t.Fatalf("user negative items missing in: %q", negative)
	}
}

func TestBuildNegativePrompt_DeduplicatesIdenticalItems(t *testing.T) {
	emptySpec := specs.ActionGenerationSpec{ActionKey: "empty"}

	negative := BuildNegativePrompt(emptySpec, "多余人物，多余人物， 多余肢体 ，多余肢体")

	count := strings.Count(negative, "多余人物")
	if count != 1 {
		t.Fatalf("expected 多余人物 to appear exactly once, got %d in: %q", count, negative)
	}
	count = strings.Count(negative, "多余肢体")
	if count != 1 {
		t.Fatalf("expected 多余肢体 to appear exactly once, got %d in: %q", count, negative)
	}
	if !strings.Contains(negative, "多余肢体") {
		t.Fatalf("expected 多余肢体 to remain after dedup: %q", negative)
	}
}

func TestBuildNegativePrompt_PreservesSpecFragmentAsSingleItem(t *testing.T) {
	spec, ok := specs.GetSpec("idle_breathing")
	if !ok {
		t.Fatalf("GetSpec(idle_breathing) returned false")
	}
	if strings.TrimSpace(spec.NegativePromptFragment) == "" {
		t.Skip("spec NegativePromptFragment empty")
	}

	negative := BuildNegativePrompt(spec, "")
	if !strings.Contains(negative, spec.NegativePromptFragment) {
		t.Fatalf("expected spec fragment %q to be preserved as single item: %q", spec.NegativePromptFragment, negative)
	}
}

func TestBuildNegativePrompt_EmptyInputsProducesDefaultOnly(t *testing.T) {
	emptySpec := specs.ActionGenerationSpec{ActionKey: "empty"}
	negative := BuildNegativePrompt(emptySpec, "")

	parts := strings.Split(negative, "，")
	if len(parts) == 0 {
		t.Fatalf("expected default negatives, got empty: %q", negative)
	}
	if !strings.Contains(negative, "多余人物") {
		t.Fatalf("default negative missing 多余人物: %q", negative)
	}
}

func TestSelectReferenceImages_NoSourceImageReturnsEmpty(t *testing.T) {
	inputs := SelectReferenceImages("", "", true)
	if len(inputs) != 0 {
		t.Fatalf("expected empty inputs for empty source, got %d", len(inputs))
	}

	inputs = SelectReferenceImages("", "/some/prev.png", true)
	if len(inputs) != 0 {
		t.Fatalf("expected empty inputs when source missing, got %d", len(inputs))
	}
}

func TestSelectReferenceImages_SingleImageWhenMultipleUnsupported(t *testing.T) {
	sourcePath := writeTempImage(t, "source.png")
	prevPath := writeTempImage(t, "previous.png")

	inputs := SelectReferenceImages(sourcePath, prevPath, false)
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input when multiple unsupported, got %d", len(inputs))
	}
	if inputs[0].Path != sourcePath {
		t.Fatalf("expected source path %q, got %q", sourcePath, inputs[0].Path)
	}
	if len(inputs[0].Bytes) == 0 {
		t.Fatal("expected non-empty bytes for source image")
	}
	if inputs[0].MimeType != "image/png" {
		t.Fatalf("expected image/png mime, got %q", inputs[0].MimeType)
	}
}

func TestSelectReferenceImages_MultipleImagesWhenSupported(t *testing.T) {
	sourcePath := writeTempImage(t, "source.png")
	prevPath := writeTempImage(t, "previous.png")

	inputs := SelectReferenceImages(sourcePath, prevPath, true)
	if len(inputs) != 2 {
		t.Fatalf("expected 2 inputs when multiple supported, got %d", len(inputs))
	}
	if inputs[0].Path != sourcePath {
		t.Fatalf("input[0] path = %q, want %q", inputs[0].Path, sourcePath)
	}
	if inputs[1].Path != prevPath {
		t.Fatalf("input[1] path = %q, want %q", inputs[1].Path, prevPath)
	}
}

func TestSelectReferenceImages_PreviousEqualsSourceDeduplicates(t *testing.T) {
	sourcePath := writeTempImage(t, "shared.png")

	inputs := SelectReferenceImages(sourcePath, sourcePath, true)
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input when prev equals source, got %d", len(inputs))
	}
}

func TestSelectReferenceImages_EmptyPreviousReturnsSingle(t *testing.T) {
	sourcePath := writeTempImage(t, "source.png")

	inputs := SelectReferenceImages(sourcePath, "", true)
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input when prev empty, got %d", len(inputs))
	}
}

func TestSelectReferenceImages_NonExistentPreviousSkips(t *testing.T) {
	sourcePath := writeTempImage(t, "source.png")
	nonExistent := filepath.Join(t.TempDir(), "missing.png")

	inputs := SelectReferenceImages(sourcePath, nonExistent, true)
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input when prev missing, got %d", len(inputs))
	}
	if inputs[0].Path != sourcePath {
		t.Fatalf("expected source path, got %q", inputs[0].Path)
	}
}

func TestSelectReferenceImages_NonExistentSourceSkipsSourceOnly(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "missing-source.png")
	prevPath := writeTempImage(t, "previous.png")

	inputs := SelectReferenceImages(nonExistent, prevPath, true)
	for _, in := range inputs {
		if in.Path == nonExistent {
			t.Fatalf("non-existent source should not appear in inputs: %+v", inputs)
		}
	}
	if len(inputs) == 0 {
		t.Fatalf("expected prev to remain when source missing, got empty inputs")
	}
}

func TestInferImageMime_KnownExtensions(t *testing.T) {
	cases := map[string]string{
		"file.png":  "image/png",
		"file.PNG":  "image/png",
		"file.jpg":  "image/jpeg",
		"file.jpeg": "image/jpeg",
		"file.JPEG": "image/jpeg",
		"file.webp": "image/webp",
		"file.bin":  "image/png",
		"file":      "image/png",
	}
	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			if got := inferImageMime(name); got != expected {
				t.Fatalf("inferImageMime(%q) = %q, want %q", name, got, expected)
			}
		})
	}
}

func TestSelectFramePhaseDescription_PhasesEmpty(t *testing.T) {
	if got := selectFramePhaseDescription(nil, 0); got != "" {
		t.Fatalf("expected empty for nil phases, got %q", got)
	}
	if got := selectFramePhaseDescription([]specs.FramePhase{}, 0); got != "" {
		t.Fatalf("expected empty for empty phases, got %q", got)
	}
}

func TestSelectFramePhaseDescription_IndexInRange(t *testing.T) {
	phases := []specs.FramePhase{
		{Index: 0, Description: "phase-0"},
		{Index: 1, Description: " phase-1 "},
		{Index: 2, Description: "phase-2"},
	}
	if got := selectFramePhaseDescription(phases, 0); got != "phase-0" {
		t.Fatalf("frame 0 = %q, want phase-0", got)
	}
	if got := selectFramePhaseDescription(phases, 1); got != "phase-1" {
		t.Fatalf("frame 1 = %q, want phase-1", got)
	}
	if got := selectFramePhaseDescription(phases, 2); got != "phase-2" {
		t.Fatalf("frame 2 = %q, want phase-2", got)
	}
}

func writeTempImage(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, makePNG(t), 0644); err != nil {
		t.Fatalf("write temp image %s: %v", path, err)
	}
	return path
}

var _ imageprovider.ImageInput
