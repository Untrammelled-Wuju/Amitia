package kernel

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type ExtensionKernelReadiness struct {
	TaskRuntimeReady       bool
	EventServiceReady      bool
	ScheduleServiceReady   bool
	RuntimeSupervisorReady bool
	EnabledRuntimesReady   bool
	LegacyCounterZero      bool
	NoOrphanRuntimes       bool
	MCPDuplicateDetails    []MCPDuplicateDetail
}

func (r ExtensionKernelReadiness) Ready() bool {
	return r.TaskRuntimeReady && r.EventServiceReady && r.ScheduleServiceReady && r.RuntimeSupervisorReady && r.EnabledRuntimesReady && r.LegacyCounterZero && r.NoOrphanRuntimes
}

func (r ExtensionKernelReadiness) FailedComponents() []string {
	var failed []string
	if !r.TaskRuntimeReady {
		failed = append(failed, "task_runtime_ready")
	}
	if !r.EventServiceReady {
		failed = append(failed, "event_service_ready")
	}
	if !r.ScheduleServiceReady {
		failed = append(failed, "schedule_service_ready")
	}
	if !r.RuntimeSupervisorReady {
		failed = append(failed, "runtime_supervisor_ready")
	}
	if !r.EnabledRuntimesReady {
		failed = append(failed, "enabled_runtimes_ready")
	}
	if !r.LegacyCounterZero {
		failed = append(failed, "legacy_counter_zero")
	}
	if !r.NoOrphanRuntimes {
		failed = append(failed, "orphan_runtimes_not_cleaned")
	}
	return failed
}

func (c *Container) CheckExtensionKernelReadiness(ctx context.Context) ExtensionKernelReadiness {
	r := ExtensionKernelReadiness{}
	if c == nil {
		return r
	}
	r.TaskRuntimeReady = c.TaskRuntimeService != nil
	if c.EventService != nil {
		r.EventServiceReady = true
	}
	if c.ScheduleService != nil {
		r.ScheduleServiceReady = c.ScheduleService.IsRunning()
	}
	r.RuntimeSupervisorReady = c.RuntimeSupervisor != nil
	r.EnabledRuntimesReady = c.checkEnabledRuntimesReady(ctx)

	gateReport := c.EvaluateFinalGate(ctx)
	r.LegacyCounterZero = gateReport.Passed

	r.NoOrphanRuntimes = true
	if c.DevModeReloader != nil {
		if orphanCount, err := c.DevModeReloader.PendingCleanupFailures(ctx); err == nil && orphanCount > 0 {
			r.NoOrphanRuntimes = false
			GlobalLegacyCallCounter().SetOrphanRuntimeInstances(orphanCount)
		}
	}

	if c.MCPDuplicateProvider != nil {
		if details, err := c.MCPDuplicateProvider.ListUnresolved(ctx); err == nil {
			r.MCPDuplicateDetails = details
		}
	}
	return r
}

func (c *Container) checkEnabledRuntimesReady(ctx context.Context) bool {
	if c.RuntimeSupervisor == nil || c.InstallationRepository == nil || c.ModuleRepository == nil {
		return true
	}
	insts, err := c.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		return false
	}
	for _, inst := range insts {
		if inst.InstallationState != domain.InstallationStateInstalled {
			continue
		}
		if inst.EnablementState != domain.EnablementEnabled {
			continue
		}
		modules, err := c.ModuleRepository.ListModules(ctx, inst.ExtensionID)
		if err != nil {
			return false
		}
		for _, mod := range modules {
			if mod.Runtime == nil || mod.Runtime.Type == "" || mod.Runtime.Type == domain.RuntimeTypeBuiltin {
				continue
			}
			defID := runtime_supervisor.BuildRuntimeDefinitionID(string(inst.ExtensionID), string(mod.ID), mod.Runtime.Type)
			snap := c.RuntimeSupervisor.Snapshot(ctx, defID)
			hasReady := false
			for _, instance := range snap.Instances {
				if instance.Actual == runtime_supervisor.ActualReady || instance.Actual == runtime_supervisor.ActualDegraded {
					hasReady = true
					break
				}
			}
			if !hasReady {
				return false
			}
		}
	}
	return true
}
