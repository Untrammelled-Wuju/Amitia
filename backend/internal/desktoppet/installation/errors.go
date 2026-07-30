// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"errors"
	"fmt"
)

const (
	ErrCodeInstallationNotFound        = "INSTALLATION_NOT_FOUND"
	ErrCodeInstallationDuplicate       = "INSTALLATION_DUPLICATE"
	ErrCodeInstallationInvalid         = "INSTALLATION_INVALID"
	ErrCodeInstallationFailed          = "INSTALLATION_FAILED"
	ErrCodeRuntimeSettingsNotFound     = "RUNTIME_SETTINGS_NOT_FOUND"
	ErrCodePackageNotReady             = "PACKAGE_NOT_READY"
	ErrCodePackagePathTraversal        = "PACKAGE_PATH_TRAVERSAL"
	ErrCodePackageSymlinkEscape        = "PACKAGE_SYMLINK_ESCAPE"
	ErrCodePackageExecutableFound      = "PACKAGE_EXECUTABLE_FOUND"
	ErrCodePackageHashMismatch         = "PACKAGE_HASH_MISMATCH"
	ErrCodePackageDefaultActionInvalid = "PACKAGE_DEFAULT_ACTION_INVALID"
	ErrCodeCharacterNotFound           = "CHARACTER_NOT_FOUND"
	ErrCodePurgeNotConfirmed           = "PURGE_NOT_CONFIRMED"
	ErrCodeDefaultActionNotIdle        = "DEFAULT_ACTION_NOT_IDLE"
	ErrCodePetNotEnabled               = "PET_NOT_ENABLED"
	ErrCodeActionNotFound              = "ACTION_NOT_FOUND"
	ErrCodeRevisionConflict            = "REVISION_CONFLICT"
	ErrCodeRuntimeDeliveryFailed       = "RUNTIME_DELIVERY_FAILED"
	ErrCodePackageQualityGateBlocked   = "PACKAGE_QUALITY_GATE_BLOCKED"
)

var (
	ErrInstallationNotFound        = errors.New("installation not found")
	ErrInstallationDuplicate       = errors.New("installation already exists for the same package and version")
	ErrInstallationInvalid         = errors.New("installation is invalid for this operation")
	ErrInstallationFailed          = errors.New("installation failed")
	ErrRuntimeSettingsNotFound     = errors.New("runtime settings not found")
	ErrPackageNotReady             = errors.New("package is not ready for installation")
	ErrPackagePathTraversal        = errors.New("package contains path traversal entries")
	ErrPackageSymlinkEscape        = errors.New("package contains symlink escaping the package root")
	ErrPackageExecutableFound      = errors.New("package contains forbidden executable file")
	ErrPackageHashMismatch         = errors.New("package hash mismatch")
	ErrPackageDefaultActionInvalid = errors.New("package default action is invalid")
	ErrCharacterNotFound           = errors.New("character not found")
	ErrPurgeNotConfirmed           = errors.New("purge operation not confirmed")
	ErrDefaultActionNotIdle        = errors.New("default action does not support idle")
	ErrPetNotEnabled               = errors.New("pet is not enabled")
	ErrActionNotFound              = errors.New("action not found in manifest")
	ErrRevisionConflict            = errors.New("settings revision conflict")
	ErrPackageQualityGateBlocked   = errors.New("package quality gate blocked")
)

type InstallationError struct {
	Code    string
	Message string
	Err     error
}

func (e *InstallationError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *InstallationError) Unwrap() error { return e.Err }

func NewInstallationError(code, message string, err error) *InstallationError {
	return &InstallationError{Code: code, Message: message, Err: err}
}

type RevisionConflictError struct {
	Expected int
	Actual   int
}

func (e *RevisionConflictError) Error() string {
	return fmt.Sprintf("revision conflict: expected %d, actual %d", e.Expected, e.Actual)
}
