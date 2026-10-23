package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/pkg/app"
)

func registerChatImportRoutes(r *gin.RouterGroup, handler *Handler) {
	r.GET("/imports/batches", handler.ImportsBatches)
	r.GET("/imports/batches/:id", handler.ImportsBatchDetail)
	r.GET("/imports/batches/:id/summary", handler.ImportsBatchSummary)
	r.DELETE("/imports/batches/:id", handler.ImportsBatchDelete)
	r.POST("/imports/batches/:id/generate-summary", handler.ImportsBatchGenerateSummary)
	r.POST("/imports/parse-text", handler.ImportsParseText)
	r.POST("/imports/confirm", handler.ImportsConfirm)
}

func RegisterDeviceChatImportRoutes(r *gin.RouterGroup, ctx *app.AppContext, chatSvc chat.Service) {
	if r == nil || ctx == nil {
		return
	}
	svc := NewService(ctx, runtimeprofile.ProfileDeviceAgent)
	handler := NewHandler(svc, ctx.DB, chatSvc, nil, nil, nil, nil)
	registerChatImportRoutes(r, handler)
}
