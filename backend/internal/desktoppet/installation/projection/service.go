// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package projection

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) UpdateProjection(ctx context.Context, userID, deviceID string, updateFn func(*InstallationRuntimeProjection) error) error {
	var proj InstallationRuntimeProjection
	err := s.db.WithContext(ctx).Where("user_id = ? AND device_id = ?", userID, deviceID).Take(&proj).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			proj = InstallationRuntimeProjection{
				UserID:   userID,
				DeviceID: deviceID,
			}
		} else {
			return err
		}
	}
	if err := updateFn(&proj); err != nil {
		return err
	}
	proj.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return s.db.WithContext(ctx).Save(&proj).Error
}

func (s *Service) HandleRuntimeHeartbeat(ctx context.Context, userID, deviceID, runtimeID string, heartbeat *coordinator.RuntimeHeartbeat) error {
	return s.UpdateProjection(ctx, userID, deviceID, func(p *InstallationRuntimeProjection) error {
		p.RuntimeID = runtimeID
		p.AppliedDesiredRevision = heartbeat.AppliedDesiredRevision
		p.AppliedSettingsRevision = heartbeat.AppliedSettingsRevision
		p.ActualReleaseID = heartbeat.ActualReleaseID
		p.ActualVisible = heartbeat.ActualVisible
		p.ActualActionKey = heartbeat.ActualActionKey
		p.ActualHealth = heartbeat.ActualHealth
		if p.RuntimeSyncState == "" {
			p.RuntimeSyncState = SyncStateSyncing
		}
		if heartbeat.ActualHealth == "healthy" && p.AppliedDesiredRevision > 0 {
			p.RuntimeSyncState = SyncStateApplied
		}
		p.LastHeartbeatAt = heartbeat.Timestamp
		return nil
	})
}

func (s *Service) HandleCommandResult(ctx context.Context, userID, deviceID string, result *coordinator.CommandResult) error {
	return s.UpdateProjection(ctx, userID, deviceID, func(p *InstallationRuntimeProjection) error {
		if result.Success {
			p.AppliedDesiredRevision = result.AppliedRevision
			p.RuntimeSyncState = SyncStateApplied
			p.LastAppliedAt = result.Timestamp
		} else {
			p.RuntimeSyncState = SyncStateFailed
		}
		return nil
	})
}
