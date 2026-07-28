package desktop_update

import "errors"

type ErrorCode string

const (
	ErrorCodeNoUpdate              ErrorCode = "no_update"
	ErrorCodeNetwork               ErrorCode = "network_error"
	ErrorCodeIndexInvalid          ErrorCode = "index_invalid"
	ErrorCodeIndexSignatureInvalid ErrorCode = "index_signature_invalid"
	ErrorCodePackageHashMismatch   ErrorCode = "package_hash_mismatch"
	ErrorCodePublisherMismatch     ErrorCode = "publisher_mismatch"
	ErrorCodeCompatibilityFailed   ErrorCode = "compatibility_failed"
)

type UpdateError struct {
	Code ErrorCode
	Err  error
}

func (e *UpdateError) Error() string {
	if e.Err == nil {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Err.Error()
}

func (e *UpdateError) Unwrap() error { return e.Err }

func ErrorCodeOf(err error) ErrorCode {
	var updateErr *UpdateError
	if errors.As(err, &updateErr) {
		return updateErr.Code
	}
	return ""
}

var (
	ErrUpdateNotFound            = errors.New("desktop_update: update not found")
	ErrUpdateOperationNotFound   = errors.New("desktop_update: update operation not found")
	ErrDownloadFailed            = errors.New("desktop_update: download failed")
	ErrDownloadCancelled         = errors.New("desktop_update: download cancelled")
	ErrHashMismatch              = errors.New("desktop_update: hash mismatch")
	ErrSignatureInvalid          = errors.New("desktop_update: signature invalid")
	ErrPublisherChanged          = errors.New("desktop_update: publisher changed")
	ErrPreflightFailed           = errors.New("desktop_update: preflight check failed")
	ErrHealthCheckFailed         = errors.New("desktop_update: health check failed")
	ErrRollbackFailed            = errors.New("desktop_update: rollback failed")
	ErrRecoveryRequired          = errors.New("desktop_update: recovery required")
	ErrManualIntervention        = errors.New("desktop_update: manual intervention required")
	ErrMigrationFailed           = errors.New("desktop_update: migration failed")
	ErrDrainTimeout              = errors.New("desktop_update: drain timeout")
	ErrGenerationSwitchFailed    = errors.New("desktop_update: generation switch failed")
	ErrCommitFailed              = errors.New("desktop_update: commit failed")
	ErrUpdateConflict            = errors.New("desktop_update: update conflict")
	ErrUpdateSourceDisabled      = errors.New("desktop_update: update source disabled")
	ErrUpdateSourceNotTrusted    = errors.New("desktop_update: update source not trusted")
	ErrVersionNotCompatible      = errors.New("desktop_update: version not compatible")
	ErrPlatformNotSupported      = errors.New("desktop_update: platform not supported")
	ErrUpdateAlreadyRunning      = errors.New("desktop_update: update already running")
	ErrUpdateCancelled           = errors.New("desktop_update: update cancelled")
	ErrInvalidUpdateSource       = errors.New("desktop_update: invalid update source")
	ErrInvalidMetadata           = errors.New("desktop_update: invalid metadata")
	ErrDownloadTimeout           = errors.New("desktop_update: download timeout")
	ErrDownloadRedirect          = errors.New("desktop_update: download redirect limit exceeded")
	ErrDownloadSizeExceeded      = errors.New("desktop_update: download size exceeded")
	ErrDownloadProtocolForbidden = errors.New("desktop_update: download protocol forbidden")
	ErrQuarantineTriggered       = errors.New("desktop_update: quarantine triggered")
	ErrIndexInvalid              = errors.New("desktop_update: release index invalid")
	ErrIndexSignatureInvalid     = errors.New("desktop_update: release index signature invalid")
	ErrRegistryNetwork           = errors.New("desktop_update: registry network error")
)
