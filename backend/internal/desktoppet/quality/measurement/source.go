// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package measurement

import (
	"context"
	"fmt"
	"image"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type ActionRevisionMeasurementSource struct {
	inputRepo quality.QualityInputRepository
	engine    quality.ImageMeasurementEngine
}

func NewActionRevisionMeasurementSource(inputRepo quality.QualityInputRepository, engine quality.ImageMeasurementEngine) *ActionRevisionMeasurementSource {
	return &ActionRevisionMeasurementSource{
		inputRepo: inputRepo,
		engine:    engine,
	}
}

func (s *ActionRevisionMeasurementSource) LoadActionMeasurements(ctx context.Context, actionRevisionID string) (*quality.ActionMeasurementSet, error) {
	input, err := s.inputRepo.LoadActionRevisionInput(ctx, "", actionRevisionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load action input: %w", err)
	}
	if input == nil {
		return nil, quality.ErrActionRevisionNotFound
	}

	set, err := s.buildMeasurementSet(ctx, input)
	if err != nil {
		return nil, err
	}

	return set, nil
}

func (s *ActionRevisionMeasurementSource) OpenFrame(ctx context.Context, actionRevisionID string, frameIndex int) (image.Image, error) {
	input, err := s.inputRepo.LoadActionRevisionInput(ctx, "", actionRevisionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load action input: %w", err)
	}

	for _, frame := range input.Frames {
		if frame.FrameIndex == frameIndex && frame.AbsolutePath != "" {
			return nil, nil
		}
	}

	return nil, fmt.Errorf("frame not found: revision=%s index=%d", actionRevisionID, frameIndex)
}

func (s *ActionRevisionMeasurementSource) buildMeasurementSet(ctx context.Context, input *quality.QualityActionInput) (*quality.ActionMeasurementSet, error) {
	frameCount := len(input.Frames)
	if input.ExpectedFrameCount > 0 {
		frameCount = input.ExpectedFrameCount
	}

	measurements := s.createEmptyMeasurements(input.ActionRevisionID, input.ActionKey, frameCount)

	measurements.PlaybackMode = input.PlaybackMode
	measurements.LoopType = s.detectLoopType(input)

	for _, frame := range input.Frames {
		if frame.AbsolutePath == "" {
			continue
		}

		result, err := s.engine.MeasureFrame(ctx, frame.AbsolutePath, frame.ContentHash, frame.FrameArtifactID)
		if err != nil {
			continue
		}

		fm := frameMeasurementFromResult(frame.FrameIndex, frame, result)
		for i := range measurements.FrameMeasurements {
			if measurements.FrameMeasurements[i].FrameIndex == frame.FrameIndex {
				measurements.FrameMeasurements[i] = fm
				break
			}
		}
	}

	measurements.RevisionHash = computeActionRevisionHash(input)

	return measurements, nil
}

func (s *ActionRevisionMeasurementSource) createEmptyMeasurements(actionRevisionID, actionKey string, frameCount int) *quality.ActionMeasurementSet {
	frames := make([]quality.FrameMeasurement, frameCount)
	for i := 0; i < frameCount; i++ {
		frames[i] = quality.FrameMeasurement{
			FrameIndex: i,
			Decodable:  false,
			FileExists: false,
		}
	}

	return &quality.ActionMeasurementSet{
		ActionRevisionID:  actionRevisionID,
		ActionKey:         actionKey,
		FrameCount:        frameCount,
		FrameMeasurements: frames,
	}
}

func (s *ActionRevisionMeasurementSource) detectLoopType(input *quality.QualityActionInput) string {
	if input.PlaybackMode == "pingpong" {
		return "pingpong"
	}
	if input.PlaybackMode == "oneshot" {
		return "oneshot"
	}
	return "loop"
}

func frameMeasurementFromResult(frameIndex int, frame quality.QualityFrameInput, result *quality.FrameMeasurementResult) quality.FrameMeasurement {
	fm := quality.FrameMeasurement{
		FrameIndex:    frameIndex,
		FilePath:      frame.AbsolutePath,
		FileHash:      result.FileHash,
		PixelHash:     result.PixelHash,
		Width:         result.Width,
		Height:        result.Height,
		HasAlpha:      result.HasAlphaChannel,
		AlphaCoverage: result.AlphaCoverage,
		Decodable:     result.Decodable,
		MimeType:      result.MimeType,
	}

	if result.Decodable && result.Width > 0 {
		fm.FileExists = true
		fm.HasAlphaChannel = result.HasAlphaChannel
		fm.FullyTransparentRatio = result.FullyTransparentRatio
		fm.SemiTransparentRatio = result.SemiTransparentRatio
		fm.OpaqueRatio = result.OpaqueRatio
	}

	return fm
}

func computeActionRevisionHash(input *quality.QualityActionInput) string {
	hash := input.ActionContentHash
	for _, frame := range input.Frames {
		hash += frame.ContentHash
	}
	return hash
}
