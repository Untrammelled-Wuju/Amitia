package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerMeshBusinessRouter(group *gin.RouterGroup, services *AppServices, coreID string) {
	if services.DeviceMesh == nil || services.OwnedBusiness == nil {
		return
	}
	mesh := group.Group("/device-mesh/v1/business")
	registerMeshTaskOwnerRouter(mesh, services)
	mesh.POST("/continuity/signals", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		var payload struct {
			RequestID      string            `json:"requestId"`
			CharacterID    string            `json:"characterId"`
			TargetDeviceID string            `json:"targetDeviceId"`
			Signal         continuity.Signal `json:"signal"`
		}
		if c.ShouldBindJSON(&payload) != nil {
			c.JSON(400, gin.H{"message": "持续事项事件参数无效"})
			return
		}
		resolved, err := services.OwnedBusiness.SignalContinuity(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: payload.TargetDeviceID, RoleID: payload.CharacterID, RequestID: payload.RequestID}, payload.Signal)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"resolvedWaitIds": resolved}})
	})
	mesh.GET("/continuity", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: c.Query("targetDeviceId"), RoleID: c.Query("characterId"), RequestID: uuid.NewString()}
		if id := c.Query("id"); id != "" {
			document, _, err := services.OwnedBusiness.Continuity(c.Request.Context(), request, business.ContinuityMutation{ID: id, Action: "read"})
			if err != nil {
				c.JSON(409, gin.H{"message": err.Error()})
				return
			}
			c.JSON(200, gin.H{"code": 200, "data": document})
			return
		}
		cursor := c.Query("cursor")
		if len(cursor) > 4096 {
			c.JSON(400, gin.H{"message": "持续事项分页参数无效"})
			return
		}
		result, err := services.OwnedBusiness.Query(c.Request.Context(), request, coordination.DataQuery{ResourceKind: "continuity", Cursor: cursor, Limit: 128})
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		documents := []business.OwnedContinuity{}
		for _, resource := range result.Snapshot.Resources {
			if resource.Kind != "continuity" {
				continue
			}
			var document business.OwnedContinuity
			if json.Unmarshal(resource.Body, &document) != nil {
				c.JSON(503, gin.H{"message": "持续事项数据格式无效"})
				return
			}
			documents = append(documents, business.PresentContinuity(document, result.Scope, time.Now()))
		}
		if c.Query("pagination") == "1" {
			c.JSON(200, gin.H{"code": 200, "data": gin.H{"documents": documents, "executionScope": result.Scope, "nextCursor": result.Snapshot.NextCursors["continuity"]}})
		} else {
			c.JSON(200, gin.H{"code": 200, "data": documents})
		}
	})
	mesh.POST("/continuity", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		var payload struct {
			business.ContinuityMutation
			RequestID      string `json:"requestId"`
			RoleID         string `json:"characterId"`
			TargetDeviceID string `json:"targetDeviceId"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(400, gin.H{"message": "持续事项参数无效"})
			return
		}
		if payload.ExpectedCoreID == "" || payload.ExpectedOwnerID == "" || payload.ExpectedModeRevision < 1 {
			c.JSON(400, gin.H{"message": "持续事项操作缺少服务提供者和数据归属版本，请刷新后重试"})
			return
		}
		if payload.Action == "claim" || payload.Action == "heartbeat" || payload.Action == "finish" || payload.Action == "mark_unknown" || payload.Action == "read" {
			c.JSON(403, gin.H{"message": "执行租约只能由 Core 调度器管理"})
			return
		}
		document, ack, err := services.OwnedBusiness.Continuity(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: payload.TargetDeviceID, RoleID: payload.RoleID, RequestID: payload.RequestID}, payload.ContinuityMutation)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"document": document, "acknowledgement": ack}})
	})
	queryHandler := func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: c.Query("targetDeviceId"), RoleID: c.Query("characterId")}
		query := coordination.DataQuery{ConversationID: c.Param("conversationId"), HistoricalRoleID: c.Query("historicalRoleId"), ListConversations: c.Param("conversationId") == "", Limit: 128}
		search, searchErr := coordination.NormalizeConversationSearch(c.Query("keyword"))
		if searchErr != nil || search != "" && !query.ListConversations {
			c.JSON(400, gin.H{"message": "会话搜索参数无效"})
			return
		}
		query.SearchQuery = search
		if owner := c.Query("conversationOwnerId"); owner != "" {
			if query.ConversationID == "" || len(owner) > 512 {
				c.JSON(400, gin.H{"message": "会话来源参数无效"})
				return
			}
			request.ConversationOrigin = &business.ConversationOrigin{OwnerID: owner, ID: query.ConversationID}
		}
		query.ResourceKind, query.Cursor, query.HistoricalCursor = c.Query("resourceKind"), c.Query("cursor"), c.Query("historicalCursor")
		query.LegacyCursor, query.HistoricalLegacyCursor = c.Query("legacyCursor"), c.Query("historicalLegacyCursor")
		query.HistoricalListCursor = c.Query("historicalListCursor")
		if len(query.HistoricalListCursor) > 4096 || (query.HistoricalListCursor != "" && (!query.ListConversations || query.ResourceKind != "conversation")) {
			c.JSON(400, gin.H{"message": "设备历史会话分页参数无效"})
			return
		}
		if query.ResourceKind != "" && query.ResourceKind != "message" && query.ResourceKind != "conversation" {
			c.JSON(400, gin.H{"message": "聊天历史类型无效"})
			return
		}
		if len(query.Cursor) > 4096 || len(query.HistoricalCursor) > 4096 || len(query.LegacyCursor) > 4096 || len(query.HistoricalLegacyCursor) > 4096 || ((query.Cursor != "" || query.HistoricalCursor != "" || query.LegacyCursor != "" || query.HistoricalLegacyCursor != "") && query.ResourceKind == "") {
			c.JSON(400, gin.H{"message": "聊天历史分页参数无效"})
			return
		}
		if value := c.Query("limit"); value != "" {
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 128 {
				c.JSON(400, gin.H{"message": "聊天历史分页数量无效"})
				return
			}
			query.Limit = limit
		}
		result, err := services.OwnedBusiness.Query(c.Request.Context(), request, query)
		if err != nil {
			status := 409
			if errors.Is(err, coordination.ErrCapabilityGrant) {
				status = 403
			}
			c.JSON(status, gin.H{"code": "mesh.context_unavailable", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": result})
	}
	mesh.GET("/conversations", queryHandler)
	mesh.GET("/conversations/:conversationId", queryHandler)
	mesh.POST("/conversations/:conversationId/summary/generate", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		var input struct {
			RequestID        string                       `json:"requestId"`
			RoleID           string                       `json:"characterId"`
			TargetDeviceID   string                       `json:"targetDeviceId"`
			Origin           *business.ConversationOrigin `json:"conversationOrigin"`
			ExpectedScope    *coordination.ExecutionScope `json:"expectedExecutionScope"`
			ExpectedRevision *int64                       `json:"expectedRevision"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		if c.ShouldBindJSON(&input) != nil || input.ExpectedRevision == nil || input.ExpectedScope == nil {
			c.JSON(400, gin.H{"message": "摘要生成缺少页面权限或数据版本"})
			return
		}
		result, err := services.OwnedBusiness.GenerateSummary(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: input.TargetDeviceID, RoleID: input.RoleID, ConversationID: c.Param("conversationId"), RequestID: input.RequestID, ExpectedScope: input.ExpectedScope, ConversationOrigin: input.Origin}, *input.ExpectedRevision)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": result})
	})
	mesh.GET("/data", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		kind := c.Query("kind")
		switch kind {
		case "memory", "working", "profile", "episodic", "fact", "vector", "graph", "summary":
		default:
			c.JSON(400, gin.H{"message": "记忆层类型无效"})
			return
		}
		cursor := c.Query("cursor")
		legacyCursor := c.Query("legacyCursor")
		if len(cursor) > 4096 || len(legacyCursor) > 4096 {
			c.JSON(400, gin.H{"message": "记忆分页参数无效"})
			return
		}
		request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: c.Query("targetDeviceId"), RoleID: c.Query("characterId")}
		historicalRole := c.Query("historicalRoleId")
		historicalCursor := c.Query("historicalCursor")
		historicalLegacyCursor := c.Query("historicalLegacyCursor")
		if len(historicalRole) > 512 || len(historicalCursor) > 4096 || len(historicalLegacyCursor) > 4096 {
			c.JSON(400, gin.H{"message": "历史记忆参数无效"})
			return
		}
		result, err := services.OwnedBusiness.Query(c.Request.Context(), request, coordination.DataQuery{ResourceKind: kind, Cursor: cursor, LegacyCursor: legacyCursor, HistoricalCursor: historicalCursor, HistoricalLegacyCursor: historicalLegacyCursor, HistoricalRoleID: historicalRole, ConversationID: c.Query("conversationId"), Management: true, Limit: 128})
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": result})
	})
	mesh.GET("/historical-roles", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		roles, scope, err := services.OwnedBusiness.HistoricalRoles(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: c.Query("targetDeviceId"), RoleID: c.Query("characterId"), RequestID: uuid.NewString()})
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"roles": roles, "executionScope": scope}})
	})
	projectionHandler := func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: c.Query("targetDeviceId"), RoleID: c.Query("characterId"), RequestID: uuid.NewString()}
		var expected *coordination.ExecutionScope
		rebuild := c.Request.Method == http.MethodPost
		if rebuild {
			var input struct {
				RoleID         string                       `json:"characterId"`
				TargetDeviceID string                       `json:"targetDeviceId"`
				ExpectedScope  *coordination.ExecutionScope `json:"expectedExecutionScope"`
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
			if err := c.ShouldBindJSON(&input); err != nil || input.ExpectedScope == nil {
				c.JSON(400, gin.H{"message": "索引重建参数无效，请刷新后重试"})
				return
			}
			request.RoleID, request.TargetDeviceID, expected = input.RoleID, input.TargetDeviceID, input.ExpectedScope
		}
		status, scope, err := services.OwnedBusiness.Projections(c.Request.Context(), request, rebuild, expected)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"status": status, "executionScope": scope, "queued": rebuild}})
	}
	mesh.GET("/projections", projectionHandler)
	mesh.POST("/projections/rebuild", projectionHandler)
	mesh.GET("/resources", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		kind, id := c.Query("kind"), c.Query("id")
		if kind == "checkpoint" || kind == "continuity" || id == "" || len(id) > 512 {
			c.JSON(400, gin.H{"message": "数据查询参数无效"})
			return
		}
		resource, scope, err := services.OwnedBusiness.ReadResource(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: c.Query("targetDeviceId"), RoleID: c.Query("characterId"), RequestID: uuid.NewString()}, kind, id)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"resource": resource, "executionScope": scope}})
	})
	mesh.POST("/resources/edit", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		var edit business.EditRequest
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
		if err := c.ShouldBindJSON(&edit); err != nil {
			c.JSON(400, gin.H{"message": "数据修改参数无效"})
			return
		}
		if edit.ExpectedScope == nil || edit.ExpectedScope.CoreID == "" || edit.ExpectedScope.ResourceOwnerID == "" || edit.ExpectedScope.ModeRevision < 1 {
			c.JSON(400, gin.H{"message": "修改请求缺少数据来源版本，请刷新后重试"})
			return
		}
		ack, err := services.OwnedBusiness.Edit(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID}, edit)
		if err != nil {
			c.JSON(409, gin.H{"code": "mesh.edit_conflict", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": ack})
	})
	mesh.POST("/messages/:requestId/interrupt", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		requestID := c.Param("requestId")
		if requestID == "" || len(requestID) > 128 {
			c.JSON(400, gin.H{"message": "请求编号无效"})
			return
		}
		stopped := services.OwnedBusiness.Interrupt(actor.SpaceID.String(), actor.DeviceID.String(), requestID)
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"interrupted": stopped, "requestId": requestID}})
	})
	mesh.GET("/devices/:deviceId/grants", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || (actor.DeviceID.String() != c.Param("deviceId") && !actor.HasPermission(auth.PermSystemAdmin)) {
			c.AbortWithStatusJSON(403, gin.H{"code": "mesh.administrator_required", "message": "只能查看本机授权或使用 Core 管理员权限"})
			return
		}
		grants, err := services.DeviceMesh.Coordination.CapabilityGrants(c.Request.Context(), actor.SpaceID.String(), c.Param("deviceId"))
		if err != nil {
			c.JSON(503, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"grants": grants})
	})
	mesh.PUT("/devices/:deviceId/grants", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || (actor.DeviceID.String() != c.Param("deviceId") && !actor.HasPermission(auth.PermSystemAdmin)) {
			c.AbortWithStatusJSON(403, gin.H{"code": "mesh.administrator_required", "message": "能力授权只能由目标设备或 Core 管理员修改"})
			return
		}
		var request struct {
			CallerID         string `json:"callerId" binding:"required"`
			ExpectedCoreID   string `json:"expectedCoreId" binding:"required"`
			Capability       string `json:"capability" binding:"required"`
			Allowed          bool   `json:"allowed"`
			ExpectedRevision int64  `json:"expectedRevision"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{"message": "授权参数无效"})
			return
		}
		if request.ExpectedCoreID != coreID {
			c.JSON(409, gin.H{"code": "mesh.scope_expired", "message": "服务提供者已变化，请刷新后重新授权"})
			return
		}
		grant, err := services.DeviceMesh.Coordination.SetCapabilityGrant(c.Request.Context(), actor.SpaceID.String(), request.CallerID, c.Param("deviceId"), request.Capability, request.ExpectedRevision, request.Allowed)
		if err != nil {
			c.JSON(409, gin.H{"code": "mesh.grant_conflict", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"grant": grant})
	})
	mesh.GET("/roles", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx, scope, finish, err := services.DeviceMesh.Coordination.Begin(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String(), c.Query("targetDeviceId"), coreID, "", uuid.NewString())
		if err != nil {
			c.JSON(409, gin.H{"code": "mesh.scope_expired", "message": err.Error()})
			return
		}
		defer finish()
		if err := services.DeviceMesh.Coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
			c.JSON(403, gin.H{"code": "mesh.target_grant_required", "message": err.Error()})
			return
		}
		roles, err := services.DeviceMesh.Roles(ctx, scope)
		if err != nil {
			c.JSON(503, gin.H{"code": "mesh.role_source_unavailable", "message": err.Error()})
			return
		}
		visibleRoles := make([]gin.H, 0, len(roles))
		for _, role := range roles {
			visibleRoles = append(visibleRoles, gin.H{"id": role.ID, "name": role.Name, "revision": role.Revision})
		}
		c.JSON(200, gin.H{"roles": visibleRoles, "roleOwnerId": scope.RoleOwnerID, "providerEpoch": scope.ProviderEpoch, "modeRevision": scope.ModeRevision, "executionScope": scope})
	})
	mesh.POST("/messages", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var request business.Request
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{"code": "mesh.invalid_message", "message": "消息参数无效"})
			return
		}
		request.SpaceID = actor.SpaceID.String()
		request.DeviceID = actor.DeviceID.String()
		request.CoreID = coreID
		if request.TargetDeviceID != "" && request.TargetDeviceID != request.DeviceID && request.ExpectedScope == nil {
			c.JSON(400, gin.H{"code": "mesh.invalid_scope", "message": "跨设备调用缺少目标权限快照，请刷新角色后重试"})
			return
		}
		streaming := strings.Contains(c.GetHeader("Accept"), "text/event-stream")
		var emit func(business.Event) error
		if streaming {
			emit = func(event business.Event) error {
				if err := c.Request.Context().Err(); err != nil {
					return err
				}
				c.Header("Cache-Control", "no-store")
				c.Header("X-Accel-Buffering", "no")
				c.SSEvent("message", event)
				c.Writer.Flush()
				return nil
			}
		}
		response, err := services.OwnedBusiness.RunEvents(c.Request.Context(), request, emit)
		if err != nil {
			if streaming && c.Writer.Written() {
				eventType := "failed"
				if response.Interrupted {
					eventType = "interrupted"
				}
				c.SSEvent("message", gin.H{"type": eventType, "message": err.Error(), "data": response})
				c.Writer.Flush()
				return
			}
			status := 503
			code := "mesh.business_unavailable"
			switch {
			case errors.Is(err, coordination.ErrCapabilityGrant):
				status = 403
				code = "mesh.target_grant_required"
			case errors.Is(err, coordination.ErrRoleRequired), errors.Is(err, coordination.ErrRoleSelection):
				status = 409
				code = "mesh.role_required"
			case errors.Is(err, coordination.ErrScopeExpired):
				status = 409
				code = "mesh.scope_expired"
			case errors.Is(err, business.ErrUncertainExecution):
				status = 409
				code = "mesh.execution_uncertain"
			case errors.Is(err, coordination.ErrRequestConflict):
				status = 409
				code = "mesh.request_conflict"
			}
			c.JSON(status, gin.H{"code": code, "message": err.Error(), "data": response})
			return
		}
		if streaming {
			c.Header("Cache-Control", "no-store")
			c.SSEvent("message", gin.H{"type": "completed", "data": response})
			c.Writer.Flush()
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": response, "msg": "操作成功"})
	})
}
