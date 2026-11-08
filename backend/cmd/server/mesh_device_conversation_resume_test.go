package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestFirstConversationSendUsesRoleCatalogScope(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center := newThreeCoreFixture(t, "first-send-center", schemas)
	child := newThreeCoreFixture(t, "first-send-child", schemas)
	pairThreeCoreFixtures(t, child, center)
	policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, child.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, center, child, policy.ModeRevision)

	var catalog struct {
		ExecutionScope coordination.ExecutionScope `json:"executionScope"`
		Roles          []coordination.Role         `json:"roles"`
	}
	if err := json.Unmarshal(center.request(t, child, http.MethodGet, "/api/device-mesh/v1/business/roles", nil, http.StatusOK), &catalog); err != nil {
		t.Fatal(err)
	}
	for _, role := range catalog.Roles {
		if role.ID == "one" {
			catalog.ExecutionScope.RoleID = role.ID
			catalog.ExecutionScope.RoleRevision = role.Revision
			break
		}
	}
	first := decodeThreeCoreMemoryResponse[business.Response](t, center.request(t, child, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{
		"requestId":              "first-catalog-send",
		"characterId":            "one",
		"message":                "新会话第一条消息",
		"expectedExecutionScope": catalog.ExecutionScope,
	}, http.StatusOK))
	if !first.Saved || first.ConversationID == "" || first.Scope.ResourceOwnerID != child.device.DeviceID.String() || first.Scope.RoleOwnerID != center.core || first.Scope.MemoryRoleID != "one" {
		raw, _ := json.Marshal(first)
		t.Fatalf("first catalog-scoped send returned the wrong authority: %s", raw)
	}
}

func TestChildDeviceConversationReadMarkerSurvivesCloudReconnect(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center := newThreeCoreFixture(t, "resume-center", schemas)
	child := newThreeCoreFixture(t, "resume-child", schemas)
	pairThreeCoreFixtures(t, child, center)

	reply := decodeThreeCoreMemoryResponse[business.Response](t, center.request(t, child, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{
		"requestId":   "resume-conversation",
		"characterId": "one",
		"message":     "重启后仍应读到这条消息",
	}, http.StatusOK))
	if !reply.Saved {
		t.Fatal("conversation was not saved")
	}

	conversationPath := "/api/device-mesh/v1/business/conversations/" + url.PathEscape(reply.ConversationID) + "?characterId=one&conversationOwnerId=" + url.QueryEscape(child.device.DeviceID.String())
	conversation := decodeThreeCoreMemoryResponse[business.QueryResult](t, center.request(t, child, http.MethodGet, conversationPath, nil, http.StatusOK))
	if len(conversation.Snapshot.Resources) < 2 {
		t.Fatalf("conversation history was not restored: %+v", conversation)
	}

	resourcePath := "/api/device-mesh/v1/business/resources?kind=conversation&id=" + url.QueryEscape(reply.ConversationID) + "&characterId=one&targetDeviceId=" + url.QueryEscape(child.device.DeviceID.String())
	resource := decodeThreeCoreMemoryResponse[struct {
		Resource coordination.Resource       `json:"resource"`
		Scope    coordination.ExecutionScope `json:"executionScope"`
	}](t, center.request(t, child, http.MethodGet, resourcePath, nil, http.StatusOK))
	if resource.Resource.ID != reply.ConversationID || resource.Resource.OwnerID != child.device.DeviceID.String() || resource.Resource.Revision < 1 {
		t.Fatalf("conversation resource was not resolved for the child device: %+v", resource)
	}

	edit := decodeThreeCoreMemoryResponse[coordination.Acknowledgement](t, center.request(t, child, http.MethodPost, "/api/device-mesh/v1/business/resources/edit", map[string]any{
		"requestId":              "resume-read-marker",
		"kind":                   "conversation",
		"id":                     reply.ConversationID,
		"characterId":            "one",
		"expectedRevision":       resource.Resource.Revision,
		"expectedExecutionScope": resource.Scope,
		"changes": map[string]any{
			"lastReadTurnSequence": conversation.Snapshot.Resources[0].Revision,
		},
	}, http.StatusOK))
	if edit.OwnerID != child.device.DeviceID.String() || edit.Versions["conversation/"+reply.ConversationID] != resource.Resource.Revision+1 {
		raw, _ := json.Marshal(edit)
		t.Fatalf("read marker was not acknowledged by the child owner: %s", raw)
	}
}

func TestChildDeviceCanContinueConversationOwnedByCoordinatedCenter(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center := newThreeCoreFixture(t, "continue-center", schemas)
	child := newThreeCoreFixture(t, "continue-child", schemas)
	pairThreeCoreFixtures(t, child, center)
	policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, child.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, center, child, policy.ModeRevision)

	created := decodeThreeCoreMemoryResponse[business.Response](t, center.request(t, child, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{
		"requestId":   "continue-source",
		"characterId": "one",
		"message":     "建立待接续会话",
	}, http.StatusOK))
	conversationPath := "/api/device-mesh/v1/business/conversations/" + url.PathEscape(created.ConversationID) + "?characterId=one&conversationOwnerId=" + url.QueryEscape(child.device.DeviceID.String())
	view := decodeThreeCoreMemoryResponse[business.QueryResult](t, center.request(t, child, http.MethodGet, conversationPath, nil, http.StatusOK))

	continued := decodeThreeCoreMemoryResponse[business.Response](t, center.request(t, child, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{
		"requestId":              "continue-next",
		"characterId":            "one",
		"message":                "继续这个会话",
		"conversationId":         created.ConversationID,
		"conversationOrigin":     business.ConversationOrigin{OwnerID: child.device.DeviceID.String(), ID: created.ConversationID},
		"expectedExecutionScope": view.Scope,
	}, http.StatusOK))
	if !continued.Saved || continued.ConversationOrigin == nil || continued.ConversationOrigin.OwnerID != child.device.DeviceID.String() || continued.Scope.ResourceOwnerID != child.device.DeviceID.String() || continued.Scope.TargetDeviceID != child.device.DeviceID.String() {
		raw, _ := json.Marshal(continued)
		t.Fatalf("统筹中心接续会话归属错误: %s", raw)
	}
	runtime := decodeThreeCoreMemoryResponse[struct {
		Messages []map[string]any `json:"messages"`
	}](t, center.request(t, child, http.MethodPost, "/api/device-mesh/v1/business/chat-runtime/snapshot", map[string]any{
		"requestId":              "runtime-snapshot",
		"conversationId":         created.ConversationID,
		"conversationOrigin":     business.ConversationOrigin{OwnerID: child.device.DeviceID.String(), ID: created.ConversationID},
		"characterId":            "one",
		"targetDeviceId":         child.device.DeviceID.String(),
		"expectedExecutionScope": continued.Scope,
	}, http.StatusOK))
	if len(runtime.Messages) != 4 || runtime.Messages[2]["content"] != "继续这个会话" || runtime.Messages[3]["requestId"] != "continue-next" {
		raw, _ := json.Marshal(runtime)
		t.Fatalf("chat runtime snapshot lost or reordered conversation history: %s", raw)
	}
	for index, message := range runtime.Messages {
		if message["sequence"] != float64(index+1) {
			raw, _ := json.Marshal(runtime)
			t.Fatalf("chat runtime snapshot message order is unstable: %s", raw)
		}
	}
	feedback := decodeThreeCoreMemoryResponse[struct {
		Items []map[string]any `json:"items"`
	}](t, center.request(t, child, http.MethodPost, "/api/device-mesh/v1/business/chat-management/feedback/list", map[string]any{
		"messageId":              "continue-next/assistant",
		"characterId":            "one",
		"targetDeviceId":         child.device.DeviceID.String(),
		"expectedExecutionScope": continued.Scope,
	}, http.StatusOK))
	if feedback.Items == nil {
		t.Fatal("continued message feedback list was not returned")
	}
}
