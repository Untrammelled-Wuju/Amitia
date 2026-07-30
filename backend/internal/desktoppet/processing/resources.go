// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FrameInfo struct {
	Index      int    `json:"index"`
	File       string `json:"file"`
	DurationMs int    `json:"durationMs"`
}

type ActionJSON struct {
	Key             string      `json:"key"`
	Name            string      `json:"name"`
	Version         int         `json:"version"`
	LoopType        string      `json:"loopType"`
	Fps             int         `json:"fps"`
	FrameDurationMs int         `json:"frameDurationMs"`
	FrameCount      int         `json:"frameCount"`
	Frames          []FrameInfo `json:"frames"`
	Anchor          AnchorJSON  `json:"anchor"`
	Interruptible   bool        `json:"interruptible"`
	ReturnAction    string      `json:"returnAction"`
	PlaybackMode    string      `json:"playbackMode"`
	ReturnPolicy    string      `json:"returnPolicy"`
	Priority        int         `json:"priority"`
	CooldownMs      int         `json:"cooldownMs"`
	MutexGroup      string      `json:"mutexGroup"`
	QueuePolicy     string      `json:"queuePolicy"`
	DedupWindowMs   int         `json:"dedupWindowMs"`
	InterruptAfterMs int        `json:"interruptAfterMs"`
	MinimumPlayMs   int         `json:"minimumPlayMs"`
	MaximumPlayMs   int         `json:"maximumPlayMs"`
	AnchorProfile   string      `json:"anchorProfile"`
	ActionSpecHash  string      `json:"actionSpecHash"`
}

type AnchorJSON struct {
	Type string  `json:"type"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

type ResourceWriter struct {
	dataDir string
}

func NewResourceWriter(dataDir string) *ResourceWriter {
	return &ResourceWriter{dataDir: dataDir}
}

func (w *ResourceWriter) WriteActionFrames(taskID string, processingVersion int, actionKey string, frames []image.Image) ([]string, error) {
	if taskID == "" {
		return nil, fmt.Errorf("taskID is empty")
	}
	if actionKey == "" {
		return nil, fmt.Errorf("actionKey is empty")
	}
	if processingVersion <= 0 {
		return nil, fmt.Errorf("processingVersion must be positive")
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("frames is empty")
	}

	processedDir := filepath.Join(w.dataDir, "desktop-pets", "generation-tasks", taskID, "processed")
	framesDir := filepath.Join(processedDir, fmt.Sprintf("version-%d", processingVersion), "actions", actionKey, "frames")
	tmpDir := filepath.Join(processedDir, ".tmp", actionKey)

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("create tmp dir failed: %w", err)
	}

	relPaths := make([]string, 0, len(frames))
	tmpPaths := make([]string, 0, len(frames))

	for i, frame := range frames {
		if frame == nil {
			return nil, fmt.Errorf("frame %d is nil", i)
		}

		fileName := fmt.Sprintf("frame-%04d.png", i+1)
		tmpPath := filepath.Join(tmpDir, fileName)
		relPath := filepath.ToSlash(filepath.Join("frames", fileName))

		f, err := os.Create(tmpPath)
		if err != nil {
			return nil, fmt.Errorf("create frame file %s failed: %w", tmpPath, err)
		}
		if err := png.Encode(f, frame); err != nil {
			f.Close()
			return nil, fmt.Errorf("encode frame %d failed: %w", i, err)
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("close frame file %s failed: %w", tmpPath, err)
		}

		tmpPaths = append(tmpPaths, tmpPath)
		relPaths = append(relPaths, relPath)
	}

	if err := os.MkdirAll(framesDir, 0755); err != nil {
		return nil, fmt.Errorf("create frames dir failed: %w", err)
	}

	for i, tmpPath := range tmpPaths {
		finalPath := filepath.Join(framesDir, filepath.Base(tmpPath))
		if err := atomicMove(tmpPath, finalPath); err != nil {
			return nil, fmt.Errorf("move frame %d to final failed: %w", i, err)
		}
	}

	os.Remove(tmpDir)
	return relPaths, nil
}

func (w *ResourceWriter) WriteActionJSON(taskID string, processingVersion int, action *ActionJSON) error {
	if taskID == "" {
		return fmt.Errorf("taskID is empty")
	}
	if action == nil {
		return fmt.Errorf("action is nil")
	}
	if processingVersion <= 0 {
		return fmt.Errorf("processingVersion must be positive")
	}
	if action.Key == "" {
		return fmt.Errorf("action key is empty")
	}

	actionsDir := filepath.Join(w.dataDir, "desktop-pets", "generation-tasks", taskID, "processed", fmt.Sprintf("version-%d", processingVersion), "actions", action.Key)
	if err := os.MkdirAll(actionsDir, 0755); err != nil {
		return fmt.Errorf("create actions dir failed: %w", err)
	}

	finalPath := filepath.Join(actionsDir, "action.json")
	tmpPath := filepath.Join(actionsDir, ".action.json.tmp")

	data, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal action json failed: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp action json failed: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename action json failed: %w", err)
	}

	return nil
}

func BuildActionJSON(actionKey, actionName string, frameCount, fps int, anchor Anchor, loopType string) *ActionJSON {
	if fps <= 0 {
		fps = DefaultFPSForAction(actionKey)
	}
	durationMs := 1000 / fps
	if durationMs <= 0 {
		durationMs = 100
	}

	if frameCount < 0 {
		frameCount = 0
	}

	frames := make([]FrameInfo, frameCount)
	for i := 0; i < frameCount; i++ {
		frames[i] = FrameInfo{
			Index:      i,
			File:       fmt.Sprintf("frames/frame-%04d.png", i+1),
			DurationMs: durationMs,
		}
	}

	returnAction := ""
	playbackMode := "once"
	returnPolicy := "previous"
	if IsLoopAction(actionKey) {
		returnAction = "idle_normal"
		playbackMode = "loop"
		returnPolicy = "none"
	}
	if loopType == "loop" {
		playbackMode = "loop"
		returnPolicy = "none"
	}

	return &ActionJSON{
		Key:             actionKey,
		Name:            actionName,
		Version:         1,
		LoopType:        loopType,
		Fps:             fps,
		FrameDurationMs: durationMs,
		FrameCount:      frameCount,
		Frames:          frames,
		Anchor: AnchorJSON{
			Type: string(anchor.Type),
			X:    anchor.X,
			Y:    anchor.Y,
		},
		Interruptible:  true,
		ReturnAction:   returnAction,
		PlaybackMode:   playbackMode,
		ReturnPolicy:   returnPolicy,
		QueuePolicy:    "replace",
		AnchorProfile:  "feet_center",
	}
}

func EnrichActionJSONFromSpec(a *ActionJSON, action *ProcessingAction) {
	if a == nil || action == nil {
		return
	}
	if action.PlaybackMode != "" {
		a.PlaybackMode = action.PlaybackMode
	}
	if action.ReturnPolicy != "" {
		a.ReturnPolicy = action.ReturnPolicy
	}
	if action.ReturnActionKey != "" {
		a.ReturnAction = action.ReturnActionKey
	}
	a.Priority = action.Priority
	a.CooldownMs = action.CooldownMS
	a.MutexGroup = action.MutexGroup
	if action.QueuePolicy != "" {
		a.QueuePolicy = action.QueuePolicy
	}
	a.DedupWindowMs = action.DedupWindowMS
	a.InterruptAfterMs = action.InterruptAfterMS
	a.MinimumPlayMs = action.MinimumPlayMS
	a.MaximumPlayMs = action.MaximumPlayMS
	if action.AnchorProfile != "" {
		a.AnchorProfile = action.AnchorProfile
	}
	if action.ActionSpecHash != "" {
		a.ActionSpecHash = action.ActionSpecHash
	}
	a.Interruptible = action.Interruptible != 0
}

func DefaultFPSForAction(actionKey string) int {
	switch {
	case strings.HasPrefix(actionKey, "idle_"):
		return 8
	case strings.HasPrefix(actionKey, "sleep_"):
		return 6
	case strings.HasPrefix(actionKey, "walk_"), strings.HasPrefix(actionKey, "run_"):
		return 12
	case strings.HasPrefix(actionKey, "click_"):
		return 12
	case actionKey == "happy", actionKey == "wave", actionKey == "speaking":
		return 10
	default:
		return 10
	}
}

func FPSFromSpeed(speed string, baseFPS int) int {
	if baseFPS <= 0 {
		baseFPS = 10
	}
	switch strings.ToLower(speed) {
	case "slow":
		return int(float64(baseFPS) * 0.75)
	case "fast":
		return int(float64(baseFPS) * 1.25)
	case "standard":
		return baseFPS
	default:
		return baseFPS
	}
}

func atomicMove(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return os.Remove(src)
}
