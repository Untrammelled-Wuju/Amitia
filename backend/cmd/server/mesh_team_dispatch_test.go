package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/agentplatform"
)

func TestTeamDispatchTargetsDeviceRole(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center := newThreeCoreFixture(t, "dispatch-center", schemas)
	admin := newThreeCoreFixture(t, "dispatch-admin", schemas)
	peer := newThreeCoreFixture(t, "dispatch-peer", schemas)
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

	platform, err := agentplatform.NewService(center.services.DB, agentplatform.ServiceConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	center.services.AgentPlatform = platform
	team, err := platform.CreateTeam(t.Context(), center.core, agentplatform.CreateTeamInput{
		Name: "跨设备调度团队",
		Members: []agentplatform.TeamMemberInput{{
			MemberType: "agent", MemberID: "one", OwnerType: "device",
			OwnerID: peer.device.DeviceID.String(), InvocationMode: "role",
			Role: "lead", RoleRevision: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := map[string]any{"message": "帮忙整理今天的待办"}
	body := center.request(t, admin, http.MethodPost, endpoint+"/teams/"+team.ID+"/dispatch", payload, http.StatusOK)
	var envelope struct {
		Data struct {
			TeamID   string `json:"teamId"`
			Strategy string `json:"strategy"`
			Targets  []struct {
				OwnerID string `json:"ownerId"`
				RoleID  string `json:"roleId"`
				Mode    string `json:"mode"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.TeamID != team.ID || len(envelope.Data.Targets) != 1 {
		t.Fatalf("团队调度未返回唯一目标: %+v", envelope.Data)
	}
	target := envelope.Data.Targets[0]
	if target.OwnerID != peer.device.DeviceID.String() || target.RoleID != "one" || target.Mode != "role" {
		t.Fatalf("团队调度目标未指向设备角色: %+v", target)
	}
	if target.Status == "center" || target.Status == "unsupported" {
		t.Fatalf("设备角色被当作本机目标处理: %+v", target)
	}
	if target.Status != "completed" {
		t.Fatalf("设备角色团队调度未完成: %+v", target)
	}

	var denial struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(center.request(t, peer, http.MethodPost, endpoint+"/teams/"+team.ID+"/dispatch", payload, http.StatusForbidden), &denial); err != nil || denial.Code != "mesh.administrator_required" {
		t.Fatalf("未授权的团队调度未被拒绝: code=%s error=%v", denial.Code, err)
	}

	peerPolicy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, peer.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := center.services.DeviceMesh.Coordination.SelectRoleDomain(t.Context(), center.core, peer.device.DeviceID.String(), peerPolicy.ModeRevision, "local", "one"); err != nil {
		t.Fatal(err)
	}
	capabilityTeam, err := platform.CreateTeam(t.Context(), center.core, agentplatform.CreateTeamInput{
		Name: "设备能力团队",
		Members: []agentplatform.TeamMemberInput{{
			MemberType: "device", OwnerType: "device", OwnerID: peer.device.DeviceID.String(),
			MemberID: peer.device.DeviceID.String(), InvocationMode: "capability",
			Role: "lead", Capabilities: []string{"ai.chat"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	capabilityBody := center.request(t, admin, http.MethodPost, endpoint+"/teams/"+capabilityTeam.ID+"/dispatch", payload, http.StatusOK)
	var capabilityEnvelope struct {
		Data struct {
			Targets []struct {
				OwnerID string `json:"ownerId"`
				Mode    string `json:"mode"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(capabilityBody, &capabilityEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(capabilityEnvelope.Data.Targets) != 1 {
		t.Fatalf("设备能力团队调度未返回目标: %+v", capabilityEnvelope.Data)
	}
	capabilityTarget := capabilityEnvelope.Data.Targets[0]
	if capabilityTarget.Mode != "capability" || capabilityTarget.OwnerID != peer.device.DeviceID.String() {
		t.Fatalf("设备能力目标错误: %+v", capabilityTarget)
	}
	if capabilityTarget.Status != "completed" {
		t.Fatalf("设备能力成员调度未完成: %+v", capabilityTarget)
	}
}
