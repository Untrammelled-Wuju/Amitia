package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

type Envelope struct {
	Protocol string      `json:"protocol"`
	Type     MessageType `json:"type"`

	ID        string `json:"id,omitempty"`
	RequestID string `json:"requestId,omitempty"`

	Method string `json:"method,omitempty"`

	RuntimeID string `json:"runtimeId,omitempty"`
	PluginID  string `json:"pluginId,omitempty"`
	ServiceID string `json:"serviceId,omitempty"`

	Generation uint64 `json:"generation,omitempty"`

	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *ProtocolError  `json:"error,omitempty"`

	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (e Envelope) Validate() error {
	if e.Protocol != ProtocolVersion {
		return fmt.Errorf("invalid protocol: %s, expected: %s", e.Protocol, ProtocolVersion)
	}

	switch e.Type {
	case MessageTypeRequest, MessageTypeResponse, MessageTypeNotification, MessageTypeError:
	default:
		return fmt.Errorf("invalid message type: %s", e.Type)
	}

	if err := ValidateMessageID(e.ID); err != nil {
		return fmt.Errorf("invalid id: %w", err)
	}

	switch e.Type {
	case MessageTypeRequest:
		if err := ValidateRequestEnvelope(e); err != nil {
			return err
		}
	case MessageTypeResponse:
		if err := ValidateResponseEnvelope(e); err != nil {
			return err
		}
	case MessageTypeNotification:
		if err := ValidateNotificationEnvelope(e); err != nil {
			return err
		}
	case MessageTypeError:
		if err := ValidateErrorEnvelope(e); err != nil {
			return err
		}
	}

	return nil
}

func ValidateRequestEnvelope(e Envelope) error {
	if e.Method == "" {
		return fmt.Errorf("request must have method")
	}
	if e.RequestID != "" {
		return fmt.Errorf("request must not have requestId")
	}
	if e.Error != nil {
		return fmt.Errorf("request must not have error")
	}
	return nil
}

func ValidateResponseEnvelope(e Envelope) error {
	if e.RequestID == "" {
		return fmt.Errorf("response must have requestId")
	}
	if e.Error != nil {
		return fmt.Errorf("response must not have error")
	}
	return nil
}

func ValidateNotificationEnvelope(e Envelope) error {
	if e.Method == "" {
		return fmt.Errorf("notification must have method")
	}
	if e.RequestID != "" {
		return fmt.Errorf("notification must not have requestId")
	}
	if e.Error != nil {
		return fmt.Errorf("notification must not have error")
	}
	return nil
}

func ValidateErrorEnvelope(e Envelope) error {
	if e.Error == nil {
		return fmt.Errorf("error envelope must have error")
	}
	if e.Error.Code == "" {
		return fmt.Errorf("error code must not be empty")
	}
	if e.Error.Message == "" {
		return fmt.Errorf("error message must not be empty")
	}
	return nil
}

func ValidateMessageID(id string) error {
	if id == "" {
		return fmt.Errorf("message id must not be empty")
	}
	const maxLength = 256
	if len(id) > maxLength {
		return fmt.Errorf("message id exceeds maximum length of %d", maxLength)
	}
	for _, r := range id {
		if unicode.IsControl(r) {
			return fmt.Errorf("message id contains control character")
		}
	}
	return nil
}

func ValidateMethod(method string) error {
	if method == "" {
		return fmt.Errorf("method must not be empty")
	}
	parts := strings.Split(method, ".")
	if len(parts) < 2 {
		return fmt.Errorf("method must have at least two parts separated by dots")
	}
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("method parts must not be empty")
		}
		for _, r := range part {
			if unicode.IsUpper(r) {
				return fmt.Errorf("method must be lowercase")
			}
		}
	}
	return nil
}

func IsReservedNamespace(method string) bool {
	reserved := []string{"host.", "plugin.", "runtime.", "service.", "channel.", "control.", "emergency.", "secret."}
	for _, ns := range reserved {
		if strings.HasPrefix(method, ns) {
			return true
		}
	}
	return false
}

func ValidatePluginMethod(method string) error {
	if err := ValidateMethod(method); err != nil {
		return err
	}
	if IsReservedNamespace(method) {
		return fmt.Errorf("method '%s' uses reserved namespace", method)
	}
	return nil
}

func Encode(e Envelope) ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("failed to encode envelope: %w", err)
	}
	return data, nil
}

func Decode(data []byte) (Envelope, error) {
	var e Envelope
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&e); err != nil {
		return Envelope{}, fmt.Errorf("failed to decode envelope: %w", err)
	}
	if err := e.Validate(); err != nil {
		return Envelope{}, err
	}
	return e, nil
}
