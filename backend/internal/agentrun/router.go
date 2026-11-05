package agentrun

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *Service
	spaceID string
}

func RegisterRoutes(group *gin.RouterGroup, service *Service, spaceID string) {
	if service == nil {
		return
	}
	handler := &HTTPHandler{service: service, spaceID: spaceID}
	runs := group.Group("/roles/runs")
	runs.GET("", handler.list)
	runs.GET("/:id", handler.get)
	runs.GET("/:id/messages", handler.messages)
	runs.POST("/:id/messages", handler.send)
	runs.POST("/:id/messages/claim", handler.claim)
	runs.POST("/:id/messages/acknowledge", handler.acknowledge)
}

func (h *HTTPHandler) respond(c *gin.Context, data any, err error) {
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": data})
}

func (h *HTTPHandler) list(c *gin.Context) {
	conversationID := strings.TrimSpace(c.Query("conversationId"))
	if conversationID == "" {
		h.respond(c, nil, errors.New("conversationId is required"))
		return
	}
	items, err := h.service.List(c.Request.Context(), h.spaceID, conversationID)
	h.respond(c, gin.H{"runs": items}, err)
}

func (h *HTTPHandler) get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) messages(c *gin.Context) {
	items, err := h.service.Messages(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, gin.H{"messages": items}, err)
}

func (h *HTTPHandler) send(c *gin.Context) {
	var input struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "error": err})
		return
	}
	message, err := h.service.SendFromUser(c.Request.Context(), h.spaceID, c.Param("id"), input.Content)
	h.respond(c, message, err)
}

func (h *HTTPHandler) claim(c *gin.Context) {
	var input struct {
		Owner        string `json:"owner"`
		LeaseSeconds int    `json:"leaseSeconds"`
		Limit        int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "error": err})
		return
	}
	leaseDuration := time.Duration(input.LeaseSeconds) * time.Second
	claim, err := h.service.Claim(c.Request.Context(), h.spaceID, c.Param("id"), input.Owner, leaseDuration, input.Limit)
	h.respond(c, claim, err)
}

func (h *HTTPHandler) acknowledge(c *gin.Context) {
	var input struct {
		LeaseToken string   `json:"leaseToken"`
		MessageIDs []string `json:"messageIds"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "error": err})
		return
	}
	count, err := h.service.Acknowledge(c.Request.Context(), h.spaceID, c.Param("id"), input.LeaseToken, input.MessageIDs)
	h.respond(c, gin.H{"acknowledgedCount": count}, err)
}
