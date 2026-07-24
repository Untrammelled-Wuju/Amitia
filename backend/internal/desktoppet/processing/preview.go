// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

type PreviewGenerator struct {
	dataDir string
}

func NewPreviewGenerator(dataDir string) *PreviewGenerator {
	return &PreviewGenerator{dataDir: dataDir}
}

func (g *PreviewGenerator) GenerateActionPreview(taskID string, processingVersion int, actionKey string, frames []image.Image) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("taskID is empty")
	}
	if actionKey == "" {
		return "", fmt.Errorf("actionKey is empty")
	}
	if processingVersion <= 0 {
		return "", fmt.Errorf("processingVersion must be positive")
	}
	if len(frames) == 0 {
		return "", fmt.Errorf("frames is empty")
	}

	midIndex := len(frames) / 2
	if midIndex >= len(frames) {
		midIndex = len(frames) - 1
	}
	previewFrame := frames[midIndex]
	if previewFrame == nil {
		return "", fmt.Errorf("preview frame is nil")
	}

	actionDir := filepath.Join(g.dataDir, "desktop-pets", "generation-tasks", taskID, "processed", fmt.Sprintf("version-%d", processingVersion), "actions", actionKey)
	if err := os.MkdirAll(actionDir, 0755); err != nil {
		return "", fmt.Errorf("create action dir failed: %w", err)
	}

	finalPath := filepath.Join(actionDir, "preview.png")
	tmpPath := filepath.Join(actionDir, ".preview.png.tmp")

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create tmp preview failed: %w", err)
	}
	if err := png.Encode(f, previewFrame); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("encode preview failed: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close tmp preview failed: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename preview failed: %w", err)
	}

	return finalPath, nil
}

func (g *PreviewGenerator) GeneratePackagePreview(taskID string, processingVersion int, defaultIdleFrames []image.Image) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("taskID is empty")
	}
	if processingVersion <= 0 {
		return "", fmt.Errorf("processingVersion must be positive")
	}
	if len(defaultIdleFrames) == 0 {
		return "", fmt.Errorf("default idle frames is empty")
	}

	midIndex := len(defaultIdleFrames) / 2
	if midIndex >= len(defaultIdleFrames) {
		midIndex = len(defaultIdleFrames) - 1
	}
	previewFrame := defaultIdleFrames[midIndex]
	if previewFrame == nil {
		return "", fmt.Errorf("preview frame is nil")
	}

	versionDir := filepath.Join(g.dataDir, "desktop-pets", "generation-tasks", taskID, "processed", fmt.Sprintf("version-%d", processingVersion))
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return "", fmt.Errorf("create version dir failed: %w", err)
	}

	finalPath := filepath.Join(versionDir, "package-preview.png")
	tmpPath := filepath.Join(versionDir, ".package-preview.png.tmp")

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create tmp package preview failed: %w", err)
	}
	if err := png.Encode(f, previewFrame); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("encode package preview failed: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close tmp package preview failed: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename package preview failed: %w", err)
	}

	return finalPath, nil
}

func (g *PreviewGenerator) GenerateProcessingReport(taskID string, processingVersion int, report *ProcessingReport) error {
	if taskID == "" {
		return fmt.Errorf("taskID is empty")
	}
	if processingVersion <= 0 {
		return fmt.Errorf("processingVersion must be positive")
	}
	if report == nil {
		return fmt.Errorf("report is nil")
	}

	if report.GeneratedAt == "" {
		report.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}

	versionDir := filepath.Join(g.dataDir, "desktop-pets", "generation-tasks", taskID, "processed", fmt.Sprintf("version-%d", processingVersion))
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("create version dir failed: %w", err)
	}

	finalPath := filepath.Join(versionDir, "processing-report.json")
	tmpPath := filepath.Join(versionDir, ".processing-report.json.tmp")

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report failed: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp report failed: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename report failed: %w", err)
	}

	return nil
}

type ProcessingReport struct {
	ProcessingTaskID  string         `json:"processingTaskId"`
	ProcessingVersion int            `json:"processingVersion"`
	GeneratedAt       string         `json:"generatedAt"`
	Actions           []ActionReport `json:"actions"`
	TotalActions      int            `json:"totalActions"`
	SucceededActions  int            `json:"succeededActions"`
	FailedActions     int            `json:"failedActions"`
	WarningActions    int            `json:"warningActions"`
}

type ActionReport struct {
	ActionKey    string   `json:"actionKey"`
	ActionName   string   `json:"actionName"`
	Status       string   `json:"status"`
	QualityLevel string   `json:"qualityLevel"`
	QualityFlags []string `json:"qualityFlags"`
	FrameCount   int      `json:"frameCount"`
	Error        string   `json:"error,omitempty"`
}
