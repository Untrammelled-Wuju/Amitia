package contracts

import "encoding/json"

type SinkRegisterInput struct {
	SinkID    string          `json:"sinkId"`
	Kind      string          `json:"kind"`
	ServiceID string          `json:"serviceId,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}
