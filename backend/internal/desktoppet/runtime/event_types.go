// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"encoding/json"
	"time"
)

// RuntimeDomainEvent is the transport-neutral event envelope shared by the
// canonical Runtime V2 outbox and the desktop-pet behavior engine. It is not a
// Runtime V1 wire-protocol type.
type RuntimeDomainEvent struct {
	EventType      string
	RuntimeID      string
	SessionID      string
	DeviceID       string
	InstallationID string
	UserID         string
	CharacterID    string
	Timestamp      time.Time
	Payload        json.RawMessage
}

// RuntimeEventSink consumes canonical Runtime V2 domain events after they have
// crossed the durable outbox boundary.
type RuntimeEventSink interface {
	OnRuntimeEvent(ctx context.Context, event RuntimeDomainEvent) error
}

// NoopEventSink is useful for wiring paths that intentionally ignore runtime
// events while retaining the same transport-neutral interface.
type NoopEventSink struct{}

func (NoopEventSink) OnRuntimeEvent(_ context.Context, _ RuntimeDomainEvent) error { return nil }
