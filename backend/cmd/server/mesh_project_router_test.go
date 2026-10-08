package main

import (
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type projectListing struct {
	Projects []struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		OwnerID  string `json:"ownerId"`
		RoleID   string `json:"roleId"`
		Revision int64  `json:"revision"`
		ReadOnly bool   `json:"readOnly"`
	} `json:"projects"`
	HistoricalProjects []struct {
		ID       string `json:"id"`
		OwnerID  string `json:"ownerId"`
		ReadOnly bool   `json:"readOnly"`
	} `json:"historicalProjects"`
	Scope coordination.ExecutionScope `json:"executionScope"`
}

func TestActualTLSProjectAPIPreservesSourceHistoryAndRejectsChangedOwnership(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "projects-a", schemas), newThreeCoreFixture(t, "projects-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	query := decodeThreeCoreMemoryResponse[projectListing](t, b.request(t, a, http.MethodGet, "/api/device-mesh/v1/business/projects?characterId=one", nil, 200))
	if len(query.Projects) != 0 || query.Scope.ResourceOwnerID != a.device.DeviceID.String() {
		t.Fatal("new device projects have wrong owner")
	}
	payload := map[string]any{"requestId": "source-project", "title": "设备分组", "characterId": "one", "expectedExecutionScope": query.Scope}
	created := decodeThreeCoreMemoryResponse[business.ProjectResponse](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/projects", payload, 200))
	if !created.Saved || created.Acknowledgement.OwnerID != a.device.DeviceID.String() {
		t.Fatal("Source did not confirm project")
	}
	for i := 0; i < 2; i++ {
		replayed := decodeThreeCoreMemoryResponse[business.ProjectResponse](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/projects", payload, 200))
		if replayed.Project != created.Project {
			t.Fatal("project retry changed identity")
		}
	}
	query = decodeThreeCoreMemoryResponse[projectListing](t, b.request(t, a, http.MethodGet, "/api/device-mesh/v1/business/projects?characterId=one", nil, 200))
	if len(query.Projects) != 1 || query.Projects[0].OwnerID != a.device.DeviceID.String() || query.Projects[0].ReadOnly {
		t.Fatal("Source project list is incorrect")
	}
	read := decodeThreeCoreMemoryResponse[struct {
		Resource coordination.Resource `json:"resource"`
	}](t, b.request(t, a, http.MethodGet, "/api/device-mesh/v1/business/resources?characterId=one&kind=project&id="+created.Project.ID, nil, 200))
	if read.Resource.Revision != 1 || read.Resource.OwnerID != a.device.DeviceID.String() {
		t.Fatal("Source project resource read failed")
	}
	assertThreeCorePrivateCopies(t, b, a.device.DeviceID.String())
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
	payload["requestId"] = "old-device-form"
	b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/projects", payload, http.StatusConflict)
	query = decodeThreeCoreMemoryResponse[projectListing](t, b.request(t, a, http.MethodGet, "/api/device-mesh/v1/business/projects?characterId=one&historicalRoleId=one", nil, 200))
	if len(query.Projects) != 0 || len(query.HistoricalProjects) != 1 || !query.HistoricalProjects[0].ReadOnly || query.HistoricalProjects[0].OwnerID != a.device.DeviceID.String() {
		t.Fatal("old device project was migrated or became unavailable")
	}
	payload["requestId"], payload["title"], payload["expectedExecutionScope"] = "core-project", "云端分组", query.Scope
	current := decodeThreeCoreMemoryResponse[business.ProjectResponse](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/projects", payload, 200))
	if current.Acknowledgement.OwnerID != b.core {
		t.Fatal("coordinated project not stored at Core")
	}
	query = decodeThreeCoreMemoryResponse[projectListing](t, b.request(t, a, http.MethodGet, "/api/device-mesh/v1/business/projects?characterId=one&historicalRoleId=one", nil, 200))
	if len(query.Projects) != 1 || query.Projects[0].OwnerID != b.core || len(query.HistoricalProjects) != 1 {
		t.Fatal("project owners were mixed")
	}
	for _, fixture := range []*threeCoreFixture{a, b} {
		owner := fixture.core
		if fixture == a {
			owner = a.device.DeviceID.String()
		}
		rows, err := coordination.NewOwnershipStore(fixture.services.KernelContainer.DeviceRegistry.Database(), owner).List(t.Context(), "project", "one", false)
		if err != nil || len(rows) != 1 {
			t.Fatal("project missing from its original database")
		}
	}
}
