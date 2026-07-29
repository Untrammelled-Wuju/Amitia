package observability

import (
	"context"
	"sort"
)

type QueryService interface {
	GetInvocationTree(ctx context.Context, rootID string) (*InvocationNode, error)
	GetTraceTimeline(ctx context.Context, traceID string) (*TraceTimeline, error)
	ListOperations(ctx context.Context, filter OperationFilter) ([]OperationRecord, string, error)
	ListInvocations(ctx context.Context, filter InvocationFilter) ([]InvocationRecord, string, error)
	ListRuntimeEvents(ctx context.Context, filter EventFilter) ([]RuntimeEventRecord, string, error)
	ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, string, error)
	GetInvocation(ctx context.Context, invocationID string) (*InvocationRecord, error)
	GetAuditEvent(ctx context.Context, auditID string) (*AuditEvent, error)
}

type InvocationNode struct {
	Invocation InvocationRecord   `json:"invocation"`
	Attempts   []ExecutionAttempt `json:"attempts,omitempty"`
	Children   []InvocationNode   `json:"children,omitempty"`
}

type TraceTimeline struct {
	Trace       Trace                `json:"trace"`
	Operations  []OperationRecord    `json:"operations"`
	Invocations []InvocationRecord   `json:"invocations"`
	Events      []RuntimeEventRecord `json:"events"`
}

type DefaultQueryService struct {
	store StorageBackend
}

func NewQueryService(store StorageBackend) *DefaultQueryService {
	return &DefaultQueryService{store: store}
}

func (q *DefaultQueryService) GetInvocationTree(ctx context.Context, rootID string) (*InvocationNode, error) {
	root, err := q.store.GetInvocation(ctx, rootID)
	if err != nil {
		return nil, err
	}

	attempts, _ := q.store.ListAttemptsByInvocation(ctx, rootID)
	node := &InvocationNode{
		Invocation: *root,
		Attempts:   attempts,
	}

	children, err := q.store.GetInvocationChildren(ctx, rootID)
	if err != nil {
		return node, nil
	}

	for _, child := range children {
		childNode, err := q.GetInvocationTree(ctx, child.InvocationID)
		if err == nil {
			node.Children = append(node.Children, *childNode)
		}
	}

	return node, nil
}

func (q *DefaultQueryService) GetTraceTimeline(ctx context.Context, traceID string) (*TraceTimeline, error) {
	trace, err := q.store.GetTrace(ctx, traceID)
	if err != nil {
		return nil, err
	}

	ops, _, _ := q.store.ListOperations(ctx, OperationFilter{TraceID: traceID, ListOptions: ListOptions{Limit: 1000}})
	invs, _, _ := q.store.ListInvocations(ctx, InvocationFilter{TraceID: traceID, ListOptions: ListOptions{Limit: 1000}})
	events, _, _ := q.store.ListRuntimeEvents(ctx, EventFilter{TraceID: traceID, ListOptions: ListOptions{Limit: 1000}})

	sort.Slice(ops, func(i, j int) bool { return ops[i].StartedAt.Before(ops[j].StartedAt) })
	sort.Slice(invs, func(i, j int) bool { return invs[i].CreatedAt.Before(invs[j].CreatedAt) })
	sort.Slice(events, func(i, j int) bool { return events[i].Timestamp.Before(events[j].Timestamp) })

	return &TraceTimeline{
		Trace:       *trace,
		Operations:  ops,
		Invocations: invs,
		Events:      events,
	}, nil
}

func (q *DefaultQueryService) ListOperations(ctx context.Context, filter OperationFilter) ([]OperationRecord, string, error) {
	return q.store.ListOperations(ctx, filter)
}

func (q *DefaultQueryService) ListInvocations(ctx context.Context, filter InvocationFilter) ([]InvocationRecord, string, error) {
	return q.store.ListInvocations(ctx, filter)
}

func (q *DefaultQueryService) ListRuntimeEvents(ctx context.Context, filter EventFilter) ([]RuntimeEventRecord, string, error) {
	return q.store.ListRuntimeEvents(ctx, filter)
}

func (q *DefaultQueryService) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, string, error) {
	return q.store.ListAuditEvents(ctx, filter)
}

func (q *DefaultQueryService) GetInvocation(ctx context.Context, invocationID string) (*InvocationRecord, error) {
	return q.store.GetInvocation(ctx, invocationID)
}

func (q *DefaultQueryService) GetAuditEvent(ctx context.Context, auditID string) (*AuditEvent, error) {
	return q.store.GetAuditEvent(ctx, auditID)
}
