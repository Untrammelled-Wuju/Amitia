package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type WorkflowConcurrencyMode string

const (
	WorkflowConcurrencyAllow     WorkflowConcurrencyMode = "ALLOW"
	WorkflowConcurrencySingleton WorkflowConcurrencyMode = "SINGLETON"
	WorkflowConcurrencyQueue     WorkflowConcurrencyMode = "QUEUE"
	WorkflowConcurrencyReplace   WorkflowConcurrencyMode = "REPLACE"
	WorkflowConcurrencyDrop      WorkflowConcurrencyMode = "DROP"
	WorkflowConcurrencyMaxN      WorkflowConcurrencyMode = "MAX_N"
)

type WorkflowConcurrencyPolicy struct {
	Mode WorkflowConcurrencyMode `json:"mode,omitempty"`
	MaxN int                     `json:"maxN,omitempty"`
}

func (p WorkflowConcurrencyPolicy) Normalize() WorkflowConcurrencyPolicy {
	p.Mode = WorkflowConcurrencyMode(strings.ToUpper(strings.TrimSpace(string(p.Mode))))
	if p.Mode == "" {
		p.Mode = WorkflowConcurrencyAllow
	}
	if p.Mode == WorkflowConcurrencyMaxN && p.MaxN < 1 {
		p.MaxN = 1
	}
	return p
}

func (p WorkflowConcurrencyPolicy) Validate() error {
	p = p.Normalize()
	switch p.Mode {
	case WorkflowConcurrencyAllow, WorkflowConcurrencySingleton, WorkflowConcurrencyQueue, WorkflowConcurrencyReplace, WorkflowConcurrencyDrop:
		return nil
	case WorkflowConcurrencyMaxN:
		if p.MaxN < 1 {
			return errors.New("workflow concurrency MAX_N requires maxN >= 1")
		}
		return nil
	default:
		return fmt.Errorf("workflow concurrency mode %q is invalid", p.Mode)
	}
}

// WorkflowConcurrencyStore is optional so in-memory/lightweight executors keep
// working. Production SQLite implements it to make QUEUE/REPLACE/MAX_N durable.
type WorkflowConcurrencyStore interface {
	ListActiveWorkflowRuns(ctx context.Context, workflowID string, limit int) ([]WorkflowRun, error)
	EnqueueWorkflowRun(ctx context.Context, run WorkflowRun) error
	ListQueuedWorkflowRuns(ctx context.Context, workflowID string, limit int) ([]WorkflowRun, error)
	DropQueuedWorkflowRuns(ctx context.Context, workflowID, reason string, at time.Time) (int, error)
}
