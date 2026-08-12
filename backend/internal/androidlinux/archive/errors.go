//go:build linux && !android

package archive

import "fmt"

type Error struct {
	code    string
	message string
}

func (e *Error) Error() string {
	return e.code + ": " + e.message
}

func (e *Error) Code() string {
	return e.code
}

const (
	ErrCodeInvalidRequest            = "archive.invalid_request"
	ErrCodeFormatUnsupported         = "archive.format_unsupported"
	ErrCodeFormatRequired            = "archive.format_required"
	ErrCodeNotArchive                = "archive.not_archive"
	ErrCodeCorrupt                   = "archive.corrupt"
	ErrCodeEntryUnsafe               = "archive.entry_unsafe"
	ErrCodePathEscape                = "archive.path_escape"
	ErrCodeSymlinkForbidden          = "archive.symlink_forbidden"
	ErrCodeHardlinkForbidden         = "archive.hardlink_forbidden"
	ErrCodeSpecialFileForbidden      = "archive.special_file_forbidden"
	ErrCodeTooManyEntries            = "archive.too_many_entries"
	ErrCodeTooLarge                  = "archive.too_large"
	ErrCodeEntryTooLarge             = "archive.entry_too_large"
	ErrCodeCompressionRatioExceeded  = "archive.compression_ratio_exceeded"
	ErrCodeEncryptedUnsupported      = "archive.encrypted_unsupported"
	ErrCodeTargetExists              = "archive.target_exists"
	ErrCodeSourceNotFound            = "archive.source_not_found"
	ErrCodeTargetInvalid             = "archive.target_invalid"
	ErrCodeReadFailed                = "archive.read_failed"
	ErrCodeWriteFailed               = "archive.write_failed"
	ErrCodeVerifyFailed              = "archive.verify_failed"
	ErrCodeTimeout                   = "archive.timeout"
	ErrCodeCancelled                 = "archive.cancelled"
)

func ErrInvalidRequest(msg string) *Error {
	return &Error{code: ErrCodeInvalidRequest, message: msg}
}

func ErrFormatUnsupported(format string) *Error {
	return &Error{code: ErrCodeFormatUnsupported, message: "unsupported format: " + format}
}

func ErrFormatRequired() *Error {
	return &Error{code: ErrCodeFormatRequired, message: "cannot determine archive format"}
}

func ErrNotArchive(path string) *Error {
	return &Error{code: ErrCodeNotArchive, message: "not a valid archive: " + path}
}

func ErrCorrupt(reason string) *Error {
	return &Error{code: ErrCodeCorrupt, message: "archive is corrupt: " + reason}
}

func ErrEntryUnsafe(name string) *Error {
	return &Error{code: ErrCodeEntryUnsafe, message: "unsafe archive entry: " + name}
}

func ErrPathEscape(name string) *Error {
	return &Error{code: ErrCodePathEscape, message: "path escape attempt: " + name}
}

func ErrSymlinkForbidden(name string) *Error {
	return &Error{code: ErrCodeSymlinkForbidden, message: "symlink entry forbidden: " + name}
}

func ErrHardlinkForbidden(name string) *Error {
	return &Error{code: ErrCodeHardlinkForbidden, message: "hardlink entry forbidden: " + name}
}

func ErrSpecialFileForbidden(name string) *Error {
	return &Error{code: ErrCodeSpecialFileForbidden, message: "special file entry forbidden: " + name}
}

func ErrTooManyEntries(max int) *Error {
	return &Error{code: ErrCodeTooManyEntries, message: fmt.Sprintf("too many entries, max: %d", max)}
}

func ErrTooLarge(maxBytes int64) *Error {
	return &Error{code: ErrCodeTooLarge, message: fmt.Sprintf("archive too large, max: %d bytes", maxBytes)}
}

func ErrEntryTooLarge(name string, maxBytes int64) *Error {
	return &Error{code: ErrCodeEntryTooLarge, message: fmt.Sprintf("entry %s too large, max: %d bytes", name, maxBytes)}
}

func ErrCompressionRatioExceeded(ratio float64) *Error {
	return &Error{code: ErrCodeCompressionRatioExceeded, message: fmt.Sprintf("compression ratio exceeded: %.0f", ratio)}
}

func ErrEncryptedUnsupported() *Error {
	return &Error{code: ErrCodeEncryptedUnsupported, message: "encrypted archives are not supported"}
}

func ErrTargetExists(path string) *Error {
	return &Error{code: ErrCodeTargetExists, message: "target already exists: " + path}
}

func ErrSourceNotFound(path string) *Error {
	return &Error{code: ErrCodeSourceNotFound, message: "source not found: " + path}
}

func ErrTargetInvalid(reason string) *Error {
	return &Error{code: ErrCodeTargetInvalid, message: "invalid target: " + reason}
}

func ErrReadFailed(reason string) *Error {
	return &Error{code: ErrCodeReadFailed, message: "read failed: " + reason}
}

func ErrWriteFailed(reason string) *Error {
	return &Error{code: ErrCodeWriteFailed, message: "write failed: " + reason}
}

func ErrVerifyFailed(reason string) *Error {
	return &Error{code: ErrCodeVerifyFailed, message: "verify failed: " + reason}
}

func ErrTimeout() *Error {
	return &Error{code: ErrCodeTimeout, message: "operation timed out"}
}

func ErrCancelled() *Error {
	return &Error{code: ErrCodeCancelled, message: "operation cancelled"}
}
