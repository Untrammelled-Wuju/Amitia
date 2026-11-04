package agentplatform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	auth := group.Group("/agent-platform/auth")
	auth.POST("/login", handler.loginPlatformUser)
	auth.POST("/logout", handler.logoutPlatformUser)

	roles := group.Group("/roles")
	roles.Use(handler.platformActor)
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

	evaluations := roles.Group("/evaluations")
	evaluations.GET("/suites", handler.listEvaluationSuites)
	evaluations.POST("/suites", handler.createEvaluationSuite)
	evaluations.GET("/suites/:id/cases", handler.listEvaluationCases)
	evaluations.POST("/suites/:id/cases", handler.createEvaluationCase)
	evaluations.POST("/suites/:id/run", handler.runEvaluation)
	evaluations.GET("/runs", handler.listEvaluationRuns)

	activity := roles.Group("/activity")
	activity.GET("", handler.listActivity)
	activity.GET("/:id", handler.getActivity)

	a2a := roles.Group("/a2a")
	a2a.GET("/cards", handler.listA2ACards)
	a2a.POST("/cards", handler.registerA2ACard)
	a2a.GET("/tasks", handler.listA2ATasks)
	a2a.POST("/tasks", handler.submitA2ATask)
	a2a.POST("/tasks/:id/cancel", handler.cancelA2ATask)
}

func RegisterPublicRoutes(group *gin.RouterGroup, service *Service, spaceID string) {
	handler := &HTTPHandler{service: service, spaceID: spaceID}
	group.GET("/a2a/agents/:id", handler.getPublicA2ACard)
	group.POST("/a2a/tasks", handler.handleInboundA2ATask)
}

func (h *HTTPHandler) platformActor(c *gin.Context) {
	token := strings.TrimSpace(c.GetHeader("X-Amitia-Platform-Session"))
	if token == "" {
		c.Request = c.Request.WithContext(WithPlatformActor(c.Request.Context(), h.ownerID, "local_owner"))
		c.Next()
		return
	}
	user, _, err := h.service.ResolvePlatformSession(c.Request.Context(), token)
	if err != nil {
		h.respondError(c, err)
		c.Abort()
		return
	}
	permission := "platform.read"
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
		permission = "platform.write"
	}
	if err := h.service.Authorize(c.Request.Context(), h.spaceID, user.ID, permission); err != nil {
		h.respondError(c, err)
		c.Abort()
		return
	}
	c.Set("platformUserID", user.ID)
	c.Request = c.Request.WithContext(WithPlatformActor(c.Request.Context(), user.ID, "platform_user"))
	c.Next()
}

func (h *HTTPHandler) loginPlatformUser(c *gin.Context) {
	var input PlatformLoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.LoginPlatformUser(c.Request.Context(), h.spaceID, input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) logoutPlatformUser(c *gin.Context) {
	token := strings.TrimSpace(c.GetHeader("X-Amitia-Platform-Session"))
	err := h.service.LogoutPlatformSession(c.Request.Context(), token)
	h.respond(c, gin.H{"loggedOut": err == nil}, err)
}

func (h *HTTPHandler) bootstrap(c *gin.Context) {
	templates := []map[string]any{
		{"id": "companion", "name": "陪伴 Agent", "archetype": "companion", "category": "agent-identities"},
		{"id": "worker", "name": "执行 Agent", "archetype": "worker", "category": "productivity"},
		{"id": "team", "name": "团队负责人", "archetype": "team_lead", "category": "productivity"},
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{
		"spaceId": h.spaceID, "userId": h.ownerID, "templates": templates,
		"categories": []any{}, "runtimeProviders": []string{"local", "isolated", "oci", "remote", "android"},
		"roles": []string{"owner", "admin", "operator", "member", "viewer"},
	}})
}

func (h *HTTPHandler) overview(c *gin.Context) {
	ctx := c.Request.Context()
	events, summary, err := h.service.ListActivity(ctx, h.spaceID, ActivityFilter{Limit: 20})
	if err != nil {
		h.respondError(c, err)
		return
	}
	var agents []AgentProfile
	agents, err = h.service.ListAgents(ctx, h.spaceID, true, 20)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{
		"agents": agents, "activity": events, "summary": summary,
	}})
}

func (h *HTTPHandler) listAgents(c *gin.Context) {
	items, err := h.service.ListAgents(c.Request.Context(), h.spaceID, queryBool(c, "includeDisabled"), queryInt(c, "limit"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) listTemplates(c *gin.Context) {
	var items []AgentTemplate
	err := h.service.db.WithContext(c.Request.Context()).Order("builtin DESC, name ASC").Find(&items).Error
	h.respond(c, items, err)
}

func (h *HTTPHandler) listUsers(c *gin.Context) {
	items, err := h.service.ListUsers(c.Request.Context(), h.spaceID)
	h.respond(c, items, err)
}

func (h *HTTPHandler) createUser(c *gin.Context) {
	var input CreateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CreateUser(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) listMemberships(c *gin.Context) {
	items, err := h.service.ListMemberships(c.Request.Context(), h.spaceID, c.Query("userId"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) inviteUser(c *gin.Context) {
	var input InviteUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.InviteUser(c.Request.Context(), h.spaceID, h.actorID(c), input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) acceptInvitation(c *gin.Context) {
	var input struct {
		Token       string `json:"token"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.AcceptInvitation(c.Request.Context(), h.spaceID, input.Token, input.DisplayName, input.Password)
	h.respond(c, item, err)
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

func (h *HTTPHandler) listEvaluationSuites(c *gin.Context) {
	items, err := h.service.ListEvaluationSuites(c.Request.Context(), h.spaceID, c.Query("agentId"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) createEvaluationSuite(c *gin.Context) {
	var input CreateEvaluationSuiteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CreateEvaluationSuite(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) listEvaluationCases(c *gin.Context) {
	items, err := h.service.ListEvaluationCases(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) createEvaluationCase(c *gin.Context) {
	var input CreateEvaluationCaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CreateEvaluationCase(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) runEvaluation(c *gin.Context) {
	var input struct {
		AgentID string `json:"agentId"`
		RunEvaluationInput
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.RunEvaluation(c.Request.Context(), h.spaceID, input.AgentID, c.Param("id"), input.RunEvaluationInput)
	h.respond(c, item, err)
}

func (h *HTTPHandler) listEvaluationRuns(c *gin.Context) {
	items, err := h.service.ListEvaluationRuns(c.Request.Context(), h.spaceID, c.Query("agentId"), queryInt(c, "limit"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) listActivity(c *gin.Context) {
	items, summary, err := h.service.ListActivity(c.Request.Context(), h.spaceID, ActivityFilter{
		AgentID: c.Query("agentId"), TeamID: c.Query("teamId"), ConversationID: c.Query("conversationId"),
		Kind: c.Query("kind"), State: c.Query("state"), Limit: queryInt(c, "limit"),
	})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{"items": items, "summary": summary}})
}

func (h *HTTPHandler) getActivity(c *gin.Context) {
	item, err := h.service.GetActivity(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) listA2ACards(c *gin.Context) {
	items, err := h.service.ListA2ACards(c.Request.Context(), h.spaceID)
	h.respond(c, items, err)
}

func (h *HTTPHandler) registerA2ACard(c *gin.Context) {
	var input RegisterA2ACardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.RegisterA2ACard(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) listA2ATasks(c *gin.Context) {
	items, err := h.service.ListA2ATasks(c.Request.Context(), h.spaceID, c.Query("agentId"), queryInt(c, "limit"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) submitA2ATask(c *gin.Context) {
	var input A2ATaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.SubmitA2ATask(c.Request.Context(), h.spaceID, input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) cancelA2ATask(c *gin.Context) {
	var input struct {
		RemoteCancelURL string `json:"remoteCancelUrl"`
	}
	if err := bindOptionalJSON(c, &input); err != nil {
		h.respondError(c, err)
		return
	}
	item, err := h.service.CancelA2ATask(c.Request.Context(), h.spaceID, c.Param("id"), input.RemoteCancelURL)
	h.respond(c, item, err)
}

func (h *HTTPHandler) getPublicA2ACard(c *gin.Context) {
	item, err := h.service.GetPublicA2ACard(c.Request.Context(), h.spaceID, c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"name": item.Name, "description": item.Description, "url": item.URL,
		"version": item.Version, "skills": json.RawMessage(item.SkillsJSON),
		"security": json.RawMessage(item.SecurityJSON),
	})
}

func (h *HTTPHandler) handleInboundA2ATask(c *gin.Context) {
	var input InboundA2ATaskInput
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 8<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if err := json.Unmarshal(body, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	input.RawBody = body
	input.SenderAgentID = strings.TrimSpace(c.GetHeader("X-Amitia-A2A-Agent"))
	input.Timestamp = strings.TrimSpace(c.GetHeader("X-Amitia-A2A-Timestamp"))
	input.Nonce = strings.TrimSpace(c.GetHeader("X-Amitia-A2A-Nonce"))
	input.Signature = strings.TrimSpace(c.GetHeader("X-Amitia-A2A-Signature"))
	ctx := WithPlatformActor(c.Request.Context(), input.SenderAgentID, "remote_agent")
	item, err := h.service.HandleInboundA2ATask(ctx, h.spaceID, input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) actorID(c *gin.Context) string {
	if value := strings.TrimSpace(c.GetHeader("X-Amitia-User-ID")); value != "" {
		return value
	}
	if value := strings.TrimSpace(c.GetString("platformUserID")); value != "" {
		return value
	}
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

func bindOptionalJSON(c *gin.Context, target any) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(target); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func queryInt(c *gin.Context, key string) int {
	value, _ := strconv.Atoi(c.Query(key))
	return value
}

func queryBool(c *gin.Context, key string) bool {
	value, _ := strconv.ParseBool(c.Query(key))
	return value
}
