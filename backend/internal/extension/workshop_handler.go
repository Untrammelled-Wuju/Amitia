package extension

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
)

type WorkshopHandler struct {
	service *WorkshopService
	base    *Handler
}

func NewWorkshopHandler(service *WorkshopService, base *Handler) *WorkshopHandler {
	return &WorkshopHandler{service: service, base: base}
}
func (h *WorkshopHandler) scope(c *gin.Context) ExecutionScope {
	scope := h.base.baseScope(c)
	scope.CharacterID = c.Query("characterId")
	scope.ConversationID = c.Query("conversationId")
	scope.Channel = c.Query("channel")
	scope.SessionID = c.Query("sessionId")
	return scope
}
func (h *WorkshopHandler) revision(c *gin.Context) (int64, bool) {
	value, err := strconv.ParseInt(c.Param("revision"), 10, 64)
	if err != nil || value < 1 {
		h.base.problem(c, NewExtensionError(ErrWorkshopRevisionNotFound, "修订号无效", c.Param("revision"), false, err))
		return 0, false
	}
	return value, true
}
func (h *WorkshopHandler) ListSessions(c *gin.Context) {
	scope := h.scope(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	result, err := h.service.ListSessions(c.Request.Context(), scope, WorkshopSessionFilter{Status: WorkshopSessionStatus(c.Query("status")), CharacterID: c.Query("characterId"), Page: page, PageSize: pageSize})
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) Metrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"counters": WorkshopMetricsSnapshot()})
}
func (h *WorkshopHandler) CreateSession(c *gin.Context) {
	var request CreateWorkshopSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.base.problem(c, NewExtensionError(ErrWorkshopGenerationOutputInvalid, "请求无效", err.Error(), false, err))
		return
	}
	request.Scope = h.scope(c)
	result, err := h.service.CreateSession(c.Request.Context(), request)
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}
func (h *WorkshopHandler) GetSession(c *gin.Context) {
	result, err := h.service.GetSession(c.Request.Context(), h.scope(c), c.Param("id"))
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) Archive(c *gin.Context) {
	if err := h.service.Archive(c.Request.Context(), h.scope(c), c.Param("id")); err != nil {
		h.base.problem(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *WorkshopHandler) ListRevisions(c *gin.Context) {
	result, err := h.service.ListRevisions(c.Request.Context(), h.scope(c), c.Param("id"))
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) GetRevision(c *gin.Context) {
	revision, ok := h.revision(c)
	if !ok {
		return
	}
	result, err := h.service.GetRevision(c.Request.Context(), h.scope(c), c.Param("id"), revision)
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) Generate(c *gin.Context) {
	var request GenerateWorkshopDraftRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			h.base.problem(c, NewExtensionError(ErrWorkshopGenerationOutputInvalid, "请求无效", err.Error(), false, err))
			return
		}
	}
	request.Scope = h.scope(c)
	result, err := h.service.Generate(c.Request.Context(), c.Param("id"), request)
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) Validate(c *gin.Context) {
	revision, ok := h.revision(c)
	if !ok {
		return
	}
	result, err := h.service.Validate(c.Request.Context(), h.scope(c), c.Param("id"), revision)
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) ConfirmPermissions(c *gin.Context) {
	revision, ok := h.revision(c)
	if !ok {
		return
	}
	var request PermissionConfirmation
	if err := c.ShouldBindJSON(&request); err != nil {
		h.base.problem(c, NewExtensionError(ErrWorkshopPermissionRequired, "权限确认请求无效", err.Error(), false, err))
		return
	}
	if err := h.service.ConfirmPermissions(c.Request.Context(), h.scope(c), c.Param("id"), revision, request); err != nil {
		h.base.problem(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *WorkshopHandler) Test(c *gin.Context) {
	revision, ok := h.revision(c)
	if !ok {
		return
	}
	var request WorkshopTestRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.base.problem(c, NewExtensionError(ErrWorkshopTestFailed, "测试请求无效", err.Error(), false, err))
		return
	}
	request.Scope = h.scope(c)
	result, err := h.service.Test(c.Request.Context(), h.scope(c), c.Param("id"), revision, request)
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) Install(c *gin.Context) {
	revision, ok := h.revision(c)
	if !ok {
		return
	}
	result, err := h.service.Install(c.Request.Context(), h.scope(c), c.Param("id"), revision)
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) ListTests(c *gin.Context) {
	result, err := h.service.ListTests(c.Request.Context(), h.scope(c), c.Param("id"))
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}
func (h *WorkshopHandler) GetTest(c *gin.Context) {
	result, err := h.service.GetTest(c.Request.Context(), h.scope(c), c.Param("testRunId"))
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) Rollback(c *gin.Context) {
	result, err := h.service.Rollback(c.Request.Context(), h.scope(c), c.Param("id"), c.Param("version"))
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) GetArtifact(c *gin.Context) {
	result, err := h.service.GetArtifact(c.Request.Context(), h.scope(c), c.Param("id"))
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *WorkshopHandler) Export(c *gin.Context) {
	artifact, err := h.service.GetArtifact(c.Request.Context(), h.scope(c), c.Param("id"))
	if err != nil {
		h.base.problem(c, err)
		return
	}
	var schemas map[string]json.RawMessage
	_ = json.Unmarshal(artifact.Schemas, &schemas)
	files := map[string][]byte{"manifest.json": artifact.Manifest, "schemas/input.schema.json": schemas["input"], "schemas/output.schema.json": schemas["output"], "schemas/config.schema.json": schemas["config"], "config/defaults.json": schemas["defaults"], "workflows/main.json": artifact.Workflow, "tests/cases.json": artifact.Tests, "README.md": []byte(artifact.Readme)}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var checksums bytes.Buffer
	for _, name := range names {
		sum := sha256.Sum256(files[name])
		checksums.WriteString(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	}
	files["checksums.sha256"] = checksums.Bytes()
	names = append(names, "checksums.sha256")
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range names {
		content := files[name]
		entry, createErr := writer.Create(name)
		if createErr != nil {
			h.base.problem(c, createErr)
			return
		}
		if _, writeErr := entry.Write(content); writeErr != nil {
			h.base.problem(c, writeErr)
			return
		}
	}
	if err := writer.Close(); err != nil {
		h.base.problem(c, err)
		return
	}
	c.Header("Content-Type", "application/vnd.amitia.extension+zip")
	c.Header("Content-Disposition", `attachment; filename="`+artifact.ExtensionID+`-`+artifact.ExtensionVersion+`.amitiax"`)
	c.Data(http.StatusOK, "application/vnd.amitia.extension+zip", buffer.Bytes())
}
func (h *WorkshopHandler) Fork(c *gin.Context) {
	result, err := h.service.ForkSkill(c.Request.Context(), h.scope(c), c.Param("id"))
	if err != nil {
		h.base.problem(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}
