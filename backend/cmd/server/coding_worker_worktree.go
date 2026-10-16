package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/gitworktree"
	"github.com/u-ai/backend/internal/interaction"
	workspacegit "github.com/u-ai/backend/internal/workspace/git"
)

type codingTaskWorktreeCreator interface {
	CreateTaskWorktree(context.Context, workspacegit.WorktreeCreateRequest) (*gitworktree.Entry, error)
}

type codingWorkerWorktreeAllocator struct {
	git           codingTaskWorktreeCreator
	localDeviceID string
	mesh          *codingWorkerMeshAllocator
	services      *AppServices
}

func (a codingWorkerWorktreeAllocator) EnsureCodingWorkerWorktree(ctx context.Context, req interaction.WorkerRunRequest) (interaction.CodingWorkerWorkspace, error) {
	if a.mesh == nil && a.services != nil {
		a.mesh = newCodingWorkerWorktreeAllocator(func() *AppServices { return a.services }, a.git, a.localDeviceID).mesh
	}
	if req.WorkspaceDeviceID != "" && req.WorkspaceDeviceID != a.localDeviceID && a.mesh != nil {
		return a.mesh.EnsureCodingWorkerWorktree(ctx, req)
	}
	if a.git == nil || a.localDeviceID == "" || req.WorkerRole != "coding_worker" || req.Source != "multi_agent" ||
		(req.WorkspaceDeviceID != "" && req.WorkspaceDeviceID != a.localDeviceID) {
		return interaction.CodingWorkerWorkspace{}, fmt.Errorf("coding worker worktree service unavailable or unauthorized")
	}
	const prefix = "amitia://workspace/@"
	if !strings.HasPrefix(req.WorkspaceRootURI, prefix) {
		return interaction.CodingWorkerWorkspace{}, fmt.Errorf("coding worker source must be an authorized local workspace mount")
	}
	mountID := strings.TrimSuffix(strings.TrimPrefix(req.WorkspaceRootURI, prefix), "/")
	if mountID == "" || strings.ContainsAny(mountID, "/?#\\") || mountID != req.WorkspaceID {
		return interaction.CodingWorkerWorkspace{}, fmt.Errorf("coding worker workspace mount identity mismatch")
	}
	entry, err := a.git.CreateTaskWorktree(ctx, workspacegit.WorktreeCreateRequest{
		WorkspaceURI:   req.WorkspaceRootURI,
		TaskID:         req.AssignmentID,
		ConversationID: req.ConversationID,
		Name:           "Coding Agent " + req.AssignmentID,
	})
	if err != nil {
		return interaction.CodingWorkerWorkspace{}, err
	}
	if entry == nil || !strings.HasPrefix(entry.WorkspaceURI, prefix) {
		return interaction.CodingWorkerWorkspace{}, fmt.Errorf("coding worktree registration returned invalid workspace")
	}
	id := strings.TrimSuffix(strings.TrimPrefix(entry.WorkspaceURI, prefix), "/")
	if id == "" || strings.ContainsAny(id, "/?#\\") || entry.TaskBinding == nil ||
		entry.TaskBinding.TaskID != req.AssignmentID || entry.TaskBinding.ConversationID != req.ConversationID {
		return interaction.CodingWorkerWorkspace{}, fmt.Errorf("coding worktree registration identity mismatch")
	}
	return interaction.CodingWorkerWorkspace{
		WorkspaceID:      id,
		WorkspaceRootURI: entry.WorkspaceURI,
		WorkspaceName:    "Coding Agent " + req.AssignmentID,
		WorkspaceKind:    "local",
	}, nil
}
