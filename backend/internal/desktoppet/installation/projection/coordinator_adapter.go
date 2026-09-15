// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package projection

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
)

type CoordinatorAdapter struct {
	service *Service
}

func NewCoordinatorAdapter(service *Service) *CoordinatorAdapter {
	return &CoordinatorAdapter{service: service}
}

func (a *CoordinatorAdapter) UpdateProjection(ctx context.Context, spaceID, deviceID string, updateFn func(*coordinator.Projection) error) error {
	return a.service.UpdateProjection(ctx, spaceID, deviceID, func(p *InstallationRuntimeProjection) error {
		view := &coordinator.Projection{
			InstallationID:         p.InstallationID,
			PetID:                  p.PetID,
			AppliedDesiredRevision: p.AppliedDesiredRevision,
			ActualReleaseID:        p.ActualReleaseID,
			RuntimeSyncState:       p.RuntimeSyncState,
		}
		if err := updateFn(view); err != nil {
			return err
		}
		p.InstallationID = view.InstallationID
		p.PetID = view.PetID
		p.AppliedDesiredRevision = view.AppliedDesiredRevision
		p.ActualReleaseID = view.ActualReleaseID
		p.RuntimeSyncState = view.RuntimeSyncState
		return nil
	})
}

func (a *CoordinatorAdapter) HandleRuntimeHeartbeat(ctx context.Context, spaceID, deviceID, runtimeID string, heartbeat *coordinator.RuntimeHeartbeat) error {
	return a.service.HandleRuntimeHeartbeat(ctx, spaceID, deviceID, runtimeID, heartbeat)
}

func (a *CoordinatorAdapter) HandleCommandResult(ctx context.Context, spaceID, deviceID string, result *coordinator.CommandResult) error {
	return a.service.HandleCommandResult(ctx, spaceID, deviceID, result)
}

var _ coordinator.ProjectionService = (*CoordinatorAdapter)(nil)
