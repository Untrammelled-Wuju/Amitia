package main

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/workspace"
)

func TestConversationCreatorAdapterUsesOwnerConversationTransaction(t *testing.T) {
	port := setupMeshLocalDataPort(t)
	database := port.services.KernelContainer.DeviceRegistry.Database()
	if _, err := port.services.KernelContainer.DeviceRegistry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: "device-a", TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	engine := business.NewEngine(coordination.NewService(database), port, nil)
	ctx := auth.WithActor(t.Context(), &auth.ActorContext{PrincipalType: auth.PrincipalTest, SpaceID: "core", DeviceID: "device-a", Permissions: auth.OwnerDevicePermissions()})
	ctx = kernel.WithHostAPIScope(ctx, kernel.HostAPIScopeContext{SpaceID: "core", CharacterID: "one"})
	creator := &conversationCreatorAdapter{engine: engine, coreID: "core"}
	raw, err := creator.CreateConversation(ctx, kernel.ConversationCreateRequest{
		SpaceID:     "core",
		CharacterID: "one",
		RequestID:   "create-owner-conversation",
		Title:       "Owner conversation",
		Channel:     "web",
	})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		ConversationID string `json:"conversationId"`
		Title          string `json:"title"`
		Source         string `json:"source"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document.ConversationID == "" || document.Title != "Owner conversation" || document.Source != "agent_tool" {
		t.Fatalf("unexpected created conversation: %s", raw)
	}
	deleteCtx := kernel.WithHostAPIScope(ctx, kernel.HostAPIScopeContext{SpaceID: "core", CharacterID: "one", ConversationID: document.ConversationID})
	deleted, err := creator.DeleteConversation(deleteCtx, kernel.ConversationDeleteRequest{
		SpaceID:        "core",
		CharacterID:    "one",
		ConversationID: document.ConversationID,
		RequestID:      "delete-owner-conversation",
	})
	if err != nil {
		t.Fatal(err)
	}
	var deletion struct {
		Deleted bool `json:"deleted"`
	}
	if err := json.Unmarshal(deleted, &deletion); err != nil || !deletion.Deleted {
		t.Fatalf("unexpected delete result: %s %v", deleted, err)
	}
}

func TestConversationCreatorAdapterUsesDeviceProjectInCoordinatedMode(t *testing.T) {
	port := setupMeshLocalDataPort(t)
	ctx := auth.WithActor(t.Context(), &auth.ActorContext{PrincipalType: auth.PrincipalTest, SpaceID: "core", DeviceID: "device-a", Permissions: auth.OwnerDevicePermissions()})
	registry := port.services.KernelContainer.DeviceRegistry
	if _, err := registry.EnsureDevice(ctx, host_registry.DeviceRecord{SpaceID: "core", DeviceID: "device-a", TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	service := coordination.NewService(registry.Database())
	policy, err := service.Get(ctx, "core", "device-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.InitializeCoordinated(ctx, "core", "device-a", policy.ModeRevision, "one"); err != nil {
		t.Fatal(err)
	}
	engine := business.NewEngine(service, port, nil)
	request := business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", TargetDeviceID: "device-a", ResourceOwnerID: "device-a", RoleOwnerID: "core", RoleID: "one"}
	view, err := engine.Query(ctx, request, coordination.DataQuery{Management: true, ResourceKind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &view.Scope
	request.RequestID = "project-for-bridge"
	project, err := engine.CreateProject(ctx, request, business.CreateProjectInput{
		Title: "Bridge project", PrimaryWorkspaceID: "mount-a",
		Workspaces: []business.ProjectWorkspace{{ID: "mount-a", DeviceID: "device-a", Name: "Local", Kind: string(workspace.WorkspaceKindLocal), RootURI: workspace.MountURI("mount-a")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx = kernel.WithHostAPIScope(ctx, kernel.HostAPIScopeContext{SpaceID: "core", CharacterID: "one"})
	creator := &conversationCreatorAdapter{engine: engine, coreID: "core"}
	raw, err := creator.CreateConversation(ctx, kernel.ConversationCreateRequest{
		SpaceID: "core", CharacterID: "one", RequestID: "create-project-session", ProjectID: project.Project.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		ConversationID string `json:"conversationId"`
	}
	if err := json.Unmarshal(raw, &document); err != nil || document.ConversationID == "" {
		t.Fatalf("invalid conversation: %s, %v", raw, err)
	}
	resolved, err := creator.ResolveConversationWorkspace(ctx, document.ConversationID)
	if err != nil || resolved.ProjectID != project.Project.ID || resolved.WorkspaceID != "mount-a" {
		t.Fatalf("device project workspace missing: %+v, %v", resolved, err)
	}
}
