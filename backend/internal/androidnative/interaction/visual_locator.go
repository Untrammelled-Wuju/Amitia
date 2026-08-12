package interaction

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

type VisualLocator interface {
	Locate(
		ctx context.Context,
		request VisualLocateRequest,
	) ([]VisualCandidate, error)
}

type ImageIntelligenceOCR interface {
	Recognize(
		ctx context.Context,
		imageRef string,
	) (*OCRResult, error)
}

type ImageIntelligenceUnderstand interface {
	Understand(
		ctx context.Context,
		imageRef string,
		description string,
	) (*UnderstandResult, error)
}

type ScreenshotProvider interface {
	Capture(
		ctx context.Context,
		displayID int,
	) (*ScreenshotResult, error)
}

type OCRResult struct {
	Lines []OCRLine
}

type OCRLine struct {
	Text      string     `json:"text"`
	Bounds    uitreeRect `json:"bounds"`
	Confidence float64   `json:"confidence"`
}

type UnderstandResult struct {
	Candidates []VisualCandidate
}

type ScreenshotResult struct {
	ResourceURI   string `json:"resourceUri"`
	DisplayID     int    `json:"displayId"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	CapturedAt    int64  `json:"capturedAt"`
	ModelWidth    int    `json:"modelWidth"`
	ModelHeight   int    `json:"modelHeight"`
}

type uitreeRect = uitree.Rect

type DefaultVisualLocator struct {
	screenshot   ScreenshotProvider
	ocr          ImageIntelligenceOCR
	understand   ImageIntelligenceUnderstand
	nodeResolver uitree.NodeResolver
	policy       Policy
}

func NewDefaultVisualLocator(
	screenshot ScreenshotProvider,
	ocr ImageIntelligenceOCR,
	understand ImageIntelligenceUnderstand,
	nodeResolver uitree.NodeResolver,
	policy Policy,
) *DefaultVisualLocator {
	return &DefaultVisualLocator{
		screenshot:   screenshot,
		ocr:          ocr,
		understand:   understand,
		nodeResolver: nodeResolver,
		policy:       policy,
	}
}

func (v *DefaultVisualLocator) Locate(
	ctx context.Context,
	request VisualLocateRequest,
) ([]VisualCandidate, error) {
	if v.screenshot == nil {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: "screenshot provider not available"}
	}

	screenshot, err := v.screenshot.Capture(ctx, 0)
	if err != nil {
		return nil, &Error{Code: INTERACTION_SCREENSHOT_FAILED, Message: "failed to capture screenshot: " + err.Error()}
	}

	age := time.Since(time.UnixMilli(screenshot.CapturedAt))
	if age > v.policy.MaxScreenshotAge {
		return nil, &Error{Code: INTERACTION_VISUAL_STATE_STALE, Message: "screenshot too old"}
	}

	var candidates []VisualCandidate

	if request.Text != "" && (request.OCRFirst || v.understand == nil) {
		candidates, err = v.locateByOCR(ctx, screenshot, request)
		if err != nil && v.understand != nil {
			candidates, err = v.locateByUnderstand(ctx, screenshot, request)
			if err != nil {
				return nil, err
			}
		}
	} else if request.Description != "" && v.understand != nil {
		candidates, err = v.locateByUnderstand(ctx, screenshot, request)
		if err != nil {
			return nil, err
		}
	} else if request.Text != "" && v.ocr != nil {
		candidates, err = v.locateByOCR(ctx, screenshot, request)
		if err != nil {
			return nil, err
		}
	}

	if len(candidates) > v.policy.MaxVisualCandidates {
		candidates = candidates[:v.policy.MaxVisualCandidates]
	}

	return candidates, nil
}

func (v *DefaultVisualLocator) locateByOCR(
	ctx context.Context,
	screenshot *ScreenshotResult,
	request VisualLocateRequest,
) ([]VisualCandidate, error) {
	if v.ocr == nil {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: "OCR not available"}
	}

	ocrResult, err := v.ocr.Recognize(ctx, screenshot.ResourceURI)
	if err != nil {
		return nil, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "OCR recognition failed: " + err.Error()}
	}

	var candidates []VisualCandidate
	for _, line := range ocrResult.Lines {
		if matchesText(line.Text, request.Text) {
			candidate := VisualCandidate{
				Source:      StrategyVisualOCR,
				Text:        line.Text,
				Bounds:      toDisplayRect(line.Bounds, screenshot),
				CenterX:     (line.Bounds.Left + line.Bounds.Right) / 2,
				CenterY:     (line.Bounds.Top + line.Bounds.Bottom) / 2,
				Confidence:  line.Confidence,
			}
			if candidate.CenterX >= 0 && candidate.CenterY >= 0 {
				candidates = append(candidates, candidate)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "no OCR match found"}
	}

	return candidates, nil
}

func (v *DefaultVisualLocator) locateByUnderstand(
	ctx context.Context,
	screenshot *ScreenshotResult,
	request VisualLocateRequest,
) ([]VisualCandidate, error) {
	if v.understand == nil {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: "image understanding not available"}
	}

	description := request.Description
	if description == "" {
		description = request.Text
	}

	result, err := v.understand.Understand(ctx, screenshot.ResourceURI, description)
	if err != nil {
		return nil, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "image understanding failed: " + err.Error()}
	}

	var candidates []VisualCandidate
	for _, c := range result.Candidates {
		c.Bounds = toDisplayRect(c.Bounds, screenshot)
		c.CenterX = (c.Bounds.Left + c.Bounds.Right) / 2
		c.CenterY = (c.Bounds.Top + c.Bounds.Bottom) / 2
		c.Source = StrategyVisualUnderstand
		candidates = append(candidates, c)
	}

	if len(candidates) == 0 {
		return nil, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "no visual target found"}
	}

	return candidates, nil
}

func matchesText(lineText, query string) bool {
	if query == "" {
		return true
	}
	if lineText == query {
		return true
	}
	return strings.Contains(strings.ToLower(lineText), strings.ToLower(query))
}

func toDisplayRect(bounds uitreeRect, screenshot *ScreenshotResult) uitree.Rect {
	if screenshot.ModelWidth <= 0 || screenshot.ModelHeight <= 0 {
		return bounds
	}
	screenX := bounds.Left * screenshot.Width / screenshot.ModelWidth
	screenY := bounds.Top * screenshot.Height / screenshot.ModelHeight
	screenRight := bounds.Right * screenshot.Width / screenshot.ModelWidth
	screenBottom := bounds.Bottom * screenshot.Height / screenshot.ModelHeight
	return uitree.Rect{
		Left:   screenX,
		Top:    screenY,
		Right:  screenRight,
		Bottom: screenBottom,
	}
}
