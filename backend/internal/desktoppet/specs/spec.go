// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

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

func fromContracts(spec contracts.ActionSpec) ActionGenerationSpec {
	loopType := LoopTypeOnce
	if spec.Playback.Mode == contracts.PlaybackLoop || spec.Playback.Mode == contracts.PlaybackPingPong {
		loopType = LoopTypeLoop
	}

	phases := make([]FramePhase, len(spec.Generation.FramePhases))
	for i, p := range spec.Generation.FramePhases {
		phases[i] = FramePhase{Index: p.Index, Description: p.Description}
	}

	return ActionGenerationSpec{
		ActionKey:               spec.Identity.Key,
		LoopType:               loopType,
		FrameCount:             spec.Generation.FrameCount,
		FramePhases:            phases,
		MotionDescription:      spec.Generation.MotionDescription,
		CameraConstraint:       spec.Generation.CameraConstraint,
		PoseConstraint:         spec.Generation.PoseConstraint,
		ContinuityConstraint:   spec.Generation.ContinuityConstraint,
		PromptFragment:         spec.Generation.PromptFragment,
		NegativePromptFragment: spec.Generation.NegativePromptFragment,
		GenerationStrategy:     spec.Generation.Strategy,
		Version:                spec.Generation.Version,
	}
}

func SpecFromJSON(jsonStr string) (ActionGenerationSpec, bool) {
	if jsonStr == "" {
		return ActionGenerationSpec{}, false
	}
	var cs contracts.ActionSpec
	if err := json.Unmarshal([]byte(jsonStr), &cs); err != nil {
		return ActionGenerationSpec{}, false
	}
	return fromContracts(cs), true
}
