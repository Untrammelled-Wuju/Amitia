// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"errors"
	"fmt"
)

var (
	ErrAuthFailOpen          = errors.New("security: auth fail-open detected")
	ErrInvalidToken          = errors.New("security: invalid token")
	ErrTokenExpired          = errors.New("security: token expired")
	ErrNoSecretConfigured    = errors.New("security: JWT secret not configured")
	ErrLocalTokenMismatch    = errors.New("security: local token mismatch")
	ErrSecurityMisconfigured = errors.New("security: security configuration invalid")
	ErrUnsafePath            = errors.New("security: unsafe file path detected")
	ErrPathEscape            = errors.New("security: path escape detected")
	ErrRootDelete            = errors.New("security: root directory cannot be deleted")
	ErrStagingCrossUser      = errors.New("security: cross-user import staging access")
	ErrStagingConsumed       = errors.New("security: import staging already consumed")
	ErrStagingExpired        = errors.New("security: import staging expired")
	ErrRevisionConflict      = errors.New("security: revision ownership conflict")
	ErrJobConflict           = errors.New("security: job ownership conflict")
	ErrCandidateConflict     = errors.New("security: candidate ownership conflict")
	ErrResourceOrphan        = errors.New("security: resource ownership broken")
	ErrLegacyWriteBlocked    = errors.New("security: legacy write blocked after migration")
	ErrMigrationLockFailed   = errors.New("security: migration lock acquisition failed")
	ErrForbiddenPermission   = errors.New("security: permission denied")
)

type SafeError struct {
	PublicCode    string
	PublicMessage string
	InternalError error
}

func (e *SafeError) Error() string {
	if e.InternalError != nil {
		return fmt.Sprintf("%s: %v", e.PublicMessage, e.InternalError)
	}
	return e.PublicMessage
}

func (e *SafeError) Unwrap() error {
	return e.InternalError
}

func NewSafeError(publicCode, publicMessage string, internal error) *SafeError {
	return &SafeError{
		PublicCode:    publicCode,
		PublicMessage: publicMessage,
		InternalError: internal,
	}
}

func SanitizeInternalError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrUnsafePath) || errors.Is(err, ErrPathEscape) {
		return ErrUnsafePath
	}
	if errors.Is(err, ErrAuthFailOpen) || errors.Is(err, ErrSecurityMisconfigured) {
		return ErrSecurityMisconfigured
	}
	return err
}
