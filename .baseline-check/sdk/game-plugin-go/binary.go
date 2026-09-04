package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

const (
	MethodBinaryCreate  = "binary.create"
	MethodBinaryWrite   = "binary.write"
	MethodBinarySeal    = "binary.seal"
	MethodBinaryRead    = "binary.read"
	MethodBinaryStat    = "binary.stat"
	MethodBinaryRelease = "binary.release"
	MethodBinaryAbort   = "binary.abort"

	DefaultBinaryChunkSize = 512 * 1024
	MaxBinaryChunkSize     = 2 * 1024 * 1024
)

type BinaryReference = protocol.BinaryReference
type BinaryStorageKind = protocol.BinaryStorageKind
type BinaryLifetime = protocol.BinaryLifetime
type BinaryChecksum = protocol.BinaryChecksum

const (
	BinaryStorageFile     = protocol.BinaryStorageFile
	BinaryLifetimeMessage = protocol.BinaryLifetimeMessage
	BinaryLifetimeRuntime = protocol.BinaryLifetimeRuntime
)

type BinaryCreateInput struct {
	ChannelID    string                     `json:"channelId"`
	ExpectedSize int64                      `json:"expectedSize"`
	MediaType    string                     `json:"mediaType,omitempty"`
	Lifetime     BinaryLifetime             `json:"lifetime,omitempty"`
	Metadata     map[string]json.RawMessage `json:"metadata,omitempty"`
}

type BinaryCreateResult struct {
	ID            string            `json:"id"`
	Kind          BinaryStorageKind `json:"kind"`
	ChunkSize     int               `json:"chunkSize"`
	MaxChunkSize  int               `json:"maxChunkSize"`
	MaxObjectSize int64             `json:"maxObjectSize"`
}

type BinaryWriteResult struct {
	ID         string `json:"id"`
	Written    int    `json:"written"`
	NextOffset int64  `json:"nextOffset"`
}

type BinarySealResult struct {
	Reference BinaryReference `json:"reference"`
}

type BinaryReadResult struct {
	ID         string `json:"id"`
	Offset     int64  `json:"offset"`
	NextOffset int64  `json:"nextOffset"`
	Data       string `json:"data"`
	EOF        bool   `json:"eof"`
	Size       int64  `json:"size"`
}

type BinaryStatResult struct {
	ID        string                     `json:"id"`
	Kind      BinaryStorageKind          `json:"kind"`
	Size      int64                      `json:"size"`
	MediaType string                     `json:"mediaType,omitempty"`
	Lifetime  BinaryLifetime             `json:"lifetime"`
	Checksum  *BinaryChecksum            `json:"checksum,omitempty"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
	State     string                     `json:"state"`
}

func (c *Client) BinaryCreate(ctx context.Context, input BinaryCreateInput, opts ...MessageOption) (BinaryCreateResult, error) {
	if input.Lifetime == "" {
		input.Lifetime = BinaryLifetimeMessage
	}
	var out BinaryCreateResult
	if err := c.binaryRequest(ctx, MethodBinaryCreate, input, &out, opts...); err != nil {
		return BinaryCreateResult{}, err
	}
	return out, nil
}

func (c *Client) BinaryWrite(ctx context.Context, id string, offset int64, data []byte, opts ...MessageOption) (BinaryWriteResult, error) {
	if len(data) == 0 {
		return BinaryWriteResult{}, fmt.Errorf("binary write chunk must not be empty")
	}
	if len(data) > MaxBinaryChunkSize {
		return BinaryWriteResult{}, fmt.Errorf("binary write chunk exceeds %d bytes", MaxBinaryChunkSize)
	}
	if offset < 0 {
		return BinaryWriteResult{}, fmt.Errorf("binary write offset must not be negative")
	}
	if fast, ok := c.transport.(BinaryFrameTransport); ok {
		return c.binaryWriteFrame(ctx, fast, id, offset, data, opts...)
	}
	input := map[string]any{
		"id":     id,
		"offset": offset,
		"data":   base64.StdEncoding.EncodeToString(data),
	}
	var out BinaryWriteResult
	if err := c.binaryRequest(ctx, MethodBinaryWrite, input, &out, opts...); err != nil {
		return BinaryWriteResult{}, err
	}
	return out, nil
}

func (c *Client) binaryWriteFrame(ctx context.Context, transport BinaryFrameTransport, id string, offset int64, data []byte, opts ...MessageOption) (BinaryWriteResult, error) {
	envelope, err := c.NewRequest(MethodBinaryWrite, nil, opts...)
	if err != nil {
		return BinaryWriteResult{}, err
	}
	timeout := c.pendingTimeoutMs
	if envelope.Metadata != nil {
		if raw, ok := envelope.Metadata["__timeout"]; ok {
			var ms int
			if json.Unmarshal(raw, &ms) == nil && ms > 0 {
				timeout = time.Duration(ms) * time.Millisecond
			}
		}
	}
	pending := c.registerPending(envelope.ID, MethodBinaryWrite, timeout)
	defer c.removePending(envelope.ID)
	if err := transport.SendBinaryFrame(ctx, envelope, id, offset, data); err != nil {
		return BinaryWriteResult{}, NewTransportError("send binary frame failed: %v", err)
	}

	select {
	case <-pending.Done:
		pending.mu.Lock()
		state := pending.state
		resp := pending.Response
		pendingErr := pending.Err
		pending.mu.Unlock()
		if state != stateCompleted {
			if pendingErr != nil {
				return BinaryWriteResult{}, pendingErr
			}
			return BinaryWriteResult{}, NewTransportError("binary write request %s ended in state %d", envelope.ID, state)
		}
		if pendingErr != nil {
			return BinaryWriteResult{}, pendingErr
		}
		var out BinaryWriteResult
		if len(resp.Payload) == 0 {
			return BinaryWriteResult{}, NewEncodeError("binary.write returned an empty response")
		}
		if err := json.Unmarshal(resp.Payload, &out); err != nil {
			return BinaryWriteResult{}, NewEncodeError("unmarshal binary.write response: %v", err)
		}
		return out, nil
	case <-ctx.Done():
		return BinaryWriteResult{}, NewTransportError("binary write request %s cancelled: %v", envelope.ID, ctx.Err())
	}
}

func (c *Client) BinarySeal(ctx context.Context, id string, opts ...MessageOption) (BinaryReference, error) {
	var out BinarySealResult
	if err := c.binaryRequest(ctx, MethodBinarySeal, map[string]any{"id": id}, &out, opts...); err != nil {
		return BinaryReference{}, err
	}
	if err := out.Reference.Validate(); err != nil {
		return BinaryReference{}, fmt.Errorf("invalid binary reference returned by host: %w", err)
	}
	return out.Reference, nil
}

func (c *Client) BinaryAbort(ctx context.Context, id string, opts ...MessageOption) error {
	return c.binaryRequest(ctx, MethodBinaryAbort, map[string]any{"id": id}, nil, opts...)
}

func (c *Client) BinaryRelease(ctx context.Context, id string, opts ...MessageOption) error {
	return c.binaryRequest(ctx, MethodBinaryRelease, map[string]any{"id": id}, nil, opts...)
}

func (c *Client) BinaryStat(ctx context.Context, id string, opts ...MessageOption) (BinaryStatResult, error) {
	var out BinaryStatResult
	if err := c.binaryRequest(ctx, MethodBinaryStat, map[string]any{"id": id}, &out, opts...); err != nil {
		return BinaryStatResult{}, err
	}
	return out, nil
}

func (c *Client) BinaryRead(ctx context.Context, ref BinaryReference, offset int64, maxBytes int, opts ...MessageOption) ([]byte, BinaryReadResult, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultBinaryChunkSize
	}
	if maxBytes > MaxBinaryChunkSize {
		return nil, BinaryReadResult{}, fmt.Errorf("binary read chunk exceeds %d bytes", MaxBinaryChunkSize)
	}
	input := map[string]any{"reference": ref, "offset": offset, "maxBytes": maxBytes}
	var out BinaryReadResult
	if err := c.binaryRequest(ctx, MethodBinaryRead, input, &out, opts...); err != nil {
		return nil, BinaryReadResult{}, err
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil {
		return nil, BinaryReadResult{}, fmt.Errorf("host returned invalid base64 binary chunk: %w", err)
	}
	return data, out, nil
}

// BinaryUpload is the normal high-level upload path. It creates a host-owned
// binary object, uploads bounded chunks, and seals it into a BinaryReference.
// Any failure aborts the partial object before returning.
func (c *Client) BinaryUpload(ctx context.Context, input BinaryCreateInput, data []byte, opts ...MessageOption) (BinaryReference, error) {
	input.ExpectedSize = int64(len(data))
	created, err := c.BinaryCreate(ctx, input, opts...)
	if err != nil {
		return BinaryReference{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = c.BinaryAbort(context.Background(), created.ID)
		}
	}()

	chunkSize := created.ChunkSize
	if chunkSize <= 0 || chunkSize > created.MaxChunkSize {
		chunkSize = DefaultBinaryChunkSize
	}
	if created.MaxChunkSize > 0 && chunkSize > created.MaxChunkSize {
		chunkSize = created.MaxChunkSize
	}
	if chunkSize > MaxBinaryChunkSize {
		chunkSize = MaxBinaryChunkSize
	}

	offset := int64(0)
	for len(data) > 0 {
		n := chunkSize
		if len(data) < n {
			n = len(data)
		}
		result, err := c.BinaryWrite(ctx, created.ID, offset, data[:n], opts...)
		if err != nil {
			return BinaryReference{}, err
		}
		offset = result.NextOffset
		data = data[n:]
	}
	ref, err := c.BinarySeal(ctx, created.ID, opts...)
	if err != nil {
		return BinaryReference{}, err
	}
	committed = true
	return ref, nil
}

func (c *Client) BinaryReadAll(ctx context.Context, ref BinaryReference, opts ...MessageOption) ([]byte, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	if ref.Size < 0 || ref.Size > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("binary object is too large for this process")
	}
	out := make([]byte, 0, int(ref.Size))
	offset := int64(0)
	for offset < ref.Size {
		chunk, result, err := c.BinaryRead(ctx, ref, offset, DefaultBinaryChunkSize, opts...)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
		if result.NextOffset <= offset && !result.EOF {
			return nil, fmt.Errorf("binary read made no progress at offset %d", offset)
		}
		offset = result.NextOffset
		if result.EOF {
			break
		}
	}
	if int64(len(out)) != ref.Size {
		return nil, fmt.Errorf("binary read size mismatch: got %d, want %d", len(out), ref.Size)
	}
	return out, nil
}

func (c *Client) ChannelPublishBinary(ctx context.Context, channelID string, ref BinaryReference, metadata map[string]json.RawMessage, opts ...MessageOption) (protocol.Envelope, error) {
	if err := ref.Validate(); err != nil {
		return protocol.Envelope{}, err
	}
	payload, err := json.Marshal(ref)
	if err != nil {
		return protocol.Envelope{}, err
	}
	return c.ChannelPublish(ctx, ChannelPublishInput{ChannelID: channelID, Payload: payload, Metadata: metadata}, opts...)
}

func (c *Client) binaryRequest(ctx context.Context, method string, input any, output any, opts ...MessageOption) error {
	envelope, err := c.SendReservedRequest(ctx, method, input, opts...)
	if err != nil {
		return err
	}
	if output == nil || len(envelope.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(envelope.Payload, output); err != nil {
		return NewEncodeError("unmarshal %s response: %v", method, err)
	}
	return nil
}
