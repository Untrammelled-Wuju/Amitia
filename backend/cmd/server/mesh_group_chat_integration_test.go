package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

func TestGroupChatInvitesMemberPersistsOneInputAndRunsEachRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schemas := newThreeCoreSchemas(t)
	fixture := newThreeCoreFixture(t, "core-group", schemas)
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
	var secondMemberID string
	for _, member := range invitation.Data.Group.Members {
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
	manual := fixture.request(t, caller, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+first.Data.ConversationID+"/reply-requests", map[string]any{"requestId": "group-manual", "characterId": "one", "sourceMessageId": result.Data.UserMessageID, "members": []map[string]any{{"memberId": secondMemberID, "characterId": "two"}}}, http.StatusOK)
	var manualResult struct {
		Data business.GroupResponse `json:"data"`
	}
	if json.Unmarshal(manual, &manualResult) != nil || !manualResult.Data.Saved || manualResult.Data.UserMessageID != result.Data.UserMessageID || len(manualResult.Data.Deliveries) != 1 || manualResult.Data.Deliveries[0].Scope.RoleID != "two" {
		t.Fatalf("手动点名未复用原用户消息: %s", manual)
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
	if userMessages != 2 || assistantMessages != 3 {
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
