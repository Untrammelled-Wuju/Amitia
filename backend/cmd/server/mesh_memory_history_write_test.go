package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestMemoryManagementWritesHistoricalDeviceMemory(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center := newThreeCoreFixture(t, "history-center", schemas)
	admin := newThreeCoreFixture(t, "history-admin", schemas)
	peer := newThreeCoreFixture(t, "history-peer", schemas)
	pairThreeCoreFixtures(t, admin, center)
	pairThreeCoreFixtures(t, peer, center)
	const endpoint = "/api/device-mesh/v1/business"

	policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, admin.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, center, admin, policy.ModeRevision)
	policy, err = center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, admin.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := center.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), center.core, admin.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	peerPolicy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, peer.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, center, peer, peerPolicy.ModeRevision)
	grants, err := center.services.DeviceMesh.Coordination.CapabilityGrants(t.Context(), center.core, peer.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	revision := int64(0)
	for _, grant := range grants {
		if grant.CallerID == admin.device.DeviceID.String() && grant.Capability == "ai.chat" {
			revision = grant.Revision
		}
	}
	if _, err := center.services.DeviceMesh.Coordination.SetCapabilityGrant(t.Context(), center.core, admin.device.DeviceID.String(), peer.device.DeviceID.String(), "ai.chat", revision, true); err != nil {
		t.Fatal(err)
	}

	peerRolesPath := endpoint + "/roles?domain=local&device=" + peer.device.DeviceID.String()
	var catalog struct {
		Roles []coordination.Role `json:"roles"`
	}
	if err := json.Unmarshal(center.request(t, admin, http.MethodGet, peerRolesPath, nil, http.StatusOK), &catalog); err != nil || len(catalog.Roles) == 0 {
		t.Fatalf("管理员未读取到目标设备本机角色: roles=%+v error=%v", catalog.Roles, err)
	}
	roleID := catalog.Roles[0].ID
	requestID := uuid.NewString()

	ctx, scope, finish, err := center.services.DeviceMesh.Coordination.BeginWithOwners(t.Context(), center.core, admin.device.DeviceID.String(), peer.device.DeviceID.String(), center.core, "", requestID, peer.device.DeviceID.String(), peer.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	roles, err := center.services.DeviceMesh.Roles(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	role, err := coordination.ResolveRole(roleID, roles)
	if err != nil {
		t.Fatal(err)
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	if err := coordination.ResolveMemoryScope(&scope, ""); err != nil {
		t.Fatal(err)
	}

	payload := map[string]any{
		"requestId":              requestID,
		"characterId":            roleID,
		"targetDeviceId":         peer.device.DeviceID.String(),
		"expectedExecutionScope": scope,
		"action":                 "create",
		"key":                    "旧库饮品",
		"value":                  "喜欢乌龙茶",
		"memoryType":             "preference",
		"importance":             7,
		"retentionLevel":         2,
	}
	created := decodeThreeCoreMemoryResponse[business.MemoryManagementResponse](t, center.request(t, admin, http.MethodPost, endpoint+"/memories/manage", payload, http.StatusOK))
	if !created.Saved || created.Acknowledgement.OwnerID != peer.device.DeviceID.String() {
		t.Fatalf("历史记忆未被目标设备确认: owner=%q saved=%v", created.Acknowledgement.OwnerID, created.Saved)
	}
	rows, err := coordination.NewOwnershipStore(peer.services.KernelContainer.DeviceRegistry.Database(), peer.device.DeviceID.String()).List(t.Context(), "memory", coordination.StorageRole(scope, "memory"), false)
	if err != nil || len(rows) != 1 {
		t.Fatalf("历史记忆未写入目标设备原库: rows=%d error=%v", len(rows), err)
	}

	var scopeEnvelope struct {
		Data struct {
			ExecutionScope coordination.ExecutionScope `json:"executionScope"`
		} `json:"data"`
	}
	scopePath := endpoint + "/historical-scope?targetDeviceId=" + peer.device.DeviceID.String() + "&characterId=" + roleID
	if err := json.Unmarshal(center.request(t, admin, http.MethodGet, scopePath, nil, http.StatusOK), &scopeEnvelope); err != nil {
		t.Fatal(err)
	}
	if scopeEnvelope.Data.ExecutionScope.RoleOwnerID != peer.device.DeviceID.String() || scopeEnvelope.Data.ExecutionScope.ResourceOwnerID != peer.device.DeviceID.String() || scopeEnvelope.Data.ExecutionScope.TargetDeviceID != peer.device.DeviceID.String() {
		t.Fatalf("历史执行范围未指向目标设备: %+v", scopeEnvelope.Data.ExecutionScope)
	}
	second := map[string]any{
		"requestId":              uuid.NewString(),
		"characterId":            roleID,
		"targetDeviceId":         peer.device.DeviceID.String(),
		"expectedExecutionScope": scopeEnvelope.Data.ExecutionScope,
		"action":                 "create",
		"key":                    "旧库书目",
		"value":                  "在读通鉴",
		"memoryType":             "fact",
		"importance":             4,
		"retentionLevel":         2,
	}
	center.request(t, admin, http.MethodPost, endpoint+"/memories/manage", second, http.StatusOK)

	var editedMemory coordination.Resource
	for _, resource := range created.Resources {
		if resource.Kind == "memory" {
			editedMemory = resource
		}
	}
	if editedMemory.ID == "" {
		t.Fatal("历史记忆未返回资源标识")
	}
	edited := center.request(t, admin, http.MethodPost, endpoint+"/resources/edit", map[string]any{
		"requestId":              uuid.NewString(),
		"kind":                   "memory",
		"id":                     editedMemory.ID,
		"characterId":            roleID,
		"expectedRevision":       editedMemory.Revision,
		"expectedExecutionScope": scope,
		"changes":                map[string]any{"pinned": true},
	}, http.StatusOK)
	if len(edited) == 0 {
		t.Fatal("历史记忆修改未返回确认")
	}
	after, err := coordination.NewOwnershipStore(peer.services.KernelContainer.DeviceRegistry.Database(), peer.device.DeviceID.String()).List(t.Context(), "memory", coordination.StorageRole(scope, "memory"), false)
	if err != nil || len(after) != 2 {
		t.Fatalf("历史记忆未落在目标设备原库: rows=%+v error=%v", after, err)
	}
	bumped := false
	for _, row := range after {
		if row.ID == editedMemory.ID && row.Revision == editedMemory.Revision+1 {
			bumped = true
		}
	}
	if !bumped {
		t.Fatalf("历史记忆修改未落到目标设备原库: rows=%+v", after)
	}
}
