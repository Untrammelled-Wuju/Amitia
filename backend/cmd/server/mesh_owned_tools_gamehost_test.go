package main

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/gamemode"
	"gorm.io/gorm"
)

func TestOwnedExecutionScopeUsesActiveGameHostDevice(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := gamemode.NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Acquire(context.Background(), gamemode.AcquireInput{
		SpaceID:   "space-a",
		DeviceID:  "center-device",
		GameTitle: "Game",
	}); err != nil {
		t.Fatal(err)
	}
	runtime := &meshOwnedToolRuntime{services: &AppServices{GameMode: service}}
	invocation, err := runtime.executionScope(context.Background(), business.Inference{
		Scope: coordination.ExecutionScope{
			SpaceID:           "space-a",
			InitiatorDeviceID: "phone-a",
			TargetDeviceID:    "phone-a",
			CoreID:            "space-a",
			RoleID:            "role-a",
			ResourceOwnerID:   "space-a",
			RoleOwnerID:       "space-a",
			RequestID:         "request-a",
		},
		ConversationID: "conversation-a",
	}, "call-a")
	if err != nil {
		t.Fatal(err)
	}
	if invocation.ExecContext == nil || invocation.ExecContext.Metadata == nil {
		t.Fatal("execution context metadata is missing")
	}
	if got := invocation.ExecContext.Metadata["gameHostDeviceId"]; got != "center-device" {
		t.Fatalf("game host device = %v, want center-device", got)
	}
	if invocation.ExecContext.RuntimeTarget == nil || invocation.ExecContext.RuntimeTarget.DeviceID != "phone-a" {
		t.Fatalf("resource target changed: %+v", invocation.ExecContext.RuntimeTarget)
	}
}
