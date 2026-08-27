// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"fmt"
	"strings"
)

const (
	clickThroughModeOff         = "off"
	clickThroughModeFull        = "full"
	clickThroughModeAlpha       = "alpha"
	clickThroughModeBoundingBox = "boundingBox"
	positionModeAbsolute        = "absolute"
	positionModeRelative        = "relative"
	positionModeRecenter        = "recenter"
)

var validClickThroughModes = map[string]bool{
	clickThroughModeOff:         true,
	clickThroughModeFull:        true,
	clickThroughModeAlpha:       true,
	clickThroughModeBoundingBox: true,
}

var validPositionModes = map[string]bool{
	positionModeAbsolute: true,
	positionModeRelative: true,
	positionModeRecenter: true,
}

type UpdateRuntimeSettingsRequest struct {
	AlwaysOnTop            *int     `json:"alwaysOnTop"`
	LaunchOnStartup        *int     `json:"launchOnStartup"`
	Scale                  *float64 `json:"scale"`
	PositionX              *int     `json:"positionX"`
	PositionY              *int     `json:"positionY"`
	ScreenID               *string  `json:"screenId"`
	IdleEnabled            *int     `json:"idleEnabled"`
	IdleIntervalMinSeconds *int     `json:"idleIntervalMinSeconds"`
	IdleIntervalMaxSeconds *int     `json:"idleIntervalMaxSeconds"`
	ClickThroughMode       *string  `json:"clickThroughMode"`
	SoundEnabled           *int     `json:"soundEnabled"`
	RestoreOnAppStart      *int     `json:"restoreOnAppStart"`
	PositionMode           *string  `json:"positionMode"`
	DisplayFingerprint     *string  `json:"displayFingerprint"`
	RelativeX              *float64 `json:"relativeX"`
	RelativeY              *float64 `json:"relativeY"`
	LastWindowWidth        *int     `json:"lastWindowWidth"`
	LastWindowHeight       *int     `json:"lastWindowHeight"`
	ExpectedRevision       *int     `json:"expectedRevision"`
}

func (r *UpdateRuntimeSettingsRequest) Validate() error {
	if r == nil {
		return NewInstallationError(ErrCodeInstallationInvalid, "请求体为空", ErrInstallationInvalid)
	}
	if r.Scale != nil {
		v := *r.Scale
		if v < 0.1 || v > 5.0 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("scale 超出范围 [0.1, 5.0]: %f", v), ErrInstallationInvalid)
		}
	}
	if r.IdleIntervalMinSeconds != nil {
		v := *r.IdleIntervalMinSeconds
		if v < 5 || v > 3600 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("idleIntervalMinSeconds 超出范围 [5, 3600]: %d", v), ErrInstallationInvalid)
		}
	}
	if r.IdleIntervalMaxSeconds != nil {
		v := *r.IdleIntervalMaxSeconds
		if v < 5 || v > 3600 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("idleIntervalMaxSeconds 超出范围 [5, 3600]: %d", v), ErrInstallationInvalid)
		}
	}
	if r.IdleIntervalMinSeconds != nil && r.IdleIntervalMaxSeconds != nil {
		if *r.IdleIntervalMinSeconds > *r.IdleIntervalMaxSeconds {
			return NewInstallationError(ErrCodeInstallationInvalid,
				"idleIntervalMinSeconds 不能大于 idleIntervalMaxSeconds", ErrInstallationInvalid)
		}
	}
	if r.ClickThroughMode != nil {
		v := strings.ToLower(*r.ClickThroughMode)
		if !validClickThroughModes[v] {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("clickThroughMode 无效: %s (允许: off/full/alpha/boundingBox)", *r.ClickThroughMode), ErrInstallationInvalid)
		}
		r.ClickThroughMode = &v
	}
	if r.PositionMode != nil {
		v := strings.ToLower(*r.PositionMode)
		if !validPositionModes[v] {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("positionMode 无效: %s (允许: absolute/relative/recenter)", *r.PositionMode), ErrInstallationInvalid)
		}
		r.PositionMode = &v
	}
	if r.RelativeX != nil {
		v := *r.RelativeX
		if v < 0.0 || v > 1.0 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("relativeX 超出范围 [0.0, 1.0]: %f", v), ErrInstallationInvalid)
		}
	}
	if r.RelativeY != nil {
		v := *r.RelativeY
		if v < 0.0 || v > 1.0 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("relativeY 超出范围 [0.0, 1.0]: %f", v), ErrInstallationInvalid)
		}
	}
	if r.AlwaysOnTop != nil {
		v := *r.AlwaysOnTop
		if v != 0 && v != 1 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("alwaysOnTop 必须为 0 或 1: %d", v), ErrInstallationInvalid)
		}
	}
	if r.LaunchOnStartup != nil {
		v := *r.LaunchOnStartup
		if v != 0 && v != 1 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("launchOnStartup 必须为 0 或 1: %d", v), ErrInstallationInvalid)
		}
	}
	if r.IdleEnabled != nil {
		v := *r.IdleEnabled
		if v != 0 && v != 1 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("idleEnabled 必须为 0 或 1: %d", v), ErrInstallationInvalid)
		}
	}
	if r.SoundEnabled != nil {
		v := *r.SoundEnabled
		if v != 0 && v != 1 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("soundEnabled 必须为 0 或 1: %d", v), ErrInstallationInvalid)
		}
	}
	if r.RestoreOnAppStart != nil {
		v := *r.RestoreOnAppStart
		if v != 0 && v != 1 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("restoreOnAppStart 必须为 0 或 1: %d", v), ErrInstallationInvalid)
		}
	}
	if r.LastWindowWidth != nil {
		v := *r.LastWindowWidth
		if v < 0 || v > 4096 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("lastWindowWidth 超出范围 [0, 4096]: %d", v), ErrInstallationInvalid)
		}
	}
	if r.LastWindowHeight != nil {
		v := *r.LastWindowHeight
		if v < 0 || v > 4096 {
			return NewInstallationError(ErrCodeInstallationInvalid,
				fmt.Sprintf("lastWindowHeight 超出范围 [0, 4096]: %d", v), ErrInstallationInvalid)
		}
	}
	return nil
}

func (r *UpdateRuntimeSettingsRequest) IsEmpty() bool {
	if r == nil {
		return true
	}
	return r.AlwaysOnTop == nil && r.LaunchOnStartup == nil && r.Scale == nil &&
		r.PositionX == nil && r.PositionY == nil && r.ScreenID == nil &&
		r.IdleEnabled == nil && r.IdleIntervalMinSeconds == nil && r.IdleIntervalMaxSeconds == nil &&
		r.ClickThroughMode == nil && r.SoundEnabled == nil && r.RestoreOnAppStart == nil &&
		r.PositionMode == nil && r.DisplayFingerprint == nil && r.RelativeX == nil &&
		r.RelativeY == nil && r.LastWindowWidth == nil && r.LastWindowHeight == nil
}

func (r *UpdateRuntimeSettingsRequest) ToUpdates() map[string]interface{} {
	updates := make(map[string]interface{})
	if r.AlwaysOnTop != nil {
		updates["always_on_top"] = *r.AlwaysOnTop
	}
	if r.LaunchOnStartup != nil {
		updates["launch_on_startup"] = *r.LaunchOnStartup
	}
	if r.Scale != nil {
		updates["scale"] = *r.Scale
	}
	if r.PositionX != nil {
		updates["position_x"] = *r.PositionX
		updates["position_mode"] = positionModeAbsolute
		updates["position_updated_at"] = ""
	}
	if r.PositionY != nil {
		updates["position_y"] = *r.PositionY
		updates["position_mode"] = positionModeAbsolute
		updates["position_updated_at"] = ""
	}
	if r.ScreenID != nil {
		updates["screen_id"] = *r.ScreenID
	}
	if r.IdleEnabled != nil {
		updates["idle_enabled"] = *r.IdleEnabled
	}
	if r.IdleIntervalMinSeconds != nil {
		updates["idle_interval_min_seconds"] = *r.IdleIntervalMinSeconds
	}
	if r.IdleIntervalMaxSeconds != nil {
		updates["idle_interval_max_seconds"] = *r.IdleIntervalMaxSeconds
	}
	if r.ClickThroughMode != nil {
		updates["click_through_mode"] = *r.ClickThroughMode
	}
	if r.SoundEnabled != nil {
		updates["sound_enabled"] = *r.SoundEnabled
	}
	if r.RestoreOnAppStart != nil {
		updates["restore_on_app_start"] = *r.RestoreOnAppStart
	}
	if r.PositionMode != nil {
		updates["position_mode"] = *r.PositionMode
	}
	if r.DisplayFingerprint != nil {
		updates["display_fingerprint"] = *r.DisplayFingerprint
	}
	if r.RelativeX != nil {
		updates["relative_x"] = *r.RelativeX
	}
	if r.RelativeY != nil {
		updates["relative_y"] = *r.RelativeY
	}
	if r.LastWindowWidth != nil {
		updates["last_window_width"] = *r.LastWindowWidth
	}
	if r.LastWindowHeight != nil {
		updates["last_window_height"] = *r.LastWindowHeight
	}
	return updates
}

func (r *UpdateRuntimeSettingsRequest) HasPositionChange() bool {
	return r.PositionX != nil || r.PositionY != nil || r.ScreenID != nil ||
		r.PositionMode != nil || r.DisplayFingerprint != nil ||
		r.RelativeX != nil || r.RelativeY != nil
}
