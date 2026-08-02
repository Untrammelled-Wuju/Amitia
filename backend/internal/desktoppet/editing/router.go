package editing

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterEditingRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	dataDir := config.AppCfg.Storage.DataDir
	repo := NewRepository(ctx.DB)
	assetStore := NewAssetStore(dataDir, repo)
	genAdapter := NewGenerationAdapter(ctx)
	procAdapter := NewProcessingAdapter(ctx)
	qualAdapter := NewQualityAdapter(ctx)
	svc := NewService(repo, assetStore, genAdapter, procAdapter, qualAdapter, ctx.DB, dataDir)
	registry := security.NewPathRootRegistry()
	_ = registry.Register(dataDir)
	responder := security.NewSafeArtifactResponder(registry)
	registerRoutes(r, svc, responder)
}

func RegisterEditingRouterWithService(r *gin.RouterGroup, svc Service) {
	registry := security.NewPathRootRegistry()
	_ = registry.Register(config.AppCfg.Storage.DataDir)
	responder := security.NewSafeArtifactResponder(registry)
	registerRoutes(r, svc, responder)
}

func registerRoutes(r *gin.RouterGroup, svc Service, responder *security.SafeArtifactResponder) {
	handler := NewHandler(svc, responder)
	g := r.Group("/desktop-pets")
	{
		g.GET("/processing-tasks/:processingTaskId/actions/:actionKey/revisions", handler.ListRevisions)
		g.GET("/processing-tasks/:processingTaskId/actions/:actionKey/revisions/:revisionId", handler.GetRevision)
		g.GET("/processing-tasks/:processingTaskId/actions/:actionKey/active-revision", handler.GetActiveRevision)
		g.POST("/processing-tasks/:processingTaskId/actions/:actionKey/active-revision", handler.ActivateRevision)
		g.GET("/processing-tasks/:processingTaskId/actions/:actionKey/edit-summary", handler.GetActionEditSummary)
		g.POST("/processing-tasks/:processingTaskId/actions/:actionKey/import-legacy", handler.ImportLegacyRevision)

		g.GET("/action-streams", handler.ListActionStreams)
		g.GET("/action-streams/:streamId/revisions", handler.ListRevisionsByStream)
		g.GET("/action-streams/:streamId/active-revision", handler.GetActiveRevisionByStream)

		g.GET("/revisions/:revisionId/frames/:frameId/image", handler.GetFrameImage)
		g.GET("/revisions/:revisionId/frames/:frameId/thumbnail", handler.GetFrameThumbnail)
		g.GET("/revisions/:revisionId/preview-manifest", handler.GetPreviewManifest)
		g.POST("/revisions/:revisionId/quality-evaluations", handler.TriggerQualityEvaluation)
		g.GET("/revisions/:revisionId/quality-evaluations/latest", handler.GetLatestQualityEvaluation)

		g.POST("/processing-tasks/:processingTaskId/actions/:actionKey/edit-sessions", handler.CreateSession)
		g.GET("/edit-sessions/:sessionId", handler.GetSession)
		g.POST("/edit-sessions/:sessionId/operations", handler.ApplyOperation)
		g.POST("/edit-sessions/:sessionId/undo", handler.Undo)
		g.POST("/edit-sessions/:sessionId/redo", handler.Redo)
		g.POST("/edit-sessions/:sessionId/checkpoints", handler.CreateCheckpoint)
		g.POST("/edit-sessions/:sessionId/commit", handler.CommitSession)
		g.POST("/edit-sessions/:sessionId/abandon", handler.AbandonSession)
		g.GET("/edit-sessions/:sessionId/events", handler.SessionEvents)

		g.POST("/edit-sessions/:sessionId/regeneration-jobs", handler.CreateRegenerationJob)
		g.GET("/edit-sessions/:sessionId/regeneration-jobs/:jobId", handler.GetRegenerationJob)
		g.POST("/edit-sessions/:sessionId/regeneration-jobs/:jobId/cancel", handler.CancelRegenerationJob)
		g.GET("/regeneration-jobs/:jobId", handler.GetRegenerationJobByID)
		g.GET("/regeneration-jobs", handler.ListRegenerationJobs)
		g.POST("/edit-sessions/:sessionId/candidates/:candidateId/accept", handler.AcceptCandidate)
		g.POST("/edit-sessions/:sessionId/candidates/:candidateId/reject", handler.RejectCandidate)

		g.POST("/edit-sessions/:sessionId/upload-candidates", handler.UploadCandidate)

		g.POST("/edit-sessions/:sessionId/frames/:frameId/background-patches", handler.ApplyBackgroundPatch)
		g.DELETE("/edit-sessions/:sessionId/frames/:frameId/background-patches", handler.ResetBackgroundPatch)
		g.POST("/edit-sessions/:sessionId/frames/:frameId/anchor", handler.SetFrameAnchor)
		g.POST("/edit-sessions/:sessionId/anchors/batch-offset", handler.BatchOffsetAnchors)
		g.POST("/edit-sessions/:sessionId/anchors/reset", handler.ResetAnchors)
	}
}
