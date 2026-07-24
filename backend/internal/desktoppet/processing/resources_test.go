// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourceWriteActionFrames(t *testing.T) {
	dir := t.TempDir()
	w := NewResourceWriter(dir)

	frames := []image.Image{
		newTestImage(64, 64, color.NRGBA{255, 0, 0, 255}),
		newTestImage(64, 64, color.NRGBA{0, 255, 0, 255}),
		newTestImage(64, 64, color.NRGBA{0, 0, 255, 255}),
	}

	paths, err := w.WriteActionFrames("task-1", 1, "idle_normal", frames)
	if err != nil {
		t.Fatalf("WriteActionFrames failed: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}

	for i, p := range paths {
		if !strings.HasPrefix(p, "frames/frame-") {
			t.Errorf("path %d unexpected: %s", i, p)
		}
		if !strings.HasSuffix(p, ".png") {
			t.Errorf("path %d should end with .png: %s", i, p)
		}
	}

	for i := 0; i < 3; i++ {
		fp := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", "version-1", "actions", "idle_normal", filepath.FromSlash(paths[i]))
		if _, err := os.Stat(fp); err != nil {
			t.Errorf("frame file %d not exist: %v", i, err)
		}
	}

	actionTmpDir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", ".tmp", "idle_normal")
	if _, err := os.Stat(actionTmpDir); !os.IsNotExist(err) {
		t.Errorf("action tmp dir should be removed, got err=%v", err)
	}
}

func TestResourceWriteActionFramesZeroPadding(t *testing.T) {
	dir := t.TempDir()
	w := NewResourceWriter(dir)

	frames := make([]image.Image, 12)
	for i := range frames {
		frames[i] = newTestImage(8, 8, color.NRGBA{255, 255, 255, 255})
	}

	paths, err := w.WriteActionFrames("task-pad", 1, "idle_breathing", frames)
	if err != nil {
		t.Fatalf("WriteActionFrames failed: %v", err)
	}
	if len(paths) != 12 {
		t.Fatalf("expected 12 paths, got %d", len(paths))
	}

	if paths[0] != "frames/frame-0001.png" {
		t.Errorf("expected frame-0001.png, got %s", paths[0])
	}
	if paths[9] != "frames/frame-0010.png" {
		t.Errorf("expected frame-0010.png, got %s", paths[9])
	}
	if paths[11] != "frames/frame-0012.png" {
		t.Errorf("expected frame-0012.png, got %s", paths[11])
	}
}

func TestResourceWriteActionFramesNoFrames(t *testing.T) {
	dir := t.TempDir()
	w := NewResourceWriter(dir)

	_, err := w.WriteActionFrames("task-empty", 1, "idle_normal", nil)
	if err == nil {
		t.Fatal("expected error for empty frames")
	}
}

func TestResourceWriteActionFramesNilFrame(t *testing.T) {
	dir := t.TempDir()
	w := NewResourceWriter(dir)

	frames := []image.Image{
		newTestImage(32, 32, color.NRGBA{255, 0, 0, 255}),
		nil,
	}

	_, err := w.WriteActionFrames("task-nil", 1, "idle_normal", frames)
	if err == nil {
		t.Fatal("expected error for nil frame")
	}
}

func TestResourceWriteActionFramesInvalidVersion(t *testing.T) {
	dir := t.TempDir()
	w := NewResourceWriter(dir)

	frames := []image.Image{newTestImage(8, 8, color.NRGBA{255, 0, 0, 255})}
	_, err := w.WriteActionFrames("task-v", 0, "idle_normal", frames)
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestResourceWriteActionFramesAtomicMove(t *testing.T) {
	dir := t.TempDir()
	w := NewResourceWriter(dir)

	frames := []image.Image{
		newTestImage(16, 16, color.NRGBA{10, 20, 30, 255}),
	}

	paths, err := w.WriteActionFrames("task-atomic", 2, "wave", frames)
	if err != nil {
		t.Fatalf("WriteActionFrames failed: %v", err)
	}

	fp := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-atomic", "processed", "version-2", "actions", "wave", filepath.FromSlash(paths[0]))

	f, err := os.Open(fp)
	if err != nil {
		t.Fatalf("open frame file failed: %v", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode png failed: %v", err)
	}

	b := img.Bounds()
	if b.Dx() != 16 || b.Dy() != 16 {
		t.Errorf("expected 16x16, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestResourceWriteActionJSON(t *testing.T) {
	dir := t.TempDir()
	w := NewResourceWriter(dir)

	anchor := DefaultAnchorForActionKey("idle_normal")
	action := BuildActionJSON("idle_normal", "待机", 3, 8, anchor, "loop")

	err := w.WriteActionJSON("task-json", 1, action)
	if err != nil {
		t.Fatalf("WriteActionJSON failed: %v", err)
	}

	p := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-json", "processed", "version-1", "actions", "idle_normal", "action.json")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read action.json failed: %v", err)
	}

	var got ActionJSON
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal action.json failed: %v", err)
	}
	if got.Key != "idle_normal" {
		t.Errorf("expected key idle_normal, got %s", got.Key)
	}
	if got.FrameCount != 3 {
		t.Errorf("expected frame count 3, got %d", got.FrameCount)
	}
	if got.Fps != 8 {
		t.Errorf("expected fps 8, got %d", got.Fps)
	}
	if got.FrameDurationMs != 125 {
		t.Errorf("expected durationMs 125, got %d", got.FrameDurationMs)
	}
	if got.ReturnAction != "idle_normal" {
		t.Errorf("expected returnAction idle_normal, got %s", got.ReturnAction)
	}
	if !got.Interruptible {
		t.Errorf("expected interruptible true")
	}
	if len(got.Frames) != 3 {
		t.Errorf("expected 3 frames, got %d", len(got.Frames))
	}
	if got.Frames[0].File != "frames/frame-0001.png" {
		t.Errorf("expected frame file frames/frame-0001.png, got %s", got.Frames[0].File)
	}
	if got.Frames[2].Index != 2 {
		t.Errorf("expected frame index 2, got %d", got.Frames[2].Index)
	}
	if got.Anchor.Type != string(AnchorFeetCenter) {
		t.Errorf("expected anchor type feet_center, got %s", got.Anchor.Type)
	}
	if got.Anchor.X != 0.5 || got.Anchor.Y != 0.92 {
		t.Errorf("expected anchor 0.5,0.92, got %v,%v", got.Anchor.X, got.Anchor.Y)
	}
}

func TestResourceWriteActionJSONNil(t *testing.T) {
	dir := t.TempDir()
	w := NewResourceWriter(dir)

	err := w.WriteActionJSON("task-nil", 1, nil)
	if err == nil {
		t.Fatal("expected error for nil action")
	}
}

func TestResourceBuildActionJSONLoop(t *testing.T) {
	anchor := DefaultAnchorForActionKey("walk_left")
	a := BuildActionJSON("walk_left", "向左走", 4, 12, anchor, "loop")
	if a.ReturnAction != "idle_normal" {
		t.Errorf("expected returnAction idle_normal for loop, got %s", a.ReturnAction)
	}
	if a.LoopType != "loop" {
		t.Errorf("expected loopType loop, got %s", a.LoopType)
	}
	if a.Anchor.Type != string(AnchorFeetCenter) {
		t.Errorf("expected anchor type feet_center, got %s", a.Anchor.Type)
	}
	if a.Version != 1 {
		t.Errorf("expected version 1, got %d", a.Version)
	}
}

func TestResourceBuildActionJSONNonLoop(t *testing.T) {
	anchor := DefaultAnchorForActionKey("wave")
	a := BuildActionJSON("wave", "挥手", 2, 10, anchor, "once")
	if a.ReturnAction != "" {
		t.Errorf("expected empty returnAction for non-loop, got %s", a.ReturnAction)
	}
	if a.LoopType != "once" {
		t.Errorf("expected loopType once, got %s", a.LoopType)
	}
}

func TestResourceBuildActionJSONZeroFPS(t *testing.T) {
	anchor := DefaultAnchorForActionKey("idle_normal")
	a := BuildActionJSON("idle_normal", "待机", 2, 0, anchor, "loop")
	if a.Fps != 8 {
		t.Errorf("expected default fps 8, got %d", a.Fps)
	}
	if a.FrameDurationMs != 125 {
		t.Errorf("expected durationMs 125, got %d", a.FrameDurationMs)
	}
}

func TestResourceBuildActionJSONNegativeFrameCount(t *testing.T) {
	anchor := DefaultAnchorForActionKey("idle_normal")
	a := BuildActionJSON("idle_normal", "待机", -1, 8, anchor, "loop")
	if a.FrameCount != 0 {
		t.Errorf("expected frame count 0 for negative input, got %d", a.FrameCount)
	}
	if len(a.Frames) != 0 {
		t.Errorf("expected empty frames for negative input, got %d", len(a.Frames))
	}
}

func TestResourceDefaultFPSIdle(t *testing.T) {
	cases := map[string]int{
		"idle_normal":    8,
		"idle_breathing": 8,
		"idle_blink":     8,
		"idle_sway":      8,
	}
	for key, expected := range cases {
		if got := DefaultFPSForAction(key); got != expected {
			t.Errorf("for %s expected %d, got %d", key, expected, got)
		}
	}
}

func TestResourceDefaultFPSSleep(t *testing.T) {
	cases := map[string]int{
		"sleep_start": 6,
		"sleep_loop":  6,
		"sleep_end":   6,
	}
	for key, expected := range cases {
		if got := DefaultFPSForAction(key); got != expected {
			t.Errorf("for %s expected %d, got %d", key, expected, got)
		}
	}
}

func TestResourceDefaultFPSWalkRun(t *testing.T) {
	cases := map[string]int{
		"walk_left":  12,
		"walk_right": 12,
		"run_left":   12,
		"run_right":  12,
	}
	for key, expected := range cases {
		if got := DefaultFPSForAction(key); got != expected {
			t.Errorf("for %s expected %d, got %d", key, expected, got)
		}
	}
}

func TestResourceDefaultFPSClick(t *testing.T) {
	cases := map[string]int{
		"click_happy": 12,
		"click_angry": 12,
	}
	for key, expected := range cases {
		if got := DefaultFPSForAction(key); got != expected {
			t.Errorf("for %s expected %d, got %d", key, expected, got)
		}
	}
}

func TestResourceDefaultFPSEmotion(t *testing.T) {
	cases := map[string]int{
		"happy":    10,
		"wave":     10,
		"speaking": 10,
	}
	for key, expected := range cases {
		if got := DefaultFPSForAction(key); got != expected {
			t.Errorf("for %s expected %d, got %d", key, expected, got)
		}
	}
}

func TestResourceDefaultFPSUnknown(t *testing.T) {
	if got := DefaultFPSForAction("unknown_action"); got != 10 {
		t.Errorf("expected 10 for unknown, got %d", got)
	}
	if got := DefaultFPSForAction(""); got != 10 {
		t.Errorf("expected 10 for empty, got %d", got)
	}
}

func TestResourceFPSFromSpeedSlow(t *testing.T) {
	if got := FPSFromSpeed("slow", 12); got != 9 {
		t.Errorf("expected 9 for slow*12, got %d", got)
	}
	if got := FPSFromSpeed("SLOW", 8); got != 6 {
		t.Errorf("expected 6 for SLOW*8, got %d", got)
	}
}

func TestResourceFPSFromSpeedStandard(t *testing.T) {
	if got := FPSFromSpeed("standard", 10); got != 10 {
		t.Errorf("expected 10 for standard*10, got %d", got)
	}
	if got := FPSFromSpeed("STANDARD", 12); got != 12 {
		t.Errorf("expected 12 for STANDARD*12, got %d", got)
	}
}

func TestResourceFPSFromSpeedFast(t *testing.T) {
	if got := FPSFromSpeed("fast", 8); got != 10 {
		t.Errorf("expected 10 for fast*8, got %d", got)
	}
	if got := FPSFromSpeed("FAST", 12); got != 15 {
		t.Errorf("expected 15 for FAST*12, got %d", got)
	}
}

func TestResourceFPSFromSpeedUnknown(t *testing.T) {
	if got := FPSFromSpeed("unknown", 12); got != 12 {
		t.Errorf("expected 12 for unknown*12, got %d", got)
	}
	if got := FPSFromSpeed("", 10); got != 10 {
		t.Errorf("expected 10 for empty*10, got %d", got)
	}
}

func TestResourceFPSFromSpeedZeroBase(t *testing.T) {
	if got := FPSFromSpeed("standard", 0); got != 10 {
		t.Errorf("expected 10 for standard*0, got %d", got)
	}
	if got := FPSFromSpeed("fast", 0); got != 12 {
		t.Errorf("expected 12 for fast*0, got %d", got)
	}
}
