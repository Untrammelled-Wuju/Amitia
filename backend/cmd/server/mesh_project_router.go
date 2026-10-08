package main

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerMeshProjectRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	mesh.GET("/projects", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		cursor := c.Query("cursor")
		historicalRole, historicalCursor := c.Query("historicalRoleId"), c.Query("historicalCursor")
		if len(cursor) > 4096 || len(historicalCursor) > 4096 || len(historicalRole) > 512 || historicalCursor != "" && historicalRole == "" {
			c.JSON(400, gin.H{"message": "项目分页参数无效"})
			return
		}
		result, err := services.OwnedBusiness.Query(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: c.Query("targetDeviceId"), RoleID: c.Query("characterId")}, coordination.DataQuery{Management: true, ResourceKind: "project", HistoricalRoleID: historicalRole, HistoricalCursor: historicalCursor, Cursor: cursor, Limit: 128})
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		collections := [2][]gin.H{{}, {}}
		for index, snapshot := range []*coordination.DataSnapshot{&result.Snapshot, result.HistoricalSnapshot} {
			if snapshot == nil {
				continue
			}
			for _, row := range snapshot.Resources {
				if row.Kind != "project" {
					continue
				}
				var project business.Project
				if json.Unmarshal(row.Body, &project) != nil || project.ID != row.ID {
					c.JSON(503, gin.H{"message": "项目数据格式无效"})
					return
				}
				collections[index] = append(collections[index], gin.H{"id": project.ID, "title": project.Title, "createdAt": project.CreatedAt, "updatedAt": project.UpdatedAt, "pinnedAt": project.PinnedAt, "revision": row.Revision, "ownerId": row.OwnerID, "roleId": row.RoleID, "readOnly": index == 1})
			}
		}
		nextHistoricalCursor := ""
		if result.HistoricalSnapshot != nil {
			nextHistoricalCursor = result.HistoricalSnapshot.NextCursors["project"]
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"projects": collections[0], "historicalProjects": collections[1], "executionScope": result.Scope, "nextCursor": result.Snapshot.NextCursors["project"], "nextHistoricalCursor": nextHistoricalCursor}})
	})
	mesh.POST("/projects", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		var payload struct {
			RequestID      string                       `json:"requestId"`
			RoleID         string                       `json:"characterId"`
			TargetDeviceID string                       `json:"targetDeviceId"`
			Title          string                       `json:"title"`
			ExpectedScope  *coordination.ExecutionScope `json:"expectedExecutionScope"`
		}
		if c.ShouldBindJSON(&payload) != nil {
			c.JSON(400, gin.H{"message": "项目参数无效"})
			return
		}
		result, err := services.OwnedBusiness.CreateProject(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: payload.TargetDeviceID, RoleID: payload.RoleID, RequestID: payload.RequestID, ExpectedScope: payload.ExpectedScope}, payload.Title)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": result})
	})
}
