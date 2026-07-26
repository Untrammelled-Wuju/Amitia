package observability

import (
	"context"
	"fmt"
	"time"
)

type LegacyRunEntry struct {
	LegacyType     string
	LegacyID       string
	ExtensionID    string
	ModuleID       string
	ToolID         string
	Status         string
	ErrorCode      string
	CreatedAt      string
	UserID         string
	CharacterID    string
	ConversationID string
	Metadata       map[string]any
}

type MigrationReport struct {
	TotalLegacyRecords int      `json:"totalLegacyRecords"`
	MigratedRecords    int      `json:"migratedRecords"`
	UnmappedStatuses   int      `json:"unmappedStatuses"`
	OrphanRecords      int      `json:"orphanRecords"`
	DuplicateRecords   int      `json:"duplicateRecords"`
	UnmappedStatusList []string `json:"unmappedStatusList,omitempty"`
	OrphanList         []string `json:"orphanList,omitempty"`
	Errors             []string `json:"errors,omitempty"`
}

type MigrationMapping struct {
	OldTable      string
	OperationType OperationType
	SubjectType   SubjectType
	StatusMap     map[string]ExecutionStatus
}

func DefaultMigrationMappings() []MigrationMapping {
	return []MigrationMapping{
		{
			OldTable:      "plugin_runs",
			OperationType: OpPluginHook,
			SubjectType:   SubjectTool,
			StatusMap: map[string]ExecutionStatus{
				"pending":   StatusQueued,
				"running":   StatusRunning,
				"success":   StatusSucceeded,
				"failed":    StatusFailed,
				"cancelled": StatusCancelled,
				"timeout":   StatusTimedOut,
				"denied":    StatusDenied,
			},
		},
		{
			OldTable:      "mcp_operations",
			OperationType: OpMCPToolExecute,
			SubjectType:   SubjectMCPServer,
			StatusMap: map[string]ExecutionStatus{
				"queued":    StatusQueued,
				"running":   StatusRunning,
				"completed": StatusSucceeded,
				"failed":    StatusFailed,
				"timeout":   StatusTimedOut,
			},
		},
		{
			OldTable:      "workflow_runs",
			OperationType: OpWorkflowExecute,
			SubjectType:   SubjectWorkflow,
			StatusMap: map[string]ExecutionStatus{
				"pending":   StatusQueued,
				"running":   StatusRunning,
				"success":   StatusSucceeded,
				"failed":    StatusFailed,
				"cancelled": StatusCancelled,
			},
		},
		{
			OldTable:      "extension_runs",
			OperationType: OpToolExecute,
			SubjectType:   SubjectExtension,
			StatusMap: map[string]ExecutionStatus{
				"queued":    StatusQueued,
				"running":   StatusRunning,
				"success":   StatusSucceeded,
				"failed":    StatusFailed,
				"cancelled": StatusCancelled,
				"timeout":   StatusTimedOut,
				"denied":    StatusDenied,
			},
		},
		{
			OldTable:      "package_operations",
			OperationType: OpExtensionInstall,
			SubjectType:   SubjectPackage,
			StatusMap: map[string]ExecutionStatus{
				"installing":  StatusRunning,
				"installed":   StatusSucceeded,
				"failed":      StatusFailed,
				"rolled_back": StatusCancelled,
			},
		},
		{
			OldTable:      "agent_skill_activations",
			OperationType: OpAgentSkillActivate,
			SubjectType:   SubjectAgentSkill,
			StatusMap: map[string]ExecutionStatus{
				"active":  StatusSucceeded,
				"failed":  StatusFailed,
				"removed": StatusCancelled,
			},
		},
	}
}

type Migrator struct {
	store    StorageBackend
	mappings []MigrationMapping
	report   MigrationReport
}

func NewMigrator(store StorageBackend, mappings []MigrationMapping) *Migrator {
	return &Migrator{
		store:    store,
		mappings: mappings,
	}
}

func (m *Migrator) MigrateLegacyRecord(ctx context.Context, mappings []MigrationMapping, entry LegacyRunEntry) error {
	var mapping *MigrationMapping
	for i := range mappings {
		if mappings[i].OldTable == entry.LegacyType {
			mapping = &mappings[i]
			break
		}
	}
	if mapping == nil {
		m.report.OrphanRecords++
		m.report.OrphanList = append(m.report.OrphanList, fmt.Sprintf("%s:%s", entry.LegacyType, entry.LegacyID))
		return nil
	}

	status, ok := mapping.StatusMap[entry.Status]
	if !ok {
		m.report.UnmappedStatuses++
		m.report.UnmappedStatusList = append(m.report.UnmappedStatusList,
			fmt.Sprintf("%s:%s", entry.LegacyType, entry.Status))
		status = StatusFailed
	}

	now := time.Now()
	_ = now

	traceID := NewTraceID()
	operationID := NewOperationID()
	invocationID := NewInvocationID()

	trace := Trace{
		TraceID:   traceID,
		RootOpID:  operationID,
		CreatedAt: time.Now(),
		Metadata: map[string]any{
			"legacy_type": entry.LegacyType,
			"legacy_id":   entry.LegacyID,
		},
	}
	if err := m.store.SaveTrace(ctx, trace); err != nil {
		m.report.Errors = append(m.report.Errors, fmt.Sprintf("save trace: %v", err))
		return err
	}

	op := OperationRecord{
		OperationID: operationID,
		TraceID:     traceID,
		Type:        mapping.OperationType,
		ActorType:   ActorSystem,
		Status:      status,
		SubjectType: mapping.SubjectType,
		SubjectID:   entry.ToolID,
		CreatedAt:   time.Now(),
		Metadata: map[string]any{
			"legacy_type": entry.LegacyType,
			"legacy_id":   entry.LegacyID,
		},
	}
	if err := m.store.SaveOperation(ctx, op); err != nil {
		m.report.Errors = append(m.report.Errors, fmt.Sprintf("save op: %v", err))
		return err
	}

	inv := InvocationRecord{
		InvocationID:   invocationID,
		TraceID:        traceID,
		OperationID:    operationID,
		ExtensionID:    entry.ExtensionID,
		ModuleID:       entry.ModuleID,
		CapabilityID:   entry.ToolID,
		UserID:         entry.UserID,
		CharacterID:    entry.CharacterID,
		ConversationID: entry.ConversationID,
		Status:         status,
		ErrorCode:      entry.ErrorCode,
		CreatedAt:      time.Now(),
		Metadata: map[string]any{
			"legacy_type":   entry.LegacyType,
			"legacy_id":     entry.LegacyID,
			"legacy_status": entry.Status,
		},
	}
	if err := m.store.SaveInvocation(ctx, inv); err != nil {
		m.report.Errors = append(m.report.Errors, fmt.Sprintf("save inv: %v", err))
		return err
	}

	m.report.MigratedRecords++
	return nil
}

func (m *Migrator) Report() MigrationReport {
	return m.report
}

func (m *Migrator) TotalMigrated() int {
	return m.report.MigratedRecords
}
