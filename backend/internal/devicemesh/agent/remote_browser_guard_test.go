package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestRemoteBrowserReadGuardScopedAndReadOnly(t *testing.T) {
	root := t.TempDir()
	store := NewCredentialStore(root)
	credential := &StoredCredential{CredentialID: "browser-credential", Credential: "token", SpaceID: "space", DeviceID: "device", RuntimeID: "runtime", ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.SaveCredential(credential); err != nil {
		t.Fatal(err)
	}
	guard := NewOwnedToolGuard(nil, root)
	invocation := protocol.RuntimeInvokePayload{InvocationID: "read-1", Handler: RemoteBrowserReadHandler, RuntimeType: "browser", SpaceID: "space", DeviceID: "device", RuntimeID: "runtime"}
	calls := 0
	success := func(context.Context) (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{Status: "success"}, nil
	}
	if _, err := guard(context.Background(), invocation, success); err != nil || calls != 1 {
		t.Fatalf("trusted browser read denied: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*protocol.RuntimeInvokePayload)
	}{
		{"wrong-device", func(x *protocol.RuntimeInvokePayload) { x.DeviceID = "other" }},
		{"wrong-space", func(x *protocol.RuntimeInvokePayload) { x.SpaceID = "other" }},
		{"wrong-runtime", func(x *protocol.RuntimeInvokePayload) { x.RuntimeID = "other" }},
		{"write-handler", func(x *protocol.RuntimeInvokePayload) { x.Handler = "workbench.browser.input" }},
		{"wrong-runtime-type", func(x *protocol.RuntimeInvokePayload) { x.RuntimeType = "workflow" }},
		{"authority-injection", func(x *protocol.RuntimeInvokePayload) { x.AuthorityCallID = "forged" }},
		{"missing-invocation", func(x *protocol.RuntimeInvokePayload) { x.InvocationID = "" }},
	}
	for _, tc := range cases {
		candidate := invocation
		tc.mutate(&candidate)
		if _, err := guard(context.Background(), candidate, success); !errors.Is(err, coordination.ErrWrongOwner) || calls != 1 {
			t.Fatalf("%s bypassed guard: %v", tc.name, err)
		}
	}
	if err := store.DeleteCredential(); err != nil {
		t.Fatal(err)
	}
	if _, err := guard(context.Background(), invocation, success); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("revoked credential accepted: %v", err)
	}
}

func TestRemoteBrowserReadGuardRejectsUnpairDuringRead(t *testing.T) {
	root := t.TempDir()
	store := NewCredentialStore(root)
	if err := store.SaveCredential(&StoredCredential{CredentialID: "one", Credential: "token", SpaceID: "space", DeviceID: "device", RuntimeID: "runtime", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	guard := NewOwnedToolGuard(nil, root)
	invocation := protocol.RuntimeInvokePayload{InvocationID: "read", Handler: RemoteBrowserReadHandler, RuntimeType: "browser", SpaceID: "space", DeviceID: "device", RuntimeID: "runtime"}
	_, err := guard(context.Background(), invocation, func(context.Context) (*protocol.RuntimeResultPayload, error) {
		if err := store.DeleteCredential(); err != nil {
			t.Fatal(err)
		}
		return &protocol.RuntimeResultPayload{Status: "success"}, nil
	})
	if !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("revoked during read returned data: %v", err)
	}
}
