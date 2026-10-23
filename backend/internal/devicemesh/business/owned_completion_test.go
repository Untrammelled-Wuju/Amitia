package business

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type completionFailurePort struct{ coordination.DataPort }

func (p completionFailurePort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	for _, mutation := range commit.Mutations {
		if mutation.Kind == "message" && strings.HasSuffix(mutation.ID, "/assistant") {
			return coordination.Acknowledgement{}, coordination.ErrWrongOwner
		}
	}
	return p.DataPort.Commit(ctx, commit)
}

func TestOwnedCompletionRunsAfterOwnerCommitAndPreservesFailure(t *testing.T) {
	for _, scenario := range []string{"saved", "commit-failed", "model-failed"} {
		t.Run(scenario, func(t *testing.T) {
			engine, db, _, model := engineHarness(t)
			if scenario == "commit-failed" {
				engine.data = completionFailurePort{engine.data}
			}
			called := 0
			model.generate = func(_ context.Context, inference Inference) (Generation, error) {
				if inference.RegisterCompletion == nil {
					t.Fatal("engine omitted turn completion")
				}
				inference.RegisterCompletion(func(ctx context.Context, generation Generation, cause error) error {
					called++
					row, err := coordination.NewOwnershipStore(db, inference.Scope.ResourceOwnerID).Get(ctx, "message", "completion/assistant")
					if err != nil {
						t.Fatal(err)
					}
					if scenario == "saved" {
						if cause != nil || row == nil || generation.Text != "confirmed" {
							t.Fatalf("completion preceded save: %+v %v", generation, cause)
						}
					} else if cause == nil || row != nil {
						t.Fatalf("failed reply appeared saved: row=%+v cause=%v", row, cause)
					}
					return nil
				})
				if scenario == "model-failed" {
					return Generation{}, errors.New("controlled model failure")
				}
				return Generation{Text: "confirmed"}, nil
			}
			_, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "conversation", RequestID: "completion", Message: "check commit"})
			if called != 1 || (err == nil) != (scenario == "saved") {
				t.Fatalf("completion=%d err=%v", called, err)
			}
			if scenario == "model-failed" {
				row, getErr := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "checkpoint", "turn/completion")
				if getErr != nil || row == nil {
					t.Fatalf("failed checkpoint missing: %+v %v", row, getErr)
				}
				var checkpoint struct {
					Status string `json:"status"`
				}
				if json.Unmarshal(row.Body, &checkpoint) != nil || checkpoint.Status != "failed" {
					t.Fatalf("failed checkpoint status: %+v", checkpoint)
				}
				if errors.Is(err, coordination.ErrWrongOwner) {
					t.Fatalf("model failure was replaced by authorization error: %v", err)
				}
			}
		})
	}
}

func TestOwnedReplyNormalizesEmptyCitationSnapshot(t *testing.T) {
	for _, value := range []string{"", "null", " null ", "[]"} {
		t.Run(value, func(t *testing.T) {
			engine, db, _, model := engineHarness(t)
			model.generate = func(context.Context, Inference) (Generation, error) {
				return Generation{Text: "confirmed", CitationSnapshot: json.RawMessage(value)}, nil
			}
			response, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "conversation", RequestID: "empty-citations", Message: "check completion"})
			if err != nil || !response.Saved || string(response.CitationSnapshot) != "[]" {
				t.Fatalf("response=%+v error=%v", response, err)
			}
			raw, err := json.Marshal(response)
			if err != nil || !strings.Contains(string(raw), `"citationSnapshot":[]`) {
				t.Fatalf("completion JSON=%s error=%v", raw, err)
			}
			row, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "message", "empty-citations/assistant")
			if err != nil || row == nil || !strings.Contains(string(row.Body), `"citationSnapshot":[]`) {
				t.Fatalf("saved message=%+v error=%v", row, err)
			}
		})
	}
}

type phaseTitleModel struct {
	*testModel
	titles int
}

func (m *phaseTitleModel) PlanOwnedTitle(context.Context, Inference, Generation) (string, error) {
	m.titles++
	return "测试对话", nil
}

func TestStreamingReplyCompletesBeforeDurablePostProcessing(t *testing.T) {
	engine, db, coordinator, model := engineHarness(t)
	titles := &phaseTitleModel{testModel: model}
	engine.model = titles
	model.generate = func(ctx context.Context, inference Inference) (Generation, error) {
		if err := inference.Emit(Event{Type: "delta", Text: "reason", Reasoning: true}); err != nil {
			return Generation{}, err
		}
		if err := inference.Emit(Event{Type: "delta", Text: "reply"}); err != nil {
			return Generation{}, err
		}
		return Generation{Text: "reply", Reasoning: "reason"}, nil
	}
	var events []string
	response, err := engine.RunEvents(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "conversation", RequestID: "phases", Message: "你好"}, func(event Event) error {
		events = append(events, event.Type)
		if event.Type == "output-completed" && model.extractions.Load() != 0 {
			t.Fatal("output completion waited for memory")
		}
		return nil
	})
	if err != nil || !response.Saved || response.MemoryStatus != "pending" || model.extractions.Load() != 0 || titles.titles != 0 {
		t.Fatalf("response=%+v error=%v", response, err)
	}
	if strings.Join(events, ",") != "started,delta,delta,reasoning-completed,output-completed" {
		t.Fatalf("events=%v", events)
	}
	if _, err := db.Exec(`UPDATE kernel_device_memory_jobs SET next_attempt_at=0`); err != nil {
		t.Fatal(err)
	}
	jobs, err := coordinator.PendingMemoryJobs(t.Context())
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs=%v error=%v", jobs, err)
	}
	resumed, err := engine.ResumeMemory(t.Context(), jobs[0])
	if err != nil || resumed.MemoryStatus != "saved" || titles.titles != 1 || model.extractions.Load() != 1 {
		t.Fatalf("resumed=%+v titles=%d error=%v", resumed, titles.titles, err)
	}
}
