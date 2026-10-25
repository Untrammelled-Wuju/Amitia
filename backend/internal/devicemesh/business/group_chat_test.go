package business

import (
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestResolveGroupTargetsUsesStableMemberIdentityAndOrder(t *testing.T) {
	state := coordination.GroupState{RoleOwnerID: "owner", Members: []coordination.GroupMember{
		{MemberID: "member-b", CharacterID: "role-b", RoleOwnerID: "owner", Status: coordination.GroupMemberStatusActive, OrderIndex: 1},
		{MemberID: "member-a", CharacterID: "role-a", RoleOwnerID: "owner", Status: coordination.GroupMemberStatusActive, OrderIndex: 0},
		{MemberID: "member-left", CharacterID: "role-c", RoleOwnerID: "owner", Status: coordination.GroupMemberStatusLeft, OrderIndex: 2},
	}}
	targets, err := resolveGroupTargets(state, []GroupMention{{MemberID: "member-b", CharacterID: "role-b"}}, false)
	if err != nil || len(targets) != 1 || targets[0].MemberID != "member-b" {
		t.Fatalf("目标解析失败: %#v %v", targets, err)
	}
	all, err := resolveGroupTargets(state, nil, true)
	if err != nil || len(all) != 2 || all[0].MemberID != "member-a" || all[1].MemberID != "member-b" {
		t.Fatalf("全部成员顺序错误: %#v %v", all, err)
	}
	for _, mention := range []GroupMention{
		{MemberID: "member-left"},
		{MemberID: "member-b", CharacterID: "role-a"},
		{MemberID: "missing"},
	} {
		if _, err := resolveGroupTargets(state, []GroupMention{mention}, false); err == nil {
			t.Fatalf("非法目标未被拒绝: %#v", mention)
		}
	}
}

func TestGroupStateRejectsDuplicateActiveMembers(t *testing.T) {
	state := coordination.GroupState{
		ConversationID: "chat", ConversationRoleID: "role-a", RoleOwnerID: "owner",
		ConversationType: "group", MembershipRevision: 1, SettingsRevision: 1,
		Members: []coordination.GroupMember{
			{MemberID: "one", CharacterID: "role-a", RoleOwnerID: "owner", Status: coordination.GroupMemberStatusActive},
			{MemberID: "two", CharacterID: "role-a", RoleOwnerID: "owner", Status: coordination.GroupMemberStatusActive},
		},
	}
	if err := state.Validate(); err == nil {
		t.Fatal("重复有效成员未被拒绝")
	}
}
