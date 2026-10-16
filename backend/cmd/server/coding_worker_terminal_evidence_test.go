package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/workbenchterminal"
)

type fakeTerminalReceipts struct {
	items        []workbenchterminal.TaskReceipt
	err          error
	calls        int
	owner        string
	conversation string
	task         string
}

func (f *fakeTerminalReceipts) ListRecorded(owner, conversation, task string) ([]workbenchterminal.TaskReceipt, error) {
	f.calls++
	f.owner, f.conversation, f.task = owner, conversation, task
	return f.items, f.err
}
func testTerminalActor() *auth.ActorContext {
	return &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, IsLocalTrusted: true, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: runtimeidentity.DeviceID("device-a"), SessionID: "login-a"}
}
func TestCodingTerminalRequiresTrustedLocalActor(t *testing.T) {
	cases := []struct {
		name  string
		actor *auth.ActorContext
		scope chat.SkillScope
	}{
		{"no-actor", nil, chat.SkillScope{SpaceID: "space-a", ConversationID: "conv-a", SessionID: "login-a"}},
		{"space-conflict", testTerminalActor(), chat.SkillScope{SpaceID: "space-other", ConversationID: "conv-a", SessionID: "login-a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeTerminalReceipts{}
			ctx := context.Background()
			if tc.actor != nil {
				ctx = auth.WithActor(ctx, tc.actor)
			}
			_, err := (codingTaskTerminalService{journal: f}).ReadCodingTask(ctx, tc.scope, "amitia://workspace/@task-a/", "task-a")
			if !errors.Is(err, errTerminalScopeUnavailable) || f.calls != 0 {
				t.Fatalf("leak on invalid scope %v %d", err, f.calls)
			}
		})
	}
}
func TestCodingTerminalFiltersWorkspaceAndDoesNotClaimSuccess(t *testing.T) {
	now := time.Now().UTC()
	zero := 0
	f := &fakeTerminalReceipts{items: []workbenchterminal.TaskReceipt{
		{SessionID: "c", TaskID: "task-a", ConversationID: "conv-a", WorkspaceURI: "amitia://workspace/@task-a/", StartedAt: now, State: "running"},
		{SessionID: "b", TaskID: "task-a", ConversationID: "conv-a", WorkspaceURI: "amitia://workspace/@task-a/", StartedAt: now, State: "exited", ExitCode: &zero},
		{SessionID: "a", TaskID: "task-a", ConversationID: "conv-a", WorkspaceURI: "amitia://workspace/@other/", StartedAt: now, State: "revoked"},
	}}
	ctx := auth.WithActor(context.Background(), testTerminalActor())
	got, err := (codingTaskTerminalService{journal: f}).ReadCodingTask(ctx, chat.SkillScope{SpaceID: "space-a", ConversationID: "conv-a", SessionID: "login-a"}, "amitia://workspace/@task-a/", "task-a")
	if err != nil || got.Total != 2 || got.UnverifiedActive != 1 || got.Exited != 1 || got.State != "recorded" || got.LatestState != "unverified" || got.VerificationKind != "terminal-lifecycle-metadata-not-test-proof" || f.owner != "space-a:device-a:login-a:conv-a" {
		t.Fatalf("incorrect scoped summary: %+v %v owner=%s", got, err, f.owner)
	}
}
func TestCodingTerminalPropagatesStoreFailures(t *testing.T) {
	f := &fakeTerminalReceipts{err: errors.New("cannot lock journal")}
	ctx := auth.WithActor(context.Background(), testTerminalActor())
	_, err := (codingTaskTerminalService{journal: f}).ReadCodingTask(ctx, chat.SkillScope{SpaceID: "space-a", ConversationID: "conv-a", SessionID: "login-a"}, "amitia://workspace/@task-a/", "task-a")
	if err == nil || f.calls != 1 {
		t.Fatalf("store errors hidden: %v", err)
	}
}

func TestCodingTerminalUsesAuthenticatedSessionNotChatSession(t *testing.T) {
	f := &fakeTerminalReceipts{}
	ctx := auth.WithActor(context.Background(), testTerminalActor())
	_, err := (codingTaskTerminalService{journal: f}).ReadCodingTask(ctx, chat.SkillScope{SpaceID: "space-a", ConversationID: "conv-a", SessionID: "different-chat-session"}, "amitia://workspace/@task-a/", "task-a")
	if err != nil || f.owner != "space-a:device-a:login-a:conv-a" {
		t.Fatalf("unexpected auth binding: %v %s", err, f.owner)
	}
}

func TestCodingTerminalDesktopSessionWithoutPhysicalDevice(t *testing.T) {
	actor := testTerminalActor()
	actor.DeviceID = runtimeidentity.DeviceID("")
	fake := &fakeTerminalReceipts{}
	ctx := auth.WithActor(context.Background(), actor)
	scope := chat.SkillScope{SpaceID: "space-a", ConversationID: "conv-a", SessionID: "any-chat-session"}
	_, err := (codingTaskTerminalService{journal: fake}).ReadCodingTask(ctx, scope, "amitia://workspace/@task-a/", "task-a")
	if err != nil || fake.owner != "space-a::login-a:conv-a" {
		t.Fatalf("desktop session missing device should match terminal API owner: %v %q", err, fake.owner)
	}
	actor.SessionID = ""
	fake.calls = 0
	_, err = (codingTaskTerminalService{journal: fake}).ReadCodingTask(ctx, scope, "amitia://workspace/@task-a/", "task-a")
	if !errors.Is(err, errTerminalScopeUnavailable) || fake.calls != 0 {
		t.Fatalf("sessionless parent bypassed history gate: %v", err)
	}
}
