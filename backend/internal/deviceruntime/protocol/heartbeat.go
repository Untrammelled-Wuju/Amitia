package protocol

import (
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type PingPayload struct {
	Time time.Time `json:"time"`
}

type PongPayload struct {
	Time time.Time `json:"time"`
}

type Heartbeat struct {
	RuntimeSessionID runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	SentAt           time.Time                        `json:"sentAt"`
	LastSequence     int64                            `json:"lastSequence,omitempty"`
}
