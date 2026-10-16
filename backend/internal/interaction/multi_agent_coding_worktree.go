package interaction

import (
	"context"
	"fmt"
	"strings"
)

type CodingWorkerWorkspace struct {
	WorkspaceID      string
	WorkspaceRootURI string
	WorkspaceName    string
	WorkspaceKind    string
}

type CodingWorkerWorktreeAllocator interface {
	EnsureCodingWorkerWorktree(context.Context, WorkerRunRequest) (CodingWorkerWorkspace, error)
}

type codingWorktreeWorkerRunner struct {
	worker    AgentWorkerRunner
	allocator CodingWorkerWorktreeAllocator
}

func NewCodingWorktreeWorkerRunner(worker AgentWorkerRunner, allocator CodingWorkerWorktreeAllocator) AgentWorkerRunner {
	return &codingWorktreeWorkerRunner{worker: worker, allocator: allocator}
}

func (r *codingWorktreeWorkerRunner) WorkerInteractionID(req WorkerRunRequest) string {
	if r == nil || r.worker == nil {
		return ""
	}
	if worker, ok := r.worker.(interface{ WorkerInteractionID(WorkerRunRequest) string }); ok {
		return worker.WorkerInteractionID(req)
	}
	return ""
}

func (r *codingWorktreeWorkerRunner) WorkerIsActive(id string) bool {
	if r == nil || r.worker == nil {
		return false
	}
	if worker, ok := r.worker.(interface{ WorkerIsActive(string) bool }); ok {
		return worker.WorkerIsActive(id)
	}
	return false
}

func (r *codingWorktreeWorkerRunner) CancelWorker(ctx context.Context, id string) error {
	if r == nil || r.worker == nil {
		return nil
	}
	if worker, ok := r.worker.(interface {
		CancelWorker(context.Context, string) error
	}); ok {
		return worker.CancelWorker(ctx, id)
	}
	return nil
}

func (r *codingWorktreeWorkerRunner) Shutdown(ctx context.Context) error {
	if r == nil || r.worker == nil {
		return nil
	}
	if worker, ok := r.worker.(interface{ Shutdown(context.Context) error }); ok {
		return worker.Shutdown(ctx)
	}
	return nil
}

func (r *codingWorktreeWorkerRunner) StartWorker(ctx context.Context, req WorkerRunRequest) (string, error) {
	if r == nil || r.worker == nil {
		return "", fmt.Errorf("multi_agent: no underlying worker runner")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if req.WorkerRole != "coding_worker" {
		return r.worker.StartWorker(ctx, req)
	}
	if req.Source != "multi_agent" || strings.TrimSpace(req.AssignmentID) == "" ||
		strings.TrimSpace(req.ConversationID) == "" || strings.TrimSpace(req.WorkspaceID) == "" ||
		strings.TrimSpace(req.WorkspaceRootURI) == "" || r.allocator == nil ||
		!validCodingMountID(req.WorkspaceID) || !validCodingMountURI(req.WorkspaceRootURI, req.WorkspaceID) {
		return "", fmt.Errorf("multi_agent: coding worker requires an authorized worktree allocator and source workspace")
	}
	workspace, err := r.allocator.EnsureCodingWorkerWorktree(ctx, req)
	if err != nil {
		return "", fmt.Errorf("multi_agent: coding worktree allocation: %w", err)
	}
	if !validIsolatedCodingWorkspace(req, workspace) {
		return "", fmt.Errorf("multi_agent: coding worktree allocator returned an invalid or non-isolated local workspace")
	}
	req.WorkspaceID = workspace.WorkspaceID
	req.WorkspaceRootURI = workspace.WorkspaceRootURI
	req.WorkspaceName = workspace.WorkspaceName
	req.WorkspaceKind = workspace.WorkspaceKind
	return r.worker.StartWorker(ctx, req)
}

func validIsolatedCodingWorkspace(req WorkerRunRequest, workspace CodingWorkerWorkspace) bool {
	const prefix = "amitia://workspace/@"
	if workspace.WorkspaceKind != "local" || !strings.HasPrefix(req.WorkspaceRootURI, prefix) ||
		!strings.HasPrefix(workspace.WorkspaceRootURI, prefix) {
		return false
	}
	sourceID := strings.TrimSuffix(strings.TrimPrefix(req.WorkspaceRootURI, prefix), "/")
	mountID := workspace.WorkspaceID
	if sourceID != req.WorkspaceID || !validCodingMountID(sourceID) || !validCodingMountID(mountID) ||
		sourceID == mountID {
		return false
	}
	return validCodingMountURI(req.WorkspaceRootURI, sourceID) && validCodingMountURI(workspace.WorkspaceRootURI, mountID)
}

func validCodingMountURI(uri, id string) bool {
	return uri == "amitia://workspace/@"+id || uri == "amitia://workspace/@"+id+"/"
}

func validCodingMountID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' ||
			ch >= '0' && ch <= '9' || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}
