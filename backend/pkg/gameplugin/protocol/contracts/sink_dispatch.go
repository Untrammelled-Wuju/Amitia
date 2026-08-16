package contracts

import "encoding/json"

const MethodSinkDispatch = "control.sink.dispatch"

type SinkEffectDispatchPayload struct {
	SinkID     string          `json:"sinkId"`
	ServiceID  string          `json:"serviceId"`
	OutputID   string          `json:"outputId"`
	Epoch      uint64          `json:"epoch"`
	Generation uint64          `json:"generation"`
	Payload    json.RawMessage `json:"payload"`
}

type SinkEffectCommitResult struct {
	Accepted   bool   `json:"accepted"`
	Committed  bool   `json:"committed"`
	EffectID   string `json:"effectId"`
	Generation uint64 `json:"generation"`
	ErrorCode  string `json:"errorCode,omitempty"`
	Message    string `json:"message,omitempty"`
}
