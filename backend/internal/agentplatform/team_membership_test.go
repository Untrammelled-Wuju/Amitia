package agentplatform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateTeamWithDeviceOwnedMembers(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	lead := createTestAgent(t, service, spaceID, ownerID, "中心负责人")
	team, err := service.CreateTeam(ctx, spaceID, CreateTeamInput{
		Name:        "跨设备团队",
		OwnerID:     ownerID,
		LeadAgentID: lead.Profile.ID,
		Members: []TeamMemberInput{
			{MemberType: TeamMemberTypeAgent, MemberID: "role-remote", OwnerType: MemberOwnerDevice, OwnerID: "device-k50", RoleRevision: 3},
			{MemberType: TeamMemberTypeDevice, OwnerID: "device-k50"},
		},
	})
	if err != nil {
		t.Fatalf("创建跨设备团队失败: %v", err)
	}
	detail, err := service.GetTeam(ctx, spaceID, team.ID)
	if err != nil {
		t.Fatalf("读取团队失败: %v", err)
	}
	if len(detail.Members) != 3 {
		t.Fatalf("团队成员数 = %d, want 3", len(detail.Members))
	}
	byMember := map[string]TeamMember{}
	for _, member := range detail.Members {
		byMember[member.MemberType+":"+member.MemberID] = member
	}
	remote, ok := byMember["agent:role-remote"]
	if !ok {
		t.Fatalf("未找到跨设备角色成员: %+v", detail.Members)
	}
	if remote.OwnerType != MemberOwnerDevice || remote.OwnerID != "device-k50" || remote.InvocationMode != InvocationModeRole || remote.RoleRevision != 3 {
		t.Fatalf("跨设备角色成员归属错误: %+v", remote)
	}
	deviceCapability, ok := byMember["device:device-k50"]
	if !ok {
		t.Fatalf("未找到设备能力成员: %+v", detail.Members)
	}
	if deviceCapability.OwnerType != MemberOwnerDevice || deviceCapability.InvocationMode != InvocationModeCapability {
		t.Fatalf("设备能力成员归属错误: %+v", deviceCapability)
	}
	leadMember := byMember["agent:"+lead.Profile.ID]
	if leadMember.OwnerType != MemberOwnerCenter || leadMember.InvocationMode != InvocationModeRole {
		t.Fatalf("中心成员归属错误: %+v", leadMember)
	}
	var capabilities []string
	if err := json.Unmarshal([]byte(leadMember.Capabilities), &capabilities); err != nil || !teamContainsString(capabilities, "*") {
		t.Fatalf("中心负责人能力错误: %s", leadMember.Capabilities)
	}
}

func TestCreateTeamRejectsDuplicateCrossOwnerMembers(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	_, err := service.CreateTeam(ctx, spaceID, CreateTeamInput{
		Name:    "重复成员团队",
		OwnerID: ownerID,
		Members: []TeamMemberInput{
			{MemberType: TeamMemberTypeDevice, OwnerID: "device-k50"},
			{MemberType: TeamMemberTypeDevice, OwnerID: "device-k50"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "团队成员重复") {
		t.Fatalf("重复成员未被拒绝: %v", err)
	}
}

func TestAddTeamMemberOwnershipValidation(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	team, err := service.CreateTeam(ctx, spaceID, CreateTeamInput{Name: "校验团队", OwnerID: ownerID})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		input TeamMemberInput
		want  string
	}{
		{name: "能力模式限定设备成员", input: TeamMemberInput{MemberType: TeamMemberTypeAgent, MemberID: "role", InvocationMode: InvocationModeCapability}, want: "能力调用成员必须是设备成员"},
		{name: "设备归属必须有设备 ID", input: TeamMemberInput{MemberType: TeamMemberTypeAgent, MemberID: "role", OwnerType: MemberOwnerDevice}, want: "设备归属成员必须指定设备 ID"},
		{name: "设备角色必须有角色 ID", input: TeamMemberInput{MemberType: TeamMemberTypeAgent, MemberID: "device-k50", OwnerType: MemberOwnerDevice, OwnerID: "device-k50"}, want: "设备角色成员必须指定角色 ID"},
		{name: "中心成员必须存在", input: TeamMemberInput{MemberType: TeamMemberTypeAgent, MemberID: "missing-role"}, want: "团队成员包含不存在的角色"},
		{name: "中心成员不记录版本", input: TeamMemberInput{MemberType: TeamMemberTypeAgent, MemberID: "role", RoleRevision: 2}, want: "统筹中心角色成员不记录角色版本"},
		{name: "用户成员归属中心", input: TeamMemberInput{MemberType: TeamMemberTypeUser, MemberID: "user", OwnerType: MemberOwnerDevice, OwnerID: "device-k50"}, want: "用户成员必须归属统筹中心"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			_, err := service.AddTeamMember(ctx, spaceID, team.ID, item.input)
			if err == nil || !strings.Contains(err.Error(), item.want) {
				t.Fatalf("错误 = %v, want 包含 %q", err, item.want)
			}
		})
	}
	member, err := service.AddTeamMember(ctx, spaceID, team.ID, TeamMemberInput{
		MemberType: TeamMemberTypeAgent, MemberID: "role-remote",
		OwnerType: MemberOwnerDevice, OwnerID: "device-k50", RoleRevision: 5,
	})
	if err != nil {
		t.Fatalf("添加跨设备成员失败: %v", err)
	}
	if member.OwnerID != "device-k50" || member.InvocationMode != InvocationModeRole || member.RoleRevision != 5 {
		t.Fatalf("跨设备成员字段错误: %+v", member)
	}
	if _, err := service.AddTeamMember(ctx, spaceID, team.ID, TeamMemberInput{
		MemberType: TeamMemberTypeAgent, MemberID: "role-remote",
		OwnerType: MemberOwnerDevice, OwnerID: "device-k50",
	}); err == nil || !strings.Contains(err.Error(), "已在此团队中") {
		t.Fatalf("重复成员未被拒绝: %v", err)
	}
	if _, err := service.AddTeamMember(ctx, spaceID, team.ID, TeamMemberInput{
		MemberType: TeamMemberTypeDevice, OwnerID: "device-k50",
	}); err != nil {
		t.Fatalf("同一设备的不同调用模式应可共存: %v", err)
	}
}

func TestRouteTeamMessageCarriesMemberOwnership(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	ctx := context.Background()
	lead := createTestAgent(t, service, spaceID, ownerID, "中心负责人")
	team, err := service.CreateTeam(ctx, spaceID, CreateTeamInput{
		Name: "路由团队", OwnerID: ownerID, LeadAgentID: lead.Profile.ID,
		Members: []TeamMemberInput{
			{MemberType: TeamMemberTypeAgent, MemberID: "role-remote", OwnerType: MemberOwnerDevice, OwnerID: "device-k50", RoleRevision: 4},
			{MemberType: TeamMemberTypeDevice, OwnerID: "device-k50"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	byDevice, err := service.RouteTeamMessage(ctx, spaceID, team.ID, RouteTeamMessageInput{
		Message: "让红米K50上的小美处理", MentionAgentIDs: []string{"device-k50"},
	})
	if err != nil {
		t.Fatalf("按设备寻址路由失败: %v", err)
	}
	if len(byDevice.Targets) != 2 {
		t.Fatalf("设备寻址目标数 = %d, want 2", len(byDevice.Targets))
	}
	for _, target := range byDevice.Targets {
		if target.OwnerType != MemberOwnerDevice || target.OwnerID != "device-k50" || !target.RequiresDevice {
			t.Fatalf("目标归属错误: %+v", target)
		}
	}
	byRole, err := service.RouteTeamMessage(ctx, spaceID, team.ID, RouteTeamMessageInput{
		Message: "小美你来看一下", MentionAgentIDs: []string{"role-remote"},
	})
	if err != nil {
		t.Fatalf("按角色寻址路由失败: %v", err)
	}
	if len(byRole.Targets) != 1 {
		t.Fatalf("角色寻址目标数 = %d, want 1", len(byRole.Targets))
	}
	target := byRole.Targets[0]
	if target.InvocationMode != InvocationModeRole || target.OwnerID != "device-k50" || target.RoleRevision != 4 || target.MemberType != TeamMemberTypeAgent {
		t.Fatalf("角色调用目标错误: %+v", target)
	}
	leadRoute, err := service.RouteTeamMessage(ctx, spaceID, team.ID, RouteTeamMessageInput{Message: "大家好"})
	if err != nil {
		t.Fatalf("默认路由失败: %v", err)
	}
	if leadRoute.Strategy != "lead" || len(leadRoute.Targets) != 1 {
		t.Fatalf("默认路由 = %+v", leadRoute)
	}
	if leadRoute.Targets[0].OwnerType != MemberOwnerCenter || leadRoute.Targets[0].RequiresDevice {
		t.Fatalf("中心负责人目标错误: %+v", leadRoute.Targets[0])
	}
}
