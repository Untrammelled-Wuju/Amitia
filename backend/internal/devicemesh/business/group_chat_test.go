package business

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestGroupDeliveryStreamDoesNotRepeatAvatar(t *testing.T) {
	avatar := strings.Repeat("x", 85883)
	delivery := GroupDelivery{DeliveryID: "delivery", CharacterID: "role", CharacterName: "角色", RoleOwnerID: "owner"}
	started := groupDeliveryEvent(Event{Type: "started"}, delivery, 0, "round")
	if started.CharacterAvatar != "" {
		t.Fatal("开始事件重复携带头像")
	}
	total := 0
	for i := 0; i < 1000; i++ {
		event := groupDeliveryEvent(Event{Type: "delta", CharacterAvatar: avatar, Text: "字"}, delivery, 0, "round")
		if event.CharacterAvatar != "" || event.DeliveryID != "delivery" || event.CharacterID != "role" {
			t.Fatal("增量事件头像或投递身份错误")
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		total += len(encoded)
	}
	if total >= 8*1024*1024 {
		t.Fatalf("增量数据超出接收上限: %d", total)
	}
}

func TestResolveGroupTargetsUsesStableMemberIdentityAndOrder(t *testing.T) {
	state := coordination.GroupState{ConversationRoleID: "role-a", RoleOwnerID: "owner", Members: []coordination.GroupMember{
		{MemberID: "member-b", CharacterID: "role-b", RoleOwnerID: "owner", Status: coordination.GroupMemberStatusActive, OrderIndex: 1},
		{MemberID: "member-a", CharacterID: "role-a", RoleOwnerID: "owner", Status: coordination.GroupMemberStatusActive, OrderIndex: 0},
		{MemberID: "member-left", CharacterID: "role-c", RoleOwnerID: "owner", Status: coordination.GroupMemberStatusLeft, OrderIndex: 2},
	}}
	mentions, err := resolveRequestedGroupMentions(state, nil)
	if err != nil || len(mentions) != 1 || mentions[0].MemberID != "member-a" {
		t.Fatalf("未点名时未使用默认角色: %#v %v", mentions, err)
	}
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

func TestResolveGroupTextInvitationsUsesExactActiveMemberNames(t *testing.T) {
	state := coordination.GroupState{RoleOwnerID: "owner", Members: []coordination.GroupMember{
		{MemberID: "member-a", CharacterID: "role-a", RoleOwnerID: "owner", CharacterName: "大肥鱼", Status: coordination.GroupMemberStatusActive, OrderIndex: 0},
		{MemberID: "member-b", CharacterID: "role-b", RoleOwnerID: "owner", CharacterName: "大肥鱼 (副本)", Status: coordination.GroupMemberStatusActive, OrderIndex: 1},
		{MemberID: "member-c", CharacterID: "role-c", RoleOwnerID: "owner", CharacterName: "小林", Status: coordination.GroupMemberStatusActive, OrderIndex: 2},
		{MemberID: "member-left", CharacterID: "role-left", RoleOwnerID: "owner", CharacterName: "已离开", Status: coordination.GroupMemberStatusLeft, OrderIndex: 3},
	}}
	proposals := resolveGroupTextInvitations(state, "@大肥鱼 (副本) 你先说，再请 @大肥鱼 补充。@大肥鱼列表 和 @已离开 不算。", "role-a")
	if len(proposals) != 1 || proposals[0].TargetMemberID != "member-b" {
		t.Fatalf("最长成员名或边界解析错误: %#v", proposals)
	}
	proposals = resolveGroupTextInvitations(state, "@小林 请接话，@大肥鱼 (副本) 也来。", "role-a")
	if len(proposals) != 2 || proposals[0].TargetMemberID != "member-c" || proposals[1].TargetMemberID != "member-b" {
		t.Fatalf("多个正文点名解析错误: %#v", proposals)
	}
}

func TestResolveDiscussionMembersUsesDefaultRoleAndBoundedRoundRobin(t *testing.T) {
	state := coordination.GroupState{
		ConversationRoleID: "role-b",
		Settings:           coordination.GroupSettings{MaxAutoReplies: 5},
		Members: []coordination.GroupMember{
			{MemberID: "member-a", CharacterID: "role-a", Status: coordination.GroupMemberStatusActive, OrderIndex: 0},
			{MemberID: "member-b", CharacterID: "role-b", Status: coordination.GroupMemberStatusActive, OrderIndex: 1},
			{MemberID: "member-c", CharacterID: "role-c", Status: coordination.GroupMemberStatusActive, OrderIndex: 2},
			{MemberID: "member-left", CharacterID: "role-left", Status: coordination.GroupMemberStatusLeft, OrderIndex: 3},
		},
	}
	members, err := resolveDiscussionMembers(state)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(members))
	for _, member := range members {
		got = append(got, member.CharacterID)
	}
	want := []string{"role-b", "role-c", "role-a", "role-b", "role-c"}
	if len(got) != len(want) {
		t.Fatalf("讨论成员数量错误: %#v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("讨论成员顺序错误: got=%#v want=%#v", got, want)
		}
	}
	state.Settings.MaxAutoReplies = 1
	if _, err := resolveDiscussionMembers(state); err == nil {
		t.Fatal("讨论预算不足时未被拒绝")
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
