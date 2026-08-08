package sdk

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type MessageOption func(*protocol.Envelope)

func WithRuntimeID(id string) MessageOption {
	return func(e *protocol.Envelope) {
		e.RuntimeID = id
	}
}

func WithPluginID(id string) MessageOption {
	return func(e *protocol.Envelope) {
		e.PluginID = id
	}
}

func WithServiceID(id string) MessageOption {
	return func(e *protocol.Envelope) {
		e.ServiceID = id
	}
}

func WithMetadata(key string, value json.RawMessage) MessageOption {
	return func(e *protocol.Envelope) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]json.RawMessage)
		}
		e.Metadata[key] = value
	}
}

type Client struct {
	transport    Transport
	idGenerator  IDGenerator
	pluginID     string
	runtimeID    string
	serviceID    string
}

type ClientOption func(*Client)

func WithIDGenerator(g IDGenerator) ClientOption {
	return func(c *Client) {
		c.idGenerator = g
	}
}

func WithClientPluginID(id string) ClientOption {
	return func(c *Client) {
		c.pluginID = id
	}
}

func WithClientRuntimeID(id string) ClientOption {
	return func(c *Client) {
		c.runtimeID = id
	}
}

func WithClientServiceID(id string) ClientOption {
	return func(c *Client) {
		c.serviceID = id
	}
}

func NewClient(transport Transport, opts ...ClientOption) *Client {
	c := &Client{
		transport:   transport,
		idGenerator: UUIDGenerator{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) Transport() Transport {
	return c.transport
}

func (c *Client) SendRequest(ctx context.Context, method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	if err := protocol.ValidatePluginMethod(method); err != nil {
		return protocol.Envelope{}, NewValidationError("invalid method: %w", err)
	}
	if protocol.IsReservedNamespace(method) {
		return protocol.Envelope{}, NewValidationError("method '%s' uses reserved namespace", method)
	}

	envelope, err := c.NewRequest(method, payload, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send request failed: %w", err)
	}
	return envelope, nil
}

func (c *Client) SendResponse(ctx context.Context, request protocol.Envelope, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	envelope, err := c.NewResponse(request, payload, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send response failed: %w", err)
	}
	return envelope, nil
}

func (c *Client) SendNotification(ctx context.Context, method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	if err := protocol.ValidatePluginMethod(method); err != nil {
		return protocol.Envelope{}, NewValidationError("invalid method: %w", err)
	}
	if protocol.IsReservedNamespace(method) {
		return protocol.Envelope{}, NewValidationError("method '%s' uses reserved namespace", method)
	}

	envelope, err := c.NewNotification(method, payload, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send notification failed: %w", err)
	}
	return envelope, nil
}

func (c *Client) SendError(ctx context.Context, request protocol.Envelope, code protocol.ErrorCode, message string, retryable bool, data any, opts ...MessageOption) (protocol.Envelope, error) {
	envelope, err := c.NewError(request, code, message, retryable, data, opts...)
	if err != nil {
		return protocol.Envelope{}, err
	}

	if err := c.transport.Send(ctx, envelope); err != nil {
		return protocol.Envelope{}, NewTransportError("send error failed: %w", err)
	}
	return envelope, nil
}

func (c *Client) NewRequest(method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	id := c.idGenerator.NewID()

	var rawPayload json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return protocol.Envelope{}, NewEncodeError("marshal payload failed: %w", err)
		}
		rawPayload = data
	}

	envelope := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       id,
		Method:   method,
		Payload:  rawPayload,
		PluginID: c.pluginID,
		RuntimeID: c.runtimeID,
		ServiceID: c.serviceID,
	}

	for _, opt := range opts {
		opt(&envelope)
	}

	if err := envelope.Validate(); err != nil {
		return protocol.Envelope{}, NewValidationError("envelope validation failed: %w", err)
	}

	return envelope, nil
}

func (c *Client) NewResponse(request protocol.Envelope, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	id := c.idGenerator.NewID()

	var rawPayload json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return protocol.Envelope{}, NewEncodeError("marshal payload failed: %w", err)
		}
		rawPayload = data
	}

	envelope := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeResponse,
		ID:        id,
		RequestID: request.ID,
		Payload:   rawPayload,
		PluginID:  c.pluginID,
		RuntimeID: c.runtimeID,
		ServiceID: c.serviceID,
	}

	for _, opt := range opts {
		opt(&envelope)
	}

	if err := envelope.Validate(); err != nil {
		return protocol.Envelope{}, NewValidationError("envelope validation failed: %w", err)
	}

	return envelope, nil
}

func (c *Client) NewNotification(method string, payload any, opts ...MessageOption) (protocol.Envelope, error) {
	id := c.idGenerator.NewID()

	var rawPayload json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return protocol.Envelope{}, NewEncodeError("marshal payload failed: %w", err)
		}
		rawPayload = data
	}

	envelope := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeNotification,
		ID:       id,
		Method:   method,
		Payload:  rawPayload,
		PluginID: c.pluginID,
		RuntimeID: c.runtimeID,
		ServiceID: c.serviceID,
	}

	for _, opt := range opts {
		opt(&envelope)
	}

	if err := envelope.Validate(); err != nil {
		return protocol.Envelope{}, NewValidationError("envelope validation failed: %w", err)
	}

	return envelope, nil
}

func (c *Client) NewError(request protocol.Envelope, code protocol.ErrorCode, message string, retryable bool, data any, opts ...MessageOption) (protocol.Envelope, error) {
	id := c.idGenerator.NewID()

	var rawData json.RawMessage
	if data != nil {
		d, err := json.Marshal(data)
		if err != nil {
			return protocol.Envelope{}, NewEncodeError("marshal error data failed: %w", err)
		}
		rawData = d
	}

	if err := protocol.ValidateErrorCode(code); err != nil {
		return protocol.Envelope{}, NewValidationError("invalid error code: %w", err)
	}

	envelope := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeError,
		ID:        id,
		RequestID: request.ID,
		PluginID:  c.pluginID,
		RuntimeID: c.runtimeID,
		ServiceID: c.serviceID,
		Error: &protocol.ProtocolError{
			Code:      code,
			Message:   message,
			Retryable: retryable,
			Data:      rawData,
		},
	}

	for _, opt := range opts {
		opt(&envelope)
	}

	if err := envelope.Validate(); err != nil {
		return protocol.Envelope{}, NewValidationError("envelope validation failed: %w", err)
	}

	return envelope, nil
}

func (c *Client) Receive(ctx context.Context) (protocol.Envelope, error) {
	envelope, err := c.transport.Receive(ctx)
	if err != nil {
		return protocol.Envelope{}, NewTransportError("receive failed: %w", err)
	}
	return envelope, nil
}

func (c *Client) Close() error {
	return c.transport.Close()
}
