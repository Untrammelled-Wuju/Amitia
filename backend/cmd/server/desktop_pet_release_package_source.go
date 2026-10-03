// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/release"
)

type generatedReleasePackageSource struct {
	processing processing.Service
}

func newGeneratedReleasePackageSource(svc processing.Service) release.GeneratedPackageSource {
	return &generatedReleasePackageSource{processing: svc}
}

func (s *generatedReleasePackageSource) CheckProcessingTaskOwnership(
	ctx context.Context,
	spaceID, processingTaskID string,
) error {
	_ = ctx
	return s.processing.CheckProcessingTaskOwnership(processingTaskID, spaceID)
}

func (s *generatedReleasePackageSource) BuildGeneratedPackage(
	ctx context.Context,
	spaceID, processingTaskID, defaultAction string,
	includedActions []string,
) (*release.GeneratedPackageSourceResult, error) {
	_ = ctx
	if err := s.processing.CheckProcessingTaskOwnership(processingTaskID, spaceID); err != nil {
		return nil, err
	}
	result, err := s.processing.BuildReleasePackageSource(&processing.CreatePackageRequest{
		ProcessingTaskID: processingTaskID,
		SpaceID:          spaceID,
		DefaultAction:    defaultAction,
		IncludedActions:  includedActions,
	})
	if err != nil {
		return nil, err
	}
	return &release.GeneratedPackageSourceResult{
		PackageID:    result.PackageID,
		PackageHash:  result.PackageHash,
		PackageDir:   result.PackageDir,
		ManifestData: result.ManifestData,
		Ephemeral:    true,
	}, nil
}
