package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

type groupScopeIntegrationModel struct {
	*threeCoreMemoryModel
	engine  *business.Engine
	service *coordination.Service
	roles   coordination.DataPort
}

type failingOnceGroupModel struct {
	*groupScopeIntegrationModel
	mu     sync.Mutex
	failed bool
}

type blockingGroupModel struct {
	*groupScopeIntegrationModel
	started chan struct{}
	once    sync.Once
}

type autoInviteGroupModel struct {
	*groupScopeIntegrationModel
}

func (m *autoInviteGroupModel) OwnedReplyCommitted(ctx context.Context, inference business.Inference, _ business.Generation, messageID string) error {
	if inference.Group == nil || inference.Scope.RoleID != "one" || inference.Message != "自动接话" {
		return nil
	}
	member, ok := inference.Group.State.ActiveMember("two")
	if !ok {
		return nil
	}
	return m.engine.AcceptGroupInvitations(ctx, inference, messageID, []business.GroupInviteProposal{{InvitationID: "auto-two", TargetMemberID: member.MemberID, TargetCharacterID: member.CharacterID, TargetCharacterName: member.CharacterName}})
}

func (m *blockingGroupModel) GenerateOwnedReply(ctx context.Context, input business.Inference) (business.Generation, error) {
	if input.Message == "等待切换" {
		m.once.Do(func() { close(m.started) })
		<-ctx.Done()
		return business.Generation{}, ctx.Err()
	}
	return m.groupScopeIntegrationModel.GenerateOwnedReply(ctx, input)
}

func (m *failingOnceGroupModel) GenerateOwnedReply(ctx context.Context, input business.Inference) (business.Generation, error) {
	m.mu.Lock()
	shouldFail := input.Message == "触发群成员失败" && !m.failed
	if shouldFail {
		m.failed = true
	}
	m.mu.Unlock()
	if shouldFail {
		return business.Generation{}, errors.New("模拟群成员生成失败")
	}
	return m.groupScopeIntegrationModel.GenerateOwnedReply(ctx, input)
}

func (m *groupScopeIntegrationModel) GenerateOwnedReply(ctx context.Context, input business.Inference) (business.Generation, error) {
	if input.Scope.ConversationRoleID != "" {
		if err := m.engine.WithExecutionScope(ctx, input.Scope, func(context.Context) error { return nil }); err != nil {
			return business.Generation{}, err
		}
		restored, finish, err := m.service.Restore(ctx, input.Scope, m.roles)
		if err != nil {
			return business.Generation{}, err
		}
		defer finish()
		if err := coordination.ValidateCurrent(restored); err != nil {
			return business.Generation{}, err
		}
	}
	generation, err := m.threeCoreMemoryModel.GenerateOwnedReply(ctx, input)
	if err == nil && input.Scope.ConversationRoleID != "" {
		generation.StateEffects, err = json.Marshal(interaction.RuntimeAssembly{Appraisal: &interaction.AppraisalResult{EventType: "chat", NeedDeltas: map[string]float64{"connection": 0.1}}})
	}
	return generation, err
}

func TestGroupChatInvitesMemberPersistsOneInputAndRunsEachRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schemas := newThreeCoreSchemas(t)
	fixture := newThreeCoreFixture(t, "core-group", schemas)
	model := &groupScopeIntegrationModel{threeCoreMemoryModel: &threeCoreMemoryModel{core: fixture.core}, service: fixture.services.DeviceMesh.Coordination, roles: fixture.services.DeviceMesh}
	fixture.services.OwnedBusiness = business.NewEngine(model.service, model.roles, model)
	model.engine = fixture.services.OwnedBusiness
	caller := newThreeCoreFixture(t, "device-group", schemas)
	pairThreeCoreFixtures(t, caller, fixture)
	policy, err := fixture.services.DeviceMesh.Coordination.Get(t.Context(), fixture.core, caller.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, fixture, caller, policy.ModeRevision)
	var existing character.Character
	if err := fixture.services.DB.Where("id=?", "one").First(&existing).Error; err != nil {
		t.Fatal(err)
	}
	second := &character.Character{ID: "two", SpaceID: existing.SpaceID, Name: "第二角色", Status: "enabled", Revision: 1, PersonalityConfig: "{}", ChatStyleConfig: "{}", SceneRules: "{}", CardDataJSON: "{}"}
	var storedSecond character.Character
	if err := fixture.services.DB.Where("id=?", "two").First(&storedSecond).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		if err := fixture.services.DB.Create(second).Error; err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	firstTurn := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "group-first", "characterId": "one", "message": "建立会话"}, http.StatusOK)
	var first struct {
		Data business.Response `json:"data"`
	}
	if json.Unmarshal(firstTurn, &first) != nil || first.Data.ConversationID == "" {
		t.Fatal("初始单聊未建立")
	}
	invited := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+first.Data.ConversationID+"/members", map[string]any{"requestId": "invite-two", "characterId": "one", "expectedMembershipRevision": 1, "members": []map[string]any{{"characterId": "two", "shareHistory": false}}}, http.StatusOK)
	var invitation struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(invited, &invitation) != nil || !invitation.Data.Saved || invitation.Data.Group.ConversationType != "group" || len(invitation.Data.Group.Members) != 2 {
		t.Fatalf("邀请成员失败: %s", invited)
	}
	var firstMemberID, secondMemberID string
	for _, member := range invitation.Data.Group.Members {
		if member.CharacterID == "one" {
			firstMemberID = member.MemberID
		}
		if member.CharacterID == "two" {
			secondMemberID = member.MemberID
		}
	}
	if secondMemberID == "" {
		t.Fatal("第二角色缺少稳定成员编号")
	}
	turn := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "group-turn", "characterId": "one", "conversationId": first.Data.ConversationID, "message": "@第二角色 你好", "group": map[string]any{"expectedMembershipRevision": invitation.Data.Group.MembershipRevision, "expectedSettingsRevision": invitation.Data.Group.SettingsRevision}, "mentions": []map[string]any{{"memberId": secondMemberID, "characterId": "two"}}}, http.StatusOK)
	var result struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(turn, &result) != nil || !result.Data.Saved || result.Data.Status != business.GroupRoundCompleted || len(result.Data.Deliveries) != 1 || result.Data.Deliveries[0].Scope.RoleID != "two" {
		detail := ""
		if resource, err := coordination.NewOwnershipStore(fixture.services.KernelContainer.DeviceRegistry.Database(), fixture.core).Get(t.Context(), "group-round", result.Data.GroupRoundID); err == nil && resource != nil {
			detail = string(resource.Body)
			var stored business.GroupRound
			if json.Unmarshal(resource.Body, &stored) == nil {
				for _, delivery := range stored.Deliveries {
					if deliveryResource, getErr := coordination.NewOwnershipStore(fixture.services.KernelContainer.DeviceRegistry.Database(), fixture.core).Get(t.Context(), "group-delivery", delivery.DeliveryID); getErr == nil && deliveryResource != nil {
						detail += " " + string(deliveryResource.Body)
					}
				}
			}
		}
		t.Fatalf("群聊执行失败: %s %s status=%s", turn, detail, result.Data.Status)
	}
	roundResource, roundErr := coordination.NewOwnershipStore(fixture.services.KernelContainer.DeviceRegistry.Database(), fixture.core).Get(t.Context(), "group-round", result.Data.GroupRoundID)
	var storedRound business.GroupRound
	if roundErr != nil || roundResource == nil || json.Unmarshal(roundResource.Body, &storedRound) != nil || len(storedRound.Deliveries) != 1 || storedRound.Deliveries[0].Status != business.GroupDeliveryCompleted {
		t.Fatalf("群聊轮次未保存最终投递状态: %+v", storedRound)
	}
	manual := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+first.Data.ConversationID+"/reply-requests", map[string]any{"requestId": "group-manual", "characterId": "one", "sourceMessageId": result.Data.UserMessageID, "members": []map[string]any{{"memberId": secondMemberID, "characterId": "two"}}}, http.StatusOK)
	var manualResult struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(manual, &manualResult) != nil || !manualResult.Data.Saved || manualResult.Data.UserMessageID != result.Data.UserMessageID || len(manualResult.Data.Deliveries) != 1 || manualResult.Data.Deliveries[0].Scope.RoleID != "two" {
		t.Fatalf("手动点名未复用原用户消息: %s", manual)
	}
	all := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "group-all", "characterId": "one", "conversationId": first.Data.ConversationID, "message": "@所有成员 你好", "group": map[string]any{"expectedMembershipRevision": invitation.Data.Group.MembershipRevision, "expectedSettingsRevision": invitation.Data.Group.SettingsRevision}, "mentions": []map[string]any{{"memberId": firstMemberID, "characterId": "one"}, {"memberId": secondMemberID, "characterId": "two"}}}, http.StatusOK)
	var allResult struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(all, &allResult) != nil || allResult.Data.Status != business.GroupRoundCompleted || len(allResult.Data.Deliveries) != 2 || allResult.Data.Deliveries[0].Scope.RoleID == allResult.Data.Deliveries[1].Scope.RoleID {
		t.Fatalf("所有成员未分别回复: %s", all)
	}
	view := fixture.request(t, caller, http.MethodGet, "/api/device-mesh/v1/business/conversations/"+first.Data.ConversationID+"?characterId=one", nil, http.StatusOK)
	var snapshot struct {
		Data business.QueryResult `json:"data"`
	}
	if json.Unmarshal(view, &snapshot) != nil || snapshot.Data.Snapshot.Group == nil || len(snapshot.Data.Snapshot.Group.Members) != 2 {
		t.Fatalf("群聊快照缺失: %s", view)
	}
	userMessages := 0
	assistantMessages := 0
	for _, resource := range snapshot.Data.Snapshot.Resources {
		if resource.Kind != "message" {
			continue
		}
		var message struct {
			Role string `json:"role"`
		}
		_ = json.Unmarshal(resource.Body, &message)
		if message.Role == "user" {
			userMessages++
		}
		if message.Role == "assistant" {
			assistantMessages++
		}
	}
	if userMessages != 3 || assistantMessages != 5 {
		t.Fatalf("消息身份或数量错误: users=%d assistants=%d", userMessages, assistantMessages)
	}
}

func TestCreateEmptyGroupConversationWithoutModelMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schemas := newThreeCoreSchemas(t)
	fixture := newThreeCoreFixture(t, "core-new-group", schemas)
	caller := newThreeCoreFixture(t, "device-new-group", schemas)
	pairThreeCoreFixtures(t, caller, fixture)
	policy, err := fixture.services.DeviceMesh.Coordination.Get(t.Context(), fixture.core, caller.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, fixture, caller, policy.ModeRevision)
	created := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "new-empty-group", "characterId": "one", "members": []map[string]any{{"characterId": "two", "shareHistory": false}}}, http.StatusOK)
	var response struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(created, &response) != nil || !response.Data.Saved || response.Data.Group.ConversationType != "group" || len(response.Data.Group.Members) != 2 {
		t.Fatalf("创建空群聊失败: %s", created)
	}
	view := fixture.request(t, caller, http.MethodGet, "/api/device-mesh/v1/business/conversations/"+response.Data.Group.ConversationID+"?characterId=one", nil, http.StatusOK)
	var snapshot struct {
		Data business.QueryResult `json:"data"`
	}
	if json.Unmarshal(view, &snapshot) != nil || snapshot.Data.Snapshot.Group == nil || len(snapshot.Data.Snapshot.Group.Members) != 2 {
		t.Fatalf("新群聊快照无效: %s", view)
	}
	for _, resource := range snapshot.Data.Snapshot.Resources {
		if resource.Kind == "message" {
			t.Fatal("创建空群聊不应生成用户消息")
		}
	}
}

func TestGroupChatLifecycleValidationAndIdempotency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schemas := newThreeCoreSchemas(t)
	fixture := newThreeCoreFixture(t, "core-group-matrix", schemas)
	caller := newThreeCoreFixture(t, "device-group-matrix", schemas)
	pairThreeCoreFixtures(t, caller, fixture)
	policy, err := fixture.services.DeviceMesh.Coordination.Get(t.Context(), fixture.core, caller.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, fixture, caller, policy.ModeRevision)
	roleIDs := []string{"two", "three", "four", "five", "six", "seven", "eight", "nine"}
	for _, roleID := range roleIDs {
		ensureGroupCharacter(t, fixture, roleID, "群聊角色 "+roleID, "enabled")
	}

	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "group-invalid-self", "characterId": "one", "members": []map[string]any{{"characterId": "one"}}}, http.StatusConflict)
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "group-invalid-unknown", "characterId": "one", "members": []map[string]any{{"characterId": "missing"}}}, http.StatusConflict)
	ensureGroupCharacter(t, fixture, "disabled", "禁用角色", "disabled")
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "group-invalid-disabled", "characterId": "one", "members": []map[string]any{{"characterId": "disabled"}}}, http.StatusConflict)

	duplicate := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "group-create-duplicate", "characterId": "one", "members": []map[string]any{{"characterId": "two"}, {"characterId": "two"}}}, http.StatusOK)
	var duplicateResult struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(duplicate, &duplicateResult) != nil || !duplicateResult.Data.Saved || len(duplicateResult.Data.Group.Members) != 2 {
		t.Fatalf("重复创建成员未正确去重: %s", duplicate)
	}

	maxMembers := make([]map[string]any, 0, 7)
	for _, roleID := range roleIDs[:7] {
		maxMembers = append(maxMembers, map[string]any{"characterId": roleID, "shareHistory": false})
	}
	createdRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "group-create-max", "characterId": "one", "members": maxMembers}, http.StatusOK)
	var created struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(createdRaw, &created) != nil || !created.Data.Saved || len(created.Data.Group.Members) != coordination.GroupMaxMembers {
		t.Fatalf("八人群聊创建失败: %s", createdRaw)
	}
	conversationID := created.Data.Group.ConversationID
	overLimit := append(maxMembers, map[string]any{"characterId": "nine", "shareHistory": false})
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "group-create-over-limit", "characterId": "one", "members": overLimit}, http.StatusConflict)

	memberByRole := make(map[string]coordination.GroupMember)
	for _, member := range created.Data.Group.Members {
		memberByRole[member.CharacterID] = member
	}
	repeatedInvite := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+conversationID+"/members", map[string]any{"requestId": "group-invite-existing", "characterId": "one", "expectedMembershipRevision": 1, "members": []map[string]any{{"characterId": "two"}}}, http.StatusOK)
	var repeated struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(repeatedInvite, &repeated) != nil || !repeated.Data.Saved || repeated.Data.Group.MembershipRevision != 1 {
		t.Fatalf("重复邀请不应改变成员版本: %s", repeatedInvite)
	}
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+conversationID+"/members", map[string]any{"requestId": "group-invite-stale", "characterId": "one", "expectedMembershipRevision": 0, "members": []map[string]any{{"characterId": "two"}}}, http.StatusConflict)
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+conversationID+"/members", map[string]any{"requestId": "group-invite-unknown", "characterId": "one", "expectedMembershipRevision": 1, "members": []map[string]any{{"characterId": "missing"}}}, http.StatusConflict)

	activeIDs := make([]string, 0, len(memberByRole))
	for index := len(maxMembers) - 1; index >= 0; index-- {
		activeIDs = append(activeIDs, memberByRole[roleIDs[index]].MemberID)
	}
	activeIDs = append(activeIDs, memberByRole["one"].MemberID)
	for name, order := range map[string][]string{
		"unknown":   {"missing"},
		"duplicate": {activeIDs[0], activeIDs[0], activeIDs[1], activeIDs[2], activeIDs[3], activeIDs[4], activeIDs[5], activeIDs[6]},
		"missing":   activeIDs[:7],
	} {
		fixture.request(t, caller, http.MethodPatch, "/api/device-mesh/v1/business/conversations/"+conversationID+"/group-settings", map[string]any{"requestId": "group-settings-" + name, "characterId": "one", "expectedSettingsRevision": 1, "memberOrder": order}, http.StatusConflict)
	}
	reorderedRaw := fixture.request(t, caller, http.MethodPatch, "/api/device-mesh/v1/business/conversations/"+conversationID+"/group-settings", map[string]any{"requestId": "group-settings-valid-order", "characterId": "one", "expectedSettingsRevision": 1, "memberOrder": activeIDs}, http.StatusOK)
	var reordered struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(reorderedRaw, &reordered) != nil || reordered.Data.Group.SettingsRevision != 2 || activeMemberOrder(reordered.Data.Group.Members) != strings.Join(activeIDs, ",") {
		t.Fatalf("合法成员排序未生效: %s", reorderedRaw)
	}
	noopRaw := fixture.request(t, caller, http.MethodPatch, "/api/device-mesh/v1/business/conversations/"+conversationID+"/group-settings", map[string]any{"requestId": "group-settings-noop", "characterId": "one", "expectedSettingsRevision": 2, "memberOrder": activeIDs, "allowCharacterMentions": false}, http.StatusOK)
	var noop struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(noopRaw, &noop) != nil || noop.Data.Group.SettingsRevision != 2 {
		t.Fatalf("无变化设置不应改变版本: %s", noopRaw)
	}
	fixture.request(t, caller, http.MethodPatch, "/api/device-mesh/v1/business/conversations/"+conversationID+"/group-settings", map[string]any{"requestId": "group-settings-stale", "characterId": "one", "expectedSettingsRevision": 1, "name": "旧版本改名"}, http.StatusConflict)
	fixture.request(t, caller, http.MethodPatch, "/api/device-mesh/v1/business/conversations/"+conversationID+"/group-settings", map[string]any{"requestId": "group-settings-max-invalid", "characterId": "one", "expectedSettingsRevision": 2, "maxAutoReplies": coordination.GroupDefaultAutoReplies + 1}, http.StatusConflict)
	enabledRaw := fixture.request(t, caller, http.MethodPatch, "/api/device-mesh/v1/business/conversations/"+conversationID+"/group-settings", map[string]any{"requestId": "group-settings-enable", "characterId": "one", "expectedSettingsRevision": 2, "allowCharacterMentions": true}, http.StatusOK)
	var enabled struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(enabledRaw, &enabled) != nil || enabled.Data.Group.SettingsRevision != 3 || !enabled.Data.Group.Settings.AllowCharacterMentions {
		t.Fatalf("角色互相接话开关未生效: %s", enabledRaw)
	}
	chineseTitle := strings.Repeat("群", 80)
	renamedRaw := fixture.request(t, caller, http.MethodPatch, "/api/device-mesh/v1/business/conversations/"+conversationID+"/group-settings", map[string]any{"requestId": "group-settings-chinese-name", "characterId": "one", "expectedSettingsRevision": 3, "name": chineseTitle}, http.StatusOK)
	var renamed struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(renamedRaw, &renamed) != nil || renamed.Data.Group.SettingsRevision != 4 || renamed.Data.Group.Title != chineseTitle {
		t.Fatalf("中文字符群名被误判为超长: %s", renamedRaw)
	}

	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "group-empty-message", "characterId": "one", "conversationId": conversationID, "message": "   ", "group": map[string]any{"expectedMembershipRevision": 1, "expectedSettingsRevision": 4}, "mentions": []map[string]any{}}, http.StatusConflict)
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "group-long-message", "characterId": "one", "conversationId": conversationID, "message": strings.Repeat("长", (128<<10)+1), "group": map[string]any{"expectedMembershipRevision": 1, "expectedSettingsRevision": 4}, "mentions": []map[string]any{}}, http.StatusConflict)

	turnPayload := map[string]any{"requestId": "group-idempotent-turn", "characterId": "one", "conversationId": conversationID, "message": "@群聊角色 two 你好", "group": map[string]any{"expectedMembershipRevision": 1, "expectedSettingsRevision": 4}, "mentions": []map[string]any{{"memberId": memberByRole["two"].MemberID, "characterId": "two"}, {"memberId": memberByRole["two"].MemberID, "characterId": "two"}}}
	turnRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", turnPayload, http.StatusOK)
	var turn struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(turnRaw, &turn) != nil || !turn.Data.Saved || turn.Data.Status != business.GroupRoundCompleted || len(turn.Data.Deliveries) != 1 || turn.Data.Deliveries[0].Scope.RoleID != "two" {
		t.Fatalf("群消息或重复点名处理失败: %s", turnRaw)
	}
	replayedRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", turnPayload, http.StatusOK)
	var replayed struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(replayedRaw, &replayed) != nil || replayed.Data.GroupRoundID != turn.Data.GroupRoundID || replayed.Data.Status != business.GroupRoundCompleted || len(replayed.Data.Deliveries) != 0 {
		t.Fatalf("相同请求重放产生了重复回复: %s", replayedRaw)
	}
	changedPayload := map[string]any{"requestId": turnPayload["requestId"], "characterId": "one", "conversationId": conversationID, "message": "被替换的消息", "group": turnPayload["group"], "mentions": turnPayload["mentions"]}
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", changedPayload, http.StatusConflict)

	removedThree := fixture.request(t, caller, http.MethodDelete, "/api/device-mesh/v1/business/conversations/"+conversationID+"/members/"+memberByRole["three"].MemberID, map[string]any{"requestId": "group-remove-three", "characterId": "one", "expectedMembershipRevision": 1}, http.StatusOK)
	var removed struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(removedThree, &removed) != nil || removed.Data.Group.MembershipRevision != 2 {
		t.Fatalf("移除成员失败: %s", removedThree)
	}
	leftThreeMemberID := memberByRole["three"].MemberID
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "group-mention-left", "characterId": "one", "conversationId": conversationID, "message": "@群聊角色 three 你好", "group": map[string]any{"expectedMembershipRevision": 2, "expectedSettingsRevision": 4}, "mentions": []map[string]any{{"memberId": leftThreeMemberID, "characterId": "three"}}}, http.StatusConflict)
	reinvitedRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+conversationID+"/members", map[string]any{"requestId": "group-reinvite-three", "characterId": "one", "expectedMembershipRevision": 2, "members": []map[string]any{{"characterId": "three", "shareHistory": false}}}, http.StatusOK)
	var reinvited struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(reinvitedRaw, &reinvited) != nil || reinvited.Data.Group.MembershipRevision != 3 {
		t.Fatalf("重新邀请已离开角色失败: %s", reinvitedRaw)
	}
	orderIDs := make(map[int]bool)
	for _, member := range reinvited.Data.Group.Members {
		if member.Status == coordination.GroupMemberStatusActive {
			if orderIDs[member.OrderIndex] {
				t.Fatalf("重新邀请后出现重复成员顺序: %+v", reinvited.Data.Group.Members)
			}
			orderIDs[member.OrderIndex] = true
		}
		if member.CharacterID == "three" && member.Status == coordination.GroupMemberStatusActive {
			memberByRole["three"] = member
		}
	}
	fixture.request(t, caller, http.MethodDelete, "/api/device-mesh/v1/business/conversations/"+conversationID+"/members/"+leftThreeMemberID, map[string]any{"requestId": "group-remove-three-again", "characterId": "one", "expectedMembershipRevision": 3}, http.StatusConflict)
	fixture.request(t, caller, http.MethodDelete, "/api/device-mesh/v1/business/conversations/"+conversationID+"/members/"+memberByRole["three"].MemberID, map[string]any{"requestId": "group-remove-reinvited-three", "characterId": "one", "expectedMembershipRevision": 3}, http.StatusOK)
	for index, roleID := range []string{"two", "four", "five", "six", "seven", "eight"} {
		currentRevision := index + 4
		fixture.request(t, caller, http.MethodDelete, "/api/device-mesh/v1/business/conversations/"+conversationID+"/members/"+memberByRole[roleID].MemberID, map[string]any{"requestId": "group-remove-" + roleID, "characterId": "one", "expectedMembershipRevision": currentRevision}, http.StatusOK)
	}
	fixture.request(t, caller, http.MethodDelete, "/api/device-mesh/v1/business/conversations/"+conversationID+"/members/"+memberByRole["one"].MemberID, map[string]any{"requestId": "group-remove-last", "characterId": "one", "expectedMembershipRevision": 10}, http.StatusConflict)

	historyConversationID := duplicateResult.Data.Group.ConversationID
	var duplicateTwo coordination.GroupMember
	for _, member := range duplicateResult.Data.Group.Members {
		if member.CharacterID == "two" {
			duplicateTwo = member
		}
	}
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "group-history-baseline", "characterId": "one", "conversationId": historyConversationID, "message": "@群聊角色 two 历史基线", "group": map[string]any{"expectedMembershipRevision": 1, "expectedSettingsRevision": 1}, "mentions": []map[string]any{{"memberId": duplicateTwo.MemberID, "characterId": "two"}}}, http.StatusOK)
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+historyConversationID+"/members", map[string]any{"requestId": "group-history-hidden", "characterId": "one", "expectedMembershipRevision": 1, "members": []map[string]any{{"characterId": "three", "shareHistory": false}}}, http.StatusOK)
	hiddenView := fixture.request(t, caller, http.MethodGet, "/api/device-mesh/v1/business/conversations/"+historyConversationID+"?characterId=three", nil, http.StatusOK)
	var hiddenSnapshot struct {
		Data business.QueryResult `json:"data"`
	}
	if json.Unmarshal(hiddenView, &hiddenSnapshot) != nil || countConversationMessages(hiddenSnapshot.Data.Snapshot.Resources) != 0 {
		t.Fatalf("未共享历史的新成员仍看到旧消息: %s", hiddenView)
	}
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+historyConversationID+"/members", map[string]any{"requestId": "group-history-visible", "characterId": "one", "expectedMembershipRevision": 2, "members": []map[string]any{{"characterId": "four", "shareHistory": true}}}, http.StatusOK)
	visibleView := fixture.request(t, caller, http.MethodGet, "/api/device-mesh/v1/business/conversations/"+historyConversationID+"?characterId=four", nil, http.StatusOK)
	var visibleSnapshot struct {
		Data business.QueryResult `json:"data"`
	}
	if json.Unmarshal(visibleView, &visibleSnapshot) != nil || countConversationMessages(visibleSnapshot.Data.Snapshot.Resources) != 2 {
		t.Fatalf("共享历史的新成员未看到旧消息: %s", visibleView)
	}
}

func TestGroupChatInterruptPersistsCancelledRound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schemas := newThreeCoreSchemas(t)
	fixture := newThreeCoreFixture(t, "core-group-cancel", schemas)
	baseModel := &groupScopeIntegrationModel{threeCoreMemoryModel: &threeCoreMemoryModel{core: fixture.core}, service: fixture.services.DeviceMesh.Coordination, roles: fixture.services.DeviceMesh}
	model := &blockingGroupModel{groupScopeIntegrationModel: baseModel, started: make(chan struct{})}
	fixture.services.OwnedBusiness = business.NewEngine(model.service, model.roles, model)
	model.engine = fixture.services.OwnedBusiness
	caller := newThreeCoreFixture(t, "device-group-cancel", schemas)
	pairThreeCoreFixtures(t, caller, fixture)
	policy, err := fixture.services.DeviceMesh.Coordination.Get(t.Context(), fixture.core, caller.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, fixture, caller, policy.ModeRevision)
	ensureGroupCharacter(t, fixture, "two", "可中断角色", "enabled")
	createdRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "group-cancel-create", "characterId": "one", "members": []map[string]any{{"characterId": "two"}}}, http.StatusOK)
	var created struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(createdRaw, &created) != nil || !created.Data.Saved {
		t.Fatalf("创建取消测试群聊失败: %s", createdRaw)
	}
	var secondMember coordination.GroupMember
	for _, member := range created.Data.Group.Members {
		if member.CharacterID == "two" {
			secondMember = member
		}
	}
	requestID := "group-cancel-running"
	interrupted := make(chan bool, 1)
	go func() {
		<-model.started
		credential, loadErr := caller.local.LoadCredential()
		if loadErr != nil {
			interrupted <- false
			return
		}
		interrupted <- fixture.services.OwnedBusiness.Interrupt(credential.SpaceID.String(), caller.device.DeviceID.String(), requestID)
	}()
	fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": requestID, "characterId": "one", "conversationId": created.Data.Group.ConversationID, "message": "等待切换", "group": map[string]any{"expectedMembershipRevision": 1, "expectedSettingsRevision": 1}, "mentions": []map[string]any{{"memberId": secondMember.MemberID, "characterId": "two"}}}, http.StatusConflict)
	if !<-interrupted {
		t.Fatal("运行中的群聊轮次未被中断")
	}
	view := fixture.request(t, caller, http.MethodGet, "/api/device-mesh/v1/business/conversations/"+created.Data.Group.ConversationID+"?characterId=one", nil, http.StatusOK)
	var snapshot struct {
		Data business.QueryResult `json:"data"`
	}
	if json.Unmarshal(view, &snapshot) != nil {
		t.Fatalf("取消后群聊快照无效: %s", view)
	}
	found := false
	for _, resource := range snapshot.Data.Snapshot.Resources {
		if resource.Kind != "group-round" {
			continue
		}
		var round business.GroupRound
		if json.Unmarshal(resource.Body, &round) != nil || round.RootRequestID != requestID {
			continue
		}
		found = true
		if round.Status != business.GroupRoundCancelled || len(round.Deliveries) != 1 || round.Deliveries[0].Status != business.GroupDeliveryCancelled {
			t.Fatalf("中断后的轮次状态错误: %+v", round)
		}
	}
	if !found {
		t.Fatal("未找到已中断的群聊轮次")
	}
}

func TestGroupChatRetryIsIdempotentAndKeepsSingleReply(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schemas := newThreeCoreSchemas(t)
	fixture := newThreeCoreFixture(t, "core-group-retry", schemas)
	baseModel := &groupScopeIntegrationModel{threeCoreMemoryModel: &threeCoreMemoryModel{core: fixture.core}, service: fixture.services.DeviceMesh.Coordination, roles: fixture.services.DeviceMesh}
	model := &failingOnceGroupModel{groupScopeIntegrationModel: baseModel}
	fixture.services.OwnedBusiness = business.NewEngine(baseModel.service, baseModel.roles, model)
	baseModel.engine = fixture.services.OwnedBusiness
	caller := newThreeCoreFixture(t, "device-group-retry", schemas)
	pairThreeCoreFixtures(t, caller, fixture)
	policy, err := fixture.services.DeviceMesh.Coordination.Get(t.Context(), fixture.core, caller.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, fixture, caller, policy.ModeRevision)
	ensureGroupCharacter(t, fixture, "two", "重试角色", "enabled")
	createdRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "group-retry-create", "characterId": "one", "members": []map[string]any{{"characterId": "two"}}}, http.StatusOK)
	var created struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(createdRaw, &created) != nil || !created.Data.Saved {
		t.Fatalf("创建重试测试群聊失败: %s", createdRaw)
	}
	var secondMember coordination.GroupMember
	for _, member := range created.Data.Group.Members {
		if member.CharacterID == "two" {
			secondMember = member
		}
	}
	failedRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "group-retry-original", "characterId": "one", "conversationId": created.Data.Group.ConversationID, "message": "触发群成员失败", "group": map[string]any{"expectedMembershipRevision": 1, "expectedSettingsRevision": 1}, "mentions": []map[string]any{{"memberId": secondMember.MemberID, "characterId": "two"}}}, http.StatusOK)
	var failedTurn struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(failedRaw, &failedTurn) != nil || failedTurn.Data.Status != business.GroupRoundCompletedWithErrors || len(failedTurn.Data.Deliveries) != 0 {
		t.Fatalf("失败投递未正确返回: %s", failedRaw)
	}
	roundResource, err := coordination.NewOwnershipStore(fixture.services.KernelContainer.DeviceRegistry.Database(), fixture.core).Get(t.Context(), "group-round", failedTurn.Data.GroupRoundID)
	var failedRound business.GroupRound
	if err != nil || roundResource == nil || json.Unmarshal(roundResource.Body, &failedRound) != nil || len(failedRound.Deliveries) != 1 || failedRound.Deliveries[0].Status != business.GroupDeliveryFailed {
		t.Fatalf("失败投递未持久化: %+v %v", failedRound, err)
	}
	retryPayload := map[string]any{"requestId": "group-retry-once", "characterId": "one", "deliveryId": failedRound.Deliveries[0].DeliveryID}
	retriedRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+created.Data.Group.ConversationID+"/group-rounds/retry?roundId="+failedTurn.Data.GroupRoundID, retryPayload, http.StatusOK)
	var retried struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(retriedRaw, &retried) != nil || retried.Data.Status != business.GroupRoundCompleted || len(retried.Data.Deliveries) != 1 || retried.Data.Deliveries[0].Scope.RoleID != "two" {
		t.Fatalf("失败投递重试未成功: %s", retriedRaw)
	}
	replayedRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+created.Data.Group.ConversationID+"/group-rounds/retry?roundId="+failedTurn.Data.GroupRoundID, retryPayload, http.StatusOK)
	var replayed struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(replayedRaw, &replayed) != nil || replayed.Data.GroupRoundID != retried.Data.GroupRoundID || replayed.Data.Status != business.GroupRoundCompleted || len(replayed.Data.Deliveries) != 0 {
		t.Fatalf("重试请求重放产生了重复回复: %s", replayedRaw)
	}
	view := fixture.request(t, caller, http.MethodGet, "/api/device-mesh/v1/business/conversations/"+created.Data.Group.ConversationID+"?characterId=one", nil, http.StatusOK)
	var snapshot struct {
		Data business.QueryResult `json:"data"`
	}
	if json.Unmarshal(view, &snapshot) != nil {
		t.Fatalf("重试后群聊快照无效: %s", view)
	}
	assistantMessages := 0
	for _, resource := range snapshot.Data.Snapshot.Resources {
		if resource.Kind != "message" {
			continue
		}
		var message struct {
			Role string `json:"role"`
		}
		if json.Unmarshal(resource.Body, &message) == nil && message.Role == "assistant" {
			assistantMessages++
		}
	}
	if assistantMessages != 1 {
		t.Fatalf("重试幂等失败，助手消息数=%d", assistantMessages)
	}
}

func TestGroupChatAutoInvitationRunsAppendedDelivery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schemas := newThreeCoreSchemas(t)
	fixture := newThreeCoreFixture(t, "core-group-auto", schemas)
	baseModel := &groupScopeIntegrationModel{threeCoreMemoryModel: &threeCoreMemoryModel{core: fixture.core}, service: fixture.services.DeviceMesh.Coordination, roles: fixture.services.DeviceMesh}
	model := &autoInviteGroupModel{groupScopeIntegrationModel: baseModel}
	fixture.services.OwnedBusiness = business.NewEngine(baseModel.service, baseModel.roles, model)
	baseModel.engine = fixture.services.OwnedBusiness
	caller := newThreeCoreFixture(t, "device-group-auto", schemas)
	pairThreeCoreFixtures(t, caller, fixture)
	policy, err := fixture.services.DeviceMesh.Coordination.Get(t.Context(), fixture.core, caller.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, fixture, caller, policy.ModeRevision)
	ensureGroupCharacter(t, fixture, "two", "自动接话角色", "enabled")
	createdRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/group", map[string]any{"requestId": "group-auto-create", "characterId": "one", "members": []map[string]any{{"characterId": "two"}}}, http.StatusOK)
	var created struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(createdRaw, &created) != nil || !created.Data.Saved {
		t.Fatalf("创建自动接话测试群聊失败: %s", createdRaw)
	}
	var firstMember coordination.GroupMember
	var secondMember coordination.GroupMember
	for _, member := range created.Data.Group.Members {
		if member.CharacterID == "one" {
			firstMember = member
		}
		if member.CharacterID == "two" {
			secondMember = member
		}
	}
	settingsRaw := fixture.request(t, caller, http.MethodPatch, "/api/device-mesh/v1/business/conversations/"+created.Data.Group.ConversationID+"/group-settings", map[string]any{"requestId": "group-auto-enable", "characterId": "one", "expectedSettingsRevision": 1, "allowCharacterMentions": true}, http.StatusOK)
	var settings struct {
		Data business.GroupMutationResponse `json:"data"`
	}
	if json.Unmarshal(settingsRaw, &settings) != nil || settings.Data.Group.SettingsRevision != 2 {
		t.Fatalf("自动接话设置失败: %s", settingsRaw)
	}
	turnRaw := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "group-auto-turn", "characterId": "one", "conversationId": created.Data.Group.ConversationID, "message": "自动接话", "group": map[string]any{"expectedMembershipRevision": 1, "expectedSettingsRevision": 2}, "mentions": []map[string]any{{"memberId": firstMember.MemberID, "characterId": "one"}, {"memberId": secondMember.MemberID, "characterId": "two"}}}, http.StatusOK)
	var turn struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(turnRaw, &turn) != nil || turn.Data.Status != business.GroupRoundCompleted || len(turn.Data.Deliveries) != 2 {
		t.Fatalf("自动接话新增投递未执行: %s", turnRaw)
	}
	roles := map[string]bool{}
	for _, delivery := range turn.Data.Deliveries {
		roles[delivery.Scope.RoleID] = true
	}
	if !roles["one"] || !roles["two"] {
		t.Fatalf("自动接话角色回复不完整: %+v", roles)
	}
	roundResource, err := coordination.NewOwnershipStore(fixture.services.KernelContainer.DeviceRegistry.Database(), fixture.core).Get(t.Context(), "group-round", turn.Data.GroupRoundID)
	var round business.GroupRound
	if err != nil || roundResource == nil || json.Unmarshal(roundResource.Body, &round) != nil || round.Status != business.GroupRoundCompleted || len(round.Deliveries) != 2 {
		t.Fatalf("自动接话轮次未完成持久化: %+v %v", round, err)
	}
	for _, delivery := range round.Deliveries {
		if delivery.Status != business.GroupDeliveryCompleted {
			t.Fatalf("自动接话投递未完成: %+v", delivery)
		}
	}
}

func ensureGroupCharacter(t *testing.T, fixture *threeCoreFixture, id, name, status string) {
	t.Helper()
	var existing character.Character
	err := fixture.services.DB.Where("id=?", id).First(&existing).Error
	if err == nil {
		if existing.Status != status || existing.Name != name {
			if updateErr := fixture.services.DB.Model(&existing).Updates(map[string]any{"status": status, "name": name}).Error; updateErr != nil {
				t.Fatal(updateErr)
			}
		}
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal(err)
	}
	var base character.Character
	if err := fixture.services.DB.Where("id=?", "one").First(&base).Error; err != nil {
		t.Fatal(err)
	}
	created := &character.Character{ID: id, SpaceID: base.SpaceID, Name: name, Status: status, Revision: 1, PersonalityConfig: "{}", ChatStyleConfig: "{}", SceneRules: "{}", CardDataJSON: "{}"}
	if err := fixture.services.DB.Create(created).Error; err != nil {
		t.Fatal(err)
	}
}

func activeMemberOrder(members []coordination.GroupMember) string {
	active := make([]coordination.GroupMember, 0, len(members))
	for _, member := range members {
		if member.Status == coordination.GroupMemberStatusActive {
			active = append(active, member)
		}
	}
	for left := 0; left < len(active); left++ {
		for right := left + 1; right < len(active); right++ {
			if active[right].OrderIndex < active[left].OrderIndex {
				active[left], active[right] = active[right], active[left]
			}
		}
	}
	ids := make([]string, 0, len(active))
	for _, member := range active {
		ids = append(ids, member.MemberID)
	}
	return strings.Join(ids, ",")
}

func countConversationMessages(resources []coordination.Resource) int {
	count := 0
	for _, resource := range resources {
		if resource.Kind != "message" {
			continue
		}
		var message struct {
			Role string `json:"role"`
		}
		if json.Unmarshal(resource.Body, &message) == nil && (message.Role == "user" || message.Role == "assistant") {
			count++
		}
	}
	return count
}
