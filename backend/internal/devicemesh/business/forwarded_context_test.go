package business

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestForwardedContextIsUsedWithoutCopyingHistoricalMessages(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	forwarded := &ForwardedContext{PreviousCoreID: "old-core", ConversationID: "conversation", Summary: "已有对话摘要", Messages: []ContextMessage{{ID: "old-message", OwnerID: "old-core", Role: "assistant", Content: "历史内容", Status: "completed"}}}
	model.generate = func(_ context.Context, inference Inference) (Generation, error) {
		if inference.Context == nil || inference.Context.Summary != forwarded.Summary || inference.Scope.CoreID != "core" || inference.Scope.RoleOwnerID != "a" {
			t.Fatalf("context or authority changed: %+v", inference)
		}
		return Generation{Text: "接续回复"}, nil
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "continued", Message: "继续", Context: forwarded}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "message", "role", false)
	if err != nil || len(rows) != 2 {
		t.Fatalf("historical data copied: %+v %v", rows, err)
	}
	forwarded.Messages[0].Content = "另一段历史"
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("changed context reused request: %v", err)
	}
	if model.calls.Load() != 1 {
		t.Fatal("same request generated twice")
	}
}

func TestForwardedContextRejectsInvalidAndUnboundedInput(t *testing.T) {
	valid := ForwardedContext{PreviousCoreID: "previous", ConversationID: "conversation", Messages: []ContextMessage{{ID: "one", Role: "user", Content: "hello"}}}
	for _, mutate := range []func(*ForwardedContext){
		func(c *ForwardedContext) { c.ConversationID = "other" },
		func(c *ForwardedContext) { c.PreviousCoreID = "" },
		func(c *ForwardedContext) { c.Summary = strings.Repeat("x", (64<<10)+1) },
		func(c *ForwardedContext) { c.Messages[0].Role = "system" },
		func(c *ForwardedContext) { c.Messages[0].Status = "sending" },
		func(c *ForwardedContext) { c.Messages[0].Content = strings.Repeat("x", (128<<10)+1) },
		func(c *ForwardedContext) { c.Messages = append(c.Messages, c.Messages[0]) },
	} {
		candidate := valid
		candidate.Messages = append([]ContextMessage(nil), valid.Messages...)
		mutate(&candidate)
		if err := validateForwardedContext(Request{ConversationID: "conversation", Context: &candidate}); err == nil {
			t.Fatalf("invalid context accepted: %+v", candidate)
		}
	}
	if err := validateForwardedContext(Request{ConversationID: "conversation", Context: &valid}); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryRecoveryRetainsForwardedContextWithoutGeneratingReplyAgain(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	forwarded := &ForwardedContext{PreviousCoreID: "previous-core", ConversationID: "conversation", Summary: "之前已经确认的摘要", Messages: []ContextMessage{{ID: "historical", Role: "assistant", Content: "历史回复"}}}
	model.extract = func(_ context.Context, inference Inference, _ Generation) ([]DerivedMemory, error) {
		if model.extractions.Load() == 1 {
			return nil, errors.New("temporary extraction failure")
		}
		if inference.Context == nil || inference.Context.Summary != forwarded.Summary || inference.Context.PreviousCoreID != "previous-core" {
			t.Fatal("recovered extraction lost forwarded context")
		}
		return nil, nil
	}
	response, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "continued-memory", Message: "继续", Context: forwarded})
	if err != nil || !response.Saved || response.MemoryStatus != "failed" {
		t.Fatalf("initial reply not saved: %+v %v", response, err)
	}
	if _, err := db.Exec(`UPDATE kernel_device_memory_jobs SET next_attempt_at=0`); err != nil {
		t.Fatal(err)
	}
	jobs, err := service.PendingMemoryJobs(t.Context())
	if err != nil || len(jobs) != 1 {
		t.Fatalf("memory retry missing: %+v %v", jobs, err)
	}
	recovered, err := engine.ResumeMemory(t.Context(), jobs[0])
	if err != nil || recovered.MemoryStatus != "saved" || model.calls.Load() != 1 || model.extractions.Load() != 2 {
		t.Fatalf("recovery repeated reply or failed: %+v %v", recovered, err)
	}
}
