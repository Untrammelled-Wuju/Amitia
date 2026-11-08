package coordination_test

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/capabilities"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func withoutCapability(id string) []string {
	result := make([]string, 0, len(capabilities.IDs()))
	for _, capability := range capabilities.IDs() {
		if capability != id {
			result = append(result, capability)
		}
	}
	return result
}

func TestCapabilityExposureDefaultsOpenAndClosesCenterAccess(t *testing.T) {
	_, svc := setup(t)
	if err := svc.InitializeCoreConsole(t.Context(), "space", "a", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.RequireCapability(t.Context(), "space", "a", "b", "task.execute"); err != nil {
		t.Fatalf("default exposure blocked member capability: %v", err)
	}
	catalog, err := svc.SetCapabilityExposure(t.Context(), "space", "b", 2, withoutCapability("task.execute"))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Revision != 2 {
		t.Fatalf("unexpected catalog revision: %+v", catalog)
	}
	for _, capability := range catalog.Capabilities {
		if capability == "task.execute" {
			t.Fatal("closed capability remained visible")
		}
	}
	if err := svc.RequireCapability(t.Context(), "space", "a", "b", "task.execute"); !errors.Is(err, coordination.ErrCapabilityGrant) {
		t.Fatalf("center used capability closed by target: %v", err)
	}
	if _, err := svc.SetCapabilityGrant(t.Context(), "space", "a", "b", "task.execute", 0, true); !errors.Is(err, coordination.ErrCapabilityGrant) {
		t.Fatalf("grant was created for closed capability: %v", err)
	}
}

func TestCapabilityClosureRevokesExistingGrantAndPublishesChange(t *testing.T) {
	_, svc := setup(t)
	if err := svc.InitializeCoreConsole(t.Context(), "space", "a", true); err != nil {
		t.Fatal(err)
	}
	grant, err := svc.SetCapabilityGrant(t.Context(), "space", "b", "a", "task.execute", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RequireCapability(t.Context(), "space", "b", "a", "task.execute"); err != nil {
		t.Fatal(err)
	}
	events, cancel := svc.WatchCapabilityCatalogs()
	defer cancel()
	if _, err := svc.SetCapabilityExposure(t.Context(), "space", "a", 2, withoutCapability("task.execute")); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event.DeviceID != "a" || event.Revision != 2 {
			t.Fatalf("unexpected capability event: %+v", event)
		}
	default:
		t.Fatal("capability change was not published")
	}
	if err := svc.RequireCapability(t.Context(), "space", "b", "a", "task.execute"); !errors.Is(err, coordination.ErrCapabilityGrant) {
		t.Fatalf("revoked grant remained active: %v", err)
	}
	grants, err := svc.CapabilityGrants(t.Context(), "space", "a")
	if err != nil {
		t.Fatal(err)
	}
	for _, current := range grants {
		if current.CallerID == "b" && current.Capability == grant.Capability {
			t.Fatalf("closed capability grant remained visible: %+v", current)
		}
	}
}

func TestCapabilityExposureRejectsStaleRevisionAndUnknownCapability(t *testing.T) {
	_, svc := setup(t)
	if _, err := svc.SetCapabilityExposure(t.Context(), "space", "b", 2, capabilities.IDs()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetCapabilityExposure(t.Context(), "space", "b", 1, capabilities.IDs()); !errors.Is(err, coordination.ErrRevision) {
		t.Fatalf("stale exposure revision accepted: %v", err)
	}
	if _, err := svc.SetCapabilityExposure(t.Context(), "space", "b", 3, []string{"unknown.execute"}); err == nil {
		t.Fatal("unknown capability accepted")
	}
}
