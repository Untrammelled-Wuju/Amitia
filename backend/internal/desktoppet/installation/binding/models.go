// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package binding

import (
	"errors"
)

var (
	ErrBindingAlreadyExists = errors.New("binding: device already has an active installation")
	ErrBindingNotFound      = errors.New("binding: no active installation for device")
	ErrBindingConflict      = errors.New("binding: installation already bound to another device")
)

type DeviceActiveInstallationBinding struct {
	UserID   string
	DeviceID string

	InstallationID string
	PetID          string
	ReleaseID      string

	BindingRevision int64

	BoundReason string
	BoundAt     string
	BoundBy     string

	CreatedAt string
	UpdatedAt string
}

func (DeviceActiveInstallationBinding) TableName() string {
	return "desktop_pet_device_active_installation_bindings"
}

func (b DeviceActiveInstallationBinding) IsValid() bool {
	return b.UserID != "" && b.DeviceID != "" && b.InstallationID != ""
}

type BindingHistoryEntry struct {
	ID string

	UserID   string
	DeviceID string

	PreviousInstallationID string
	NewInstallationID      string

	BindingRevision int64

	Reason      string
	Actor       string
	OperationID string

	OccurredAt string
}

func (BindingHistoryEntry) TableName() string {
	return "desktop_pet_device_installation_binding_history"
}

const (
	BoundReasonInstall = "install_bound"
	BoundReasonEnable  = "enable_bound"
	BoundReasonSwitch  = "switch_bound"
	BoundReasonRestore = "restore_bound"
)

type BindingConflictResolution struct {
	ExistingBinding       DeviceActiveInstallationBinding
	RequestedInstallation string
	Strategy              ConflictResolutionStrategy
}

type ConflictResolutionStrategy int

const (
	StrategyReject ConflictResolutionStrategy = iota
	StrategyReplace
	StrategyRejectIfRunning
)
