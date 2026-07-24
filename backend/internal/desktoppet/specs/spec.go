// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

const (
	LoopTypeLoop = "loop"
	LoopTypeOnce = "once"
)

const (
	StrategySequentialFrames = "sequential_frames"
)

type FramePhase struct {
	Index       int
	Description string
}

type ActionGenerationSpec struct {
	ActionKey               string
	LoopType               string
	FrameCount             int
	FramePhases            []FramePhase
	MotionDescription      string
	CameraConstraint       string
	PoseConstraint         string
	ContinuityConstraint   string
	PromptFragment         string
	NegativePromptFragment string
	GenerationStrategy     string
	Version                int
}
