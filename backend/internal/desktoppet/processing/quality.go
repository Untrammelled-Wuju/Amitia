// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

const (
	FlagSubjectTooSmall    = "SUBJECT_TOO_SMALL"
	FlagSubjectTooLarge    = "SUBJECT_TOO_LARGE"
	FlagSubjectTouchesEdge = "SUBJECT_TOUCHES_EDGE"
	FlagBackgroundResidue  = "BACKGROUND_RESIDUE"
	FlagEmptyFrame         = "EMPTY_FRAME"
	FlagDuplicateFrame     = "DUPLICATE_FRAME"
	FlagAlphaInvalid       = "ALPHA_INVALID"
	FlagSourceMissing      = "SOURCE_MISSING"
	FlagLoopDiscontinuity  = "LOOP_DISCONTINUITY"
)

const (
	QualityLevelNormal  = "normal"
	QualityLevelWarning = "warning"
	QualityLevelFailed  = "failed"
)

const (
	defaultMinSubjectRatio             = 0.05
	defaultMaxSubjectRatio             = 0.95
	defaultDriftThreshold              = 0.05
	defaultScaleDriftThreshold         = 0.1
	emptyFrameCoverageThreshold        = 0.01
	backgroundResidueCoverageThreshold = 0.9
)

type FrameQualityResult struct {
	FrameIndex    int
	QualityFlags  []string
	QualityLevel  string
	SubjectBox    backgroundremoval.SubjectBox
	AlphaCoverage float64
	ContentHash   string
	Error         error
}

type QualityChecker struct {
	minSubjectRatio        float64
	maxSubjectRatio        float64
	alphaThreshold         uint8
	duplicateHashThreshold int
	driftThreshold         float64
	scaleDriftThreshold    float64
}

func NewQualityChecker() *QualityChecker {
	return &QualityChecker{
		minSubjectRatio:        defaultMinSubjectRatio,
		maxSubjectRatio:        defaultMaxSubjectRatio,
		alphaThreshold:         defaultAlphaThreshold,
		duplicateHashThreshold: 0,
		driftThreshold:         defaultDriftThreshold,
		scaleDriftThreshold:    defaultScaleDriftThreshold,
	}
}

func (c *QualityChecker) CheckFrame(img image.Image, box backgroundremoval.SubjectBox, prevHash string) FrameQualityResult {
	result := FrameQualityResult{
		FrameIndex: -1,
		SubjectBox: box,
	}

	if img == nil {
		result.QualityFlags = append(result.QualityFlags, FlagSourceMissing)
		result.QualityLevel = QualityLevelFailed
		result.Error = NewProcessingError(ErrCodeFrameQualityCheckFailed, "image is nil")
		return result
	}

	bounds := img.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()

	if imgW <= 0 || imgH <= 0 {
		result.QualityFlags = append(result.QualityFlags, FlagSourceMissing)
		result.QualityLevel = QualityLevelFailed
		result.Error = NewProcessingError(ErrCodeFrameQualityCheckFailed, "image has invalid dimensions")
		return result
	}

	alphaCov, allOpaque := analyzeAlpha(img, c.alphaThreshold)
	result.AlphaCoverage = alphaCov

	hash := computeContentHash(img)
	result.ContentHash = hash

	hasAlphaType := hasAlphaChannel(img)
	if !hasAlphaType || allOpaque {
		result.QualityFlags = append(result.QualityFlags, FlagAlphaInvalid)
	}

	if alphaCov < emptyFrameCoverageThreshold {
		result.QualityFlags = append(result.QualityFlags, FlagEmptyFrame)
	}

	if box.Empty {
		result.QualityFlags = append(result.QualityFlags, FlagEmptyFrame)
		result.QualityLevel = DetermineQualityLevel(result.QualityFlags)
		return result
	}

	canvasArea := float64(imgW * imgH)
	if canvasArea > 0 {
		ratio := float64(box.Width*box.Height) / canvasArea
		if ratio < c.minSubjectRatio {
			result.QualityFlags = append(result.QualityFlags, FlagSubjectTooSmall)
		}
		if ratio > c.maxSubjectRatio {
			result.QualityFlags = append(result.QualityFlags, FlagSubjectTooLarge)
		}
	}

	if box.MinX == 0 || box.MinY == 0 || box.MaxX == imgW-1 || box.MaxY == imgH-1 {
		result.QualityFlags = append(result.QualityFlags, FlagSubjectTouchesEdge)
	}

	if hasAlphaType && alphaCov < backgroundResidueCoverageThreshold {
		result.QualityFlags = append(result.QualityFlags, FlagBackgroundResidue)
	}

	if isDuplicate(hash, prevHash) {
		result.QualityFlags = append(result.QualityFlags, FlagDuplicateFrame)
	}

	result.QualityLevel = DetermineQualityLevel(result.QualityFlags)
	return result
}

func (c *QualityChecker) CheckFrames(imgs []image.Image, boxes []backgroundremoval.SubjectBox) []FrameQualityResult {
	results := make([]FrameQualityResult, len(imgs))
	var prevHash string
	var prevBox backgroundremoval.SubjectBox
	prevBoxValid := false

	for i, img := range imgs {
		var box backgroundremoval.SubjectBox
		if i < len(boxes) {
			box = boxes[i]
		} else {
			box = backgroundremoval.SubjectBox{Empty: true}
		}

		result := c.CheckFrame(img, box, prevHash)
		result.FrameIndex = i

		if !box.Empty && prevBoxValid && !prevBox.Empty {
			if prevBox.Height > 0 {
				scaleChange := absFloat(float64(box.Height)-float64(prevBox.Height)) / float64(prevBox.Height)
				if scaleChange > c.scaleDriftThreshold {
					result.QualityFlags = append(result.QualityFlags, FlagScaleDrift)
				}
			}

			curCenterX := (float64(box.MinX) + float64(box.MaxX)) / 2.0
			curCenterY := (float64(box.MinY) + float64(box.MaxY)) / 2.0
			prevCenterX := (float64(prevBox.MinX) + float64(prevBox.MaxX)) / 2.0
			prevCenterY := (float64(prevBox.MinY) + float64(prevBox.MaxY)) / 2.0

			bounds := img.Bounds()
			imgW := bounds.Dx()
			imgH := bounds.Dy()
			if imgW > 0 && imgH > 0 {
				dx := absFloat(curCenterX-prevCenterX) / float64(imgW)
				dy := absFloat(curCenterY-prevCenterY) / float64(imgH)
				if dx > c.driftThreshold || dy > c.driftThreshold {
					result.QualityFlags = append(result.QualityFlags, FlagAnchorDrift)
				}
			}
		}

		result.QualityLevel = DetermineQualityLevel(result.QualityFlags)

		results[i] = result
		prevHash = result.ContentHash
		prevBox = box
		prevBoxValid = true
	}

	return results
}

func DetermineQualityLevel(flags []string) string {
	if IsAutoFail(flags) {
		return QualityLevelFailed
	}
	if len(flags) == 0 {
		return QualityLevelNormal
	}
	return QualityLevelWarning
}

func IsAutoFail(flags []string) bool {
	for _, f := range flags {
		switch f {
		case FlagSourceMissing, FlagSubjectTooLarge, FlagEmptyFrame, FlagAlphaInvalid:
			return true
		}
	}
	return false
}

func hasAlphaChannel(img image.Image) bool {
	switch img.(type) {
	case *image.NRGBA, *image.RGBA, *image.NRGBA64, *image.RGBA64:
		return true
	}
	return false
}

func analyzeAlpha(img image.Image, threshold uint8) (coverage float64, allOpaque bool) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return 0, true
	}

	total := w * h
	nonTransparent := 0
	allOpaque = true

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			alpha8 := uint8(a >> 8)
			if alpha8 >= threshold {
				nonTransparent++
			}
			if a != 0xffff {
				allOpaque = false
			}
		}
	}

	coverage = float64(nonTransparent) / float64(total)
	return coverage, allOpaque
}

func computeAlphaCoverage(img image.Image, threshold uint8) float64 {
	cov, _ := analyzeAlpha(img, threshold)
	return cov
}

func computeContentHash(img image.Image) string {
	if img == nil {
		return ""
	}
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	h256 := sha256.New()

	var dim [8]byte
	binary.LittleEndian.PutUint32(dim[:4], uint32(w))
	binary.LittleEndian.PutUint32(dim[4:], uint32(h))
	h256.Write(dim[:])

	rowBuf := make([]byte, w*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		idx := 0
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			rowBuf[idx] = uint8(r >> 8)
			rowBuf[idx+1] = uint8(g >> 8)
			rowBuf[idx+2] = uint8(b >> 8)
			rowBuf[idx+3] = uint8(a >> 8)
			idx += 4
		}
		h256.Write(rowBuf)
	}

	sum := h256.Sum(nil)
	return hex.EncodeToString(sum)
}

func isDuplicate(currentHash, prevHash string) bool {
	return currentHash != "" && prevHash != "" && currentHash == prevHash
}
