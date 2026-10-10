package main

import (
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func TestMeshBusinessWiringRequiresDurableContextRoutes(t *testing.T) {
	f := newThreeCoreFixture(t, "readiness-core", newThreeCoreSchemas(t))
	s := f.services
	s.RuntimeProfile = runtimeprofile.ProfileLocal
	f.local.SetExecutionJournal(executionjournal.NewStore(s.DeviceMesh.DB))
	dispatcher, ok := f.dispatcher.(agent.RuntimeDispatcher)
	if !ok {
		t.Fatal("fixture lacks real runtime dispatcher")
	}
	s.DeviceMesh.SetDispatcher(dispatcher)
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("missing Source event route accepted")
	}
	registerSourceTaskHostEventDispatcher(f.dispatcher, s)
	if err := validateMeshBusinessWiring(s); err != nil {
		t.Fatalf("real owned ports and event route rejected: %v", err)
	}
	f.local.SetExecutionJournal(nil)
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("missing durable execution journal accepted")
	}
	f.local.SetExecutionJournal(executionjournal.NewStore(s.DeviceMesh.DB))
	f.local.SetDispatcher(agent.NewRuntimeDispatcher())
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("missing owned context routes accepted")
	}
}

func TestMeshBusinessWiringRejectsMissingCoreServices(t *testing.T) {
	if err := validateMeshBusinessWiring(nil); err == nil {
		t.Fatal("nil services accepted")
	}
	for _, profile := range []runtimeprofile.Profile{runtimeprofile.ProfileLocal, runtimeprofile.ProfileCloudCore} {
		if err := validateMeshBusinessWiring(&AppServices{RuntimeProfile: profile}); err == nil {
			t.Fatalf("missing Core services accepted for %s", profile)
		}
	}
	if err := validateMeshBusinessWiring(&AppServices{RuntimeProfile: runtimeprofile.ProfileDeviceAgent}); err != nil {
		t.Fatal(err)
	}
}
