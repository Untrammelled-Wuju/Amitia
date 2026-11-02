package business

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memorypolicy"
)

func TestOwnedMemoryReinforcementIsCommittedOncePerTurn(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "seed-memory"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &view.Scope
	created, err := engine.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "tea", Value: "喜欢茶", RetentionLevel: 3, Importance: 7})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "seed-unused"
	unused, err := engine.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "coffee", Value: "偶尔喝咖啡", RetentionLevel: 3, Importance: 7})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID, request.ConversationID, request.Message, request.ExpectedScope = "recall-memory", "chat", "喜欢什么饮品", nil
	request.RecallEvidence = []coordination.ResourceVersion{{Kind: "memory", ID: created.Resources[0].ID, Revision: created.Resources[0].Revision}}
	result, err := engine.Run(t.Context(), request)
	if err != nil || result.MemoryStatus != "saved" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	read := func() memorypolicy.Lifecycle {
		resource, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", created.Resources[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]json.RawMessage
		_ = json.Unmarshal(resource.Body, &document)
		return memorypolicy.ReadLifecycle(document, time.Now())
	}
	state := read()
	if state.ReinforceCount != 1 || state.MemoryStrength <= .68 {
		t.Fatalf("not reinforced: %+v", state)
	}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if replay := read(); replay.ReinforceCount != 1 || replay.MemoryStrength != state.MemoryStrength {
		t.Fatalf("replay reinforced twice: %+v", replay)
	}
	unusedRow, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", unused.Resources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	var unusedDocument map[string]json.RawMessage
	if err := json.Unmarshal(unusedRow.Body, &unusedDocument); err != nil {
		t.Fatal(err)
	}
	if unusedState := memorypolicy.ReadLifecycle(unusedDocument, time.Now()); unusedState.ReinforceCount != 0 {
		t.Fatalf("unrecalled memory was reinforced: %+v", unusedState)
	}
}

func TestOwnedMemoryLifecycleCreateEditCASAndFilters(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "lifecycle-create"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &view.Scope
	created, err := engine.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "茶", Value: "喜欢茶", Importance: 7, RetentionLevel: 2, Pinned: true})
	if err != nil {
		t.Fatal(err)
	}
	row := created.Resources[0]
	var document map[string]json.RawMessage
	if err := json.Unmarshal(row.Body, &document); err != nil {
		t.Fatal(err)
	}
	state := memorypolicy.ReadLifecycle(document, time.Now())
	if !state.Pinned || state.RetentionLevel != 1 || state.MemoryStrength != 1 {
		t.Fatalf("state=%+v", state)
	}
	edit := EditRequest{RequestID: "lifecycle-edit", Kind: "memory", ID: row.ID, ExpectedRevision: row.Revision, ExpectedScope: &view.Scope, RoleID: "role", Changes: map[string]json.RawMessage{"pinned": body(false), "retentionLevel": body(4), "memoryStrength": body(.5)}}
	if _, err := engine.Edit(t.Context(), request, edit); err != nil {
		t.Fatal(err)
	}
	stored, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", row.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(stored.Body, &document)
	state = memorypolicy.ReadLifecycle(document, time.Now())
	if state.Pinned || state.RetentionLevel != 4 || state.MemoryStrength != .5 {
		t.Fatalf("edited=%+v", state)
	}
	edit.RequestID = "stale-edit"
	if _, err := engine.Edit(t.Context(), request, edit); err == nil {
		t.Fatal("stale edit accepted")
	}
	filtered, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{RetentionLevel: 4})
	if err != nil || len(filtered.Resources) != 1 {
		t.Fatalf("filter=%+v err=%v", filtered, err)
	}
	filtered, err = engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{RetentionLevel: 2})
	if err != nil || len(filtered.Resources) != 0 {
		t.Fatalf("wrong filter=%+v err=%v", filtered, err)
	}
}

func TestAutomaticExtractionCannotPinMemoryOrOverrideLifecycle(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) {
		return []DerivedMemory{{Kind: "fact", Key: "drink", Body: json.RawMessage(`{"value":"茶","importance":5,"pinned":true,"retentionLevel":1,"memoryStrength":1}`)}}, nil
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "model-pin", ConversationID: "chat", Message: "喜欢茶"}
	result, err := engine.Run(t.Context(), request)
	if err != nil || result.MemoryStatus != "saved" {
		t.Fatalf("run=%+v err=%v", result, err)
	}
	rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "memory", "role", false)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	var document map[string]json.RawMessage
	_ = json.Unmarshal(rows[0].Body, &document)
	state := memorypolicy.ReadLifecycle(document, time.Now())
	if state.Pinned || state.RetentionLevel != 4 {
		t.Fatalf("model overrode policy: %+v", state)
	}
}

func TestOwnedMemoryCorrectionConfirmAndDisputeFlow(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "correction-seed"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &view.Scope
	created, err := engine.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "city", Value: "上海", Importance: 7})
	if err != nil {
		t.Fatal(err)
	}
	row := created.Resources[0]
	request.RequestID, request.ExpectedScope = "correction-edit", &view.Scope
	_, err = engine.Edit(t.Context(), request, EditRequest{RequestID: request.RequestID, Kind: "memory", ID: row.ID, ExpectedRevision: row.Revision, ExpectedScope: &view.Scope, RoleID: "role", Changes: map[string]json.RawMessage{
		"content":          body(map[string]any{"value": "杭州", "memoryType": "fact", "importance": 9}),
		"correctionReason": body("用户明确纠正常住城市"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	read := func() map[string]json.RawMessage {
		resource, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", row.ID)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(resource.Body, &document); err != nil {
			t.Fatal(err)
		}
		return document
	}
	corrected := read()
	if MemoryText(corrected, "verifiedStatus") != "user_corrected" || MemoryText(corrected, "previousContentHash") == "" || MemoryText(corrected, "correctionReason") == "" {
		t.Fatalf("correction metadata=%+v", corrected)
	}
	var content map[string]json.RawMessage
	if err := json.Unmarshal(corrected["content"], &content); err != nil {
		t.Fatal(err)
	}
	if MemoryText(content, "source") != "user_correction" || memoryNumber(content["confidence"], 0) != 100 || MemoryText(content, "verifiedStatus") != "user_corrected" {
		t.Fatalf("corrected content=%+v", content)
	}
	request.RequestID = "dispute"
	revision := int64(2)
	if _, err := engine.Edit(t.Context(), request, EditRequest{RequestID: request.RequestID, Kind: "memory", ID: row.ID, ExpectedRevision: revision, ExpectedScope: &view.Scope, RoleID: "role", Changes: map[string]json.RawMessage{"verifiedStatus": body("disputed")}}); err != nil {
		t.Fatal(err)
	}
	disputed := read()
	if MemoryText(disputed, "verifiedStatus") != "disputed" || memoryNumber(disputed["confidence"], 1) != 0 || !bytes.Equal(disputed["allowContextUse"], []byte("false")) {
		t.Fatalf("disputed=%+v", disputed)
	}
	request.RequestID = "confirm"
	if _, err := engine.Edit(t.Context(), request, EditRequest{RequestID: request.RequestID, Kind: "memory", ID: row.ID, ExpectedRevision: 3, ExpectedScope: &view.Scope, RoleID: "role", Changes: map[string]json.RawMessage{"verifiedStatus": body("verified")}}); err != nil {
		t.Fatal(err)
	}
	verified := read()
	if MemoryText(verified, "verifiedStatus") != "verified" || memoryNumber(verified["confidence"], 0) != 100 || !bytes.Equal(verified["allowContextUse"], []byte("true")) {
		t.Fatalf("verified=%+v", verified)
	}
	request.RequestID = "feedback-helpful"
	if _, err := engine.Edit(t.Context(), request, EditRequest{RequestID: request.RequestID, Kind: "memory", ID: row.ID, ExpectedRevision: 4, ExpectedScope: &view.Scope, RoleID: "role", Changes: map[string]json.RawMessage{"retrievalFeedback": body("helpful")}}); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "feedback-unhelpful"
	if _, err := engine.Edit(t.Context(), request, EditRequest{RequestID: request.RequestID, Kind: "memory", ID: row.ID, ExpectedRevision: 5, ExpectedScope: &view.Scope, RoleID: "role", Changes: map[string]json.RawMessage{"retrievalFeedback": body("not_helpful")}}); err != nil {
		t.Fatal(err)
	}
	feedback := read()
	if memoryNumber(feedback["helpfulRetrievalCount"], 0) != 1 || memoryNumber(feedback["unhelpfulRetrievalCount"], 0) != 1 || memoryNumber(feedback["retrievalUtility"], 0) != .5 {
		t.Fatalf("feedback=%+v", feedback)
	}
	profile, _, err := engine.memoryRankingProfile(t.Context(), view.Scope)
	if err != nil || profile.SampleCount != 2 {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	sum := 0.0
	for _, weight := range profile.Weights.vector() {
		sum += weight
	}
	if math.Abs(sum-1) > .000001 {
		t.Fatalf("weight sum=%f profile=%+v", sum, profile)
	}
}

func TestOwnedMemoryHistoryCanRestoreImmutableVersion(t *testing.T) {
	engine, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "history-seed"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &view.Scope
	created, err := engine.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "city", Value: "上海", Importance: 8})
	if err != nil {
		t.Fatal(err)
	}
	row := created.Resources[0]
	request.RequestID = "history-edit"
	if _, err := engine.Edit(t.Context(), request, EditRequest{RequestID: request.RequestID, Kind: "memory", ID: row.ID, ExpectedRevision: 1, ExpectedScope: &view.Scope, RoleID: "role", Changes: map[string]json.RawMessage{"content": body(map[string]any{"value": "杭州", "memoryType": "fact", "importance": 8})}}); err != nil {
		t.Fatal(err)
	}
	history, _, err := engine.OwnedMemoryHistory(t.Context(), request, row.ID)
	if err != nil || len(history.Versions) != 1 || history.Versions[0].Revision != 1 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	request.RequestID = "history-restore"
	if _, err := engine.Edit(t.Context(), request, EditRequest{RequestID: request.RequestID, Kind: "memory", ID: row.ID, ExpectedRevision: 2, ExpectedScope: &view.Scope, RoleID: "role", Changes: map[string]json.RawMessage{"restoreHistoryRevision": body(1)}}); err != nil {
		t.Fatal(err)
	}
	current, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{Query: "city", Mode: "keyword"})
	if err != nil || len(current.Resources) != 1 {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	_, value, _, _, _ := memoryManagementText(current.Resources[0].Body)
	if value != "上海" {
		t.Fatalf("restored value=%q", value)
	}
}
