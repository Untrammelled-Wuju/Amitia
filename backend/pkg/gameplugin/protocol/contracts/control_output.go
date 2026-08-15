package contracts

import "encoding/json"

type ControlOutputInput struct {
	OutputID  string          `json:"outputId"`
	SinkID    string          `json:"sinkId"`
	Epoch     uint64          `json:"epoch"`
	Kind      string          `json:"kind"`
	ServiceID string          `json:"serviceId,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

type ControlOutputResult struct {
	OutputID     string `json:"outputId"`
	Allowed      bool   `json:"allowed"`
	Reason       string `json:"reason,omitempty"`
	CurrentEpoch uint64 `json:"currentEpoch"`
	Generation   int64  `json:"generation"`
}
