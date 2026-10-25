package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerMeshGroupChatRouter(group *gin.RouterGroup, services *AppServices, coreID string) {
	requestFor := func(c *gin.Context, characterID, targetDeviceID, requestID, conversationID string) business.Request {
		actor := security.GetActor(c)
		if actor == nil {
			return business.Request{}
		}
		return business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: targetDeviceID, RoleID: characterID, ConversationID: conversationID, RequestID: requestID}
	}
	respond := func(c *gin.Context, result business.GroupMutationResponse, err error) {
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
			return
		}
		status := http.StatusConflict
		switch {
		case errors.Is(err, coordination.ErrWrongOwner):
			status = http.StatusForbidden
		case errors.Is(err, coordination.ErrScopeExpired):
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"code": "mesh.group_conflict", "message": err.Error()})
	}
	group.POST("/conversations/group", func(c *gin.Context) {
		if security.GetActor(c) == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var input business.CreateGroupConversationInput
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "创建群聊参数无效"})
			return
		}
		result, err := services.OwnedBusiness.CreateGroupConversation(c.Request.Context(), requestFor(c, input.CharacterID, input.TargetDeviceID, input.RequestID, ""), input)
		respond(c, result, err)
	})
	group.POST("/conversations/:conversationId/members", func(c *gin.Context) {
		if security.GetActor(c) == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var input business.GroupMembersInput
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "群成员邀请参数无效"})
			return
		}
		result, err := services.OwnedBusiness.InviteGroupMembers(c.Request.Context(), requestFor(c, input.CharacterID, input.TargetDeviceID, input.RequestID, c.Param("conversationId")), input)
		respond(c, result, err)
	})
	group.DELETE("/conversations/:conversationId/members/:memberId", func(c *gin.Context) {
		if security.GetActor(c) == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var input business.GroupMemberRemoveInput
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		if c.ShouldBindJSON(&input) != nil {
			input = business.GroupMemberRemoveInput{RequestID: c.Query("requestId"), CharacterID: c.Query("characterId"), TargetDeviceID: c.Query("targetDeviceId")}
			if value := c.Query("expectedMembershipRevision"); value != "" {
				var revision int64
				if _, scanErr := fmt.Sscan(value, &revision); scanErr == nil {
					input.ExpectedMembershipRevision = &revision
				}
			}
			if input.RequestID == "" || input.CharacterID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"message": "移除群成员参数无效"})
				return
			}
		}
		input.MemberID = c.Param("memberId")
		result, err := services.OwnedBusiness.RemoveGroupMember(c.Request.Context(), requestFor(c, input.CharacterID, input.TargetDeviceID, input.RequestID, c.Param("conversationId")), input)
		respond(c, result, err)
	})
	group.PATCH("/conversations/:conversationId/group-settings", func(c *gin.Context) {
		if security.GetActor(c) == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var input business.GroupSettingsInput
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "群设置参数无效"})
			return
		}
		result, err := services.OwnedBusiness.UpdateGroupSettings(c.Request.Context(), requestFor(c, input.CharacterID, input.TargetDeviceID, input.RequestID, c.Param("conversationId")), input)
		respond(c, result, err)
	})
	group.POST("/conversations/:conversationId/reply-requests", func(c *gin.Context) {
		if security.GetActor(c) == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var input business.GroupReplyRequestInput
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "点名回复参数无效"})
			return
		}
		result, err := services.OwnedBusiness.CreateGroupReplyRequest(c.Request.Context(), requestFor(c, input.CharacterID, input.TargetDeviceID, input.RequestID, c.Param("conversationId")), input)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": "mesh.group_reply_failed", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
	})
	group.POST("/conversations/:conversationId/group-rounds/:roundId/retry", func(c *gin.Context) {
		if security.GetActor(c) == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var input business.GroupDeliveryRetryInput
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		if c.ShouldBindJSON(&input) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "群成员重试参数无效"})
			return
		}
		result, err := services.OwnedBusiness.RetryGroupDelivery(c.Request.Context(), requestFor(c, input.CharacterID, input.TargetDeviceID, input.RequestID, c.Param("conversationId")), c.Param("roundId"), input)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": "mesh.group_retry_failed", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
	})
}
