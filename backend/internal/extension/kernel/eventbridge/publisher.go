package eventbridge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

var ErrDurableEventPublishFailed = errors.New("eventbridge: durable event publish failed")

type Publisher struct {
	durable event.DurableEventPublisher
}

func NewPublisher(durable event.DurableEventPublisher) (*Publisher, error) {
	if durable == nil {
		return nil, errors.New("eventbridge: durable publisher required")
	}
	return &Publisher{durable: durable}, nil
}

func (p *Publisher) Publish(
	ctx context.Context,
	typeID event.EventTypeID,
	version int,
	payload any,
	opts event.PublishOptions,
) (event.PublishResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return event.PublishResult{}, fmt.Errorf("%w: marshal: %w", ErrDurableEventPublishFailed, err)
	}
	result, err := p.durable.Publish(ctx, typeID, version, raw, opts)
	if err != nil {
		return event.PublishResult{}, fmt.Errorf("%w: %w", ErrDurableEventPublishFailed, err)
	}
	return result, nil
}

func (p *Publisher) PublishTx(
	ctx context.Context,
	tx *sql.Tx,
	typeID event.EventTypeID,
	version int,
	payload any,
	opts event.PublishOptions,
) (event.PublishResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return event.PublishResult{}, fmt.Errorf("%w: marshal: %w", ErrDurableEventPublishFailed, err)
	}
	result, err := p.durable.PublishTx(ctx, tx, typeID, version, raw, opts)
	if err != nil {
		return event.PublishResult{}, fmt.Errorf("%w: %w", ErrDurableEventPublishFailed, err)
	}
	return result, nil
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(
	ctx context.Context,
	typeID event.EventTypeID,
	version int,
	payload any,
	opts event.PublishOptions,
) (event.PublishResult, error) {
	return event.PublishResult{}, nil
}

func (NoopPublisher) PublishTx(
	ctx context.Context,
	tx *sql.Tx,
	typeID event.EventTypeID,
	version int,
	payload any,
	opts event.PublishOptions,
) (event.PublishResult, error) {
	return event.PublishResult{}, nil
}
