package workbenchterminal

import (
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/workbenchhttp"
	"github.com/u-ai/backend/internal/workspace"
	amitialog "log"
)

var rootPattern = regexp.MustCompile(`^amitia://workspace/@([a-zA-Z0-9_-]{1,128})/$`)
var conversationPattern = regexp.MustCompile(`^[a-zA-Z0-9_:.-]{1,160}$`)

type Handler struct {
	manager      *Manager
	workspace    *workspace.Service
	tickets      ticketRegistry
	taskVerifier TaskVerifier
}

func NewHandler(service *workspace.Service, verifiers ...TaskVerifier) *Handler {
	h := &Handler{manager: NewManager(), workspace: service}
	if journal, err := defaultTaskReceiptJournal(); err == nil {
		h.manager.EnableTaskReceipts(journal)
		if err := journal.Prune(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	if len(verifiers) > 0 {
		h.taskVerifier = verifiers[0]
		if h.taskVerifier != nil {
			go h.watchTaskBindings()
		}
	}
	return h
}
func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	r := group.Group("/workbench/terminal")
	r.Use(func(c *gin.Context) {
		workbenchhttp.ProtectResponse(c.Writer.Header())
		if !workbenchhttp.LimitRequest(c.Writer, c.Request, workbenchhttp.TerminalCommandLimit) {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "终端请求超过大小上限"})
			return
		}
		c.Next()
	})
	r.POST("/open", h.open)
	r.GET("/sessions", h.listSessions)
	r.GET("/task-receipts", h.taskReceipts)
	r.GET("/:id", h.read)
	r.GET("/:id/export", h.export)
	r.GET("/:id/lifecycle", h.lifecycle)
	r.POST("/:id/input", h.input)
	r.POST("/:id/resize", h.resize)
	r.DELETE("/:id", h.close)
	r.POST("/:id/stream-ticket", h.streamTicket)
}
func terminalOwner(c *gin.Context, conversationID string) (string, bool) {
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil || actor.PrincipalType != auth.PrincipalLocalUI || !actor.IsLocalTrusted || actor.SpaceID == "" || actor.DeviceID == "" || !conversationPattern.MatchString(conversationID) {
		return "", false
	}
	return terminalOwnerKey(string(actor.SpaceID), string(actor.DeviceID), string(actor.RuntimeID), actor.SessionID, conversationID), true
}
func (h *Handler) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrReceiptUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, ErrClosed):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, ErrTaskBinding):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrLimit), errors.Is(err, ErrExited):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	}
}
func (h *Handler) open(c *gin.Context) {
	var request struct {
		ConversationID string `json:"conversationId"`
		WorkspaceURI   string `json:"workspaceUri"`
		TaskID         string `json:"taskId"`
		Rows           uint16 `json:"rows"`
		Cols           uint16 `json:"cols"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	if c.ShouldBindJSON(&request) != nil {
		h.respondError(c, ErrInvalid)
		return
	}
	owner, ok := terminalOwner(c, request.ConversationID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅可信本机用户可使用工作区终端"})
		return
	}
	matches := rootPattern.FindStringSubmatch(request.WorkspaceURI)
	if matches == nil || h.workspace == nil {
		h.respondError(c, ErrInvalid)
		return
	}
	cwd, err := h.workspace.LocalWorkspaceRoot(matches[1])
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if request.TaskID != "" {
		if !taskIDPattern.MatchString(request.TaskID) || h.taskVerifier == nil || h.taskVerifier(c.Request.Context(), request.WorkspaceURI, request.TaskID, request.ConversationID) != nil {
			h.respondError(c, ErrTaskBinding)
			return
		}
	}
	result, err := h.manager.OpenTask(owner, cwd, request.WorkspaceURI, request.TaskID, request.Rows, request.Cols, request.ConversationID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}
func (h *Handler) read(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	c.Header("X-Content-Type-Options", "nosniff")
	owner, ok := terminalOwner(c, c.Query("conversationId"))
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问终端"})
		return
	}
	cursor, err := strconv.ParseInt(strings.TrimSpace(c.DefaultQuery("cursor", "0")), 10, 64)
	if err != nil {
		h.respondError(c, ErrInvalid)
		return
	}
	if err := h.validateTask(c.Request.Context(), owner, c.Param("id"), c.Query("conversationId")); err != nil {
		h.respondError(c, err)
		return
	}
	result, err := h.manager.Read(owner, c.Param("id"), cursor)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *Handler) input(c *gin.Context) {
	var request struct {
		ConversationID string `json:"conversationId"`
		Data           string `json:"data"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	if c.ShouldBindJSON(&request) != nil {
		h.respondError(c, ErrInvalid)
		return
	}
	owner, ok := terminalOwner(c, request.ConversationID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权写入终端"})
		return
	}
	if len(request.Data) > 6000 {
		h.respondError(c, ErrInvalid)
		return
	}
	data, err := base64.StdEncoding.DecodeString(request.Data)
	if err != nil {
		h.respondError(c, ErrInvalid)
		return
	}
	if err = h.validateTask(c.Request.Context(), owner, c.Param("id"), request.ConversationID); err != nil {
		h.respondError(c, err)
		return
	}
	if err = h.manager.Input(owner, c.Param("id"), data); err != nil {
		h.respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) resize(c *gin.Context) {
	var request struct {
		ConversationID string `json:"conversationId"`
		Rows           uint16 `json:"rows"`
		Cols           uint16 `json:"cols"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	if c.ShouldBindJSON(&request) != nil {
		h.respondError(c, ErrInvalid)
		return
	}
	owner, ok := terminalOwner(c, request.ConversationID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权调整终端"})
		return
	}
	if err := h.validateTask(c.Request.Context(), owner, c.Param("id"), request.ConversationID); err != nil {
		h.respondError(c, err)
		return
	}
	if err := h.manager.Resize(owner, c.Param("id"), request.Rows, request.Cols); err != nil {
		h.respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) close(c *gin.Context) {
	owner, ok := terminalOwner(c, c.Query("conversationId"))
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权关闭终端"})
		return
	}
	if err := h.manager.CloseSession(owner, c.Param("id")); err != nil {
		h.respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
