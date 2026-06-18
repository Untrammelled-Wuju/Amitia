package episodic

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterEpisodicRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx, nil)
	handler := NewHandler(svc)

	r.GET("/episodic", handler.List)
	r.POST("/episodic", handler.Create)
	r.DELETE("/episodic/:id", handler.Delete)
	r.GET("/episodic/by-user", handler.GetByUserID)
	r.GET("/episodic/:id/detail", handler.GetDetail)
	r.POST("/episodic/extract", handler.Extract)
	r.GET("/episodic/system-prompt", handler.SystemPrompt)
}