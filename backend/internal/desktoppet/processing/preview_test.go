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
	"testing"
)

func TestPreviewGenerateActionPreview(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	frames := []image.Image{
		newTestImage(32, 32, color.NRGBA{255, 0, 0, 255}),
		newTestImage(32, 32, color.NRGBA{0, 255, 0, 255}),
		newTestImage(32, 32, color.NRGBA{0, 0, 255, 255}),
	}

	path, err := g.GenerateActionPreview("task-preview", 1, "idle_normal", frames)
	if err != nil {
		t.Fatalf("GenerateActionPreview failed: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	if filepath.Base(path) != "preview.png" {
		t.Errorf("expected preview.png, got %s", filepath.Base(path))
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("preview file not exist: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open preview failed: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode preview failed: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 32 || b.Dy() != 32 {
		t.Errorf("expected 32x32, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestPreviewGenerateActionPreviewMiddleFrame(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	frames := []image.Image{
		newTestImage(8, 8, color.NRGBA{255, 0, 0, 255}),
		newTestImage(8, 8, color.NRGBA{0, 255, 0, 255}),
		newTestImage(8, 8, color.NRGBA{0, 0, 255, 255}),
	}

	path, err := g.GenerateActionPreview("task-mid", 1, "idle_normal", frames)
	if err != nil {
		t.Fatalf("GenerateActionPreview failed: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open preview failed: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode preview failed: %v", err)
	}
	c := img.At(0, 0)
	_, gChannel, _, _ := c.RGBA()
	if gChannel == 0 {
		t.Errorf("expected green pixel from middle frame")
	}
}

func TestPreviewGenerateActionPreviewSingleFrame(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	frames := []image.Image{
		newTestImage(16, 16, color.NRGBA{255, 0, 0, 255}),
	}

	path, err := g.GenerateActionPreview("task-single", 1, "wave", frames)
	if err != nil {
		t.Fatalf("GenerateActionPreview failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("preview file not exist: %v", err)
	}
}

func TestPreviewGenerateActionPreviewEvenFrames(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	frames := []image.Image{
		newTestImage(8, 8, color.NRGBA{255, 0, 0, 255}),
		newTestImage(8, 8, color.NRGBA{0, 255, 0, 255}),
		newTestImage(8, 8, color.NRGBA{0, 0, 255, 255}),
		newTestImage(8, 8, color.NRGBA{255, 255, 0, 255}),
	}

	path, err := g.GenerateActionPreview("task-even", 1, "idle_normal", frames)
	if err != nil {
		t.Fatalf("GenerateActionPreview failed: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open preview failed: %v", err)
	}
	defer f.Close()
	img, _ := png.Decode(f)
	c := img.At(0, 0)
	_, gChannel, bChannel, _ := c.RGBA()
	if bChannel == 0 {
		t.Errorf("expected blue frame from middle (index 2), got g=%d b=%d", gChannel, bChannel)
	}
}

func TestPreviewGenerateActionPreviewEmpty(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	_, err := g.GenerateActionPreview("task-empty", 1, "idle_normal", nil)
	if err == nil {
		t.Fatal("expected error for empty frames")
	}
}

func TestPreviewGenerateActionPreviewNilFrame(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	frames := []image.Image{
		newTestImage(8, 8, color.NRGBA{255, 0, 0, 255}),
		nil,
		newTestImage(8, 8, color.NRGBA{0, 0, 255, 255}),
	}

	_, err := g.GenerateActionPreview("task-nil", 1, "idle_normal", frames)
	if err == nil {
		t.Fatal("expected error for nil middle frame")
	}
}

func TestPreviewGenerateActionPreviewInvalidVersion(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	frames := []image.Image{newTestImage(8, 8, color.NRGBA{255, 0, 0, 255})}
	_, err := g.GenerateActionPreview("task-v", 0, "idle_normal", frames)
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestPreviewGeneratePackagePreview(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	frames := []image.Image{
		newTestImage(64, 64, color.NRGBA{255, 0, 0, 255}),
		newTestImage(64, 64, color.NRGBA{0, 255, 0, 255}),
	}

	path, err := g.GeneratePackagePreview("task-pkg", 1, frames)
	if err != nil {
		t.Fatalf("GeneratePackagePreview failed: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	if filepath.Base(path) != "package-preview.png" {
		t.Errorf("expected package-preview.png, got %s", filepath.Base(path))
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("preview file not exist: %v", err)
	}
}

func TestPreviewGeneratePackagePreviewMiddleFrame(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	frames := []image.Image{
		newTestImage(16, 16, color.NRGBA{255, 0, 0, 255}),
		newTestImage(16, 16, color.NRGBA{0, 255, 0, 255}),
		newTestImage(16, 16, color.NRGBA{0, 0, 255, 255}),
	}

	path, err := g.GeneratePackagePreview("task-pkg-mid", 1, frames)
	if err != nil {
		t.Fatalf("GeneratePackagePreview failed: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open preview failed: %v", err)
	}
	defer f.Close()
	img, _ := png.Decode(f)
	c := img.At(0, 0)
	_, gChannel, _, _ := c.RGBA()
	if gChannel == 0 {
		t.Errorf("expected green pixel from middle frame")
	}
}

func TestPreviewGeneratePackagePreviewEmpty(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	_, err := g.GeneratePackagePreview("task-pkg-empty", 1, nil)
	if err == nil {
		t.Fatal("expected error for empty frames")
	}
}

func TestPreviewGenerateProcessingReport(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	report := &ProcessingReport{
		ProcessingTaskID:  "task-report",
		ProcessingVersion: 1,
		Actions: []ActionReport{
			{
				ActionKey:    "idle_normal",
				ActionName:   "待机",
				Status:       "succeeded",
				QualityLevel: QualityLevelNormal,
				QualityFlags: []string{},
				FrameCount:   3,
			},
			{
				ActionKey:    "wave",
				ActionName:   "挥手",
				Status:       "succeeded",
				QualityLevel: QualityLevelWarning,
				QualityFlags: []string{FlagDuplicateFrame},
				FrameCount:   2,
			},
			{
				ActionKey:    "sleep_start",
				ActionName:   "入睡",
				Status:       "failed",
				QualityLevel: QualityLevelFailed,
				QualityFlags: []string{FlagEmptyFrame},
				FrameCount:   0,
				Error:        "frame decode failed",
			},
		},
		TotalActions:     3,
		SucceededActions: 2,
		FailedActions:    1,
		WarningActions:   1,
	}

	err := g.GenerateProcessingReport("task-report", 1, report)
	if err != nil {
		t.Fatalf("GenerateProcessingReport failed: %v", err)
	}

	p := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-report", "processed", "version-1", "processing-report.json")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read report failed: %v", err)
	}
	var got ProcessingReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal report failed: %v", err)
	}
	if got.TotalActions != 3 {
		t.Errorf("expected totalActions 3, got %d", got.TotalActions)
	}
	if got.SucceededActions != 2 {
		t.Errorf("expected succeededActions 2, got %d", got.SucceededActions)
	}
	if got.FailedActions != 1 {
		t.Errorf("expected failedActions 1, got %d", got.FailedActions)
	}
	if got.WarningActions != 1 {
		t.Errorf("expected warningActions 1, got %d", got.WarningActions)
	}
	if got.GeneratedAt == "" {
		t.Errorf("expected non-empty generatedAt")
	}
	if len(got.Actions) != 3 {
		t.Errorf("expected 3 actions, got %d", len(got.Actions))
	}
	if got.Actions[2].Error != "frame decode failed" {
		t.Errorf("expected error message, got %s", got.Actions[2].Error)
	}
}

func TestPreviewGenerateProcessingReportNil(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	err := g.GenerateProcessingReport("task-nil", 1, nil)
	if err == nil {
		t.Fatal("expected error for nil report")
	}
}

func TestPreviewGenerateProcessingReportPreservesGeneratedAt(t *testing.T) {
	dir := t.TempDir()
	g := NewPreviewGenerator(dir)

	report := &ProcessingReport{
		ProcessingTaskID:  "task-time",
		ProcessingVersion: 1,
		GeneratedAt:       "2026-07-24T10:00:00Z",
		Actions:           []ActionReport{},
		TotalActions:      0,
	}

	err := g.GenerateProcessingReport("task-time", 1, report)
	if err != nil {
		t.Fatalf("GenerateProcessingReport failed: %v", err)
	}

	p := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-time", "processed", "version-1", "processing-report.json")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read report failed: %v", err)
	}
	var got ProcessingReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal report failed: %v", err)
	}
	if got.GeneratedAt != "2026-07-24T10:00:00Z" {
		t.Errorf("expected preserved timestamp, got %s", got.GeneratedAt)
	}
}
