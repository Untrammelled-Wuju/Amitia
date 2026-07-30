package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	"math"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/processing/artifact"
	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
	"github.com/u-ai/backend/internal/desktoppet/processing/decode"
	"github.com/u-ai/backend/internal/desktoppet/processing/encoding"
	"github.com/u-ai/backend/internal/desktoppet/processing/geometry"
	"github.com/u-ai/backend/internal/desktoppet/processing/measurement"
	"github.com/u-ai/backend/internal/desktoppet/processing/source"
	"github.com/u-ai/backend/internal/desktoppet/processing/split"
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

type Pipeline struct {
	bgRegistry    backgroundremoval.Registry
	dataDir       string
	artifactStore *artifact.ArtifactStore
	publisher     *artifact.RevisionPublisher
	encoder       *encoding.PNGEncoder
	splitter      *split.Splitter
}

func NewPipeline(bgRegistry backgroundremoval.Registry, dataDir string) *Pipeline {
	return &Pipeline{
		bgRegistry:    bgRegistry,
		dataDir:       dataDir,
		artifactStore: artifact.NewArtifactStore(dataDir),
		publisher:     artifact.NewRevisionPublisher(dataDir),
		encoder:       encoding.NewPNGEncoder(6),
		splitter:      split.NewSplitter(),
	}
}

type ProcessActionRequest struct {
	Context            context.Context
	SourceDescriptor   *source.ProcessingSourceDescriptor
	ConfigSnapshot     *contracts.ProcessingConfigSnapshot
	ProcessingTaskID   string
	ProcessingActionID string
	ActionKey          string
	GenerationTaskID   string
	ExecutionID        string
	ProcessingVersion  int
}

type ProcessActionResult struct {
	RevisionID          string
	FrameCount          int
	Manifest            *contracts.RevisionManifest
	Measurements        []measurement.FrameMeasurementData
	SequenceMeasurement *measurement.SequenceMeasurement
	RootRelativePath    string
}

func (p *Pipeline) ProcessAction(req ProcessActionRequest) (*ProcessActionResult, error) {
	if req.SourceDescriptor == nil {
		return nil, fmt.Errorf("pipeline: sourceDescriptor is nil")
	}
	if req.ConfigSnapshot == nil {
		return nil, fmt.Errorf("pipeline: configSnapshot is nil")
	}

	startTime := time.Now()

	revisionID := "rev-" + uuid.New().String()

	workDir, err := artifact.NewWorkDirectory(p.dataDir, req.ProcessingTaskID, req.ExecutionID, req.ActionKey, revisionID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: create work directory: %w", err)
	}

	journal := artifact.NewJournal(revisionID, workDir.JournalPath)

	if err := journal.Record("preparing", "done", "work directory created"); err != nil {
		return nil, fmt.Errorf("pipeline: record preparing: %w", err)
	}

	if err := workDir.Create(); err != nil {
		_ = journal.Record("preparing", "failed", err.Error())
		return nil, fmt.Errorf("pipeline: create work dir: %w", err)
	}

	srcFrames := req.SourceDescriptor.Frames
	if len(srcFrames) == 0 && req.SourceDescriptor.SourceKind == source.SourceSpriteSheet {
		srcFrames = []source.ProcessingSourceFrame{{
			RelativePath:   req.SourceDescriptor.Artifact.RelativePath,
			ExpectedHash:   req.SourceDescriptor.Artifact.ContentHash,
			ExpectedMIME:   req.SourceDescriptor.Artifact.MIMEType,
			ExpectedWidth:  req.SourceDescriptor.Artifact.Width,
			ExpectedHeight: req.SourceDescriptor.Artifact.Height,
		}}
	}
	if len(srcFrames) == 0 {
		_ = journal.Record("decode", "failed", "no source frames")
		return nil, fmt.Errorf("pipeline: decode stage: no source frames")
	}

	decodedImages := make([]*decode.DecodedImage, 0, len(srcFrames))
	for _, frame := range srcFrames {
		absPath := filepath.Join(p.dataDir, frame.RelativePath)
		decodeReq := decode.DecodeRequest{
			AbsolutePath: absPath,
			MaxBytes:     req.ConfigSnapshot.Decode.MaxInputBytes,
			MaxPixels:    req.ConfigSnapshot.Decode.MaxPixels,
			MaxDimension: req.ConfigSnapshot.Decode.MaxDimension,
			AllowedMIMEs: req.ConfigSnapshot.Decode.AllowedMIMEs,
		}
		if frame.ExpectedHash != "" {
			decodeReq.ExpectedHash = frame.ExpectedHash
		}
		if frame.ExpectedMIME != "" {
			decodeReq.ExpectedMIME = frame.ExpectedMIME
		}
		decoded, err := decode.Decode(decodeReq)
		if err != nil {
			_ = journal.Record("decode", "failed", err.Error())
			return nil, fmt.Errorf("pipeline: decode stage: %w", err)
		}
		decodedImages = append(decodedImages, decoded)
	}

	if err := journal.Record("decode", "done", fmt.Sprintf("decoded %d frames", len(decodedImages))); err != nil {
		return nil, fmt.Errorf("pipeline: record decode: %w", err)
	}

	var frameImages []*image.NRGBA
	if req.SourceDescriptor.SourceKind == source.SourceSpriteSheet {
		layout := req.SourceDescriptor.Artifact.Layout
		if layout == nil {
			_ = journal.Record("split", "failed", "layout is nil")
			return nil, fmt.Errorf("pipeline: split stage: layout is nil")
		}

		logicalFrameCount := len(req.SourceDescriptor.Artifact.LogicalFrames)
		if logicalFrameCount == 0 {
			logicalFrameCount = layout.Rows*layout.Columns - len(layout.EmptyCellIndexes)
		}
		if logicalFrameCount <= 0 {
			_ = journal.Record("split", "failed", "invalid logical frame count")
			return nil, fmt.Errorf("pipeline: split stage: invalid logical frame count")
		}

		sheetImage := decodedImages[0].Image
		splitResult, err := p.splitter.Split(sheetImage, layout, logicalFrameCount)
		if err != nil {
			_ = journal.Record("split", "failed", err.Error())
			return nil, fmt.Errorf("pipeline: split stage: %w", err)
		}

		frameImageMap := make(map[int]*image.NRGBA)
		for _, cell := range splitResult.Cells {
			if !cell.Empty && cell.Image != nil {
				frameImageMap[cell.FrameIndex] = cell.Image
			}
		}

		frameImages = make([]*image.NRGBA, logicalFrameCount)
		for i := 0; i < logicalFrameCount; i++ {
			img, ok := frameImageMap[i]
			if !ok {
				_ = journal.Record("split", "failed", fmt.Sprintf("missing frame %d", i))
				return nil, fmt.Errorf("pipeline: split stage: missing frame %d", i)
			}
			frameImages[i] = img
		}

		if err := journal.Record("split", "done", fmt.Sprintf("split into %d frames", len(frameImages))); err != nil {
			return nil, fmt.Errorf("pipeline: record split: %w", err)
		}
	} else {
		frameImages = make([]*image.NRGBA, len(decodedImages))
		for i, decoded := range decodedImages {
			frameImages[i] = decoded.Image
		}
	}

	frameCount := len(frameImages)

	bgPolicy := backgroundremoval.BackgroundPolicyConfig{
		ProviderName:   req.ConfigSnapshot.Background.ProviderName,
		Mode:           backgroundremoval.BackgroundMode(req.ConfigSnapshot.Background.Mode),
		FallbackPolicy: req.ConfigSnapshot.Background.FallbackPolicy,
		Timeout:        req.ConfigSnapshot.Background.Timeout,
		MaxRetries:     req.ConfigSnapshot.Background.MaxRetries,
	}

	foregrounds := make([]*image.NRGBA, frameCount)
	masks := make([]*image.Gray, frameCount)
	providerUsed := ""

	for i, frameImg := range frameImages {
		inputDesc := backgroundremoval.ImageDescriptor{
			Width:  frameImg.Bounds().Dx(),
			Height: frameImg.Bounds().Dy(),
			MIME:   "image/png",
			Pixels: int64(frameImg.Bounds().Dx()) * int64(frameImg.Bounds().Dy()),
		}

		resolved, err := p.bgRegistry.Resolve(bgPolicy, inputDesc)
		if err != nil {
			_ = journal.Record("background_removal", "failed", err.Error())
			return nil, fmt.Errorf("pipeline: background removal stage frame %d: %w", i, err)
		}

		if resolved.Provider == nil {
			foregrounds[i] = frameImg
			masks[i] = maskFromAlpha(frameImg)
			if providerUsed == "" {
				providerUsed = "keep_original"
			}
			continue
		}

		if providerUsed == "" {
			providerUsed = resolved.Provider.Name()
		}

		if v2, ok := resolved.Provider.(backgroundremoval.BackgroundRemovalProviderV2); ok {
			bgReq := backgroundremoval.BackgroundRemovalRequest{
				RequestID: uuid.New().String(),
				Image:     frameImg,
				Mode:      backgroundremoval.BackgroundMode(req.ConfigSnapshot.Background.Mode),
				Timeout:   req.ConfigSnapshot.Background.Timeout,
			}
			result, err := v2.RemoveBackgroundV2(req.Context, bgReq)
			if err != nil {
				_ = journal.Record("background_removal", "failed", err.Error())
				return nil, fmt.Errorf("pipeline: background removal v2 frame %d: %w", i, err)
			}
			foregrounds[i] = ensureForeground(result, frameImg)
			masks[i] = ensureMask(result, foregrounds[i])
		} else {
			bgInput := backgroundremoval.ImageInput{
				Image:  frameImg,
				Width:  frameImg.Bounds().Dx(),
				Height: frameImg.Bounds().Dy(),
				Mode:   backgroundremoval.BackgroundMode(req.ConfigSnapshot.Background.Mode),
			}
			result, err := resolved.Provider.RemoveBackground(req.Context, bgInput)
			if err != nil {
				_ = journal.Record("background_removal", "failed", err.Error())
				return nil, fmt.Errorf("pipeline: background removal frame %d: %w", i, err)
			}
			foregrounds[i] = ensureForeground(result, frameImg)
			masks[i] = ensureMask(result, foregrounds[i])
		}
	}

	if err := journal.Record("background_removal", "done", fmt.Sprintf("processed %d frames", frameCount)); err != nil {
		return nil, fmt.Errorf("pipeline: record background removal: %w", err)
	}

	analyses := make([]*geometry.SubjectAnalysis, frameCount)
	for i, mask := range masks {
		analysis, err := geometry.AnalyzeMask(
			mask,
			uint8(req.ConfigSnapshot.Subject.AlphaThreshold),
			geometry.SpaceForeground,
			req.ConfigSnapshot.Subject.MinComponentArea,
			req.ConfigSnapshot.Subject.MaxComponents,
		)
		if err != nil {
			_ = journal.Record("subject_analysis", "failed", err.Error())
			return nil, fmt.Errorf("pipeline: subject analysis frame %d: %w", i, err)
		}
		analyses[i] = analysis
	}

	if err := journal.Record("subject_analysis", "done", fmt.Sprintf("analyzed %d frames", frameCount)); err != nil {
		return nil, fmt.Errorf("pipeline: record subject analysis: %w", err)
	}

	anchorResults := make([]*geometry.SourceAnchorResult, frameCount)
	for i := 0; i < frameCount; i++ {
		anchorResult, err := geometry.EstimateSourceAnchor(
			analyses[i],
			masks[i],
			geometry.AnchorMode(req.ConfigSnapshot.Anchor.Mode),
			geometry.SpaceForeground,
		)
		if err != nil {
			_ = journal.Record("anchor_estimation", "failed", err.Error())
			return nil, fmt.Errorf("pipeline: anchor estimation frame %d: %w", i, err)
		}
		anchorResults[i] = anchorResult
	}

	if err := journal.Record("anchor_estimation", "done", fmt.Sprintf("estimated %d anchors", frameCount)); err != nil {
		return nil, fmt.Errorf("pipeline: record anchor estimation: %w", err)
	}

	subjectHeights := make([]float64, frameCount)
	subjectWidths := make([]float64, frameCount)
	for i, analysis := range analyses {
		subjectHeights[i] = float64(analysis.SubjectBox.Height())
		subjectWidths[i] = float64(analysis.SubjectBox.Width())
	}

	scaleResult, err := geometry.ComputeCharacterScaleBaseline(
		subjectHeights,
		subjectWidths,
		req.ConfigSnapshot.Canvas.OutputWidth,
		req.ConfigSnapshot.Canvas.OutputHeight,
		req.ConfigSnapshot.Canvas.TargetCharacterHeightRatio,
		req.ConfigSnapshot.Canvas.MaxCharacterWidthRatio,
		req.ConfigSnapshot.Canvas.SafeMarginTop,
		req.ConfigSnapshot.Canvas.SafeMarginRight,
		req.ConfigSnapshot.Canvas.SafeMarginBottom,
		req.ConfigSnapshot.Canvas.SafeMarginLeft,
	)
	if err != nil {
		_ = journal.Record("scale_computation", "failed", err.Error())
		return nil, fmt.Errorf("pipeline: scale computation: %w", err)
	}

	if err := journal.Record("scale_computation", "done", fmt.Sprintf("scale=%.4f", scaleResult.ClampedScale)); err != nil {
		return nil, fmt.Errorf("pipeline: record scale computation: %w", err)
	}

	targetAnchor := geometry.NormalizedPoint{
		X: req.ConfigSnapshot.Anchor.TargetX,
		Y: req.ConfigSnapshot.Anchor.TargetY,
	}

	resampler := geometry.NewResampler(geometry.ResampleMode(req.ConfigSnapshot.Scale.Resampler))

	mappings := make([]*geometry.CanvasMappingResult, frameCount)
	scaledForegrounds := make([]*image.NRGBA, frameCount)
	baseDrawXs := make([]int, frameCount)
	baseDrawYs := make([]int, frameCount)

	for i := 0; i < frameCount; i++ {
		mapping, err := geometry.MapToCanvas(
			anchorResults[i].Point,
			scaleResult.ClampedScale,
			targetAnchor,
			req.ConfigSnapshot.Canvas.OutputWidth,
			req.ConfigSnapshot.Canvas.OutputHeight,
			analyses[i].SubjectBox,
		)
		if err != nil {
			_ = journal.Record("canvas_mapping", "failed", err.Error())
			return nil, fmt.Errorf("pipeline: canvas mapping frame %d: %w", i, err)
		}
		mappings[i] = mapping

		scaledWidth := int(math.Round(float64(foregrounds[i].Bounds().Dx()) * scaleResult.ClampedScale))
		scaledHeight := int(math.Round(float64(foregrounds[i].Bounds().Dy()) * scaleResult.ClampedScale))
		if scaledWidth <= 0 {
			scaledWidth = 1
		}
		if scaledHeight <= 0 {
			scaledHeight = 1
		}

		scaledForeground, err := resampler.ResizeRGBA(foregrounds[i], scaledWidth, scaledHeight)
		if err != nil {
			_ = journal.Record("canvas_mapping", "failed", err.Error())
			return nil, fmt.Errorf("pipeline: resize frame %d: %w", i, err)
		}
		scaledForegrounds[i] = scaledForeground

		baseDrawXs[i] = int(math.Round(mapping.Transform.M[0][2]))
		baseDrawYs[i] = int(math.Round(mapping.Transform.M[1][2]))
	}

	if err := journal.Record("canvas_mapping", "done", fmt.Sprintf("mapped %d frames", frameCount)); err != nil {
		return nil, fmt.Errorf("pipeline: record canvas mapping: %w", err)
	}

	anchorPoints := make([]geometry.PixelPoint, frameCount)
	confidences := make([]float64, frameCount)
	for i := 0; i < frameCount; i++ {
		anchorPoints[i] = anchorResults[i].Point
		confidences[i] = anchorResults[i].Confidence
	}

	allowance := geometry.MotionAllowance{
		AllowTranslationX: !req.ConfigSnapshot.Alignment.AllowMotionX,
		AllowTranslationY: !req.ConfigSnapshot.Alignment.AllowMotionY,
		MaxStabilizationX: req.ConfigSnapshot.Alignment.MaxCorrectionX * float64(req.ConfigSnapshot.Canvas.OutputWidth),
		MaxStabilizationY: req.ConfigSnapshot.Alignment.MaxCorrectionY * float64(req.ConfigSnapshot.Canvas.OutputHeight),
		ReferenceStrategy: req.ConfigSnapshot.Alignment.ReferenceStrategy,
	}

	alignments, err := geometry.StabilizeSequence(
		anchorPoints,
		confidences,
		allowance,
		req.ConfigSnapshot.Canvas.OutputWidth,
		req.ConfigSnapshot.Canvas.OutputHeight,
	)
	if err != nil {
		_ = journal.Record("stabilization", "failed", err.Error())
		return nil, fmt.Errorf("pipeline: stabilization: %w", err)
	}

	canvases := make([]*image.NRGBA, frameCount)
	for i := 0; i < frameCount; i++ {
		canvasCorrectionX := alignments[i].CorrectionX * scaleResult.ClampedScale
		canvasCorrectionY := alignments[i].CorrectionY * scaleResult.ClampedScale
		drawX := baseDrawXs[i] + int(math.Round(canvasCorrectionX))
		drawY := baseDrawYs[i] + int(math.Round(canvasCorrectionY))

		canvas := image.NewNRGBA(image.Rect(0, 0, req.ConfigSnapshot.Canvas.OutputWidth, req.ConfigSnapshot.Canvas.OutputHeight))

		scaled := scaledForegrounds[i]
		r := image.Rect(drawX, drawY, drawX+scaled.Bounds().Dx(), drawY+scaled.Bounds().Dy())
		clip := r.Intersect(canvas.Bounds())
		if !clip.Empty() {
			sp := image.Point{X: clip.Min.X - r.Min.X, Y: clip.Min.Y - r.Min.Y}
			draw.Draw(canvas, clip, scaled, sp, draw.Over)
		}
		canvases[i] = canvas
	}

	if err := journal.Record("stabilization", "done", fmt.Sprintf("stabilized %d frames", frameCount)); err != nil {
		return nil, fmt.Errorf("pipeline: record stabilization: %w", err)
	}

	frameResults := make([]encoding.FileResult, frameCount)
	maskResults := make([]encoding.FileResult, frameCount)
	for i := 0; i < frameCount; i++ {
		frameName := fmt.Sprintf("frame_%04d.png", i)
		framePath := filepath.Join(workDir.FramesDir, frameName)
		result, err := p.encoder.Encode(canvases[i], framePath)
		if err != nil {
			_ = journal.Record("encoding", "failed", err.Error())
			return nil, fmt.Errorf("pipeline: encode frame %d: %w", i, err)
		}
		frameResults[i] = result

		if req.ConfigSnapshot.Encoding.WriteMask {
			maskName := fmt.Sprintf("mask_%04d.png", i)
			maskPath := filepath.Join(workDir.MasksDir, maskName)
			maskResult, err := p.encoder.EncodeMask(masks[i], maskPath)
			if err != nil {
				_ = journal.Record("encoding", "failed", err.Error())
				return nil, fmt.Errorf("pipeline: encode mask %d: %w", i, err)
			}
			maskResults[i] = maskResult
		}
	}

	if err := journal.Record("encoding", "done", fmt.Sprintf("encoded %d frames", frameCount)); err != nil {
		return nil, fmt.Errorf("pipeline: record encoding: %w", err)
	}

	measurements := make([]measurement.FrameMeasurementData, frameCount)
	for i := 0; i < frameCount; i++ {
		m := measurement.FrameMeasurementData{
			FrameIndex: i,
			SubjectBox: measurement.SubjectBoxData{
				MinX:  analyses[i].SubjectBox.MinX,
				MinY:  analyses[i].SubjectBox.MinY,
				MaxX:  analyses[i].SubjectBox.MaxX,
				MaxY:  analyses[i].SubjectBox.MaxY,
				Space: string(analyses[i].SubjectBox.Space),
			},
			SourceAnchor: measurement.AnchorData{
				X:          anchorResults[i].Point.X,
				Y:          anchorResults[i].Point.Y,
				Space:      string(anchorResults[i].Point.Space),
				Mode:       string(anchorResults[i].Mode),
				Confidence: anchorResults[i].Confidence,
				Estimated:  anchorResults[i].Estimated,
			},
			TargetAnchor: measurement.AnchorData{
				X:     req.ConfigSnapshot.Anchor.TargetX,
				Y:     req.ConfigSnapshot.Anchor.TargetY,
				Space: string(geometry.SpaceCanvas),
				Mode:  req.ConfigSnapshot.Anchor.Mode,
			},
			AlphaCoverage:  analyses[i].AlphaCoverage,
			ComponentCount: len(analyses[i].Components),
			EdgeContact: measurement.EdgeContactData{
				Top:    analyses[i].EdgeContact.Top,
				Bottom: analyses[i].EdgeContact.Bottom,
				Left:   analyses[i].EdgeContact.Left,
				Right:  analyses[i].EdgeContact.Right,
				Count:  analyses[i].EdgeContact.Count,
			},
			Clipping: measurement.ClippingData{
				TotalPixels: mappings[i].ClippedPixels,
				Sides:       mappings[i].ClippedSides,
			},
			Trajectory: measurement.TrajectoryData{
				OriginalX:   alignments[i].OriginalAnchor.X,
				OriginalY:   alignments[i].OriginalAnchor.Y,
				CorrectedX:  alignments[i].CorrectedAnchor.X,
				CorrectedY:  alignments[i].CorrectedAnchor.Y,
				CorrectionX: alignments[i].CorrectionX,
				CorrectionY: alignments[i].CorrectionY,
				Clamped:     alignments[i].Clamped,
			},
		}
		measurements[i] = m
	}

	totalDurationMs := time.Since(startTime).Milliseconds()

	degradedReason := ""
	if providerUsed == "keep_original" {
		degradedReason = "fallback policy keep_original"
	}

	seqMeasurement := &measurement.SequenceMeasurement{
		ActionKey:         req.ActionKey,
		FrameCount:        frameCount,
		FrameMeasurements: measurements,
		ScaleResult: measurement.ScaleResultData{
			Scale:           scaleResult.Scale,
			BaseScale:       scaleResult.BaseScale,
			ClampedScale:    scaleResult.ClampedScale,
			ClampReason:     scaleResult.ClampReason,
			ReferenceHeight: scaleResult.ReferenceHeight,
			ReferenceWidth:  scaleResult.ReferenceWidth,
		},
		ReferenceFrame:    -1,
		ReferenceStrategy: req.ConfigSnapshot.Alignment.ReferenceStrategy,
		ProcessingReport: measurement.ProcessingReport{
			PipelineVersion: contracts.PipelineVersion,
			ConfigHash:      req.ConfigSnapshot.ConfigHash,
			TotalDurationMs: totalDurationMs,
			ProviderUsed:    providerUsed,
			Degraded:        providerUsed == "keep_original",
			DegradedReason:  degradedReason,
		},
	}

	if err := journal.Record("measurement", "done", fmt.Sprintf("generated %d measurements", frameCount)); err != nil {
		return nil, fmt.Errorf("pipeline: record measurement: %w", err)
	}

	manifestFrames := make([]contracts.ManifestFrame, frameCount)
	for i := 0; i < frameCount; i++ {
		frameRelPath := filepath.ToSlash(filepath.Join("frames", fmt.Sprintf("frame_%04d.png", i)))
		mf := contracts.ManifestFrame{
			Index:     i,
			File:      frameRelPath,
			FileHash:  frameResults[i].FileHash,
			PixelHash: frameResults[i].PixelHash,
		}
		if req.ConfigSnapshot.Encoding.WriteMask {
			mf.Mask = filepath.ToSlash(filepath.Join("masks", fmt.Sprintf("mask_%04d.png", i)))
		}
		mJSON, _ := measurements[i].ToJSON()
		mf.Measurement = mJSON
		manifestFrames[i] = mf
	}

	manifest := &contracts.RevisionManifest{
		SchemaVersion:      contracts.RevisionManifestSchemaVersion,
		RevisionID:         revisionID,
		ProcessingTaskID:   req.ProcessingTaskID,
		ProcessingActionID: req.ProcessingActionID,
		ActionKey:          req.ActionKey,
		Source: contracts.ManifestSource{
			AttemptID:   req.SourceDescriptor.Artifact.AttemptID,
			Candidate:   req.SourceDescriptor.CandidateIndex,
			ArtifactID:  req.SourceDescriptor.Artifact.ArtifactID,
			ContentHash: req.SourceDescriptor.Artifact.ContentHash,
			MimeType:    req.SourceDescriptor.Artifact.MIMEType,
			Width:       req.SourceDescriptor.Artifact.Width,
			Height:      req.SourceDescriptor.Artifact.Height,
		},
		PipelineVersion: contracts.PipelineVersion,
		ConfigHash:      req.ConfigSnapshot.ConfigHash,
		FrameCount:      frameCount,
		Frames:          manifestFrames,
	}

	hash := sha256.New()
	for _, mf := range manifest.Frames {
		hash.Write([]byte(mf.FileHash))
		hash.Write([]byte(mf.PixelHash))
	}
	hash.Write([]byte(manifest.ConfigHash))
	hash.Write([]byte(manifest.PipelineVersion))
	manifest.RevisionHash = hex.EncodeToString(hash.Sum(nil))

	if err := journal.Record("validated", "done", "manifest created and validated"); err != nil {
		return nil, fmt.Errorf("pipeline: record validated: %w", err)
	}

	rootRelativePath, err := p.publisher.Publish(artifact.PublishRequest{
		WorkDir:            workDir,
		GenerationTaskID:   req.GenerationTaskID,
		RevisionID:         revisionID,
		ProcessingTaskID:   req.ProcessingTaskID,
		ProcessingActionID: req.ProcessingActionID,
		ActionKey:          req.ActionKey,
		ProcessingVersion:  req.ProcessingVersion,
		Manifest:           manifest,
		Journal:            journal,
	})
	if err != nil {
		_ = journal.Record("publish", "failed", err.Error())
		return nil, fmt.Errorf("pipeline: publish stage: %w", err)
	}

	if err := journal.Record("publish", "done", fmt.Sprintf("published to %s", rootRelativePath)); err != nil {
		return nil, fmt.Errorf("pipeline: record publish: %w", err)
	}

	return &ProcessActionResult{
		RevisionID:          revisionID,
		FrameCount:          frameCount,
		Manifest:            manifest,
		Measurements:        measurements,
		SequenceMeasurement: seqMeasurement,
		RootRelativePath:    rootRelativePath,
	}, nil
}

func ensureForeground(result *backgroundremoval.BackgroundRemovalResult, original *image.NRGBA) *image.NRGBA {
	if result.Foreground != nil {
		return result.Foreground
	}
	if result.Image != nil {
		return imageToNRGBA(result.Image)
	}
	return original
}

func ensureMask(result *backgroundremoval.BackgroundRemovalResult, foreground *image.NRGBA) *image.Gray {
	if result.Mask != nil {
		return result.Mask
	}
	return maskFromAlpha(foreground)
}

func maskFromAlpha(img *image.NRGBA) *image.Gray {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	mask := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			mask.Pix[y*mask.Stride+x] = img.Pix[y*img.Stride+x*4+3]
		}
	}
	return mask
}

func imageToNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgba
	}
	bounds := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}
	return dst
}
