package agentplatform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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
	base := group.Group("/agent-platform")
	base.Use(handler.platformActor)
	base.GET("/bootstrap", handler.bootstrap)
	base.GET("/overview", handler.overview)

	agents := base.Group("/agents")
	agents.GET("", handler.listAgents)
	agents.POST("", handler.createAgent)
	agents.GET("/templates", handler.listTemplates)
	agents.GET("/:id", handler.getAgent)
	agents.PUT("/:id", handler.updateAgent)
	agents.POST("/:id/test", handler.testAgent)
	agents.POST("/:id/publish", handler.publishAgent)
	agents.POST("/:id/rollback", handler.rollbackAgent)
	agents.POST("/:id/disable", handler.disableAgent)
	agents.POST("/:id/migrate", handler.migrateAgent)
	agents.GET("/:id/diff", handler.diffAgent)
	agents.GET("/:id/releases", handler.listAgentReleases)
	agents.GET("/:id/runtime-profile", handler.getResolvedAgentRuntime)
	agents.GET("/:id/autonomy", handler.getAgentAutonomy)
	agents.PUT("/:id/autonomy", handler.upsertAgentAutonomy)
	agents.POST("/:id/export", handler.exportAgent)
	agents.POST("/:id/export-archive", handler.exportAgentArchive)
	agents.GET("/:id/export-archive", handler.exportAgentArchive)
	agents.POST("/import", handler.importAgent)
	agents.POST("/import-archive", handler.importAgentArchive)

	users := base.Group("/users")
	users.GET("", handler.listUsers)
	users.POST("", handler.createUser)
	users.GET("/memberships", handler.listMemberships)
	users.POST("/invitations", handler.inviteUser)
	users.POST("/invitations/accept", handler.acceptInvitation)

	teams := base.Group("/teams")
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

	approvals := base.Group("/approvals")
	approvals.GET("", handler.listApprovals)
	approvals.POST("", handler.createApproval)
	approvals.GET("/:id", handler.getApproval)
	approvals.POST("/:id/decide", handler.decideApproval)
	approvals.POST("/:id/resume", handler.resumeApproval)

	reconciliations := base.Group("/reconciliations")
	reconciliations.GET("", handler.listReconciliations)
	reconciliations.POST("", handler.createReconciliation)
	reconciliations.POST("/:id/resolve", handler.resolveReconciliation)

	credentials := base.Group("/credentials")
	credentials.GET("", handler.listCredentialGrants)
	credentials.GET("/leases", handler.listCapabilityLeases)
	credentials.POST("", handler.createCredentialGrant)
	credentials.POST("/:id/revoke", handler.revokeCredentialGrant)
	credentials.POST("/:id/leases", handler.createCapabilityLease)
	credentials.POST("/leases/consume", handler.consumeCapabilityLease)

	network := base.Group("/network-policies")
	network.GET("", handler.listNetworkPolicies)
	network.GET("/decisions", handler.listEgressDecisions)
	network.POST("", handler.createNetworkPolicy)
	network.PUT("/:id", handler.updateNetworkPolicy)
	network.POST("/evaluate", handler.evaluateEgress)

	leases := base.Group("/resource-leases")
	leases.GET("", handler.listResourceLeases)
	leases.POST("", handler.acquireResourceLease)
	leases.POST("/:id/release", handler.releaseResourceLease)
	leases.POST("/operations/begin", handler.beginIdempotentOperation)
	leases.POST("/operations/:id/complete", handler.completeIdempotentOperation)

	budgets := base.Group("/budgets")
	budgets.GET("", handler.listBudgetPolicies)
	budgets.POST("", handler.createBudgetPolicy)
	budgets.GET("/check", handler.checkBudget)
	budgets.POST("/leases", handler.acquireBudgetLease)
	budgets.POST("/leases/:id/release", handler.releaseBudgetLease)
	budgets.POST("/:id/usage", handler.recordBudgetUsage)

	runtimes := base.Group("/runtimes")
	runtimes.GET("", handler.listRuntimes)
	runtimes.POST("", handler.createRuntime)
	runtimes.POST("/remote", handler.registerRemoteRuntime)
	runtimes.POST("/:id/start", handler.startRuntime)
	runtimes.POST("/:id/pause", handler.pauseRuntime)
	runtimes.POST("/:id/resume", handler.resumeRuntime)
	runtimes.POST("/:id/stop", handler.stopRuntime)
	runtimes.POST("/:id/exec", handler.execRuntime)
	runtimes.POST("/:id/snapshots", handler.snapshotRuntime)
	runtimes.POST("/:id/reset", handler.resetRuntime)

	hostControl := base.Group("/host-control")
	hostControl.POST("", handler.createHostControl)
	hostControl.POST("/:id/approve", handler.approveHostControl)
	hostControl.POST("/:id/end", handler.endHostControl)
	hostControl.POST("/:id/actions", handler.executeHostControl)

	evaluations := base.Group("/evaluations")
	evaluations.GET("/suites", handler.listEvaluationSuites)
	evaluations.POST("/suites", handler.createEvaluationSuite)
	evaluations.GET("/suites/:id/cases", handler.listEvaluationCases)
	evaluations.POST("/suites/:id/cases", handler.createEvaluationCase)
	evaluations.POST("/suites/:id/run", handler.runEvaluation)
	evaluations.GET("/runs", handler.listEvaluationRuns)

	activity := base.Group("/activity")
	activity.GET("", handler.listActivity)
	activity.GET("/:id", handler.getActivity)

	market := base.Group("/market")
	market.GET("/categories", handler.listMarketplaceCategories)
	market.GET("/listings", handler.listMarketplace)
	market.GET("/listings/:id", handler.getMarketplaceListing)
	market.POST("/listings", handler.publishMarketplaceListing)
	market.POST("/listings/:id/withdraw", handler.withdrawMarketplaceVersion)
	market.POST("/listings/:id/install", handler.installMarketplaceListing)

	a2a := base.Group("/a2a")
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
	if strings.HasPrefix(c.FullPath(), "/api/agent-platform/credentials") ||
		strings.HasPrefix(c.FullPath(), "/api/agent-platform/network-policies") ||
		strings.HasPrefix(c.FullPath(), "/api/agent-platform/budgets") {
		permission = "platform.admin"
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
	categories, err := h.service.ListMarketplaceCategories(c.Request.Context())
	if err != nil {
		h.respondError(c, err)
		return
	}
	templates := []map[string]any{
		{"id": "companion", "name": "陪伴 Agent", "archetype": "companion", "category": "agent-identities"},
		{"id": "worker", "name": "执行 Agent", "archetype": "worker", "category": "productivity"},
		{"id": "team", "name": "团队负责人", "archetype": "team_lead", "category": "productivity"},
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{
		"spaceId": h.spaceID, "userId": h.ownerID, "templates": templates,
		"categories": categories, "runtimeProviders": []string{"local", "isolated", "oci", "remote", "android"},
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
	approvals, err := h.service.ListApprovals(ctx, h.spaceID, "pending", "", "", 20)
	if err != nil {
		h.respondError(c, err)
		return
	}
	reconciliations, err := h.service.ListReconciliations(ctx, h.spaceID, "open", 20)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{
		"agents": agents, "activity": events, "summary": summary,
		"pendingApprovals": approvals, "reconciliations": reconciliations,
	}})
}

func (h *HTTPHandler) listAgents(c *gin.Context) {
	items, err := h.service.ListAgents(c.Request.Context(), h.spaceID, queryBool(c, "includeDisabled"), queryInt(c, "limit"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) createAgent(c *gin.Context) {
	var input CreateAgentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if input.OwnerID == "" {
		input.OwnerID = h.actorID(c)
	}
	item, err := h.service.CreateAgent(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) getAgent(c *gin.Context) {
	item, err := h.service.GetAgent(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) updateAgent(c *gin.Context) {
	var input UpdateAgentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.UpdateAgent(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) testAgent(c *gin.Context) {
	var input struct {
		RevisionID string `json:"revisionId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if input.RevisionID == "" {
		detail, err := h.service.GetAgent(c.Request.Context(), h.spaceID, c.Param("id"))
		if err != nil {
			h.respondError(c, err)
			return
		}
		input.RevisionID = detail.Revision.ID
	}
	item, err := h.service.TestAgentRevision(c.Request.Context(), h.spaceID, c.Param("id"), input.RevisionID)
	h.respond(c, item, err)
}

func (h *HTTPHandler) publishAgent(c *gin.Context) {
	var input PublishAgentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.PublishAgent(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) rollbackAgent(c *gin.Context) {
	var input struct {
		ReleaseID        string `json:"releaseId"`
		ExpectedRevision int64  `json:"expectedRevision"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.RollbackAgent(c.Request.Context(), h.spaceID, c.Param("id"), input.ReleaseID, input.ExpectedRevision)
	h.respond(c, item, err)
}

func (h *HTTPHandler) disableAgent(c *gin.Context) {
	var input struct {
		ExpectedRevision int64 `json:"expectedRevision"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	err := h.service.DisableAgent(c.Request.Context(), h.spaceID, c.Param("id"), input.ExpectedRevision)
	h.respond(c, gin.H{"disabled": err == nil}, err)
}

func (h *HTTPHandler) migrateAgent(c *gin.Context) {
	var input struct {
		TeamID           string `json:"teamId"`
		OwnerID          string `json:"ownerId"`
		ExpectedRevision int64  `json:"expectedRevision"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.MigrateAgent(c.Request.Context(), h.spaceID, c.Param("id"), input.TeamID, input.OwnerID, input.ExpectedRevision)
	h.respond(c, item, err)
}

func (h *HTTPHandler) diffAgent(c *gin.Context) {
	from, err := strconv.ParseInt(c.Query("from"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	to, err := strconv.ParseInt(c.Query("to"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.DiffAgentRevisions(c.Request.Context(), h.spaceID, c.Param("id"), from, to)
	h.respond(c, item, err)
}

func (h *HTTPHandler) listAgentReleases(c *gin.Context) {
	items, err := h.service.ListAgentReleases(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) getResolvedAgentRuntime(c *gin.Context) {
	detail, err := h.service.GetAgent(c.Request.Context(), h.spaceID, c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	profile, found, err := h.service.ResolveAgentRuntime(c.Request.Context(), h.spaceID, detail.Profile.CharacterID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	if !found {
		h.respondError(c, errors.New("Agent 尚未发布，不能作为运行时使用"))
		return
	}
	h.respond(c, profile, nil)
}

func (h *HTTPHandler) getAgentAutonomy(c *gin.Context) {
	item, err := h.service.GetAutonomyPolicy(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) upsertAgentAutonomy(c *gin.Context) {
	var input UpsertAutonomyPolicyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.UpsertAutonomyPolicy(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) exportAgent(c *gin.Context) {
	var input struct {
		Version string `json:"version"`
	}
	if err := bindOptionalJSON(c, &input); err != nil {
		h.respondError(c, err)
		return
	}
	if input.Version == "" {
		input.Version = c.Query("version")
	}
	item, err := h.service.ExportAgentPackage(c.Request.Context(), h.spaceID, c.Param("id"), input.Version)
	h.respond(c, item, err)
}

func (h *HTTPHandler) exportAgentArchive(c *gin.Context) {
	var input struct {
		Version string `json:"version"`
	}
	if err := bindOptionalJSON(c, &input); err != nil {
		h.respondError(c, err)
		return
	}
	if input.Version == "" {
		input.Version = c.Query("version")
	}
	archive, err := h.service.CreateAgentArchive(c.Request.Context(), h.spaceID, c.Param("id"), input.Version)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.FileAttachment(archive.Path, archive.FileName)
}

func (h *HTTPHandler) importAgent(c *gin.Context) {
	var input AgentPackage
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.ImportAgentPackage(c.Request.Context(), h.spaceID, h.actorID(c), input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) importAgentArchive(c *gin.Context) {
	file, err := c.FormFile("package")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if file.Size <= 0 || file.Size > 20<<30 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "Agent 可携带包大小无效"})
		return
	}
	source, err := file.Open()
	if err != nil {
		h.respondError(c, err)
		return
	}
	defer source.Close()
	temp, err := os.CreateTemp("", "amitia-agent-import-*.zip")
	if err != nil {
		h.respondError(c, err)
		return
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := io.Copy(temp, source); err != nil {
		temp.Close()
		h.respondError(c, err)
		return
	}
	if err := temp.Close(); err != nil {
		h.respondError(c, err)
		return
	}
	item, err := h.service.ImportAgentArchive(c.Request.Context(), h.spaceID, h.actorID(c), tempPath)
	h.respondStatus(c, item, err, http.StatusCreated)
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

func (h *HTTPHandler) listApprovals(c *gin.Context) {
	items, err := h.service.ListApprovals(c.Request.Context(), h.spaceID, c.Query("status"), c.Query("agentId"), c.Query("conversationId"), queryInt(c, "limit"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) createApproval(c *gin.Context) {
	var input CreateApprovalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if input.RequestedBy == "" {
		input.RequestedBy = h.actorID(c)
	}
	item, err := h.service.CreateApproval(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) getApproval(c *gin.Context) {
	item, decisions, err := h.service.GetApproval(c.Request.Context(), h.spaceID, c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{"approval": item, "decisions": decisions}})
}

func (h *HTTPHandler) decideApproval(c *gin.Context) {
	var input DecideApprovalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if input.ActorID == "" {
		input.ActorID = h.actorID(c)
	}
	item, err := h.service.DecideApproval(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) resumeApproval(c *gin.Context) {
	item, err := h.service.ResumeApproval(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) listReconciliations(c *gin.Context) {
	items, err := h.service.ListReconciliations(c.Request.Context(), h.spaceID, c.Query("status"), queryInt(c, "limit"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) createReconciliation(c *gin.Context) {
	var input CreateReconciliationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CreateReconciliation(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) resolveReconciliation(c *gin.Context) {
	var input ResolveReconciliationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if input.ActorID == "" {
		input.ActorID = h.actorID(c)
	}
	item, err := h.service.ResolveReconciliation(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) createCredentialGrant(c *gin.Context) {
	var input CreateCredentialGrantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CreateCredentialGrant(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) listCredentialGrants(c *gin.Context) {
	items, err := h.service.ListCredentialGrants(c.Request.Context(), h.spaceID, c.Query("agentId"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) listCapabilityLeases(c *gin.Context) {
	items, err := h.service.ListCapabilityLeases(c.Request.Context(), h.spaceID, c.Query("agentId"), queryBool(c, "activeOnly"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) revokeCredentialGrant(c *gin.Context) {
	item, err := h.service.RevokeCredentialGrant(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) createCapabilityLease(c *gin.Context) {
	var input struct {
		Capability string `json:"capability"`
		Resource   string `json:"resource"`
		MaxUses    int    `json:"maxUses"`
		TTLSeconds int    `json:"ttlSeconds"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CreateCapabilityLease(c.Request.Context(), h.spaceID, c.Param("id"), input.Capability, input.Resource, input.MaxUses, time.Duration(input.TTLSeconds)*time.Second)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) consumeCapabilityLease(c *gin.Context) {
	var input struct {
		Token      string `json:"token"`
		Capability string `json:"capability"`
		Resource   string `json:"resource"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.ConsumeCapabilityLease(c.Request.Context(), h.spaceID, input.Token, input.Capability, input.Resource)
	h.respond(c, item, err)
}

func (h *HTTPHandler) createNetworkPolicy(c *gin.Context) {
	var input CreateNetworkPolicyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CreateNetworkPolicy(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) updateNetworkPolicy(c *gin.Context) {
	var input UpdateNetworkPolicyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.respondError(c, err)
		return
	}
	item, err := h.service.UpdateNetworkPolicy(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) listNetworkPolicies(c *gin.Context) {
	items, err := h.service.ListNetworkPolicies(c.Request.Context(), h.spaceID, c.Query("agentId"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) listEgressDecisions(c *gin.Context) {
	items, err := h.service.ListEgressDecisions(c.Request.Context(), h.spaceID, c.Query("agentId"), queryInt(c, "limit"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) evaluateEgress(c *gin.Context) {
	var input EgressRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.EvaluateEgress(c.Request.Context(), h.spaceID, input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) acquireResourceLease(c *gin.Context) {
	var input AcquireResourceLeaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.AcquireResourceLease(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) releaseResourceLease(c *gin.Context) {
	var input struct {
		AgentID string `json:"agentId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.ReleaseResourceLease(c.Request.Context(), h.spaceID, c.Param("id"), input.AgentID)
	h.respond(c, item, err)
}

func (h *HTTPHandler) listResourceLeases(c *gin.Context) {
	items, err := h.service.ListResourceLeases(c.Request.Context(), h.spaceID, c.Query("resourceType"), c.Query("resourceId"), queryBool(c, "activeOnly"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) beginIdempotentOperation(c *gin.Context) {
	var input BeginIdempotentOperationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.BeginIdempotentOperation(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) completeIdempotentOperation(c *gin.Context) {
	var input CompleteIdempotentOperationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CompleteIdempotentOperation(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) createBudgetPolicy(c *gin.Context) {
	var input CreateBudgetPolicyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.CreateBudgetPolicy(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) listBudgetPolicies(c *gin.Context) {
	items, err := h.service.ListBudgetPolicies(c.Request.Context(), h.spaceID, c.Query("scopeType"), c.Query("scopeId"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) checkBudget(c *gin.Context) {
	item, err := h.service.CheckBudget(c.Request.Context(), h.spaceID, c.Query("scopeType"), c.Query("scopeId"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) acquireBudgetLease(c *gin.Context) {
	var input AcquireBudgetLeaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	lease, reservation, err := h.service.AcquireBudgetLease(c.Request.Context(), h.spaceID, input)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 201, "msg": "ok", "data": gin.H{"lease": lease, "reservation": reservation}})
}

func (h *HTTPHandler) releaseBudgetLease(c *gin.Context) {
	var input struct {
		HolderID string `json:"holderId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.ReleaseBudgetLease(c.Request.Context(), h.spaceID, c.Param("id"), input.HolderID)
	h.respond(c, item, err)
}

func (h *HTTPHandler) recordBudgetUsage(c *gin.Context) {
	var input RecordBudgetUsageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.RecordBudgetUsage(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) listRuntimes(c *gin.Context) {
	items, err := h.service.Runtime().List(c.Request.Context(), h.spaceID, c.Query("agentId"))
	h.respond(c, items, err)
}

func (h *HTTPHandler) createRuntime(c *gin.Context) {
	var input CreateRuntimeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.Runtime().Create(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) registerRemoteRuntime(c *gin.Context) {
	var input struct {
		Runtime  CreateRuntimeInput `json:"runtime"`
		Endpoint string             `json:"endpoint"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.Runtime().RegisterRemote(c.Request.Context(), h.spaceID, input.Runtime, input.Endpoint)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) startRuntime(c *gin.Context) {
	item, err := h.service.Runtime().Start(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) pauseRuntime(c *gin.Context) {
	item, err := h.service.Runtime().Pause(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) resumeRuntime(c *gin.Context) {
	item, err := h.service.Runtime().Resume(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) stopRuntime(c *gin.Context) {
	item, err := h.service.Runtime().Stop(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) execRuntime(c *gin.Context) {
	var input RuntimeExecInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.respondError(c, err)
		return
	}
	item, err := h.service.Runtime().Exec(c.Request.Context(), h.spaceID, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *HTTPHandler) snapshotRuntime(c *gin.Context) {
	var input struct {
		Name string `json:"name"`
	}
	if err := bindOptionalJSON(c, &input); err != nil {
		h.respondError(c, err)
		return
	}
	item, err := h.service.Runtime().Snapshot(c.Request.Context(), h.spaceID, c.Param("id"), input.Name)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) resetRuntime(c *gin.Context) {
	var input struct {
		SnapshotID string `json:"snapshotId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.Runtime().Reset(c.Request.Context(), h.spaceID, c.Param("id"), input.SnapshotID)
	h.respond(c, item, err)
}

func (h *HTTPHandler) createHostControl(c *gin.Context) {
	var input CreateHostControlInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.Runtime().CreateHostControl(c.Request.Context(), h.spaceID, input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) approveHostControl(c *gin.Context) {
	item, err := h.service.Runtime().ApproveHostControl(c.Request.Context(), h.spaceID, c.Param("id"), h.actorID(c))
	h.respond(c, item, err)
}

func (h *HTTPHandler) endHostControl(c *gin.Context) {
	item, err := h.service.Runtime().EndHostControl(c.Request.Context(), h.spaceID, c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) executeHostControl(c *gin.Context) {
	var input HostControlAction
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.Runtime().ExecuteHostControl(c.Request.Context(), h.spaceID, c.Param("id"), input)
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

func (h *HTTPHandler) listMarketplaceCategories(c *gin.Context) {
	items, err := h.service.ListMarketplaceCategories(c.Request.Context())
	h.respond(c, items, err)
}

func (h *HTTPHandler) listMarketplace(c *gin.Context) {
	items, err := h.service.ListMarketplace(c.Request.Context(), MarketplaceFilter{
		CategoryID: c.Query("categoryId"), Search: c.Query("q"), TrustLevel: c.Query("trust"),
		Platform: c.Query("platform"), Publisher: c.Query("publisher"), Limit: queryInt(c, "limit"),
	})
	h.respond(c, items, err)
}

func (h *HTTPHandler) getMarketplaceListing(c *gin.Context) {
	item, err := h.service.GetMarketplaceListing(c.Request.Context(), c.Param("id"))
	h.respond(c, item, err)
}

func (h *HTTPHandler) publishMarketplaceListing(c *gin.Context) {
	var input PublishMarketplaceListingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	item, err := h.service.PublishMarketplaceListing(c.Request.Context(), input)
	h.respondStatus(c, item, err, http.StatusCreated)
}

func (h *HTTPHandler) withdrawMarketplaceVersion(c *gin.Context) {
	var input struct {
		Version string `json:"version"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	err := h.service.WithdrawMarketplaceVersion(c.Request.Context(), c.Param("id"), input.Version, input.Reason)
	h.respond(c, gin.H{"withdrawn": err == nil}, err)
}

func (h *HTTPHandler) installMarketplaceListing(c *gin.Context) {
	var input struct {
		Version string `json:"version"`
	}
	if err := bindOptionalJSON(c, &input); err != nil {
		h.respondError(c, err)
		return
	}
	item, err := h.service.InstallMarketplaceListing(c.Request.Context(), h.spaceID, c.Param("id"), input.Version, h.actorID(c))
	h.respondStatus(c, item, err, http.StatusAccepted)
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
