package v2

import "errors"

var (
	ErrCommandDuplication = errors.New("command_duplication")
	ErrCommandNotFound    = errors.New("command_not_found")
)

const (
	ErrCodeProtocolUnsupported        = "RUNTIME_PROTOCOL_UNSUPPORTED"
	ErrCodeEnvelopeInvalid            = "RUNTIME_ENVELOPE_INVALID"
	ErrCodePayloadHashMismatch        = "RUNTIME_PAYLOAD_HASH_MISMATCH"
	ErrCodePayloadSchemaUnsupported   = "RUNTIME_PAYLOAD_SCHEMA_UNSUPPORTED"
	ErrCodeSessionStale               = "RUNTIME_SESSION_STALE"
	ErrCodeConnectionSuperseded       = "RUNTIME_CONNECTION_SUPERSEDED"
	ErrCodeSequenceStale              = "RUNTIME_SEQUENCE_STALE"
	ErrCodeCommandUnsupported         = "RUNTIME_COMMAND_UNSUPPORTED"
	ErrCodeCommandIdempotencyConflict = "RUNTIME_COMMAND_IDEMPOTENCY_CONFLICT"
	ErrCodeCommandExpired             = "RUNTIME_COMMAND_EXPIRED"
	ErrCodeCommandSuperseded          = "RUNTIME_COMMAND_SUPERSEDED"
	ErrCodeCommandCapabilityMissing   = "RUNTIME_COMMAND_CAPABILITY_MISSING"
	ErrCodeRuntimeOffline             = "RUNTIME_OFFLINE"
	ErrCodeRuntimeNotReady            = "RUNTIME_NOT_READY"
	ErrCodeRuntimeUnauthorized        = "RUNTIME_UNAUTHORIZED"
	ErrCodeAcceptTimeout              = "RUNTIME_ACCEPT_TIMEOUT"
	ErrCodeRendererAcceptTimeout      = "RENDERER_ACCEPT_TIMEOUT"
	ErrCodePlaybackStartTimeout       = "PLAYBACK_START_TIMEOUT"
	ErrCodePlaybackCompletionTimeout  = "PLAYBACK_COMPLETION_TIMEOUT"
	ErrCodePlaybackActionNotFound     = "PLAYBACK_ACTION_NOT_FOUND"
	ErrCodePlaybackResourceLoadFailed = "PLAYBACK_RESOURCE_LOAD_FAILED"
	ErrCodePlaybackRendererCrashed    = "PLAYBACK_RENDERER_CRASHED"
	ErrCodeDesiredHashMismatch        = "RUNTIME_DESIRED_HASH_MISMATCH"
	ErrCodeReleaseHashMismatch        = "RUNTIME_RELEASE_HASH_MISMATCH"
	ErrCodeActualStateStale           = "RUNTIME_ACTUAL_STATE_STALE"
	ErrCodeRenderEventWithoutCommand  = "RENDER_EVENT_WITHOUT_COMMAND"
	ErrCodeCommandStateConflict       = "COMMAND_STATE_CONFLICT"
	ErrCodeDesiredRevisionConflict    = "RUNTIME_DESIRED_REVISION_CONFLICT"
	ErrCodeSyncRejected               = "RUNTIME_SYNC_REJECTED"
)

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
