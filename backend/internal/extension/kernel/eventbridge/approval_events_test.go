package eventbridge_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/eventbridge"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
)

type recordedEvent struct {
	TypeID  event.EventTypeID
	Payload json.RawMessage
	Opts    event.PublishOptions
}

type stubDurablePublisher struct {
	events []recordedEvent
	fail   bool
}

func (s *stubDurablePublisher) Publish(_ context.Context, typeID event.EventTypeID, _ int, payload json.RawMessage, opts event.PublishOptions) (event.PublishResult, error) {
	if s.fail {
		return event.PublishResult{}, errors.New("durable unavailable")
	}
	s.events = append(s.events, recordedEvent{TypeID: typeID, Payload: payload, Opts: opts})
	return event.PublishResult{EventID: "event-1", OutboxID: "outbox-1"}, nil
}

func (s *stubDurablePublisher) PublishTx(ctx context.Context, _ *sql.Tx, typeID event.EventTypeID, version int, payload json.RawMessage, opts event.PublishOptions) (event.PublishResult, error) {
	return s.Publish(ctx, typeID, version, payload, opts)
}

func newApprovalPublisher(t *testing.T, durable *stubDurablePublisher) *eventbridge.ApprovalEventPublisher {
	t.Helper()
	publisher, err := eventbridge.NewPublisher(durable)
	if err != nil {
		t.Fatal(err)
	}
	return eventbridge.NewApprovalEventPublisher(publisher)
}

func TestApprovalEventPublisherPublishesRequestAndResolution(t *testing.T) {
	durable := &stubDurablePublisher{}
	publisher := newApprovalPublisher(t, durable)
	request := execution.ApprovalRequest{
		ID: "approval-1", SpaceID: "space-1", ConversationID: "conv-1", RequestID: "request-1",
		ToolName: "shell.exec", RiskLevel: "high", Status: execution.ApprovalStatusPending,
	}
	if err := publisher.Requested(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(durable.events) != 1 || durable.events[0].TypeID != "approval.requested" {
		t.Fatalf("审批请求未进入持久事件流: %+v", durable.events)
	}
	if durable.events[0].Opts.AggregateID != "approval-1" || durable.events[0].Opts.AggregateType != "approval" || durable.events[0].Opts.PartitionKey != "space-1" {
		t.Fatalf("审批事件归属错误: %+v", durable.events[0].Opts)
	}
	var payload eventbridge.ApprovalEventPayload
	if err := json.Unmarshal(durable.events[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ApprovalID != "approval-1" || payload.ToolName != "shell.exec" || payload.Status != string(execution.ApprovalStatusPending) {
		t.Fatalf("审批事件载荷错误: %+v", payload)
	}

	request.Status = execution.ApprovalStatusApproved
	request.ResolvedBy, request.ResolvedByType = "user-1", "user"
	if err := publisher.Resolved(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(durable.events) != 2 || durable.events[1].TypeID != "approval.resolved" {
		t.Fatalf("审批结果未进入持久事件流: %+v", durable.events)
	}
}

func TestApprovalEventPublisherDeduplicatesAndRetries(t *testing.T) {
	durable := &stubDurablePublisher{}
	publisher := newApprovalPublisher(t, durable)
	request := execution.ApprovalRequest{ID: "approval-2", SpaceID: "space-1", Status: execution.ApprovalStatusPending}
	if err := publisher.Requested(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Requested(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(durable.events) != 1 {
		t.Fatalf("重复通知产生了重复事件: %d", len(durable.events))
	}

	failing := &stubDurablePublisher{fail: true}
	retry := newApprovalPublisher(t, failing)
	if err := retry.Requested(context.Background(), request); err == nil {
		t.Fatal("持久投递失败被吞掉")
	}
	failing.fail = false
	if err := retry.Requested(context.Background(), request); err != nil {
		t.Fatalf("投递失败后未允许重试: %v", err)
	}
	if len(failing.events) != 1 {
		t.Fatalf("重试未重新投递: %d", len(failing.events))
	}
}
