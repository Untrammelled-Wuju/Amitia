package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type FinalGateIssue struct {
	Metric string `json:"metric"`
	Count  int64  `json:"count"`
	Detail string `json:"detail"`
}

type FinalGateReport struct {
	Passed  bool             `json:"passed"`
	Metrics map[string]int64 `json:"metrics"`
	Details []FinalGateIssue `json:"details"`
	Errors  []string         `json:"errors"`
}

type FinalGateProbe struct {
	container *Container
}

func NewFinalGateProbe(container *Container) *FinalGateProbe {
	return &FinalGateProbe{container: container}
}

func FinalGateMetricNames() []string {
	return []string{
		"duplicate_mcp_tool_registrations",
		"orphan_candidate_contributions",
		"orphan_runtime_instances",
		"orphan_ui_sessions",
		"duplicate_schedule_runs",
		"failed_cleanup_resources",
		"legacy_package_read_calls",
		"legacy_package_write_calls",
	}
}

func (p *FinalGateProbe) Evaluate(ctx context.Context) *FinalGateReport {
	report := &FinalGateReport{
		Metrics: make(map[string]int64),
		Details: make([]FinalGateIssue, 0),
		Errors:  make([]string, 0),
	}

	p.probeMCPDuplicates(ctx, report)
	p.probeOrphanCandidates(ctx, report)
	p.probeOrphanRuntimeInstances(ctx, report)
	p.probeOrphanUISessions(ctx, report)
	p.probeDuplicateScheduleRuns(ctx, report)
	p.probeFailedCleanupResources(ctx, report)

	report.Metrics["legacy_package_read_calls"] = GlobalLegacyReadCounter().PackageReadCallsFallbacks()
	report.Metrics["legacy_package_write_calls"] = GlobalLegacyCallCounter().PackageWriteCalls()

	for _, name := range FinalGateMetricNames() {
		if _, ok := report.Metrics[name]; !ok {
			report.Metrics[name] = 0
		}
	}

	report.Passed = len(report.Errors) == 0
	if !report.Passed {
		return report
	}
	for _, v := range report.Metrics {
		if v != 0 {
			report.Passed = false
			break
		}
	}

	return report
}

func (p *FinalGateProbe) probeMCPDuplicates(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.MCPDuplicateProvider == nil {
		report.Metrics["duplicate_mcp_tool_registrations"] = 0
		return
	}
	count, err := p.container.MCPDuplicateProvider.CountUnresolved(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("duplicate_mcp_tool_registrations: query failed: %v", err))
		report.Metrics["duplicate_mcp_tool_registrations"] = -1
		return
	}
	report.Metrics["duplicate_mcp_tool_registrations"] = count
	if count > 0 {
		details, listErr := p.container.MCPDuplicateProvider.ListUnresolved(ctx)
		if listErr != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate_mcp_tool_registrations: list failed: %v", listErr))
			return
		}
		for _, d := range details {
			report.Details = append(report.Details, FinalGateIssue{
				Metric: "duplicate_mcp_tool_registrations",
				Count:  1,
				Detail: fmt.Sprintf("toolID=%s serverID=%s owner=%s generation=%d detectedAt=%s", d.ToolID, d.ServerID, d.Owner, d.Generation, d.DetectedAt),
			})
		}
	}
}

func (p *FinalGateProbe) probeOrphanCandidates(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.CandidateRepository == nil {
		report.Metrics["orphan_candidate_contributions"] = 0
		return
	}
	records, err := p.container.CandidateRepository.ListAll(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("orphan_candidate_contributions: query failed: %v", err))
		report.Metrics["orphan_candidate_contributions"] = -1
		return
	}
	var orphanCount int64
	for _, r := range records {
		if r.Status != CandidateStatusPromoted {
			orphanCount++
			report.Details = append(report.Details, FinalGateIssue{
				Metric: "orphan_candidate_contributions",
				Count:  1,
				Detail: fmt.Sprintf("candidateID=%s extensionID=%s status=%s generation=%d", r.CandidateID, r.ExtensionID, r.Status, r.CandidateGeneration),
			})
		}
	}
	report.Metrics["orphan_candidate_contributions"] = orphanCount
}

func (p *FinalGateProbe) probeOrphanRuntimeInstances(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.Store == nil || p.container.InstallationRepository == nil {
		report.Metrics["orphan_runtime_instances"] = 0
		return
	}
	insts, err := p.container.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("orphan_runtime_instances: installations query failed: %v", err))
		report.Metrics["orphan_runtime_instances"] = -1
		return
	}
	installedExts := make(map[string]bool)
	for _, inst := range insts {
		if inst.InstallationState == domain.InstallationStateInstalled {
			installedExts[string(inst.ExtensionID)] = true
		}
	}

	db := p.container.Store.DB()
	rows, err := db.QueryContext(ctx, `
		SELECT instance_id, extension_id, module_id, runtime_type, desired_state, actual_state
		FROM extension_runtime_instances
	`)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("orphan_runtime_instances: query failed: %v", err))
		report.Metrics["orphan_runtime_instances"] = -1
		return
	}
	defer rows.Close()

	var orphanCount int64
	for rows.Next() {
		var instanceID, extID, modID, rtType, desiredState, actualState string
		if err := rows.Scan(&instanceID, &extID, &modID, &rtType, &desiredState, &actualState); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("orphan_runtime_instances: scan failed: %v", err))
			return
		}
		if !installedExts[extID] {
			orphanCount++
			report.Details = append(report.Details, FinalGateIssue{
				Metric: "orphan_runtime_instances",
				Count:  1,
				Detail: fmt.Sprintf("instanceID=%s extensionID=%s runtime=%s desired=%s actual=%s (extension uninstalled)", instanceID, extID, rtType, desiredState, actualState),
			})
		}
	}
	if err := rows.Err(); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("orphan_runtime_instances: iterate failed: %v", err))
		return
	}
	report.Metrics["orphan_runtime_instances"] = orphanCount
}

func (p *FinalGateProbe) probeOrphanUISessions(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.PageSessionRepository == nil || p.container.InstallationRepository == nil {
		report.Metrics["orphan_ui_sessions"] = 0
		return
	}
	sessions, err := p.container.PageSessionRepository.ListActiveSessions(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("orphan_ui_sessions: query failed: %v", err))
		report.Metrics["orphan_ui_sessions"] = -1
		return
	}
	insts, err := p.container.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("orphan_ui_sessions: installations query failed: %v", err))
		return
	}
	installedExts := make(map[string]bool)
	for _, inst := range insts {
		if inst.InstallationState == domain.InstallationStateInstalled {
			installedExts[string(inst.ExtensionID)] = true
		}
	}
	var orphanCount int64
	for _, s := range sessions {
		if !installedExts[s.ExtensionID] {
			orphanCount++
			report.Details = append(report.Details, FinalGateIssue{
				Metric: "orphan_ui_sessions",
				Count:  1,
				Detail: fmt.Sprintf("sessionID=%s extensionID=%s pageID=%s state=%s (extension uninstalled)", s.SessionID, s.ExtensionID, s.PageID, s.State),
			})
		}
	}
	report.Metrics["orphan_ui_sessions"] = orphanCount
}

func (p *FinalGateProbe) probeDuplicateScheduleRuns(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.Store == nil {
		report.Metrics["duplicate_schedule_runs"] = 0
		return
	}
	db := p.container.Store.DB()
	rows, err := db.QueryContext(ctx, `
		SELECT schedule_id, COUNT(*) as cnt
		FROM extension_schedule_runs
		WHERE status IN ('running', 'triggering', 'retry_wait')
		GROUP BY schedule_id
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("duplicate_schedule_runs: query failed: %v", err))
		report.Metrics["duplicate_schedule_runs"] = -1
		return
	}
	defer rows.Close()

	var dupCount int64
	for rows.Next() {
		var scheduleID string
		var cnt int
		if err := rows.Scan(&scheduleID, &cnt); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate_schedule_runs: scan failed: %v", err))
			return
		}
		excess := int64(cnt - 1)
		dupCount += excess
		report.Details = append(report.Details, FinalGateIssue{
			Metric: "duplicate_schedule_runs",
			Count:  excess,
			Detail: fmt.Sprintf("scheduleID=%s has %d concurrent active runs", scheduleID, cnt),
		})
	}
	if err := rows.Err(); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("duplicate_schedule_runs: iterate failed: %v", err))
		return
	}
	report.Metrics["duplicate_schedule_runs"] = dupCount
}

func (p *FinalGateProbe) probeFailedCleanupResources(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.ResourceRepository == nil {
		report.Metrics["failed_cleanup_resources"] = 0
		return
	}
	expired, err := p.container.ResourceRepository.ListExpiredResources(ctx, time.Now().UTC())
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("failed_cleanup_resources: query failed: %v", err))
		report.Metrics["failed_cleanup_resources"] = -1
		return
	}
	failCount := int64(len(expired))
	for _, r := range expired {
		report.Details = append(report.Details, FinalGateIssue{
			Metric: "failed_cleanup_resources",
			Count:  1,
			Detail: fmt.Sprintf("resourceID=%s type=%s ownerID=%s reference=%s expired", r.ResourceID, r.ResourceType, r.OwnerID, r.Reference),
		})
	}
	report.Metrics["failed_cleanup_resources"] = failCount
}

func (c *Container) FinalGateProbe() *FinalGateProbe {
	return NewFinalGateProbe(c)
}

func (c *Container) EvaluateFinalGate(ctx context.Context) *FinalGateReport {
	return NewFinalGateProbe(c).Evaluate(ctx)
}
