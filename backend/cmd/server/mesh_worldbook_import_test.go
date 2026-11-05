package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
)

func TestActualTLSWorldbookImportIsAtomic(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a := newThreeCoreFixture(t, "worldbook-import-a", schemas)
	b := newThreeCoreFixture(t, "worldbook-import-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	const endpoint = "/api/device-mesh/v1/business"
	view := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	validRule := func(pattern, content string) map[string]any {
		return map[string]any{
			"scope": "role", "matchType": "keyword", "matchPattern": pattern,
			"matchScope": "full_context", "injectContent": content, "priority": 1, "enabled": true,
		}
	}
	invalidRequest := map[string]any{
		"requestId": uuid.NewString(), "characterId": "one", "expectedExecutionScope": view.Scope,
		"rules": []map[string]any{
			validRule("有效", "有效规则"),
			{"scope": "role", "matchType": "invalid", "matchPattern": "无效", "matchScope": "full_context", "injectContent": "无效规则", "priority": 1, "enabled": true},
		},
	}
	b.request(t, a, http.MethodPost, endpoint+"/worldbook/import", invalidRequest, http.StatusBadRequest)
	unchanged := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	if len(unchanged.Snapshot.Resources) != 0 {
		t.Fatalf("invalid import committed %d rules", len(unchanged.Snapshot.Resources))
	}

	requestID := uuid.NewString()
	validRequest := map[string]any{
		"requestId": requestID, "characterId": "one", "expectedExecutionScope": view.Scope,
		"rules": []map[string]any{
			validRule("角色规则", "角色内容"),
			{"scope": "global", "matchType": "keyword", "matchPattern": "全局规则", "matchScope": "full_context", "injectContent": "全局内容", "priority": 2, "enabled": true},
		},
	}
	created := decodeThreeCoreMemoryResponse[business.WorldbookImportResponse](t, b.request(t, a, http.MethodPost, endpoint+"/worldbook/import", validRequest, http.StatusOK))
	if !created.Saved || len(created.Rules) != 2 || created.Acknowledgement.RequestID != requestID {
		t.Fatalf("import was not acknowledged: %+v", created)
	}
	replayed := decodeThreeCoreMemoryResponse[business.WorldbookImportResponse](t, b.request(t, a, http.MethodPost, endpoint+"/worldbook/import", validRequest, http.StatusOK))
	if len(replayed.Rules) != 2 || replayed.Rules[0].ID != created.Rules[0].ID || replayed.Rules[1].ID != created.Rules[1].ID {
		t.Fatalf("import replay was not idempotent: first=%+v replay=%+v", created.Rules, replayed.Rules)
	}
	listed := decodeThreeCoreMemoryResponse[business.QueryResult](t, b.request(t, a, http.MethodGet, endpoint+"/data?kind=worldbook&characterId=one", nil, http.StatusOK))
	if len(listed.Snapshot.Resources) != 2 {
		t.Fatalf("imported worldbook list has %d rules", len(listed.Snapshot.Resources))
	}
}
