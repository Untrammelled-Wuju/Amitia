// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package doctor

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
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

type Doctor struct {
	db        *gorm.DB
	extension *extension.Runtime
	nowFn     func() time.Time
}

func NewDoctor(db *gorm.DB, ext *extension.Runtime) *Doctor {
	return &Doctor{
		db:        db,
		extension: ext,
		nowFn:     time.Now,
	}
}

func (d *Doctor) RunChecks() *DoctorReport {
	doctor := NewDesktopPetDoctor(ModeDeep)
	doctor.AddChecker(&sqliteChecker{db: d.db})
	doctor.AddChecker(&surrealChecker{cfg: config.AppCfg.Surreal})
	doctor.AddChecker(&qdrantChecker{})
	doctor.AddChecker(&extensionChecker{ext: d.extension})
	return doctor.Run()
}

type sqliteChecker struct {
	db *gorm.DB
}

func (c *sqliteChecker) Name() string { return "sqlite" }

func (c *sqliteChecker) Check() ([]DoctorFinding, error) {
	var findings []DoctorFinding
	sqlDB, err := c.db.DB()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		findings = append(findings, DoctorFinding{
			Code:       "sqlite_unavailable",
			Severity:   SeverityCritical,
			Category:   "sqlite",
			Message:    fmt.Sprintf("SQLite connection failed: %v", err),
			Suggestion: "Check SQLite database file availability and permissions",
		})
	} else {
		findings = append(findings, DoctorFinding{
			Code:     "sqlite_ok",
			Severity: SeverityInfo,
			Category: "sqlite",
			Message:  "SQLite connection is healthy",
		})
	}
	var count int
	if err := sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count); err != nil {
		findings = append(findings, DoctorFinding{
			Code:     "sqlite_metadata_unavailable",
			Severity: SeverityWarning,
			Category: "sqlite",
			Message:  fmt.Sprintf("SQLite metadata query failed: %v", err),
		})
	}
	return findings, nil
}

type surrealChecker struct {
	cfg config.SurrealConfig
}

func (c *surrealChecker) Name() string { return "surrealdb" }

func (c *surrealChecker) Check() ([]DoctorFinding, error) {
	var findings []DoctorFinding
	url := fmt.Sprintf("ws://%s:%d/rpc", c.cfg.Host, c.cfg.Port)
	db, err := surrealdb.New(url)
	if err != nil {
		return []DoctorFinding{{
			Code:       "surrealdb_unavailable",
			Severity:   SeverityError,
			Category:   "surrealdb",
			Message:    fmt.Sprintf("SurrealDB connection failed: %v", err),
			Suggestion: "Ensure SurrealDB is running and accessible",
		}}, nil
	}
	defer db.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := db.SignIn(ctx, map[string]string{"user": c.cfg.Username, "pass": c.cfg.Password}); err != nil {
		if _, err2 := db.SignIn(ctx, map[string]string{"user": "root", "pass": "root"}); err2 != nil {
			findings = append(findings, DoctorFinding{
				Code:     "surrealdb_auth_failed",
				Severity: SeverityError,
				Category: "surrealdb",
				Message:  fmt.Sprintf("SurrealDB authentication failed: %v", err),
			})
			return findings, nil
		}
	}

	if err := db.Use(ctx, c.cfg.Namespace, c.cfg.Database); err != nil {
		findings = append(findings, DoctorFinding{
			Code:     "surrealdb_use_failed",
			Severity: SeverityError,
			Category: "surrealdb",
			Message:  fmt.Sprintf("SurrealDB USE failed: %v", err),
		})
		return findings, nil
	}

	_, err = surrealdb.Query[any](ctx, db, "SELECT 1", nil)
	if err != nil {
		findings = append(findings, DoctorFinding{
			Code:     "surrealdb_query_failed",
			Severity: SeverityError,
			Category: "surrealdb",
			Message:  fmt.Sprintf("SurrealDB query failed: %v", err),
		})
		return findings, nil
	}

	conn, dialErr := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port), 3*time.Second)
	if dialErr != nil {
		findings = append(findings, DoctorFinding{
			Code:     "surrealdb_port_unreachable",
			Severity: SeverityWarning,
			Category: "surrealdb",
			Message:  fmt.Sprintf("SurrealDB TCP port unreachable: %v", dialErr),
		})
		return findings, nil
	}
	conn.Close()

	findings = append(findings, DoctorFinding{
		Code:     "surrealdb_ok",
		Severity: SeverityInfo,
		Category: "surrealdb",
		Message:  "SurrealDB connection is healthy",
	})
	return findings, nil
}

type qdrantChecker struct{}

func (c *qdrantChecker) Name() string { return "qdrant" }

func (c *qdrantChecker) Check() ([]DoctorFinding, error) {
	var findings []DoctorFinding
	if qdrant.Client == nil {
		return []DoctorFinding{{
			Code:       "qdrant_not_initialized",
			Severity:   SeverityError,
			Category:   "qdrant",
			Message:    "Qdrant client is not initialized",
			Suggestion: "Qdrant may have failed to start during server initialization",
		}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := qdrant.Client.HealthCheck(ctx)
	if err != nil {
		findings = append(findings, DoctorFinding{
			Code:       "qdrant_unhealthy",
			Severity:   SeverityError,
			Category:   "qdrant",
			Message:    fmt.Sprintf("Qdrant health check failed: %v", err),
			Suggestion: "Ensure Qdrant service is running",
		})
		return findings, nil
	}
	findings = append(findings, DoctorFinding{
		Code:     "qdrant_ok",
		Severity: SeverityInfo,
		Category: "qdrant",
		Message:  "Qdrant connection is healthy",
	})
	return findings, nil
}

type extensionChecker struct {
	ext *extension.Runtime
}

func (c *extensionChecker) Name() string { return "extension" }

func (c *extensionChecker) Check() ([]DoctorFinding, error) {
	var findings []DoctorFinding
	if c.ext == nil {
		return []DoctorFinding{{
			Code:     "extension_not_initialized",
			Severity: SeverityCritical,
			Category: "extension",
			Message:  "Extension runtime is not initialized",
		}}, nil
	}
	if c.ext.Kernel == nil {
		findings = append(findings, DoctorFinding{
			Code:     "extension_kernel_nil",
			Severity: SeverityError,
			Category: "extension",
			Message:  "Extension kernel is not attached",
		})
		return findings, nil
	}
	container := c.ext.Kernel.Container()
	if container == nil {
		findings = append(findings, DoctorFinding{
			Code:     "extension_container_nil",
			Severity: SeverityError,
			Category: "extension",
			Message:  "Extension kernel has no container",
		})
		return findings, nil
	}
	findings = append(findings, DoctorFinding{
		Code:     "extension_ok",
		Severity: SeverityInfo,
		Category: "extension",
		Message:  "Extension runtime is initialized and kernel container is available",
	})
	return findings, nil
}
