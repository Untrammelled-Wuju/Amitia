package business

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
)

func TestOwnedMemoryConsumersArchiveRestorePreservesLifecycleAndIndexes(t *testing.T) {
	e, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "create"}
	page, err := e.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &page.Scope
	created, err := e.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "固定事实", Value: "保留", Pinned: true, Importance: 9})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := e.ExportMemoryArchive(t.Context(), request, "role")
	if err != nil {
		t.Fatal(err)
	}
	var roots, graphs int
	for _, entry := range archive.Entries {
		if entry.Kind == "memory" {
			roots++
		}
		if entry.Kind == "graph" {
			graphs++
		}
	}
	if roots != 1 || graphs != 1 {
		t.Fatalf("archive lost records or index: %d/%d", roots, graphs)
	}
	other, db, _, _ := engineHarness(t)
	input := dataportability.ImportRequest{OperationID: "restore", CharacterPolicy: dataportability.CollisionReplace, IdentityMap: dataportability.NewImportIdentityMap()}
	if err := other.RestoreMemoryArchive(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "restore"}, input, archive); err != nil {
		t.Fatal(err)
	}
	row, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", created.Resources[0].ID)
	if err != nil || row == nil {
		t.Fatalf("restored=%+v %v", row, err)
	}
	var document map[string]json.RawMessage
	_ = json.Unmarshal(row.Body, &document)
	var pinned bool
	_ = json.Unmarshal(document["pinned"], &pinned)
	var level int
	_ = json.Unmarshal(document["retentionLevel"], &level)
	if !pinned || level != 1 {
		t.Fatalf("lifecycle lost: %s", row.Body)
	}
	if err := other.RestoreMemoryArchive(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "replay"}, input, archive); err != nil {
		t.Fatal(err)
	}
	again, _ := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", row.ID)
	if again.Revision != row.Revision {
		t.Fatal("restore replay rewrote record")
	}
}

func TestOwnedMemoryConsumersToolsFilterBeforeLimitAndChunkAnyTitle(t *testing.T) {
	e, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "first"}
	for i, item := range []struct {
		key, value, kind string
		importance       int
	}{{"标题", "甲乙丙丁戊己", "preference", 9}, {"别的标题", "甲不同", "fact", 2}} {
		request.RequestID = string(rune('a' + i))
		if _, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": item.key, "value": item.value, "memoryType": item.kind, "importance": item.importance}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := e.MemoryTool(t.Context(), request, "query_memory", map[string]interface{}{"query": "甲", "mode": "keyword", "limit": 1, "types": []string{"preference"}, "min_importance": 8})
	if err != nil || len(result.Resources) != 1 || !strings.Contains(result.Text, "甲乙丙丁") {
		t.Fatalf("filter=%+v %v", result, err)
	}
	chunk, err := e.MemoryTool(t.Context(), request, "get_memory_by_title", map[string]interface{}{"title": "标题", "offset": 1, "max_chars": 3})
	if err != nil || chunk.Text != "乙丙丁" || chunk.Metadata["nextOffset"] != 4 || chunk.Metadata["truncated"] != true {
		t.Fatalf("chunk=%+v %v", chunk, err)
	}
}

func TestOwnedMemoryConsumersDuplicateRestoreRemapsManualRelationTargets(t *testing.T) {
	e, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "first"}
	first, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "甲", "value": "茶"})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "second"
	second, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "乙", "value": "水"})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "link"
	if _, err := e.LinkOwnedMemories(t.Context(), request, first.Resources[0].ID, second.Resources[0].ID, "related_to", 1); err != nil {
		t.Fatal(err)
	}
	archive, err := e.ExportMemoryArchive(t.Context(), request, "role")
	if err != nil {
		t.Fatal(err)
	}
	other, _, _, _ := engineHarness(t)
	identities := dataportability.NewImportIdentityMap()
	input := dataportability.ImportRequest{OperationID: "duplicate", CharacterPolicy: dataportability.CollisionDuplicate, IdentityMap: identities}
	if err := other.RestoreMemoryArchive(t.Context(), request, input, archive); err != nil {
		t.Fatal(err)
	}
	source, sourceOK := identities.GetMemory(first.Resources[0].ID)
	target, targetOK := identities.GetMemory(second.Resources[0].ID)
	if !sourceOK || !targetOK {
		t.Fatal("restored identities missing")
	}
	links, err := other.OwnedMemoryLinks(t.Context(), request, source, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range links {
		if !strings.HasPrefix(link.ID, "memory-relation/") {
			continue
		}
		_, content, err := MemoryContent(MemoryConsumerEntry{Resource: link})
		if err != nil || link.SourceID != source || MemoryText(content, "target") != target || target == second.Resources[0].ID {
			t.Fatalf("relation not remapped: %+v %v", link, err)
		}
		return
	}
	t.Fatal("restored manual relation missing")
}

func TestOwnedMemoryConsumersRelatedDeletionOnlyDeletesTargetConversation(t *testing.T) {
	e, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "first", ConversationID: "conversation-one"}
	first, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "甲", "value": "茶"})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID, request.ConversationID = "second", "conversation-two"
	second, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "乙", "value": "水"})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := e.ExportMemoryArchive(t.Context(), request, "role")
	if err != nil {
		t.Fatal(err)
	}
	results, err := e.DeleteRelatedOwnedMemories(t.Context(), request, archive.Entries, "conversation", "conversation-one")
	if err != nil || len(results) != 1 || results[0].TargetID != first.Resources[0].ID || !results[0].RetrievalBlocked {
		t.Fatalf("deletion=%+v %v", results, err)
	}
	store := coordination.NewOwnershipStore(db, "a")
	deleted, err := store.Get(t.Context(), "memory", first.Resources[0].ID)
	if err != nil || deleted == nil || !deleted.Deleted {
		t.Fatalf("target not deleted: %+v %v", deleted, err)
	}
	retained, err := store.Get(t.Context(), "memory", second.Resources[0].ID)
	if err != nil || retained == nil || retained.Deleted {
		t.Fatalf("other conversation deleted: %+v %v", retained, err)
	}
}

func TestOwnedMemoryConsumersPinnedMemorySurvivesAutomaticExtraction(t *testing.T) {
	e, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "pin"}
	page, err := e.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &page.Scope
	created, err := e.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "名字", Value: "用户确定的名字", Pinned: true, Importance: 9})
	if err != nil {
		t.Fatal(err)
	}
	ctx, scope, finish, err := e.memoryManagementScope(t.Context(), request, false)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	snapshot := coordination.DataSnapshot{OwnerID: "a", Role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{}`)}}
	mutations, err := e.prepareMemoryMutations(ctx, scope, "chat", []DerivedMemory{{Kind: "fact", Key: "名字", Body: body(map[string]interface{}{"value": "模型猜测的名字"})}}, &snapshot)
	if err != nil || len(mutations) != 0 {
		t.Fatalf("fixed memory overwritten: %v %v", mutations, err)
	}
	row, _ := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", created.Resources[0].ID)
	if !strings.Contains(string(row.Body), "用户确定的名字") {
		t.Fatal("fixed memory changed")
	}
	request.RequestID = "tool-change"
	if _, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "名字", "value": "改名"}); err == nil {
		t.Fatal("tool overwrote fixed memory")
	}
}

func TestOwnedMemoryConsumersRelationsRejectForeignAndStaleSources(t *testing.T) {
	e, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "one"}
	one, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "茶", "value": "喜欢茶"})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "two"
	two, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "饮品", "value": "温热饮品"})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "link"
	if _, err := e.LinkOwnedMemories(t.Context(), request, one.Resources[0].ID, two.Resources[0].ID, "related_to", .8); err != nil {
		t.Fatal(err)
	}
	links, err := e.OwnedMemoryLinks(t.Context(), request, one.Resources[0].ID, 1)
	if err != nil || len(links) < 1 {
		t.Fatalf("links=%v %v", links, err)
	}
	request.RequestID = "foreign"
	if _, err := e.LinkOwnedMemories(t.Context(), request, one.Resources[0].ID, "foreign", "related_to", 1); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("foreign=%v", err)
	}
}

func TestOwnedMemoryLifecycleChangesPreserveDerivedIndexes(t *testing.T) {
	e, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "seed"}
	result, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "茶", "value": "喜欢茶"})
	if err != nil {
		t.Fatal(err)
	}
	page, err := e.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	root := result.Resources[0]
	request.ExpectedScope = &page.Scope
	if _, err := e.Edit(t.Context(), request, EditRequest{RequestID: "strength", Kind: "memory", ID: root.ID, RoleID: "role", ExpectedScope: &page.Scope, ExpectedRevision: root.Revision, Changes: map[string]json.RawMessage{"memoryStrength": body(.8)}}); err != nil {
		t.Fatal(err)
	}
	graphs, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "graph", "role", false)
	if err != nil || len(graphs) != 1 {
		t.Fatalf("lifecycle deleted graph: %v %v", graphs, err)
	}
	row, _ := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", root.ID)
	if _, err := e.Edit(t.Context(), request, EditRequest{RequestID: "content", Kind: "memory", ID: root.ID, RoleID: "role", ExpectedScope: &page.Scope, ExpectedRevision: row.Revision, Changes: map[string]json.RawMessage{"content": body(map[string]interface{}{"value": "改为喜欢咖啡"})}}); err != nil {
		t.Fatal(err)
	}
	graphs, err = coordination.NewOwnershipStore(db, "a").List(t.Context(), "graph", "role", false)
	if err != nil || len(graphs) != 0 {
		t.Fatalf("content kept stale graph: %v %v", graphs, err)
	}
}

func TestOwnedMemoryPrivateProfileExtractionDoesNotPublishGlobalProfile(t *testing.T) {
	e, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "profile"}
	ctx, scope, finish, err := e.memoryManagementScope(t.Context(), request, false)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	snapshot, err := e.data.Snapshot(ctx, scope, coordination.DataQuery{Management: true})
	if err != nil {
		t.Fatal(err)
	}
	mutations, err := e.prepareMemoryMutations(ctx, scope, "chat", modelMemoryDerived([]DerivedMemory{{Kind: "profile", Key: "姓名", Body: body(map[string]interface{}{"value": "私密信息", "memoryType": "personal_info", "source": "manual"})}}), &snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: mutations}); err != nil {
		t.Fatal(err)
	}
	profiles, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "profile", "role", false)
	if err != nil || len(profiles) != 1 || !strings.Contains(string(profiles[0].Body), "私密信息") {
		t.Fatalf("role profile lost: %v %v", profiles, err)
	}
	memories, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "memory", "role", false)
	if err != nil || len(memories) != 0 {
		t.Fatalf("profile duplicated as memory: %v %v", memories, err)
	}
}

type memoryDeletionTestPort struct{ testDataPort }

func (p memoryDeletionTestPort) MemoryDeletionComplete(ctx context.Context, scope coordination.ExecutionScope, id string) (bool, error) {
	return coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID).MemoryDeletionComplete(ctx, coordination.StorageRole(scope, "memory"), id)
}

func TestOwnedMemoryPrivacyDeletionPersistsStatusAndWaitsForRealIndexCleanup(t *testing.T) {
	e, db, _, _ := engineHarness(t)
	e.data = memoryDeletionTestPort{testDataPort: e.data.(testDataPort)}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "save"}
	created, err := e.MemoryTool(t.Context(), request, "save_memory", map[string]interface{}{"key": "电话", "value": "13800138000"})
	if err != nil {
		t.Fatal(err)
	}
	page, err := e.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	entry := MemoryConsumerEntry{Resource: created.Resources[0], Scope: page.Scope}
	request.RequestID = "delete"
	deleted, err := e.DeleteOwnedMemory(t.Context(), request, entry, "all")
	if err != nil || !deleted.RetrievalBlocked || deleted.Status != "cleaning" || deleted.CleanedCount != 0 {
		t.Fatalf("premature deletion success: %+v %v", deleted, err)
	}
	if strings.Contains(deleted.ID, "/") {
		t.Fatal("deletion status id cannot be one URL path segment")
	}
	address, err := ParseMemoryDeletionID(deleted.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := e.MemoryDeletionState(t.Context(), request, address)
	if err != nil || stored.TargetID != entry.ID || stored.Status != "cleaning" {
		t.Fatalf("status lost: %+v %v", stored, err)
	}
	jobs, err := coordination.NewOwnershipStore(db, "a").PendingProjections(t.Context())
	if err != nil || len(jobs) == 0 {
		t.Fatalf("no actual cleanup job: %v %v", jobs, err)
	}
	for _, job := range jobs {
		if job.Resource.SourceID == entry.ID {
			if coordination.ProjectionUsable(job, time.Now()) {
				t.Fatal("deleted source still usable")
			}
			if err := coordination.NewOwnershipStore(db, "a").FinishProjection(t.Context(), job, "", true); err != nil {
				t.Fatal(err)
			}
		}
	}
	completed, err := e.MemoryDeletionState(t.Context(), request, address)
	if err != nil || completed.Status != "completed" || completed.CleanedCount != 1 {
		t.Fatalf("cleanup confirmation missing: %+v %v", completed, err)
	}
	address.OwnerID = "other"
	if _, err := e.MemoryDeletionState(t.Context(), request, address); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign status read accepted: %v", err)
	}
}

func TestOwnedMemoryConsumersPrivacyUsesVersionedCurrentRecords(t *testing.T) {
	entry := MemoryConsumerEntry{Resource: coordination.Resource{ID: "current", Kind: "memory", OwnerID: "a", RoleID: "role", Revision: 4, Body: body(map[string]interface{}{"key": "联系", "createdAt": time.Now().UTC().Format(time.RFC3339Nano), "content": map[string]interface{}{"value": "13800138000", "memoryType": "personal_info"}})}, Scope: coordination.ExecutionScope{ResourceOwnerID: "a", RoleID: "role"}}
	scan, err := ScanMemoryPrivacy([]MemoryConsumerEntry{entry}, func(text string) (string, string, bool) {
		return "phone", "high", strings.Contains(text, "13800138000")
	})
	if err != nil || scan.Count != 1 || len(scan.Findings) != 1 {
		t.Fatalf("scan=%+v %v", scan, err)
	}
	finding := scan.Findings[0].(map[string]interface{})
	if finding["ownerId"] != "a" || finding["revision"] != int64(4) || finding["recordId"] != "current" {
		t.Fatalf("unbound finding=%+v", finding)
	}
	if _, err := ScanMemoryPrivacy(nil, nil); err == nil {
		t.Fatal("nil matcher accepted")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := RerankMemoryScores(canceled, "昨天", nil, nil, time.Now()); err == nil {
		t.Fatal("canceled rerank accepted")
	}
}
