// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package settings

import (
	"errors"
)

var (
	ErrRevisionConflict      = errors.New("settings: revision conflict")
	ErrSettingsNotFound      = errors.New("settings: not found")
	ErrOwnershipMismatch     = errors.New("settings: ownership mismatch")
	ErrInvalidCASRevision    = errors.New("settings: invalid expected revision for CAS")
)

type RuntimeSettings struct {
	ID string

	InstallationID string
	UserID         string
	DeviceID       string

	AlwaysOnTop            int
	LaunchOnStartup        int
	Scale                  float64
	PositionX              int
	PositionY              int
	ScreenID               string
	IdleEnabled            int
	IdleIntervalMinSeconds int
	IdleIntervalMaxSeconds int
	ClickThroughMode       string
	SoundEnabled           int
	SettingsRevision       int
	RestoreOnAppStart      int
	PositionMode           string
	DisplayFingerprint     string
	RelativeX              float64
	RelativeY              float64
	LastWindowWidth        int
	LastWindowHeight       int
	PositionUpdatedAt      string

	CreatedAt string
	UpdatedAt string
}

func (RuntimeSettings) TableName() string {
	return "desktop_pet_runtime_settings"
}

type SettingsCASRequest struct {
	InstallationID     string
	ExpectedRevision   int
	InitiatorDeviceID  string
	InitiatorUserID    string
}

func (r SettingsCASRequest) IsValid() bool {
	return r.InstallationID != "" && r.InitiatorUserID != "" && r.InitiatorDeviceID != ""
}

type SettingsSnapshot struct {
	SettingsRevision int
	Scale            float64
	PositionMode     string
	PositionPolicy   string
	AlwaysOnTop      int
	SoundEnabled     int
	ClickThroughMode string
	IdleEnabled      int
}

type SettingsValidationError struct {
	Field   string
	Message string
}

func (e SettingsValidationError) Error() string {
	return "settings validation: " + e.Field + " - " + e.Message
}

const (
	PositionPolicyCenterActiveDisplay  = "center_active_display"
	PositionPolicyCenterPrimaryDisplay = "center_primary_display"
	PositionPolicyLastKnown            = "last_known"
	PositionPolicyRelative             = "relative"

	SettingsUpdateModeCAS      = "cas"
	SettingsUpdateModeLastWins = "last_write_wins"
)
