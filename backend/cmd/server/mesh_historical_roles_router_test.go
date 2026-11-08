package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type historicalRolesHTTPResponse struct {
	Data struct {
		Roles          []coordination.HistoricalRole `json:"roles"`
		ExecutionScope coordination.ExecutionScope   `json:"executionScope"`
	} `json:"data"`
}

func TestHistoricalRolesUsesActorDeviceAndReturnsCoordinatedArchive(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	source := newThreeCoreFixture(t, "history-source", schemas)
	provider := newThreeCoreFixture(t, "history-provider", schemas)
	pairThreeCoreFixtures(t, source, provider)

	var current historicalRolesHTTPResponse
	if err := json.Unmarshal(provider.request(t, source, http.MethodGet, "/api/device-mesh/v1/business/historical-roles?characterId=one", nil, http.StatusOK), &current); err != nil {
		t.Fatal(err)
	}
	if len(current.Data.Roles) != 0 ||
		current.Data.ExecutionScope.TargetDeviceID != source.device.DeviceID.String() ||
		current.Data.ExecutionScope.ResourceOwnerID != source.device.DeviceID.String() ||
		current.Data.ExecutionScope.RoleOwnerID != provider.core ||
		current.Data.ExecutionScope.Coordinated {
		t.Fatalf("未统筹历史角色目录不合法: roles=%+v scope=%+v", current.Data.Roles, current.Data.ExecutionScope)
	}

	if _, err := source.services.KernelContainer.DeviceRegistry.Database().ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,deleted,body,updated_at) VALUES(?,'memory','archived-memory','archived-role',1,0,'{}',?)`, source.device.DeviceID.String(), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	policy, err := provider.services.DeviceMesh.Coordination.Get(t.Context(), provider.core, source.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, provider, source, policy.ModeRevision)

	var archived historicalRolesHTTPResponse
	if err := json.Unmarshal(provider.request(t, source, http.MethodGet, "/api/device-mesh/v1/business/historical-roles?characterId=one", nil, http.StatusOK), &archived); err != nil {
		t.Fatal(err)
	}
	if len(archived.Data.Roles) != 1 ||
		archived.Data.Roles[0].ID != "archived-role" ||
		archived.Data.Roles[0].Name != "archived-role" ||
		archived.Data.ExecutionScope.TargetDeviceID != source.device.DeviceID.String() ||
		archived.Data.ExecutionScope.ResourceOwnerID != source.device.DeviceID.String() ||
		archived.Data.ExecutionScope.RoleOwnerID != provider.core ||
		!archived.Data.ExecutionScope.Coordinated {
		t.Fatalf("统筹历史角色目录不合法: roles=%+v scope=%+v", archived.Data.Roles, archived.Data.ExecutionScope)
	}
}
