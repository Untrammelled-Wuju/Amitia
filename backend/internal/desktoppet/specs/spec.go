// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

const (
	LoopTypeLoop = "loop"
	LoopTypeOnce = "once"
)

const FixedFrameCount = 12

const (
	StrategySequentialFrames = "sequential_frames"
)

type FramePhase struct {
	Index       int
	Description string
}

type ActionGenerationSpec struct {
	ActionKey              string
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

	phases := normalizePhases(spec.Generation.FramePhases, FixedFrameCount)

	return ActionGenerationSpec{
		ActionKey:              spec.Identity.Key,
		LoopType:               loopType,
		FrameCount:             FixedFrameCount,
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

func normalizePhases(phases []contracts.FramePhase, target int) []FramePhase {
	out := make([]FramePhase, target)
	if len(phases) <= 0 {
		for i := range out {
			out[i] = FramePhase{Index: i, Description: fmt.Sprintf("第%d帧，动作保持自然连贯过渡", i+1)}
		}
		return out
	}
	last := len(phases) - 1
	for i := 0; i < target; i++ {
		src := 0
		if target > 1 {
			src = i * last / (target - 1)
		}
		out[i] = FramePhase{Index: i, Description: phases[src].Description}
	}
	return out
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
