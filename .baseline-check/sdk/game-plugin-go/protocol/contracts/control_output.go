package contracts

import "encoding/json"

// ControlOutputInput is intentionally identity-free. Runtime, plugin, service,
// and generation are bound by the trusted GameHost connection and must never
// be supplied by an untrusted plugin payload.
type ControlOutputInput struct {
	OutputID string          `json:"outputId"`
	SinkID   string          `json:"sinkId"`
	Epoch    uint64          `json:"epoch"`
	Payload  json.RawMessage `json:"payload"`
}

type ControlOutputResult struct {
	OutputID     string `json:"outputId"`
	Allowed      bool   `json:"allowed"`
	Reason       string `json:"reason,omitempty"`
	CurrentEpoch uint64 `json:"currentEpoch"`
	Generation   int64  `json:"generation"`
}
