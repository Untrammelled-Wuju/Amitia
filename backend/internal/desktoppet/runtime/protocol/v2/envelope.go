package v2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const (
	EnvelopeVersion     = 2
	ProtocolName        = "amitia.desktop-pet.runtime"
	CurrentSchemaVersion = "2.0.0"
)

type MessageType string

const (
	MessageTypeHello        MessageType = "hello"
	MessageTypeHelloAck     MessageType = "hello_ack"
	MessageTypeCommand      MessageType = "command"
	MessageTypeCommandAck   MessageType = "command_ack"
	MessageTypeRuntimeEvent MessageType = "runtime_event"
	MessageTypeStateSnapshot MessageType = "state_snapshot"
	MessageTypeError        MessageType = "error"
	MessageTypePing         MessageType = "ping"
	MessageTypePong         MessageType = "pong"
)

func IsValidMessageType(t string) bool {
	switch MessageType(t) {
	case MessageTypeHello, MessageTypeHelloAck, MessageTypeCommand,
		MessageTypeCommandAck, MessageTypeRuntimeEvent, MessageTypeStateSnapshot,
		MessageTypeError, MessageTypePing, MessageTypePong:
		return true
	}
	return false
}

type Envelope struct {
	EnvelopeVersion      int             `json:"envelopeVersion"`
	Protocol             string          `json:"protocol"`
	MessageType          MessageType     `json:"messageType"`
	MessageName          string          `json:"messageName"`
	MessageID            string          `json:"messageId"`
	CorrelationID        string          `json:"correlationId,omitempty"`
	CausationID          string          `json:"causationId,omitempty"`
	UserID               string          `json:"userId"`
	DeviceID             string          `json:"deviceId"`
	RuntimeID            string          `json:"runtimeId"`
	RuntimeSessionID     string          `json:"runtimeSessionId"`
	ConnectionGeneration int64           `json:"connectionGeneration"`
	Sequence             int64           `json:"sequence"`
	PayloadSchemaVersion int             `json:"payloadSchemaVersion"`
	PayloadHash          string          `json:"payloadHash"`
	OccurredAt           time.Time       `json:"occurredAt,omitempty"`
	SentAt               time.Time       `json:"sentAt"`
	Payload              json.RawMessage `json:"payload,omitempty"`
}

func (e *Envelope) Validate() error {
	if e.EnvelopeVersion != EnvelopeVersion {
		return fmt.Errorf("unsupported envelope version: %d", e.EnvelopeVersion)
	}
	if e.Protocol != ProtocolName {
		return fmt.Errorf("unexpected protocol: %s", e.Protocol)
	}
	if !IsValidMessageType(string(e.MessageType)) {
		return fmt.Errorf("invalid message type: %s", e.MessageType)
	}
	if e.MessageID == "" {
		return fmt.Errorf("messageId is required")
	}
	if e.UserID == "" {
		return fmt.Errorf("userId is required")
	}
	if e.DeviceID == "" {
		return fmt.Errorf("deviceId is required")
	}
	if e.RuntimeID == "" {
		return fmt.Errorf("runtimeId is required")
	}
	if e.RuntimeSessionID == "" {
		return fmt.Errorf("runtimeSessionId is required")
	}
	if e.ConnectionGeneration < 1 {
		return fmt.Errorf("connectionGeneration must be >= 1")
	}
	if e.Sequence < 0 {
		return fmt.Errorf("sequence must be >= 0")
	}
	if e.PayloadSchemaVersion < 1 {
		return fmt.Errorf("payloadSchemaVersion must be >= 1")
	}
	if e.PayloadHash == "" {
		return fmt.Errorf("payloadHash is required")
	}
	return nil
}

func ComputePayloadHash(payload []byte) string {
	sum := sha256.Sum256([]byte(CanonicalJSON(payload)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func CanonicalJSON(data []byte) string {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	canonical, err := marshalCanonical(v)
	if err != nil {
		return string(data)
	}
	return string(canonical)
}

func marshalCanonical(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		for i := 0; i < len(keys)-1; i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		result := make([]byte, 0, 64)
		result = append(result, '{')
		for i, k := range keys {
			if i > 0 {
				result = append(result, ',')
			}
			result = append(result, '"')
			result = append(result, k...)
			result = append(result, '"', ':')
			sub, err := marshalCanonical(val[k])
			if err != nil {
				return nil, err
			}
			result = append(result, sub...)
		}
		result = append(result, '}')
		return result, nil
	case []interface{}:
		result := make([]byte, 0, 64)
		result = append(result, '[')
		for i, item := range val {
			if i > 0 {
				result = append(result, ',')
			}
			sub, err := marshalCanonical(item)
			if err != nil {
				return nil, err
			}
			result = append(result, sub...)
		}
		result = append(result, ']')
		return result, nil
	default:
		return json.Marshal(v)
	}
}

func (e *Envelope) VerifyPayloadHash() bool {
	if len(e.Payload) == 0 {
		return e.PayloadHash == ""
	}
	expected := ComputePayloadHash(e.Payload)
	return expected == e.PayloadHash
}
