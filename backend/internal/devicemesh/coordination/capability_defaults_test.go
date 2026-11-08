package coordination_test

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestCapabilityDefaultsSeparateCenterAndMemberAccess(t *testing.T) {
	_, svc := setup(t)
	if err := svc.InitializeCoreConsole(t.Context(), "space", "a", true); err != nil {
		t.Fatal(err)
	}

	if err := svc.RequireCapability(t.Context(), "space", "a", "b", "task.execute"); err != nil {
		t.Fatalf("center could not call exposed member capability: %v", err)
	}
	if err := svc.RequireCapability(t.Context(), "space", "b", "a", "ai.chat"); err != nil {
		t.Fatalf("member could not use center data or model: %v", err)
	}
	if err := svc.RequireCapability(t.Context(), "space", "b", "a", "task.execute"); !errors.Is(err, coordination.ErrCapabilityGrant) {
		t.Fatalf("member used center device capability without grant: %v", err)
	}
	if err := svc.RequireCapability(t.Context(), "space", "b", "c", "task.execute"); !errors.Is(err, coordination.ErrCapabilityGrant) {
		t.Fatalf("member used another member without grant: %v", err)
	}
}
