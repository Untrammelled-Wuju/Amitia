package kernel

import (
	"encoding/json"
	"strings"
	"testing"

	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuildBeforePromptPayloadIncludesMessageAndSource(t *testing.T) {
	facade := &ToolFacade{}
	raw := facade.buildBeforePromptPayload(InvocationScope{
		SpaceID:        "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
		SessionID:      "session-1",
		Message:        "主动发送提示",
		Source:         "proactive",
		IsInternal:     true,
	})
	var payload struct {
		Context map[string]any `json:"context"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Context["message"] != "主动发送提示" {
		t.Fatalf("expected message in hook payload, got %#v", payload.Context["message"])
	}
	if payload.Context["source"] != "proactive" {
		t.Fatalf("expected proactive source, got %#v", payload.Context["source"])
	}
	if payload.Context["internal"] != true {
		t.Fatalf("expected internal flag, got %#v", payload.Context["internal"])
	}
}

func TestWorkspacePromptListsEveryProjectWorkspace(t *testing.T) {
	contribution, ok := workspacePromptContribution(InvocationScope{ExecContext: &coreexec.ExecutionContext{
		PrimaryWorkspaceID: "workspace-b",
		WorkspaceBindings: []coreexec.WorkspaceBinding{
			{ID: "workspace-a", Name: "A", Kind: "local", RootURI: "amitia://workspace/@workspace-a/"},
			{ID: "workspace-b", Name: "B", Kind: "local", RootURI: "amitia://workspace/@workspace-b/"},
		},
	}})
	if !ok {
		t.Fatal("multi workspace prompt contribution missing")
	}
	for _, expected := range []string{"workspace-a", "workspace-b", "Primary workspaceId=workspace-b", "simultaneously accessible workspaces"} {
		if !strings.Contains(contribution.Content, expected) {
			t.Fatalf("prompt missing %q: %s", expected, contribution.Content)
		}
	}
}

func TestWorkspaceScopeAllowsOnlyProjectWorkspaceRoots(t *testing.T) {
	execCtx := &coreexec.ExecutionContext{
		PrimaryWorkspaceID: "workspace-a",
		WorkspaceBindings: []coreexec.WorkspaceBinding{
			{ID: "workspace-a", RootURI: "amitia://workspace/@workspace-a/"},
			{ID: "workspace-b", RootURI: "amitia://workspace/@workspace-b/"},
		},
	}
	raw, err := scopeWorkspaceInvocationInput(capability.ToolInvocationContext{ExecContext: execCtx}, json.RawMessage(`{"workspaceId":"workspace-b","uri":"src/main.go"}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil || payload["workspaceId"] != "workspace-b" || payload["uri"] != "amitia://workspace/@workspace-b/src/main.go" {
		t.Fatalf("secondary workspace scope=%#v", payload)
	}
	if _, err := scopeWorkspaceInvocationInput(capability.ToolInvocationContext{ExecContext: execCtx}, json.RawMessage(`{"workspaceId":"workspace-c","uri":"src/main.go"}`)); err == nil {
		t.Fatal("workspace outside project was accepted")
	}
}
