package packageformat

import "fmt"

type ErrorCode string

const (
	ErrCodePetIdentityConflict                  ErrorCode = "PET_IDENTITY_CONFLICT"
	ErrCodeReleaseVersionConflict               ErrorCode = "RELEASE_VERSION_CONFLICT"
	ErrCodeReleaseContentConflict               ErrorCode = "RELEASE_CONTENT_CONFLICT"
	ErrCodePackageSchemaUnsupported             ErrorCode = "PACKAGE_SCHEMA_UNSUPPORTED"
	ErrCodePackageSchemaMissing                 ErrorCode = "PACKAGE_SCHEMA_MISSING"
	ErrCodePackageManifestInvalid               ErrorCode = "PACKAGE_MANIFEST_INVALID"
	ErrCodePackagePathInvalid                   ErrorCode = "PACKAGE_PATH_INVALID"
	ErrCodePackageArchiveBomb                   ErrorCode = "PACKAGE_ARCHIVE_BOMB"
	ErrCodePackageDuplicateEntry                ErrorCode = "PACKAGE_DUPLICATE_ENTRY"
	ErrCodePackageExecutableForbidden           ErrorCode = "PACKAGE_EXECUTABLE_FORBIDDEN"
	ErrCodePackageSymlinkForbidden              ErrorCode = "PACKAGE_SYMLINK_FORBIDDEN"
	ErrCodePackageHashMismatch                  ErrorCode = "PACKAGE_HASH_MISMATCH"
	ErrCodePackageFileUndeclared                ErrorCode = "PACKAGE_FILE_UNDECLARED"
	ErrCodePackageFileMissing                   ErrorCode = "PACKAGE_FILE_MISSING"
	ErrCodePackageRuntimeIncompatible           ErrorCode = "PACKAGE_RUNTIME_INCOMPATIBLE"
	ErrCodePackageQualityGateBlocked            ErrorCode = "PACKAGE_QUALITY_GATE_BLOCKED"
	ErrCodePackageBindingNotAllowed             ErrorCode = "PACKAGE_BINDING_NOT_ALLOWED"
	ErrCodeActionConfigMissing                  ErrorCode = "ACTION_CONFIG_MISSING"
	ErrCodeActionConfigInvalid                  ErrorCode = "ACTION_CONFIG_INVALID"
	ErrCodeActionReferenceInvalid               ErrorCode = "ACTION_REFERENCE_INVALID"
	ErrCodeFrameMissing                         ErrorCode = "FRAME_MISSING"
	ErrCodeFrameHashMismatch                    ErrorCode = "FRAME_HASH_MISMATCH"
	ErrCodePathEscapeRejected                   ErrorCode = "PATH_ESCAPE_REJECTED"
	ErrCodeDefaultActionInvalid                 ErrorCode = "DEFAULT_ACTION_INVALID"
	ErrCodeRuntimeVersionUnsupported            ErrorCode = "RUNTIME_VERSION_UNSUPPORTED"
	ErrCodePackageJsonDuplicateKey              ErrorCode = "PACKAGE_JSON_DUPLICATE_KEY"
	ErrCodePackageJsonUnknownField              ErrorCode = "PACKAGE_JSON_UNKNOWN_FIELD"
	ErrCodePackageManifestHashMissing           ErrorCode = "PACKAGE_MANIFEST_HASH_MISSING"
	ErrCodePackageManifestHashMismatch          ErrorCode = "PACKAGE_MANIFEST_HASH_MISMATCH"
	ErrCodePackageIntegrityMissing              ErrorCode = "PACKAGE_INTEGRITY_MISSING"
	ErrCodePackageIntegrityAlgorithmUnsupported ErrorCode = "PACKAGE_INTEGRITY_ALGORITHM_UNSUPPORTED"
	ErrCodePackageMediaTypeMismatch             ErrorCode = "PACKAGE_MEDIA_TYPE_MISMATCH"
	ErrCodePackageActionSummaryMismatch         ErrorCode = "PACKAGE_ACTION_SUMMARY_MISMATCH"
	ErrCodePackageActionConfigSchemaUnsupported ErrorCode = "PACKAGE_ACTION_CONFIG_SCHEMA_UNSUPPORTED"
	ErrCodePackageActionKeyMismatch             ErrorCode = "PACKAGE_ACTION_KEY_MISMATCH"
	ErrCodePackageFrameIndexInvalid             ErrorCode = "PACKAGE_FRAME_INDEX_INVALID"
	ErrCodePackageFrameIdDuplicate              ErrorCode = "PACKAGE_FRAME_ID_DUPLICATE"
	ErrCodePackageFrameAssetIdMissing           ErrorCode = "PACKAGE_FRAME_ASSET_ID_MISSING"
	ErrCodePackageRuntimeVersionInvalid         ErrorCode = "PACKAGE_RUNTIME_VERSION_INVALID"
	ErrCodePackageRuntimeVersionRangeInvalid    ErrorCode = "PACKAGE_RUNTIME_VERSION_RANGE_INVALID"
	ErrCodePackagePreviewUndeclared             ErrorCode = "PACKAGE_PREVIEW_UNDECLARED"
	ErrCodePackageResourceNotDeclared           ErrorCode = "PACKAGE_RESOURCE_NOT_DECLARED"
	ErrCodePackageResourceHashMismatch          ErrorCode = "PACKAGE_RESOURCE_HASH_MISMATCH"
)

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

type ValidationError struct {
	Code        ErrorCode
	Severity    string
	Path        string
	ActionKey   string
	Message     string
	Expected    string
	Actual      string
	Remediation string
}

func (e *ValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("[%s] %s: %s (path=%s)", e.Severity, e.Code, e.Message, e.Path)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Severity, e.Code, e.Message)
}

type PackageError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *PackageError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *PackageError) Unwrap() error {
	return e.Err
}

func NewPackageError(code ErrorCode, message string, err error) *PackageError {
	return &PackageError{Code: code, Message: message, Err: err}
}

func NewValidationError(code ErrorCode, severity, path, message string) *ValidationError {
	return &ValidationError{
		Code:     code,
		Severity: severity,
		Path:     path,
		Message:  message,
	}
}
