package interaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type BackgroundTaskReader interface {
	GetTaskRun(ctx context.Context, taskRunID string) (*task_runtime.TaskRun, error)
	GetTaskResult(ctx context.Context, taskRunID string) (*task_runtime.TaskRunResult, error)
}

type BackgroundTaskCoordinator struct {
	tracker     InteractionTracker
	recovery    *RecoveryDescriptorService
	tasks       BackgroundTaskReader
	now         func() time.Time
}

func (c *BackgroundTaskCoordinator) Resume(ctx context.Context, taskRunID string) error {
	run, err := c.tasks.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return fmt.Errorf("resume: load task run: %w", err)
	}
	if run == nil {
		return nil
	}
	if run.Status.IsTerminal() {
		return c.HandleTerminalTask(ctx, taskRunID)
	}
	return nil
}

func (c *BackgroundTaskCoordinator) SignalExpiration(ctx context.Context, taskRunID string) error {
	run, err := c.tasks.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return fmt.Errorf("expiration: load task run: %w", err)
	}
	if run == nil {
		return nil
	}
	if !run.Status.IsTerminal() {
		return nil
	}
	return c.HandleTerminalTask(ctx, taskRunID)
}

func (c *BackgroundTaskCoordinator) ResumeBackgroundTask(ctx context.Context, taskRunID string) error {
	return c.Resume(ctx, taskRunID)
}

func (c *BackgroundTaskCoordinator) SignalBackgroundExpiration(ctx context.Context, taskRunID string) error {
	return c.SignalExpiration(ctx, taskRunID)
}

func NewBackgroundTaskCoordinator(
	tracker InteractionTracker,
	recovery *RecoveryDescriptorService,
	tasks BackgroundTaskReader,
) *BackgroundTaskCoordinator {
	return &BackgroundTaskCoordinator{
		tracker:  tracker,
		recovery: recovery,
		tasks:    tasks,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (c *BackgroundTaskCoordinator) ReconcilePending(ctx context.Context) error {
	return c.tracker.Range(ctx, func(record *InteractionRecord) bool {
		if record.RecoveryDescriptor == nil {
			return true
		}
		desc := record.RecoveryDescriptor
		if desc.Task == nil || desc.Task.TaskRunID == "" {
			return true
		}
		if desc.Task.Status.IsTerminal() {
			return true
		}
		run, err := c.tasks.GetTaskRun(ctx, desc.Task.TaskRunID)
		if err != nil || run == nil {
			return true
		}
		if !run.Status.IsTerminal() {
			return true
		}
		_ = c.HandleTerminalTask(ctx, run.TaskRunID)
		return true
	})
}

func (c *BackgroundTaskCoordinator) HandleTerminalTask(ctx context.Context, taskRunID string) error {
	run, err := c.tasks.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return fmt.Errorf("load task run: %w", err)
	}
	if run == nil {
		return nil
	}
	if !run.Status.IsTerminal() {
		return nil
	}

	descriptor, originalInteraction, ok := c.findOwningDescriptor(ctx, taskRunID)
	if !ok || descriptor == nil {
		return nil
	}

	if !c.lineageMatches(run, originalInteraction) {
		return nil
	}

	outcome := mapRunStatusToOutcome(run.Status)
	obsID := decision.BuildTaskTerminalObservationID(descriptor.Action.ActionID, run.TaskRunID, run.Generation)

	obs := &decision.Observation{
		Version:          decision.ObservationVersionV1,
		ID:               obsID,
		PlanID:           descriptor.Plan.PlanID,
		ActionID:         descriptor.Action.ActionID,
		InteractionID:    descriptor.Interaction.InteractionID,
		UserID:           descriptor.Scope.UserID,
		CharacterID:      descriptor.Scope.CharacterID,
		ConversationID:   descriptor.Scope.ConversationID,
		GoalIDs:          extractGoalIDs(descriptor.Goals),
		GoalRefs:         copyRecoveryGoalRefs(descriptor.Goals),
		Kind:             decision.ObservationKindTaskResult,
		TargetKind:       decision.ObservationTargetTask,
		Outcome:          outcome,
		TaskRunID:        run.TaskRunID,
		TaskDefinitionID: run.TaskDefinitionID,
		TaskGeneration:   run.Generation,
		ObservedAt:       c.now(),
	}

	if run.Status == task_runtime.RunStatusSucceeded {
		_ = obs
	}

	return c.persistTerminalObservation(ctx, descriptor, obs)
}

func (c *BackgroundTaskCoordinator) findOwningDescriptor(ctx context.Context, taskRunID string) (*RecoveryDescriptor, *InteractionRecord, bool) {
	var found *RecoveryDescriptor
	var interaction *InteractionRecord
	_ = c.tracker.Range(ctx, func(record *InteractionRecord) bool {
		if record.RecoveryDescriptor == nil {
			return true
		}
		desc := record.RecoveryDescriptor
		if desc.Task != nil && desc.Task.TaskRunID == taskRunID {
			found = desc
			interaction = record
			return false
		}
		return true
	})
	return found, interaction, found != nil
}

func (c *BackgroundTaskCoordinator) lineageMatches(run *task_runtime.TaskRun, originalInteraction *InteractionRecord) bool {
	if originalInteraction == nil {
		return true
	}
	if run.CorrelationID != "" && run.CorrelationID != originalInteraction.ID {
		return false
	}
	return true
}

func (c *BackgroundTaskCoordinator) persistTerminalObservation(ctx context.Context, descriptor *RecoveryDescriptor, obs *decision.Observation) error {
	if descriptor == nil {
		return nil
	}
	updated := *descriptor
	updated.Observation = &RecoveryObservationRef{
		ObservationID: obs.ID,
		Outcome:       obs.Outcome,
	}
	result, err := c.tracker.UpdateMetadata(ctx, updated.Interaction.InteractionID, InteractionMetadataUpdate{
		RecoveryDescriptor: &updated,
	})
	if err != nil {
		return err
	}
	_ = result
	return nil
}

func mapRunStatusToOutcome(status task_runtime.TaskRunStatus) decision.ObservationOutcome {
	switch status {
	case task_runtime.RunStatusSucceeded:
		return decision.ObservationOutcomeSucceeded
	case task_runtime.RunStatusFailed:
		return decision.ObservationOutcomeFailed
	case task_runtime.RunStatusCancelled:
		return decision.ObservationOutcomeCancelled
	case task_runtime.RunStatusTimedOut:
		return decision.ObservationOutcomeTimedOut
	default:
		return decision.ObservationOutcomeFailed
	}
}

func extractGoalIDs(refs []RecoveryGoalRef) []string {
	if len(refs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(refs))
	for _, r := range refs {
		ids = append(ids, r.GoalID)
	}
	return ids
}

func copyRecoveryGoalRefs(refs []RecoveryGoalRef) []decision.GoalRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]decision.GoalRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, decision.GoalRef{
			ID:       r.GoalID,
			Revision: r.Revision,
		})
	}
	return out
}

func BuildBackgroundTriggerID(taskRunID string, generation int64) string {
	h := sha256.New()
	h.Write([]byte("background-task"))
	h.Write([]byte{0x00})
	h.Write([]byte(taskRunID))
	h.Write([]byte{0x00})
	h.Write([]byte(fmt.Sprintf("%d", generation)))
	h.Write([]byte(":terminal"))
	return "bgtrigger:" + hex.EncodeToString(h.Sum(nil))[:32]
}
