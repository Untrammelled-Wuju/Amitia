package emote

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

func RegisterRouter(r *gin.RouterGroup, service *Service) {
	h := NewHandler(service)
	r.GET("/emote-groups", h.Groups)
	r.POST("/emote-groups", security.SharedCoreAdminOnly(), h.CreateGroup)
	r.PUT("/emote-groups/:id", security.SharedCoreAdminOnly(), h.UpdateGroup)
	r.DELETE("/emote-groups/:id", security.SharedCoreAdminOnly(), h.DeleteGroup)
	r.POST("/emote-groups/reorder", security.SharedCoreAdminOnly(), h.ReorderGroups)
	r.POST("/emote-groups/:id/emotes", security.SharedCoreAdminOnly(), h.AddGroupEmotes)
	r.DELETE("/emote-groups/:id/emotes/:emoteId", security.SharedCoreAdminOnly(), h.RemoveGroupEmote)
	r.GET("/emotes", h.List)
	r.GET("/emotes/:id", h.Get)
	r.POST("/emotes/upload", security.SharedCoreAdminOnly(), h.Upload)
	r.POST("/emotes/batch-upload", security.SharedCoreAdminOnly(), h.BatchUpload)
	r.PUT("/emotes/:id", security.SharedCoreAdminOnly(), h.Update)
	r.DELETE("/emotes/:id", security.SharedCoreAdminOnly(), h.Delete)
	r.POST("/emotes/batch-update", security.SharedCoreAdminOnly(), h.BatchUpdate)
	r.POST("/emotes/:id/groups", security.SharedCoreAdminOnly(), h.SetGroups)
	r.POST("/emotes/:id/role-scope", security.SharedCoreAdminOnly(), h.SetRoleScope)
	r.POST("/chat/send-emote", h.ManualSend)
	r.GET("/characters/:id/emote-settings", h.GetSettings)
	r.PUT("/characters/:id/emote-settings", h.SaveSettings)
}
