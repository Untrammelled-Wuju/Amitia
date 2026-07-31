// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package protocol

import "time"

type ClickPayload struct {
	Button     string  `json:"button"`
	ClickCount int     `json:"clickCount"`
	CanvasX    float64 `json:"canvasX"`
	CanvasY    float64 `json:"canvasY"`
	ScreenX    float64 `json:"screenX"`
	ScreenY    float64 `json:"screenY"`
	FrameIndex int     `json:"frameIndex"`
	ActionKey  string  `json:"actionKey"`
}

type DragPayload struct {
	DragID    string  `json:"dragId"`
	Phase     string  `json:"phase"`
	StartX    float64 `json:"startX"`
	StartY    float64 `json:"startY"`
	CurrentX  float64 `json:"currentX"`
	CurrentY  float64 `json:"currentY"`
	DeltaX    float64 `json:"deltaX"`
	DeltaY    float64 `json:"deltaY"`
	DisplayID string  `json:"displayId"`
}

type PlaybackPayload struct {
	ActionKey       string     `json:"actionKey"`
	PlaybackID      string     `json:"playbackId"`
	CommandID       string     `json:"commandId,omitempty"`
	FrameIndex      int        `json:"frameIndex"`
	CycleIndex      int        `json:"cycleIndex"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	InterruptReason string     `json:"interruptReason,omitempty"`
	ErrorCode       string     `json:"errorCode,omitempty"`
}

type WindowPayload struct {
	Visible    bool    `json:"visible"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	DisplayID  string  `json:"displayId"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
}

type RuntimeErrorPayload struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	Component    string `json:"component"`
	Recoverable  bool   `json:"recoverable"`
	CommandID    string `json:"commandId,omitempty"`
	PlaybackID   string `json:"playbackId,omitempty"`
	ActionKey    string `json:"actionKey,omitempty"`
}

const (
	InterruptReasonHigherPriorityAction = "higher_priority_action"
	InterruptReasonReleaseSwitch        = "release_switch"
	InterruptReasonRuntimeDisable       = "runtime_disable"
	InterruptReasonWindowDestroyed      = "window_destroyed"
	InterruptReasonUserDrag             = "user_drag"
	InterruptReasonCommandCancelled     = "command_cancelled"

	ErrorCodeRendererNotReady          = "renderer_not_ready"
	ErrorCodeWindowCreateFailed        = "window_create_failed"
	ErrorCodeReleaseLoadFailed         = "release_load_failed"
	ErrorCodeActionNotFound            = "action_not_found"
	ErrorCodeFrameMissing              = "frame_missing"
	ErrorCodeFrameDecodeFailed         = "frame_decode_failed"
	ErrorCodeAnimationTimeout          = "animation_timeout"
	ErrorCodeIpcDeliveryFailed         = "ipc_delivery_failed"
	ErrorCodeRuntimeSessionSuperseded  = "runtime_session_superseded"
	ErrorCodeProtocolVersionUnsupported = "protocol_version_unsupported"
)
