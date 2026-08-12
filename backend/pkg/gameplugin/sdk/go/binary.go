package sdk

import (
	"context"
	"encoding/json"
)

const (
	MethodBinaryRegister = "binary.register"
	MethodBinaryRelease  = "binary.release"
)

type BinaryChecksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type BinaryRegisterInput struct {
	BinaryID  string          `json:"binaryId"`
	Kind      string          `json:"kind"`
	Size      int64           `json:"size"`
	MediaType string          `json:"mediaType,omitempty"`
	Checksum  *BinaryChecksum `json:"checksum,omitempty"`
	Lifetime  string          `json:"lifetime"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

type BinaryRegisterOutput struct {
	BinaryID string `json:"binaryId"`
	Token    string `json:"token"`
}

type BinaryReleaseInput struct {
	BinaryID string `json:"binaryId"`
}

func (c *Client) RegisterBinary(ctx context.Context, input BinaryRegisterInput, opts ...MessageOption) (BinaryRegisterOutput, error) {
	payload := map[string]any{
		"binaryId": input.BinaryID,
		"kind":     input.Kind,
		"size":     input.Size,
		"lifetime": input.Lifetime,
	}
	if input.MediaType != "" {
		payload["mediaType"] = input.MediaType
	}
	if input.Checksum != nil {
		payload["checksum"] = input.Checksum
	}
	if input.Metadata != nil {
		payload["metadata"] = input.Metadata
	}

	envelope, err := c.SendRequest(ctx, MethodBinaryRegister, payload, opts...)
	if err != nil {
		return BinaryRegisterOutput{}, err
	}
	var out BinaryRegisterOutput
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return BinaryRegisterOutput{}, NewEncodeError("unmarshal binary register response failed: %v", err)
		}
	}
	return out, nil
}

func (c *Client) ReleaseBinary(ctx context.Context, input BinaryReleaseInput, opts ...MessageOption) error {
	_, err := c.SendRequest(ctx, MethodBinaryRelease, input, opts...)
	return err
}
