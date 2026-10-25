package business

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestExplicitGlobalMemoryRequestRequiresDirectUserInstruction(t *testing.T) {
	allowed := []string{
		"把这条保存到全局记忆",
		"请记住这件事并写入全局记忆",
		"Save this to global memory",
	}
	rejected := []string{
		"全局记忆是什么",
		"不要保存到全局记忆",
		"为什么要保存到全局记忆？",
		"是否保存到全局记忆",
		"以后记得提醒我",
	}
	for _, message := range allowed {
		if !ExplicitGlobalMemoryRequest(message) {
			t.Fatalf("explicit request rejected: %s", message)
		}
	}
	for _, message := range rejected {
		if ExplicitGlobalMemoryRequest(message) {
			t.Fatalf("non-request accepted: %s", message)
		}
	}
}

func TestMemoryToolGlobalScopeRequiresExplicitRequest(t *testing.T) {
	e, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "global-denied", Message: "记住我喜欢茶"}
	if _, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "饮品", "value": "茶", "scope": "global"}); err == nil {
		t.Fatal("global memory accepted without explicit user request")
	}
	request.RequestID = "global-allowed"
	request.Message = "把这条保存到全局记忆：我喜欢茶"
	result, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "饮品", "value": "茶", "scope": "global"})
	if err != nil || !result.Saved {
		t.Fatalf("explicit global save failed: %+v %v", result, err)
	}
	global, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "memory", coordination.GlobalMemoryRole, false)
	if err != nil || len(global) != 1 || !json.Valid(global[0].Body) {
		t.Fatalf("global memory=%v err=%v", global, err)
	}
	role, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "memory", "role", false)
	if err != nil || len(role) != 0 {
		t.Fatalf("role memory=%v err=%v", role, err)
	}
}
