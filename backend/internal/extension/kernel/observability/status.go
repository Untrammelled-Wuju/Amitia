package observability

import "fmt"

type ExecutionStatus string

const (
	StatusCreated            ExecutionStatus = "created"
	StatusQueued             ExecutionStatus = "queued"
	StatusAwaitingApproval   ExecutionStatus = "awaiting_approval"
	StatusRunning            ExecutionStatus = "running"
	StatusRetrying           ExecutionStatus = "retrying"
	StatusSucceeded          ExecutionStatus = "succeeded"
	StatusFailed             ExecutionStatus = "failed"
	StatusCancelled          ExecutionStatus = "cancelled"
	StatusTimedOut           ExecutionStatus = "timed_out"
	StatusDenied             ExecutionStatus = "denied"
	StatusRateLimited        ExecutionStatus = "rate_limited"
	StatusCircuitOpen        ExecutionStatus = "circuit_open"
	StatusInvalid            ExecutionStatus = "invalid"
	StatusPartiallySucceeded ExecutionStatus = "partially_succeeded"
	StatusInterrupted        ExecutionStatus = "interrupted"
)

var terminalStatuses = map[ExecutionStatus]bool{
	StatusSucceeded:          true,
	StatusFailed:             true,
	StatusCancelled:          true,
	StatusTimedOut:           true,
	StatusDenied:             true,
	StatusRateLimited:        true,
	StatusCircuitOpen:        true,
	StatusInvalid:            true,
	StatusPartiallySucceeded: true,
	StatusInterrupted:        true,
}

func (s ExecutionStatus) IsTerminal() bool {
	return terminalStatuses[s]
}

func (s ExecutionStatus) IsValid() bool {
	switch s {
	case StatusCreated, StatusQueued, StatusAwaitingApproval, StatusRunning, StatusRetrying,
		StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut, StatusDenied,
		StatusRateLimited, StatusCircuitOpen, StatusInvalid, StatusPartiallySucceeded, StatusInterrupted:
		return true
	default:
		return false
	}
}

type AuditOutcome string

const (
	AuditOutcomeKnown    AuditOutcome = "outcome_known"
	AuditOutcomeUnknown  AuditOutcome = "outcome_unknown"
	AuditOutcomeDegraded AuditOutcome = "audit_degraded"
)

type DataSensitivity string

const (
	SensitivityPublic     DataSensitivity = "public"
	SensitivityInternal   DataSensitivity = "internal"
	SensitivitySensitive  DataSensitivity = "sensitive"
	SensitivityRestricted DataSensitivity = "restricted"
	SensitivitySecret     DataSensitivity = "secret"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type OperationType string

const (
	OpToolExecute         OperationType = "tool.execute"
	OpWorkflowExecute     OperationType = "workflow.execute"
	OpWorkflowSchedule    OperationType = "workflow.schedule"
	OpPluginHook          OperationType = "plugin.hook"
	OpPluginEvent         OperationType = "plugin.event"
	OpPluginSchedule      OperationType = "plugin.schedule"
	OpMCPConnect          OperationType = "mcp.connect"
	OpMCPDisconnect       OperationType = "mcp.disconnect"
	OpMCPDiscover         OperationType = "mcp.discover"
	OpMCPToolExecute      OperationType = "mcp.tool.execute"
	OpExtensionInstall    OperationType = "extension.install"
	OpExtensionEnable     OperationType = "extension.enable"
	OpExtensionDisable    OperationType = "extension.disable"
	OpExtensionUpdate     OperationType = "extension.update"
	OpExtensionRollback   OperationType = "extension.rollback"
	OpExtensionUninstall  OperationType = "extension.uninstall"
	OpExtensionRestore    OperationType = "extension.restore"
	OpAgentSkillImport    OperationType = "agent_skill.import"
	OpAgentSkillActivate  OperationType = "agent_skill.activate"
	OpAgentSkillRemove    OperationType = "agent_skill.remove"
	OpPermissionGrant     OperationType = "permission.grant"
	OpPermissionRevoke    OperationType = "permission.revoke"
	OpScopeBind           OperationType = "scope.bind"
	OpScopeUnbind         OperationType = "scope.unbind"
	OpMigrationExecute    OperationType = "migration.execute"
	OpRuntimeStart        OperationType = "runtime.start"
	OpRuntimeStop         OperationType = "runtime.stop"
	OpRuntimeCrash        OperationType = "runtime.crash"
)

type ActorType string

const (
	ActorUser       ActorType = "user"
	ActorSystem     ActorType = "system"
	ActorModel      ActorType = "model"
	ActorExtension  ActorType = "extension"
	ActorPlugin     ActorType = "plugin"
	ActorWorkflow   ActorType = "workflow"
	ActorScheduler  ActorType = "scheduler"
	ActorMigration  ActorType = "migration"
	ActorMCPServer  ActorType = "mcp_server"
)

type SubjectType string

const (
	SubjectTool           SubjectType = "tool"
	SubjectExtension      SubjectType = "extension"
	SubjectModule         SubjectType = "module"
	SubjectMCPServer      SubjectType = "mcp_server"
	SubjectWorkflow       SubjectType = "workflow"
	SubjectAgentSkill     SubjectType = "agent_skill"
	SubjectPermissionGrant SubjectType = "permission_grant"
	SubjectScopeBinding   SubjectType = "scope_binding"
	SubjectRuntime        SubjectType = "runtime"
	SubjectPackage        SubjectType = "package"
)

type ApprovalMode string

const (
	ApprovalAuto    ApprovalMode = "auto"
	ApprovalManual  ApprovalMode = "manual"
	ApprovalSession ApprovalMode = "session"
)

func IsTransitionValid(from, to ExecutionStatus) error {
	if !from.IsValid() {
		return fmt.Errorf("observability: invalid source status %q", from)
	}
	if !to.IsValid() {
		return fmt.Errorf("observability: invalid target status %q", to)
	}

	if from.IsTerminal() {
		return fmt.Errorf("observability: cannot transition from terminal status %q to %q", from, to)
	}

	switch from {
	case StatusCreated:
		if to == StatusQueued || to == StatusDenied || to == StatusInvalid {
			return nil
		}
	case StatusQueued:
		if to == StatusAwaitingApproval || to == StatusRunning || to == StatusCancelled || to == StatusInvalid {
			return nil
		}
	case StatusAwaitingApproval:
		if to == StatusRunning || to == StatusDenied || to == StatusCancelled || to == StatusTimedOut {
			return nil
		}
	case StatusRunning:
		if to == StatusSucceeded || to == StatusFailed || to == StatusPartiallySucceeded ||
			to == StatusCancelled || to == StatusTimedOut || to == StatusInterrupted ||
			to == StatusRetrying || to == StatusRateLimited || to == StatusCircuitOpen {
			return nil
		}
	case StatusRetrying:
		if to == StatusRunning || to == StatusFailed || to == StatusCancelled || to == StatusTimedOut {
			return nil
		}
	default:
		return fmt.Errorf("observability: no transition rules defined for source status %q", from)
	}

	return fmt.Errorf("observability: invalid transition from %q to %q", from, to)
}
