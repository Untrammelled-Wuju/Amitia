// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type RuntimeDomainEventOutbox interface {
	Append(
		ctx context.Context,
		event RuntimeDomainEvent,
	) error
}

type OutboxRuntimeEventSink struct {
	outbox RuntimeDomainEventOutbox
}

func NewOutboxRuntimeEventSink(
	outbox RuntimeDomainEventOutbox,
) *OutboxRuntimeEventSink {
	return &OutboxRuntimeEventSink{
		outbox: outbox,
	}
}

func (
	s *OutboxRuntimeEventSink,
) OnRuntimeEvent(
	ctx context.Context,
	event RuntimeDomainEvent,
) error {
	if s == nil || s.outbox == nil {
		return errors.New(
			"runtime event outbox is unavailable",
		)
	}
	return s.outbox.Append(ctx, event)
}

type v2StateEventOutbox struct {
	appendFn func(eventType, aggregateID string, payload []byte, t time.Time, idemKey *string) (bool, error)
}

func NewV2ActualStateEventOutbox(
	appendFn func(eventType, aggregateID string, payload []byte, t time.Time, idemKey *string) (bool, error),
) RuntimeDomainEventOutbox {
	return &v2StateEventOutbox{appendFn: appendFn}
}

func (o *v2StateEventOutbox) Append(ctx context.Context, event RuntimeDomainEvent) error {
	// Always persist the complete domain envelope. Persisting only event.Payload
	// loses user/device/session/installation identity and makes downstream
	// Behavior consumers unable to correlate runtime feedback reliably.
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = o.appendFn(
		event.EventType,
		event.RuntimeID,
		payload,
		time.Now(),
		nil,
	)
	return err
}
