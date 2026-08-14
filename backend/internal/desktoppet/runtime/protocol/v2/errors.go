package v2

import (
	"errors"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

var (
	ErrCommandDuplication = errors.New("command_duplication")
	ErrCommandNotFound    = errors.New("command_not_found")
)

const (
	ErrCodeProtocolUnsupported        = protocol.ErrCodeProtocolUnsupported
	ErrCodeEnvelopeInvalid            = protocol.ErrCodeEnvelopeInvalid
	ErrCodePayloadHashMismatch        = protocol.ErrCodePayloadHashMismatch
	ErrCodePayloadSchemaUnsupported   = protocol.ErrCodePayloadSchemaUnsupported
	ErrCodeSessionStale               = protocol.ErrCodeSessionStale
	ErrCodeConnectionSuperseded       = protocol.ErrCodeConnectionSuperseded
	ErrCodeSequenceStale              = protocol.ErrCodeSequenceStale
	ErrCodeRuntimeOffline             = protocol.ErrCodeRuntimeOffline
	ErrCodeRuntimeNotReady            = protocol.ErrCodeRuntimeNotReady
	ErrCodeRuntimeUnauthorized        = protocol.ErrCodeRuntimeUnauthorized
	ErrCodeCommandUnsupported         = "RUNTIME_COMMAND_UNSUPPORTED"
	ErrCodeCommandIdempotencyConflict = "RUNTIME_COMMAND_IDEMPOTENCY_CONFLICT"
	ErrCodeCommandExpired             = "RUNTIME_COMMAND_EXPIRED"
	ErrCodeCommandSuperseded          = "RUNTIME_COMMAND_SUPERSEDED"
	ErrCodeCommandCapabilityMissing   = "RUNTIME_COMMAND_CAPABILITY_MISSING"
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

type ProtocolError = protocol.ProtocolError

func NewProtocolError(code, message string) *ProtocolError {
	return protocol.NewProtocolError(code, message)
}
