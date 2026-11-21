package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestPlatformMatrixServesLocalRolesAndHistoricalScope(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	platforms := []runtimeidentity.Platform{
		runtimeidentity.PlatformWindows,
		runtimeidentity.PlatformAndroid,
		runtimeidentity.PlatformIOS,
	}
	const endpoint = "/api/device-mesh/v1/business"
	for _, platform := range platforms {
		t.Run(string(platform), func(t *testing.T) {
			center := newThreeCoreFixture(t, "matrix-center-"+string(platform), schemas)
			device := newThreeCoreFixture(t, "matrix-device-"+string(platform), schemas, platform)
			pairThreeCoreFixtures(t, device, center)

			policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, device.device.DeviceID.String())
			if err != nil {
				t.Fatal(err)
			}
			ensureCoordinatedForTest(t, center, device, policy.ModeRevision)
			policy, err = center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, device.device.DeviceID.String())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := center.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), center.core, device.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
				t.Fatal(err)
			}

			rolesPath := endpoint + "/roles?domain=local&device=" + device.device.DeviceID.String()
			var catalog struct {
				Roles       []coordination.Role `json:"roles"`
				RoleOwnerID string              `json:"roleOwnerId"`
			}
			if err := json.Unmarshal(center.request(t, device, http.MethodGet, rolesPath, nil, http.StatusOK), &catalog); err != nil {
				t.Fatal(err)
			}
			if catalog.RoleOwnerID != device.device.DeviceID.String() || len(catalog.Roles) == 0 {
				t.Fatalf("平台 %s 的本机角色目录不可读: owner=%q roles=%+v", platform, catalog.RoleOwnerID, catalog.Roles)
			}

			scopePath := endpoint + "/historical-scope?targetDeviceId=" + device.device.DeviceID.String() + "&characterId=" + catalog.Roles[0].ID
			var envelope struct {
				Data struct {
					ExecutionScope coordination.ExecutionScope `json:"executionScope"`
				} `json:"data"`
			}
			if err := json.Unmarshal(center.request(t, device, http.MethodGet, scopePath, nil, http.StatusOK), &envelope); err != nil {
				t.Fatal(err)
			}
			scope := envelope.Data.ExecutionScope
			if scope.RoleOwnerID != device.device.DeviceID.String() || scope.ResourceOwnerID != device.device.DeviceID.String() || scope.TargetDeviceID != device.device.DeviceID.String() {
				t.Fatalf("平台 %s 的历史执行范围不可用: %+v", platform, scope)
			}
		})
	}
}

func TestBrowserDeviceServesLocalDataWithoutCoordination(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center := newThreeCoreFixture(t, "browser-center", schemas)
	browser := newThreeCoreFixture(t, "browser-device", schemas, runtimeidentity.PlatformWeb)
	pairThreeCoreFixtures(t, browser, center)

	var catalog struct {
		Roles       []coordination.Role `json:"roles"`
		RoleOwnerID string              `json:"roleOwnerId"`
		Domain      string              `json:"domain"`
	}
	path := "/api/device-mesh/v1/business/roles?domain=local&device=" + browser.device.DeviceID.String()
	if err := json.Unmarshal(center.request(t, browser, http.MethodGet, path, nil, http.StatusOK), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.RoleOwnerID != browser.device.DeviceID.String() || catalog.Domain != coordination.RoleDomainLocal || len(catalog.Roles) == 0 {
		t.Fatalf("关闭统筹的浏览器设备未提供本机数据端口: owner=%q domain=%q roles=%+v", catalog.RoleOwnerID, catalog.Domain, catalog.Roles)
	}
}

func TestHistoricalScopeFailsAfterCapabilityRevoked(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center := newThreeCoreFixture(t, "revoke-center", schemas)
	admin := newThreeCoreFixture(t, "revoke-admin", schemas)
	device := newThreeCoreFixture(t, "revoke-device", schemas, runtimeidentity.PlatformAndroid)
	pairThreeCoreFixtures(t, admin, center)
	pairThreeCoreFixtures(t, device, center)
	const endpoint = "/api/device-mesh/v1/business"

	for _, fixture := range []*threeCoreFixture{admin, device} {
		policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, fixture.device.DeviceID.String())
		if err != nil {
			t.Fatal(err)
		}
		ensureCoordinatedForTest(t, center, fixture, policy.ModeRevision)
	}
	policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, admin.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := center.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), center.core, admin.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	grants, err := center.services.DeviceMesh.Coordination.CapabilityGrants(t.Context(), center.core, device.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	revision := int64(0)
	for _, grant := range grants {
		if grant.CallerID == admin.device.DeviceID.String() && grant.Capability == "ai.chat" {
			revision = grant.Revision
		}
	}
	if _, err := center.services.DeviceMesh.Coordination.SetCapabilityGrant(t.Context(), center.core, admin.device.DeviceID.String(), device.device.DeviceID.String(), "ai.chat", revision, true); err != nil {
		t.Fatal(err)
	}

	rolesPath := endpoint + "/roles?domain=local&device=" + device.device.DeviceID.String()
	var catalog struct {
		Roles []coordination.Role `json:"roles"`
	}
	if err := json.Unmarshal(center.request(t, admin, http.MethodGet, rolesPath, nil, http.StatusOK), &catalog); err != nil || len(catalog.Roles) == 0 {
		t.Fatalf("读取本机角色失败: roles=%+v error=%v", catalog.Roles, err)
	}
	scopePath := endpoint + "/historical-scope?targetDeviceId=" + device.device.DeviceID.String() + "&characterId=" + catalog.Roles[0].ID
	var envelope struct {
		Data struct {
			ExecutionScope coordination.ExecutionScope `json:"executionScope"`
		} `json:"data"`
	}
	if err := json.Unmarshal(center.request(t, admin, http.MethodGet, scopePath, nil, http.StatusOK), &envelope); err != nil {
		t.Fatal(err)
	}
	scope := envelope.Data.ExecutionScope
	if _, err := center.services.DeviceMesh.Coordination.SetCapabilityGrant(t.Context(), center.core, admin.device.DeviceID.String(), device.device.DeviceID.String(), "ai.chat", revision+1, false); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"requestId":              uuid.NewString(),
		"characterId":            catalog.Roles[0].ID,
		"targetDeviceId":         device.device.DeviceID.String(),
		"expectedExecutionScope": scope,
		"action":                 "create",
		"key":                    "撤销后写入",
		"value":                  "不应落库",
		"memoryType":             "fact",
		"importance":             3,
		"retentionLevel":         2,
	}
	center.request(t, admin, http.MethodPost, endpoint+"/memories/manage", payload, http.StatusConflict)
	rows, err := coordination.NewOwnershipStore(device.services.KernelContainer.DeviceRegistry.Database(), device.device.DeviceID.String()).List(t.Context(), "memory", coordination.StorageRole(scope, "memory"), false)
	if err != nil || len(rows) != 0 {
		t.Fatalf("撤销权限后仍写入了设备原库: rows=%+v error=%v", rows, err)
	}
}
