package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestActualTLSWorldbookStaysAtSourceAndReachesOwnedChat(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "worldbook-source-a", schemas), newThreeCoreFixture(t, "worldbook-core-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	const endpoint = "/api/device-mesh/v1/business"
	view := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	if view.Scope.ResourceOwnerID != a.device.DeviceID.String() || len(view.Snapshot.Resources) != 0 {
		t.Fatal("worldbook did not select the source device as data owner")
	}
	create := func(scopeName, pattern, content, requestID string) (business.WorldbookMutationResponse, string) {
		if requestID == "" {
			requestID = uuid.NewString()
		}
		response := decodeThreeCoreMemoryResponse[business.WorldbookMutationResponse](t, b.request(t, a, http.MethodPost, endpoint+"/worldbook/manage", map[string]any{
			"requestId": requestID, "characterId": "one", "expectedExecutionScope": view.Scope,
			"action": "create", "rule": map[string]any{
				"scope": scopeName, "matchType": "keyword", "matchPattern": pattern,
				"matchScope": "full_context", "injectContent": content, "priority": 5, "enabled": true,
			},
		}, http.StatusOK))
		if !response.Saved || response.Scope.ResourceOwnerID != a.device.DeviceID.String() || response.Acknowledgement.OwnerID != a.device.DeviceID.String() || response.Acknowledgement.Versions["checkpoint/worldbook/"+requestID] != 1 {
			t.Fatal("worldbook was not acknowledged by the source device")
		}
		return response, requestID
	}
	roleRule, roleRequestID := create("role", "晨星", "晨星计划属于角色设定", "")
	globalRule, _ := create("global", "晨星联盟", "晨星联盟是全局世界观", "")
	replayed, _ := create("role", "晨星", "晨星计划属于角色设定", roleRequestID)
	if roleRule.Rule == nil || globalRule.Rule == nil || roleRule.Rule.RoleID != "one" || globalRule.Rule.RoleID != coordination.GlobalWorldbookRole {
		t.Fatal("worldbook role and global partitions were not preserved")
	}
	if replayed.Rule == nil || replayed.Rule.ID != roleRule.Rule.ID || replayed.Rule.Revision != roleRule.Rule.Revision {
		t.Fatal("worldbook create replay was not idempotent")
	}
	updated := decodeThreeCoreMemoryResponse[business.WorldbookMutationResponse](t, b.request(t, a, http.MethodPost, endpoint+"/worldbook/manage", map[string]any{
		"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope,
		"action": "update", "expectedRevision": roleRule.Rule.Revision,
		"rule": map[string]any{
			"id": roleRule.Rule.ID, "scope": "role", "matchType": "keyword", "matchPattern": "晨星计划",
			"matchScope": "full_context", "injectContent": "更新后的角色设定", "priority": 7, "enabled": true,
			"createdAt": roleRule.Rule.CreatedAt,
		},
	}, http.StatusOK))
	if !updated.Saved || updated.Rule == nil || updated.Rule.Revision != roleRule.Rule.Revision+1 || updated.Rule.MatchPattern != "晨星计划" {
		t.Fatal("worldbook update did not preserve owner CAS")
	}
	listed := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	if len(listed.Snapshot.Resources) != 2 {
		t.Fatalf("source worldbook list has %d rules, want 2", len(listed.Snapshot.Resources))
	}
	matched := decodeThreeCoreMemoryResponse[business.WorldbookMatchResponse](t, b.request(t, a, http.MethodPost, endpoint+"/worldbook/match", map[string]any{
		"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope,
		"text": "晨星计划和晨星联盟",
	}, http.StatusOK))
	if len(matched.Matches) != 2 || matched.Scope.ResourceOwnerID != a.device.DeviceID.String() {
		t.Fatalf("worldbook match did not use source rules: %+v", matched)
	}
	var sourceCount, coreCount int
	if err := a.services.KernelContainer.DeviceRegistry.Database().QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='worldbook' AND deleted=0`, a.device.DeviceID.String()).Scan(&sourceCount); err != nil || sourceCount != 2 {
		t.Fatalf("source worldbook rows=%d err=%v", sourceCount, err)
	}
	if err := b.services.KernelContainer.DeviceRegistry.Database().QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='worldbook' AND deleted=0`).Scan(&coreCount); err != nil || coreCount != 0 {
		t.Fatalf("worldbook leaked into Core rows=%d err=%v", coreCount, err)
	}
	reply := decodeThreeCoreMemoryResponse[business.Response](t, b.request(t, a, http.MethodPost, endpoint+"/messages", map[string]any{
		"requestId": "worldbook-chat", "characterId": "one", "message": "请说明晨星计划和晨星联盟",
	}, http.StatusOK))
	if !reply.Saved || reply.Scope.ResourceOwnerID != a.device.DeviceID.String() {
		t.Fatal("worldbook chat reply was not saved by the source device")
	}
	select {
	case resources := <-b.model.worldbooks:
		found := map[string]bool{}
		for _, resource := range resources {
			if resource.Kind == "worldbook" {
				found[resource.RoleID] = true
			}
		}
		if !found["one"] || !found[coordination.GlobalWorldbookRole] {
			t.Fatalf("chat model did not receive role and global worldbooks: %+v", found)
		}
	default:
		t.Fatal("chat model was not invoked")
	}
	staleUpdate := map[string]any{
		"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope,
		"action": "update", "expectedRevision": roleRule.Rule.Revision,
		"rule": map[string]any{
			"id": roleRule.Rule.ID, "scope": "role", "matchType": "keyword", "matchPattern": "过期写入",
			"matchScope": "full_context", "injectContent": "不得保存", "priority": 1, "enabled": true,
		},
	}
	b.request(t, a, http.MethodPost, endpoint+"/worldbook/manage", staleUpdate, http.StatusConflict)
	deleted := decodeThreeCoreMemoryResponse[business.WorldbookMutationResponse](t, b.request(t, a, http.MethodPost, endpoint+"/worldbook/manage", map[string]any{
		"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope,
		"action": "delete", "expectedRevision": updated.Rule.Revision,
		"rule": map[string]any{"id": roleRule.Rule.ID, "scope": "role"},
	}, http.StatusOK))
	if !deleted.Saved || deleted.Rule != nil || deleted.Acknowledgement.OwnerID != a.device.DeviceID.String() {
		t.Fatal("worldbook delete did not receive owner confirmation")
	}
}

func TestActualTLSWorldbookMovesToCurrentCoreAuthority(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "worldbook-switch-a", schemas), newThreeCoreFixture(t, "worldbook-switch-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	const endpoint = "/api/device-mesh/v1/business"
	view := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, http.StatusOK)
	stale := map[string]any{
		"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope,
		"action": "create", "rule": map[string]any{"scope": "role", "matchType": "keyword", "matchPattern": "旧设备", "matchScope": "full_context", "injectContent": "旧设备规则", "priority": 1, "enabled": true},
	}
	b.request(t, a, http.MethodPost, endpoint+"/worldbook/manage", stale, http.StatusConflict)
	current := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	if current.Scope.ResourceOwnerID != b.core || len(current.Snapshot.Resources) != 0 {
		t.Fatal("worldbook did not move to the coordinated Core owner")
	}
	stale["requestId"], stale["expectedExecutionScope"] = uuid.NewString(), current.Scope
	created := decodeThreeCoreMemoryResponse[business.WorldbookMutationResponse](t, b.request(t, a, http.MethodPost, endpoint+"/worldbook/manage", stale, http.StatusOK))
	if !created.Saved || created.Acknowledgement.OwnerID != b.core || created.Rule == nil {
		t.Fatal("coordinated worldbook was not saved at Core")
	}
	var rows int
	if err := b.services.KernelContainer.DeviceRegistry.Database().QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='worldbook' AND deleted=0`, b.core).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("core worldbook rows=%d err=%v", rows, err)
	}
	var body []byte
	if err := b.services.KernelContainer.DeviceRegistry.Database().QueryRow(`SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='worldbook' AND deleted=0`, b.core).Scan(&body); err != nil {
		t.Fatal(err)
	}
	var rule business.WorldbookRule
	if json.Unmarshal(body, &rule) != nil || rule.MatchPattern != "旧设备" {
		t.Fatal("coordinated worldbook body changed during save")
	}
}
