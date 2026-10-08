package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/notificationruntime"
)

func registerMeshRealtimeInvitationRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	mesh.POST("/realtime/invitations", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		var input struct {
			business.Request
			CallType string `json:"callType"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(400, gin.H{"message": "邀请参数无效"})
			return
		}
		input.Request.SpaceID, input.Request.DeviceID, input.Request.CoreID = actor.SpaceID.String(), actor.DeviceID.String(), coreID
		invitation, ack, err := services.OwnedBusiness.CreateRealtimeInvitation(c.Request.Context(), input.Request, input.CallType)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		queued := 0
		if services.NotificationRuntime != nil {
			queued, err = services.NotificationRuntime.PushIncomingCall(c.Request.Context(), actor.SpaceID.String(), notificationruntime.IncomingCall{CallID: invitation.ID, ConversationID: invitation.ConversationID, CharacterID: invitation.CharacterID, CallerName: actor.DeviceID.String(), CallType: invitation.CallType, RecipientDeviceID: invitation.RecipientDeviceID, Owned: true})
		}
		data := gin.H{"invitation": invitation, "executionScope": invitation.Scope, "saved": true, "acknowledgement": ack, "queuedDevices": queued}
		if err != nil {
			data["deliveryError"] = err.Error()
		}
		c.JSON(200, gin.H{"code": 200, "data": data})
	})
	mesh.GET("/realtime/invitations/:id", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, RoleID: c.Query("characterId"), RequestID: uuid.NewString()}
		invitation, err := services.OwnedBusiness.ReadRealtimeInvitation(c.Request.Context(), request, c.Param("id"))
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"invitation": invitation, "executionScope": invitation.Scope}})
	})
	mesh.POST("/realtime/invitations/:id/accept", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		var input struct {
			RequestID   string                       `json:"requestId"`
			CharacterID string                       `json:"characterId"`
			Scope       *coordination.ExecutionScope `json:"expectedExecutionScope"`
			Revision    int64                        `json:"expectedRevision"`
			Nonce       string                       `json:"nonce"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(400, gin.H{"message": "接听参数无效"})
			return
		}
		origin, valid := ownedRealtimeOrigin(c.GetHeader("Origin"))
		if !valid {
			c.AbortWithStatus(403)
			return
		}
		request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, RoleID: input.CharacterID, RequestID: input.RequestID, ExpectedScope: input.Scope}
		invitation, ack, err := services.OwnedBusiness.AcceptRealtimeInvitation(c.Request.Context(), request, c.Param("id"), input.Nonce, input.Revision)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		request.ConversationID, request.ConversationOrigin, request.HistoricalRoleID = invitation.ConversationID, invitation.ConversationOrigin, invitation.HistoricalRoleID
		request.ExpectedScope = &invitation.Scope
		ctx, scope, finish, err := services.OwnedBusiness.RealtimeAuthority(c.Request.Context(), request)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		defer finish()
		if err := coordination.ValidateCurrent(ctx); err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		ticket, err := issueOwnedRealtimeTicket(services, request, origin, scope)
		if err != nil {
			c.JSON(503, gin.H{"message": err.Error(), "saved": true})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"invitation": invitation, "executionScope": invitation.Scope, "saved": true, "acknowledgement": ack, "ticket": ticket}})
	})
}
