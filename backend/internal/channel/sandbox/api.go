package sandbox

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	middlewaresecurity "github.com/u-ai/backend/internal/middleware/security"
)

func RegisterRoutes(group *gin.RouterGroup, service *Service) {
	if group == nil || service == nil {
		return
	}
	group.GET("/channels/sandbox-policies", func(c *gin.Context) {
		spaceID := actorSpaceID(c)
		if spaceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "需要登录后操作"})
			return
		}
		channelID := strings.TrimSpace(c.Query("channelId"))
		if channelID == "" {
			rows, err := service.List(spaceID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功", "data": gin.H{"items": rows}})
			return
		}
		policy, err := service.Get(spaceID, channelID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功", "data": policy})
	})
	group.PUT("/channels/sandbox-policies", func(c *gin.Context) {
		spaceID := actorSpaceID(c)
		if spaceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "需要登录后操作"})
			return
		}
		var body struct {
			ChannelID    string `json:"channelId"`
			Enabled      bool   `json:"enabled"`
			AllowNetwork *bool  `json:"allowNetwork"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		allowNetwork := true
		if body.AllowNetwork != nil {
			allowNetwork = *body.AllowNetwork
		}
		policy, err := service.Set(spaceID, body.ChannelID, body.Enabled, allowNetwork)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "操作成功", "data": policy})
	})
}

func (s *Service) List(spaceID string) ([]Policy, error) {
	if s == nil || s.db == nil {
		return []Policy{}, nil
	}
	rows := make([]Policy, 0, 8)
	if err := s.db.Where("space_id = ?", strings.TrimSpace(spaceID)).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func actorSpaceID(c *gin.Context) string {
	actor := middlewaresecurity.GetActor(c)
	if actor == nil {
		return ""
	}
	return strings.TrimSpace(actor.SpaceID.String())
}
