// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

const (
	ErrCodeCanvasNormalizationFailed = "CANVAS_NORMALIZATION_FAILED"
	ErrCodeSubjectAlignmentFailed    = "SUBJECT_ALIGNMENT_FAILED"
	ErrCodeSubjectOutOfBounds        = "SUBJECT_OUT_OF_BOUNDS"
	ErrCodeAnchorCalculationFailed   = "ANCHOR_CALCULATION_FAILED"

	ErrCodeFrameQualityCheckFailed = "FRAME_QUALITY_CHECK_FAILED"
	ErrCodeActionFrameCountInvalid = "ACTION_FRAME_COUNT_INVALID"
	ErrCodeLoopDiscontinuity       = "LOOP_DISCONTINUITY"
	ErrCodeExcessiveFrameDrift     = "EXCESSIVE_FRAME_DRIFT"

	ErrCodeDataConflict              = "PROCESSING_DATA_CONFLICT"
	ErrCodeDuplicateIdentity         = "PROCESSING_DUPLICATE_IDENTITY"
	ErrCodeExecutionOwnershipLost    = "PROCESSING_EXECUTION_OWNERSHIP_LOST"
	ErrCodeAttemptConflict           = "PROCESSING_ATTEMPT_CONFLICT"
	ErrCodeMigrationUnresolved       = "PROCESSING_MIGRATION_UNRESOLVED"
	ErrCodeVersionAllocationConflict = "PROCESSING_VERSION_ALLOCATION_CONFLICT"
)
