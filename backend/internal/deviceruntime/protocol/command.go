package protocol

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type CommandName string

type CommandPayload struct {
	CommandID       string          `json:"commandId"`
	CommandName     string          `json:"commandType"`
	CommandSequence int64           `json:"commandSequence"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

type CommandAckPayload struct {
	CommandID        string                           `json:"commandId"`
	CommandSequence  int64                            `json:"commandSequence"`
	Status           string                           `json:"status"`
	PayloadHash      string                           `json:"payloadHash,omitempty"`
	RejectReason     string                           `json:"rejectReason,omitempty"`
	RejectErrorCode  string                           `json:"rejectErrorCode,omitempty"`
	EstimatedStartMs int64                            `json:"estimatedStartMs,omitempty"`
	RuntimeSessionID runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ReceivedAt       time.Time                        `json:"receivedAt"`
}
