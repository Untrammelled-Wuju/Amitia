// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package protocol

import "encoding/json"

type LegacyEventAdapter struct{}

var legacyEventAliases = map[string]string{
	"clicked":              "desktop.pet.clicked",
	"double_clicked":       "desktop.pet.double_clicked",
	"hovered":              "desktop.pet.hover.moved",
	"drag_start":           "desktop.pet.drag.started",
	"drag_started":         "desktop.pet.drag.started",
	"dragged":              "desktop.pet.drag.completed",
	"drag_end":             "desktop.pet.drag.completed",
	"drag_moved":           "desktop.pet.drag.moved",
	"action_started":       "playback.action.started",
	"action_complete":      "playback.action.completed",
	"action_switch":        "playback.action.started",
	"action_interrupted":   "playback.action.interrupted",
	"playback_completed":   "playback.action.completed",
	"playback_interrupted": "playback.action.interrupted",
	"play_action_forwarded": "playback.action.requested",
}

func (LegacyEventAdapter) Translate(rawEventType string) (string, bool) {
	if rawEventType == "" {
		return "", false
	}
	if mapped, ok := legacyEventAliases[rawEventType]; ok {
		return mapped, true
	}
	if IsValidEventType(rawEventType) {
		return rawEventType, true
	}
	return "", false
}

type LegacyEventWrapper struct {
	EventType        string
	LegacyEventType  string
	LegacyProtocol   bool
	Payload          json.RawMessage
}

func (LegacyEventAdapter) Wrap(legacyType, standardType string, payload json.RawMessage) LegacyEventWrapper {
	return LegacyEventWrapper{
		EventType:       standardType,
		LegacyEventType: legacyType,
		LegacyProtocol:  true,
		Payload:         payload,
	}
}

func IsLegacyEventType(eventType string) bool {
	_, ok := legacyEventAliases[eventType]
	return ok
}
