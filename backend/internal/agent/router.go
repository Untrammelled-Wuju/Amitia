package agent

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterAgentRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	memRepo := memory.NewRepository(ctx); memSvc := memory.NewService(memRepo, ctx); chatSvc := chat.NewService(chat.NewRepository(ctx), ctx, memSvc)
	svc := NewService(ctx, chatSvc)
	handler := NewHandler(svc)

	r.POST("/agent/test", handler.Test)
	r.GET("/agent/context-preview", handler.ContextPreview)
	r.POST("/agent/webhook", handler.Webhook)
}
