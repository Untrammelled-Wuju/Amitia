// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"image"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

var loopActionKeys = map[string]bool{
	"idle_normal":    true,
	"idle_breathing": true,
	"idle_blink":     true,
	"idle_sway":      true,
	"walk_left":      true,
	"walk_right":     true,
	"run_left":       true,
	"run_right":      true,
}

const (
	defaultDiscontinuityThreshold = 0.15
	defaultAnchorJumpThreshold    = 0.1
	frameDiffOpaqueThreshold      = 128
)

func IsLoopAction(actionKey string) bool {
	return loopActionKeys[actionKey]
}

func IsLoopActionByKeyOrMode(actionKey, playbackMode string) bool {
	if loopActionKeys[actionKey] {
		return true
	}
	return playbackMode == "loop" || playbackMode == "ping_pong"
}

type LoopCheckResult struct {
	IsLoopAction    bool
	IsDiscontinuous bool
	QualityFlags    []string
	HeadTailDiff    float64
	AnchorJump      bool
	AdjustedFrames  []image.Image
}

type LoopChecker struct {
	discontinuityThreshold float64
	anchorJumpThreshold    float64
}

func NewLoopChecker() *LoopChecker {
	return &LoopChecker{
		discontinuityThreshold: defaultDiscontinuityThreshold,
		anchorJumpThreshold:    defaultAnchorJumpThreshold,
	}
}

func (c *LoopChecker) CheckLoop(actionKey string, imgs []image.Image, boxes []backgroundremoval.SubjectBox) LoopCheckResult {
	return c.CheckLoopWithMode(actionKey, "", imgs, boxes)
}

func (c *LoopChecker) CheckLoopWithMode(actionKey, playbackMode string, imgs []image.Image, boxes []backgroundremoval.SubjectBox) LoopCheckResult {
	result := LoopCheckResult{}

	if !IsLoopActionByKeyOrMode(actionKey, playbackMode) {
		result.IsLoopAction = false
		return result
	}

	result.IsLoopAction = true

	if len(imgs) < 2 {
		result.AdjustedFrames = imgs
		return result
	}

	headTailDiff := computeFrameDiff(imgs[0], imgs[len(imgs)-1])
	result.HeadTailDiff = headTailDiff

	if headTailDiff > c.discontinuityThreshold {
		result.IsDiscontinuous = true
		result.QualityFlags = append(result.QualityFlags, FlagLoopDiscontinuity)
	}

	if len(boxes) >= 2 {
		headBox := boxes[0]
		tailBox := boxes[len(boxes)-1]
		if !headBox.Empty && !tailBox.Empty {
			headCenterX := (float64(headBox.MinX) + float64(headBox.MaxX)) / 2.0
			headCenterY := (float64(headBox.MinY) + float64(headBox.MaxY)) / 2.0
			tailCenterX := (float64(tailBox.MinX) + float64(tailBox.MaxX)) / 2.0
			tailCenterY := (float64(tailBox.MinY) + float64(tailBox.MaxY)) / 2.0

			imgBounds := imgs[0].Bounds()
			imgW := imgBounds.Dx()
			imgH := imgBounds.Dy()
			if imgW > 0 && imgH > 0 {
				dx := absFloat(headCenterX - tailCenterX) / float64(imgW)
				dy := absFloat(headCenterY - tailCenterY) / float64(imgH)
				if dx > c.anchorJumpThreshold || dy > c.anchorJumpThreshold {
					result.AnchorJump = true
				}
			}
		}
	}

	result.AdjustedFrames = removeDuplicateTailFrame(imgs)

	return result
}

func computeFrameDiff(a, b image.Image) float64 {
	if a == nil || b == nil {
		return 1.0
	}
	aBounds := a.Bounds()
	bBounds := b.Bounds()
	if aBounds.Dx() != bBounds.Dx() || aBounds.Dy() != bBounds.Dy() {
		return 1.0
	}

	w := aBounds.Dx()
	h := aBounds.Dy()
	if w <= 0 || h <= 0 {
		return 0
	}

	intersection := 0
	union := 0
	opaqueThreshold := uint32(frameDiffOpaqueThreshold) << 8

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			_, _, _, aA := a.At(aBounds.Min.X+x, aBounds.Min.Y+y).RGBA()
			_, _, _, bA := b.At(bBounds.Min.X+x, bBounds.Min.Y+y).RGBA()
			aOpaque := aA >= opaqueThreshold
			bOpaque := bA >= opaqueThreshold
			if aOpaque && bOpaque {
				intersection++
			}
			if aOpaque || bOpaque {
				union++
			}
		}
	}

	if union == 0 {
		return 0
	}
	return 1.0 - float64(intersection)/float64(union)
}

func removeDuplicateTailFrame(imgs []image.Image) []image.Image {
	n := len(imgs)
	if n < 2 {
		out := make([]image.Image, len(imgs))
		copy(out, imgs)
		return out
	}
	tailHash := computeContentHash(imgs[n-1])
	headHash := computeContentHash(imgs[0])
	if tailHash == headHash {
		out := make([]image.Image, n-1)
		copy(out, imgs[:n-1])
		return out
	}
	out := make([]image.Image, n)
	copy(out, imgs)
	return out
}

func duplicateHeadAsTail(imgs []image.Image) []image.Image {
	if len(imgs) == 0 {
		return imgs
	}
	result := make([]image.Image, len(imgs)+1)
	copy(result, imgs)
	result[len(imgs)] = imgs[0]
	return result
}
