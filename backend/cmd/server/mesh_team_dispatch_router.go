package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/agentplatform"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/middleware/security"
)

type meshTeamDispatchTarget struct {
	OwnerID   string `json:"ownerId,omitempty"`
	RoleID    string `json:"roleId,omitempty"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	RequestID string `json:"requestId,omitempty"`
	Text      string `json:"text,omitempty"`
	Message   string `json:"message,omitempty"`
}

func registerMeshTeamDispatchRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	if services.OwnedBusiness == nil || services.DeviceMesh == nil {
		return
	}
	mesh.POST("/teams/:id/dispatch", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		policy, err := services.DeviceMesh.Coordination.Get(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
			return
		}
		isConsole, err := services.DeviceMesh.Coordination.IsCoreConsole(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
			return
		}
		if !policy.Administrator && !isConsole {
			c.JSON(http.StatusForbidden, gin.H{"code": "mesh.administrator_required", "message": "调度其他设备的角色需要统筹中心管理员授权"})
			return
		}
		var input struct {
			Message            string   `json:"message"`
			MentionAgentIDs    []string `json:"mentionAgentIds"`
			RequiredCapability string   `json:"requiredCapability"`
			ConversationID     string   `json:"conversationId"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
		if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Message) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": "mesh.invalid_team_message", "message": "团队调度消息不能为空"})
			return
		}
		platform := services.AgentPlatform
		if platform == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "mesh.team_service_unavailable", "message": "团队服务不可用"})
			return
		}
		result, err := platform.RouteTeamMessage(c.Request.Context(), actor.SpaceID.String(), c.Param("id"), agentplatform.RouteTeamMessageInput{
			Message:            input.Message,
			MentionAgentIDs:    input.MentionAgentIDs,
			RequiredCapability: input.RequiredCapability,
			ConversationID:     input.ConversationID,
		})
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": "mesh.team_route_failed", "message": err.Error()})
			return
		}
		targets := make([]meshTeamDispatchTarget, 0, len(result.Targets))
		for _, target := range result.Targets {
			item := meshTeamDispatchTarget{OwnerID: target.OwnerID, RoleID: target.AgentID, Mode: target.InvocationMode}
			switch {
			case !target.RequiresDevice:
				item.Status = "center"
			case target.AgentID == "" && target.InvocationMode != agentplatform.InvocationModeCapability:
				item.Status = "skipped"
				item.Message = "团队成员缺少角色"
			default:
				requestID := uuid.NewString()
				roleID := target.AgentID
				if target.InvocationMode == agentplatform.InvocationModeCapability {
					roleID = ""
					item.RoleID = ""
				}
				request := business.Request{
					SpaceID:         actor.SpaceID.String(),
					DeviceID:        actor.DeviceID.String(),
					CoreID:          coreID,
					TargetDeviceID:  target.OwnerID,
					ResourceOwnerID: target.OwnerID,
					RoleOwnerID:     target.OwnerID,
					RoleID:          roleID,
					ConversationID:  input.ConversationID,
					Message:         input.Message,
					RequestID:       requestID,
				}
				response, runErr := services.OwnedBusiness.RunEvents(c.Request.Context(), request, nil)
				item.RequestID = requestID
				if runErr != nil {
					item.Status = "failed"
					item.Message = runErr.Error()
				} else {
					item.Status = "completed"
					item.Text = response.Text
				}
			}
			targets = append(targets, item)
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"teamId": result.TeamID, "strategy": result.Strategy, "targets": targets}})
	})
}
