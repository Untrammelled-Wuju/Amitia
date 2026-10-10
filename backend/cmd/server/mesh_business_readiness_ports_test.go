package main

import (
	"testing"

	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func TestMeshBusinessWiringRejectsWrongActualRuntimePorts(t *testing.T) {
	f := newThreeCoreFixture(t, "readiness-ports", newThreeCoreSchemas(t))
	s := f.services
	s.RuntimeProfile = runtimeprofile.ProfileLocal
	f.local.SetExecutionJournal(executionjournal.NewStore(s.DeviceMesh.DB))
	s.DeviceMesh.SetDispatcher(f.dispatcher.(agent.RuntimeDispatcher))
	registerSourceTaskHostEventDispatcher(f.dispatcher, s)
	if err := validateMeshBusinessWiring(s); err != nil {
		t.Fatal(err)
	}
	originalRegistry := s.KernelContainer.DeviceRegistry
	s.KernelContainer.DeviceRegistry = nil
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("missing kernel authority registry accepted")
	}
	s.KernelContainer.DeviceRegistry = originalRegistry
	originalHandler := s.DeviceMesh.LocalHandler
	s.DeviceMesh.LocalHandler = nil
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("missing actual Source execution handler accepted")
	}
	s.DeviceMesh.LocalHandler = originalHandler
	s.DeviceMesh.CoreDataPort, s.DeviceMesh.LocalDeviceDataPort = s.DeviceMesh.LocalDeviceDataPort, s.DeviceMesh.CoreDataPort
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("swapped Core and Source owner ports accepted")
	}
	s.DeviceMesh.CoreDataPort, s.DeviceMesh.LocalDeviceDataPort = s.DeviceMesh.LocalDeviceDataPort, s.DeviceMesh.CoreDataPort
	original := s.OwnedBusiness
	s.OwnedBusiness = business.NewEngine(coordination.NewService(s.DeviceMesh.DB), s.DeviceMesh, f.model)
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("foreign coordinator accepted")
	}
	s.OwnedBusiness = original
	var absentModel *threeCoreModel
	s.OwnedBusiness = business.NewEngine(s.DeviceMesh.Coordination, s.DeviceMesh, absentModel)
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("typed-nil production model accepted")
	}
	s.OwnedBusiness = original
	originalPort := s.DeviceMesh.LocalDeviceDataPort
	var absentPort *devicemesh.Runtime
	s.DeviceMesh.LocalDeviceDataPort = absentPort
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("typed-nil Source role port accepted")
	}
	s.DeviceMesh.LocalDeviceDataPort = originalPort
	originalBootstrap := s.DeviceMesh.BootstrapSvc
	s.DeviceMesh.BootstrapSvc = nil
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("missing credential bootstrap accepted")
	}
	s.DeviceMesh.BootstrapSvc = originalBootstrap
	originalCredentials := s.DeviceMesh.CredentialSvc
	s.DeviceMesh.CredentialSvc = credential.NewService(credential.NewRepository(s.DeviceMesh.DB), 3600)
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("unsigned credential verifier accepted")
	}
	s.DeviceMesh.CredentialSvc = originalCredentials
	s.DeviceMesh.BootstrapSvc = bootstrap.NewService(bootstrap.NewRepository(s.DeviceMesh.DB), 3600)
	if err := validateMeshBusinessWiring(s); err == nil {
		t.Fatal("bootstrap without atomic trust/exchange accepted")
	}
	s.DeviceMesh.BootstrapSvc = originalBootstrap
	if err := validateMeshBusinessWiring(s); err != nil {
		t.Fatal(err)
	}
}
