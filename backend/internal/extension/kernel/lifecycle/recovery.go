package lifecycle

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type RecoveryReport struct {
	StartupID              string
	ScannedAt              time.Time
	CleanShutdown          bool
	PendingPackageOps      []string
	PendingRuntimeStates   []string
	PendingWorkflowStates  []string
	PendingMCPStates       []string
	PendingSchedules       []string
	OrphanResources        []string
	InterruptedComponents  []string
	HighRiskItems          []RecoveryItem
	Items                  []RecoveryItem
}

type RecoveryItem struct {
	Category    string
	ComponentID string
	Subject     string
	Severity    string
	Action      string
	Metadata    map[string]any
}

type RecoveryScanner struct {
	journal   *Journal
	scanHooks []ScanHook
}

type ScanHook func(ctx context.Context, startupID string) ([]RecoveryItem, error)

func NewRecoveryScanner(journal *Journal) *RecoveryScanner {
	return &RecoveryScanner{journal: journal}
}

func (s *RecoveryScanner) RegisterScanHook(hook ScanHook) {
	s.scanHooks = append(s.scanHooks, hook)
}

func (s *RecoveryScanner) Scan(ctx context.Context, startupID string) (*RecoveryReport, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	report := &RecoveryReport{
		StartupID:             startupID,
		ScannedAt:             journalTimeNow(),
		CleanShutdown:         s.journal.IsCleanShutdown(),
		InterruptedComponents: s.journal.InterruptedComponents(),
	}
	for _, hook := range s.scanHooks {
		items, err := hook(ctx, startupID)
		if err != nil {
			return nil, fmt.Errorf("recovery scan hook failed: %w", err)
		}
		for _, item := range items {
			report.Items = append(report.Items, item)
			switch item.Category {
			case "package":
				report.PendingPackageOps = append(report.PendingPackageOps, item.Subject)
			case "runtime":
				report.PendingRuntimeStates = append(report.PendingRuntimeStates, item.Subject)
			case "workflow":
				report.PendingWorkflowStates = append(report.PendingWorkflowStates, item.Subject)
			case "mcp":
				report.PendingMCPStates = append(report.PendingMCPStates, item.Subject)
			case "schedule":
				report.PendingSchedules = append(report.PendingSchedules, item.Subject)
			case "resource":
				report.OrphanResources = append(report.OrphanResources, item.Subject)
			}
			if item.Severity == "high" || item.Severity == "critical" {
				report.HighRiskItems = append(report.HighRiskItems, item)
			}
		}
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].Severity != report.Items[j].Severity {
			return severityRank(report.Items[i].Severity) > severityRank(report.Items[j].Severity)
		}
		return report.Items[i].Category < report.Items[j].Category
	})
	return report, nil
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 4
	case "high":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

type ReconciliationReport struct {
	Conflicts []ReconciliationConflict
	GeneratedAt time.Time
}

type ReconciliationConflict struct {
	Type        string
	ComponentID string
	Subject     string
	Severity    string
	Detail      string
	RecommendedAction string
	Metadata    map[string]any
}

type ReconciliationPlan struct {
	Actions []ReconciliationAction
}

type ReconciliationAction struct {
	ComponentID string
	Subject     string
	Type        string
	Severity    string
	Action      string
	Metadata    map[string]any
}

type ReconciliationResult struct {
	Applied []string
	Failed  []string
	Skipped []string
}

type StateReconciler interface {
	Inspect(ctx context.Context) ReconciliationReport
	Plan(ctx context.Context, report ReconciliationReport) ReconciliationPlan
	Apply(ctx context.Context, plan ReconciliationPlan) ReconciliationResult
}

type DefaultReconciler struct {
	inspectHooks []func(ctx context.Context) []ReconciliationConflict
	applyHooks   []func(ctx context.Context, action ReconciliationAction) error
}

func NewDefaultReconciler() *DefaultReconciler {
	return &DefaultReconciler{}
}

func (r *DefaultReconciler) RegisterInspect(hook func(ctx context.Context) []ReconciliationConflict) {
	r.inspectHooks = append(r.inspectHooks, hook)
}

func (r *DefaultReconciler) RegisterApply(hook func(ctx context.Context, action ReconciliationAction) error) {
	r.applyHooks = append(r.applyHooks, hook)
}

func (r *DefaultReconciler) Inspect(ctx context.Context) ReconciliationReport {
	report := ReconciliationReport{GeneratedAt: now()}
	for _, hook := range r.inspectHooks {
		conflicts := hook(ctx)
		report.Conflicts = append(report.Conflicts, conflicts...)
	}
	return report
}

func (r *DefaultReconciler) Plan(_ context.Context, report ReconciliationReport) ReconciliationPlan {
	plan := ReconciliationPlan{}
	for _, c := range report.Conflicts {
		action := ReconciliationAction{
			ComponentID: c.ComponentID,
			Subject:     c.Subject,
			Type:        c.Type,
			Severity:    c.Severity,
			Action:      c.RecommendedAction,
			Metadata:    c.Metadata,
		}
		if action.Action == "" {
			action.Action = "manual_review"
		}
		plan.Actions = append(plan.Actions, action)
	}
	return plan
}

func (r *DefaultReconciler) Apply(ctx context.Context, plan ReconciliationPlan) ReconciliationResult {
	result := ReconciliationResult{}
	for _, action := range plan.Actions {
		applied := false
		for _, hook := range r.applyHooks {
			if err := hook(ctx, action); err != nil {
				result.Failed = append(result.Failed, action.ComponentID+":"+action.Action)
				continue
			}
			applied = true
		}
		if applied {
			result.Applied = append(result.Applied, action.ComponentID+":"+action.Action)
		} else if len(r.applyHooks) == 0 {
			result.Skipped = append(result.Skipped, action.ComponentID+":"+action.Action)
		}
	}
	return result
}

var _ StateReconciler = (*DefaultReconciler)(nil)
