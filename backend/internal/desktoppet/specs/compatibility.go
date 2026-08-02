// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

type LegacySpecOptions struct {
	ActionKey           string
	ActionName          string
	Description         string
	CategoryKey         string
	CategoryName        string
	FrameCount          int
	SupportsDefaultIdle bool
	SortOrder           int
	DefinitionVersion   int
	LoopType            string
	FPS                 int
	EstimatedGenCount   int
}

type CompatibilityResolver struct{}

func NewCompatibilityResolver() *CompatibilityResolver {
	return &CompatibilityResolver{}
}

func (c *CompatibilityResolver) ResolveLegacy(opts LegacySpecOptions) (contracts.ActionSpec, error) {
	if spec, ok := CatalogGet(opts.ActionKey); ok {
		return spec, nil
	}

	if strings.TrimSpace(opts.ActionKey) == "" {
		return contracts.ActionSpec{}, fmt.Errorf("action key is empty")
	}

	mode := MapLoopTypeToPlayback(opts.LoopType)

	fps := opts.FPS
	if fps == 0 {
		fps = 10
	}

	returnPolicy := MapReturnActionToPolicy("", mode)

	spec := contracts.ActionSpec{
		SchemaVersion: contracts.ActionSpecSchemaVersion,
		Identity: contracts.ActionIdentity{
			Key:                 opts.ActionKey,
			Name:                opts.ActionName,
			Description:         opts.Description,
			CategoryKey:         opts.CategoryKey,
			CategoryName:        opts.CategoryName,
			Source:              contracts.ActionSourceBuiltin,
			DefinitionVersion:   opts.DefinitionVersion,
			Enabled:             true,
			Recommended:         false,
			SupportsDefaultIdle: opts.SupportsDefaultIdle,
			ActionSortOrder:     opts.SortOrder,
			Tags:                []string{"legacy"},
		},
		Playback: contracts.ActionPlaybackSpec{
			Mode:          mode,
			DefaultFPS:    fps,
			ReturnPolicy:  returnPolicy,
			Interruptible: true,
			MutexGroup:    "legacy",
			QueuePolicy:   contracts.QueueReplace,
		},
		Processing: contracts.ActionProcessingHints{
			AnchorProfile:     contracts.AnchorFeetCenter,
			CheckLoopSeam:     mode == contracts.PlaybackLoop,
			PreserveLastFrame: false,
		},
	}

	if cat, ok := contracts.CategoryByKey(opts.CategoryKey); ok {
		spec.Identity.CategorySortOrder = cat.SortOrder
	}

	if opts.FrameCount > 0 {
		spec.Generation.FrameCount = opts.FrameCount
		spec.Generation.FramePhases = []contracts.FramePhase{}
		spec.Generation.Strategy = contracts.GenerationTypeSequential
	}

	return spec, nil
}

func (c *CompatibilityResolver) IsLegacySpec(spec contracts.ActionSpec) bool {
	for _, tag := range spec.Identity.Tags {
		if tag == "legacy" {
			return true
		}
	}
	return false
}

func MapLoopTypeToPlayback(loopType string) contracts.PlaybackMode {
	switch strings.ToLower(strings.TrimSpace(loopType)) {
	case "loop":
		return contracts.PlaybackLoop
	case "once":
		return contracts.PlaybackOnce
	default:
		return contracts.PlaybackOnce
	}
}

func MapReturnActionToPolicy(returnAction string, mode contracts.PlaybackMode) contracts.ReturnPolicy {
	if strings.TrimSpace(returnAction) != "" {
		return contracts.ReturnSpecific
	}
	if mode == contracts.PlaybackLoop {
		return contracts.ReturnNone
	}
	return contracts.ReturnPrevious
}
