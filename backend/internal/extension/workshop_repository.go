package extension

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type workshopSessionRecord struct {
	ID                             string `gorm:"column:id;primaryKey"`
	UserID                         string `gorm:"column:user_id"`
	CharacterID                    string `gorm:"column:character_id"`
	Status                         string `gorm:"column:status"`
	Requirement                    string `gorm:"column:requirement"`
	CurrentRevision                int64  `gorm:"column:current_revision"`
	CurrentDraftID                 string `gorm:"column:current_draft_id"`
	ValidationSummary              string `gorm:"column:validation_summary"`
	RiskSummary                    string `gorm:"column:risk_summary"`
	TestSummary                    string `gorm:"column:test_summary"`
	InstalledSkillID               string `gorm:"column:installed_skill_id"`
	InstalledVersion               string `gorm:"column:installed_version"`
	PermissionConfirmationJSON     string `gorm:"column:permission_confirmation_json"`
	PermissionRevision             int64  `gorm:"column:permission_revision"`
	PermissionChecksum             string `gorm:"column:permission_checksum"`
	TestPermissionConfirmationJSON string `gorm:"column:test_permission_confirmation_json"`
	TestPermissionRevision         int64  `gorm:"column:test_permission_revision"`
	TestPermissionChecksum         string `gorm:"column:test_permission_checksum"`
	LockVersion                    int64  `gorm:"column:lock_version"`
	CreatedAt                      string `gorm:"column:created_at"`
	UpdatedAt                      string `gorm:"column:updated_at"`
	ArchivedAt                     string `gorm:"column:archived_at"`
}

func (workshopSessionRecord) TableName() string { return "extension_workshop_sessions" }

type workshopRevisionRecord struct {
	ID                     string `gorm:"column:id;primaryKey"`
	SessionID              string `gorm:"column:session_id"`
	Revision               int64  `gorm:"column:revision"`
	RawModelOutput         string `gorm:"column:raw_model_output"`
	PlanJSON               string `gorm:"column:plan_json"`
	RawDraftJSON           string `gorm:"column:raw_draft_json"`
	NormalizedDraftJSON    string `gorm:"column:normalized_draft_json"`
	ManifestJSON           string `gorm:"column:manifest_json"`
	InputSchemaJSON        string `gorm:"column:input_schema_json"`
	OutputSchemaJSON       string `gorm:"column:output_schema_json"`
	ConfigSchemaJSON       string `gorm:"column:config_schema_json"`
	WorkflowJSON           string `gorm:"column:workflow_json"`
	CompiledWorkflowJSON   string `gorm:"column:compiled_workflow_json"`
	CapabilityAnalysisJSON string `gorm:"column:capability_analysis_json"`
	RiskAnalysisJSON       string `gorm:"column:risk_analysis_json"`
	ValidationResultJSON   string `gorm:"column:validation_result_json"`
	WorkflowChecksum       string `gorm:"column:workflow_checksum"`
	ModelProvider          string `gorm:"column:model_provider"`
	ModelName              string `gorm:"column:model_name"`
	ModelInputSummaryJSON  string `gorm:"column:model_input_summary_json"`
	ModelOutputSummaryJSON string `gorm:"column:model_output_summary_json"`
	CreatedAt              string `gorm:"column:created_at"`
}

func (workshopRevisionRecord) TableName() string { return "extension_workshop_revisions" }

type workshopTestRunRecord struct {
	ID                   string `gorm:"column:id;primaryKey"`
	TestRunID            string `gorm:"column:test_run_id"`
	UserID               string `gorm:"column:user_id"`
	CharacterID          string `gorm:"column:character_id"`
	SessionID            string `gorm:"column:session_id"`
	Revision             int64  `gorm:"column:revision"`
	WorkflowChecksum     string `gorm:"column:workflow_checksum"`
	Mode                 string `gorm:"column:mode"`
	Status               string `gorm:"column:status"`
	InputSummary         string `gorm:"column:input_summary"`
	OutputSummary        string `gorm:"column:output_summary"`
	StepResultsJSON      string `gorm:"column:step_results_json"`
	AssertionResultsJSON string `gorm:"column:assertion_results_json"`
	SideEffectsJSON      string `gorm:"column:side_effects_json"`
	CapabilitiesJSON     string `gorm:"column:capabilities_json"`
	WarningsJSON         string `gorm:"column:warnings_json"`
	ErrorCode            string `gorm:"column:error_code"`
	ErrorDetail          string `gorm:"column:error_detail"`
	TraceID              string `gorm:"column:trace_id"`
	StartedAt            string `gorm:"column:started_at"`
	FinishedAt           string `gorm:"column:finished_at"`
	DurationMS           int64  `gorm:"column:duration_ms"`
	CreatedAt            string `gorm:"column:created_at"`
}

func (workshopTestRunRecord) TableName() string { return "extension_workshop_test_runs" }

type extensionArtifactRecord struct {
	ID                   string `gorm:"column:id;primaryKey"`
	ArtifactID           string `gorm:"column:artifact_id"`
	ExtensionID          string `gorm:"column:extension_id"`
	ExtensionVersion     string `gorm:"column:extension_version"`
	Source               string `gorm:"column:source"`
	SessionID            string `gorm:"column:session_id"`
	Revision             int64  `gorm:"column:revision"`
	ManifestJSON         string `gorm:"column:manifest_json"`
	WorkflowJSON         string `gorm:"column:workflow_json"`
	SchemasJSON          string `gorm:"column:schemas_json"`
	CompiledWorkflowJSON string `gorm:"column:compiled_workflow_json"`
	TestsJSON            string `gorm:"column:tests_json"`
	ReadmeText           string `gorm:"column:readme_text"`
	Checksum             string `gorm:"column:checksum"`
	SizeBytes            int64  `gorm:"column:size_bytes"`
	CreatedAt            string `gorm:"column:created_at"`
	ArchivedAt           string `gorm:"column:archived_at"`
}

func (extensionArtifactRecord) TableName() string { return "extension_artifacts" }

type WorkshopRepository struct{ db *gorm.DB }

func NewWorkshopRepository(db *gorm.DB) *WorkshopRepository { return &WorkshopRepository{db: db} }

func insertWorkshopAudit(tx *gorm.DB, session workshopSessionRecord, scope ExecutionScope, from, to WorkshopSessionStatus, operation string, revision int64, checksum, model, errorCode string, durationMS int64) error {
	detail, _ := json.Marshal(map[string]interface{}{"sessionId": session.ID, "revision": revision, "userId": session.UserID, "characterId": session.CharacterID, "operation": operation, "fromStatus": from, "toStatus": to, "status": to, "durationMs": durationMS, "model": model, "checksum": checksum, "errorCode": errorCode})
	scopeType, scopeID := string(ScopeGlobal), session.UserID
	if session.CharacterID != "" {
		scopeType, scopeID = string(ScopeCharacter), session.CharacterID
	}
	return tx.Create(&pluginAuditRecord{ID: uuid.NewString(), ExtensionID: "workshop:" + session.ID, Action: "workshop.state.transition", ScopeType: scopeType, ScopeID: scopeID, DetailJSON: compactSensitiveJSON(detail), TraceID: scope.TraceID, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}).Error
}

func (r *WorkshopRepository) CreateSession(ctx context.Context, scope ExecutionScope, requirement, characterID string) (WorkshopSession, error) {
	now := time.Now().UTC()
	record := workshopSessionRecord{ID: uuid.New().String(), UserID: scope.UserID, CharacterID: characterID, Status: string(WorkshopDraft), Requirement: requirement, ValidationSummary: "{}", RiskSummary: "{}", TestSummary: "{}", PermissionConfirmationJSON: "{}", TestPermissionConfirmationJSON: "{}", LockVersion: 1, CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano)}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		return insertWorkshopAudit(tx, record, scope, "", WorkshopDraft, "session.create", 0, "", "", "", 0)
	}); err != nil {
		return WorkshopSession{}, err
	}
	return sessionFromRecord(record), nil
}

func (r *WorkshopRepository) GetSession(ctx context.Context, scope ExecutionScope, id string) (WorkshopSession, workshopSessionRecord, error) {
	var record workshopSessionRecord
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WorkshopSession{}, record, NewExtensionError(ErrWorkshopSessionNotFound, "工坊会话不存在", id, false, nil)
	}
	if err != nil {
		return WorkshopSession{}, record, err
	}
	if record.UserID != scope.UserID || (record.CharacterID != "" && scope.CharacterID != "" && record.CharacterID != scope.CharacterID) {
		return WorkshopSession{}, record, NewExtensionError(ErrWorkshopSessionForbidden, "无权访问该工坊会话", id, false, nil)
	}
	return sessionFromRecord(record), record, nil
}

func (r *WorkshopRepository) ListSessions(ctx context.Context, scope ExecutionScope, filter WorkshopSessionFilter) (PagedWorkshopSessions, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := r.db.WithContext(ctx).Model(&workshopSessionRecord{}).Where("user_id = ?", scope.UserID)
	if filter.CharacterID != "" {
		query = query.Where("character_id = ?", filter.CharacterID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return PagedWorkshopSessions{}, err
	}
	var records []workshopSessionRecord
	if err := query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return PagedWorkshopSessions{}, err
	}
	items := make([]WorkshopSession, len(records))
	for index, record := range records {
		items[index] = sessionFromRecord(record)
	}
	return PagedWorkshopSessions{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *WorkshopRepository) CASStatus(ctx context.Context, scope ExecutionScope, id string, version int64, from []WorkshopSessionStatus, to WorkshopSessionStatus, operation string, updates map[string]interface{}) error {
	allowed := make([]string, len(from))
	for index, value := range from {
		if !validWorkshopTransition(value, to) {
			return NewExtensionError(ErrWorkshopInvalidState, "非法工坊状态转换", string(value)+" -> "+string(to), false, nil)
		}
		allowed[index] = string(value)
	}
	updates["status"] = string(to)
	updates["lock_version"] = gorm.Expr("lock_version + 1")
	updates["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session workshopSessionRecord
		if err := tx.Where("id = ?", id).First(&session).Error; err != nil {
			return err
		}
		result := tx.Model(&workshopSessionRecord{}).Where("id = ? AND lock_version = ? AND status IN ?", id, version, allowed).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return NewExtensionError(ErrWorkshopRevisionConflict, "工坊会话已被其他操作修改", id, true, nil)
		}
		return insertWorkshopAudit(tx, session, scope, WorkshopSessionStatus(session.Status), to, operation, session.CurrentRevision, "", "", "", 0)
	})
}

func validWorkshopTransition(from, to WorkshopSessionStatus) bool {
	allowed := map[WorkshopSessionStatus]map[WorkshopSessionStatus]bool{
		WorkshopDraft:               {WorkshopGenerating: true, WorkshopGenerated: true, WorkshopArchived: true, WorkshopError: true},
		WorkshopGenerating:          {WorkshopGenerated: true, WorkshopError: true},
		WorkshopGenerated:           {WorkshopGenerating: true, WorkshopValidating: true, WorkshopValidated: true, WorkshopValidationFailed: true, WorkshopArchived: true, WorkshopError: true},
		WorkshopValidating:          {WorkshopValidated: true, WorkshopValidationFailed: true, WorkshopError: true},
		WorkshopValidationFailed:    {WorkshopGenerating: true, WorkshopValidating: true, WorkshopValidated: true, WorkshopValidationFailed: true, WorkshopArchived: true},
		WorkshopValidated:           {WorkshopGenerating: true, WorkshopValidating: true, WorkshopAwaitingPermissions: true, WorkshopArchived: true},
		WorkshopAwaitingPermissions: {WorkshopGenerating: true, WorkshopValidating: true, WorkshopTesting: true, WorkshopTestPassed: true, WorkshopTestFailed: true, WorkshopArchived: true},
		WorkshopTesting:             {WorkshopTestPassed: true, WorkshopTestFailed: true, WorkshopError: true},
		WorkshopTestFailed:          {WorkshopGenerating: true, WorkshopValidating: true, WorkshopTesting: true, WorkshopTestPassed: true, WorkshopTestFailed: true, WorkshopArchived: true},
		WorkshopTestPassed:          {WorkshopGenerating: true, WorkshopValidating: true, WorkshopTesting: true, WorkshopAwaitingPermissions: true, WorkshopInstalling: true, WorkshopInstalled: true, WorkshopArchived: true},
		WorkshopInstalling:          {WorkshopInstalled: true, WorkshopTestPassed: true, WorkshopError: true},
		WorkshopInstalled:           {WorkshopEnabled: true, WorkshopDisabled: true, WorkshopArchived: true},
		WorkshopEnabled:             {WorkshopDisabled: true, WorkshopArchived: true},
		WorkshopDisabled:            {WorkshopEnabled: true, WorkshopArchived: true},
		WorkshopError:               {WorkshopGenerating: true, WorkshopValidating: true, WorkshopArchived: true},
	}
	return allowed[from][to]
}

func (r *WorkshopRepository) SaveRevision(ctx context.Context, scope ExecutionScope, session workshopSessionRecord, requirement string, plan WorkshopPlan, rawModel string, raw, normalized ExtensionDraft, compiled CompiledWorkflow, analysis CapabilityAnalysis, modelProvider, modelName string) (WorkshopRevisionView, error) {
	revision := session.CurrentRevision + 1
	id := uuid.New().String()
	rawDraft, _ := json.Marshal(raw)
	planRaw, _ := json.Marshal(plan)
	normalizedDraft, _ := json.Marshal(normalized)
	manifest, _ := json.Marshal(normalized.Manifest)
	workflow, _ := json.Marshal(normalized.Workflow)
	compiledRaw, _ := json.Marshal(compiled)
	analysisRaw, _ := json.Marshal(analysis)
	inputSummary, _ := json.Marshal(workshopContentSummary([]byte(requirement)))
	outputSummary, _ := json.Marshal(map[string]interface{}{"plan": workshopContentSummary(planRaw), "draft": workshopContentSummary([]byte(rawModel))})
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := workshopRevisionRecord{ID: id, SessionID: session.ID, Revision: revision, RawModelOutput: string(redactJSON(json.RawMessage(rawModel))), PlanJSON: string(planRaw), RawDraftJSON: string(rawDraft), NormalizedDraftJSON: string(normalizedDraft), ManifestJSON: string(manifest), InputSchemaJSON: string(normalized.InputSchema), OutputSchemaJSON: string(normalized.OutputSchema), ConfigSchemaJSON: string(normalizeJSON(normalized.ConfigSchema)), WorkflowJSON: string(workflow), CompiledWorkflowJSON: string(compiledRaw), CapabilityAnalysisJSON: string(analysisRaw), RiskAnalysisJSON: "{}", ValidationResultJSON: "{}", WorkflowChecksum: compiled.Checksum, ModelProvider: modelProvider, ModelName: modelName, ModelInputSummaryJSON: string(inputSummary), ModelOutputSummaryJSON: string(outputSummary), CreatedAt: now}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		result := tx.Model(&workshopSessionRecord{}).Where("id = ? AND lock_version = ? AND current_revision = ?", session.ID, session.LockVersion, session.CurrentRevision).Updates(map[string]interface{}{"status": string(WorkshopGenerated), "current_revision": revision, "current_draft_id": id, "permission_confirmation_json": "{}", "permission_revision": 0, "permission_checksum": "", "test_permission_confirmation_json": "{}", "test_permission_revision": 0, "test_permission_checksum": "", "validation_summary": "{}", "test_summary": "{}", "lock_version": gorm.Expr("lock_version + 1"), "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return NewExtensionError(ErrWorkshopRevisionConflict, "并发生成冲突", session.ID, true, nil)
		}
		return insertWorkshopAudit(tx, session, scope, WorkshopGenerating, WorkshopGenerated, "revision.generated", revision, compiled.Checksum, strings.Trim(strings.Join([]string{modelProvider, modelName}, "/"), "/"), "", 0)
	})
	if err != nil {
		return WorkshopRevisionView{}, err
	}
	return revisionFromRecord(record), nil
}

func workshopContentSummary(raw []byte) map[string]interface{} {
	hash := sha256.Sum256(raw)
	return map[string]interface{}{"sha256": hex.EncodeToString(hash[:]), "bytes": len(raw)}
}

func (r *WorkshopRepository) GetRevision(ctx context.Context, scope ExecutionScope, sessionID string, revision int64) (WorkshopRevisionView, workshopRevisionRecord, error) {
	if _, _, err := r.GetSession(ctx, scope, sessionID); err != nil {
		return WorkshopRevisionView{}, workshopRevisionRecord{}, err
	}
	var record workshopRevisionRecord
	err := r.db.WithContext(ctx).Where("session_id = ? AND revision = ?", sessionID, revision).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WorkshopRevisionView{}, record, NewExtensionError(ErrWorkshopRevisionNotFound, "工坊修订不存在", fmt.Sprintf("%s/%d", sessionID, revision), false, nil)
	}
	if err != nil {
		return WorkshopRevisionView{}, record, err
	}
	return revisionFromRecord(record), record, nil
}
func (r *WorkshopRepository) ListRevisions(ctx context.Context, scope ExecutionScope, sessionID string) ([]WorkshopRevisionView, error) {
	if _, _, err := r.GetSession(ctx, scope, sessionID); err != nil {
		return nil, err
	}
	var records []workshopRevisionRecord
	if err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("revision DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]WorkshopRevisionView, len(records))
	for index, record := range records {
		result[index] = revisionFromRecord(record)
	}
	return result, nil
}

func (r *WorkshopRepository) SaveValidation(ctx context.Context, scope ExecutionScope, session workshopSessionRecord, revision int64, result WorkshopValidationResult) error {
	raw, _ := json.Marshal(result)
	status := WorkshopValidated
	if !result.Valid {
		status = WorkshopValidationFailed
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&workshopRevisionRecord{}).Where("session_id = ? AND revision = ?", session.ID, revision).Update("validation_result_json", string(raw)).Error; err != nil {
			return err
		}
		update := tx.Model(&workshopSessionRecord{}).Where("id = ? AND lock_version = ? AND current_revision = ?", session.ID, session.LockVersion, revision).Updates(map[string]interface{}{"status": string(status), "validation_summary": string(raw), "risk_summary": string(raw), "lock_version": gorm.Expr("lock_version + 1"), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return NewExtensionError(ErrWorkshopRevisionConflict, "校验结果已过期", session.ID, true, nil)
		}
		return insertWorkshopAudit(tx, session, scope, WorkshopValidating, status, "revision.validated", revision, result.WorkflowChecksum, "", "", 0)
	})
}

func (r *WorkshopRepository) SaveConfirmation(ctx context.Context, scope ExecutionScope, session workshopSessionRecord, revision int64, checksum string, confirmation PermissionConfirmation) error {
	raw, _ := json.Marshal(confirmation)
	updates := map[string]interface{}{"lock_version": gorm.Expr("lock_version + 1"), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}
	if confirmation.Production {
		updates["permission_confirmation_json"] = string(raw)
		updates["permission_revision"] = revision
		updates["permission_checksum"] = checksum
	} else {
		updates["status"] = string(WorkshopAwaitingPermissions)
		updates["test_permission_confirmation_json"] = string(raw)
		updates["test_permission_revision"] = revision
		updates["test_permission_checksum"] = checksum
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&workshopSessionRecord{}).Where("id = ? AND lock_version = ? AND current_revision = ?", session.ID, session.LockVersion, revision).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return NewExtensionError(ErrWorkshopRevisionConflict, "权限确认已过期", session.ID, true, nil)
		}
		to := WorkshopSessionStatus(session.Status)
		operation := "permission.production.confirmed"
		if !confirmation.Production {
			to = WorkshopAwaitingPermissions
			operation = "permission.test.confirmed"
		}
		return insertWorkshopAudit(tx, session, scope, WorkshopSessionStatus(session.Status), to, operation, revision, checksum, "", "", 0)
	})
}

func (r *WorkshopRepository) SaveTestReport(ctx context.Context, scope ExecutionScope, report WorkshopTestReport, input json.RawMessage) error {
	report = redactWorkshopTestReport(report)
	steps, _ := json.Marshal(report.StepResults)
	assertions, _ := json.Marshal(report.Assertions)
	effects, _ := json.Marshal(report.SideEffects)
	capabilities, _ := json.Marshal(report.Capabilities)
	warnings, _ := json.Marshal(report.Warnings)
	errorCode, errorDetail := "", ""
	if report.Error != nil {
		errorCode, errorDetail = report.Error.Code, report.Error.Detail
	}
	record := workshopTestRunRecord{ID: uuid.New().String(), TestRunID: report.TestRunID, UserID: scope.UserID, CharacterID: scope.CharacterID, SessionID: report.SessionID, Revision: report.Revision, WorkflowChecksum: report.WorkflowChecksum, Mode: string(reportStatusMode(report)), Status: report.Status, InputSummary: compactSensitiveJSON(input), OutputSummary: compactSensitiveJSON(report.Output), StepResultsJSON: string(steps), AssertionResultsJSON: string(assertions), SideEffectsJSON: string(effects), CapabilitiesJSON: string(capabilities), WarningsJSON: string(warnings), ErrorCode: errorCode, ErrorDetail: errorDetail, TraceID: scope.TraceID, StartedAt: report.StartedAt.Format(time.RFC3339Nano), FinishedAt: report.FinishedAt.Format(time.RFC3339Nano), DurationMS: report.DurationMS, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	summary, _ := json.Marshal(report)
	status := WorkshopTestPassed
	if report.Status != "passed" {
		status = WorkshopTestFailed
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		update := tx.Model(&workshopSessionRecord{}).Where("id = ? AND current_revision = ?", report.SessionID, report.Revision).Updates(map[string]interface{}{"status": string(status), "test_summary": string(summary), "lock_version": gorm.Expr("lock_version + 1"), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)})
		if update.Error != nil {
			return update.Error
		}
		var session workshopSessionRecord
		if err := tx.Where("id = ?", report.SessionID).First(&session).Error; err != nil {
			return err
		}
		return insertWorkshopAudit(tx, session, scope, WorkshopTesting, status, "revision.tested", report.Revision, report.WorkflowChecksum, "", errorCode, report.DurationMS)
	})
}

func redactWorkshopTestReport(report WorkshopTestReport) WorkshopTestReport {
	report.Output = redactJSON(report.Output)
	if report.Error != nil {
		cloned := *report.Error
		cloned.Detail = redactWorkshopErrorDetail(cloned.Detail)
		report.Error = &cloned
	}
	for index := range report.StepResults {
		if report.StepResults[index].Error != nil {
			cloned := *report.StepResults[index].Error
			cloned.Detail = redactWorkshopErrorDetail(cloned.Detail)
			report.StepResults[index].Error = &cloned
		}
	}
	return report
}

func redactWorkshopErrorDetail(value string) string {
	if secretPattern.MatchString(value) {
		return "[REDACTED]"
	}
	if len(value) > 512 {
		return value[:512]
	}
	return value
}

func (r *WorkshopRepository) ListTestReports(ctx context.Context, scope ExecutionScope, sessionID string) ([]WorkshopTestReport, error) {
	if _, _, err := r.GetSession(ctx, scope, sessionID); err != nil {
		return nil, err
	}
	var records []workshopTestRunRecord
	if err := r.db.WithContext(ctx).Where("session_id = ? AND user_id = ?", sessionID, scope.UserID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]WorkshopTestReport, len(records))
	for index, record := range records {
		result[index] = testReportFromRecord(record)
	}
	return result, nil
}
func (r *WorkshopRepository) GetTestReport(ctx context.Context, scope ExecutionScope, testRunID string) (WorkshopTestReport, error) {
	var record workshopTestRunRecord
	err := r.db.WithContext(ctx).Where("test_run_id = ? AND user_id = ?", testRunID, scope.UserID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WorkshopTestReport{}, NewExtensionError(ErrWorkshopTestRequired, "测试报告不存在", testRunID, false, nil)
	}
	if err != nil {
		return WorkshopTestReport{}, err
	}
	if record.CharacterID != "" && scope.CharacterID != "" && record.CharacterID != scope.CharacterID {
		return WorkshopTestReport{}, NewExtensionError(ErrWorkshopSessionForbidden, "无权访问测试报告", testRunID, false, nil)
	}
	return testReportFromRecord(record), nil
}
func (r *WorkshopRepository) LatestPassedTest(ctx context.Context, sessionID string, revision int64, checksum string) (*WorkshopTestReport, error) {
	var record workshopTestRunRecord
	err := r.db.WithContext(ctx).Where("session_id = ? AND revision = ? AND workflow_checksum = ? AND status = 'passed'", sessionID, revision, checksum).Order("created_at DESC").First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	report := testReportFromRecord(record)
	return &report, nil
}
func (r *WorkshopRepository) GetArtifact(ctx context.Context, artifactID string) (extensionArtifactRecord, error) {
	var record extensionArtifactRecord
	err := r.db.WithContext(ctx).Where("artifact_id = ?", artifactID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, NewExtensionError(ErrWorkshopArtifactInvalid, "制品不存在", artifactID, false, nil)
	}
	return record, err
}
func (r *WorkshopRepository) GetSessionArtifact(ctx context.Context, scope ExecutionScope, sessionID string) (extensionArtifactRecord, error) {
	if _, _, err := r.GetSession(ctx, scope, sessionID); err != nil {
		return extensionArtifactRecord{}, err
	}
	var record extensionArtifactRecord
	err := r.db.WithContext(ctx).Where("session_id = ? AND archived_at = ''", sessionID).Order("created_at DESC").First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, NewExtensionError(ErrWorkshopArtifactInvalid, "当前会话还没有已安装制品", sessionID, false, nil)
	}
	return record, err
}
func (r *WorkshopRepository) CurrentArtifacts(ctx context.Context) ([]extensionArtifactRecord, error) {
	var records []extensionArtifactRecord
	err := r.db.WithContext(ctx).Table("extension_artifacts AS a").Joins("JOIN extensions AS e ON e.extension_id = a.extension_id AND e.current_version = a.extension_version").Where("a.archived_at = '' AND a.source = 'workshop'").Find(&records).Error
	return records, err
}

func sessionFromRecord(record workshopSessionRecord) WorkshopSession {
	created, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
	updated, _ := time.Parse(time.RFC3339Nano, record.UpdatedAt)
	var archived *time.Time
	if record.ArchivedAt != "" {
		value, _ := time.Parse(time.RFC3339Nano, record.ArchivedAt)
		archived = &value
	}
	return WorkshopSession{ID: record.ID, UserID: record.UserID, CharacterID: record.CharacterID, Status: WorkshopSessionStatus(record.Status), Requirement: record.Requirement, CurrentRevision: record.CurrentRevision, CurrentDraftID: record.CurrentDraftID, ValidationSummary: json.RawMessage(record.ValidationSummary), RiskSummary: json.RawMessage(record.RiskSummary), TestSummary: json.RawMessage(record.TestSummary), InstalledSkillID: record.InstalledSkillID, InstalledVersion: record.InstalledVersion, TestPermissionConfirmed: record.TestPermissionRevision == record.CurrentRevision && record.TestPermissionChecksum != "", ProductionPermissionConfirmed: record.PermissionRevision == record.CurrentRevision && record.PermissionChecksum != "", CreatedAt: created, UpdatedAt: updated, ArchivedAt: archived}
}
func revisionFromRecord(record workshopRevisionRecord) WorkshopRevisionView {
	var draft, normalized ExtensionDraft
	var plan WorkshopPlan
	_ = json.Unmarshal([]byte(record.PlanJSON), &plan)
	var validation WorkshopValidationResult
	_ = json.Unmarshal([]byte(record.RawDraftJSON), &draft)
	_ = json.Unmarshal([]byte(record.NormalizedDraftJSON), &normalized)
	var validationPtr *WorkshopValidationResult
	if record.ValidationResultJSON != "" && record.ValidationResultJSON != "{}" && json.Unmarshal([]byte(record.ValidationResultJSON), &validation) == nil {
		validationPtr = &validation
	}
	created, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
	return WorkshopRevisionView{ID: record.ID, SessionID: record.SessionID, Revision: record.Revision, Plan: plan, Draft: draft, NormalizedDraft: normalized, WorkflowChecksum: record.WorkflowChecksum, Validation: validationPtr, ModelProvider: record.ModelProvider, ModelName: record.ModelName, CreatedAt: created}
}
func testReportFromRecord(record workshopTestRunRecord) WorkshopTestReport {
	report := WorkshopTestReport{TestRunID: record.TestRunID, SessionID: record.SessionID, Revision: record.Revision, WorkflowChecksum: record.WorkflowChecksum, Status: record.Status, DurationMS: record.DurationMS, Output: json.RawMessage(record.OutputSummary)}
	report.StartedAt, _ = time.Parse(time.RFC3339Nano, record.StartedAt)
	report.FinishedAt, _ = time.Parse(time.RFC3339Nano, record.FinishedAt)
	_ = json.Unmarshal([]byte(record.StepResultsJSON), &report.StepResults)
	_ = json.Unmarshal([]byte(record.AssertionResultsJSON), &report.Assertions)
	_ = json.Unmarshal([]byte(record.SideEffectsJSON), &report.SideEffects)
	_ = json.Unmarshal([]byte(record.CapabilitiesJSON), &report.Capabilities)
	_ = json.Unmarshal([]byte(record.WarningsJSON), &report.Warnings)
	if record.ErrorCode != "" {
		report.Error = NewExtensionError(record.ErrorCode, "测试失败", record.ErrorDetail, false, nil)
	}
	return report
}
func reportStatusMode(report WorkshopTestReport) WorkflowExecutionMode {
	for _, warning := range report.Warnings {
		if strings.HasPrefix(warning.Code, "mode:") {
			return WorkflowExecutionMode(strings.TrimPrefix(warning.Code, "mode:"))
		}
	}
	return WorkflowDryRun
}
