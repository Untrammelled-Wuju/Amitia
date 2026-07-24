// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

type AnchorMode string

const (
	AnchorFeetCenter        AnchorMode = "feet_center"
	AnchorBodyCenter        AnchorMode = "body_center"
	AnchorHeadCenter        AnchorMode = "head_center"
	AnchorHandContact       AnchorMode = "hand_contact"
	AnchorWindowEdgeContact AnchorMode = "window_edge_contact"
)

type Anchor struct {
	Type AnchorMode `json:"type"`
	X    float64    `json:"x"`
	Y    float64    `json:"y"`
}

var DefaultFeetCenterAnchor = Anchor{Type: AnchorFeetCenter, X: 0.5, Y: 0.92}

var defaultAnchorMap = map[string]Anchor{
	"idle_normal":      {AnchorFeetCenter, 0.5, 0.92},
	"idle_breathing":   {AnchorFeetCenter, 0.5, 0.92},
	"idle_blink":       {AnchorFeetCenter, 0.5, 0.92},
	"idle_sway":        {AnchorFeetCenter, 0.5, 0.92},
	"wave":             {AnchorFeetCenter, 0.5, 0.92},
	"happy":            {AnchorFeetCenter, 0.5, 0.92},
	"speaking":         {AnchorFeetCenter, 0.5, 0.92},
	"walk_left":        {AnchorFeetCenter, 0.5, 0.92},
	"walk_right":       {AnchorFeetCenter, 0.5, 0.92},
	"run_left":         {AnchorFeetCenter, 0.5, 0.92},
	"run_right":        {AnchorFeetCenter, 0.5, 0.92},
	"click_happy":      {AnchorFeetCenter, 0.5, 0.92},
	"click_angry":      {AnchorFeetCenter, 0.5, 0.92},
	"sleep_start":      {AnchorFeetCenter, 0.5, 0.92},
	"sleep_loop":       {AnchorFeetCenter, 0.5, 0.92},
	"sleep_end":        {AnchorFeetCenter, 0.5, 0.92},
	"fall":             {AnchorBodyCenter, 0.5, 0.5},
	"picked_up":        {AnchorBodyCenter, 0.5, 0.5},
	"sit_window_edge":  {AnchorWindowEdgeContact, 0.5, 0.95},
	"climb_window":     {AnchorHandContact, 0.5, 0.1},
	"sleep_on_desk":    {AnchorBodyCenter, 0.5, 0.95},
	"sleep_on_desktop": {AnchorBodyCenter, 0.5, 0.95},
	"edge_sit":         {AnchorWindowEdgeContact, 0.5, 0.95},
	"edge_climb":       {AnchorHandContact, 0.5, 0.1},
}

func DefaultAnchorForActionKey(actionKey string) Anchor {
	if a, ok := defaultAnchorMap[actionKey]; ok {
		return a
	}
	return DefaultFeetCenterAnchor
}

func ComputeAnchor(box backgroundremoval.SubjectBox, mode AnchorMode) Anchor {
	if box.Empty {
		return Anchor{Type: mode, X: 0.5, Y: 0.5}
	}

	centerX := (float64(box.MinX) + float64(box.MaxX)) / 2.0
	centerY := (float64(box.MinY) + float64(box.MaxY)) / 2.0

	switch mode {
	case AnchorFeetCenter:
		return Anchor{Type: mode, X: centerX, Y: float64(box.MaxY)}
	case AnchorBodyCenter:
		return Anchor{Type: mode, X: centerX, Y: centerY}
	case AnchorHeadCenter:
		return Anchor{Type: mode, X: centerX, Y: float64(box.MinY)}
	case AnchorHandContact:
		return Anchor{Type: mode, X: centerX, Y: float64(box.MinY)}
	case AnchorWindowEdgeContact:
		return Anchor{Type: mode, X: centerX, Y: float64(box.MaxY)}
	default:
		return Anchor{Type: mode, X: centerX, Y: float64(box.MaxY)}
	}
}
