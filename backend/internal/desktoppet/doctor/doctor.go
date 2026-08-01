// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package doctor

import (
	"fmt"
	"time"
)

type DoctorMode string

const (
	ModeReadOnly   DoctorMode = "read_only"
	ModeStartup    DoctorMode = "startup"
	ModeDeep       DoctorMode = "deep"
	ModeRepairPlan DoctorMode = "repair_plan"
)

type FindingSeverity string

const (
	SeverityInfo     FindingSeverity = "info"
	SeverityWarning  FindingSeverity = "warning"
	SeverityError    FindingSeverity = "error"
	SeverityCritical FindingSeverity = "critical"
)

type DoctorFinding struct {
	Code        string          `json:"code"`
	Severity    FindingSeverity `json:"severity"`
	Category    string          `json:"category"`
	Message     string          `json:"message"`
	ResourceRef string          `json:"resourceRef,omitempty"`
	Suggestion  string          `json:"suggestion,omitempty"`
}

type DoctorReport struct {
	Status                  string          `json:"status"`
	Mode                    DoctorMode      `json:"mode"`
	StartedAt               string          `json:"startedAt"`
	CompletedAt             string          `json:"completedAt"`
	DurationMs              int64           `json:"durationMs"`
	TotalFindings           int             `json:"totalFindings"`
	BlockingFindings        int             `json:"blockingFindings"`
	Findings                []DoctorFinding `json:"findings"`
	Categories              map[string]int  `json:"categories"`
	AuthFailOpenCount       int             `json:"authFailOpenCount"`
	UnscopedHandlerCount    int             `json:"unscopedHandlerCount"`
	UnsafePathWriteCount    int             `json:"unsafePathWriteCount"`
	UnsafeDeleteCount       int             `json:"unsafeDeleteCount"`
	LegacyWriterCount       int             `json:"legacyWriterCount"`
	UnresolvedConflictCount int             `json:"unresolvedConflictCount"`
	BlockingJournalCount    int             `json:"blockingJournalCount"`
	RequiredWorkerDownCount int             `json:"requiredWorkerDownCount"`
	ContractMismatchCount   int             `json:"contractMismatchCount"`
	GofmtViolationCount     int             `json:"gofmtViolationCount"`
}

type DoctorChecker interface {
	Name() string
	Check() ([]DoctorFinding, error)
}

type DesktopPetDoctor struct {
	mode     DoctorMode
	checkers []DoctorChecker
	nowFn    func() time.Time
}

func NewDesktopPetDoctor(mode DoctorMode) *DesktopPetDoctor {
	return &DesktopPetDoctor{
		mode:  mode,
		nowFn: time.Now,
	}
}

func (d *DesktopPetDoctor) AddChecker(checker DoctorChecker) {
	d.checkers = append(d.checkers, checker)
}

func (d *DesktopPetDoctor) Run() *DoctorReport {
	startedAt := d.nowFn().UTC()
	startStr := startedAt.Format(time.RFC3339Nano)
	report := &DoctorReport{
		Status:     "running",
		Mode:       d.mode,
		StartedAt:  startStr,
		Categories: make(map[string]int),
		Findings:   make([]DoctorFinding, 0),
	}

	for _, checker := range d.checkers {
		findings, err := checker.Check()
		if err != nil {
			report.Findings = append(report.Findings, DoctorFinding{
				Code:     "checker_error",
				Severity: SeverityError,
				Category: checker.Name(),
				Message:  fmt.Sprintf("Checker failed: %v", err),
			})
			continue
		}
		for _, f := range findings {
			report.Findings = append(report.Findings, f)
			report.Categories[f.Category]++
			report.TotalFindings++
			if f.Severity == SeverityError || f.Severity == SeverityCritical {
				report.BlockingFindings++
			}
			switch f.Code {
			case "auth_fail_open":
				report.AuthFailOpenCount++
			case "unscoped_handler":
				report.UnscopedHandlerCount++
			case "unsafe_path_write":
				report.UnsafePathWriteCount++
			case "unsafe_delete":
				report.UnsafeDeleteCount++
			case "legacy_writer":
				report.LegacyWriterCount++
			case "unresolved_conflict":
				report.UnresolvedConflictCount++
			case "blocking_journal":
				report.BlockingJournalCount++
			case "required_worker_down":
				report.RequiredWorkerDownCount++
			case "contract_mismatch":
				report.ContractMismatchCount++
			case "gofmt_violation":
				report.GofmtViolationCount++
			}
		}
	}

	completedAt := d.nowFn().UTC()
	report.CompletedAt = completedAt.Format(time.RFC3339Nano)
	report.DurationMs = completedAt.Sub(startedAt).Milliseconds()
	if report.BlockingFindings > 0 {
		report.Status = "blocked"
	} else {
		report.Status = "ready"
	}
	return report
}

func (r *DoctorReport) IsHealthy() bool {
	return r.BlockingFindings == 0 &&
		r.AuthFailOpenCount == 0 &&
		r.UnscopedHandlerCount == 0 &&
		r.UnsafePathWriteCount == 0 &&
		r.UnsafeDeleteCount == 0 &&
		r.LegacyWriterCount == 0 &&
		r.UnresolvedConflictCount == 0 &&
		r.BlockingJournalCount == 0 &&
		r.RequiredWorkerDownCount == 0 &&
		r.ContractMismatchCount == 0 &&
		r.GofmtViolationCount == 0
}
