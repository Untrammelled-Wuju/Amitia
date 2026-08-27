// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package readiness

import "github.com/gin-gonic/gin"

// RegisterRouter exposes the production readiness service as the single source of
// truth. The router must never construct a second, partial readiness graph.
func RegisterRouter(r *gin.RouterGroup, svc *ReadinessService) {
	r.GET("/desktop-pets/readiness/complete", func(c *gin.Context) {
		if svc == nil {
			c.JSON(503, gin.H{
				"code": 503,
				"msg":  "readiness service unavailable",
				"data": gin.H{
					"overallStatus": StatusBlocked,
					"blockingCount": 1,
				},
			})
			return
		}

		snapshot := svc.Snapshot()
		httpStatus := 200
		if snapshot.OverallStatus == StatusBlocked {
			httpStatus = 503
		}
		c.JSON(httpStatus, gin.H{
			"code": httpStatus,
			"msg":  string(snapshot.OverallStatus),
			"data": snapshot,
		})
	})

	r.GET("/desktop-pets/readiness/startup", func(c *gin.Context) {
		if svc == nil {
			c.JSON(503, gin.H{
				"code": 503,
				"msg":  "system not ready",
				"data": gin.H{
					"ready":         false,
					"overallStatus": StatusBlocked,
					"blocking":      []string{"readiness_service"},
				},
			})
			return
		}

		snapshot := svc.Snapshot()
		ready := snapshot.BlockingCount == 0
		result := gin.H{
			"ready":         ready,
			"overallStatus": snapshot.OverallStatus,
			"timestamp":     snapshot.Timestamp,
		}
		if !ready {
			blocking := make([]string, 0)
			for name, check := range snapshot.Checks {
				if check.Status == StatusBlocked && check.Required {
					blocking = append(blocking, name)
				}
			}
			result["blocking"] = blocking
			c.JSON(503, gin.H{
				"code": 503,
				"msg":  "system not ready",
				"data": result,
			})
			return
		}
		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "system is ready",
			"data": result,
		})
	})
}
