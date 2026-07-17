package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type WorkshopService struct {
	repository    *WorkshopRepository
	generator     *WorkshopGenerator
	compiler      *WorkflowCompiler
	executor      *WorkflowExecutor
	validator     *SchemaValidator
	registry      *Registry
	skillExecutor *Executor
	installer     *WorkshopInstaller
	agentSkills   *AgentSkillService
	sessionLocks  sync.Map
}

func NewWorkshopService(repository *WorkshopRepository, generator *WorkshopGenerator, compiler *WorkflowCompiler, executor *WorkflowExecutor, validator *SchemaValidator, registry *Registry, skillExecutor *Executor) *WorkshopService {
	service := &WorkshopService{repository: repository, generator: generator, compiler: compiler, executor: executor, validator: validator, registry: registry, skillExecutor: skillExecutor}
	service.installer = NewWorkshopInstaller(repository, registry, compiler, executor, validator)
	return service
}
func (s *WorkshopService) SetModelGenerator(model WorkshopModelGenerator) {
	s.generator.SetModel(model)
}

func (s *WorkshopService) AttachAgentSkills(service *AgentSkillService) { s.agentSkills = service }

func (s *WorkshopService) GenerateInstruction(ctx context.Context, scope ExecutionScope, requirement string) (AgentSkillImportPreview, error) {
	if s.agentSkills == nil {
		return AgentSkillImportPreview{}, NewExtensionError(ErrWorkshopGenerationFailed, "Agent Skill 工坊不可用", "", false, nil)
	}
	draft, err := s.generator.GenerateInstruction(ctx, strings.TrimSpace(requirement))
	if err != nil {
		return AgentSkillImportPreview{}, err
	}
	files := map[string][]byte{"SKILL.md": []byte(fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s", draft.Name, strconv.Quote(draft.Description), draft.Body))}
	for name, content := range draft.References {
		files["references/"+name] = []byte(content)
	}
	for name, content := range draft.Assets {
		files["assets/"+name] = []byte(content)
	}
	if draft.DisplayName != "" || draft.ShortDescription != "" {
		files["agents/openai.yaml"] = []byte(fmt.Sprintf("interface:\n  display_name: %s\n  short_description: %s\n", strconv.Quote(draft.DisplayName), strconv.Quote(draft.ShortDescription)))
	}
	parsed, err := parseAgentSkillFiles(files, draft.Name, AgentSkillSourceWorkshop, s.agentSkills.limits)
	if err != nil {
		return AgentSkillImportPreview{}, err
	}
	return s.agentSkills.storePreview(scope.UserID, parsed), nil
}
func (s *WorkshopService) lockSession(id string) (func(), bool) {
	value, _ := s.sessionLocks.LoadOrStore(id, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	if !mutex.TryLock() {
		return nil, false
	}
	return mutex.Unlock, true
}

func (s *WorkshopService) CreateSession(ctx context.Context, request CreateWorkshopSessionRequest) (result WorkshopSession, err error) {
	defer func() {
		if err == nil {
			incrementWorkshopMetric(WorkshopMetricSessionCreated)
		}
		recordWorkshopErrorMetric(err)
	}()
	requirement := strings.TrimSpace(request.Requirement)
	if requirement == "" || len(requirement) > 20000 {
		return WorkshopSession{}, NewExtensionError(ErrWorkshopGenerationOutputInvalid, "需求不能为空且不能超过 20000 字符", "", false, nil)
	}
	characterID := request.CharacterID
	if characterID == "" {
		characterID = request.Scope.CharacterID
	}
	return s.repository.CreateSession(ctx, request.Scope, requirement, characterID)
}
func (s *WorkshopService) ListSessions(ctx context.Context, scope ExecutionScope, filter WorkshopSessionFilter) (PagedWorkshopSessions, error) {
	return s.repository.ListSessions(ctx, scope, filter)
}
func (s *WorkshopService) GetSession(ctx context.Context, scope ExecutionScope, id string) (WorkshopSessionDetailView, error) {
	session, _, err := s.repository.GetSession(ctx, scope, id)
	if err != nil {
		return WorkshopSessionDetailView{}, err
	}
	var revision *WorkshopRevisionView
	if session.CurrentRevision > 0 {
		view, _, getErr := s.repository.GetRevision(ctx, scope, id, session.CurrentRevision)
		if getErr != nil {
			return WorkshopSessionDetailView{}, getErr
		}
		revision = &view
	}
	tests, err := s.repository.ListTestReports(ctx, scope, id)
	if err != nil {
		return WorkshopSessionDetailView{}, err
	}
	return WorkshopSessionDetailView{WorkshopSession: session, Revision: revision, TestReports: tests}, nil
}
func (s *WorkshopService) ListRevisions(ctx context.Context, scope ExecutionScope, id string) ([]WorkshopRevisionView, error) {
	return s.repository.ListRevisions(ctx, scope, id)
}
func (s *WorkshopService) GetRevision(ctx context.Context, scope ExecutionScope, id string, revision int64) (WorkshopRevisionView, error) {
	view, _, err := s.repository.GetRevision(ctx, scope, id, revision)
	return view, err
}

func (s *WorkshopService) Generate(ctx context.Context, sessionID string, request GenerateWorkshopDraftRequest) (result WorkshopRevisionView, err error) {
	incrementWorkshopMetric(WorkshopMetricGeneration)
	defer func() {
		if err != nil {
			incrementWorkshopMetric(WorkshopMetricGenerationFailure)
		}
		recordWorkshopErrorMetric(err)
	}()
	unlock, ok := s.lockSession(sessionID)
	if !ok {
		return WorkshopRevisionView{}, NewExtensionError(ErrWorkshopRevisionConflict, "该会话正在生成或安装", sessionID, true, nil)
	}
	defer unlock()
	session, record, err := s.repository.GetSession(ctx, request.Scope, sessionID)
	if err != nil {
		return WorkshopRevisionView{}, err
	}
	if session.Status == WorkshopArchived || session.Status == WorkshopInstalling {
		return WorkshopRevisionView{}, NewExtensionError(ErrWorkshopInvalidState, "当前状态不能生成新修订", string(session.Status), false, nil)
	}
	if err := s.repository.CASStatus(ctx, request.Scope, sessionID, record.LockVersion, []WorkshopSessionStatus{session.Status}, WorkshopGenerating, "revision.generate.started", map[string]interface{}{}); err != nil {
		return WorkshopRevisionView{}, err
	}
	_, record, err = s.repository.GetSession(ctx, request.Scope, sessionID)
	if err != nil {
		return WorkshopRevisionView{}, err
	}
	completed := false
	defer func() {
		if completed {
			return
		}
		_, current, getErr := s.repository.GetSession(context.Background(), request.Scope, sessionID)
		if getErr == nil && WorkshopSessionStatus(current.Status) == WorkshopGenerating {
			_ = s.repository.CASStatus(context.Background(), request.Scope, sessionID, current.LockVersion, []WorkshopSessionStatus{WorkshopGenerating}, WorkshopError, "revision.generate.failed", map[string]interface{}{})
		}
	}()
	requirement := strings.TrimSpace(request.Requirement)
	if requirement == "" {
		requirement = session.Requirement
	}
	var draft ExtensionDraft
	var plan WorkshopPlan
	var raw, provider, model string
	if request.Draft != nil {
		draft = *request.Draft
		plan = planFromDraft(draft)
		rawBytes, _ := json.Marshal(draft)
		raw = string(rawBytes)
		provider = "structured-editor"
	} else {
		draft, plan, raw, provider, model, err = s.generator.Generate(ctx, requirement)
		if err != nil {
			return WorkshopRevisionView{}, err
		}
	}
	normalized, warnings := normalizeWorkshopDraft(draft, request.Scope.UserID)
	compiled, compilerIssues, compileErr := s.compiler.Compile(ctx, normalized.Workflow)
	if compileErr != nil {
		return WorkshopRevisionView{}, NewExtensionError(ErrWorkshopGenerationOutputInvalid, "生成的工作流无法编译", summarizeIssues(compilerIssues), false, compileErr)
	}
	declared := append([]string(nil), normalized.Capabilities...)
	normalized.Capabilities = append([]string{}, compiled.Capabilities...)
	if !sameStringSets(declared, compiled.Capabilities) {
		normalized.Warnings = append(warnings, DraftWarning{Code: "CAPABILITIES_DERIVED", Message: "Capability 已根据工作流行为重新推导", Path: "capabilities"})
	}
	artifactID := "artifact." + uuid.New().String()
	normalized.Manifest = buildWorkshopManifest(normalized, compiled, artifactID)
	normalized.Intent.SideEffects = sideEffectNames(normalized.Workflow, compiled)
	normalized.Dependencies = dependenciesFromCompiled(compiled)
	normalized.Workflow.Limits = compiled.Limits
	compiled, compilerIssues, compileErr = s.compiler.Compile(ctx, normalized.Workflow)
	if compileErr != nil {
		return WorkshopRevisionView{}, NewExtensionError(ErrWorkshopGenerationOutputInvalid, "规范化工作流无法编译", summarizeIssues(compilerIssues), false, compileErr)
	}
	analysis := analyzeCapabilityDeclaration(normalized.Capabilities, compiled)
	view, err := s.repository.SaveRevision(ctx, request.Scope, record, requirement, plan, raw, draft, normalized, compiled, analysis, provider, model)
	completed = err == nil
	return view, err
}

func (s *WorkshopService) Validate(ctx context.Context, scope ExecutionScope, sessionID string, revision int64) (result WorkshopValidationResult, err error) {
	defer func() {
		if err != nil || !result.Valid {
			incrementWorkshopMetric(WorkshopMetricValidationFailure)
		}
		recordWorkshopErrorMetric(err)
	}()
	unlock, ok := s.lockSession(sessionID)
	if !ok {
		return WorkshopValidationResult{}, NewExtensionError(ErrWorkshopRevisionConflict, "该会话正在执行其他操作", sessionID, true, nil)
	}
	defer unlock()
	session, record, err := s.repository.GetSession(ctx, scope, sessionID)
	if err != nil {
		return WorkshopValidationResult{}, err
	}
	if revision != session.CurrentRevision {
		return WorkshopValidationResult{}, NewExtensionError(ErrWorkshopRevisionConflict, "只能校验当前修订", fmt.Sprint(revision), false, nil)
	}
	if session.Status == WorkshopArchived || session.Status == WorkshopInstalling {
		return WorkshopValidationResult{}, NewExtensionError(ErrWorkshopInvalidState, "当前状态不能校验", string(session.Status), false, nil)
	}
	if err := s.repository.CASStatus(ctx, scope, sessionID, record.LockVersion, []WorkshopSessionStatus{session.Status}, WorkshopValidating, "revision.validate.started", map[string]interface{}{}); err != nil {
		return WorkshopValidationResult{}, err
	}
	_, record, err = s.repository.GetSession(ctx, scope, sessionID)
	if err != nil {
		return WorkshopValidationResult{}, err
	}
	view, _, err := s.repository.GetRevision(ctx, scope, sessionID, revision)
	if err != nil {
		return WorkshopValidationResult{}, err
	}
	draft := view.NormalizedDraft
	compiled, issues, _ := s.compiler.Compile(ctx, draft.Workflow)
	issues = append(issues, ScanWorkshopSecrets(mustJSON(draft))...)
	declaredSecrets := workshopSecretFields(draft.ConfigSchema)
	for _, step := range draft.Workflow.Steps {
		for _, name := range secretReferenceNames(step.Input) {
			if !declaredSecrets[name] {
				issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSecretDetected, Message: "Secret 引用未在 Config Schema 中声明为 writeOnly", Path: "configSchema.properties." + name, StepID: step.ID})
			}
		}
	}
	if draft.Manifest.Kind != "Skill" {
		issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopManifestInvalid, Message: "工坊只能生成 Skill", Path: "manifest.kind"})
	}
	if draft.Manifest.Entry.Kind != "workflow" {
		issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopManifestInvalid, Message: "工坊入口只能是 workflow", Path: "manifest.entry.kind"})
	}
	if strings.HasPrefix(draft.Metadata.ID, "dev.amitia.") {
		issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopManifestInvalid, Message: "禁止使用官方命名空间", Path: "metadata.id"})
	}
	if current, getErr := s.registry.Get(ctx, draft.Metadata.ID); getErr == nil {
		if compareSemver(draft.Metadata.Version, current.Definition.Version) <= 0 {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopVersionConflict, Message: "更新版本必须高于当前生产版本", Path: "metadata.version"})
		} else {
			suggestion, breaking := suggestWorkshopVersion(current.Definition, draft)
			if len(breaking) > 0 && semverMajor(draft.Metadata.Version) <= semverMajor(current.Definition.Version) {
				issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopVersionConflict, Message: "不兼容 Schema 变更必须增加 MAJOR 版本: " + strings.Join(breaking, "; "), Path: "metadata.version"})
			} else {
				issues = append(issues, AnalysisIssue{Level: "info", Code: "WORKSHOP_VERSION_SUGGESTION", Message: "后端建议版本: " + suggestion, Path: "metadata.version"})
			}
		}
	}
	manifestRaw, _ := json.Marshal(draft.Manifest)
	if err := s.validator.ValidateManifest(manifestRaw); err != nil {
		issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopManifestInvalid, Message: err.Error(), Path: "manifest"})
	}
	for name, schema := range map[string]json.RawMessage{"inputSchema": draft.InputSchema, "outputSchema": draft.OutputSchema, "configSchema": draft.ConfigSchema} {
		if name == "configSchema" && len(schema) == 0 {
			continue
		}
		if err := s.validator.ValidateSchema("workshop-"+name, schema); err != nil {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSchemaInvalid, Message: err.Error(), Path: name})
		}
	}
	if len(draft.ConfigSchema) > 0 {
		if err := s.validator.Validate("workshop-default-config", draft.ConfigSchema, normalizeJSON(draft.DefaultConfig)); err != nil {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSchemaInvalid, Message: "默认配置不符合 Config Schema: " + err.Error(), Path: "defaultConfig"})
		}
	}
	capabilityAnalysis := analyzeCapabilityDeclaration(draft.Capabilities, compiled)
	issues = append(issues, s.compiler.AnalyzeDependencyCycles(ctx, draft.Metadata.ID, compiled.Dependencies)...)
	for _, missing := range capabilityAnalysis.Missing {
		issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopCapabilityMismatch, Message: "Manifest 缺少 Capability: " + missing, Path: "capabilities"})
	}
	for _, excess := range capabilityAnalysis.Excess {
		issues = append(issues, AnalysisIssue{Level: "warning", Code: "WORKSHOP_CAPABILITY_EXCESS", Message: "未使用的 Capability: " + excess, Path: "capabilities"})
	}
	result = WorkshopValidationResult{SessionID: sessionID, Revision: revision, WorkflowChecksum: compiled.Checksum, Valid: !hasErrorIssues(issues), Issues: issues, Capabilities: capabilityAnalysis, HasSideEffects: compiled.HasSideEffects, Idempotent: compiled.Idempotent, ValidatedAt: time.Now().UTC()}
	if err := s.repository.SaveValidation(ctx, scope, record, revision, result); err != nil {
		return WorkshopValidationResult{}, err
	}
	return result, nil
}

func workshopSecretFields(schemaRaw json.RawMessage) map[string]bool {
	var schema struct {
		Properties map[string]struct {
			Format    string `json:"format"`
			WriteOnly bool   `json:"writeOnly"`
		} `json:"properties"`
	}
	_ = json.Unmarshal(normalizeJSON(schemaRaw), &schema)
	result := map[string]bool{}
	for name, property := range schema.Properties {
		if property.WriteOnly || property.Format == "password" || property.Format == "secret" {
			result[name] = true
		}
	}
	return result
}

func (s *WorkshopService) ConfirmPermissions(ctx context.Context, scope ExecutionScope, sessionID string, revision int64, confirmation PermissionConfirmation) error {
	session, record, err := s.repository.GetSession(ctx, scope, sessionID)
	if err != nil {
		return err
	}
	allowedState := session.Status == WorkshopValidated || session.Status == WorkshopAwaitingPermissions
	if confirmation.Production {
		allowedState = session.Status == WorkshopTestPassed
	}
	if revision != session.CurrentRevision || !allowedState {
		return NewExtensionError(ErrWorkshopInvalidState, "必须先通过当前修订校验", string(session.Status), false, nil)
	}
	view, _, err := s.repository.GetRevision(ctx, scope, sessionID, revision)
	if err != nil {
		return err
	}
	if view.Validation == nil || !view.Validation.Valid || confirmation.WorkflowChecksum != view.WorkflowChecksum {
		return NewExtensionError(ErrWorkshopPermissionStale, "权限确认与当前修订不匹配", "", false, nil)
	}
	required := view.Validation.Capabilities.Required
	if !sameStringSets(required, confirmation.Capabilities) {
		return NewExtensionError(ErrWorkshopPermissionRequired, "必须逐项确认全部 Capability", "", false, nil)
	}
	high := view.Validation.Capabilities.HighRisk
	for _, capability := range high {
		if !containsString(confirmation.ConfirmedHighRisk, capability) {
			return NewExtensionError(ErrWorkshopPermissionRequired, "高风险 Capability 必须单独确认", capability, false, nil)
		}
	}
	return s.repository.SaveConfirmation(ctx, scope, record, revision, view.WorkflowChecksum, confirmation)
}

func (s *WorkshopService) Test(ctx context.Context, scope ExecutionScope, sessionID string, revision int64, request WorkshopTestRequest) (result WorkshopTestReport, err error) {
	incrementWorkshopMetric(WorkshopMetricTest)
	defer func() {
		if err != nil || result.Status == "failed" {
			incrementWorkshopMetric(WorkshopMetricTestFailure)
		}
		recordWorkshopErrorMetric(err)
	}()
	unlock, ok := s.lockSession(sessionID)
	if !ok {
		return WorkshopTestReport{}, NewExtensionError(ErrWorkshopRevisionConflict, "该会话正在执行其他操作", sessionID, true, nil)
	}
	defer unlock()
	session, record, err := s.repository.GetSession(ctx, scope, sessionID)
	if err != nil {
		return WorkshopTestReport{}, err
	}
	if revision != session.CurrentRevision || session.Status != WorkshopAwaitingPermissions && session.Status != WorkshopTestFailed && session.Status != WorkshopTestPassed {
		return WorkshopTestReport{}, NewExtensionError(ErrWorkshopInvalidState, "必须完成校验和测试权限确认", string(session.Status), false, nil)
	}
	if request.Mode == "controlled_live" && !request.ControlledLiveConfirmed {
		return WorkshopTestReport{}, NewExtensionError(ErrWorkshopPermissionRequired, "Controlled Live 需要单独确认", "", false, nil)
	}
	if request.Mode != "dry_run" && request.Mode != "mocked" && request.Mode != "controlled_live" {
		return WorkshopTestReport{}, NewExtensionError(ErrWorkshopInvalidState, "不支持的测试模式", request.Mode, false, nil)
	}
	view, revisionRecord, err := s.repository.GetRevision(ctx, scope, sessionID, revision)
	if err != nil {
		return WorkshopTestReport{}, err
	}
	if record.TestPermissionRevision != revision || record.TestPermissionChecksum != view.WorkflowChecksum {
		return WorkshopTestReport{}, NewExtensionError(ErrWorkshopPermissionStale, "测试权限确认已失效", "", false, nil)
	}
	var testConfirmation PermissionConfirmation
	if json.Unmarshal([]byte(record.TestPermissionConfirmationJSON), &testConfirmation) != nil || testConfirmation.Production {
		return WorkshopTestReport{}, NewExtensionError(ErrWorkshopPermissionRequired, "测试需要独立的测试权限确认", "", false, nil)
	}
	var compiled CompiledWorkflow
	if err := json.Unmarshal([]byte(revisionRecord.CompiledWorkflowJSON), &compiled); err != nil || compiled.Checksum != view.WorkflowChecksum {
		return WorkshopTestReport{}, NewExtensionError(ErrWorkshopChecksumMismatch, "工作流制品校验失败", "", false, err)
	}
	testCases := request.TestCases
	if len(testCases) == 0 {
		testCases = view.NormalizedDraft.TestCases
	}
	if len(testCases) == 0 {
		testCases = []WorkshopTestCase{{ID: "default", Name: "默认 Dry Run", Mode: request.Mode, Input: json.RawMessage(`{}`), Config: normalizeJSON(view.NormalizedDraft.DefaultConfig)}}
	}
	if err := s.repository.CASStatus(ctx, scope, sessionID, record.LockVersion, []WorkshopSessionStatus{session.Status}, WorkshopTesting, "revision.test.started", map[string]interface{}{}); err != nil {
		return WorkshopTestReport{}, err
	}
	started := time.Now().UTC()
	aggregate := WorkshopTestReport{TestRunID: uuid.New().String(), SessionID: sessionID, Revision: revision, WorkflowChecksum: compiled.Checksum, Status: "passed", StartedAt: started, Capabilities: compiled.Capabilities, Warnings: []DraftWarning{{Code: "mode:" + request.Mode, Message: "测试模式"}}}
	var storedInput json.RawMessage = json.RawMessage(`{}`)
	for _, testCase := range testCases {
		if len(request.TestCaseIDs) > 0 && !containsString(request.TestCaseIDs, testCase.ID) {
			continue
		}
		if testCase.Mode != "" && testCase.Mode != request.Mode {
			continue
		}
		storedInput = testCase.Input
		if err := s.validator.Validate("workshop-test-input", view.NormalizedDraft.InputSchema, normalizeJSON(testCase.Input)); err != nil {
			aggregate.Status = "failed"
			aggregate.Error = NewExtensionError(ErrWorkshopTestFailed, "测试输入不符合 Schema", err.Error(), false, err)
			break
		}
		execution, execErr := s.executor.Execute(ctx, WorkflowExecutionRequest{Workflow: compiled, Input: normalizeJSON(testCase.Input), Config: normalizeJSON(testCase.Config), Scope: scope, Mode: WorkflowExecutionMode(request.Mode), HTTPMocks: testCase.HTTPMocks, SkillMocks: testCase.SkillMocks}, view.NormalizedDraft.OutputSchema)
		aggregate.StepResults = append(aggregate.StepResults, execution.Steps...)
		aggregate.SideEffects = append(aggregate.SideEffects, execution.SideEffects...)
		aggregate.Output = execution.Output
		if execErr != nil {
			aggregate.Status = "failed"
			aggregate.Error = asExtensionError(execErr)
			break
		}
		assertions := evaluateAssertions(testCase.Assertions, execution, s.validator)
		aggregate.Assertions = append(aggregate.Assertions, assertions...)
		for _, assertion := range assertions {
			if !assertion.Passed {
				aggregate.Status = "failed"
			}
		}
	}
	aggregate.FinishedAt = time.Now().UTC()
	aggregate.DurationMS = aggregate.FinishedAt.Sub(started).Milliseconds()
	if aggregate.Status == "passed" && len(aggregate.StepResults) == 0 {
		aggregate.Status = "failed"
		aggregate.Error = NewExtensionError(ErrWorkshopTestFailed, "没有执行匹配的测试用例", "", false, nil)
	}
	aggregate = redactWorkshopTestReport(aggregate)
	if err := s.repository.SaveTestReport(ctx, scope, aggregate, storedInput); err != nil {
		return WorkshopTestReport{}, err
	}
	return aggregate, nil
}

func (s *WorkshopService) Install(ctx context.Context, scope ExecutionScope, sessionID string, revision int64) (result WorkshopInstallResult, err error) {
	incrementWorkshopMetric(WorkshopMetricInstall)
	defer func() {
		if err != nil {
			incrementWorkshopMetric(WorkshopMetricInstallFailure)
		}
		recordWorkshopErrorMetric(err)
	}()
	unlock, ok := s.lockSession(sessionID)
	if !ok {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopRevisionConflict, "该会话正在安装", sessionID, true, nil)
	}
	defer unlock()
	session, record, err := s.repository.GetSession(ctx, scope, sessionID)
	if err != nil {
		return WorkshopInstallResult{}, err
	}
	if session.Status == WorkshopInstalled && session.CurrentRevision == revision {
		return WorkshopInstallResult{SessionID: sessionID, SkillID: session.InstalledSkillID, Version: session.InstalledVersion, Enabled: false}, nil
	}
	if session.Status != WorkshopTestPassed || revision != session.CurrentRevision {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopInvalidState, "只有当前且测试通过的修订可以安装", string(session.Status), false, nil)
	}
	if err := s.repository.CASStatus(ctx, scope, sessionID, record.LockVersion, []WorkshopSessionStatus{WorkshopTestPassed}, WorkshopInstalling, "revision.install.started", map[string]interface{}{}); err != nil {
		return WorkshopInstallResult{}, err
	}
	result, err = s.installer.Install(ctx, scope, sessionID, revision)
	if err != nil {
		_, current, getErr := s.repository.GetSession(context.Background(), scope, sessionID)
		if getErr == nil && WorkshopSessionStatus(current.Status) == WorkshopInstalling {
			_ = s.repository.CASStatus(context.Background(), scope, sessionID, current.LockVersion, []WorkshopSessionStatus{WorkshopInstalling}, WorkshopTestPassed, "revision.install.failed", map[string]interface{}{})
		}
	}
	return result, err
}
func (s *WorkshopService) Archive(ctx context.Context, scope ExecutionScope, sessionID string) error {
	session, record, err := s.repository.GetSession(ctx, scope, sessionID)
	if err != nil {
		return err
	}
	if session.Status == WorkshopInstalling {
		return NewExtensionError(ErrWorkshopInvalidState, "安装中不能归档", "", false, nil)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return s.repository.CASStatus(ctx, scope, sessionID, record.LockVersion, []WorkshopSessionStatus{session.Status}, WorkshopArchived, "session.archived", map[string]interface{}{"archived_at": now})
}
func (s *WorkshopService) ListTests(ctx context.Context, scope ExecutionScope, sessionID string) ([]WorkshopTestReport, error) {
	return s.repository.ListTestReports(ctx, scope, sessionID)
}
func (s *WorkshopService) GetTest(ctx context.Context, scope ExecutionScope, testRunID string) (WorkshopTestReport, error) {
	return s.repository.GetTestReport(ctx, scope, testRunID)
}
func (s *WorkshopService) Rollback(ctx context.Context, scope ExecutionScope, skillID, version string) (result WorkshopInstallResult, err error) {
	incrementWorkshopMetric(WorkshopMetricRollback)
	defer func() { recordWorkshopErrorMetric(err) }()
	return s.installer.Rollback(ctx, scope, skillID, version)
}
func (s *WorkshopService) Restore(ctx context.Context) error { return s.installer.Restore(ctx) }

func (s *WorkshopService) GetArtifact(ctx context.Context, scope ExecutionScope, sessionID string) (WorkshopArtifactView, error) {
	record, err := s.repository.GetSessionArtifact(ctx, scope, sessionID)
	if err != nil {
		return WorkshopArtifactView{}, err
	}
	created, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
	return WorkshopArtifactView{ArtifactID: record.ArtifactID, ExtensionID: record.ExtensionID, ExtensionVersion: record.ExtensionVersion, SessionID: record.SessionID, Revision: record.Revision, Manifest: json.RawMessage(record.ManifestJSON), Workflow: json.RawMessage(record.WorkflowJSON), Schemas: json.RawMessage(record.SchemasJSON), Tests: json.RawMessage(record.TestsJSON), Readme: record.ReadmeText, Checksum: record.Checksum, SizeBytes: record.SizeBytes, CreatedAt: created}, nil
}

func (s *WorkshopService) ForkSkill(ctx context.Context, scope ExecutionScope, skillID string) (WorkshopSessionDetailView, error) {
	registered, err := s.registry.Get(ctx, skillID)
	if err != nil {
		return WorkshopSessionDetailView{}, err
	}
	if registered.Definition.Source != SkillSourceWorkflow {
		return WorkshopSessionDetailView{}, NewExtensionError(ErrWorkshopArtifactInvalid, "只有工坊 Workflow Skill 可以创建修订分支", skillID, false, nil)
	}
	var artifact extensionArtifactRecord
	if err := s.repository.db.WithContext(ctx).Where("extension_id = ? AND extension_version = ?", skillID, registered.Definition.Version).First(&artifact).Error; err != nil {
		return WorkshopSessionDetailView{}, err
	}
	var manifest Manifest
	var workflow WorkflowDefinition
	var schemas map[string]json.RawMessage
	_ = json.Unmarshal([]byte(artifact.ManifestJSON), &manifest)
	_ = json.Unmarshal([]byte(artifact.WorkflowJSON), &workflow)
	_ = json.Unmarshal([]byte(artifact.SchemasJSON), &schemas)
	draft := ExtensionDraft{DraftVersion: "1.0.0", Metadata: DraftMetadata{ID: manifest.Metadata.ID, Name: manifest.Metadata.Name, Version: bumpPatchVersion(manifest.Metadata.Version), Description: manifest.Metadata.Description, Author: manifest.Metadata.Author, License: manifest.Metadata.License}, Intent: DraftIntent{Goal: "基于已安装 Skill 创建新版本", Triggers: manifest.Triggers}, InputSchema: schemas["input"], OutputSchema: schemas["output"], ConfigSchema: schemas["config"], DefaultConfig: schemas["defaults"], Workflow: workflow, TestCases: []WorkshopTestCase{}}
	session, err := s.CreateSession(ctx, CreateWorkshopSessionRequest{Scope: scope, Requirement: "基于 " + skillID + " 创建新的声明式 Skill", CharacterID: scope.CharacterID})
	if err != nil {
		return WorkshopSessionDetailView{}, err
	}
	if _, err := s.Generate(ctx, session.ID, GenerateWorkshopDraftRequest{Scope: scope, Draft: &draft}); err != nil {
		return WorkshopSessionDetailView{}, err
	}
	return s.GetSession(ctx, scope, session.ID)
}

func bumpPatchVersion(version string) string {
	parts := strings.SplitN(version, "+", 2)
	parts = strings.SplitN(parts[0], "-", 2)
	segments := strings.Split(parts[0], ".")
	if len(segments) != 3 {
		return "1.0.0"
	}
	patch, err := strconv.Atoi(segments[2])
	if err != nil {
		return "1.0.0"
	}
	return segments[0] + "." + segments[1] + "." + strconv.Itoa(patch+1)
}

func buildWorkshopManifest(draft ExtensionDraft, compiled CompiledWorkflow, artifactID string) Manifest {
	triggers := uniqueSortedTriggers(draft.Intent.Triggers)
	return Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: draft.Metadata.ID, Name: draft.Metadata.Name, Version: draft.Metadata.Version, Description: draft.Metadata.Description, Author: draft.Metadata.Author, License: draft.Metadata.License}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0", EngineMaxExclusive: "2.0.0"}, Entry: SkillEntry{Kind: "workflow", ArtifactID: artifactID}, Capabilities: append([]string{}, compiled.Capabilities...), Triggers: triggers, Execution: ManifestExecution{TimeoutMS: compiled.Limits.MaxExecutionDurationMS, HasSideEffects: compiled.HasSideEffects, Retryable: compiled.Idempotent && !compiled.HasSideEffects, Idempotent: compiled.Idempotent}, InputSchema: draft.InputSchema, OutputSchema: draft.OutputSchema, ConfigSchema: draft.ConfigSchema, DefaultConfig: draft.DefaultConfig, Enabled: false, AllowLLM: containsTrigger(triggers, TriggerLLM), AllowManual: containsTrigger(triggers, TriggerManual)}
}
func containsTrigger(values []SkillTrigger, expected SkillTrigger) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
func analyzeCapabilityDeclaration(declared []string, compiled CompiledWorkflow) CapabilityAnalysis {
	required := append([]string(nil), compiled.Capabilities...)
	sort.Strings(required)
	declared = uniqueSortedStrings(declared)
	analysis := CapabilityAnalysis{Required: required, Declared: declared, ByStep: map[string][]string{}}
	requiredSet := map[string]bool{}
	declaredSet := map[string]bool{}
	for _, value := range required {
		requiredSet[value] = true
		if definition, ok := Capability(value); ok && definition.Risk == "high" {
			analysis.HighRisk = append(analysis.HighRisk, value)
		}
	}
	for _, value := range declared {
		declaredSet[value] = true
		if !requiredSet[value] {
			analysis.Excess = append(analysis.Excess, value)
		}
	}
	for _, value := range required {
		if !declaredSet[value] {
			analysis.Missing = append(analysis.Missing, value)
		}
	}
	for _, step := range compiled.Steps {
		switch step.Type {
		case "http":
			analysis.ByStep[step.ID] = []string{"network.https"}
		case "schedule":
			analysis.ByStep[step.ID] = []string{"scheduler.own.manage"}
		case "notification":
			analysis.ByStep[step.ID] = []string{"notification.send"}
		case "memory_candidate":
			analysis.ByStep[step.ID] = []string{"memory.candidate.write"}
		case "context_contribution":
			analysis.ByStep[step.ID] = []string{"context.inject"}
		}
	}
	return analysis
}
func uniqueSortedStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	result := sortedKeys(set)
	return result
}
func sameStringSets(left, right []string) bool {
	return reflect.DeepEqual(uniqueSortedStrings(left), uniqueSortedStrings(right))
}
func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
func dependenciesFromCompiled(compiled CompiledWorkflow) []SkillDependency {
	result := make([]SkillDependency, len(compiled.Dependencies))
	for index, item := range compiled.Dependencies {
		result[index] = SkillDependency{SkillID: item.SkillID, Version: item.Version}
	}
	return result
}
func sideEffectNames(workflow WorkflowDefinition, compiled CompiledWorkflow) []string {
	if !compiled.HasSideEffects {
		return []string{}
	}
	set := map[string]bool{}
	for _, step := range workflow.Steps {
		switch step.Type {
		case "http":
			set["network_request"] = true
		case "schedule":
			set["schedule_create"] = true
		case "notification":
			set["notification_send"] = true
		case "memory_candidate":
			set["memory_candidate_write"] = true
		case "context_contribution":
			set["context_injection"] = true
		case "call_skill":
			set["skill_side_effect"] = true
		}
	}
	return sortedKeys(set)
}
func summarizeIssues(issues []AnalysisIssue) string {
	parts := []string{}
	for _, issue := range issues {
		if issue.Level == "error" {
			parts = append(parts, issue.Message)
		}
		if len(parts) >= 5 {
			break
		}
	}
	return strings.Join(parts, "; ")
}
func mustJSON(value interface{}) []byte { raw, _ := json.Marshal(value); return raw }
func evaluateAssertions(assertions []TestAssertion, result WorkflowExecutionResult, validator *SchemaValidator) []AssertionResult {
	output := map[string]interface{}{}
	_ = json.Unmarshal(result.Output, &output)
	values := map[string]interface{}{"output": output, "steps": map[string]interface{}{}}
	for _, step := range result.Steps {
		values["steps"].(map[string]interface{})[step.StepID] = map[string]interface{}{"status": step.Status}
	}
	reports := make([]AssertionResult, 0, len(assertions))
	for _, assertion := range assertions {
		report := AssertionResult{Type: assertion.Type}
		switch assertion.Type {
		case "equals", "not_equals", "exists", "not_exists", "contains":
			value, err := resolveReference(assertion.Path, values)
			if assertion.Type == "exists" {
				report.Passed = err == nil
			} else if assertion.Type == "not_exists" {
				report.Passed = err != nil
			} else if err == nil && assertion.Type == "equals" {
				report.Passed = reflect.DeepEqual(value, assertion.Expected)
			} else if err == nil && assertion.Type == "not_equals" {
				report.Passed = !reflect.DeepEqual(value, assertion.Expected)
			} else if err == nil && assertion.Type == "contains" {
				report.Passed = strings.Contains(fmt.Sprint(value), fmt.Sprint(assertion.Expected))
			}
		case "status_is":
			report.Passed = fmt.Sprint(assertion.Expected) == "succeeded"
		case "step_succeeded", "step_failed":
			for _, step := range result.Steps {
				if step.StepID == assertion.StepID {
					report.Passed = step.Status == strings.TrimPrefix(assertion.Type, "step_")
				}
			}
		case "side_effect_count":
			number, ok := asFloat(assertion.Expected)
			report.Passed = ok && len(result.SideEffects) == int(number)
		case "duration_less_than":
			var total int64
			for _, step := range result.Steps {
				total += step.DurationMS
			}
			number, ok := asFloat(assertion.Expected)
			report.Passed = ok && total < int64(number)
		case "matches_schema":
			schema, err := json.Marshal(assertion.Expected)
			report.Passed = err == nil && validator != nil && validator.Validate("workshop-assertion-schema", schema, result.Output) == nil
		default:
			report.Passed = false
		}
		if !report.Passed {
			report.Message = "断言未通过"
		}
		reports = append(reports, report)
	}
	return reports
}
