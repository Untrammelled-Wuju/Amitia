package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/temporal"
	"strconv"
	"time"
)

func registerMeshRelationshipTimeRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	handle := func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: c.Query("targetDeviceId"), RoleID: c.Query("characterId")}
		input := coordination.RelationshipTimeRequest{EpisodeID: c.Query("episodeId")}
		input.Limit, _ = strconv.Atoi(c.Query("limit"))
		if c.Request.Method == "PUT" {
			var payload struct {
				Settings json.RawMessage              `json:"settings"`
				Expected *coordination.ExecutionScope `json:"expectedExecutionScope"`
			}
			if c.ShouldBindJSON(&payload) != nil || len(payload.Settings) == 0 || payload.Expected == nil {
				c.JSON(400, gin.H{"message": "关系时间设置或权限版本无效"})
				return
			}
			input.Settings, request.ExpectedScope = payload.Settings, payload.Expected
		}
		result, err := services.OwnedBusiness.RelationshipTime(c.Request.Context(), request, input)
		if err != nil {
			writeRuntimeStateError(c, err)
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": result})
	}
	mesh.GET("/relationship-time", handle)
	mesh.PUT("/relationship-time", handle)
}

func (p *meshLocalDataPort) RelationshipTime(ctx context.Context, scope coordination.ExecutionScope, request coordination.RelationshipTimeRequest) (json.RawMessage, error) {
	if scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID {
		return nil, coordination.ErrWrongOwner
	}
	if p.services.RelTimeCoordinator == nil || !temporal.FeatureFlagsFromEnvironment().RelationshipTimeEnabled {
		return nil, errors.New("关系时间服务未启用")
	}
	var result json.RawMessage
	err := p.WithSourceRole(ctx, scope, func() error {
		return coordination.CommitCurrent(ctx, func() error {
			coordinator := p.services.RelTimeCoordinator.ForOwner(p.ownerID)
			if len(request.Settings) > 0 {
				var settings temporal.RelationshipTimeSettings
				if json.Unmarshal(request.Settings, &settings) != nil || settings.MaxMentionSentences < 0 || settings.MaxMentionSentences > 2 || settings.Sensitivity != "conservative" && settings.Sensitivity != "balanced" && settings.Sensitivity != "expressive" {
					return errors.New("关系时间设置格式无效")
				}
				settings.CharacterID = scope.RoleID
				current, err := coordinator.GetSettings(ctx, scope.RoleID)
				if err != nil {
					return err
				}
				if current != nil && current.UpdatedAt != settings.UpdatedAt || current == nil && settings.UpdatedAt != "" {
					return coordination.ErrResourceVersion
				}
				settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
				if err := coordinator.SaveSettings(ctx, &settings); err != nil {
					return err
				}
			}
			settings, err := coordinator.GetSettings(ctx, scope.RoleID)
			if err != nil {
				return err
			}
			if settings == nil {
				defaults := temporal.DefaultRelationshipTimeSettings(scope.RoleID)
				settings = &defaults
			}
			space := temporal.RelationshipOwnerSpace(p.legacySpaceID, p.ownerID)
			state, err := coordinator.GetState(ctx, space, scope.RoleID)
			if err != nil {
				return err
			}
			episodes, err := coordinator.ListReunionEpisodes(ctx, space, scope.RoleID, request.Limit)
			if err != nil {
				return err
			}
			var episode *temporal.ReunionEpisode
			if request.EpisodeID != "" {
				if len(request.EpisodeID) > 512 {
					return coordination.ErrPendingLimit
				}
				episode, err = coordinator.GetReunionEpisode(ctx, request.EpisodeID)
				if err != nil {
					return err
				}
				if episode == nil || episode.SpaceID != space || episode.CharacterID != scope.RoleID {
					return coordination.ErrWrongOwner
				}
			}
			result, err = json.Marshal(map[string]any{"episode": episode, "ownerId": p.ownerID, "roleId": scope.RoleID, "executionScope": scope, "settings": settings, "state": state, "episodes": episodes})
			return err
		})
	})
	return result, err
}
