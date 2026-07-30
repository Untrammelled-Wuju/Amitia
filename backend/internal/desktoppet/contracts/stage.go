// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package contracts

type Stage string

const (
	StageCreated   Stage = "created"
	StageQueued    Stage = "queued"
	StageCompleted Stage = "completed"
	StageFailed    Stage = "failed"
	StageCancelled Stage = "cancelled"
)

const (
	StagePreparing   Stage = "preparing"
	StageGenerating  Stage = "generating"
	StageFinalizing  Stage = "finalizing"
)

const (
	StagePreparingFrames Stage = "preparing_frames"
	StageSubmitting      Stage = "submitting"
	StagePolling         Stage = "polling"
	StageDownloading     Stage = "downloading"
	StagePersisting      Stage = "persisting"
)

const (
	StageValidatingSources   Stage = "validating_sources"
	StageDetectingSubject    Stage = "detecting_subject"
	StageRemovingBackground  Stage = "removing_background"
	StageNormalizingCanvas   Stage = "normalizing_canvas"
	StageAligningAnchors     Stage = "aligning_anchors"
	StagePersistingFrames    Stage = "persisting_frames"
	StageValidatingOutputs   Stage = "validating_outputs"
	StagePackaging           Stage = "packaging"
)

const (
	StageLoadingSources     Stage = "loading_sources"
	StageProcessingFrames   Stage = "processing_frames"
	StageValidatingAction   Stage = "validating_action"
)

const (
	StageLoading            Stage = "loading"
	StageBackgroundRemoval  Stage = "background_removal"
	StageNormalizing        Stage = "normalizing"
	StageAligning           Stage = "aligning"
)

const (
	StageCollectingAssets Stage = "collecting_assets"
	StageWritingManifest  Stage = "writing_manifest"
	StageCopyingAssets    Stage = "copying_assets"
	StageVerifying        Stage = "verifying"
	StageHashing          Stage = "hashing"
	StageCommitting       Stage = "committing"
)

func (s Stage) String() string { return string(s) }

func (s Stage) IsTerminalStage() bool {
	switch s {
	case StageCompleted, StageFailed, StageCancelled:
		return true
	default:
		return false
	}
}

func (s Stage) IsInitialStage() bool {
	switch s {
	case StageCreated, StageQueued:
		return true
	default:
		return false
	}
}

func (s Stage) IsActivityStage() bool {
	return !s.IsTerminalStage() && !s.IsInitialStage()
}

func StageForTerminalStatus(status LifecycleStatus) Stage {
	switch status {
	case StatusSucceeded, StatusPartiallySucceeded:
		return StageCompleted
	case StatusFailed:
		return StageFailed
	case StatusCancelled:
		return StageCancelled
	default:
		return ""
	}
}

func AllowedStagesFor(et EntityType) []Stage {
	switch et {
	case EntityGenerationTask:
		return []Stage{
			StageCreated, StageQueued, StagePreparing, StageGenerating, StageFinalizing,
			StageCompleted, StageFailed, StageCancelled,
		}
	case EntityGenerationAction:
		return []Stage{
			StageCreated, StageQueued, StagePreparingFrames, StageSubmitting, StagePolling,
			StagePersisting, StageFinalizing, StageCompleted, StageFailed, StageCancelled,
		}
	case EntityGenerationFrame:
		return []Stage{
			StageCreated, StageQueued, StageSubmitting, StagePolling, StageDownloading,
			StagePersisting, StageCompleted, StageFailed, StageCancelled,
		}
	case EntityProcessingTask:
		return []Stage{
			StageCreated, StageQueued, StageValidatingSources, StageDetectingSubject,
			StageRemovingBackground, StageNormalizingCanvas, StageAligningAnchors,
			StagePersistingFrames, StageValidatingOutputs, StagePackaging, StageFinalizing,
			StageCompleted, StageFailed, StageCancelled,
		}
	case EntityProcessingAction:
		return []Stage{
			StageCreated, StageQueued, StageLoadingSources, StageProcessingFrames,
			StagePersistingFrames, StageValidatingAction,
			StageCompleted, StageFailed, StageCancelled,
		}
	case EntityProcessedFrame:
		return []Stage{
			StageCreated, StageQueued, StageLoading, StageBackgroundRemoval,
			StageNormalizing, StageAligning, StagePersisting,
			StageCompleted, StageFailed, StageCancelled,
		}
	case EntityPackage:
		return []Stage{
			StageCreated, StageCollectingAssets, StageWritingManifest, StageCopyingAssets,
			StageVerifying, StageHashing, StageCommitting,
			StageCompleted, StageFailed, StageCancelled,
		}
	default:
		return nil
	}
}

func IsAllowedStageFor(et EntityType, s Stage) bool {
	for _, v := range AllowedStagesFor(et) {
		if v == s {
			return true
		}
	}
	return false
}

func ValidateStatusStageCombo(et EntityType, status LifecycleStatus, stage Stage) bool {
	switch status {
	case StatusPending:
		return stage == StageCreated
	case StatusQueued:
		return stage == StageQueued
	case StatusProcessing:
		return stage.IsActivityStage() && IsAllowedStageFor(et, stage)
	case StatusCancelling:
		return (stage.IsActivityStage() && IsAllowedStageFor(et, stage)) || stage == StageCancelled
	case StatusSucceeded, StatusPartiallySucceeded:
		return stage == StageCompleted
	case StatusFailed:
		return stage == StageFailed
	case StatusCancelled:
		return stage == StageCancelled
	default:
		return false
	}
}
