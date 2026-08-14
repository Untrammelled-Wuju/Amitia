package protocol

import "fmt"

type ProtocolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ProtocolError) Error() string {
	return e.Code + ": " + e.Message
}

func NewProtocolError(code, message string) *ProtocolError {
	return &ProtocolError{
		Code:    code,
		Message: message,
	}
}

const (
	ErrCodeProtocolUnsupported      = "RUNTIME_PROTOCOL_UNSUPPORTED"
	ErrCodeEnvelopeInvalid          = "RUNTIME_ENVELOPE_INVALID"
	ErrCodePayloadHashMismatch      = "RUNTIME_PAYLOAD_HASH_MISMATCH"
	ErrCodePayloadSchemaUnsupported = "RUNTIME_PAYLOAD_SCHEMA_UNSUPPORTED"
	ErrCodeSessionStale             = "RUNTIME_SESSION_STALE"
	ErrCodeConnectionSuperseded     = "RUNTIME_CONNECTION_SUPERSEDED"
	ErrCodeSequenceStale            = "RUNTIME_SEQUENCE_STALE"
	ErrCodeRuntimeOffline           = "RUNTIME_OFFLINE"
	ErrCodeRuntimeNotReady          = "RUNTIME_NOT_READY"
	ErrCodeRuntimeUnauthorized      = "RUNTIME_UNAUTHORIZED"
)

type VersionError struct {
	ExpectedProtocol string
	ActualProtocol   string
	ExpectedEnvelope int
	ActualEnvelope   int
}

func (e *VersionError) Error() string {
	return fmt.Errorf("protocol mismatch: expected %s@%d, got %s@%d",
		e.ExpectedProtocol, e.ExpectedEnvelope, e.ActualProtocol, e.ActualEnvelope).Error()
}
