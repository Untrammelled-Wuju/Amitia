package business

import (
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type readinessDataOnly struct{ coordination.DataPort }

func TestOwnedRuntimeWiringRejectsForeignAuthorityAndMissingReplayPort(t *testing.T) {
	e, db, service, _ := engineHarness(t)
	data := e.data.(testDataPort)
	e.data = &data
	if err := e.ValidateRuntimeWiring(service, &data); err != nil {
		t.Fatal(err)
	}
	if err := e.ValidateRuntimeWiring(coordination.NewService(db), &data); err == nil {
		t.Fatal("foreign cancellation authority accepted")
	}
	other := data
	if err := e.ValidateRuntimeWiring(service, &other); err == nil {
		t.Fatal("foreign Runtime data route accepted")
	}
	missingReplay := &readinessDataOnly{DataPort: &data}
	e.data = missingReplay
	if err := e.ValidateRuntimeWiring(service, missingReplay); err == nil {
		t.Fatal("data without original resource replay accepted")
	}
}

func TestOwnedRuntimeWiringRejectsTypedNilAndUninitializedExecution(t *testing.T) {
	e, _, service, model := engineHarness(t)
	data := e.data.(testDataPort)
	e.data = &data
	var absentModel *testModel
	e.model = absentModel
	if err := e.ValidateRuntimeWiring(service, &data); err == nil {
		t.Fatal("typed-nil model accepted")
	}
	e.model = model
	var absentData *testDataPort
	e.data = absentData
	if err := e.ValidateRuntimeWiring(service, absentData); err == nil {
		t.Fatal("typed-nil data port accepted")
	}
	e.data = &data
	e.active = nil
	if err := e.ValidateRuntimeWiring(service, &data); err == nil {
		t.Fatal("missing cancellation map accepted")
	}
}
