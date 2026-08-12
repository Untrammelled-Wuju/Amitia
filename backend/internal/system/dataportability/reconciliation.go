package dataportability

import "context"

type ReconcileScope string

const (
	ReconcileAll       ReconcileScope = "all"
	ReconcileVectors   ReconcileScope = "vectors"
	ReconcileGraph     ReconcileScope = "graph"
	ReconcileMessages  ReconcileScope = "messages"
	ReconcileMemories  ReconcileScope = "memories"
)

type ReconcileCheck struct {
	Scope     ReconcileScope `json:"scope"`
	Healthy   bool           `json:"healthy"`
	Pending   bool           `json:"pending"`
	Details   string         `json:"details,omitempty"`
	IssueCount int64         `json:"issueCount"`
}

type Reconciler interface {
	ID() string
	Check(ctx context.Context, scope ReconcileScope) (ReconcileCheck, error)
	Repair(ctx context.Context, scope ReconcileScope) error
}

type ReconcileManager struct {
	reconcilers []Reconciler
}

func NewReconcileManager() *ReconcileManager {
	return &ReconcileManager{reconcilers: make([]Reconciler, 0)}
}

func (m *ReconcileManager) Register(r Reconciler) {
	m.reconcilers = append(m.reconcilers, r)
}

func (m *ReconcileManager) Reconcile(ctx context.Context, scope ReconcileScope) []ReconcileCheck {
	var results []ReconcileCheck
	for _, r := range m.reconcilers {
		check, err := r.Check(ctx, scope)
		if err != nil {
			results = append(results, ReconcileCheck{
				Scope:   scope,
				Healthy: false,
				Details: err.Error(),
			})
			continue
		}
		if !check.Healthy {
			if err := r.Repair(ctx, scope); err != nil {
				check.Details = err.Error()
				check.Pending = true
			} else {
				check.Healthy = true
				check.Pending = false
			}
		}
		results = append(results, check)
	}
	return results
}

func (m *ReconcileManager) HasPending() bool {
	return false
}

type VectorReconciler struct{}

func (VectorReconciler) ID() string { return "vector-reconciler" }

func (VectorReconciler) Check(ctx context.Context, scope ReconcileScope) (ReconcileCheck, error) {
	return ReconcileCheck{Scope: scope, Healthy: true, Pending: false}, nil
}

func (VectorReconciler) Repair(ctx context.Context, scope ReconcileScope) error {
	_ = ctx
	_ = scope
	return nil
}

type GraphReconciler struct{}

func (GraphReconciler) ID() string { return "graph-reconciler" }

func (GraphReconciler) Check(ctx context.Context, scope ReconcileScope) (ReconcileCheck, error) {
	return ReconcileCheck{Scope: scope, Healthy: true, Pending: false}, nil
}

func (GraphReconciler) Repair(ctx context.Context, scope ReconcileScope) error {
	_ = ctx
	_ = scope
	return nil
}

type ConversationReconciler struct{}

func (ConversationReconciler) ID() string { return "conversation-reconciler" }

func (ConversationReconciler) Check(ctx context.Context, scope ReconcileScope) (ReconcileCheck, error) {
	return ReconcileCheck{Scope: scope, Healthy: true, Pending: false}, nil
}

func (ConversationReconciler) Repair(ctx context.Context, scope ReconcileScope) error {
	_ = ctx
	_ = scope
	return nil
}
