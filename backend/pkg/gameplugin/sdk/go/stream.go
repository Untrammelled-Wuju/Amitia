package sdk

import (
	"context"
	"encoding/json"
)

const (
	MethodStreamOpen    = "stream.open"
	MethodStreamWrite   = "stream.write"
	MethodStreamRead    = "stream.read"
	MethodStreamClose   = "stream.close"
	MethodStreamCursor  = "stream.cursor"
)

type StreamCursor struct {
	RuntimeID  string `json:"runtimeId"`
	ServiceID  string `json:"serviceId"`
	ChannelID  string `json:"channelId"`
	Generation string `json:"generation"`
	Sequence   int64  `json:"sequence"`
}

type StreamOpenInput struct {
	ChannelID string `json:"channelId"`
	Cursor    *StreamCursor `json:"cursor,omitempty"`
}

type StreamOpenOutput struct {
	StreamID   string        `json:"streamId"`
	Generation string        `json:"generation"`
	Cursor     StreamCursor  `json:"cursor"`
}

type StreamWriteInput struct {
	StreamID string          `json:"streamId"`
	Data     json.RawMessage `json:"data"`
}

type StreamWriteOutput struct {
	Sequence int64 `json:"sequence"`
}

type StreamReadInput struct {
	StreamID string `json:"streamId"`
	Cursor   StreamCursor `json:"cursor"`
	Limit    int    `json:"limit,omitempty"`
}

type StreamReadOutput struct {
	Items   []StreamFrame `json:"items"`
	Cursor  StreamCursor  `json:"cursor"`
	HasMore bool          `json:"hasMore"`
}

type StreamFrame struct {
	Sequence int64           `json:"sequence"`
	Data     json.RawMessage `json:"data"`
}

type StreamCloseInput struct {
	StreamID string `json:"streamId"`
}

type StreamCursorInput struct {
	StreamID string `json:"streamId"`
}

type StreamCursorOutput struct {
	Cursor StreamCursor `json:"cursor"`
}

func (c *Client) StreamOpen(ctx context.Context, input StreamOpenInput, opts ...MessageOption) (StreamOpenOutput, error) {
	payload := map[string]any{
		"channelId": input.ChannelID,
	}
	if input.Cursor != nil {
		payload["cursor"] = input.Cursor
	}

	envelope, err := c.SendRequest(ctx, MethodStreamOpen, payload, opts...)
	if err != nil {
		return StreamOpenOutput{}, err
	}
	var out StreamOpenOutput
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return StreamOpenOutput{}, NewEncodeError("unmarshal stream open response failed: %v", err)
		}
	}
	return out, nil
}

func (c *Client) StreamWrite(ctx context.Context, input StreamWriteInput, opts ...MessageOption) (StreamWriteOutput, error) {
	envelope, err := c.SendRequest(ctx, MethodStreamWrite, input, opts...)
	if err != nil {
		return StreamWriteOutput{}, err
	}
	var out StreamWriteOutput
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return StreamWriteOutput{}, NewEncodeError("unmarshal stream write response failed: %v", err)
		}
	}
	return out, nil
}

func (c *Client) StreamRead(ctx context.Context, input StreamReadInput, opts ...MessageOption) (StreamReadOutput, error) {
	payload := map[string]any{
		"streamId": input.StreamID,
		"cursor":   input.Cursor,
	}
	if input.Limit > 0 {
		payload["limit"] = input.Limit
	}

	envelope, err := c.SendRequest(ctx, MethodStreamRead, payload, opts...)
	if err != nil {
		return StreamReadOutput{}, err
	}
	var out StreamReadOutput
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return StreamReadOutput{}, NewEncodeError("unmarshal stream read response failed: %v", err)
		}
	}
	return out, nil
}

func (c *Client) StreamClose(ctx context.Context, input StreamCloseInput, opts ...MessageOption) error {
	_, err := c.SendRequest(ctx, MethodStreamClose, input, opts...)
	return err
}

func (c *Client) StreamCursor(ctx context.Context, input StreamCursorInput, opts ...MessageOption) (StreamCursorOutput, error) {
	envelope, err := c.SendRequest(ctx, MethodStreamCursor, input, opts...)
	if err != nil {
		return StreamCursorOutput{}, err
	}
	var out StreamCursorOutput
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return StreamCursorOutput{}, NewEncodeError("unmarshal stream cursor response failed: %v", err)
		}
	}
	return out, nil
}
