// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"image"
	"image/color"
	"testing"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

func hasFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

func TestQualityCheckFrameNormal(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 2, 2, 98, 98, color.NRGBA{255, 255, 255, 255})
	box := backgroundremoval.SubjectBox{MinX: 2, MinY: 2, MaxX: 97, MaxY: 97, Width: 96, Height: 96, Empty: false}

	c := NewQualityChecker()
	result := c.CheckFrame(img, box, "")

	if len(result.QualityFlags) != 0 {
		t.Errorf("expected no flags for normal frame, got %v", result.QualityFlags)
	}
	if result.QualityLevel != QualityLevelNormal {
		t.Errorf("expected normal level, got %s", result.QualityLevel)
	}
	if result.ContentHash == "" {
		t.Errorf("expected non-empty content hash")
	}
	if result.AlphaCoverage <= 0 {
		t.Errorf("expected positive alpha coverage, got %v", result.AlphaCoverage)
	}
}

func TestQualitySubjectTooSmall(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 40, 40, 60, 60, color.NRGBA{255, 255, 255, 255})
	box := backgroundremoval.SubjectBox{MinX: 40, MinY: 40, MaxX: 59, MaxY: 59, Width: 20, Height: 20, Empty: false}

	c := NewQualityChecker()
	result := c.CheckFrame(img, box, "")

	if !hasFlag(result.QualityFlags, FlagSubjectTooSmall) {
		t.Errorf("expected SUBJECT_TOO_SMALL flag, got %v", result.QualityFlags)
	}
	if result.QualityLevel != QualityLevelWarning {
		t.Errorf("expected warning level, got %s", result.QualityLevel)
	}
}

func TestQualitySubjectTooLarge(t *testing.T) {
	img := newTestImage(200, 200, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 2, 2, 198, 198, color.NRGBA{255, 255, 255, 255})
	box := backgroundremoval.SubjectBox{MinX: 2, MinY: 2, MaxX: 197, MaxY: 197, Width: 196, Height: 196, Empty: false}

	c := NewQualityChecker()
	result := c.CheckFrame(img, box, "")

	if !hasFlag(result.QualityFlags, FlagSubjectTooLarge) {
		t.Errorf("expected SUBJECT_TOO_LARGE flag, got %v", result.QualityFlags)
	}
	if result.QualityLevel != QualityLevelFailed {
		t.Errorf("expected failed level, got %s", result.QualityLevel)
	}
	if !IsAutoFail(result.QualityFlags) {
		t.Errorf("expected auto fail")
	}
}

func TestQualitySubjectTouchesEdge(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 0, 0, 40, 40, color.NRGBA{255, 255, 255, 255})
	box := backgroundremoval.SubjectBox{MinX: 0, MinY: 0, MaxX: 39, MaxY: 39, Width: 40, Height: 40, Empty: false}

	c := NewQualityChecker()
	result := c.CheckFrame(img, box, "")

	if !hasFlag(result.QualityFlags, FlagSubjectTouchesEdge) {
		t.Errorf("expected SUBJECT_TOUCHES_EDGE flag, got %v", result.QualityFlags)
	}
	if result.QualityLevel != QualityLevelWarning {
		t.Errorf("expected warning level, got %s", result.QualityLevel)
	}
}

func TestQualityAlphaFullyTransparent(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	box := backgroundremoval.SubjectBox{Empty: true}

	c := NewQualityChecker()
	result := c.CheckFrame(img, box, "")

	if !hasFlag(result.QualityFlags, FlagEmptyFrame) {
		t.Errorf("expected EMPTY_FRAME flag, got %v", result.QualityFlags)
	}
	if result.QualityLevel != QualityLevelFailed {
		t.Errorf("expected failed level, got %s", result.QualityLevel)
	}
	if result.AlphaCoverage != 0 {
		t.Errorf("expected zero alpha coverage, got %v", result.AlphaCoverage)
	}
}

func TestQualitySubjectMissing(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 30, 30, 70, 70, color.NRGBA{255, 255, 255, 255})
	box := backgroundremoval.SubjectBox{Empty: true}

	c := NewQualityChecker()
	result := c.CheckFrame(img, box, "")

	if !hasFlag(result.QualityFlags, FlagEmptyFrame) {
		t.Errorf("expected EMPTY_FRAME flag, got %v", result.QualityFlags)
	}
	if result.QualityLevel != QualityLevelFailed {
		t.Errorf("expected failed level, got %s", result.QualityLevel)
	}
}

func TestQualityEmptyFrame(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	img.Set(50, 50, color.NRGBA{255, 255, 255, 255})
	box := backgroundremoval.SubjectBox{MinX: 50, MinY: 50, MaxX: 50, MaxY: 50, Width: 1, Height: 1, Empty: false}

	c := NewQualityChecker()
	result := c.CheckFrame(img, box, "")

	if !hasFlag(result.QualityFlags, FlagEmptyFrame) {
		t.Errorf("expected EMPTY_FRAME flag, got %v", result.QualityFlags)
	}
	if result.AlphaCoverage >= emptyFrameCoverageThreshold {
		t.Errorf("expected alpha coverage below %v, got %v", emptyFrameCoverageThreshold, result.AlphaCoverage)
	}
}

func TestQualityDuplicateFrame(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 30, 30, 70, 70, color.NRGBA{255, 0, 0, 255})
	box := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}

	prevHash := computeContentHash(img)

	c := NewQualityChecker()
	result := c.CheckFrame(img, box, prevHash)

	if !hasFlag(result.QualityFlags, FlagDuplicateFrame) {
		t.Errorf("expected DUPLICATE_FRAME flag, got %v", result.QualityFlags)
	}
	if result.QualityLevel != QualityLevelWarning {
		t.Errorf("expected warning level, got %s", result.QualityLevel)
	}
}

func TestQualityAnchorDrift(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 30, 30, 70, 70, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 60, 30, 100, 70, color.NRGBA{0, 255, 0, 255})

	box1 := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}
	box2 := backgroundremoval.SubjectBox{MinX: 60, MinY: 30, MaxX: 99, MaxY: 69, Width: 40, Height: 40, Empty: false}

	c := NewQualityChecker()
	results := c.CheckFrames([]image.Image{img1, img2}, []backgroundremoval.SubjectBox{box1, box2})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !hasFlag(results[1].QualityFlags, FlagAnchorDrift) {
		t.Errorf("expected ANCHOR_DRIFT flag on second frame, got %v", results[1].QualityFlags)
	}
}

func TestQualityScaleDrift(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 30, 30, 70, 70, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 30, 30, 70, 100, color.NRGBA{0, 255, 0, 255})

	box1 := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}
	box2 := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 99, Width: 40, Height: 70, Empty: false}

	c := NewQualityChecker()
	results := c.CheckFrames([]image.Image{img1, img2}, []backgroundremoval.SubjectBox{box1, box2})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !hasFlag(results[1].QualityFlags, FlagScaleDrift) {
		t.Errorf("expected SCALE_DRIFT flag on second frame, got %v", results[1].QualityFlags)
	}
}

func TestQualityBackgroundResidue(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 30, 30, 70, 70, color.NRGBA{255, 255, 255, 255})
	box := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}

	c := NewQualityChecker()
	result := c.CheckFrame(img, box, "")

	if !hasFlag(result.QualityFlags, FlagBackgroundResidue) {
		t.Errorf("expected BACKGROUND_RESIDUE flag, got %v", result.QualityFlags)
	}
	if result.AlphaCoverage >= backgroundResidueCoverageThreshold {
		t.Errorf("expected alpha coverage below %v, got %v", backgroundResidueCoverageThreshold, result.AlphaCoverage)
	}
}

func TestQualityDetermineLevel(t *testing.T) {
	if DetermineQualityLevel(nil) != QualityLevelNormal {
		t.Errorf("nil flags should be normal")
	}
	if DetermineQualityLevel([]string{}) != QualityLevelNormal {
		t.Errorf("empty flags should be normal")
	}
	if DetermineQualityLevel([]string{FlagSubjectTooSmall}) != QualityLevelWarning {
		t.Errorf("SUBJECT_TOO_SMALL should be warning")
	}
	if DetermineQualityLevel([]string{FlagDuplicateFrame}) != QualityLevelWarning {
		t.Errorf("DUPLICATE_FRAME should be warning")
	}
	if DetermineQualityLevel([]string{FlagSubjectTooLarge}) != QualityLevelFailed {
		t.Errorf("SUBJECT_TOO_LARGE should be failed")
	}
	if DetermineQualityLevel([]string{FlagEmptyFrame}) != QualityLevelFailed {
		t.Errorf("EMPTY_FRAME should be failed")
	}
	if DetermineQualityLevel([]string{FlagSourceMissing}) != QualityLevelFailed {
		t.Errorf("SOURCE_MISSING should be failed")
	}
	if DetermineQualityLevel([]string{FlagAlphaInvalid}) != QualityLevelFailed {
		t.Errorf("ALPHA_INVALID should be failed")
	}
	if DetermineQualityLevel([]string{FlagSubjectTooSmall, FlagDuplicateFrame}) != QualityLevelWarning {
		t.Errorf("warning flags should be warning")
	}
	if DetermineQualityLevel([]string{FlagSubjectTooSmall, FlagSubjectTooLarge}) != QualityLevelFailed {
		t.Errorf("mixed with auto-fail should be failed")
	}
}

func TestQualityIsAutoFail(t *testing.T) {
	if !IsAutoFail([]string{FlagSubjectTooLarge}) {
		t.Errorf("SUBJECT_TOO_LARGE should auto fail")
	}
	if !IsAutoFail([]string{FlagEmptyFrame}) {
		t.Errorf("EMPTY_FRAME should auto fail")
	}
	if !IsAutoFail([]string{FlagSourceMissing}) {
		t.Errorf("SOURCE_MISSING should auto fail")
	}
	if !IsAutoFail([]string{FlagAlphaInvalid}) {
		t.Errorf("ALPHA_INVALID should auto fail")
	}
	if IsAutoFail([]string{FlagSubjectTooSmall}) {
		t.Errorf("SUBJECT_TOO_SMALL should not auto fail")
	}
	if IsAutoFail([]string{FlagDuplicateFrame}) {
		t.Errorf("DUPLICATE_FRAME should not auto fail")
	}
	if IsAutoFail([]string{FlagBackgroundResidue}) {
		t.Errorf("BACKGROUND_RESIDUE should not auto fail")
	}
	if IsAutoFail([]string{FlagAnchorDrift}) {
		t.Errorf("ANCHOR_DRIFT should not auto fail")
	}
	if IsAutoFail([]string{FlagScaleDrift}) {
		t.Errorf("SCALE_DRIFT should not auto fail")
	}
	if IsAutoFail(nil) {
		t.Errorf("nil flags should not auto fail")
	}
	if IsAutoFail([]string{}) {
		t.Errorf("empty flags should not auto fail")
	}
}

func TestQualityCheckFramesBatch(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 30, 30, 70, 70, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 30, 30, 70, 70, color.NRGBA{0, 255, 0, 255})

	box1 := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}
	box2 := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}

	c := NewQualityChecker()
	results := c.CheckFrames([]image.Image{img1, img2}, []backgroundremoval.SubjectBox{box1, box2})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].FrameIndex != 0 {
		t.Errorf("expected frame index 0, got %d", results[0].FrameIndex)
	}
	if results[1].FrameIndex != 1 {
		t.Errorf("expected frame index 1, got %d", results[1].FrameIndex)
	}
	if results[0].ContentHash == "" {
		t.Errorf("expected non-empty hash for frame 0")
	}
	if results[1].ContentHash == "" {
		t.Errorf("expected non-empty hash for frame 1")
	}
}

func TestQualityNilImage(t *testing.T) {
	box := backgroundremoval.SubjectBox{Empty: true}
	c := NewQualityChecker()
	result := c.CheckFrame(nil, box, "")

	if !hasFlag(result.QualityFlags, FlagSourceMissing) {
		t.Errorf("expected SOURCE_MISSING flag, got %v", result.QualityFlags)
	}
	if result.QualityLevel != QualityLevelFailed {
		t.Errorf("expected failed level, got %s", result.QualityLevel)
	}
	if result.Error == nil {
		t.Errorf("expected error for nil image")
	}
}
