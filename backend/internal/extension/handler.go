package extension

import (
	"embed"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware"
)

//go:embed schema/openapi.json
var openAPIFS embed.FS

const authenticatedUserKey = "extension_authenticated_user_id"

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) OpenAPI(c *gin.Context) {
	raw, err := openAPIFS.ReadFile("schema/openapi.json")
	if err != nil {
		h.problem(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

func (h *Handler) baseScope(c *gin.Context) ExecutionScope {
	userID := fmt.Sprint(c.GetInt(authenticatedUserKey))
	traceID, _ := c.Get(middleware.CtxKeyRequestID)
	return ExecutionScope{UserID: userID, TraceID: fmt.Sprint(traceID), RequestID: fmt.Sprint(traceID)}
}

func (h *Handler) problem(c *gin.Context, err error) {
	extErr := asExtensionError(err)
	status := problemStatus(extErr.Code)
	traceID, _ := c.Get(middleware.CtxKeyRequestID)
	detail := extErr.Detail
	if detail == "" {
		detail = extErr.Message
	}
	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, ProblemDetail{
		Type:     "https://errors.amitia.dev/extensions/" + strings.ToLower(strings.ReplaceAll(extErr.Code, "_", "-")),
		Title:    extErr.Message,
		Status:   status,
		Detail:   detail,
		Instance: c.Request.URL.Path,
		Code:     extErr.Code,
		TraceID:  fmt.Sprint(traceID),
	})
}

func problemStatus(code string) int {
	switch {
	case strings.HasSuffix(code, "_NOT_FOUND"):
		return http.StatusNotFound
	case strings.Contains(code, "FORBIDDEN"), strings.Contains(code, "DENIED"), strings.Contains(code, "PATH_TRAVERSAL"):
		return http.StatusForbidden
	case strings.HasPrefix(code, "PACKAGE_"):
		switch code {
		case "PACKAGE_OPERATION_IN_PROGRESS", "PACKAGE_EXPORT_NOT_ALLOWED", "PACKAGE_ID_CONFLICT", "PACKAGE_NAME_CONFLICT", "PACKAGE_VERSION_CONFLICT", "PACKAGE_SAME_VERSION_DIFFERENT_CONTENT", "PACKAGE_DEPENDENCY_IN_USE":
			return http.StatusConflict
		case "PACKAGE_IMPORT_SESSION_EXPIRED", "PACKAGE_IMPORT_SESSION_CONSUMED":
			return http.StatusGone
		case "PACKAGE_ARCHIVE_LIMIT":
			return http.StatusRequestEntityTooLarge
		case "PACKAGE_HIGH_RISK_CONFIRMATION_REQUIRED", "PACKAGE_CONFIG_MIGRATION_REQUIRED":
			return http.StatusPreconditionRequired
		case "PACKAGE_REPOSITORY_UNAVAILABLE":
			return http.StatusServiceUnavailable
		default:
			return http.StatusUnprocessableEntity
		}
	case strings.HasPrefix(code, "AGENT_SKILL_"), strings.HasPrefix(code, "SKILL_"), strings.HasPrefix(code, "WORKSHOP_"), strings.HasPrefix(code, "WORKFLOW_"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功", "data": data})
}
