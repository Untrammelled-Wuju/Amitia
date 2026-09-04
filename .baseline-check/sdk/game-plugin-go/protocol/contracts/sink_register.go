package contracts

import "encoding/json"

// SinkRegisterInput is intentionally identity-free. The service owning the
// sink is the trusted RPC peer, not a serviceId supplied by plugin JSON.
type SinkRegisterInput struct {
	SinkID   string          `json:"sinkId"`
	Kind     string          `json:"kind"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}
