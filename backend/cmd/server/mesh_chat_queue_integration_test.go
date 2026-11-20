package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"gorm.io/gorm"
)

func TestQueuedGroupMessageUsesGroupExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schemas := newThreeCoreSchemas(t)
	center := newThreeCoreFixture(t, "queue-group-core", schemas)
	child := newThreeCoreFixture(t, "queue-group-device", schemas)
	model := &groupScopeIntegrationModel{threeCoreMemoryModel: &threeCoreMemoryModel{core: center.core}, service: center.services.DeviceMesh.Coordination, roles: center.services.DeviceMesh}
	center.services.OwnedBusiness = business.NewEngine(model.service, model.roles, model)
	model.engine = center.services.OwnedBusiness
	pairThreeCoreFixtures(t, child, center)
	policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, child.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, center, child, policy.ModeRevision)
	var firstRole character.Character
	if err := center.services.DB.Where("id=?", "one").First(&firstRole).Error; err != nil {
		t.Fatal(err)
	}
	second := &character.Character{ID: "two", SpaceID: firstRole.SpaceID, Name: "第二角色", Status: "enabled", Revision: 1, PersonalityConfig: "{}", ChatStyleConfig: "{}", SceneRules: "{}", CardDataJSON: "{}"}
	var storedSecond character.Character
	if err := center.services.DB.Where("id=?", "two").First(&storedSecond).Error; err == gorm.ErrRecordNotFound {
		if err := center.services.DB.Create(second).Error; err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	const prefix = "/api/device-mesh/v1/business"
	created := decodeThreeCoreMemoryResponse[business.Response](t, center.request(t, child, http.MethodPost, prefix+"/messages", map[string]any{"requestId": "queue-group-start", "characterId": "one", "message": "建立会话"}, http.StatusOK))
	group := decodeThreeCoreMemoryResponse[business.GroupMutationResponse](t, center.request(t, child, http.MethodPost, prefix+"/conversations/"+created.ConversationID+"/members", map[string]any{"requestId": "queue-group-invite", "characterId": "one", "expectedMembershipRevision": 1, "members": []map[string]any{{"characterId": "two", "shareHistory": false}}}, http.StatusOK))
	group = decodeThreeCoreMemoryResponse[business.GroupMutationResponse](t, center.request(t, child, http.MethodPatch, prefix+"/conversations/"+created.ConversationID+"/group-settings", map[string]any{"requestId": "queue-group-enable-discussion", "characterId": "one", "expectedSettingsRevision": group.Group.SettingsRevision, "allowCharacterMentions": true}, http.StatusOK))
	path := prefix + "/conversations/" + url.PathEscape(created.ConversationID) + "?characterId=one&conversationOwnerId=" + url.QueryEscape(child.device.DeviceID.String())
	view := decodeThreeCoreMemoryResponse[business.QueryResult](t, center.request(t, child, http.MethodGet, path, nil, http.StatusOK))
	base := map[string]any{"requestId": "queue-group-message", "characterId": "one", "conversationId": created.ConversationID, "targetDeviceId": child.device.DeviceID.String(), "expectedExecutionScope": view.Scope}
	base["message"] = "群聊排队消息"
	base["group"] = map[string]any{"expectedMembershipRevision": group.Group.MembershipRevision, "expectedSettingsRevision": group.Group.SettingsRevision, "mode": "standard"}
	center.request(t, child, http.MethodPost, prefix+"/chat-runtime/queue/enqueue", base, http.StatusOK)
	waitStatus := func(statusRequest map[string]any, expected string) business.QueuedChatResponse {
		deadline := time.Now().Add(10 * time.Second)
		for {
			status := decodeThreeCoreMemoryResponse[business.QueuedChatResponse](t, center.request(t, child, http.MethodPost, prefix+"/chat-runtime/queue/status", statusRequest, http.StatusOK))
			if status.Status == expected {
				return status
			}
			if expected != "failed" && expected != "uncertain" && (status.Status == "failed" || status.Status == "uncertain") {
				t.Fatalf("group queue failed: %+v", status)
			}
			if time.Now().After(deadline) {
				t.Fatalf("group queue did not reach %s: %+v", expected, status)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	waitStatus(base, "completed")
	stale := map[string]any{}
	for key, value := range base {
		stale[key] = value
	}
	stale["requestId"] = "queue-group-stale"
	stale["message"] = "群成员变化后的排队消息"
	stale["group"] = map[string]any{"expectedMembershipRevision": 1, "expectedSettingsRevision": group.Group.SettingsRevision, "mode": "standard"}
	center.request(t, child, http.MethodPost, prefix+"/chat-runtime/queue/enqueue", stale, http.StatusOK)
	failed := waitStatus(stale, "failed")
	if !strings.Contains(failed.Error, "群成员已变化") {
		t.Fatalf("group membership failure lost: %+v", failed)
	}
	discussion := map[string]any{}
	for key, value := range base {
		discussion[key] = value
	}
	discussion["requestId"] = "queue-group-discussion"
	discussion["message"] = "按顺序讨论排队消息"
	discussion["mentions"] = []map[string]any{}
	discussion["group"] = map[string]any{"expectedMembershipRevision": group.Group.MembershipRevision, "expectedSettingsRevision": group.Group.SettingsRevision, "mode": "discussion"}
	center.request(t, child, http.MethodPost, prefix+"/chat-runtime/queue/enqueue", discussion, http.StatusOK)
	waitStatus(discussion, "completed")
}
