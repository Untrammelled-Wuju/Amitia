package emote

import "github.com/gin-gonic/gin"

func RegisterRouter(r *gin.RouterGroup, service *Service) {
	h := NewHandler(service)
	r.GET("/emote-groups", h.Groups)
	r.POST("/emote-groups", h.CreateGroup)
	r.PUT("/emote-groups/:id", h.UpdateGroup)
	r.DELETE("/emote-groups/:id", h.DeleteGroup)
	r.POST("/emote-groups/reorder", h.ReorderGroups)
	r.POST("/emote-groups/:id/emotes", h.AddGroupEmotes)
	r.DELETE("/emote-groups/:id/emotes/:emoteId", h.RemoveGroupEmote)
	r.GET("/emotes", h.List)
	r.GET("/emotes/:id", h.Get)
	r.POST("/emotes/upload", h.Upload)
	r.POST("/emotes/batch-upload", h.BatchUpload)
	r.PUT("/emotes/:id", h.Update)
	r.DELETE("/emotes/:id", h.Delete)
	r.POST("/emotes/batch-update", h.BatchUpdate)
	r.POST("/emotes/:id/groups", h.SetGroups)
	r.POST("/emotes/:id/role-scope", h.SetRoleScope)
	r.POST("/chat/send-emote", h.ManualSend)
	r.GET("/characters/:id/emote-settings", h.GetSettings)
	r.PUT("/characters/:id/emote-settings", h.SaveSettings)
}
