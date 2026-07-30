package kernel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
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
	container   *Container
	metricsRepo *FinalGateMetricsRepository
}

func NewFinalGateProbe(container *Container) *FinalGateProbe {
	p := &FinalGateProbe{container: container}
	if container != nil && container.Store != nil {
		p.metricsRepo = NewFinalGateMetricsRepository(container.Store.DB())
	}
	return p
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
		"legacy_tool_execute_calls",
		"legacy_mcp_execute_calls",
		"duplicate_contribution_registrations",
		"orphan_sandbox_sessions",
		"audit_incomplete_operations",
		"lifecycle_requires_recovery",
		"new_package_legacy_read_calls",
		"orphan_staging_directories",
		"orphan_installed_directories",
		"missing_artifact_rows",
		"artifact_hash_mismatch",
		"incomplete_package_operations",
		"requires_recovery_operations",
		"installation_without_files",
		"files_without_installation",
		"active_contribution_for_disabled_installation",
		"unresolved_package_operations",
		"orphan_artifacts",
		"orphan_installation_generations",
		"installation_read_model_mismatches",
		"unsigned_production_packages",
		"untrusted_installed_packages",
		"corrupted_artifacts",
		"failed_uninstall_restores",
		"ambiguous_recovery_operations",
	}
}

var legacyStoreOnce sync.Once

func (p *FinalGateProbe) ensureLegacyStore(ctx context.Context) {
	legacyStoreOnce.Do(func() {
		if p.container != nil && p.container.Store != nil {
			db := p.container.Store.DB()
			store := NewLegacyCounterStore(db)
			_ = store.LoadAll(ctx)
			globalLegacyCallCounter.SetStore(store)
			globalLegacyReadCounter.SetStore(store)
		}
	})
}

func (p *FinalGateProbe) Evaluate(ctx context.Context) *FinalGateReport {
	report := &FinalGateReport{
		Metrics: make(map[string]int64),
		Details: make([]FinalGateIssue, 0),
		Errors:  make([]string, 0),
	}

	p.ensureLegacyStore(ctx)

	if p.metricsRepo != nil {
		if _, err := p.metricsRepo.CountAllOpen(ctx); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("final_gate: query persisted open metrics failed: %v", err))
		}
	}

	p.probeMCPDuplicates(ctx, report)
	p.probeOrphanCandidates(ctx, report)
	p.probeOrphanRuntimeInstances(ctx, report)
	p.probeOrphanUISessions(ctx, report)
	p.probeDuplicateScheduleRuns(ctx, report)
	p.probeFailedCleanupResources(ctx, report)
	p.probeCandidateStates(ctx, report)
	p.probeOrphanSandboxSessions(ctx, report)
	p.probeLifecycleRecovery(ctx, report)
	p.probeAuditIncomplete(ctx, report)
	p.probePackageReleaseGate(ctx, report)

	legacyMetrics := GlobalLegacyCallCounter().FinalGateMetrics()
	report.Metrics["new_package_legacy_read_calls"] = legacyMetrics["legacy_package_read_calls"]
	for _, name := range []string{
		"legacy_tool_execute_calls",
		"legacy_mcp_execute_calls",
		"legacy_package_write_calls",
		"legacy_package_read_calls",
		"duplicate_contribution_registrations",
	} {
		if _, exists := report.Metrics[name]; !exists {
			report.Metrics[name] = legacyMetrics[name]
		}
	}

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

func (p *FinalGateProbe) probePackageReleaseGate(ctx context.Context, report *FinalGateReport) {
	names := []string{
		"orphan_staging_directories", "orphan_installed_directories", "missing_artifact_rows", "artifact_hash_mismatch",
		"incomplete_package_operations", "requires_recovery_operations", "installation_without_files", "files_without_installation",
		"active_contribution_for_disabled_installation", "unresolved_package_operations", "orphan_artifacts",
		"orphan_installation_generations", "installation_read_model_mismatches", "unsigned_production_packages",
		"untrusted_installed_packages", "corrupted_artifacts", "failed_uninstall_restores", "ambiguous_recovery_operations",
		"legacy_package_write_calls",
	}
	for _, name := range names {
		report.Metrics[name] = -1
	}
	if p.container == nil || p.container.Store == nil || p.container.PackageRepository == nil || p.container.PackageArtifactStore == nil || p.container.PackageGenerationStore == nil || p.container.InstallationRepository == nil {
		report.Errors = append(report.Errors, "package_release_gate: dependency not injected")
		return
	}
	db := p.container.Store.DB()
	queries := map[string]string{
		"incomplete_package_operations":                 `SELECT COUNT(*) FROM extension_package_operations WHERE status NOT IN ('completed', 'failed', 'cancelled', 'rolled_back')`,
		"legacy_package_write_calls":                    `SELECT COALESCE((SELECT count FROM kernel_legacy_call_counters WHERE metric_name = 'legacy_package_write_calls'), 0)`,
		"requires_recovery_operations":                  `SELECT COUNT(*) FROM extension_package_operations WHERE status = 'requires_recovery'`,
		"unresolved_package_operations":                 `SELECT COUNT(*) FROM extension_package_operations WHERE status NOT IN ('completed', 'failed', 'cancelled', 'rolled_back')`,
		"missing_artifact_rows":                         `SELECT COUNT(*) FROM extension_installations i LEFT JOIN extension_package_artifacts a ON a.artifact_id = json_extract(i.installation_json, '$.packageId') WHERE COALESCE(json_extract(i.installation_json, '$.packageId'), '') <> '' AND a.artifact_id IS NULL`,
		"active_contribution_for_disabled_installation": `SELECT COUNT(*) FROM extension_contributions c JOIN extension_installations i ON i.extension_id = c.extension_id WHERE c.registered = 1 AND i.enabled = 0`,
		"orphan_artifacts":                              `SELECT COUNT(*) FROM extension_package_artifacts a WHERE a.deleted_at = '' AND a.quarantined_at = '' AND a.reference_count = 0 AND a.retention_state NOT IN ('retained', 'deleted') AND NOT EXISTS (SELECT 1 FROM extension_package_artifact_references r WHERE r.artifact_id = a.artifact_id AND r.released_at = '')`,
		"installation_read_model_mismatches":            `SELECT COUNT(*) FROM extension_installations i LEFT JOIN extension_definitions d ON d.extension_id = i.extension_id AND d.version = i.version LEFT JOIN extension_package_artifacts a ON a.artifact_id = json_extract(i.installation_json, '$.packageId') WHERE i.installed = 1 AND (d.id IS NULL OR a.artifact_id IS NULL OR a.extension_id <> i.extension_id OR a.version <> i.version OR COALESCE(json_extract(i.installation_json, '$.installedVersion'), '') <> i.version)`,
		"unsigned_production_packages":                  `SELECT COUNT(*) FROM extension_installations i JOIN extension_package_artifacts a ON a.artifact_id = json_extract(i.installation_json, '$.packageId') WHERE i.installed = 1 AND a.signature_status <> 'valid' AND COALESCE(json_extract(i.installation_json, '$.metadata.devOnly'), 0) NOT IN (1, 'true')`,
		"untrusted_installed_packages":                  `SELECT COUNT(*) FROM extension_installations i JOIN extension_package_artifacts a ON a.artifact_id = json_extract(i.installation_json, '$.packageId') WHERE i.installed = 1 AND a.trust_decision NOT IN ('official', 'trusted', 'user_trusted', 'development')`,
		"failed_uninstall_restores":                     `SELECT COUNT(*) FROM extension_package_operations WHERE operation_type = 'uninstall' AND status = 'requires_recovery' AND (current_step = 'restore_quarantine' OR error_detail LIKE '%restore quarantined installation%')`,
		"ambiguous_recovery_operations":                 `SELECT COUNT(*) FROM extension_package_operations WHERE status = 'requires_recovery' AND (current_step = 'recovery_manual' OR error_detail LIKE '%ambiguous%' OR error_detail LIKE '%could not be proven%')`,
	}
	for name, query := range queries {
		var count int64
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			report.Metrics[name] = -1
			report.Errors = append(report.Errors, fmt.Sprintf("%s: query failed: %v", name, err))
		} else {
			report.Metrics[name] = count
		}
	}
	installations, err := p.container.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		report.Metrics["installation_without_files"] = -1
		report.Metrics["orphan_installation_generations"] = -1
		report.Errors = append(report.Errors, fmt.Sprintf("package_release_gate: installations query failed: %v", err))
		return
	}
	installedPaths := make(map[string]bool)
	var installationWithoutFiles int64
	var generationReadModelMismatches int64
	for _, installation := range installations {
		path, _ := installation.Metadata["installedPath"].(string)
		if path == "" {
			installationWithoutFiles++
			continue
		}
		absolute, absErr := filepath.Abs(path)
		if absErr != nil {
			installationWithoutFiles++
			continue
		}
		installedPaths[filepath.Clean(absolute)] = true
		if info, statErr := os.Stat(absolute); statErr != nil || !info.IsDir() {
			installationWithoutFiles++
		}
		generationID, _ := installation.Metadata["generationId"].(string)
		if generationID == "" {
			generationReadModelMismatches++
			continue
		}
		current, currentErr := p.container.PackageGenerationStore.ReadCurrent(string(installation.ExtensionID))
		treeHash, _ := installation.Metadata["installedTreeHash"].(string)
		artifactID, _ := installation.Metadata["artifactId"].(string)
		if currentErr != nil || current.GenerationID != generationID || current.TreeHash != treeHash || current.ArtifactID != artifactID || p.container.PackageGenerationStore.VerifyGeneration(ctx, current) != nil {
			generationReadModelMismatches++
		}
	}
	rollbackRows, rollbackErr := db.QueryContext(ctx, `SELECT installed_path FROM extension_package_rollback_points WHERE installed_path <> ''`)
	if rollbackErr != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("package_release_gate: rollback paths query failed: %v", rollbackErr))
		return
	}
	for rollbackRows.Next() {
		var path string
		if scanErr := rollbackRows.Scan(&path); scanErr != nil {
			rollbackRows.Close()
			report.Errors = append(report.Errors, fmt.Sprintf("package_release_gate: rollback path scan failed: %v", scanErr))
			return
		}
		if absolute, absErr := filepath.Abs(path); absErr == nil {
			installedPaths[filepath.Clean(absolute)] = true
		}
	}
	if rowsErr := rollbackRows.Err(); rowsErr != nil {
		rollbackRows.Close()
		report.Errors = append(report.Errors, fmt.Sprintf("package_release_gate: rollback path iterate failed: %v", rowsErr))
		return
	}
	rollbackRows.Close()
	report.Metrics["installation_without_files"] = installationWithoutFiles
	if report.Metrics["installation_read_model_mismatches"] >= 0 && generationReadModelMismatches > report.Metrics["installation_read_model_mismatches"] {
		report.Metrics["installation_read_model_mismatches"] = generationReadModelMismatches
	}
	installationsRoot := filepath.Join(p.container.PackageArtifactStore.root, "installations")
	var orphanStaging int64
	stagingWalkErr := filepath.WalkDir(installationsRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() || filepath.Base(filepath.Dir(path)) != "staging" {
			return nil
		}
		orphanStaging++
		return filepath.SkipDir
	})
	if stagingWalkErr != nil && !os.IsNotExist(stagingWalkErr) {
		report.Metrics["orphan_staging_directories"] = -1
		report.Errors = append(report.Errors, fmt.Sprintf("orphan_staging_directories: scan failed: %v", stagingWalkErr))
	} else {
		report.Metrics["orphan_staging_directories"] = orphanStaging
	}
	var filesWithoutInstallation int64
	walkErr := filepath.WalkDir(installationsRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() || filepath.Base(filepath.Dir(path)) != "generations" {
			return nil
		}
		parent, absErr := filepath.Abs(path)
		if absErr != nil || !installedPaths[filepath.Clean(parent)] {
			filesWithoutInstallation++
		}
		return filepath.SkipDir
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		report.Metrics["files_without_installation"] = -1
		report.Metrics["orphan_installed_directories"] = -1
		report.Errors = append(report.Errors, fmt.Sprintf("files_without_installation: scan failed: %v", walkErr))
	} else {
		report.Metrics["files_without_installation"] = filesWithoutInstallation
		legacyInstalledRoot := filepath.Join(p.container.PackageArtifactStore.root, "installed")
		var legacyInstalled int64
		_ = filepath.WalkDir(legacyInstalledRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr == nil && !entry.IsDir() && entry.Name() == amitiax.ManifestFile {
				legacyInstalled++
			}
			return walkErr
		})
		report.Metrics["orphan_installed_directories"] = legacyInstalled
		report.Metrics["orphan_installation_generations"] = filesWithoutInstallation
	}
	rows, queryErr := db.QueryContext(ctx, `SELECT artifact_id FROM extension_package_artifacts WHERE deleted_at = ''`)
	if queryErr != nil {
		report.Metrics["artifact_hash_mismatch"] = -1
		report.Errors = append(report.Errors, fmt.Sprintf("artifact_hash_mismatch: query failed: %v", queryErr))
		return
	}
	var artifactIDs []string
	for rows.Next() {
		var artifactID string
		if scanErr := rows.Scan(&artifactID); scanErr != nil {
			rows.Close()
			report.Errors = append(report.Errors, fmt.Sprintf("artifact_hash_mismatch: scan failed: %v", scanErr))
			return
		}
		artifactIDs = append(artifactIDs, artifactID)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		report.Errors = append(report.Errors, fmt.Sprintf("artifact_hash_mismatch: iterate failed: %v", rowsErr))
		return
	}
	rows.Close()
	var mismatch int64
	corrupted := make(map[string]bool)
	for _, artifactID := range artifactIDs {
		artifact, getErr := p.container.PackageRepository.GetArtifact(ctx, artifactID)
		if getErr != nil {
			mismatch++
			corrupted[artifactID] = true
			continue
		}
		if artifact.VerificationStatus == "corrupted" {
			corrupted[artifactID] = true
		}
		if p.container.PackageArtifactStore.VerifyArchive(artifact) != nil {
			mismatch++
			corrupted[artifactID] = true
		}
	}
	report.Metrics["artifact_hash_mismatch"] = mismatch
	report.Metrics["corrupted_artifacts"] = int64(len(corrupted))
}

func (p *FinalGateProbe) ProbePackageReleaseGate(ctx context.Context, report *FinalGateReport) {
	p.probePackageReleaseGate(ctx, report)
}

func (p *FinalGateProbe) probeMCPDuplicates(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.MCPDuplicateProvider == nil {
		report.Errors = append(report.Errors, "duplicate_mcp_tool_registrations: dependency not injected (MCPDuplicateProvider is nil)")
		report.Metrics["duplicate_mcp_tool_registrations"] = -1
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
		report.Errors = append(report.Errors, "orphan_candidate_contributions: dependency not injected (CandidateRepository is nil)")
		report.Metrics["orphan_candidate_contributions"] = -1
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
		report.Errors = append(report.Errors, "orphan_runtime_instances: dependency not injected (Store or InstallationRepository is nil)")
		report.Metrics["orphan_runtime_instances"] = -1
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
		report.Errors = append(report.Errors, "orphan_ui_sessions: dependency not injected (PageSessionRepository or InstallationRepository is nil)")
		report.Metrics["orphan_ui_sessions"] = -1
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
		report.Errors = append(report.Errors, "duplicate_schedule_runs: dependency not injected (Store is nil)")
		report.Metrics["duplicate_schedule_runs"] = -1
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

	nowStr := time.Now().UTC().Format(time.RFC3339Nano)
	expiredRows, err := db.QueryContext(ctx, `
		SELECT lease_id, trigger_id, schedule_id, lease_owner, expires_at
		FROM extension_schedule_leases
		WHERE released = 0 AND expires_at < ?
	`, nowStr)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("duplicate_schedule_runs: expired lease query failed: %v", err))
		return
	}
	defer expiredRows.Close()
	for expiredRows.Next() {
		var leaseID, triggerID, scheduleID, leaseOwner, expiresAt string
		if err := expiredRows.Scan(&leaseID, &triggerID, &scheduleID, &leaseOwner, &expiresAt); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate_schedule_runs: expired lease scan failed: %v", err))
			return
		}
		report.Errors = append(report.Errors, fmt.Sprintf("duplicate_schedule_runs: expired unreleased lease leaseID=%s triggerID=%s scheduleID=%s owner=%s expiresAt=%s", leaseID, triggerID, scheduleID, leaseOwner, expiresAt))
	}
	if err := expiredRows.Err(); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("duplicate_schedule_runs: expired lease iterate failed: %v", err))
	}
}

func (p *FinalGateProbe) probeFailedCleanupResources(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.ResourceRepository == nil {
		report.Errors = append(report.Errors, "failed_cleanup_resources: dependency not injected (ResourceRepository is nil)")
		report.Metrics["failed_cleanup_resources"] = -1
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

func (p *FinalGateProbe) probeCandidateStates(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.CandidateRepository == nil {
		report.Errors = append(report.Errors, "candidate_states: dependency not injected (CandidateRepository is nil)")
		return
	}
	records, err := p.container.CandidateRepository.ListAll(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("candidate_states: query failed: %v", err))
		return
	}
	promoteTimeout := 10 * time.Minute
	now := time.Now().UTC()

	for _, r := range records {
		switch r.Status {
		case CandidateStatusPromoting:
			if !r.PromoteStartedAt.IsZero() && now.Sub(r.PromoteStartedAt) > promoteTimeout {
				report.Errors = append(report.Errors, fmt.Sprintf("candidate_states: candidate %s promoting timeout (started=%s, elapsed=%s)", r.CandidateID, r.PromoteStartedAt.Format(time.RFC3339Nano), now.Sub(r.PromoteStartedAt)))
			}
			if !r.RollbackStartedAt.IsZero() && r.RollbackFinishedAt.IsZero() {
				report.Errors = append(report.Errors, fmt.Sprintf("candidate_states: candidate %s rollback pending (started=%s)", r.CandidateID, r.RollbackStartedAt.Format(time.RFC3339Nano)))
			}
			if p.container.CandidateMgr != nil {
				if _, found := p.container.CandidateMgr.GetCandidate(r.CandidateID); !found {
					report.Errors = append(report.Errors, fmt.Sprintf("candidate_states: candidate %s exists in repository but not in memory (namespace/repository inconsistency)", r.CandidateID))
				}
			}
			if p.container.Store != nil {
				db := p.container.Store.DB()
				var snapCount int
				err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_candidate_stable_snapshots WHERE candidate_id = ?`, r.CandidateID).Scan(&snapCount)
				if err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("candidate_states: stable snapshot check failed for %s: %v", r.CandidateID, err))
				} else if snapCount == 0 {
					report.Errors = append(report.Errors, fmt.Sprintf("candidate_states: candidate %s in promoting state but stable snapshot missing", r.CandidateID))
				}
			}
		case CandidateStatusRequiresRecovery:
			report.Errors = append(report.Errors, fmt.Sprintf("candidate_states: candidate %s in requires_recovery state (extension=%s)", r.CandidateID, r.ExtensionID))
		case CandidateStatusFailed:
			if !r.RollbackStartedAt.IsZero() && r.RollbackFinishedAt.IsZero() {
				report.Errors = append(report.Errors, fmt.Sprintf("candidate_states: candidate %s rollback pending in failed state (started=%s)", r.CandidateID, r.RollbackStartedAt.Format(time.RFC3339Nano)))
			}
		}
	}
}

func (p *FinalGateProbe) probeOrphanSandboxSessions(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.SandboxHost == nil {
		report.Errors = append(report.Errors, "orphan_sandbox_sessions: dependency not injected (SandboxHost is nil)")
		report.Metrics["orphan_sandbox_sessions"] = -1
		return
	}
	if p.container.InstallationRepository == nil {
		report.Errors = append(report.Errors, "orphan_sandbox_sessions: dependency not injected (InstallationRepository is nil)")
		report.Metrics["orphan_sandbox_sessions"] = -1
		return
	}
	sessions := p.container.SandboxHost.ListSessions()
	insts, err := p.container.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("orphan_sandbox_sessions: installations query failed: %v", err))
		report.Metrics["orphan_sandbox_sessions"] = -1
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
				Metric: "orphan_sandbox_sessions",
				Count:  1,
				Detail: fmt.Sprintf("sessionID=%s extensionID=%s moduleID=%s state=%s sandbox=%s (extension uninstalled)", s.SessionID, s.ExtensionID, s.ModuleID, s.State, s.Sandbox),
			})
		}
	}
	report.Metrics["orphan_sandbox_sessions"] = orphanCount
}

func (p *FinalGateProbe) probeLifecycleRecovery(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.InstallationRepository == nil {
		report.Errors = append(report.Errors, "lifecycle_requires_recovery: dependency not injected (InstallationRepository is nil)")
		report.Metrics["lifecycle_requires_recovery"] = -1
		return
	}
	insts, err := p.container.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("lifecycle_requires_recovery: query failed: %v", err))
		report.Metrics["lifecycle_requires_recovery"] = -1
		return
	}
	var recoveryCount int64
	for _, inst := range insts {
		if inst.EnablementState == domain.EnablementRequiresRecovery {
			recoveryCount++
			report.Details = append(report.Details, FinalGateIssue{
				Metric: "lifecycle_requires_recovery",
				Count:  1,
				Detail: fmt.Sprintf("extensionID=%s state=%s generation=%d requires recovery", inst.ExtensionID, inst.EnablementState, inst.Generation),
			})
		}
	}
	report.Metrics["lifecycle_requires_recovery"] = recoveryCount
}

func (p *FinalGateProbe) probeAuditIncomplete(ctx context.Context, report *FinalGateReport) {
	if p.container == nil || p.container.Store == nil {
		report.Errors = append(report.Errors, "audit_incomplete_operations: dependency not injected (Store is nil)")
		report.Metrics["audit_incomplete_operations"] = -1
		return
	}
	db := p.container.Store.DB()
	var count int64
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM host_api_audit_logs WHERE finished_at IS NULL
	`).Scan(&count)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("audit_incomplete_operations: query failed: %v", err))
		report.Metrics["audit_incomplete_operations"] = -1
		return
	}
	if count > 0 {
		rows, qErr := db.QueryContext(ctx, `
			SELECT call_id, extension_id, method, started_at
			FROM host_api_audit_logs WHERE finished_at IS NULL
			LIMIT 100
		`)
		if qErr != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("audit_incomplete_operations: detail query failed: %v", qErr))
			report.Metrics["audit_incomplete_operations"] = count
			return
		}
		defer rows.Close()
		for rows.Next() {
			var callID, extID, method, startedAt string
			if err := rows.Scan(&callID, &extID, &method, &startedAt); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("audit_incomplete_operations: scan failed: %v", err))
				return
			}
			report.Details = append(report.Details, FinalGateIssue{
				Metric: "audit_incomplete_operations",
				Count:  1,
				Detail: fmt.Sprintf("callID=%s extensionID=%s method=%s startedAt=%s (unfinished)", callID, extID, method, startedAt),
			})
		}
	}
	report.Metrics["audit_incomplete_operations"] = count
}

func (c *Container) FinalGateProbe() *FinalGateProbe {
	return NewFinalGateProbe(c)
}

func (c *Container) EvaluateFinalGate(ctx context.Context) *FinalGateReport {
	return NewFinalGateProbe(c).Evaluate(ctx)
}
