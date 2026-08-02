// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"github.com/u-ai/backend/internal/desktoppet"
)

const (
	ProgressValidatingSources = 5
	ProgressBackgroundRemoval = 30
	ProgressSubjectDetection  = 35
	ProgressScaling           = 40
	ProgressAnchor            = 45
	ProgressCanvas            = 50
	ProgressAlignment         = 55
	ProgressQuality           = 70
	ProgressLoop              = 75
	ProgressWriteFrames       = 80
	ProgressActionJSON        = 85
	ProgressPreview           = 90
	ProgressManifest          = 95
	ProgressPackage           = 100
)

const (
	StageValidatingSources = "validating_sources"
	StageBackgroundRemoval = "background_removal"
	StageSubjectDetection  = "subject_detection"
	StageScaling           = "scaling"
	StageAnchor            = "anchor"
	StageCanvas            = "canvas_normalization"
	StageAlignment         = "alignment"
	StageQuality           = "quality_check"
	StageLoop              = "loop_check"
	StageWriteFrames       = "write_frames"
	StageActionJSON        = "action_json"
	StagePreview           = "generating_previews"
	StageManifest          = "manifest"
	StagePackaging         = "packaging"
	StageCompleted         = "completed"
)

func (w *Worker) publishProgress(taskID string, progress int, stage string) {
	desktoppet.PublishTaskEvent(taskID, "processing.progress", map[string]interface{}{
		"progress": progress,
		"stage":    stage,
	})
}

func (w *Worker) publishActionEvent(taskID, actionKey, status string) {
	desktoppet.PublishTaskEvent(taskID, "processing.action", map[string]interface{}{
		"actionKey": actionKey,
		"status":    status,
	})
}

func (w *Worker) publishActionProgress(taskID, actionKey, stage string, progress int) {
	desktoppet.PublishTaskEvent(taskID, "processing.action.progress", map[string]interface{}{
		"actionKey": actionKey,
		"progress":  progress,
		"stage":     stage,
	})
}

func (w *Worker) publishCompleted(taskID, status string, succeeded, failed, total int) {
	desktoppet.PublishTaskEvent(taskID, "processing.completed", map[string]interface{}{
		"status":    status,
		"succeeded": succeeded,
		"failed":    failed,
		"total":     total,
	})
}
