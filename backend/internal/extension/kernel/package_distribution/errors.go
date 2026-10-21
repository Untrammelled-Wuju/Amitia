package package_distribution

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
	ErrDownloadNotFound          = errors.New("package_distribution: download not found")
	ErrDownloadFailed            = errors.New("package_distribution: download failed")
	ErrDownloadCancelled         = errors.New("package_distribution: download cancelled")
	ErrHashMismatch              = errors.New("package_distribution: hash mismatch")
	ErrUpdateConflict            = errors.New("package_distribution: download conflict")
	ErrInvalidMetadata           = errors.New("package_distribution: invalid metadata")
	ErrVersionNotCompatible      = errors.New("package_distribution: version not compatible")
	ErrPlatformNotSupported      = errors.New("package_distribution: platform not supported")
	ErrDownloadRedirect          = errors.New("package_distribution: download redirect limit exceeded")
	ErrDownloadSizeExceeded      = errors.New("package_distribution: download size exceeded")
	ErrDownloadProtocolForbidden = errors.New("package_distribution: download protocol forbidden")
	ErrIndexInvalid              = errors.New("package_distribution: release index invalid")
	ErrIndexSignatureInvalid     = errors.New("package_distribution: release index signature invalid")
	ErrRegistryNetwork           = errors.New("package_distribution: registry network error")
	ErrRegistryNotConfigured     = errors.New("package_distribution: registry not configured")
)
