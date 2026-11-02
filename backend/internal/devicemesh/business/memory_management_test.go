package business

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memorypolicy"
)

func TestOwnedMemoryManagementSemanticAndPaginationIntent(t *testing.T) {
	e, db, _, model := engineHarness(t)
	e.model = &semanticTestModel{testModel: model}
	req := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "search"}
	view, err := e.Query(t.Context(), req, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	scope := view.Scope
	scope.RequestID = "seed-search"
	mutations := []coordination.Mutation{}
	for _, id := range []string{"one", "two", "three"} {
		mutations = append(mutations, coordination.Mutation{Kind: "memory", ID: id, RoleID: "role", Body: json.RawMessage(`{"key":"` + id + `","content":{"value":"饮品","source":"manual"}}`)}, coordination.Mutation{Kind: "vector", ID: "v-" + id, RoleID: "role", SourceID: id, Body: json.RawMessage(`{"content":{"values":[1,0],"modelFingerprint":"model"}}`)})
	}
	mutations = append(mutations, coordination.Mutation{Kind: "vector", ID: "foreign", RoleID: "role", SourceID: "one", Body: json.RawMessage(`{"content":{"values":[1,0],"modelFingerprint":"other-model"}}`)})
	if _, err := coordination.NewOwnershipStore(db, "a").Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: mutations}); err != nil {
		t.Fatal(err)
	}
	found, err := e.SearchOwnedMemories(t.Context(), req, MemoryManagementQuery{Mode: "vector", Query: "茶", Limit: 1})
	if err != nil || len(found.Resources) != 1 || found.NextCursor == "" {
		t.Fatalf("semantic=%+v err=%v", found, err)
	}
	page, err := e.SearchOwnedMemories(t.Context(), req, MemoryManagementQuery{Mode: "vector", Query: "茶", Limit: 1, Cursor: found.NextCursor})
	if err != nil || len(page.Resources) != 1 || page.Resources[0].ID == found.Resources[0].ID {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if _, err := e.SearchOwnedMemories(t.Context(), req, MemoryManagementQuery{Mode: "vector", Query: "咖啡", Limit: 1, Cursor: found.NextCursor}); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("filter cursor=%v", err)
	}
	scope.RequestID = "changed-catalog"
	if _, err := coordination.NewOwnershipStore(db, "a").Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "memory", ID: "one", RoleID: "role", ExpectedRevision: 1, Body: json.RawMessage(`{"key":"one","content":{"value":"咖啡"}}`)}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SearchOwnedMemories(t.Context(), req, MemoryManagementQuery{Mode: "vector", Query: "茶", Limit: 1, Cursor: found.NextCursor}); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("catalog cursor=%v", err)
	}
}

func TestOwnedMemoryManagementCreateConflictCASAndReplay(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		t.Run(map[bool]string{false: "Source", true: "Core"}[coordinated], func(t *testing.T) {
			engine, db, service, _ := engineHarness(t)
			if coordinated {
				if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
					t.Fatal(err)
				}
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "manual"}
			view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedScope = &view.Scope
			input := MemoryManagementInput{Action: "create", Key: "饮品", Value: "喜欢茶", MemoryType: "preference", Importance: 7}
			created, err := engine.ManageOwnedMemory(t.Context(), request, input)
			if err != nil || !created.Saved || len(created.Resources) < 2 || created.Acknowledgement.Versions["checkpoint/memory-management/manual"] != 1 {
				t.Fatalf("created=%+v err=%v", created, err)
			}
			replay, err := engine.ManageOwnedMemory(t.Context(), request, input)
			if err != nil || replay.Resources[0].ID != created.Resources[0].ID {
				t.Fatalf("replay=%+v err=%v", replay, err)
			}
			input.Value = "喜欢咖啡"
			if _, err := engine.ManageOwnedMemory(t.Context(), request, input); !errors.Is(err, coordination.ErrRequestConflict) {
				t.Fatalf("changed retry=%v", err)
			}
			request.RequestID = "new-memory"
			_, err = engine.ManageOwnedMemory(t.Context(), request, input)
			var conflict MemoryManagementConflict
			if !errors.As(err, &conflict) || len(conflict.Resources) != 1 {
				t.Fatalf("conflict=%v", err)
			}
			input.Action, input.Resolution, input.ConflictID = "resolve", "merge", conflict.Resources[0].ID
			input.ExpectedConflictRevision = conflict.Resources[0].Revision + 1
			if _, err := engine.ManageOwnedMemory(t.Context(), request, input); !errors.Is(err, coordination.ErrResourceVersion) {
				t.Fatalf("CAS=%v", err)
			}
			input.ExpectedConflictRevision--
			resolved, err := engine.ManageOwnedMemory(t.Context(), request, input)
			if err != nil || !resolved.Saved {
				t.Fatalf("resolved=%+v err=%v", resolved, err)
			}
			resource, err := coordination.NewOwnershipStore(db, view.Scope.ResourceOwnerID).Get(t.Context(), "memory", conflict.Resources[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			_, value, _, _, _ := memoryManagementText(resource.Body)
			if value != "喜欢茶\n喜欢咖啡" || resource.Revision != 2 {
				t.Fatalf("memory=%+v text=%q", resource, value)
			}
			other := "core"
			if coordinated {
				other = "a"
			}
			rows, err := coordination.NewOwnershipStore(db, other).List(t.Context(), "memory", "role", false)
			if err != nil || len(rows) != 0 {
				t.Fatal("manual memory copied to another owner")
			}
		})
	}
}

func TestOwnedMemoryManagementRejectsOldCoreModeRoleAndRealm(t *testing.T) {
	engine, _, service, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "manual"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	input := MemoryManagementInput{Action: "create", Key: "记忆", Value: "不能跨归属"}
	for _, field := range []string{"core", "realm", "role", "permission", "target"} {
		stale := view.Scope
		switch field {
		case "core":
			stale.CoreID = "different"
		case "realm":
			stale.AuthorizationRealm = "different"
		case "role":
			stale.RoleRevision++
		case "permission":
			stale.PermissionRevision++
		case "target":
			stale.TargetPermissionRevision++
		}
		request.ExpectedScope = &stale
		if _, err := engine.ManageOwnedMemory(t.Context(), request, input); !errors.Is(err, coordination.ErrScopeExpired) {
			t.Fatalf("%s err=%v", field, err)
		}
	}
	request.ExpectedScope = &view.Scope
	if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ManageOwnedMemory(t.Context(), request, input); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("mode err=%v", err)
	}
}

func TestOwnedMemoryManagementSearchKeepsRoleOwnerAndContentFilters(t *testing.T) {
	engine, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "manual"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &view.Scope
	if _, err := engine.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "饮品", Value: "喜欢茶", MemoryType: "preference", Importance: 7}); err != nil {
		t.Fatal(err)
	}
	result, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{Query: "茶", Mode: "keyword", MemoryType: "preference", Source: "manual"})
	if err != nil || len(result.Resources) != 1 || result.Resources[0].OwnerID != "a" || result.Resources[0].RoleID != "role" {
		t.Fatalf("search=%+v err=%v", result, err)
	}
	result, err = engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{Query: "咖啡", Mode: "keyword"})
	if err != nil || len(result.Resources) != 0 {
		t.Fatalf("no match=%+v err=%v", result, err)
	}
}

func TestOwnedMemorySearchIncludesDormantOnlyForExplicitDeepRetrieval(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "dormant-search"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	document := map[string]json.RawMessage{
		"key":     json.RawMessage(`"宠物"`),
		"kind":    json.RawMessage(`"fact"`),
		"content": json.RawMessage(`{"value":"用户养了一只猫","memoryType":"fact","importance":2}`),
	}
	memorypolicy.WriteLifecycle(document, memorypolicy.NewLifecycleWithPolicy(memorypolicy.DecayPolicyDurable, 5, false, 2, time.Now().Add(-365*24*time.Hour)))
	encoded, _ := json.Marshal(document)
	if _, err := coordination.NewOwnershipStore(db, "a").Apply(t.Context(), coordination.Commit{Scope: view.Scope, Mutations: []coordination.Mutation{{Kind: "memory", ID: "dormant", RoleID: "role", Body: encoded}}}); err != nil {
		t.Fatal(err)
	}
	passive, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{ContextUsable: true, Query: "宠物", Mode: "keyword"})
	if err != nil {
		t.Fatal(err)
	}
	if passive.Total != 0 {
		t.Fatalf("dormant memory leaked into passive recall: %+v", passive)
	}
	explicit, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{ContextUsable: true, IncludeDormant: true, Query: "宠物", Mode: "keyword"})
	if err != nil {
		t.Fatal(err)
	}
	if explicit.Total != 1 || explicit.Resources[0].ID != "dormant" || explicit.Scores["dormant"] <= 0 {
		t.Fatalf("explicit recall=%+v", explicit)
	}
}

func TestOwnedMemoryHybridRankingUsesMultipleSignals(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	engine.model = &semanticTestModel{testModel: model}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "ranked-search"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	documents := map[string]map[string]json.RawMessage{
		"verified": {
			"key":       json.RawMessage(`"饮品A"`),
			"kind":      json.RawMessage(`"fact"`),
			"content":   json.RawMessage(`{"value":"喜欢茶","memoryType":"preference","importance":9,"confidence":100,"source":"reviewed"}`),
			"createdAt": json.RawMessage(`"` + now.Format(time.RFC3339Nano) + `"`),
		},
		"uncertain": {
			"key":       json.RawMessage(`"饮品B"`),
			"kind":      json.RawMessage(`"fact"`),
			"content":   json.RawMessage(`{"value":"可能喜欢茶","memoryType":"preference","importance":2,"confidence":20,"source":"extraction"}`),
			"createdAt": json.RawMessage(`"` + now.Add(-180*24*time.Hour).Format(time.RFC3339Nano) + `"`),
		},
	}
	mutations := []coordination.Mutation{}
	for id, document := range documents {
		memorypolicy.WriteLifecycle(document, memorypolicy.NewLifecycleWithPolicy(memorypolicy.DecayPolicyDurable, 4, false, 5, now))
		mutations = append(mutations, coordination.Mutation{Kind: "memory", ID: id, RoleID: "role", Body: body(document)})
	}
	if _, err := coordination.NewOwnershipStore(db, "a").Apply(t.Context(), coordination.Commit{Scope: view.Scope, Mutations: mutations}); err != nil {
		t.Fatal(err)
	}
	result, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{Query: "茶", Mode: "hybrid"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Resources) != 2 || result.Resources[0].ID != "verified" || result.Scores["verified"] <= result.Scores["uncertain"] {
		t.Fatalf("ranking=%+v scores=%+v", result.Resources, result.Scores)
	}
	verifiedDetails, uncertainDetails := result.ScoreDetails["verified"], result.ScoreDetails["uncertain"]
	if verifiedDetails.FinalScore <= 0 || verifiedDetails.Verification <= uncertainDetails.Verification || verifiedDetails.Importance <= uncertainDetails.Importance || verifiedDetails.SemanticScore != 0 {
		t.Fatalf("score details verified=%+v uncertain=%+v", verifiedDetails, uncertainDetails)
	}
}

func TestOwnedMemorySearchDeduplicatesSameLayerKey(t *testing.T) {
	rows := []coordination.Resource{
		{ID: "role", Kind: "memory", Body: json.RawMessage(`{"key":"city","kind":"fact","content":{"value":"杭州"}}`)},
		{ID: "global", Kind: "memory", Body: json.RawMessage(`{"key":"city","kind":"fact","content":{"value":"上海"}}`)},
	}
	deduped := dedupeMemorySearchResults(rows)
	if len(deduped) != 1 || deduped[0].ID != "role" {
		t.Fatalf("dedupe=%+v", deduped)
	}
}

func TestOfflineConsolidationAutoMergesSafeExactDuplicates(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "auto-merge"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	mutations := []coordination.Mutation{}
	for _, id := range []string{"duplicate-a", "duplicate-b"} {
		document := map[string]json.RawMessage{
			"key":     json.RawMessage(`"` + id + `"`),
			"kind":    json.RawMessage(`"fact"`),
			"content": json.RawMessage(`{"value":"用户长期偏好喝无糖绿茶","memoryType":"preference","importance":6,"confidence":60,"source":"extraction"}`),
		}
		memorypolicy.WriteLifecycle(document, memorypolicy.NewLifecycleWithPolicy(memorypolicy.DecayPolicyDurable, 4, false, 6, time.Now()))
		mutations = append(mutations, coordination.Mutation{Kind: "memory", ID: id, RoleID: "role", Body: body(document)})
	}
	if _, err := coordination.NewOwnershipStore(db, "a").Apply(t.Context(), coordination.Commit{Scope: view.Scope, Mutations: mutations}); err != nil {
		t.Fatal(err)
	}
	report, err := engine.ConsolidateOwnedMemories(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if report.AutoMergedCount != 1 {
		t.Fatalf("report=%+v", report)
	}
	store := coordination.NewOwnershipStore(db, "a")
	merged := 0
	for _, id := range []string{"duplicate-a", "duplicate-b"} {
		resource, err := store.Get(t.Context(), "memory", id)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(resource.Body, &document); err != nil {
			t.Fatal(err)
		}
		if MemoryText(document, "consolidationReason") == "exact-duplicate-content" {
			merged++
		}
	}
	if merged != 1 {
		t.Fatalf("merged=%d", merged)
	}
}
