package channel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/state"
	"github.com/u-ai/backend/internal/gamehost/stream"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type GenericChannelSink interface {
	Publish(ctx context.Context, channel RuntimeChannel, message ChannelMessage) error
}

type BinarySink interface {
	PublishBinary(ctx context.Context, channel RuntimeChannel, message BinaryChannelMessage) error
}

type BinaryChannelMessage struct {
	PluginID  domain.PluginID
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID

	ChannelID domain.ChannelID
	Payload   json.RawMessage
	Metadata  map[string]json.RawMessage
}

type ChannelMessage struct {
	PluginID  domain.PluginID
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID

	ChannelID domain.ChannelID
	Payload   json.RawMessage
	Metadata  map[string]json.RawMessage
}

type IncomingChannelMessage struct {
	Peer      ipc.Peer
	ChannelID domain.ChannelID
	Payload   json.RawMessage
	Metadata  map[string]json.RawMessage
}

type OutgoingChannelMessage struct {
	ChannelID domain.ChannelID
	Payload   json.RawMessage
	Metadata  map[string]json.RawMessage
}

type ChannelTargetResolver interface {
	ResolveConnection(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ipc.Peer, bool, error)
}

type RouterConfig struct {
	Registry Registry
	Events   stream.EventPublisher
	States   state.StateStore
	Generic  GenericChannelSink
	Binary   BinarySink
	Target   ChannelTargetResolver
	NowFunc  func() time.Time
}

type Router struct {
	registry Registry
	events   stream.EventPublisher
	states   state.StateStore
	generic  GenericChannelSink
	binary   BinarySink
	target   ChannelTargetResolver
	nowFunc  func() time.Time
}

func NewRouter(cfg RouterConfig) *Router {
	now := cfg.NowFunc
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Router{
		registry: cfg.Registry,
		events:   cfg.Events,
		states:   cfg.States,
		generic:  cfg.Generic,
		binary:   cfg.Binary,
		target:   cfg.Target,
		nowFunc:  now,
	}
}

func (r *Router) Route(ctx context.Context, msg IncomingChannelMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := msg.Peer.Validate(); err != nil {
		return err
	}

	channel, err := r.registry.Resolve(ctx, msg.Peer.RuntimeID, msg.Peer.ServiceID, msg.ChannelID)
	if err != nil {
		return err
	}

	if channel.PluginID != msg.Peer.PluginID {
		return ErrPluginMismatch
	}
	if channel.RuntimeID != msg.Peer.RuntimeID {
		return ErrRuntimeMismatch
	}

	if err := ValidateDirection(channel, protocol.ChannelDirectionPluginToHost); err != nil {
		return err
	}

	switch channel.Kind {
	case domain.ChannelKindEvent:
		return r.routeEvent(ctx, channel, msg)
	case domain.ChannelKindState:
		return r.routeState(ctx, channel, msg)
	case domain.ChannelKindLog, domain.ChannelKindMetric, domain.ChannelKindCustom:
		return r.routeGeneric(ctx, channel, msg)
	case domain.ChannelKindBinary:
		return r.routeBinary(ctx, channel, msg)
	default:
		return ErrKindUnknown
	}
}

func (r *Router) routeEvent(ctx context.Context, channel RuntimeChannel, msg IncomingChannelMessage) error {
	// In production channel.publish itself travels through the durable notification
	// pipeline. A separate EventPublisher is optional and is used only when a
	// caller wants an additional typed stream event.
	if r.events == nil {
		return nil
	}

	envelope := stream.EventEnvelope{
		ID:         r.generateID(),
		TypeID:     "gamehost.channel.event",
		Version:    1,
		PluginID:   msg.Peer.PluginID,
		RuntimeID:  msg.Peer.RuntimeID,
		ServiceID:  msg.Peer.ServiceID,
		Method:     string(msg.ChannelID),
		Payload:    copyRawMessage(msg.Payload),
		Metadata:   copyMetadata(msg.Metadata),
		TraceID:    string(channel.ID),
		OccurredAt: r.nowFunc().Unix(),
	}

	return r.events.PublishEvent(ctx, envelope)
}

func (r *Router) routeState(ctx context.Context, channel RuntimeChannel, msg IncomingChannelMessage) error {
	if r.states == nil {
		return errors.New("channel router: state store is nil")
	}

	stateKey := buildStateKey(channel, msg.Metadata)

	update := state.StateUpdate{
		ID:         r.generateID(),
		PluginID:   msg.Peer.PluginID,
		RuntimeID:  msg.Peer.RuntimeID,
		ServiceID:  msg.Peer.ServiceID,
		Key:        stateKey,
		Payload:    copyRawMessage(msg.Payload),
		Metadata:   copyMetadata(msg.Metadata),
		ReceivedAt: r.nowFunc(),
	}

	_, err := r.states.Put(ctx, update)
	return err
}

func (r *Router) routeBinary(ctx context.Context, channel RuntimeChannel, msg IncomingChannelMessage) error {
	if r.binary == nil {
		return ErrBinaryNotSupported
	}

	chMsg := BinaryChannelMessage{
		PluginID:  msg.Peer.PluginID,
		RuntimeID: msg.Peer.RuntimeID,
		ServiceID: msg.Peer.ServiceID,
		ChannelID: msg.ChannelID,
		Payload:   copyRawMessage(msg.Payload),
		Metadata:  copyMetadata(msg.Metadata),
	}

	return r.binary.PublishBinary(ctx, channel, chMsg)
}

func (r *Router) routeGeneric(ctx context.Context, channel RuntimeChannel, msg IncomingChannelMessage) error {
	if r.generic == nil {
		return nil
	}

	chMsg := ChannelMessage{
		PluginID:  msg.Peer.PluginID,
		RuntimeID: msg.Peer.RuntimeID,
		ServiceID: msg.Peer.ServiceID,
		ChannelID: msg.ChannelID,
		Payload:   copyRawMessage(msg.Payload),
		Metadata:  copyMetadata(msg.Metadata),
	}

	return r.generic.Publish(ctx, channel, chMsg)
}

func (r *Router) SendToChannel(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID, msg OutgoingChannelMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	channel, err := r.registry.Resolve(ctx, runtimeID, serviceID, channelID)
	if err != nil {
		return err
	}

	if err := ValidateDirection(channel, protocol.ChannelDirectionHostToPlugin); err != nil {
		return err
	}

	if r.target == nil {
		return errors.New("channel router: target resolver is nil")
	}

	peer, found, err := r.target.ResolveConnection(ctx, channel.RuntimeID, channel.ServiceID)
	if err != nil {
		return err
	}
	if !found {
		return domain.NewHostError(domain.ErrRuntimeUnavailable, "channel: target service connection not available")
	}

	return r.deliverOutbound(ctx, peer, channel, msg)
}

func (r *Router) deliverOutbound(ctx context.Context, peer ipc.Peer, channel RuntimeChannel, msg OutgoingChannelMessage) error {
	return nil
}

func buildStateKey(channel RuntimeChannel, metadata map[string]json.RawMessage) string {
	if metadata != nil {
		if lk, ok := metadata["stateKey"]; ok {
			key := string(lk)
			key = strings.TrimPrefix(key, `"`)
			key = strings.TrimSuffix(key, `"`)
			if key != "" {
				return string(channel.ChannelID) + "/" + key
			}
		}
	}
	return string(channel.ChannelID)
}

func (r *Router) generateID() string {
	return r.nowFunc().Format("20060102T150405.000000")
}

func copyRawMessage(src json.RawMessage) json.RawMessage {
	if src == nil {
		return nil
	}
	cp := make(json.RawMessage, len(src))
	copy(cp, src)
	return cp
}

func copyMetadata(src map[string]json.RawMessage) map[string]json.RawMessage {
	if src == nil {
		return nil
	}
	cp := make(map[string]json.RawMessage, len(src))
	for k, v := range src {
		cp[k] = copyRawMessage(v)
	}
	return cp
}
