package protocol

import "encoding/json"

type ErrorPayload struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Retryable bool            `json:"retryable,omitempty"`
	Details   json.RawMessage `json:"details,omitempty"`
}
