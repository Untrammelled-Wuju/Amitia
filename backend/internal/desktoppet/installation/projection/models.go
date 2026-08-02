// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package projection

import (
	"errors"
)

var (
	ErrProjectionConflict = errors.New("projection: state conflict")
)

const (
	SyncStatePending  = "pending"
	SyncStateOffline  = "offline"
	SyncStateSyncing  = "syncing"
	SyncStateApplied  = "applied"
	SyncStateDegraded = "degraded"
	SyncStateFailed   = "failed"
)

type InstallationRuntimeProjection struct {
	ID string

	UserID    string
	DeviceID  string
	RuntimeID string

	InstallationID string
	PetID          string

	AppliedDesiredRevision  int64
	AppliedSettingsRevision int64
	ActualReleaseID         string
	ActualVisible           int
	ActualActionKey         string
	ActualHealth            string
	RuntimeSyncState        string

	LastAppliedAt   string
	LastHeartbeatAt string

	CreatedAt string
	UpdatedAt string
}

func (InstallationRuntimeProjection) TableName() string {
	return "desktop_pet_installation_runtime_projections"
}

func (p InstallationRuntimeProjection) IsApplied(desiredRevision int64) bool {
	return p.AppliedDesiredRevision >= desiredRevision &&
		p.RuntimeSyncState == SyncStateApplied
}

func (p InstallationRuntimeProjection) IsRuntimeHealthy() bool {
	return p.RuntimeSyncState == SyncStateOffline ||
		p.RuntimeSyncState == SyncStateSyncing ||
		p.RuntimeSyncState == SyncStateApplied
}

type ProjectionDelta struct {
	PreviousState   string
	NextState       string
	DesiredRevision int64
	AppliedRevision int64
	EventSource     string
	EventID         string
}
