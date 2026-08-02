// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type FrameData struct {
	FrameIndex    int
	FilePath      string
	PixelHash     string
	Width         int
	Height        int
	AlphaCoverage float64
	SubjectBox    string
	AnchorX       float64
	AnchorY       float64
	ContentHash   string
	Status        string
}

type ActionMetadata struct {
	ActionKey      string
	LoopType       string
	PlaybackMode   string
	AnchorProfile  string
	ActionSpecHash string
	CanvasWidth    int
	CanvasHeight   int
	FrameCount     int
	RevisionHash   string
}

type MeasurementDataProvider interface {
	GetActionMetadata(ctx context.Context, processingActionID string) (*ActionMetadata, error)
	ListFrameData(ctx context.Context, processingActionID string) ([]FrameData, error)
	GetActiveActionRevisionID(ctx context.Context, processingActionID string) (string, error)
}

type ProcessingMeasurementAdapter struct {
	provider MeasurementDataProvider
	dataDir  string
}

func NewProcessingMeasurementAdapter(provider MeasurementDataProvider, dataDir string) *ProcessingMeasurementAdapter {
	return &ProcessingMeasurementAdapter{provider: provider, dataDir: dataDir}
}

func (a *ProcessingMeasurementAdapter) resolvePath(p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(a.dataDir, p)
}

func (a *ProcessingMeasurementAdapter) LoadActionMeasurements(ctx context.Context, processingActionID string) (*ActionMeasurementSet, error) {
	actionRevisionID, err := a.provider.GetActiveActionRevisionID(ctx, processingActionID)
	if err != nil {
		return nil, NewQualityError(ErrCodeRevisionNotFound, "failed to resolve action revision id", err)
	}
	if actionRevisionID == "" {
		return nil, ErrRevisionNotFound
	}

	meta, err := a.provider.GetActionMetadata(ctx, processingActionID)
	if err != nil {
		return nil, NewQualityError(ErrCodeRevisionNotFound, "failed to get action metadata", err)
	}
	if meta == nil {
		return nil, ErrRevisionNotFound
	}

	frames, err := a.provider.ListFrameData(ctx, processingActionID)
	if err != nil {
		return nil, NewQualityError(ErrCodeMeasurementMissing, "failed to list frame data", err)
	}

	measurements := make([]FrameMeasurement, 0, len(frames))
	for _, fd := range frames {
		resolvedPath := a.resolvePath(fd.FilePath)
		fm := FrameMeasurement{
			FrameIndex: fd.FrameIndex,
			FilePath:   resolvedPath,
			PixelHash:  fd.PixelHash,
			FileHash:   fd.ContentHash,
			Width:      fd.Width,
			Height:     fd.Height,
			AnchorX:    fd.AnchorX,
			AnchorY:    fd.AnchorY,
		}

		fm.FileExists = resolvedPath != "" && fileExists(resolvedPath)
		fm.Decodable = fm.FileExists && fd.Width > 0 && fd.Height > 0
		fm.HasAlpha = true

		foreground, semi, opaque, transparent, border := computeCoverageFromAlpha(fd.AlphaCoverage, fd.SubjectBox, meta.CanvasWidth, meta.CanvasHeight)
		fm.ForegroundCoverage = foreground
		fm.SemiTransparentCoverage = semi
		fm.OpaqueCoverage = opaque
		fm.TransparentCoverage = transparent
		fm.BorderForegroundCoverage = border

		boxX, boxY, boxW, boxH := parseSubjectBox(fd.SubjectBox)
		fm.SubjectBoxX = boxX
		fm.SubjectBoxY = boxY
		fm.SubjectBoxWidth = boxW
		fm.SubjectBoxHeight = boxH

		fm.MaskArea = computeMaskArea(fd.AlphaCoverage, meta.CanvasWidth, meta.CanvasHeight)
		fm.CentroidX = boxX + boxW/2
		fm.CentroidY = boxY + boxH/2

		if fd.Width > 0 && fd.Height > 0 {
			fm.FileSize = int64(fd.Width * fd.Height * 4)
		}

		measurements = append(measurements, fm)
	}

	sort.Slice(measurements, func(i, j int) bool {
		return measurements[i].FrameIndex < measurements[j].FrameIndex
	})

	revHash := meta.RevisionHash
	if revHash == "" {
		revHash = computeRevisionHash(measurements)
	}

	return &ActionMeasurementSet{
		ActionRevisionID:  actionRevisionID,
		ActionKey:         meta.ActionKey,
		CanvasWidth:       meta.CanvasWidth,
		CanvasHeight:      meta.CanvasHeight,
		FrameCount:        meta.FrameCount,
		FrameMeasurements: measurements,
		LoopType:          meta.LoopType,
		PlaybackMode:      meta.PlaybackMode,
		AnchorProfile:     meta.AnchorProfile,
		ActionSpecHash:    meta.ActionSpecHash,
		RevisionHash:      revHash,
	}, nil
}

func (a *ProcessingMeasurementAdapter) OpenFrame(ctx context.Context, actionRevisionID string, frameIndex int) (image.Image, error) {
	frames, err := a.provider.ListFrameData(ctx, actionRevisionID)
	if err != nil {
		return nil, err
	}
	for _, f := range frames {
		if f.FrameIndex == frameIndex {
			resolvedPath := a.resolvePath(f.FilePath)
			if resolvedPath == "" || !fileExists(resolvedPath) {
				return nil, fmt.Errorf("frame file not found: index %d", frameIndex)
			}
			file, err := os.Open(resolvedPath)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			img, _, err := image.Decode(file)
			return img, err
		}
	}
	return nil, fmt.Errorf("frame not found: index %d", frameIndex)
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func parseSubjectBox(boxJSON string) (x, y, w, h float64) {
	if boxJSON == "" {
		return 0, 0, 0, 0
	}
	var box struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
		MinX   float64 `json:"minX"`
		MinY   float64 `json:"minY"`
		MaxX   float64 `json:"maxX"`
		MaxY   float64 `json:"maxY"`
	}
	if err := json.Unmarshal([]byte(boxJSON), &box); err != nil {
		return 0, 0, 0, 0
	}
	if box.Width > 0 || box.Height > 0 {
		return box.X, box.Y, box.Width, box.Height
	}
	if box.MaxX > box.MinX || box.MaxY > box.MinY {
		return box.MinX, box.MinY, box.MaxX - box.MinX, box.MaxY - box.MinY
	}
	return 0, 0, 0, 0
}

func computeCoverageFromAlpha(alphaCoverage float64, subjectBox string, canvasW, canvasH int) (foreground, semi, opaque, transparent, border float64) {
	foreground = alphaCoverage
	transparent = 1.0 - alphaCoverage

	if alphaCoverage > 0.95 {
		opaque = alphaCoverage
		semi = 0
	} else if alphaCoverage > 0.5 {
		opaque = alphaCoverage * 0.7
		semi = alphaCoverage * 0.3
	} else {
		opaque = 0
		semi = alphaCoverage * 0.1
	}

	boxX, boxY, boxW, boxH := parseSubjectBox(subjectBox)
	if canvasW > 0 && canvasH > 0 {
		marginW := float64(canvasW) * 0.02
		marginH := float64(canvasH) * 0.02
		borderRatio := 0.0
		if boxW > 0 && boxH > 0 {
			leftOverlap := math.Max(0, marginW-boxX)
			rightOverlap := math.Max(0, boxX+boxW-(float64(canvasW)-marginW))
			topOverlap := math.Max(0, marginH-boxY)
			bottomOverlap := math.Max(0, boxY+boxH-(float64(canvasH)-marginH))
			borderArea := (leftOverlap+rightOverlap)*boxH + (topOverlap+bottomOverlap)*boxW
			boxArea := boxW * boxH
			if boxArea > 0 {
				borderRatio = borderArea / boxArea
			}
		}
		border = borderRatio * foreground
	}
	return
}

func computeMaskArea(alphaCoverage float64, canvasW, canvasH int) float64 {
	if canvasW <= 0 || canvasH <= 0 {
		return alphaCoverage
	}
	return alphaCoverage * float64(canvasW*canvasH)
}

func computeRevisionHash(measurements []FrameMeasurement) string {
	h := sha256.New()
	for _, m := range measurements {
		fmt.Fprintf(h, "%d:%s:%s:%d:%d", m.FrameIndex, m.PixelHash, m.FileHash, m.Width, m.Height)
	}
	return hex.EncodeToString(h.Sum(nil))
}
