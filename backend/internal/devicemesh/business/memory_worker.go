package business

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type forwardedMemoryContextKey struct{}

func (e *Engine) ResumeMemory(ctx context.Context, job coordination.MemoryJobLocation) (Response, error) {
	unlock := e.lock(job.SpaceID + "\x00" + job.TargetID + "\x00" + job.ConversationID)
	defer unlock()
	ctx, scope, finish, err := e.coordination.Begin(ctx, job.SpaceID, job.DeviceID, job.TargetID, job.SpaceID, job.RoleID, job.RequestID)
	if err != nil {
		return Response{}, err
	}
	defer finish()
	if scope.ResourceOwnerID != job.OwnerID || scope.ModeRevision != job.ModeRevision || scope.ProviderEpoch != job.ProviderEpoch {
		return Response{}, coordination.ErrScopeExpired
	}
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return Response{}, err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return Response{}, err
	}
	role, err := coordination.ResolveRole(job.RoleID, roles)
	if err != nil {
		return Response{}, err
	}
	scope.RoleRevision = role.Revision
	ctx = coordination.WithScope(ctx, scope)
	snapshot, err := e.data.Snapshot(ctx, scope, coordination.DataQuery{RequestID: job.RequestID, ConversationID: job.ConversationID, Limit: 128})
	if err != nil {
		return Response{}, err
	}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		return Response{}, err
	}
	var saved checkpoint
	var revision int64
	var message string
	for _, resource := range snapshot.Resources {
		if resource.Kind == "checkpoint" && resource.ID == "turn/"+job.RequestID {
			if err := json.Unmarshal(resource.Body, &saved); err != nil {
				return Response{}, err
			}
			revision = resource.Revision
		}
		if resource.Kind == "message" && resource.ID == job.RequestID+"/user" {
			var input struct {
				Transcription              string `json:"transcription"`
				TranscriptionSourceContent string `json:"transcriptionSourceContent"`
				Content                    string `json:"content"`
				ConversationID             string `json:"conversationId"`
			}
			if err := json.Unmarshal(resource.Body, &input); err != nil {
				return Response{}, err
			}
			if input.ConversationID != job.ConversationID {
				return Response{}, coordination.ErrWrongOwner
			}
			message = input.Content
			if input.Transcription != "" && input.Content == input.TranscriptionSourceContent {
				message = input.Transcription
			}
		}
	}
	if saved.Status != "completed" || saved.Response == nil || saved.Hash != job.Hash || revision < 2 || message == "" {
		return Response{}, ErrUncertainExecution
	}
	if saved.Response.MemoryStatus == "saved" {
		return *saved.Response, e.coordination.FinishMemoryJob(ctx, saved.Response.Scope)
	}
	inference := Inference{Scope: saved.Response.Scope, ConversationID: job.ConversationID, Snapshot: snapshot, Message: message, Context: saved.Context}
	return e.completeMemory(coordination.WithScope(ctx, inference.Scope), inference, saved.Hash, *saved.Response, revision)
}

func (e *Engine) RunMemoryWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobs, err := e.coordination.PendingMemoryJobs(ctx)
			if err != nil {
				continue
			}
			for _, job := range jobs {
				jobContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
				response, err := e.ResumeMemory(jobContext, job)
				cancel()
				if ctx.Err() != nil {
					return
				}
				if err != nil || response.MemoryStatus != "saved" {
					paused := response.MemoryStatus == "paused" || errors.Is(err, coordination.ErrScopeExpired) || errors.Is(err, coordination.ErrRoleRequired) || errors.Is(err, coordination.ErrCapabilityGrant) || errors.Is(err, coordination.ErrResourceVersion)
					_ = e.coordination.RetryMemoryJob(ctx, job, paused)
				}
			}
		}
	}
}
