// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"encoding/json"
	"time"

	runtimev2 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v2"
)

func DecodeV2OutboxEvent(event runtimev2.DomainEventOutbox) (RuntimeDomainEvent, error) {
	if len(event.Payload) > 0 {
		var fullEvent RuntimeDomainEvent
		if err := json.Unmarshal(event.Payload, &fullEvent); err == nil && fullEvent.EventType != "" {
			if fullEvent.RuntimeID == "" {
				fullEvent.RuntimeID = event.AggregateID
			}
			return fullEvent, nil
		}
	}
	return RuntimeDomainEvent{
		EventType: event.EventType,
		RuntimeID: event.AggregateID,
		Payload:   event.Payload,
		Timestamp: time.Now(),
	}, nil
}
