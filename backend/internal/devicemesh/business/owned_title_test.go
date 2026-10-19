package business

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedTitleTestModel struct {
	*testModel
	plan func(context.Context, Inference, Generation) (string, error)
}

func (m ownedTitleTestModel) PlanOwnedTitle(ctx context.Context, inference Inference, generation Generation) (string, error) {
	return m.plan(ctx, inference, generation)
}

func TestOwnedTitlePersistsOnlyOnResourceOwner(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	var inference Inference
	model.generate = func(_ context.Context, input Inference) (Generation, error) {
		inference = input
		return Generation{Text: "这里是具体说明"}, nil
	}
	response, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "title", Message: "介绍项目"})
	if err != nil {
		t.Fatal(err)
	}
	engine.model = ownedTitleTestModel{testModel: model, plan: func(context.Context, Inference, Generation) (string, error) { return "项目介绍", nil }}
	if err := engine.finishOwnedTitle(coordination.WithScope(t.Context(), response.Scope), inference, response.Generation); err != nil {
		t.Fatal(err)
	}
	resource, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "conversation", "conversation")
	if err != nil || resource == nil {
		t.Fatalf("owner title missing: %+v %v", resource, err)
	}
	var saved struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(resource.Body, &saved); err != nil || saved.Title != "项目介绍" {
		t.Fatalf("bad saved title: %s %v", resource.Body, err)
	}
	mirror, err := coordination.NewOwnershipStore(db, "core").Get(t.Context(), "conversation", "conversation")
	if err != nil || mirror != nil {
		t.Fatalf("Core has owner title mirror: %+v %v", mirror, err)
	}
}

func TestOwnedTitleDoesNotOverwriteConcurrentRename(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	var inference Inference
	model.generate = func(_ context.Context, input Inference) (Generation, error) {
		inference = input
		return Generation{Text: "说明"}, nil
	}
	response, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "rename", Message: "介绍项目"})
	if err != nil {
		t.Fatal(err)
	}
	store := coordination.NewOwnershipStore(db, "a")
	engine.model = ownedTitleTestModel{testModel: model, plan: func(ctx context.Context, _ Inference, _ Generation) (string, error) {
		resource, err := store.Get(ctx, "conversation", "conversation")
		if err != nil {
			return "", err
		}
		var content map[string]json.RawMessage
		if err := json.Unmarshal(resource.Body, &content); err != nil {
			return "", err
		}
		content["title"] = body("用户命名")
		scope := response.Scope
		scope.RequestID += "|manual-title"
		_, err = store.Apply(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "conversation", ID: resource.ID, RoleID: scope.RoleID, ExpectedRevision: resource.Revision, Body: body(content)}}})
		return "自动标题", err
	}}
	if err := engine.finishOwnedTitle(coordination.WithScope(t.Context(), response.Scope), inference, response.Generation); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("expected concurrent revision rejection: %v", err)
	}
	resource, err := store.Get(t.Context(), "conversation", "conversation")
	if err != nil {
		t.Fatal(err)
	}
	var content struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(resource.Body, &content); err != nil || content.Title != "用户命名" {
		t.Fatalf("manual title overwritten: %s %v", resource.Body, err)
	}
}

func TestOwnedTitleRejectsModeChangeDuringGeneration(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	var inference Inference
	model.generate = func(_ context.Context, input Inference) (Generation, error) {
		inference = input
		return Generation{Text: "说明"}, nil
	}
	response, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "mode-title", Message: "介绍项目"})
	if err != nil {
		t.Fatal(err)
	}
	engine.model = ownedTitleTestModel{testModel: model, plan: func(ctx context.Context, _ Inference, _ Generation) (string, error) {
		_, err := service.ChangeMode(ctx, "core", "a", 1, true, "role")
		return "自动标题", err
	}}
	if err := engine.finishOwnedTitle(coordination.WithScope(t.Context(), response.Scope), inference, response.Generation); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("expected changed authority rejection: %v", err)
	}
	resource, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "conversation", "conversation")
	if err != nil || resource == nil {
		t.Fatalf("owner missing: %v", err)
	}
	var content struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(resource.Body, &content); err != nil || content.Title != "介绍项目" {
		t.Fatalf("expired title was saved: %s %v", resource.Body, err)
	}
}
