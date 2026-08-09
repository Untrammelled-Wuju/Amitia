// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package coordinator

import (
	"errors"

	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/internal/desktoppet/installation/settings"
)

type InstallRequest struct {
	DeviceCtx       device.DeviceContext
	PetID           string
	TargetReleaseID string
	SourceReleaseID string
	IdempotencyKey  string
	Metadata        map[string]string
}

func (r InstallRequest) Validate() error {
	if !r.DeviceCtx.IsValid() {
		return ErrInvalidInstallRequest
	}
	if r.TargetReleaseID == "" {
		return ErrInvalidInstallRequest
	}
	return nil
}

type InstallResult struct {
	OperationID    string
	InstallationID string
	Status         string
	Stage          string
	Retryable      bool
	ErrorCode      string
	ErrorMessage   string
}

type EnableDisableRequest struct {
	DeviceCtx      device.DeviceContext
	InstallationID string
	IdempotencyKey string
	Reason         string
}

func (r EnableDisableRequest) Validate() error {
	if !r.DeviceCtx.IsValid() || r.InstallationID == "" {
		return ErrInvalidEnableDisableRequest
	}
	return nil
}

type EnableDisableResult struct {
	OperationID     string
	DesiredRevision int64
	Status          string
	ErrorCode       string
}

type SwitchRequest struct {
	DeviceCtx            device.DeviceContext
	SourceInstallationID string
	TargetReleaseID      string
	IdempotencyKey       string
	Metadata             map[string]string
}

func (r SwitchRequest) Validate() error {
	if !r.DeviceCtx.IsValid() || r.SourceInstallationID == "" || r.TargetReleaseID == "" {
		return ErrInvalidSwitchRequest
	}
	return nil
}

type SwitchResult struct {
	OperationID     string
	InstallationID  string
	DesiredRevision int64
	Status          string
	ErrorCode       string
}

type UpgradeRequest struct {
	DeviceCtx       device.DeviceContext
	InstallationID  string
	TargetReleaseID string
	IdempotencyKey  string
}

func (r UpgradeRequest) Validate() error {
	if !r.DeviceCtx.IsValid() || r.InstallationID == "" || r.TargetReleaseID == "" {
		return ErrInvalidUpgradeRequest
	}
	return nil
}

type DowngradeRequest struct {
	DeviceCtx       device.DeviceContext
	InstallationID  string
	TargetReleaseID string
	IdempotencyKey  string
	SafetyConfirm   bool
}

func (r DowngradeRequest) Validate() error {
	if !r.DeviceCtx.IsValid() || r.InstallationID == "" || r.TargetReleaseID == "" {
		return ErrInvalidDowngradeRequest
	}
	return nil
}

type RepairRequest struct {
	DeviceCtx      device.DeviceContext
	InstallationID string
	IdempotencyKey string
}

func (r RepairRequest) Validate() error {
	if !r.DeviceCtx.IsValid() || r.InstallationID == "" {
		return ErrInvalidRepairRequest
	}
	return nil
}

type UninstallRequest struct {
	DeviceCtx      device.DeviceContext
	InstallationID string
	IdempotencyKey string
	Metadata       map[string]string
}

func (r UninstallRequest) Validate() error {
	if !r.DeviceCtx.IsValid() || r.InstallationID == "" {
		return ErrInvalidUninstallRequest
	}
	return nil
}

type UninstallResult struct {
	OperationID  string
	Status       string
	ErrorMessage string
}

type SettingsRequest struct {
	DeviceCtx        device.DeviceContext
	InstallationID   string
	ExpectedRevision int
	Updates          map[string]interface{}
	IdempotencyKey   string
}

func (r SettingsRequest) Validate() error {
	if !r.DeviceCtx.IsValid() || r.InstallationID == "" {
		return ErrInvalidSettingsRequest
	}
	return nil
}

type SettingsResult struct {
	OperationID      string
	SettingsRevision int
	Status           string
	ErrorCode        string
	ErrorMessage     string
	DesiredRevision  int64
}

type DefaultActionRequest struct {
	DeviceCtx        device.DeviceContext
	InstallationID   string
	DesiredActionKey string
	IdempotencyKey   string
}

func (r DefaultActionRequest) Validate() error {
	if !r.DeviceCtx.IsValid() || r.InstallationID == "" {
		return ErrInvalidDefaultActionRequest
	}
	return nil
}

type RecenterRequest struct {
	DeviceCtx      device.DeviceContext
	InstallationID string
	IdempotencyKey string
}

func (r RecenterRequest) Validate() error {
	if !r.DeviceCtx.IsValid() || r.InstallationID == "" {
		return ErrInvalidRecenterRequest
	}
	return nil
}

type OperationContext struct {
	DeviceCtx      device.DeviceContext
	InstallationID string
	OperationID    string
	ExecutionID    string
	IdempotencyKey string
}

func (ctx OperationContext) IsValid() bool {
	return ctx.DeviceCtx.IsValid() && ctx.InstallationID != "" && ctx.OperationID != ""
}

type ReleaseValidationResult struct {
	ReleaseID        string
	IsInstallable    bool
	HasStagingCopy   bool
	HasPublishedCopy bool
	ManifestValid    bool
	StagingPathKey   string
	PublishedPathKey string
	SettingsRevision int
	DesiredSettings  settings.SettingsSnapshot
	ErrorMessage     string
}

type CommitStageResult struct {
	JournalID       string
	NextStage       string
	DesiredRevision int64
	DesiredHash     string
	OperationStatus string
}

type SwitchStageResult struct {
	JournalID          string
	NextStage          string
	NewDesiredRevision int64
	OperationStatus    string
}

var (
	ErrInvalidInstallRequest       = errors.New("coordinator: invalid install request")
	ErrInvalidEnableDisableRequest = errors.New("coordinator: invalid enable/disable request")
	ErrInvalidSwitchRequest        = errors.New("coordinator: invalid switch request")
	ErrInvalidUpgradeRequest       = errors.New("coordinator: invalid upgrade request")
	ErrInvalidDowngradeRequest     = errors.New("coordinator: invalid downgrade request")
	ErrInvalidRepairRequest        = errors.New("coordinator: invalid repair request")
	ErrInvalidUninstallRequest     = errors.New("coordinator: invalid uninstall request")
	ErrInvalidSettingsRequest      = errors.New("coordinator: invalid settings request")
	ErrInvalidDefaultActionRequest = errors.New("coordinator: invalid default action request")
	ErrInvalidRecenterRequest      = errors.New("coordinator: invalid recenter request")
	ErrCoordinatorInternal         = errors.New("coordinator: internal error")
	ErrOwnershipMismatch           = errors.New("coordinator: ownership mismatch")
	ErrReleaseNotInstallable       = errors.New("coordinator: release not installable")
	ErrOperationFailedRetryable    = errors.New("coordinator: operation failed retryable")
	ErrOperationFailedTerminal     = errors.New("coordinator: operation failed terminal")
	ErrDesiredStateUpdateFailed    = errors.New("coordinator: desired state update failed")
	ErrBindingUpdateFailed         = errors.New("coordinator: binding update failed")
	ErrJournalUpdateFailed         = errors.New("coordinator: journal update failed")
)
