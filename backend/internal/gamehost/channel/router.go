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
	PublishBinary(ctx context.Context, channel RuntimeChannel, message BinaryChannelMessage) (json.RawMessage, error)
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

type RouterConfig struct {
	Registry Registry
	Events   stream.EventPublisher
	States   state.StateStore
	Generic  GenericChannelSink
	Binary   BinarySink
	NowFunc  func() time.Time
}

type Router struct {
	registry Registry
	events   stream.EventPublisher
	states   state.StateStore
	generic  GenericChannelSink
	binary   BinarySink
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
		nowFunc:  now,
	}
}

func (r *Router) Route(ctx context.Context, msg IncomingChannelMessage) error {
	_, err := r.RouteCanonical(ctx, msg)
	return err
}

// RouteCanonical validates and routes a plugin-to-host channel message and
// returns the canonical payload that is safe for downstream persistence/fanout.
// For ordinary JSON channels the payload is copied unchanged. Binary channels
// are replaced with a host-authoritative BinaryReference so caller-controlled
// checksum/mediaType/metadata fields cannot survive validation.
func (r *Router) RouteCanonical(ctx context.Context, msg IncomingChannelMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := msg.Peer.Validate(); err != nil {
		return nil, err
	}

	channel, err := r.registry.Resolve(ctx, msg.Peer.RuntimeID, msg.Peer.ServiceID, msg.ChannelID)
	if err != nil {
		return nil, err
	}

	if channel.PluginID != msg.Peer.PluginID {
		return nil, ErrPluginMismatch
	}
	if channel.RuntimeID != msg.Peer.RuntimeID {
		return nil, ErrRuntimeMismatch
	}

	if err := ValidateDirection(channel, protocol.ChannelDirectionPluginToHost); err != nil {
		return nil, err
	}

	switch channel.Kind {
	case domain.ChannelKindEvent:
		if err := r.routeEvent(ctx, channel, msg); err != nil {
			return nil, err
		}
	case domain.ChannelKindState:
		if err := r.routeState(ctx, channel, msg); err != nil {
			return nil, err
		}
	case domain.ChannelKindLog, domain.ChannelKindMetric, domain.ChannelKindCustom:
		if err := r.routeGeneric(ctx, channel, msg); err != nil {
			return nil, err
		}
	case domain.ChannelKindBinary:
		return r.routeBinary(ctx, channel, msg)
	default:
		return nil, ErrKindUnknown
	}

	return copyRawMessage(msg.Payload), nil
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

func (r *Router) routeBinary(ctx context.Context, channel RuntimeChannel, msg IncomingChannelMessage) (json.RawMessage, error) {
	if r.binary == nil {
		return nil, ErrBinaryNotSupported
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
