package extension

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware"
)

type AgentSkillHandler struct {
	service  *AgentSkillService
	problems *Handler
}

func NewAgentSkillHandler(service *AgentSkillService, problems *Handler) *AgentSkillHandler {
	return &AgentSkillHandler{service: service, problems: problems}
}
func (h *AgentSkillHandler) scope(c *gin.Context) ExecutionScope {
	trace, _ := c.Get(middleware.CtxKeyRequestID)
	return ExecutionScope{UserID: fmt.Sprint(c.GetInt(authenticatedUserKey)), CharacterID: c.Query("characterId"), ConversationID: c.Query("conversationId"), Channel: c.DefaultQuery("channel", "web"), TraceID: fmt.Sprint(trace), RequestID: fmt.Sprint(trace), Trigger: TriggerManual}
}

func (h *AgentSkillHandler) Preview(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 55<<20)
	if err := c.Request.ParseMultipartForm(55 << 20); err != nil {
		h.problems.problem(c, NewExtensionError(ErrAgentSkillArchiveLimit, "import request exceeds limit", err.Error(), false, err))
		return
	}
	userID := fmt.Sprint(c.GetInt(authenticatedUserKey))
	source := c.PostForm("source")
	if source == "directory" {
		headers := c.Request.MultipartForm.File["files"]
		var paths []string
		if err := json.Unmarshal([]byte(c.PostForm("paths")), &paths); err != nil || len(paths) != len(headers) {
			h.problems.problem(c, NewExtensionError(ErrAgentSkillInvalidArchive, "directory paths are invalid", "", false, err))
			return
		}
		files := map[string][]byte{}
		for index, header := range headers {
			file, err := header.Open()
			if err != nil {
				h.problems.problem(c, err)
				return
			}
			content, readErr := io.ReadAll(io.LimitReader(file, h.service.limits.MaxResourceBytes+1))
			file.Close()
			if readErr != nil {
				h.problems.problem(c, readErr)
				return
			}
			files[paths[index]] = content
		}
		preview, err := h.service.PreviewDirectory(c.Request.Context(), userID, c.PostForm("rootName"), files)
		if err != nil {
			h.problems.problem(c, err)
			return
		}
		success(c, preview)
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		h.problems.problem(c, NewExtensionError(ErrAgentSkillInvalidArchive, "ZIP file is required", "", false, err))
		return
	}
	file, err := header.Open()
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, 55<<20))
	file.Close()
	if readErr != nil {
		h.problems.problem(c, readErr)
		return
	}
	preview, err := h.service.PreviewZIP(c.Request.Context(), userID, raw)
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, preview)
}
func (h *AgentSkillHandler) Install(c *gin.Context) {
	var request struct {
		PreviewID   string          `json:"previewId"`
		Scope       AgentSkillScope `json:"scope"`
		CharacterID string          `json:"characterId"`
		Enable      bool            `json:"enable"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		h.problems.problem(c, NewExtensionError(ErrSkillInputInvalid, "invalid install request", err.Error(), false, err))
		return
	}
	definition, err := h.service.Install(c.Request.Context(), InstallAgentSkillRequest{UserID: fmt.Sprint(c.GetInt(authenticatedUserKey)), CharacterID: request.CharacterID, PreviewID: request.PreviewID, Scope: request.Scope, Enable: request.Enable})
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	c.JSON(http.StatusCreated, definition)
}
func (h *AgentSkillHandler) List(c *gin.Context) {
	scope := h.scope(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	result, err := h.service.List(c.Request.Context(), scope, AgentSkillFilter{Query: c.Query("query"), Status: AgentSkillCompatibilityStatus(c.Query("status")), Scope: AgentSkillScope(c.Query("scope")), Page: page, PageSize: pageSize})
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, result)
}
func (h *AgentSkillHandler) Get(c *gin.Context) {
	scope := h.scope(c)
	definition, report, err := h.service.Get(c.Request.Context(), scope, c.Param("id"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	activations, _ := h.service.repository.ListAgentSkillActivations(c.Request.Context(), definition.ExtensionID, scope.UserID, 20)
	success(c, map[string]interface{}{"definition": definition, "compatibilityReport": report, "activations": activations})
}
func (h *AgentSkillHandler) Enable(c *gin.Context) {
	if err := h.service.Enable(c.Request.Context(), h.scope(c), c.Param("id")); err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, map[string]bool{"enabled": true})
}
func (h *AgentSkillHandler) Disable(c *gin.Context) {
	if err := h.service.Disable(c.Request.Context(), h.scope(c), c.Param("id")); err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, map[string]bool{"enabled": false})
}
func (h *AgentSkillHandler) Remove(c *gin.Context) {
	if err := h.service.Remove(c.Request.Context(), h.scope(c), c.Param("id")); err != nil {
		h.problems.problem(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *AgentSkillHandler) Compatibility(c *gin.Context) {
	_, report, err := h.service.Get(c.Request.Context(), h.scope(c), c.Param("id"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, report)
}
func (h *AgentSkillHandler) Resources(c *gin.Context) {
	definition, _, err := h.service.Get(c.Request.Context(), h.scope(c), c.Param("id"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	kind := AgentSkillResourceKind(c.Query("kind"))
	result := []AgentSkillResource{}
	for _, resource := range definition.Resources {
		if kind == "" || resource.Kind == kind {
			resource.Executable = false
			result = append(result, resource)
		}
	}
	success(c, result)
}
func (h *AgentSkillHandler) ResourceContent(c *gin.Context) { h.serveResource(c, false) }
func (h *AgentSkillHandler) AssetContent(c *gin.Context)    { h.serveResource(c, true) }
func (h *AgentSkillHandler) serveResource(c *gin.Context, assetOnly bool) {
	definition, _, files, err := h.service.repository.LoadAgentSkill(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	if !h.service.visible(h.scope(c), definition) {
		h.problems.problem(c, NewExtensionError(ErrAgentSkillScopeForbidden, "Agent Skill is outside the current scope", "", false, nil))
		return
	}
	clean, err := validateAgentSkillRelativePath(c.Query("path"), h.service.limits)
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	var resource *AgentSkillResource
	for index := range definition.Resources {
		if definition.Resources[index].Path == clean {
			resource = &definition.Resources[index]
			break
		}
	}
	if resource == nil {
		h.problems.problem(c, NewExtensionError(ErrAgentSkillResourceNotFound, "Agent Skill resource not found", clean, false, nil))
		return
	}
	if assetOnly && resource.Kind != AgentSkillResourceAsset {
		h.problems.problem(c, NewExtensionError(ErrAgentSkillResourceDenied, "resource is not an asset", clean, false, nil))
		return
	}
	content := files[clean]
	if strings.Contains(strings.ToLower(resource.MIMEType), "svg") && unsafeSVG(content) {
		h.problems.problem(c, NewExtensionError(ErrAgentSkillResourceDenied, "unsafe SVG is blocked", clean, false, nil))
		return
	}
	if !assetOnly && !resource.TextReadable {
		h.problems.problem(c, NewExtensionError(ErrAgentSkillResourceDenied, "resource is not readable as text", clean, false, nil))
		return
	}
	mimeType := resource.MIMEType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Header("Content-Disposition", "attachment; filename=\""+strings.ReplaceAll(path.Base(clean), "\"", "")+"\"")
	c.Data(http.StatusOK, mimeType, content)
}
func (h *AgentSkillHandler) Activations(c *gin.Context) {
	definition, _, err := h.service.Get(c.Request.Context(), h.scope(c), c.Param("id"))
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	items, err := h.service.repository.ListAgentSkillActivations(c.Request.Context(), definition.ExtensionID, h.scope(c).UserID, 50)
	if err != nil {
		h.problems.problem(c, err)
		return
	}
	success(c, items)
}
func (h *AgentSkillHandler) Metrics(c *gin.Context) { success(c, agentSkillMetricsSnapshot()) }
func unsafeSVG(content []byte) bool {
	lower := strings.ToLower(string(content))
	for _, needle := range []string{"<script", "javascript:", "data:", "onload=", "onerror=", "http://", "https://", "xlink:href"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}
