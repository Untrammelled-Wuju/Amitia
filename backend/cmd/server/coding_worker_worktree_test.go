package main

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/gitworktree"
	"github.com/u-ai/backend/internal/interaction"
	workspacegit "github.com/u-ai/backend/internal/workspace/git"
)

type fakeCodingWorktreeCreator struct {
	calls   int
	request workspacegit.WorktreeCreateRequest
	entry   *gitworktree.Entry
	err     error
}

func (f *fakeCodingWorktreeCreator) CreateTaskWorktree(_ context.Context, req workspacegit.WorktreeCreateRequest) (*gitworktree.Entry, error) {
	f.calls++
	f.request = req
	return f.entry, f.err
}

func TestCodingWorktreeAllocatorAcceptsStandardWorkspaceRoot(t *testing.T) {
	creator := &fakeCodingWorktreeCreator{entry: &gitworktree.Entry{
		ID:           "0123456789abcdef0123456789abcdef",
		WorkspaceURI: "amitia://workspace/@child-uuid/",
		TaskBinding:  &gitworktree.TaskBinding{TaskID: "assignment-1", ConversationID: "conversation-1"},
	}}
	input := interaction.WorkerRunRequest{
		WorkerRole: "coding_worker", Source: "multi_agent", AssignmentID: "assignment-1",
		ConversationID: "conversation-1", WorkspaceID: "parent-uuid",
		WorkspaceRootURI: "amitia://workspace/@parent-uuid/", WorkspaceDeviceID: "device-a",
	}
	result, err := (codingWorkerWorktreeAllocator{git: creator, localDeviceID: "device-a"}).EnsureCodingWorkerWorktree(context.Background(), input)
	if err != nil || creator.calls != 1 {
		t.Fatalf("standard URI rejected: %v", err)
	}
	if creator.request.WorkspaceURI != input.WorkspaceRootURI ||
		creator.request.TaskID != input.AssignmentID || creator.request.ConversationID != input.ConversationID {
		t.Fatalf("provisioning identity mismatch: %+v", creator.request)
	}
	if result.WorkspaceID != "child-uuid" || result.WorkspaceKind != "local" || result.WorkspaceRootURI != creator.entry.WorkspaceURI {
		t.Fatalf("worker did not receive actual allocated workspace: %+v", result)
	}
}

func TestCodingWorktreeAllocatorRejectsUntrustedWorkspaceIdentity(t *testing.T) {
	for _, tc := range []struct{ name, root, id string }{
		{"mismatched ID", "amitia://workspace/@another/", "parent"},
		{"embedded path", "amitia://workspace/@parent/nested/", "parent"},
		{"remote path", "file:///tmp/project", "parent"},
		{"query injection", "amitia://workspace/@parent?root=other", "parent"},
		{"empty ID", "amitia://workspace/@/", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			creator := &fakeCodingWorktreeCreator{}
			_, err := (codingWorkerWorktreeAllocator{git: creator, localDeviceID: "device-a"}).EnsureCodingWorkerWorktree(context.Background(), interaction.WorkerRunRequest{
				WorkerRole: "coding_worker", Source: "multi_agent", AssignmentID: "assignment-1",
				ConversationID: "conversation-1", WorkspaceID: tc.id, WorkspaceRootURI: tc.root,
			})
			if err == nil || creator.calls != 0 {
				t.Fatalf("untrusted workspace provisioned: %v", err)
			}
		})
	}
}

func TestCodingWorktreeAllocatorRejectsInvalidBindingOrRegistration(t *testing.T) {
	req := interaction.WorkerRunRequest{WorkerRole: "coding_worker", Source: "multi_agent", AssignmentID: "task-a",
		ConversationID: "conv-a", WorkspaceID: "parent", WorkspaceRootURI: "amitia://workspace/@parent/"}
	for _, entry := range []*gitworktree.Entry{
		nil,
		{WorkspaceURI: "amitia://workspace/@child/"},
		{WorkspaceURI: "amitia://workspace/@child/nested/", TaskBinding: &gitworktree.TaskBinding{TaskID: "task-a", ConversationID: "conv-a"}},
		{WorkspaceURI: "amitia://workspace/@child/", TaskBinding: &gitworktree.TaskBinding{TaskID: "other", ConversationID: "conv-a"}},
		{WorkspaceURI: "amitia://workspace/@child/", TaskBinding: &gitworktree.TaskBinding{TaskID: "task-a", ConversationID: "other"}},
	} {
		creator := &fakeCodingWorktreeCreator{entry: entry}
		if _, err := (codingWorkerWorktreeAllocator{git: creator, localDeviceID: "device-a"}).EnsureCodingWorkerWorktree(context.Background(), req); err == nil {
			t.Fatalf("untrusted result accepted: %+v", entry)
		}
	}
	issue := errors.New("no disk capacity")
	creator := &fakeCodingWorktreeCreator{err: issue}
	if _, err := (codingWorkerWorktreeAllocator{git: creator, localDeviceID: "device-a"}).EnsureCodingWorkerWorktree(context.Background(), req); !errors.Is(err, issue) {
		t.Fatalf("allocator swallowed error: %v", err)
	}
}

func TestCodingWorktreeAllocatorRejectsCrossDeviceExecution(t *testing.T) {
	input := interaction.WorkerRunRequest{
		WorkerRole: "coding_worker", Source: "multi_agent", AssignmentID: "assignment-1",
		ConversationID: "conversation-1", WorkspaceID: "parent-uuid",
		WorkspaceRootURI: "amitia://workspace/@parent-uuid/", WorkspaceDeviceID: "device-b",
	}
	creator := &fakeCodingWorktreeCreator{}
	if _, err := (codingWorkerWorktreeAllocator{git: creator, localDeviceID: "device-a"}).EnsureCodingWorkerWorktree(context.Background(), input); err == nil || creator.calls > 0 {
		t.Fatal("worker tried to allocate a remote device workspace locally")
	}
	input.WorkspaceDeviceID = ""
	if _, err := (codingWorkerWorktreeAllocator{git: creator}).EnsureCodingWorkerWorktree(context.Background(), input); err == nil || creator.calls > 0 {
		t.Fatal("cloud core without a local device identity provisioned a worktree")
	}
}
