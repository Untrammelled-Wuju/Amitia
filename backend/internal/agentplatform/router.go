package agentplatform

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *Service
	spaceID string
	ownerID string
}

func RegisterRoutes(group *gin.RouterGroup, service *Service, spaceID, ownerID string) {
	handler := &HTTPHandler{service: service, spaceID: spaceID, ownerID: ownerID}
	roles := group.Group("/roles")
	roles.Use(handler.localActor)
	roles.GET("", handler.listAgents)

	teams := roles.Group("/teams")
	teams.GET("/templates", handler.listTeamTemplates)
	teams.POST("/from-template", handler.createTeamFromTemplate)
	teams.GET("", handler.listTeams)
	teams.POST("", handler.createTeam)
	teams.GET("/:id", handler.getTeam)
	teams.PUT("/:id", handler.updateTeam)
	teams.POST("/:id/members", handler.addTeamMember)
	teams.POST("/:id/contexts", handler.createTeamContext)
	teams.GET("/:id/context", handler.sharedTeamContext)
	teams.POST("/:id/handoffs", handler.createTeamHandoff)
	teams.POST("/:id/route", handler.routeTeamMessage)
	teams.POST("/:id/handoffs/:handoffId/accept", handler.acceptTeamHandoff)
	teams.POST("/:id/handoffs/:handoffId/complete", handler.completeTeamHandoff)
}

func (h *HTTPHandler) localActor(c *gin.Context) {
	c.Request = c.Request.WithContext(WithPlatformActor(c.Request.Context(), h.ownerID, "local_owner"))
	c.Next()
}

func (h *HTTPHandler) listAgents(c *gin.Context) {
	items, err := h.service.ListAgents(c.Request.Context(), h.spaceID, queryBool(c, "includeDisabled"), queryInt(c, "limit"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) listTeams(c *gin.Context) {
	items, err := h.service.ListTeams(c.Request.Context(), h.spaceID)
	h.respond(c, items, err)
}

func (h *HTTPHandler) createTeam(c *gin.Context) {
	var input CreateTeamInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if input.OwnerID == "" {
		input.OwnerID = h.actorID(c)
	}
	item, err := h.service.CreateTeam(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) listTeamTemplates(c *gin.Context) {
	items, err := h.service.ListTeamTemplates(c.Request.Context())
	h.respond(c, items, err)
}

func (h *HTTPHandler) createTeamFromTemplate(c *gin.Context) {
	var input CreateTeamFromTemplateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if input.OwnerID == "" {
		input.OwnerID = h.actorID(c)
	}
	item, err := h.service.CreateTeamFromTemplate(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) getTeam(c *gin.Context) {
	item, err := h.service.GetTeam(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) updateTeam(c *gin.Context) {
	var input UpdateTeamInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.UpdateTeam(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) addTeamMember(c *gin.Context) {
	var input struct {
		MemberType    string   `json:"memberType"`
		MemberID      string   `json:"memberId"`
		Role          string   `json:"role"`
		ResponderMode string   `json:"responderMode"`
		Capabilities  []string `json:"capabilities"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.AddTeamMember(c.Request.Context(), h.spaceID, c.Param("id"), input.MemberType, input.MemberID, input.Role, input.ResponderMode, input.Capabilities)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) createTeamContext(c *gin.Context) {
	var input CreateTeamContextInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if input.CreatedBy == "" {
		input.CreatedBy = h.actorID(c)
	}
	item, err := h.service.CreateTeamContext(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) sharedTeamContext(c *gin.Context) {
	value, err := h.service.SharedTeamContextJSON(c.Request.Context(), h.spaceID, c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": value})
}

func (h *HTTPHandler) createTeamHandoff(c *gin.Context) {
	var input CreateHandoffInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CreateTeamHandoff(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) routeTeamMessage(c *gin.Context) {
	var input RouteTeamMessageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.RouteTeamMessage(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) acceptTeamHandoff(c *gin.Context) {
	item, err := h.service.AcceptTeamHandoff(c.Request.Context(), h.spaceID, c.Param("id"), c.Param("handoffId"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) completeTeamHandoff(c *gin.Context) {
	item, err := h.service.CompleteTeamHandoff(c.Request.Context(), h.spaceID, c.Param("id"), c.Param("handoffId"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) actorID(c *gin.Context) string {
	return h.ownerID
}

func (h *HTTPHandler) respond(c *gin.Context, value any, err error) {
	h.respondStatus(c, value, err, http.StatusOK)
}

func (h *HTTPHandler) respondStatus(c *gin.Context, value any, err error, status int) {
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(status, gin.H{"code": status, "msg": "ok", "data": value})
}

func (h *HTTPHandler) respondError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	message := err.Error()
	if errors.Is(err, context.Canceled) {
		status = 499
	}
	if strings.Contains(message, "不存在") || strings.Contains(message, "not found") {
		status = http.StatusNotFound
	}
	if strings.Contains(message, "unavailable") || strings.Contains(message, "数据库") || strings.Contains(message, "读取") {
		status = http.StatusInternalServerError
	}
	c.JSON(status, gin.H{"code": status, "msg": message, "error": message})
}

func queryInt(c *gin.Context, key string) int {
	value, _ := strconv.Atoi(c.Query(key))
	return value
}

func queryBool(c *gin.Context, key string) bool {
	value, _ := strconv.ParseBool(c.Query(key))
	return value
}
