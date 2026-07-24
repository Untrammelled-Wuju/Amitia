// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"image"
	"image/color"
	"testing"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

func TestLoopIsLoopAction(t *testing.T) {
	loopKeys := []string{
		"idle_normal",
		"idle_breathing",
		"idle_blink",
		"idle_sway",
		"walk_left",
		"walk_right",
		"run_left",
		"run_right",
	}
	for _, key := range loopKeys {
		if !IsLoopAction(key) {
			t.Errorf("expected %q to be a loop action", key)
		}
	}

	nonLoopKeys := []string{
		"wave",
		"happy",
		"speaking",
		"fall",
		"picked_up",
		"sleep_start",
		"sleep_end",
		"",
		"unknown",
	}
	for _, key := range nonLoopKeys {
		if IsLoopAction(key) {
			t.Errorf("expected %q to NOT be a loop action", key)
		}
	}
}

func TestLoopCheckNonLoopAction(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 30, 30, 70, 70, color.NRGBA{255, 255, 255, 255})
	box := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}

	c := NewLoopChecker()
	result := c.CheckLoop("wave", []image.Image{img, img}, []backgroundremoval.SubjectBox{box, box})

	if result.IsLoopAction {
		t.Errorf("expected non-loop action for wave")
	}
}

func TestLoopCheckContinuousLoop(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 30, 30, 70, 70, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 32, 30, 72, 70, color.NRGBA{0, 255, 0, 255})

	box1 := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}
	box2 := backgroundremoval.SubjectBox{MinX: 32, MinY: 30, MaxX: 71, MaxY: 69, Width: 40, Height: 40, Empty: false}

	imgs := []image.Image{img1, img2, img1}
	boxes := []backgroundremoval.SubjectBox{box1, box2, box1}

	c := NewLoopChecker()
	result := c.CheckLoop("idle_normal", imgs, boxes)

	if !result.IsLoopAction {
		t.Errorf("expected loop action for idle_normal")
	}
	if result.HeadTailDiff > defaultDiscontinuityThreshold {
		t.Errorf("expected low head-tail diff, got %v", result.HeadTailDiff)
	}
	if result.IsDiscontinuous {
		t.Errorf("expected continuous loop, got discontinuous")
	}
	if hasFlag(result.QualityFlags, FlagLoopDiscontinuity) {
		t.Errorf("expected no LOOP_DISCONTINUITY flag, got %v", result.QualityFlags)
	}
	if len(result.AdjustedFrames) != 2 {
		t.Errorf("expected 2 adjusted frames after removing duplicate tail, got %d", len(result.AdjustedFrames))
	}
}

func TestLoopCheckDiscontinuousLoop(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 10, 10, 50, 50, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 30, 30, 70, 70, color.NRGBA{0, 255, 0, 255})
	img3 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img3, 60, 60, 100, 100, color.NRGBA{0, 0, 255, 255})

	box1 := backgroundremoval.SubjectBox{MinX: 10, MinY: 10, MaxX: 49, MaxY: 49, Width: 40, Height: 40, Empty: false}
	box2 := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}
	box3 := backgroundremoval.SubjectBox{MinX: 60, MinY: 60, MaxX: 99, MaxY: 99, Width: 40, Height: 40, Empty: false}

	imgs := []image.Image{img1, img2, img3}
	boxes := []backgroundremoval.SubjectBox{box1, box2, box3}

	c := NewLoopChecker()
	result := c.CheckLoop("walk_left", imgs, boxes)

	if !result.IsLoopAction {
		t.Errorf("expected loop action for walk_left")
	}
	if !result.IsDiscontinuous {
		t.Errorf("expected discontinuous loop")
	}
	if !hasFlag(result.QualityFlags, FlagLoopDiscontinuity) {
		t.Errorf("expected LOOP_DISCONTINUITY flag, got %v", result.QualityFlags)
	}
	if result.HeadTailDiff <= defaultDiscontinuityThreshold {
		t.Errorf("expected high head-tail diff, got %v", result.HeadTailDiff)
	}
}

func TestLoopRemoveDuplicateTail(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 30, 30, 70, 70, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 32, 30, 72, 70, color.NRGBA{0, 255, 0, 255})

	imgs := []image.Image{img1, img2, img1}

	result := removeDuplicateTailFrame(imgs)

	if len(result) != 2 {
		t.Errorf("expected 2 frames after removing duplicate tail, got %d", len(result))
	}
}

func TestLoopRemoveDuplicateTailNoDuplicate(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 30, 30, 70, 70, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 32, 30, 72, 70, color.NRGBA{0, 255, 0, 255})
	img3 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img3, 34, 30, 74, 70, color.NRGBA{0, 0, 255, 255})

	imgs := []image.Image{img1, img2, img3}

	result := removeDuplicateTailFrame(imgs)

	if len(result) != 3 {
		t.Errorf("expected 3 frames when no duplicate tail, got %d", len(result))
	}
}

func TestLoopDuplicateHeadAsTail(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 30, 30, 70, 70, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 32, 30, 72, 70, color.NRGBA{0, 255, 0, 255})

	imgs := []image.Image{img1, img2}

	result := duplicateHeadAsTail(imgs)

	if len(result) != 3 {
		t.Errorf("expected 3 frames after duplicating head as tail, got %d", len(result))
	}

	headHash := computeContentHash(img1)
	tailHash := computeContentHash(result[len(result)-1])
	if headHash != tailHash {
		t.Errorf("expected tail to equal head after duplication")
	}
}

func TestLoopDuplicateHeadAsTailEmpty(t *testing.T) {
	result := duplicateHeadAsTail(nil)
	if result != nil {
		t.Errorf("expected nil for nil input")
	}

	result = duplicateHeadAsTail([]image.Image{})
	if len(result) != 0 {
		t.Errorf("expected empty for empty input")
	}
}

func TestLoopCheckAnchorJump(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 10, 10, 50, 50, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 30, 30, 70, 70, color.NRGBA{0, 255, 0, 255})
	img3 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img3, 50, 50, 90, 90, color.NRGBA{0, 0, 255, 255})

	box1 := backgroundremoval.SubjectBox{MinX: 10, MinY: 10, MaxX: 49, MaxY: 49, Width: 40, Height: 40, Empty: false}
	box2 := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}
	box3 := backgroundremoval.SubjectBox{MinX: 50, MinY: 50, MaxX: 89, MaxY: 89, Width: 40, Height: 40, Empty: false}

	imgs := []image.Image{img1, img2, img3}
	boxes := []backgroundremoval.SubjectBox{box1, box2, box3}

	c := NewLoopChecker()
	result := c.CheckLoop("idle_breathing", imgs, boxes)

	if !result.AnchorJump {
		t.Errorf("expected anchor jump between head and tail")
	}
}

func TestLoopCheckSingleFrame(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 30, 30, 70, 70, color.NRGBA{255, 255, 255, 255})
	box := backgroundremoval.SubjectBox{MinX: 30, MinY: 30, MaxX: 69, MaxY: 69, Width: 40, Height: 40, Empty: false}

	c := NewLoopChecker()
	result := c.CheckLoop("idle_normal", []image.Image{img}, []backgroundremoval.SubjectBox{box})

	if !result.IsLoopAction {
		t.Errorf("expected loop action")
	}
	if result.IsDiscontinuous {
		t.Errorf("should not be discontinuous for single frame")
	}
}

func TestLoopComputeFrameDiffIdentical(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 30, 30, 70, 70, color.NRGBA{255, 255, 255, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 30, 30, 70, 70, color.NRGBA{255, 255, 255, 255})

	diff := computeFrameDiff(img1, img2)
	if diff != 0 {
		t.Errorf("expected 0 diff for identical frames, got %v", diff)
	}
}

func TestLoopComputeFrameDiffDifferent(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 10, 10, 50, 50, color.NRGBA{255, 255, 255, 255})
	img2 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 60, 60, 100, 100, color.NRGBA{255, 255, 255, 255})

	diff := computeFrameDiff(img1, img2)
	if diff != 1.0 {
		t.Errorf("expected diff 1.0 for non-overlapping frames, got %v", diff)
	}
}

func TestLoopComputeFrameDiffSizeMismatch(t *testing.T) {
	img1 := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	img2 := newTestImage(80, 80, color.NRGBA{0, 0, 0, 0})

	diff := computeFrameDiff(img1, img2)
	if diff != 1.0 {
		t.Errorf("expected diff 1.0 for size mismatch, got %v", diff)
	}
}
