package business

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"strings"
	"testing"
)

func TestOwnedMemoryCandidatesReplayAndAtomicReview(t *testing.T) {
	for _, on := range []bool{false, true} {
		t.Run(map[bool]string{false: "Source", true: "Core"}[on], func(t *testing.T) {
			e, db, service, model := engineHarness(t)
			if on {
				if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
			}
			model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
			req := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "chat", RequestID: "chat-request", Message: "我喜欢茶"}
			chat, err := e.Run(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			model.extract = func(_ context.Context, in Inference, _ Generation) ([]DerivedMemory, error) {
				if in.Message != "我喜欢茶\n" {
					t.Fatalf("source=%q", in.Message)
				}
				return []DerivedMemory{{Kind: "fact", Key: "饮品", Body: json.RawMessage(`{"value":"茶","importance":7}`)}}, nil
			}
			before := model.extractions.Load()
			req.RequestID = "candidate"
			req.ExpectedScope = &chat.Scope
			input := MemoryCandidateInput{Action: "generate", ConversationID: "chat"}
			generated, err := e.ManageOwnedMemoryCandidates(t.Context(), req, input)
			if err != nil || len(generated.Resources) != 1 {
				t.Fatalf("generated=%+v err=%v", generated, err)
			}
			replay, err := e.ManageOwnedMemoryCandidates(t.Context(), req, input)
			if err != nil || replay.Scope.RequestID != "candidate" || replay.Acknowledgement.RequestID != "candidate|candidate-result" || model.extractions.Load() != before+1 {
				t.Fatalf("replay=%+v err=%v", replay, err)
			}
			memories, err := coordination.NewOwnershipStore(db, chat.Scope.ResourceOwnerID).List(t.Context(), "memory", "role", false)
			if err != nil || len(memories) != 0 {
				t.Fatal("candidate wrote memory before review")
			}
			req.RequestID = "accept"
			accepted, err := e.ManageOwnedMemoryCandidates(t.Context(), req, MemoryCandidateInput{Action: "accept", ID: generated.Resources[0].ID, ExpectedRevision: 1})
			if err != nil || !accepted.Saved || accepted.Acknowledgement.Versions["checkpoint/"+generated.Resources[0].ID] != 2 {
				t.Fatalf("accepted=%+v err=%v", accepted, err)
			}
			req.RequestID = "accept-again"
			if _, err := e.ManageOwnedMemoryCandidates(t.Context(), req, MemoryCandidateInput{Action: "accept", ID: generated.Resources[0].ID, ExpectedRevision: 1}); !errors.Is(err, coordination.ErrResourceVersion) {
				t.Fatalf("double review=%v", err)
			}
		})
	}
}

func TestOwnedMemoryCandidatesHistoricalSourceProvenance(t *testing.T) {
	e, db, service, model := engineHarness(t)
	e.data = originDataPort{testDataPort{db: db, role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{}`)}}}
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
	req := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "chat", RequestID: "chat", Message: "原设备的茶"}
	source, err := e.Run(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	req.RequestID = "history-candidate"
	req.ConversationOrigin = &ConversationOrigin{OwnerID: "a", ID: "chat"}
	current, err := e.Query(t.Context(), req, coordination.DataQuery{Management: true, ResourceKind: "memory", ConversationID: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	req.ExpectedScope = &current.Scope
	model.extract = func(_ context.Context, in Inference, _ Generation) ([]DerivedMemory, error) {
		if in.Message != "原设备的茶\n" {
			t.Fatal("wrong historical source")
		}
		return []DerivedMemory{{Kind: "fact", Key: "茶", Body: json.RawMessage(`{"value":"来源于设备"}`)}}, nil
	}
	generated, err := e.ManageOwnedMemoryCandidates(t.Context(), req, MemoryCandidateInput{Action: "generate", ConversationID: "chat"})
	if err != nil || len(generated.Resources) != 1 || generated.Resources[0].OwnerID != "core" {
		t.Fatalf("history=%+v err=%v", generated, err)
	}
	store := coordination.NewOwnershipStore(db, "a")
	messages, err := store.List(t.Context(), "message", "role", false)
	if err != nil {
		t.Fatal(err)
	}
	source.Scope.RequestID = "edit-source"
	_, err = store.Apply(t.Context(), coordination.Commit{Scope: source.Scope, Mutations: []coordination.Mutation{{Kind: "message", ID: messages[0].ID, RoleID: "role", ExpectedRevision: messages[0].Revision, Body: body(map[string]any{"conversationId": "chat", "role": "user", "content": "后来改成咖啡"})}}})
	if err != nil {
		t.Fatal(err)
	}
	req.RequestID = "accept-history"
	if _, err := e.ManageOwnedMemoryCandidates(t.Context(), req, MemoryCandidateInput{Action: "accept", ID: generated.Resources[0].ID, ExpectedRevision: 1}); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("stale provenance=%v", err)
	}
}

func TestOwnedMemoryCandidatesLateScopeAndUncertainRetry(t *testing.T) {
	for _, failure := range []string{"mode", "model"} {
		t.Run(failure, func(t *testing.T) {
			e, db, service, model := engineHarness(t)
			model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
			req := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "chat", RequestID: "chat", Message: "用户原消息"}
			chat, err := e.Run(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			req.RequestID = "candidate-late"
			req.ExpectedScope = &chat.Scope
			before := model.extractions.Load()
			model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) {
				if failure == "mode" {
					if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
						t.Fatal(err)
					}
					return []DerivedMemory{{Kind: "fact", Key: "late", Body: json.RawMessage(`{"value":"迟到结果"}`)}}, nil
				}
				return nil, errors.New("模型连接断开")
			}
			input := MemoryCandidateInput{Action: "generate", ConversationID: "chat"}
			if _, err := e.ManageOwnedMemoryCandidates(t.Context(), req, input); err == nil {
				t.Fatal("late generation saved")
			}
			if _, err := e.ManageOwnedMemoryCandidates(t.Context(), req, input); err == nil {
				t.Fatal("uncertain request replayed")
			}
			if model.extractions.Load() != before+1 {
				t.Fatal("uncertain inference repeated")
			}
			for _, owner := range []string{"a", "core"} {
				rows, err := coordination.NewOwnershipStore(db, owner).List(t.Context(), "checkpoint", "role", false)
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range rows {
					if strings.HasPrefix(row.ID, "memory-candidate/") {
						t.Fatal("late candidate checkpoint exists")
					}
				}
			}
		})
	}
}

type candidateOnlyModel struct {
	*testModel
	candidates int
}

func (m *candidateOnlyModel) ExtractOwnedMemoryCandidates(context.Context, Inference, Generation) ([]DerivedMemory, error) {
	m.candidates++
	return []DerivedMemory{{Kind: "profile", Key: "偏好", Body: json.RawMessage(`{"preference":"喜欢茶"}`)}, {Kind: "episodic", Key: "事件", Body: json.RawMessage(`{"event":"昨天见了朋友"}`)}}, nil
}

func TestOwnedMemoryCandidatesDedicatedModelAndStructuredContent(t *testing.T) {
	e, _, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
	req := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "chat", RequestID: "chat", Message: "茶与朋友"}
	chat, err := e.Run(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	dedicated := &candidateOnlyModel{testModel: model}
	e.model = dedicated
	before := model.extractions.Load()
	req.RequestID = "special-candidates"
	req.ExpectedScope = &chat.Scope
	result, err := e.ManageOwnedMemoryCandidates(t.Context(), req, MemoryCandidateInput{Action: "generate", ConversationID: "chat"})
	if err != nil || len(result.Resources) != 2 || dedicated.candidates != 1 || model.extractions.Load() != before {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, row := range result.Resources {
		var candidate OwnedMemoryCandidate
		if json.Unmarshal(row.Body, &candidate) != nil || candidate.Value == "" {
			t.Fatal("structured candidate text missing")
		}
		req.RequestID = "accept-" + candidate.DerivedKind
		accepted, err := e.ManageOwnedMemoryCandidates(t.Context(), req, MemoryCandidateInput{Action: "accept", ID: row.ID, ExpectedRevision: 1})
		if err != nil || !accepted.Saved {
			t.Fatalf("accept=%+v err=%v", accepted, err)
		}
	}
}
