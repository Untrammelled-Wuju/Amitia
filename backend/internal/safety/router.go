package safety

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterSafetyRouter(r *gin.RouterGroup, db *gorm.DB) {
	h := NewHandler(db)
	r.GET("/safety/bdi-config", h.GetBdiConfig)
	r.PUT("/safety/bdi-config", h.PutBdiConfig)
	r.GET("/safety/audit-logs", h.GetAuditLogs)
	r.GET("/safety/config", h.GetConfig)
	r.PUT("/safety/config", h.PutConfig)
}
