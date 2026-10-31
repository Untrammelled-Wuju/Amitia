package main

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/interaction"
	amitialog "log"
)

func (r *ownedMultiAgentWorkerRunner) settleWorker(runCtx context.Context, childID string, req interaction.WorkerRunRequest, response business.Response, runErr error) {
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, parentErr := r.verifyWorkerParent(checkCtx, req)
	_, workerErr := r.workerCancellationRequested(checkCtx, childID, req)
	checkCancel()
	if parentErr != nil || workerErr != nil {
		r.settleUnverifiedWorker(childID)
		return
	}
	record, found, readErr := r.tracker.Get(context.Background(), childID)
	if readErr != nil || !found || record == nil || record.IsTerminal() {
		return
	}
	switch classifyOwnedWorkerSettlement(runCtx.Err() != nil || !record.CancelRequestedAt.IsZero(), response, runErr) {
	case interaction.InteractionStatusCompleted:
		if _, err := r.tracker.Complete(context.Background(), childID, record.StatusVersion, response.Text); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	case interaction.InteractionStatusInterrupted:
		if _, err := r.tracker.TransitionCAS(context.Background(), childID, record.StatusVersion, interaction.InteractionStatusInterrupted); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	default:
		if runErr == nil {
			runErr = errors.New("owned worker result was not fully committed")
		}
		if _, err := r.tracker.Fail(context.Background(), childID, record.StatusVersion, "owned_worker_failed", runErr.Error()); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
}
