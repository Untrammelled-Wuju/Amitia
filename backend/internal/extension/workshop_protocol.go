package extension

import (
	"bytes"
	"encoding/json"
	"time"
)

type WorkshopSessionStatus string

const (
	WorkshopDraft               WorkshopSessionStatus = "draft"
	WorkshopGenerating          WorkshopSessionStatus = "generating"
	WorkshopGenerated           WorkshopSessionStatus = "generated"
	WorkshopValidating          WorkshopSessionStatus = "validating"
	WorkshopValidationFailed    WorkshopSessionStatus = "validation_failed"
	WorkshopValidated           WorkshopSessionStatus = "validated"
	WorkshopAwaitingPermissions WorkshopSessionStatus = "awaiting_permission_confirmation"
	WorkshopTesting             WorkshopSessionStatus = "testing"
	WorkshopTestFailed          WorkshopSessionStatus = "test_failed"
	WorkshopTestPassed          WorkshopSessionStatus = "test_passed"
	WorkshopInstalling          WorkshopSessionStatus = "installing"
	WorkshopInstalled           WorkshopSessionStatus = "installed"
	WorkshopEnabled             WorkshopSessionStatus = "enabled"
	WorkshopDisabled            WorkshopSessionStatus = "disabled"
	WorkshopArchived            WorkshopSessionStatus = "archived"
	WorkshopError               WorkshopSessionStatus = "error"
)

const (
	ErrWorkshopSessionNotFound         = "WORKSHOP_SESSION_NOT_FOUND"
	ErrWorkshopSessionForbidden        = "WORKSHOP_SESSION_FORBIDDEN"
	ErrWorkshopInvalidState            = "WORKSHOP_INVALID_STATE"
	ErrWorkshopRevisionNotFound        = "WORKSHOP_REVISION_NOT_FOUND"
	ErrWorkshopRevisionConflict        = "WORKSHOP_REVISION_CONFLICT"
	ErrWorkshopGenerationFailed        = "WORKSHOP_GENERATION_FAILED"
	ErrWorkshopGenerationOutputInvalid = "WORKSHOP_GENERATION_OUTPUT_INVALID"
	ErrWorkshopManifestInvalid         = "WORKSHOP_MANIFEST_INVALID"
	ErrWorkshopWorkflowInvalid         = "WORKSHOP_WORKFLOW_INVALID"
	ErrWorkshopSchemaInvalid           = "WORKSHOP_SCHEMA_INVALID"
	ErrWorkshopStaticAnalysisFailed    = "WORKSHOP_STATIC_ANALYSIS_FAILED"
	ErrWorkshopCapabilityMismatch      = "WORKSHOP_CAPABILITY_MISMATCH"
	ErrWorkshopPermissionRequired      = "WORKSHOP_PERMISSION_CONFIRMATION_REQUIRED"
	ErrWorkshopPermissionStale         = "WORKSHOP_PERMISSION_CONFIRMATION_STALE"
	ErrWorkshopSecretDetected          = "WORKSHOP_SECRET_DETECTED"
	ErrWorkshopNetworkDenied           = "WORKSHOP_NETWORK_TARGET_DENIED"
	ErrWorkshopDependencyNotFound      = "WORKSHOP_DEPENDENCY_NOT_FOUND"
	ErrWorkshopDependencyCycle         = "WORKSHOP_DEPENDENCY_CYCLE"
	ErrWorkshopTestRequired            = "WORKSHOP_TEST_REQUIRED"
	ErrWorkshopTestFailed              = "WORKSHOP_TEST_FAILED"
	ErrWorkshopTestStale               = "WORKSHOP_TEST_STALE"
	ErrWorkshopSandboxLimit            = "WORKSHOP_SANDBOX_LIMIT_EXCEEDED"
	ErrWorkshopInstallFailed           = "WORKSHOP_INSTALL_FAILED"
	ErrWorkshopSkillIDConflict         = "WORKSHOP_SKILL_ID_CONFLICT"
	ErrWorkshopVersionConflict         = "WORKSHOP_VERSION_CONFLICT"
	ErrWorkshopArtifactInvalid         = "WORKSHOP_ARTIFACT_INVALID"
	ErrWorkshopChecksumMismatch        = "WORKSHOP_CHECKSUM_MISMATCH"
	ErrWorkshopRollbackFailed          = "WORKSHOP_ROLLBACK_FAILED"
	ErrWorkflowStepInvalid             = "WORKFLOW_STEP_INVALID"
	ErrWorkflowStepTimeout             = "WORKFLOW_STEP_TIMEOUT"
	ErrWorkflowReferenceInvalid        = "WORKFLOW_REFERENCE_INVALID"
	ErrWorkflowOutputInvalid           = "WORKFLOW_OUTPUT_INVALID"
)

type WorkshopSession struct {
	ID                            string                `json:"id"`
	UserID                        string                `json:"userId"`
	CharacterID                   string                `json:"characterId,omitempty"`
	Status                        WorkshopSessionStatus `json:"status"`
	Requirement                   string                `json:"requirement"`
	CurrentRevision               int64                 `json:"currentRevision"`
	CurrentDraftID                string                `json:"currentDraftId,omitempty"`
	ValidationSummary             json.RawMessage       `json:"validationSummary"`
	RiskSummary                   json.RawMessage       `json:"riskSummary"`
	TestSummary                   json.RawMessage       `json:"testSummary"`
	InstalledSkillID              string                `json:"installedSkillId,omitempty"`
	InstalledVersion              string                `json:"installedVersion,omitempty"`
	TestPermissionConfirmed       bool                  `json:"testPermissionConfirmed"`
	ProductionPermissionConfirmed bool                  `json:"productionPermissionConfirmed"`
	CreatedAt                     time.Time             `json:"createdAt"`
	UpdatedAt                     time.Time             `json:"updatedAt"`
	ArchivedAt                    *time.Time            `json:"archivedAt,omitempty"`
}

type DraftMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	License     string `json:"license"`
}

type DraftIntent struct {
	Goal           string         `json:"goal"`
	Triggers       []SkillTrigger `json:"triggers"`
	SideEffects    []string       `json:"sideEffects"`
	IdempotencyKey string         `json:"idempotencyKey,omitempty"`
}

type SkillDependency struct {
	SkillID  string `json:"skillId"`
	Version  string `json:"version,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

type DraftAssumption struct {
	Message string `json:"message"`
}

func (a *DraftAssumption) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		return json.Unmarshal(trimmed, &a.Message)
	}
	var value struct {
		Message string `json:"message"`
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	a.Message = value.Message
	return nil
}

type DraftWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

func (w *DraftWarning) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		w.Code = "MODEL_WARNING"
		return json.Unmarshal(trimmed, &w.Message)
	}
	var value struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Path    string `json:"path,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	*w = DraftWarning{Code: value.Code, Message: value.Message, Path: value.Path}
	return nil
}

type ExtensionDraft struct {
	DraftVersion  string             `json:"draftVersion"`
	Metadata      DraftMetadata      `json:"metadata"`
	Intent        DraftIntent        `json:"intent"`
	Manifest      Manifest           `json:"manifest"`
	InputSchema   json.RawMessage    `json:"inputSchema"`
	OutputSchema  json.RawMessage    `json:"outputSchema"`
	ConfigSchema  json.RawMessage    `json:"configSchema"`
	DefaultConfig json.RawMessage    `json:"defaultConfig"`
	Workflow      WorkflowDefinition `json:"workflow"`
	Capabilities  []string           `json:"capabilities"`
	Dependencies  []SkillDependency  `json:"dependencies"`
	TestCases     []WorkshopTestCase `json:"testCases"`
	Assumptions   []DraftAssumption  `json:"assumptions"`
	Warnings      []DraftWarning     `json:"warnings"`
}

type PlannedField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}
type PlannedStep struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

func (s *PlannedStep) UnmarshalJSON(data []byte) error {
	var value struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Purpose     string `json:"purpose"`
		Description string `json:"description"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	s.ID = value.ID
	s.Type = value.Type
	s.Purpose = value.Purpose
	if s.Purpose == "" {
		s.Purpose = value.Description
	}
	return nil
}

type WorkshopPlan struct {
	Goal           string         `json:"goal"`
	Inputs         []PlannedField `json:"inputs"`
	Outputs        []PlannedField `json:"outputs"`
	Configs        []PlannedField `json:"configs"`
	Steps          []PlannedStep  `json:"steps"`
	Dependencies   []string       `json:"dependencies"`
	Capabilities   []string       `json:"capabilities"`
	SideEffects    []string       `json:"sideEffects"`
	Risks          []string       `json:"risks"`
	MissingDetails []string       `json:"missingDetails"`
	Assumptions    []string       `json:"assumptions"`
}

func (p *WorkshopPlan) UnmarshalJSON(data []byte) error {
	var value struct {
		Goal           string          `json:"goal"`
		Inputs         []PlannedField  `json:"inputs"`
		Outputs        []PlannedField  `json:"outputs"`
		Configs        []PlannedField  `json:"configs"`
		Steps          []PlannedStep   `json:"steps"`
		Dependencies   []string        `json:"dependencies"`
		Capabilities   []string        `json:"capabilities"`
		SideEffects    json.RawMessage `json:"sideEffects"`
		Risks          []string        `json:"risks"`
		MissingDetails []string        `json:"missingDetails"`
		Assumptions    []string        `json:"assumptions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	var sideEffects []string
	trimmed := bytes.TrimSpace(value.SideEffects)
	if bytes.Equal(trimmed, []byte("false")) {
		sideEffects = []string{}
	} else if err := json.Unmarshal(trimmed, &sideEffects); err != nil {
		return err
	}
	*p = WorkshopPlan{Goal: value.Goal, Inputs: value.Inputs, Outputs: value.Outputs, Configs: value.Configs, Steps: value.Steps, Dependencies: value.Dependencies, Capabilities: value.Capabilities, SideEffects: sideEffects, Risks: value.Risks, MissingDetails: value.MissingDetails, Assumptions: value.Assumptions}
	return nil
}

type WorkflowLimits struct {
	MaxSteps               int   `json:"maxSteps"`
	MaxExecutionDurationMS int64 `json:"maxExecutionDurationMs"`
	MaxStepDurationMS      int64 `json:"maxStepDurationMs"`
	MaxInputBytes          int64 `json:"maxInputBytes"`
	MaxOutputBytes         int64 `json:"maxOutputBytes"`
	MaxIntermediateBytes   int64 `json:"maxIntermediateBytes"`
	MaxHTTPResponseBytes   int64 `json:"maxHttpResponseBytes"`
	MaxHTTPRedirects       int   `json:"maxHttpRedirects"`
	MaxSkillCallDepth      int   `json:"maxSkillCallDepth"`
	MaxSkillCalls          int   `json:"maxSkillCalls"`
	MaxArrayItems          int   `json:"maxArrayItems"`
	MaxExpressionDepth     int   `json:"maxExpressionDepth"`
	MaxTemplateLength      int   `json:"maxTemplateLength"`
	MaxEventsEmitted       int   `json:"maxEventsEmitted"`
	MaxSchedulesCreated    int   `json:"maxSchedulesCreated"`
	MaxSideEffects         int   `json:"maxSideEffects"`
}

type WorkflowErrorPolicy struct {
	Mode    string          `json:"mode"`
	Default json.RawMessage `json:"default,omitempty"`
}
type ConditionExpression struct {
	Op    string                `json:"op"`
	Args  []ConditionExpression `json:"args,omitempty"`
	Left  interface{}           `json:"left,omitempty"`
	Right interface{}           `json:"right,omitempty"`
	Value interface{}           `json:"value,omitempty"`
}
type WorkflowStep struct {
	ID      string               `json:"id"`
	Type    string               `json:"type"`
	Input   json.RawMessage      `json:"input"`
	When    *ConditionExpression `json:"when,omitempty"`
	OnError WorkflowErrorPolicy  `json:"onError"`
}
type WorkflowDefinition struct {
	SchemaVersion string          `json:"schemaVersion"`
	Steps         []WorkflowStep  `json:"steps"`
	Output        json.RawMessage `json:"output"`
	Limits        WorkflowLimits  `json:"limits"`
}
type CompiledStep struct {
	ID        string               `json:"id"`
	Type      string               `json:"type"`
	Input     json.RawMessage      `json:"input"`
	When      *ConditionExpression `json:"when,omitempty"`
	OnError   WorkflowErrorPolicy  `json:"onError"`
	TimeoutMS int64                `json:"timeoutMs"`
}
type ResolvedSkillDependency struct {
	SkillID        string   `json:"skillId"`
	Version        string   `json:"version"`
	Capabilities   []string `json:"capabilities"`
	HasSideEffects bool     `json:"hasSideEffects"`
	Idempotent     bool     `json:"idempotent"`
}
type CompiledWorkflow struct {
	SchemaVersion  string                    `json:"schemaVersion"`
	Steps          []CompiledStep            `json:"steps"`
	Output         json.RawMessage           `json:"output"`
	Capabilities   []string                  `json:"capabilities"`
	Dependencies   []ResolvedSkillDependency `json:"dependencies"`
	Limits         WorkflowLimits            `json:"limits"`
	HasSideEffects bool                      `json:"hasSideEffects"`
	Idempotent     bool                      `json:"idempotent"`
	Checksum       string                    `json:"checksum"`
}

type AnalysisIssue struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	StepID  string `json:"stepId,omitempty"`
}
type CapabilityAnalysis struct {
	Required []string            `json:"required"`
	Declared []string            `json:"declared"`
	Missing  []string            `json:"missing"`
	Excess   []string            `json:"excess"`
	ByStep   map[string][]string `json:"byStep"`
	HighRisk []string            `json:"highRisk"`
}
type WorkshopValidationResult struct {
	SessionID        string             `json:"sessionId"`
	Revision         int64              `json:"revision"`
	WorkflowChecksum string             `json:"workflowChecksum"`
	Valid            bool               `json:"valid"`
	Issues           []AnalysisIssue    `json:"issues"`
	Capabilities     CapabilityAnalysis `json:"capabilities"`
	HasSideEffects   bool               `json:"hasSideEffects"`
	Idempotent       bool               `json:"idempotent"`
	ValidatedAt      time.Time          `json:"validatedAt"`
}

type HTTPMock struct {
	Method          string          `json:"method"`
	URL             string          `json:"url"`
	Query           json.RawMessage `json:"query,omitempty"`
	Headers         json.RawMessage `json:"headers,omitempty"`
	Body            json.RawMessage `json:"body,omitempty"`
	Status          int             `json:"status"`
	ResponseHeaders json.RawMessage `json:"responseHeaders,omitempty"`
	ResponseBody    json.RawMessage `json:"responseBody"`
	DelayMS         int64           `json:"delayMs,omitempty"`
	Error           string          `json:"error,omitempty"`
}
type SkillMock struct {
	SkillID     string             `json:"skillId"`
	Input       json.RawMessage    `json:"input,omitempty"`
	Output      json.RawMessage    `json:"output,omitempty"`
	Status      RunStatus          `json:"status"`
	DelayMS     int64              `json:"delayMs,omitempty"`
	Error       *ExtensionError    `json:"error,omitempty"`
	SideEffects []SideEffectRecord `json:"sideEffects,omitempty"`
}
type TestAssertion struct {
	Type     string      `json:"type"`
	Path     string      `json:"path,omitempty"`
	Expected interface{} `json:"expected,omitempty"`
	StepID   string      `json:"stepId,omitempty"`
}
type WorkshopTestCase struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Mode           string          `json:"mode"`
	Input          json.RawMessage `json:"input"`
	Config         json.RawMessage `json:"config"`
	SecretRefs     []string        `json:"secretRefs"`
	HTTPMocks      []HTTPMock      `json:"httpMocks"`
	SkillMocks     []SkillMock     `json:"skillMocks"`
	ExpectedOutput json.RawMessage `json:"expectedOutput,omitempty"`
	Assertions     []TestAssertion `json:"assertions"`
}
type WorkflowStepResult struct {
	StepID        string          `json:"stepId"`
	Type          string          `json:"type"`
	Status        string          `json:"status"`
	InputSummary  string          `json:"inputSummary"`
	OutputSummary string          `json:"outputSummary"`
	DurationMS    int64           `json:"durationMs"`
	Mocked        bool            `json:"mocked"`
	Error         *ExtensionError `json:"error,omitempty"`
}
type AssertionResult struct {
	Type    string `json:"type"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}
type WorkshopTestReport struct {
	TestRunID        string               `json:"testRunId"`
	SessionID        string               `json:"sessionId"`
	Revision         int64                `json:"revision"`
	WorkflowChecksum string               `json:"workflowChecksum"`
	Status           string               `json:"status"`
	StartedAt        time.Time            `json:"startedAt"`
	FinishedAt       time.Time            `json:"finishedAt"`
	DurationMS       int64                `json:"durationMs"`
	StepResults      []WorkflowStepResult `json:"stepResults"`
	Assertions       []AssertionResult    `json:"assertions"`
	SideEffects      []SideEffectRecord   `json:"sideEffects"`
	Capabilities     []string             `json:"capabilities"`
	Warnings         []DraftWarning       `json:"warnings"`
	Output           json.RawMessage      `json:"output,omitempty"`
	Error            *ExtensionError      `json:"error,omitempty"`
}

type PermissionConfirmation struct {
	WorkflowChecksum  string   `json:"workflowChecksum"`
	Capabilities      []string `json:"capabilities"`
	ConfirmedHighRisk []string `json:"confirmedHighRisk"`
	Production        bool     `json:"production"`
}
type WorkshopInstallResult struct {
	SessionID  string `json:"sessionId"`
	SkillID    string `json:"skillId"`
	Version    string `json:"version"`
	ArtifactID string `json:"artifactId"`
	Enabled    bool   `json:"enabled"`
}
type WorkshopSessionFilter struct {
	Status      WorkshopSessionStatus
	CharacterID string
	Page        int
	PageSize    int
}
type PagedWorkshopSessions struct {
	Items    []WorkshopSession `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}
type CreateWorkshopSessionRequest struct {
	Scope       ExecutionScope `json:"-"`
	Requirement string         `json:"requirement"`
	CharacterID string         `json:"characterId,omitempty"`
}
type GenerateWorkshopDraftRequest struct {
	Scope       ExecutionScope  `json:"-"`
	Requirement string          `json:"requirement,omitempty"`
	Draft       *ExtensionDraft `json:"draft,omitempty"`
}
type WorkshopTestRequest struct {
	Scope                   ExecutionScope     `json:"-"`
	Mode                    string             `json:"mode"`
	TestCaseIDs             []string           `json:"testCaseIds,omitempty"`
	TestCases               []WorkshopTestCase `json:"testCases,omitempty"`
	ControlledLiveConfirmed bool               `json:"controlledLiveConfirmed"`
}
type WorkshopRevisionView struct {
	ID               string                    `json:"id"`
	SessionID        string                    `json:"sessionId"`
	Revision         int64                     `json:"revision"`
	Plan             WorkshopPlan              `json:"plan"`
	Draft            ExtensionDraft            `json:"draft"`
	NormalizedDraft  ExtensionDraft            `json:"normalizedDraft"`
	WorkflowChecksum string                    `json:"workflowChecksum"`
	Validation       *WorkshopValidationResult `json:"validation,omitempty"`
	ModelProvider    string                    `json:"modelProvider,omitempty"`
	ModelName        string                    `json:"modelName,omitempty"`
	CreatedAt        time.Time                 `json:"createdAt"`
}
type WorkshopSessionDetailView struct {
	WorkshopSession
	Revision    *WorkshopRevisionView `json:"revision,omitempty"`
	TestReports []WorkshopTestReport  `json:"testReports"`
}
type WorkshopArtifactView struct {
	ArtifactID       string          `json:"artifactId"`
	ExtensionID      string          `json:"extensionId"`
	ExtensionVersion string          `json:"extensionVersion"`
	SessionID        string          `json:"sessionId"`
	Revision         int64           `json:"revision"`
	Manifest         json.RawMessage `json:"manifest"`
	Workflow         json.RawMessage `json:"workflow"`
	Schemas          json.RawMessage `json:"schemas"`
	Tests            json.RawMessage `json:"tests"`
	Readme           string          `json:"readme"`
	Checksum         string          `json:"checksum"`
	SizeBytes        int64           `json:"sizeBytes"`
	CreatedAt        time.Time       `json:"createdAt"`
}
