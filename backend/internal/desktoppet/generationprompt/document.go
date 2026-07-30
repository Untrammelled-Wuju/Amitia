// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package generationprompt

type PromptDocument struct {
	SchemaVersion          int
	Mode                   string
	CharacterIdentity      string
	ArtStyle               string
	CameraConstraint       string
	PoseConstraint         string
	MotionDescription      string
	ContinuityConstraint   string
	UserPrompt             string
	PromptFragment         string
	NegativePromptFragment string
	FramePhases            []FramePhaseInput
	GridLayout             GridLayoutInput
	BackgroundStrategy     string
	UserNegativePrompt     string
}

type FramePhaseInput struct {
	Index       int
	Description string
}

type GridLayoutInput struct {
	Rows         int
	Columns      int
	ReadingOrder string
	CellCount    int
}

type PromptSnapshot struct {
	TemplateVersion    string
	DocumentJSON       string
	FinalPrompt        string
	NegativePrompt     string
	PromptHash         string
	NegativePromptHash string
}
