package interaction

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

const (
	VisualTextMatchExact    = "exact"
	VisualTextMatchContains = "contains"
	VisualTextMatchRegex    = "regex"
	VisualTextMatchFuzzy    = "fuzzy"
)

type VisualLocator interface {
	Locate(ctx context.Context, request VisualLocateRequest) ([]VisualCandidate, error)
}

type VisualCandidateValidator interface {
	ValidateCandidate(ctx context.Context, candidate VisualCandidate) error
}

type VisualProviderHealth interface {
	Available(ctx context.Context) (bool, string)
}

type VisualProviderState struct {
	ScreenshotAvailable      bool
	OCRAvailable             bool
	ImageUnderstandAvailable bool
	Reason                   string
}

type VisualCapabilityProbe interface {
	ProviderState(ctx context.Context) VisualProviderState
}

type ImageIntelligenceOCR interface {
	Recognize(ctx context.Context, imageRef string) (*OCRResult, error)
}

type ImageIntelligenceUnderstand interface {
	Understand(ctx context.Context, imageRef string, description string) (*UnderstandResult, error)
}

type ScreenshotProvider interface {
	Capture(ctx context.Context, displayID int) (*ScreenshotResult, error)
}

type OCRResult struct {
	Lines []OCRLine
}

type OCRLine struct {
	Text       string     `json:"text"`
	Bounds     uitreeRect `json:"bounds"`
	Confidence float64    `json:"confidence"`
}

type UnderstandResult struct {
	Candidates []VisualCandidate
}

type ScreenshotResult struct {
	ResourceURI string `json:"resourceUri"`
	DisplayID   int    `json:"displayId"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	CapturedAt  int64  `json:"capturedAt"`
	Generation  int64  `json:"generation"`
	StateToken  string `json:"stateToken,omitempty"`
	ModelWidth  int    `json:"modelWidth"`
	ModelHeight int    `json:"modelHeight"`
}

type uitreeRect = uitree.Rect

type DefaultVisualLocator struct {
	screenshot   ScreenshotProvider
	ocr          ImageIntelligenceOCR
	understand   ImageIntelligenceUnderstand
	nodeResolver uitree.NodeResolver
	policy       Policy
}

func NewDefaultVisualLocator(screenshot ScreenshotProvider, ocr ImageIntelligenceOCR, understand ImageIntelligenceUnderstand, nodeResolver uitree.NodeResolver, policy Policy) *DefaultVisualLocator {
	return &DefaultVisualLocator{screenshot: screenshot, ocr: ocr, understand: understand, nodeResolver: nodeResolver, policy: policy}
}

func (v *DefaultVisualLocator) ProviderState(ctx context.Context) VisualProviderState {
	state := VisualProviderState{}
	if v.screenshot == nil {
		state.Reason = "screenshot provider not configured"
		return state
	}
	state.ScreenshotAvailable = providerAvailable(ctx, v.screenshot)
	if !state.ScreenshotAvailable {
		state.Reason = "screenshot provider unavailable"
		return state
	}
	state.OCRAvailable = v.ocr != nil && providerAvailable(ctx, v.ocr)
	state.ImageUnderstandAvailable = v.understand != nil && providerAvailable(ctx, v.understand)
	if !state.OCRAvailable && !state.ImageUnderstandAvailable {
		state.Reason = "no healthy OCR or image understanding provider"
	}
	return state
}

func providerAvailable(ctx context.Context, provider any) bool {
	if health, ok := provider.(VisualProviderHealth); ok {
		available, _ := health.Available(ctx)
		return available
	}
	return provider != nil
}

func (v *DefaultVisualLocator) Locate(ctx context.Context, request VisualLocateRequest) ([]VisualCandidate, error) {
	if v.screenshot == nil {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: "screenshot provider not available"}
	}
	if ok, reason := visualProviderAvailable(ctx, v.screenshot); !ok {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: reason}
	}

	screenshot, err := v.screenshot.Capture(ctx, request.DisplayID)
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
		}
	} else if request.Description != "" && v.understand != nil {
		candidates, err = v.locateByUnderstand(ctx, screenshot, request)
	} else if request.Text != "" && v.ocr != nil {
		candidates, err = v.locateByOCR(ctx, screenshot, request)
	}
	if err != nil {
		return nil, err
	}
	for i := range candidates {
		candidates[i].DisplayID = screenshot.DisplayID
		candidates[i].ScreenshotGeneration = screenshot.Generation
		candidates[i].ScreenStateToken = screenshot.StateToken
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Confidence > candidates[j].Confidence })
	if len(candidates) > v.policy.MaxVisualCandidates {
		candidates = candidates[:v.policy.MaxVisualCandidates]
	}
	return candidates, nil
}

func (v *DefaultVisualLocator) ValidateCandidate(ctx context.Context, candidate VisualCandidate) error {
	if v.screenshot == nil || candidate.ScreenStateToken == "" {
		return nil
	}
	fresh, err := v.screenshot.Capture(ctx, candidate.DisplayID)
	if err != nil {
		return &Error{Code: INTERACTION_SCREENSHOT_FAILED, Message: "failed to re-capture before action: " + err.Error()}
	}
	if fresh.StateToken != "" && fresh.StateToken != candidate.ScreenStateToken {
		return &Error{Code: INTERACTION_VISUAL_STATE_STALE, Message: "screen changed after visual locate; relocate before action"}
	}
	return nil
}

func (v *DefaultVisualLocator) locateByOCR(ctx context.Context, screenshot *ScreenshotResult, request VisualLocateRequest) ([]VisualCandidate, error) {
	if v.ocr == nil {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: "OCR not available"}
	}
	if ok, reason := visualProviderAvailable(ctx, v.ocr); !ok {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: reason}
	}
	ocrResult, err := v.ocr.Recognize(ctx, screenshot.ResourceURI)
	if err != nil {
		return nil, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "OCR recognition failed: " + err.Error()}
	}
	var candidates []VisualCandidate
	for _, line := range ocrResult.Lines {
		if !matchesTextMode(line.Text, request.Text, request.TextMatchMode) {
			continue
		}
		bounds := toDisplayRect(line.Bounds, screenshot)
		candidate := VisualCandidate{Source: StrategyVisualOCR, Text: line.Text, Bounds: bounds, CenterX: bounds.CenterX(), CenterY: bounds.CenterY(), Confidence: line.Confidence}
		if candidate.CenterX >= 0 && candidate.CenterY >= 0 {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return nil, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "no OCR match found"}
	}
	return candidates, nil
}

func (v *DefaultVisualLocator) locateByUnderstand(ctx context.Context, screenshot *ScreenshotResult, request VisualLocateRequest) ([]VisualCandidate, error) {
	if v.understand == nil {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: "image understanding not available"}
	}
	if ok, reason := visualProviderAvailable(ctx, v.understand); !ok {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: reason}
	}
	description := request.Description
	if description == "" {
		description = request.Text
	}
	result, err := v.understand.Understand(ctx, screenshot.ResourceURI, description)
	if err != nil {
		return nil, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "image understanding failed: " + err.Error()}
	}
	candidates := make([]VisualCandidate, 0, len(result.Candidates))
	for _, c := range result.Candidates {
		c.Bounds = toDisplayRect(c.Bounds, screenshot)
		c.CenterX = c.Bounds.CenterX()
		c.CenterY = c.Bounds.CenterY()
		c.Source = StrategyVisualUnderstand
		candidates = append(candidates, c)
	}
	if len(candidates) == 0 {
		return nil, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "no visual target found"}
	}
	return candidates, nil
}

func visualProviderAvailable(ctx context.Context, provider any) (bool, string) {
	if health, ok := provider.(VisualProviderHealth); ok {
		available, reason := health.Available(ctx)
		if !available && strings.TrimSpace(reason) == "" {
			reason = "provider unavailable"
		}
		return available, reason
	}
	return provider != nil, "provider not configured"
}

func matchesTextMode(lineText, query, mode string) bool {
	if query == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case VisualTextMatchExact:
		return strings.EqualFold(lineText, query)
	case VisualTextMatchRegex:
		re, err := regexp.Compile(query)
		return err == nil && re.MatchString(lineText)
	case VisualTextMatchFuzzy:
		return fuzzyTextMatch(lineText, query)
	default:
		return strings.Contains(strings.ToLower(lineText), strings.ToLower(query))
	}
}

func fuzzyTextMatch(value, query string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	query = strings.ToLower(strings.TrimSpace(query))
	if value == query || strings.Contains(value, query) {
		return true
	}
	if query == "" || value == "" {
		return false
	}
	d := levenshtein(value, query)
	maxLen := len([]rune(value))
	if q := len([]rune(query)); q > maxLen {
		maxLen = q
	}
	return maxLen > 0 && float64(d)/float64(maxLen) <= 0.25
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ca := range ra {
		cur := make([]int, len(rb)+1)
		cur[0] = i + 1
		for j, cb := range rb {
			cost := 0
			if ca != cb {
				cost = 1
			}
			cur[j+1] = min3(cur[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
func min3(a, b, c int) int {
	if a > b {
		a = b
	}
	if a > c {
		a = c
	}
	return a
}

func toDisplayRect(bounds uitreeRect, screenshot *ScreenshotResult) uitree.Rect {
	if screenshot.ModelWidth <= 0 || screenshot.ModelHeight <= 0 {
		return bounds
	}
	return uitree.Rect{Left: bounds.Left * screenshot.Width / screenshot.ModelWidth, Top: bounds.Top * screenshot.Height / screenshot.ModelHeight, Right: bounds.Right * screenshot.Width / screenshot.ModelWidth, Bottom: bounds.Bottom * screenshot.Height / screenshot.ModelHeight}
}
