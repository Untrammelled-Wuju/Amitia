package main

import (
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
)

func TestChatManagementSearchRejectsBlankKeyword(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	source := newThreeCoreFixture(t, "search-blank", schemas)
	provisionFixtureSelfCredential(t, source)
	const endpoint = "/api/device-mesh/v1/business"
	created := decodeThreeCoreMemoryResponse[business.Response](t, source.request(t, source, http.MethodPost, endpoint+"/messages", map[string]any{
		"requestId":   "search-blank-message",
		"characterId": "one",
		"message":     "空的搜索词不应返回全部会话",
	}, http.StatusOK))
	for _, keyword := range []string{"", "   "} {
		source.request(t, source, http.MethodPost, endpoint+"/chat-management/search", map[string]any{
			"characterId":            "one",
			"targetDeviceId":         source.device.DeviceID.String(),
			"expectedExecutionScope": created.Scope,
			"keyword":                keyword,
		}, http.StatusBadRequest)
	}
	found := decodeThreeCoreMemoryResponse[struct {
		Total int `json:"total"`
	}](t, source.request(t, source, http.MethodPost, endpoint+"/chat-management/search", map[string]any{
		"characterId":            "one",
		"targetDeviceId":         source.device.DeviceID.String(),
		"expectedExecutionScope": created.Scope,
		"keyword":                "搜索词",
	}, http.StatusOK))
	if found.Total != 1 {
		t.Fatalf("keyword search total=%d want=1", found.Total)
	}
}
