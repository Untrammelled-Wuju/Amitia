package business

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ConversationWorkspace struct {
	ProjectID   string
	WorkspaceID string
	RootURI     string
	ReadOnly    bool
}

func (e *Engine) ResolveConversationWorkspace(ctx context.Context, request Request, conversationID string) (ConversationWorkspace, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ConversationWorkspace{}, errors.New("conversationId is required")
	}
	result, err := e.ManageChat(ctx, request, coordination.ChatManagementRequest{
		Operation:      "conversation/read",
		ConversationID: conversationID,
		RequestID:      request.RequestID,
	})
	if err != nil {
		return ConversationWorkspace{}, err
	}
	projectID := chatDocumentString(result, "projectId")
	resolved := ConversationWorkspace{
		ProjectID:   projectID,
		WorkspaceID: chatDocumentString(result, "workspaceId"),
		RootURI:     chatDocumentString(result, "workspaceRootUri"),
	}
	if resolved.RootURI != "" {
		return resolved, nil
	}
	if projectID == "" {
		if resolved.WorkspaceID == "" {
			return ConversationWorkspace{}, errors.New("conversation is not bound to a workspace")
		}
		return resolved, nil
	}
	scope, ok := result["executionScope"].(coordination.ExecutionScope)
	if !ok {
		return ConversationWorkspace{}, errors.New("conversation execution scope is unavailable")
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return ConversationWorkspace{}, errors.New("data source does not support project workspaces")
	}
	var project Project
	err = e.WithExecutionScope(ctx, scope, func(current context.Context) error {
		resource, err := port.Resource(current, scope, "project", projectID)
		if err != nil {
			return err
		}
		if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID {
			return coordination.ErrWrongOwner
		}
		if err := json.Unmarshal(resource.Body, &project); err != nil {
			return err
		}
		if project.ID != projectID {
			return errors.New("project id does not match conversation project")
		}
		return nil
	})
	if err != nil {
		return ConversationWorkspace{}, err
	}
	workspaces, primaryID, err := normalizeProjectWorkspaces(project.Workspaces, project.PrimaryWorkspaceID)
	if err != nil {
		return ConversationWorkspace{}, err
	}
	if !projectWorkspacesBelongToDevice(workspaces, scope.TargetDeviceID) {
		return ConversationWorkspace{}, errors.New("project workspaces do not belong to the execution device")
	}
	for _, candidate := range workspaces {
		if candidate.ID != primaryID {
			continue
		}
		resolved.WorkspaceID = candidate.ID
		resolved.RootURI = candidate.RootURI
		resolved.ReadOnly = candidate.ReadOnly
		return resolved, nil
	}
	return ConversationWorkspace{}, errors.New("project primary workspace is unavailable")
}

func chatDocumentString(document map[string]any, key string) string {
	value, _ := document[key].(string)
	return strings.TrimSpace(value)
}
