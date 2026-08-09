package stream

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/state"
)

type Hub interface {
	PublishEvent(
		ctx context.Context,
		event EventEnvelope,
		opts ...PublishEventOption,
	) error

	PublishState(
		ctx context.Context,
		update state.StateUpdate,
	) (state.StateSnapshot, error)

	GetLatestState(
		ctx context.Context,
		key state.StateKey,
	) (state.StateSnapshot, error)

	ListLatestState(
		ctx context.Context,
		filter state.StateFilter,
	) ([]state.StateSnapshot, error)

	RemoveState(
		ctx context.Context,
		key state.StateKey,
	) error

	RemoveStateByService(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
		serviceID domain.ServiceID,
	) error

	RemoveStateByRuntime(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
	) error

	StateCount(ctx context.Context) int
}

type HubConfig struct {
	EventPublisher EventPublisher
	StateStore     state.StateStore
}

type GameHub struct {
	events EventPublisher
	states state.StateStore
}

func NewGameHub(cfg HubConfig) *GameHub {
	return &GameHub{
		events: cfg.EventPublisher,
		states: cfg.StateStore,
	}
}

func (h *GameHub) PublishEvent(ctx context.Context, event EventEnvelope, opts ...PublishEventOption) error {
	if h.events == nil {
		return domain.NewHostError(domain.ErrInternal, "stream hub: event publisher is nil")
	}
	return h.events.PublishEvent(ctx, event, opts...)
}

func (h *GameHub) PublishState(ctx context.Context, update state.StateUpdate) (state.StateSnapshot, error) {
	if h.states == nil {
		return state.StateSnapshot{}, domain.NewHostError(domain.ErrInternal, "stream hub: state store is nil")
	}
	return h.states.Put(ctx, update)
}

func (h *GameHub) GetLatestState(ctx context.Context, key state.StateKey) (state.StateSnapshot, error) {
	if h.states == nil {
		return state.StateSnapshot{}, domain.NewHostError(domain.ErrInternal, "stream hub: state store is nil")
	}
	return h.states.Get(ctx, key)
}

func (h *GameHub) ListLatestState(ctx context.Context, filter state.StateFilter) ([]state.StateSnapshot, error) {
	if h.states == nil {
		return nil, domain.NewHostError(domain.ErrInternal, "stream hub: state store is nil")
	}
	return h.states.List(ctx, filter)
}

func (h *GameHub) RemoveState(ctx context.Context, key state.StateKey) error {
	if h.states == nil {
		return domain.NewHostError(domain.ErrInternal, "stream hub: state store is nil")
	}
	return h.states.Remove(ctx, key)
}

func (h *GameHub) RemoveStateByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) error {
	if h.states == nil {
		return domain.NewHostError(domain.ErrInternal, "stream hub: state store is nil")
	}
	return h.states.RemoveByService(ctx, runtimeID, serviceID)
}

func (h *GameHub) RemoveStateByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if h.states == nil {
		return domain.NewHostError(domain.ErrInternal, "stream hub: state store is nil")
	}
	return h.states.RemoveByRuntime(ctx, runtimeID)
}

func (h *GameHub) StateCount(ctx context.Context) int {
	if h.states == nil {
		return 0
	}
	return h.states.Count(ctx)
}

var _ Hub = (*GameHub)(nil)
