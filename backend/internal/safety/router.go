package safety

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
	"gorm.io/gorm"
)

func RegisterSafetyRouter(r *gin.RouterGroup, db *gorm.DB) {
	h := NewHandler(db)
	r.GET("/safety/bdi-config", security.SharedCoreAdminOnly(), h.GetBdiConfig)
	r.PUT("/safety/bdi-config", security.SharedCoreAdminOnly(), h.PutBdiConfig)
	r.GET("/safety/audit-logs", security.SharedCoreAdminOnly(), h.GetAuditLogs)
	r.GET("/safety/config", security.SharedCoreAdminOnly(), h.GetConfig)
	r.PUT("/safety/config", security.SharedCoreAdminOnly(), h.PutConfig)
}
