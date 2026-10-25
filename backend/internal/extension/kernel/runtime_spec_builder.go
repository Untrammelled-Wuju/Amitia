package kernel

import (
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func buildModuleInstanceSpec(extID domain.ExtensionID, modID domain.ModuleID, runtimeDef *domain.RuntimeDefinition, generation int64) runtime_supervisor.InstanceSpec {
	spec := runtime_supervisor.InstanceSpec{
		DefinitionID:  runtime_supervisor.BuildRuntimeDefinitionID(string(extID), string(modID), runtimeDef.Type),
		ExtensionID:   extID,
		ModuleID:      modID,
		RuntimeType:   runtimeDef.Type,
		Generation:    generation,
		Strategy:      runtime_supervisor.StrategySingletonPerModule,
		EntryPoint:    runtimeDef.EntryPoint,
		WorkerCount:   runtimeDef.WorkerCount,
		Env:           runtimeDef.Env,
		Permissions:   runtimeDef.Permissions,
		Capabilities:  runtimeDef.Capabilities,
		HostAccess:    runtimeDef.HostAccess,
		Restart:       runtime_supervisor.RestartOnCrash,
		MaxRestarts:   3,
		RestartWindow: 60 * time.Second,
		Limits: runtime_supervisor.ResourceLimits{
			MaxMemoryBytes:     runtimeDef.Memory,
			MaxExecutionTime:   0,
			MaxConcurrentCalls: 10,
			MaxQueueDepth:      64,
		},
	}
	if spec.WorkerCount <= 0 {
		spec.WorkerCount = 1
	}
	if strings.EqualFold(runtimeDef.HostAccess.Process.Mode, "direct") {
		spec.Limits.MaxProcesses = runtimeDef.HostAccess.Process.MaxProcesses + 1
	}
	return spec
}
