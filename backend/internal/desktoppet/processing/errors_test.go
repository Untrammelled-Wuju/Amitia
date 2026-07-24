// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"testing"

	"github.com/u-ai/backend/pkg/comment/response"
)

func TestErrorCodes_Defined(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"ErrCodeCanvasNormalizationFailed", ErrCodeCanvasNormalizationFailed, "CANVAS_NORMALIZATION_FAILED"},
		{"ErrCodeSubjectAlignmentFailed", ErrCodeSubjectAlignmentFailed, "SUBJECT_ALIGNMENT_FAILED"},
		{"ErrCodeSubjectOutOfBounds", ErrCodeSubjectOutOfBounds, "SUBJECT_OUT_OF_BOUNDS"},
		{"ErrCodeAnchorCalculationFailed", ErrCodeAnchorCalculationFailed, "ANCHOR_CALCULATION_FAILED"},
		{"ErrCodeFrameQualityCheckFailed", ErrCodeFrameQualityCheckFailed, "FRAME_QUALITY_CHECK_FAILED"},
		{"ErrCodeActionFrameCountInvalid", ErrCodeActionFrameCountInvalid, "ACTION_FRAME_COUNT_INVALID"},
		{"ErrCodeLoopDiscontinuity", ErrCodeLoopDiscontinuity, "LOOP_DISCONTINUITY"},
		{"ErrCodeExcessiveFrameDrift", ErrCodeExcessiveFrameDrift, "EXCESSIVE_FRAME_DRIFT"},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestMapProcessingErrorCode_NewCodes(t *testing.T) {
	cases := []struct {
		code    string
		want    int
	}{
		{ErrCodeCanvasNormalizationFailed, response.OperationFailed},
		{ErrCodeSubjectAlignmentFailed, response.OperationFailed},
		{ErrCodeSubjectOutOfBounds, response.OperationFailed},
		{ErrCodeAnchorCalculationFailed, response.OperationFailed},
		{ErrCodeFrameQualityCheckFailed, response.OperationFailed},
		{ErrCodeActionFrameCountInvalid, response.InvalidParams},
		{ErrCodeLoopDiscontinuity, response.BusinessError},
		{ErrCodeExcessiveFrameDrift, response.BusinessError},
	}

	for _, c := range cases {
		got := mapProcessingErrorCode(c.code)
		if got == 0 {
			t.Errorf("mapProcessingErrorCode(%q) = 0, want non-zero", c.code)
		}
		if got == response.InternalError {
			t.Errorf("mapProcessingErrorCode(%q) = InternalError (default), want explicit mapping", c.code)
		}
		if got != c.want {
			t.Errorf("mapProcessingErrorCode(%q) = %d, want %d", c.code, got, c.want)
		}
	}
}
