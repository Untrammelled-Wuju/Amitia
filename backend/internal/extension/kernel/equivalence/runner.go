package equivalence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type EquivalenceResult string

const (
	ResultEquivalent          EquivalenceResult = "equivalent"
	ResultImproved            EquivalenceResult = "improved"
	ResultIntentionallyChanged EquivalenceResult = "intentionally_changed"
	ResultMissing             EquivalenceResult = "missing"
	ResultRegressed           EquivalenceResult = "regressed"
	ResultNotApplicable       EquivalenceResult = "not_applicable"
	ResultBlocked             EquivalenceResult = "blocked"
)

type Category string

const (
	CategoryBuiltinTools       Category = "builtin_tools"
	CategoryAgentSkills        Category = "agent_skills"
	CategoryMCP                Category = "mcp"
	CategoryWorkflows          Category = "workflows"
	CategoryPlugins            Category = "plugins"
	CategoryLegacyAmitiax      Category = "legacy_amitiax"
	CategoryInstallation       Category = "installation"
	CategoryEnablement         Category = "enablement"
	CategoryUpdate             Category = "update"
	CategoryRollback           Category = "rollback"
	CategoryUninstall          Category = "uninstall"
	CategoryPermission         Category = "permission"
	CategoryScope              Category = "scope"
	CategoryStorage            Category = "storage"
	CategorySecret             Category = "secret"
	CategoryEvent              Category = "event"
	CategoryHook               Category = "hook"
	CategorySchedule           Category = "schedule"
	CategoryBackgroundTask     Category = "background_task"
	CategoryUIContribution     Category = "ui_contribution"
	CategoryDesktopContribution Category = "desktop_contribution"
	CategoryExtensionCenter    Category = "extension_center"
	CategoryExtensionDetail    Category = "extension_detail"
	CategoryDevMode            Category = "dev_mode"
	CategoryMigrationData      Category = "migration_data"
	CategoryRunHistory         Category = "run_history"
	CategoryResourceCleanup    Category = "resource_cleanup"
	CategoryLifecycle          Category = "lifecycle"
)

type CheckStatus string

const (
	CheckStatusPending  CheckStatus = "pending"
	CheckStatusRunning  CheckStatus = "running"
	CheckStatusPassed   CheckStatus = "passed"
	CheckStatusFailed   CheckStatus = "failed"
	CheckStatusSkipped  CheckStatus = "skipped"
	CheckStatusBlocked  CheckStatus = "blocked"
)

type Check struct {
	CheckID      string            `json:"checkId"`
	Category     Category          `json:"category"`
	Subject      string            `json:"subject"`
	Description  string            `json:"description"`
	Status       CheckStatus       `json:"status"`
	Result       EquivalenceResult `json:"result,omitempty"`
	Expected     string            `json:"expected,omitempty"`
	Actual       string            `json:"actual,omitempty"`
	Evidence     []Evidence        `json:"evidence,omitempty"`
	DurationMs   int64             `json:"durationMs,omitempty"`
	StartedAt    *time.Time        `json:"startedAt,omitempty"`
	CompletedAt  *time.Time        `json:"completedAt,omitempty"`
	Error        string            `json:"error,omitempty"`
}

type Evidence struct {
	Kind    string `json:"kind"`
	Content string `json:"content"`
	Source  string `json:"source,omitempty"`
}

type Report struct {
	ReportID    string         `json:"reportId"`
	GeneratedAt time.Time      `json:"generatedAt"`
	StartedAt   time.Time      `json:"startedAt"`
	EndedAt     *time.Time     `json:"endedAt,omitempty"`
	Checks      []Check        `json:"checks"`
	Summary     ReportSummary  `json:"summary"`
	Outcome     string         `json:"outcome"`
	Notes       []string       `json:"notes,omitempty"`
}

type ReportSummary struct {
	Total     int `json:"total"`
	Passed    int `json:"passed"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
	Blocked   int `json:"blocked"`
	Pending   int `json:"pending"`
	Equivalent int `json:"equivalent"`
	Improved  int `json:"improved"`
	Changed   int `json:"intentionally_changed"`
	Missing   int `json:"missing"`
	Regressed int `json:"regressed"`
	NotApplicable int `json:"not_applicable"`
}

type CheckFn func(ctx context.Context) (EquivalenceResult, []Evidence, error)

type Runner struct {
	mu     sync.Mutex
	checks []Check
	fns    map[string]CheckFn
}

func NewRunner() *Runner {
	return &Runner{fns: make(map[string]CheckFn)}
}

func (r *Runner) Register(check Check, fn CheckFn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks = append(r.checks, check)
	if fn != nil {
		r.fns[check.CheckID] = fn
	}
}

func (r *Runner) Run(ctx context.Context) (*Report, error) {
	r.mu.Lock()
	checks := make([]Check, len(r.checks))
	copy(checks, r.checks)
	fns := make(map[string]CheckFn, len(r.fns))
	for k, v := range r.fns {
		fns[k] = v
	}
	r.mu.Unlock()

	start := time.Now().UTC()
	for i := range checks {
		c := &checks[i]
		c.Status = CheckStatusRunning
		c.StartedAt = ptrTime(time.Now().UTC())
		fn, ok := fns[c.CheckID]
		if !ok {
			c.Status = CheckStatusSkipped
			c.CompletedAt = ptrTime(time.Now().UTC())
			continue
		}
		startTs := time.Now()
		result, evidence, err := fn(ctx)
		c.DurationMs = time.Since(startTs).Milliseconds()
		c.CompletedAt = ptrTime(time.Now().UTC())
		if err != nil {
			c.Status = CheckStatusFailed
			c.Error = err.Error()
			c.Result = ResultRegressed
			continue
		}
		c.Result = result
		c.Evidence = evidence
		switch result {
		case ResultEquivalent, ResultImproved, ResultIntentionallyChanged, ResultNotApplicable:
			c.Status = CheckStatusPassed
		case ResultMissing, ResultRegressed, ResultBlocked:
			c.Status = CheckStatusFailed
		}
	}

	end := time.Now().UTC()
	summary := summarize(checks)
	outcome := "passed"
	if summary.Failed > 0 || summary.Blocked > 0 {
		outcome = "failed"
	}
	return &Report{
		ReportID:    fmt.Sprintf("equiv-%d", start.UnixNano()),
		GeneratedAt: time.Now().UTC(),
		StartedAt:   start,
		EndedAt:     &end,
		Checks:      checks,
		Summary:     summary,
		Outcome:     outcome,
	}, nil
}

func ptrTime(t time.Time) *time.Time { return &t }

func summarize(checks []Check) ReportSummary {
	s := ReportSummary{Total: len(checks)}
	for _, c := range checks {
		switch c.Status {
		case CheckStatusPassed:
			s.Passed++
		case CheckStatusFailed:
			s.Failed++
		case CheckStatusSkipped:
			s.Skipped++
		case CheckStatusBlocked:
			s.Blocked++
		case CheckStatusPending:
			s.Pending++
		}
		switch c.Result {
		case ResultEquivalent:
			s.Equivalent++
		case ResultImproved:
			s.Improved++
		case ResultIntentionallyChanged:
			s.Changed++
		case ResultMissing:
			s.Missing++
		case ResultRegressed:
			s.Regressed++
		case ResultNotApplicable:
			s.NotApplicable++
		}
	}
	return s
}

var (
	ErrNoChecks = errors.New("equivalence: no checks registered")
)

func (r *Runner) SaveReport(report *Report, path string) error {
	if report == nil {
		return errors.New("equivalence: report is nil")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func SortChecks(checks []Check) {
	sort.SliceStable(checks, func(i, j int) bool {
		if checks[i].Category != checks[j].Category {
			return string(checks[i].Category) < string(checks[j].Category)
		}
		return checks[i].CheckID < checks[j].CheckID
	})
}
