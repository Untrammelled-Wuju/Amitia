package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerMeshMemoryManagementRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	read := func(candidates bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			actor := security.GetActor(c)
			if actor == nil || actor.DeviceID == "" {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			limit := 128
			if value := c.Query("limit"); value != "" {
				var err error
				limit, err = strconv.Atoi(value)
				if err != nil || limit < 1 || limit > 128 {
					c.JSON(400, gin.H{"message": "记忆分页参数无效"})
					return
				}
			}
			request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: c.Query("targetDeviceId"), RoleID: c.Query("characterId"), RequestID: uuid.NewString()}
			query := business.MemoryManagementQuery{Query: c.Query("query"), Mode: c.Query("mode"), MemoryType: c.Query("memoryType"), Source: c.Query("source"), Sort: c.Query("sort"), Cursor: c.Query("cursor"), Limit: limit}
			var result business.MemoryManagementResponse
			var err error
			if candidates {
				result, err = services.OwnedBusiness.ListOwnedMemoryCandidates(c.Request.Context(), request, query)
			} else {
				result, err = services.OwnedBusiness.SearchOwnedMemories(c.Request.Context(), request, query)
			}
			writeMeshMemoryManagement(c, result, err)
		}
	}
	manage := func(candidates bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			actor := security.GetActor(c)
			if actor == nil || actor.DeviceID == "" {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			payload, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 320<<10))
			var metadata struct {
				RequestID          string                       `json:"requestId"`
				RoleID             string                       `json:"characterId"`
				HistoricalRoleID   string                       `json:"historicalRoleId"`
				TargetDeviceID     string                       `json:"targetDeviceId"`
				ExpectedScope      *coordination.ExecutionScope `json:"expectedExecutionScope"`
				ConversationOrigin *business.ConversationOrigin `json:"conversationOrigin"`
			}
			if err != nil || json.Unmarshal(payload, &metadata) != nil || metadata.ExpectedScope == nil {
				c.JSON(400, gin.H{"message": "记忆操作参数无效或缺少原服务范围"})
				return
			}
			if _, err := uuid.Parse(metadata.RequestID); err != nil {
				c.JSON(400, gin.H{"message": "记忆操作请求编号无效"})
				return
			}
			request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: metadata.TargetDeviceID, RoleID: metadata.RoleID, HistoricalRoleID: metadata.HistoricalRoleID, RequestID: metadata.RequestID, ExpectedScope: metadata.ExpectedScope, ConversationOrigin: metadata.ConversationOrigin}
			var result business.MemoryManagementResponse
			if candidates {
				var input business.MemoryCandidateInput
				if json.Unmarshal(payload, &input) != nil {
					c.JSON(400, gin.H{"message": "候选记忆参数无效"})
					return
				}
				result, err = services.OwnedBusiness.ManageOwnedMemoryCandidates(c.Request.Context(), request, input)
			} else {
				var input business.MemoryManagementInput
				if json.Unmarshal(payload, &input) != nil {
					c.JSON(400, gin.H{"message": "记忆管理参数无效"})
					return
				}
				result, err = services.OwnedBusiness.ManageOwnedMemory(c.Request.Context(), request, input)
			}
			writeMeshMemoryManagement(c, result, err)
		}
	}
	mesh.GET("/memories", read(false))
	mesh.POST("/memories/manage", manage(false))
	mesh.GET("/memory-candidates", read(true))
	mesh.POST("/memory-candidates/manage", manage(true))
}

func writeMeshMemoryManagement(c *gin.Context, result business.MemoryManagementResponse, err error) {
	if err != nil {
		var conflict business.MemoryManagementConflict
		if errors.As(err, &conflict) {
			c.JSON(409, gin.H{"message": err.Error(), "conflicts": conflict.Resources})
		} else {
			c.JSON(409, gin.H{"message": err.Error()})
		}
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": result})
}
