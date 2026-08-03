package behavior

import (
	"errors"
	"fmt"
)

const (
	ErrCodeEventSchemaInvalid     = "behavior_event_schema_invalid"
	ErrCodeEventOwnershipMismatch = "behavior_event_ownership_mismatch"
	ErrCodeEventDuplicate         = "behavior_event_duplicate"
	ErrCodeEventExpired           = "behavior_event_expired"
	ErrCodeContextConflict        = "behavior_context_conflict"
	ErrCodeContextCorrupt         = "behavior_context_corrupt"
	ErrCodeBindingInvalid         = "behavior_binding_invalid"
	ErrCodeBindingActionMissing   = "behavior_binding_action_missing"
	ErrCodeNoActiveInstallation   = "behavior_no_active_installation"
	ErrCodeNoActionAvailable      = "behavior_no_action_available"
	ErrCodeRuntimeOffline         = "behavior_runtime_offline"
	ErrCodeRuntimeCommandFailed   = "behavior_runtime_command_failed"
	ErrCodePlaybackFailed         = "behavior_playback_failed"
	ErrCodeSnapshotUnavailable    = "behavior_snapshot_unavailable"
	ErrCodeMailboxOverflow        = "behavior_mailbox_overflow"
	ErrCodeRulesetInvalid         = "behavior_ruleset_invalid"
)

type BehaviorError struct {
	Code    string
	Message string
	Cause   error
}

func (e *BehaviorError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *BehaviorError) Unwrap() error { return e.Cause }

func NewBehaviorError(code, message string) *BehaviorError {
	return &BehaviorError{Code: code, Message: message}
}

func NewBehaviorErrorWithCause(code, message string, cause error) *BehaviorError {
	return &BehaviorError{Code: code, Message: message, Cause: cause}
}

var (
	ErrNoActiveInstallation = NewBehaviorError(ErrCodeNoActiveInstallation, "no active desktop pet installation")
	ErrNoActionAvailable    = NewBehaviorError(ErrCodeNoActionAvailable, "no action available for current context")
	ErrRuntimeOffline       = NewBehaviorError(ErrCodeRuntimeOffline, "runtime is offline")
	ErrMailboxOverflow      = NewBehaviorError(ErrCodeMailboxOverflow, "behavior mailbox overflow")
	ErrContextCorrupt       = NewBehaviorError(ErrCodeContextCorrupt, "behavior context corrupt")
	ErrVersionConflict      = NewBehaviorError("behavior_binding_version_conflict", "绑定版本冲突，请刷新后重试")
)

func IsBehaviorError(err error) (*BehaviorError, bool) {
	var be *BehaviorError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}

func IsErrorCode(err error, code string) bool {
	be, ok := IsBehaviorError(err)
	if !ok {
		return false
	}
	return be.Code == code
}
