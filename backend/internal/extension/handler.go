package extension

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware"
)

//go:embed schema/openapi.json
var openAPIFS embed.FS

const authenticatedUserKey = "extension_authenticated_user_id"

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListSkills(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	filter := SkillFilter{}
	if value := c.Query("enabled"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid enabled filter", value, false, err))
			return
		}
		filter.Enabled = &enabled
	}
	filter.Trigger = SkillTrigger(c.Query("trigger"))
	filter.Source = SkillSource(c.Query("source"))
	if filter.Trigger != "" && !validTrigger(filter.Trigger) {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid trigger filter", string(filter.Trigger), false, nil))
		return
	}
	if filter.Source != "" && filter.Source != SkillSourceBuiltin && filter.Source != SkillSourceLegacy && filter.Source != SkillSourceWorkflow && filter.Source != SkillSourceInstructions {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid source filter", string(filter.Source), false, nil))
		return
	}
	items, err := h.service.ListSkills(c.Request.Context(), scope, filter)
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, items)
}

func (h *Handler) GetSkill(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	item, err := h.service.GetSkill(c.Request.Context(), scope, c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, item)
}

func (h *Handler) EnableSkill(c *gin.Context) {
	if err := h.service.EnableSkill(c.Request.Context(), h.baseScope(c), c.Param("id")); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) DisableSkill(c *gin.Context) {
	if err := h.service.DisableSkill(c.Request.Context(), h.baseScope(c), c.Param("id")); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) GetPermissions(c *gin.Context) {
	items, err := h.service.GetSkillPermissions(c.Request.Context(), h.baseScope(c), c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, items)
}

func (h *Handler) UpdatePermissions(c *gin.Context) {
	var body struct {
		Grants         []PermissionGrantInput `json:"grants"`
		CharacterID    string                 `json:"characterId"`
		ConversationID string                 `json:"conversationId"`
		Channel        string                 `json:"channel"`
		SessionID      string                 `json:"sessionId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid permission request", err.Error(), false, err))
		return
	}
	scope := h.baseScope(c)
	scope.CharacterID = strings.TrimSpace(body.CharacterID)
	scope.ConversationID = strings.TrimSpace(body.ConversationID)
	scope.Channel = strings.TrimSpace(body.Channel)
	scope.SessionID = strings.TrimSpace(body.SessionID)
	if err := h.service.UpdateSkillPermissions(c.Request.Context(), scope, c.Param("id"), body.Grants); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) GetConfig(c *gin.Context) {
	config, err := h.service.GetSkillConfig(c.Request.Context(), h.baseScope(c), c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	var value interface{}
	if json.Unmarshal(config, &value) != nil {
		value = map[string]interface{}{}
	}
	success(c, value)
}

func (h *Handler) UpdateConfig(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid config request", err.Error(), false, err))
		return
	}
	if err := h.service.UpdateSkillConfig(c.Request.Context(), h.baseScope(c), c.Param("id"), raw); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) ResetConfig(c *gin.Context) {
	if err := h.service.ResetSkillConfig(c.Request.Context(), h.baseScope(c), c.Param("id")); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) Execute(c *gin.Context) {
	var body struct {
		Input          json.RawMessage `json:"input"`
		CharacterID    string          `json:"characterId"`
		ConversationID string          `json:"conversationId"`
		Channel        string          `json:"channel"`
		SessionID      string          `json:"sessionId"`
		IdempotencyKey string          `json:"idempotencyKey"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid execution request", err.Error(), false, err))
		return
	}
	if strings.TrimSpace(body.CharacterID) == "" {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "characterId is required", "", false, nil))
		return
	}
	scope := h.baseScope(c)
	scope.CharacterID = strings.TrimSpace(body.CharacterID)
	scope.ConversationID = strings.TrimSpace(body.ConversationID)
	scope.Channel = strings.TrimSpace(body.Channel)
	scope.SessionID = strings.TrimSpace(body.SessionID)
	scope.Trigger = TriggerManual
	result, err := h.service.ExecuteSkill(c.Request.Context(), ExecuteSkillRequest{SkillID: c.Param("id"), Input: body.Input, Scope: scope, IdempotencyKey: body.IdempotencyKey})
	if err != nil {
		h.problemWithResult(c, err, result)
		return
	}
	success(c, result)
}

func (h *Handler) ListRuns(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	filter := RunFilter{SkillID: c.Query("skillId"), Status: RunStatus(c.Query("status")), CharacterID: scope.CharacterID, Channel: c.Query("channel"), Trigger: SkillTrigger(c.Query("trigger")), Page: page, PageSize: pageSize}
	if filter.Status != "" && !validRunStatus(filter.Status) {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid status filter", string(filter.Status), false, nil))
		return
	}
	if filter.Trigger != "" && !validTrigger(filter.Trigger) {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid trigger filter", string(filter.Trigger), false, nil))
		return
	}
	var timeErr error
	filter.From, timeErr = normalizeFilterTime(c.Query("from"))
	if timeErr == nil {
		filter.To, timeErr = normalizeFilterTime(c.Query("to"))
	}
	if timeErr != nil {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid time filter", timeErr.Error(), false, timeErr))
		return
	}
	items, err := h.service.ListRuns(c.Request.Context(), scope, filter)
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, items)
}

func (h *Handler) GetRun(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	item, err := h.service.GetRun(c.Request.Context(), scope, c.Param("runId"))
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, item)
}

func (h *Handler) Capabilities(c *gin.Context) {
	success(c, Capabilities())
}

func (h *Handler) OpenAPI(c *gin.Context) {
	raw, err := openAPIFS.ReadFile("schema/openapi.json")
	if err != nil {
		h.problem(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

func (h *Handler) queryScope(c *gin.Context, requireCharacter bool) (ExecutionScope, bool) {
	scope := h.baseScope(c)
	scope.CharacterID = strings.TrimSpace(c.Query("characterId"))
	scope.ConversationID = strings.TrimSpace(c.Query("conversationId"))
	scope.Channel = strings.TrimSpace(c.Query("channel"))
	scope.SessionID = strings.TrimSpace(c.Query("sessionId"))
	if requireCharacter && scope.CharacterID == "" {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "characterId is required", "", false, nil))
		return ExecutionScope{}, false
	}
	return scope, true
}

func (h *Handler) baseScope(c *gin.Context) ExecutionScope {
	userID := fmt.Sprint(c.GetInt(authenticatedUserKey))
	traceID, _ := c.Get(middleware.CtxKeyRequestID)
	return ExecutionScope{UserID: userID, TraceID: fmt.Sprint(traceID), RequestID: fmt.Sprint(traceID)}
}

func (h *Handler) problem(c *gin.Context, err error) {
	h.problemWithResult(c, err, SkillResult{})
}

func (h *Handler) problemWithResult(c *gin.Context, err error, result SkillResult) {
	extErr := asExtensionError(err)
	status := problemStatus(extErr.Code)
	traceID, _ := c.Get(middleware.CtxKeyRequestID)
	detail := extErr.Detail
	if detail == "" {
		detail = extErr.Message
	}
	problem := ProblemDetail{Type: "https://errors.amitia.dev/extensions/" + strings.ToLower(strings.ReplaceAll(extErr.Code, "_", "-")), Title: extErr.Message, Status: status, Detail: detail, Instance: c.Request.URL.Path, Code: extErr.Code, TraceID: fmt.Sprint(traceID)}
	if result.RunID != "" {
		problem.Result = &result
	}
	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, problem)
}

func problemStatus(code string) int {
	if strings.HasPrefix(code, "PACKAGE_") {
		switch code {
		case ErrPackageOperationInProgress:
			return http.StatusConflict
		case ErrPackageImportSessionExpired, ErrPackageImportSessionConsumed:
			return http.StatusGone
		case ErrPackageExportNotAllowed, ErrPackageIDConflict, ErrPackageNameConflict, ErrPackageVersionConflict, ErrPackageSameVersionDifferentContent, ErrPackageDependencyInUse:
			return http.StatusConflict
		case ErrPackageArchiveLimit:
			return http.StatusRequestEntityTooLarge
		case ErrPackageHighRiskConfirmationRequired, ErrPackageConfigMigrationRequired:
			return http.StatusPreconditionRequired
		default:
			return http.StatusUnprocessableEntity
		}
	}
	switch code {
	case ErrSkillNotFound, ErrAgentSkillNotFound, ErrAgentSkillResourceNotFound, ErrPluginNotFound, ErrWorkshopSessionNotFound, ErrWorkshopRevisionNotFound:
		return http.StatusNotFound
	case ErrSkillDisabled, ErrSkillIncompatible, ErrSkillTriggerNotAllowed, ErrSkillInputInvalid, ErrSkillManifestInvalid, ErrSkillDuplicateID, ErrSkillIdempotencyConflict, ErrSkillNotExecutable, ErrAgentSkillDisabled, ErrAgentSkillBlocked, ErrAgentSkillNotExecutable, ErrAgentSkillInvalidArchive, ErrAgentSkillArchiveLimit, ErrAgentSkillMissingSkillMD, ErrAgentSkillFrontmatter, ErrAgentSkillNameInvalid, ErrAgentSkillNameMismatch, ErrAgentSkillDescription, ErrAgentSkillNameConflict, ErrAgentSkillResourceTooLarge, ErrAgentSkillScriptDisabled, ErrAgentSkillToolUnsupported, ErrAgentSkillActivationLimit, ErrAgentSkillPromptLimit, ErrAgentSkillArtifactInvalid, ErrAgentSkillChecksumMismatch, ErrPluginDisabled, ErrPluginIncompatible, ErrPluginManifestInvalid, ErrPluginStateInvalid, ErrPluginStateConflict, ErrPluginConfigInvalid, ErrPluginEventInvalid, ErrPluginEventDepthExceeded, ErrPluginEventDeadLetter, ErrPluginScheduleInvalid, ErrPluginSurfaceInvalid, ErrPluginActionNotAllowed, ErrWorkshopInvalidState, ErrWorkshopGenerationOutputInvalid, ErrWorkshopManifestInvalid, ErrWorkshopWorkflowInvalid, ErrWorkshopSchemaInvalid, ErrWorkshopStaticAnalysisFailed, ErrWorkshopCapabilityMismatch, ErrWorkshopPermissionRequired, ErrWorkshopPermissionStale, ErrWorkshopSecretDetected, ErrWorkshopNetworkDenied, ErrWorkshopDependencyNotFound, ErrWorkshopDependencyCycle, ErrWorkshopTestRequired, ErrWorkshopTestFailed, ErrWorkshopTestStale, ErrWorkshopSandboxLimit, ErrWorkshopSkillIDConflict, ErrWorkshopVersionConflict, ErrWorkshopArtifactInvalid, ErrWorkshopChecksumMismatch, ErrWorkflowStepInvalid, ErrWorkflowReferenceInvalid, ErrWorkflowOutputInvalid:
		return http.StatusBadRequest
	case ErrSkillPermissionDenied, ErrAgentSkillScopeForbidden, ErrAgentSkillResourceDenied, ErrAgentSkillPathTraversal, ErrWorkshopSessionForbidden:
		return http.StatusForbidden
	case ErrSkillTimeout, ErrPluginHookTimeout, ErrWorkflowStepTimeout:
		return http.StatusGatewayTimeout
	case ErrWorkshopRevisionConflict:
		return http.StatusConflict
	case ErrSkillCancelled:
		return 499
	default:
		return http.StatusInternalServerError
	}
}

func validTrigger(trigger SkillTrigger) bool {
	switch trigger {
	case TriggerLLM, TriggerManual, TriggerSchedule, TriggerSystemEvent:
		return true
	default:
		return false
	}
}

func validRunStatus(status RunStatus) bool {
	switch status {
	case RunPending, RunRunning, RunSucceeded, RunFailed, RunCancelled, RunTimedOut, RunPartiallySucceeded:
		return true
	default:
		return false
	}
}

func normalizeFilterTime(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", err
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功", "data": data})
}
