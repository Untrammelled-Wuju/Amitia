package interaction

import (
	"context"
	"errors"
	"strings"
)

const OwnedReplyFinishedPrefix = "owned-reply-finished/"
const OwnedReplyFailedPrefix = "owned-reply-failed/"

func OwnedReplySettled(resultRef string) bool {
	return strings.HasPrefix(resultRef, OwnedReplyFinishedPrefix) || strings.HasPrefix(resultRef, OwnedReplyFailedPrefix)
}

func FinalizeOwnedParent(ctx context.Context, tracker InteractionTracker, parentID string) error {
	for attempt := 0; attempt < 4; attempt++ {
		parent, found, err := tracker.Get(ctx, parentID)
		if err != nil {
			return err
		}
		if !found || parent == nil {
			return ErrInteractionNotFound
		}
		if parent.IsTerminal() || parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.OwnedTurn == nil || !OwnedReplySettled(parent.ResultRef) {
			return nil
		}
		task := parent.RecoveryDescriptor.MultiAgent
		if task == nil {
			return nil
		}
		switch task.Status {
		case "succeeded":
			if strings.HasPrefix(parent.ResultRef, OwnedReplyFailedPrefix) {
				_, err = tracker.Fail(ctx, parent.ID, parent.StatusVersion, "owned_turn_failed", "父回复未确认完成或保存；后台子任务结果仍需单独审核")
			} else {
				_, err = tracker.Complete(ctx, parent.ID, parent.StatusVersion, "multi-agent/"+task.CoordinationID+"/requires-review")
			}
		case "failed":
			_, err = tracker.Fail(ctx, parent.ID, parent.StatusVersion, "multi_agent_failed", "后台子任务执行失败，请查看任务历史")
		case "cancelled":
			_, err = tracker.TransitionCAS(ctx, parent.ID, parent.StatusVersion, InteractionStatusCancelled)
		default:
			return nil
		}
		if !errors.Is(err, ErrVersionConflict) && !errors.Is(err, ErrAlreadyTerminal) {
			return err
		}
	}
	return ErrVersionConflict
}
