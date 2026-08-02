// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import "errors"

const (
	ErrCodeRevisionNotFound           = "QUALITY_REVISION_NOT_FOUND"
	ErrCodeMeasurementMissing         = "QUALITY_MEASUREMENT_MISSING"
	ErrCodeProfileInvalid             = "QUALITY_PROFILE_INVALID"
	ErrCodeInputHashMismatch          = "QUALITY_INPUT_HASH_MISMATCH"
	ErrCodeEvaluationConflict         = "QUALITY_EVALUATION_CONFLICT"
	ErrCodeExecutionOwnershipLost     = "QUALITY_EXECUTION_OWNERSHIP_LOST"
	ErrCodeDetectorFailed             = "QUALITY_DETECTOR_FAILED"
	ErrCodeFeatureProviderUnavailable = "QUALITY_FEATURE_PROVIDER_UNAVAILABLE"
	ErrCodeScoreInvalid               = "QUALITY_SCORE_INVALID"
	ErrCodeReportWriteFailed          = "QUALITY_REPORT_WRITE_FAILED"
	ErrCodeDatabaseCommitFailed       = "QUALITY_DATABASE_COMMIT_FAILED"
	ErrCodeCancelled                  = "QUALITY_CANCELLED"
	ErrCodeGateIncomplete             = "QUALITY_GATE_INCOMPLETE"
	ErrCodeLegacyUnverified           = "QUALITY_LEGACY_UNVERIFIED"
	ErrCodeFrameDecodeFailed          = "QUALITY_FRAME_DECODE_FAILED"
	ErrCodeContentHashMismatch        = "QUALITY_CONTENT_HASH_MISMATCH"
	ErrCodeActionRevisionNotFound     = "action_revision_not_found"
	ErrCodeActiveBindingConflict      = "quality_active_binding_conflict"
	ErrCodeWritebackStaleRevision     = "quality_writeback_stale_revision"
	ErrCodeMeasurementIncomplete      = "measurement_incomplete"
	ErrCodeLegacyInvalidRevision      = "legacy_invalid_revision_binding"
	ErrCodeQualityDisabled            = "desktop_pet_quality_disabled"
	ErrCodeQualityEvaluationNotFound  = "QUALITY_EVALUATION_NOT_FOUND"
	ErrCodeQualityNotOwned            = "QUALITY_NOT_OWNED"
)

var (
	ErrRevisionNotFound       = errors.New("quality: action revision not found")
	ErrMeasurementMissing     = errors.New("quality: measurement missing")
	ErrProfileInvalid         = errors.New("quality: profile invalid")
	ErrInputHashMismatch      = errors.New("quality: input hash mismatch")
	ErrEvaluationConflict     = errors.New("quality: evaluation conflict")
	ErrExecutionOwnershipLost = errors.New("quality: execution ownership lost")
	ErrDetectorFailed         = errors.New("quality: detector failed")
	ErrScoreInvalid           = errors.New("quality: score invalid")
	ErrReportWriteFailed      = errors.New("quality: report write failed")
	ErrDatabaseCommitFailed   = errors.New("quality: database commit failed")
	ErrCancelled              = errors.New("quality: cancelled")
	ErrGateIncomplete         = errors.New("quality: gate incomplete")
	ErrFrameDecodeFailed      = errors.New("quality: frame decode failed")
	ErrContentHashMismatch    = errors.New("quality: content hash mismatch")
	ErrActionRevisionNotFound = errors.New("quality: action revision not found")
	ErrActiveBindingConflict  = errors.New("quality: active binding conflict")
	ErrWritebackStaleRevision = errors.New("quality: writeback stale revision")
	ErrMeasurementIncomplete  = errors.New("quality: measurement incomplete")
)

type QualityError struct {
	Code    string
	Message string
	Cause   error
}

func (e *QualityError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *QualityError) Unwrap() error {
	return e.Cause
}

func NewQualityError(code, message string, cause error) *QualityError {
	return &QualityError{Code: code, Message: message, Cause: cause}
}

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var qe *QualityError
	if errors.As(err, &qe) {
		return qe.Code
	}
	switch {
	case errors.Is(err, ErrRevisionNotFound):
		return ErrCodeRevisionNotFound
	case errors.Is(err, ErrMeasurementMissing):
		return ErrCodeMeasurementMissing
	case errors.Is(err, ErrProfileInvalid):
		return ErrCodeProfileInvalid
	case errors.Is(err, ErrInputHashMismatch):
		return ErrCodeInputHashMismatch
	case errors.Is(err, ErrEvaluationConflict):
		return ErrCodeEvaluationConflict
	case errors.Is(err, ErrExecutionOwnershipLost):
		return ErrCodeExecutionOwnershipLost
	case errors.Is(err, ErrDetectorFailed):
		return ErrCodeDetectorFailed
	case errors.Is(err, ErrScoreInvalid):
		return ErrCodeScoreInvalid
	case errors.Is(err, ErrReportWriteFailed):
		return ErrCodeReportWriteFailed
	case errors.Is(err, ErrDatabaseCommitFailed):
		return ErrCodeDatabaseCommitFailed
	case errors.Is(err, ErrCancelled):
		return ErrCodeCancelled
	case errors.Is(err, ErrGateIncomplete):
		return ErrCodeGateIncomplete
	case errors.Is(err, ErrFrameDecodeFailed):
		return ErrCodeFrameDecodeFailed
	case errors.Is(err, ErrContentHashMismatch):
		return ErrCodeContentHashMismatch
	case errors.Is(err, ErrActionRevisionNotFound):
		return ErrCodeActionRevisionNotFound
	case errors.Is(err, ErrActiveBindingConflict):
		return ErrCodeActiveBindingConflict
	case errors.Is(err, ErrWritebackStaleRevision):
		return ErrCodeWritebackStaleRevision
	case errors.Is(err, ErrMeasurementIncomplete):
		return ErrCodeMeasurementIncomplete
	}
	return "QUALITY_UNKNOWN"
}

func IsRetryable(err error) bool {
	return IsRetryableErrorCode(ErrorCode(err))
}
