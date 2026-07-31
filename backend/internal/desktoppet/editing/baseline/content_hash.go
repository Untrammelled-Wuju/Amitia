package baseline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
)

type contentHashInput struct {
	ActionKey          string                 `json:"actionKey"`
	ActionSpecVersion  string                 `json:"actionSpecVersion"`
	PlaybackMode       string                 `json:"playbackMode"`
	FPS                int                    `json:"fps"`
	Anchor             map[string]interface{} `json:"anchor"`
	Frames             []frameHashInput       `json:"frames"`
}

type frameHashInput struct {
	Index             int                    `json:"index"`
	ContentHash       string                 `json:"contentHash"`
	DurationMS        int                    `json:"durationMs"`
	RelativePath      string                 `json:"relativePath"`
	TransformMetadata map[string]interface{} `json:"transformMetadata,omitempty"`
}

type FrameHashInfo struct {
	Index             int
	ContentHash       string
	DurationMS        int
	RelativePath      string
	TransformMetadata map[string]interface{}
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
			RelativePath:      f.RelativePath,
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
			RelativePath:      f.RelativePath,
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

func ComputeActiveRevisionSetHash(refs []ActiveRevisionRef) string {
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

	data, err := json.Marshal(entries)
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
