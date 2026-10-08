package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type threeCoreMemoryModel struct{ core string }

func (m *threeCoreMemoryModel) GenerateOwnedReply(context.Context, business.Inference) (business.Generation, error) {
	return business.Generation{Text: "记忆联测回复来自 " + m.core}, nil
}

func (m *threeCoreMemoryModel) ExtractOwnedMemory(_ context.Context, input business.Inference, _ business.Generation) ([]business.DerivedMemory, error) {
	values := map[string]any{
		"working":  map[string]any{"summary": input.Message},
		"profile":  map[string]any{"preference": input.Message},
		"episodic": map[string]any{"event": input.Message},
		"fact":     map[string]any{"value": input.Message},
		"vector":   map[string]any{"values": []float32{0.1, 0.2}},
		"graph":    map[string]any{"relation": "likes", "target": input.Message},
	}
	result := make([]business.DerivedMemory, 0, len(values))
	for _, kind := range []string{"working", "profile", "episodic", "fact", "vector", "graph"} {
		body, err := json.Marshal(values[kind])
		if err != nil {
			return nil, err
		}
		result = append(result, business.DerivedMemory{Kind: kind, Key: input.Scope.RequestID, Body: body})
	}
	return result, nil
}

func (m *threeCoreMemoryModel) GenerateOwnedSummary(_ context.Context, input business.SummaryInference) (string, error) {
	if len(input.Messages) < 2 || input.Role.ID != "one" || input.Scope.CoreID != m.core {
		return "", fmt.Errorf("摘要输入未来自完整的当前角色和对话")
	}
	return "摘要由 " + m.core + " 生成", nil
}

func decodeThreeCoreMemoryResponse[T any](t *testing.T, body []byte) T {
	t.Helper()
	var response struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	return response.Data
}

func threeCoreMemoryQuery(t *testing.T, provider, source *threeCoreFixture, kind, conversation string) business.QueryResult {
	t.Helper()
	return decodeThreeCoreMemoryResponse[business.QueryResult](t, provider.request(t, source, http.MethodGet, "/api/device-mesh/v1/business/data?characterId=one&historicalRoleId=one&conversationId="+conversation+"&kind="+kind, nil, http.StatusOK))
}

func assertThreeCorePrivateCopies(t *testing.T, provider *threeCoreFixture, owner string) {
	t.Helper()
	var count int
	if err := provider.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND deleted=0`, owner).Scan(&count); err != nil || count != 0 {
		t.Fatalf("服务提供者保存了设备私有数据副本: count=%d error=%v", count, err)
	}
}

func TestThreeRealCoreSixMemoryLayersSummaryAndContinuityKeepTheirOwners(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a := newThreeCoreFixture(t, "memory-a", schemas)
	b := newThreeCoreFixture(t, "memory-b", schemas)
	c := newThreeCoreFixture(t, "memory-c", schemas)
	for _, fixture := range []*threeCoreFixture{a, b, c} {
		fixture.services.OwnedBusiness = business.NewEngine(fixture.services.DeviceMesh.Coordination, fixture.services.DeviceMesh, &threeCoreMemoryModel{core: fixture.core})
	}
	pairThreeCoreFixtures(t, a, b)
	deviceReply := decodeThreeCoreMemoryResponse[business.Response](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "device-six-layers", "characterId": "one", "message": "设备喜欢茶"}, http.StatusOK))
	if !deviceReply.Saved || deviceReply.MemoryStatus != "saved" || deviceReply.Scope.ResourceOwnerID != a.device.DeviceID.String() || deviceReply.Scope.Coordinated {
		t.Fatalf("设备记忆回写未确认: saved=%v memory=%s error=%s", deviceReply.Saved, deviceReply.MemoryStatus, deviceReply.MemoryError)
	}
	for _, kind := range []string{"working", "profile", "episodic", "fact", "vector", "graph"} {
		query := threeCoreMemoryQuery(t, b, a, kind, deviceReply.ConversationID)
		if query.Snapshot.OwnerID != a.device.DeviceID.String() || len(query.Snapshot.Resources) != 1 || query.Snapshot.Resources[0].Kind != kind {
			t.Fatalf("设备 %s 记忆读取错误: resources=%d", kind, len(query.Snapshot.Resources))
		}
	}
	deviceSummary := decodeThreeCoreMemoryResponse[business.SummaryResponse](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+deviceReply.ConversationID+"/summary/generate", map[string]any{"requestId": "device-summary", "characterId": "one", "expectedExecutionScope": deviceReply.Scope, "expectedRevision": 0}, http.StatusOK))
	if !deviceSummary.Saved || deviceSummary.OwnerID != a.device.DeviceID.String() || deviceSummary.Text != "摘要由 "+b.core+" 生成" {
		t.Fatal("设备摘要没有由 Core 计算并由 Source 保存")
	}
	deviceContinuity := decodeThreeCoreMemoryResponse[struct {
		Document business.OwnedContinuity     `json:"document"`
		Ack      coordination.Acknowledgement `json:"acknowledgement"`
	}](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/continuity", map[string]any{"requestId": "device-continuity", "characterId": "one", "action": "create", "title": "设备持续事项", "goal": "保留设备归属", "expectedCoreId": b.core, "expectedOwnerId": a.device.DeviceID.String(), "expectedModeRevision": deviceReply.Scope.ModeRevision, "expectedExecutionScope": deviceReply.Scope}, http.StatusOK))
	if deviceContinuity.Document.OwnerID != a.device.DeviceID.String() || deviceContinuity.Ack.OwnerID != a.device.DeviceID.String() {
		t.Fatal("设备持续事项没有确认写回原所有者")
	}
	assertThreeCorePrivateCopies(t, b, a.device.DeviceID.String())
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, http.StatusOK)
	policy, err = b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), b.core, a.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"working", "profile", "episodic", "fact", "vector", "graph", "summary"} {
		query := threeCoreMemoryQuery(t, b, a, kind, deviceReply.ConversationID)
		if query.Snapshot.OwnerID != b.core || len(query.Snapshot.Resources) != 0 || query.HistoricalSnapshot == nil || query.HistoricalSnapshot.OwnerID != a.device.DeviceID.String() || len(query.HistoricalSnapshot.Resources) != 1 {
			t.Fatalf("开启统筹没有保留设备 %s 历史读取", kind)
		}
	}
	coreReply := decodeThreeCoreMemoryResponse[business.Response](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "core-six-layers", "characterId": "one", "message": "云端喜欢咖啡"}, http.StatusOK))
	if !coreReply.Saved || coreReply.MemoryStatus != "saved" || coreReply.Scope.ResourceOwnerID != b.core {
		t.Fatalf("统筹记忆未保存: %s %s", coreReply.MemoryStatus, coreReply.MemoryError)
	}
	coreSummary := decodeThreeCoreMemoryResponse[business.SummaryResponse](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+coreReply.ConversationID+"/summary/generate", map[string]any{"requestId": "core-summary", "characterId": "one", "expectedExecutionScope": coreReply.Scope, "expectedRevision": 0}, http.StatusOK))
	if !coreSummary.Saved || coreSummary.OwnerID != b.core {
		t.Fatal("统筹摘要不属于 B")
	}
	coreContinuity := decodeThreeCoreMemoryResponse[struct {
		Document business.OwnedContinuity `json:"document"`
	}](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/continuity", map[string]any{"requestId": "core-continuity", "characterId": "one", "action": "create", "title": "B 专属持续事项", "expectedCoreId": b.core, "expectedOwnerId": b.core, "expectedModeRevision": coreReply.Scope.ModeRevision, "expectedExecutionScope": coreReply.Scope}, http.StatusOK))
	if coreContinuity.Document.OwnerID != b.core {
		t.Fatal("统筹持续事项不属于 B")
	}
	pairThreeCoreFixtures(t, b, c)
	expiredPolicy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil || expiredPolicy.Administrator || !expiredPolicy.Coordinated {
		t.Fatal("旧管理员授权未失效或设备统筹状态丢失")
	}
	if err := a.local.FollowSuccessor(t.Context()); err == nil {
		t.Fatal("A 未经独立审批切换至 C")
	}
	approvals, err := c.pairing.PendingApprovals(t.Context())
	if err != nil || len(approvals) != 1 {
		t.Fatal("C 缺少 A 的独立审批")
	}
	if err := c.pairing.DecideApproval(t.Context(), approvals[0].RequestID, approvals[0].Revision, true); err != nil {
		t.Fatal(err)
	}
	followApprovedThreeCoreSuccessor(t, a, c)
	currentPolicy, err := c.services.DeviceMesh.Coordination.Get(t.Context(), c.core, a.device.DeviceID.String())
	if err != nil || currentPolicy.Administrator || !currentPolicy.Coordinated {
		t.Fatal("新 Core 继承了旧管理员权限或丢失统筹状态")
	}
	for _, kind := range []string{"working", "profile", "episodic", "fact", "vector", "graph", "summary"} {
		query := threeCoreMemoryQuery(t, c, a, kind, deviceReply.ConversationID)
		if query.Snapshot.OwnerID != c.core || len(query.Snapshot.Resources) != 0 || query.HistoricalSnapshot == nil || len(query.HistoricalSnapshot.Resources) != 1 || query.HistoricalSnapshot.OwnerID != a.device.DeviceID.String() {
			t.Fatalf("切换后 %s 混入 B 专属数据或丢失设备历史", kind)
		}
	}
	c.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/continuity", map[string]any{"requestId": "stale-core-continuity", "characterId": "one", "action": "create", "title": "过期页面", "expectedCoreId": b.core, "expectedOwnerId": b.core, "expectedModeRevision": coreReply.Scope.ModeRevision, "expectedExecutionScope": coreReply.Scope}, http.StatusConflict)
	currentContinuity := decodeThreeCoreMemoryResponse[[]business.OwnedContinuity](t, c.request(t, a, http.MethodGet, "/api/device-mesh/v1/business/continuity?characterId=one", nil, http.StatusOK))
	if len(currentContinuity) != 0 {
		t.Fatal("B 专属持续事项被自动迁移到 C")
	}
	cReply := decodeThreeCoreMemoryResponse[business.Response](t, c.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "c-six-layers", "characterId": "one", "conversationId": coreReply.ConversationID, "message": "切换后继续讨论", "context": business.ForwardedContext{PreviousCoreID: b.core, ConversationID: coreReply.ConversationID, Summary: coreSummary.Text, Messages: []business.ContextMessage{{ID: "previous-b-user", OwnerID: b.core, Role: "user", Content: "云端喜欢咖啡"}, {ID: "previous-b-assistant", OwnerID: b.core, Role: "assistant", Content: coreReply.Text}}}}, http.StatusOK))
	if !cReply.Saved || cReply.MemoryStatus != "saved" || cReply.Scope.ResourceOwnerID != c.core {
		t.Fatalf("携带上下文的新 C 记忆未保存: %s %s", cReply.MemoryStatus, cReply.MemoryError)
	}
	cSummary := decodeThreeCoreMemoryResponse[business.SummaryResponse](t, c.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/conversations/"+cReply.ConversationID+"/summary/generate", map[string]any{"requestId": "c-summary", "characterId": "one", "expectedExecutionScope": cReply.Scope, "expectedRevision": 0}, http.StatusOK))
	if !cSummary.Saved || cSummary.OwnerID != c.core || cSummary.Text != "摘要由 "+c.core+" 生成" {
		t.Fatal("携带上下文后摘要没有由 C 计算并保存在 C")
	}
	c.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/continuity", map[string]any{"requestId": "c-continuity", "characterId": "one", "action": "create", "title": "C 新持续事项", "expectedCoreId": c.core, "expectedOwnerId": c.core, "expectedModeRevision": cReply.Scope.ModeRevision, "expectedExecutionScope": cReply.Scope}, http.StatusOK)
	for _, owner := range []string{b.core, a.device.DeviceID.String()} {
		assertThreeCorePrivateCopies(t, c, owner)
	}
	for _, fixture := range []*threeCoreFixture{a, b, c} {
		owner := fixture.core
		if fixture == a {
			owner = a.device.DeviceID.String()
		}
		var count int
		if err := fixture.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind IN ('working','profile','episodic','fact','vector','graph','summary','continuity') AND deleted=0`, owner).Scan(&count); err != nil || count != 8 {
			t.Fatalf("原所有者数据被迁移或丢失: count=%d error=%v", count, err)
		}
	}
}
