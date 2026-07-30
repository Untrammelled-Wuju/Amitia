// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package taskstate

import (
	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

type AggregateDecision struct {
	Status       contracts.LifecycleStatus
	Stage        contracts.Stage
	Progress     int
	Reason       contracts.TransitionReason
	ErrorCode    string
	ErrorMessage string
	FailureStage contracts.Stage
}

type GenerationSnapshot struct {
	TaskStatus                  contracts.LifecycleStatus
	CancelRequested             bool
	HasActiveChildren           bool
	AllRequiredActionsSucceeded bool
	AllSucceededArtifactsValid  bool
	HasAtLeastOneCompleteAction bool
	AllowPartialResult          bool
	ActualProgress              int
}

type ProcessingSnapshot struct {
	TaskStatus                  contracts.LifecycleStatus
	CancelRequested             bool
	HasActiveChildren           bool
	AllActionsSucceeded         bool
	HasAtLeastOneActionSucceeded bool
	PackageExists               bool
	PackageReady                bool
	PackagePathValid            bool
	ManifestValid               bool
	HashValid                   bool
	IncludedActionsMatch        bool
	AllowPartialResult          bool
	ActualProgress              int
}

func AggregateGenerationTask(s GenerationSnapshot) AggregateDecision {
	if s.CancelRequested {
		if s.HasActiveChildren {
			return AggregateDecision{
				Status: contracts.StatusCancelling,
				Reason: contracts.ReasonGenerationTaskCancelRequested,
			}
		}
		if s.HasAtLeastOneCompleteAction && s.AllowPartialResult {
			return AggregateDecision{
				Status:   contracts.StatusPartiallySucceeded,
				Stage:    contracts.StageCompleted,
				Progress: 100,
				Reason:   contracts.ReasonGenerationTaskFinalizePartial,
			}
		}
		return AggregateDecision{
			Status:   contracts.StatusCancelled,
			Stage:    contracts.StageCancelled,
			Reason:   contracts.ReasonGenerationTaskCancelConverged,
			Progress: s.ActualProgress,
		}
	}
	if s.HasActiveChildren {
		return AggregateDecision{
			Status:   contracts.StatusProcessing,
			Progress: s.ActualProgress,
		}
	}
	if s.AllRequiredActionsSucceeded && s.AllSucceededArtifactsValid {
		return AggregateDecision{
			Status:   contracts.StatusSucceeded,
			Stage:    contracts.StageCompleted,
			Progress: 100,
			Reason:   contracts.ReasonGenerationTaskFinalizeSuccess,
		}
	}
	if s.HasAtLeastOneCompleteAction && s.AllowPartialResult {
		return AggregateDecision{
			Status:   contracts.StatusPartiallySucceeded,
			Stage:    contracts.StageCompleted,
			Progress: 100,
			Reason:   contracts.ReasonGenerationTaskFinalizePartial,
		}
	}
	return AggregateDecision{
		Status:       contracts.StatusFailed,
		Stage:        contracts.StageFailed,
		Progress:     s.ActualProgress,
		Reason:       contracts.ReasonGenerationTaskFinalizeFailure,
		FailureStage: contracts.StageGenerating,
	}
}

func AggregateProcessingTask(s ProcessingSnapshot) AggregateDecision {
	if s.CancelRequested {
		if s.HasActiveChildren {
			return AggregateDecision{
				Status: contracts.StatusCancelling,
				Reason: contracts.ReasonProcessingTaskCancelRequested,
			}
		}
		if s.HasAtLeastOneActionSucceeded && s.AllowPartialResult {
			return AggregateDecision{
				Status:   contracts.StatusPartiallySucceeded,
				Stage:    contracts.StageCompleted,
				Progress: 100,
				Reason:   contracts.ReasonProcessingTaskFinalizePartial,
			}
		}
		return AggregateDecision{
			Status:   contracts.StatusCancelled,
			Stage:    contracts.StageCancelled,
			Reason:   contracts.ReasonProcessingTaskCancelConverged,
			Progress: s.ActualProgress,
		}
	}
	if s.HasActiveChildren {
		return AggregateDecision{
			Status:   contracts.StatusProcessing,
			Progress: s.ActualProgress,
		}
	}
	if s.AllActionsSucceeded && s.PackageReady && s.PackagePathValid &&
		s.ManifestValid && s.HashValid && s.IncludedActionsMatch {
		return AggregateDecision{
			Status:   contracts.StatusSucceeded,
			Stage:    contracts.StageCompleted,
			Progress: 100,
			Reason:   contracts.ReasonProcessingTaskFinalizeSuccess,
		}
	}
	if s.HasAtLeastOneActionSucceeded && s.PackageReady &&
		s.PackagePathValid && s.ManifestValid && s.HashValid &&
		s.AllowPartialResult {
		return AggregateDecision{
			Status:   contracts.StatusPartiallySucceeded,
			Stage:    contracts.StageCompleted,
			Progress: 100,
			Reason:   contracts.ReasonProcessingTaskFinalizePartial,
		}
	}
	d := AggregateDecision{
		Status:   contracts.StatusFailed,
		Stage:    contracts.StageFailed,
		Progress: s.ActualProgress,
		Reason:   contracts.ReasonProcessingTaskFinalizeFailure,
	}
	if !s.PackageReady || !s.PackagePathValid || !s.ManifestValid || !s.HashValid {
		d.FailureStage = contracts.StagePackaging
		d.ErrorCode = "desktop_pet_package_missing"
		d.ErrorMessage = "package build failed or package artifacts invalid"
	}
	return d
}

func ShouldEnterCancelling(snap EntitySnapshot) bool {
	if snap.CancelRequested {
		return true
	}
	return false
}

func CanCancelImmediately(snap EntitySnapshot) bool {
	return snap.Status == contracts.StatusQueued || snap.Status == contracts.StatusPending
}
