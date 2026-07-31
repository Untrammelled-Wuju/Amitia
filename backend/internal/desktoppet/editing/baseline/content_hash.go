package baseline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

type contentHashInput struct {
	ActionKey         string                 `json:"actionKey"`
	ActionSpecVersion string                 `json:"actionSpecVersion"`
	PlaybackMode      string                 `json:"playbackMode"`
	FPS               int                    `json:"fps"`
	Anchor            map[string]interface{} `json:"anchor"`
	Frames            []frameHashInput       `json:"frames"`
}

type frameHashInput struct {
	Index             int                    `json:"index"`
	ContentHash       string                 `json:"contentHash"`
	DurationMS        int                    `json:"durationMs"`
	TransformMetadata map[string]interface{} `json:"transformMetadata,omitempty"`
}

type contentHashInputV2 struct {
	ActionKey              string             `json:"actionKey"`
	ActionSpecHash         string             `json:"actionSpecHash"`
	ActionConfigHash       string             `json:"actionConfigHash"`
	PlaybackMode           string             `json:"playbackMode"`
	FPS                    int                `json:"fps"`
	Interruptible          bool               `json:"interruptible"`
	InterruptAfterMS       int                `json:"interruptAfterMs"`
	Priority               int                `json:"priority"`
	CooldownMS             int                `json:"cooldownMs"`
	MinimumPlayMS          int                `json:"minimumPlayMs"`
	MaximumPlayMS          *int               `json:"maximumPlayMs"`
	MutexGroup             string             `json:"mutexGroup"`
	SupportsDefaultIdle    bool               `json:"supportsDefaultIdle"`
	IsStableStateCandidate bool               `json:"isStableStateCandidate"`
	IsTransitionOnly       bool               `json:"isTransitionOnly"`
	ReturnTo               ReturnTarget       `json:"returnTo"`
	Anchor                 AnchorInfo         `json:"anchor"`
	Frames                 []frameHashInputV2 `json:"frames"`
}

type frameHashInputV2 struct {
	Index           int        `json:"logicalIndex"`
	ContentHash     string     `json:"contentHash"`
	DurationMS      int        `json:"durationMs"`
	Anchor          AnchorInfo `json:"anchor"`
	Offset          OffsetInfo `json:"offset"`
	MaskContentHash string     `json:"maskContentHash"`
	TransformHash   string     `json:"transformHash"`
	MeasurementHash string     `json:"measurementHash"`
}

type FrameHashInfo struct {
	Index             int
	ContentHash       string
	DurationMS        int
	TransformMetadata map[string]interface{}
	Anchor            AnchorInfo
	Offset            OffsetInfo
	MaskContentHash   string
	TransformHash     string
	MeasurementHash   string
}

type OffsetInfo struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type ActionConfigHashInfo struct {
	ActionKey              string
	ActionSpecHash         string
	ActionConfigHash       string
	PlaybackMode           string
	FPS                    int
	Interruptible          bool
	InterruptAfterMS       int
	Priority               int
	CooldownMS             int
	MinimumPlayMS          int
	MaximumPlayMS          *int
	MutexGroup             string
	SupportsDefaultIdle    bool
	IsStableStateCandidate bool
	IsTransitionOnly       bool
	ReturnTo               ReturnTarget
	Anchor                 AnchorInfo
}

type AnchorInfo struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Space string  `json:"space"`
}

func ComputeContentHash(
	actionKey string,
	actionSpecVersion string,
	playbackMode string,
	fps int,
	anchor AnchorInfo,
	frames []FrameHashInfo,
) string {
	sort.Slice(frames, func(i, j int) bool {
		return frames[i].Index < frames[j].Index
	})

	frameInputs := make([]frameHashInput, 0, len(frames))
	for _, f := range frames {
		frameInputs = append(frameInputs, frameHashInput{
			Index:             f.Index,
			ContentHash:       f.ContentHash,
			DurationMS:        f.DurationMS,
			TransformMetadata: f.TransformMetadata,
		})
	}

	input := contentHashInput{
		ActionKey:         actionKey,
		ActionSpecVersion: actionSpecVersion,
		PlaybackMode:      playbackMode,
		FPS:               fps,
		Anchor: map[string]interface{}{
			"x":     formatFloat(anchor.X),
			"y":     formatFloat(anchor.Y),
			"space": anchor.Space,
		},
		Frames: frameInputs,
	}

	data, err := json.Marshal(input)
	if err != nil {
		return ""
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	canonical, err := json.Marshal(canonicalize(raw))
	if err != nil {
		return ""
	}

	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func ComputeFrameSetHash(frames []FrameHashInfo) string {
	sort.Slice(frames, func(i, j int) bool {
		return frames[i].Index < frames[j].Index
	})

	frameInputs := make([]frameHashInput, 0, len(frames))
	for _, f := range frames {
		frameInputs = append(frameInputs, frameHashInput{
			Index:             f.Index,
			ContentHash:       f.ContentHash,
			DurationMS:        f.DurationMS,
			TransformMetadata: f.TransformMetadata,
		})
	}

	data, err := json.Marshal(frameInputs)
	if err != nil {
		return ""
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	canonical, err := json.Marshal(canonicalize(raw))
	if err != nil {
		return ""
	}

	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func ComputeContentHashV2(cfg ActionConfigHashInfo, frames []FrameHashInfo) (string, error) {
	sorted := make([]FrameHashInfo, len(frames))
	copy(sorted, frames)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Index < sorted[j].Index
	})

	input := contentHashInputV2{
		ActionKey:              cfg.ActionKey,
		ActionSpecHash:         cfg.ActionSpecHash,
		ActionConfigHash:       cfg.ActionConfigHash,
		PlaybackMode:           cfg.PlaybackMode,
		FPS:                    cfg.FPS,
		Interruptible:          cfg.Interruptible,
		InterruptAfterMS:       cfg.InterruptAfterMS,
		Priority:               cfg.Priority,
		CooldownMS:             cfg.CooldownMS,
		MinimumPlayMS:          cfg.MinimumPlayMS,
		MaximumPlayMS:          cfg.MaximumPlayMS,
		MutexGroup:             cfg.MutexGroup,
		SupportsDefaultIdle:    cfg.SupportsDefaultIdle,
		IsStableStateCandidate: cfg.IsStableStateCandidate,
		IsTransitionOnly:       cfg.IsTransitionOnly,
		ReturnTo:               cfg.ReturnTo,
		Anchor:                 cfg.Anchor,
		Frames:                 buildFrameHashInputsV2(sorted),
	}

	return computeCanonicalHash(input)
}

func ComputeFrameSetHashV2(frames []FrameHashInfo) (string, error) {
	sorted := make([]FrameHashInfo, len(frames))
	copy(sorted, frames)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Index < sorted[j].Index
	})

	return computeCanonicalHash(buildFrameHashInputsV2(sorted))
}

func buildFrameHashInputsV2(frames []FrameHashInfo) []frameHashInputV2 {
	result := make([]frameHashInputV2, 0, len(frames))
	for _, f := range frames {
		result = append(result, frameHashInputV2{
			Index:           f.Index,
			ContentHash:     f.ContentHash,
			DurationMS:      f.DurationMS,
			Anchor:          f.Anchor,
			Offset:          f.Offset,
			MaskContentHash: f.MaskContentHash,
			TransformHash:   f.TransformHash,
			MeasurementHash: f.MeasurementHash,
		})
	}
	return result
}

func ComputeActiveRevisionSetHash(refs []ActiveRevisionRef) (string, error) {
	sorted := make([]ActiveRevisionRef, len(refs))
	copy(sorted, refs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ActionKey < sorted[j].ActionKey
	})

	type hashEntry struct {
		ActionKey        string `json:"actionKey"`
		ActionRevisionID string `json:"actionRevisionId"`
		ContentHash      string `json:"contentHash"`
		BindingRevision  int64  `json:"bindingRevision"`
	}

	entries := make([]hashEntry, 0, len(sorted))
	for _, r := range sorted {
		entries = append(entries, hashEntry{
			ActionKey:        r.ActionKey,
			ActionRevisionID: r.ActionRevisionID,
			ContentHash:      r.ContentHash,
			BindingRevision:  r.BindingRevision,
		})
	}

	return computeCanonicalHash(entries)
}

func computeCanonicalHash(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("content_hash: marshal input: %w", err)
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("content_hash: unmarshal input: %w", err)
	}

	canonical, err := json.Marshal(canonicalize(raw))
	if err != nil {
		return "", fmt.Errorf("content_hash: marshal canonical: %w", err)
	}

	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalize(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		result := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			result[k] = canonicalize(val[k])
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = canonicalize(item)
		}
		return result
	case float64:
		return formatFloat(val)
	case json.Number:
		return val.String()
	default:
		return v
	}
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
