package knowledge

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

func RegisterKnowledgeRouter(api *gin.RouterGroup, service Service) {
	if api == nil || service == nil {
		return
	}
	h := NewHandler(service)
	g := api.Group("/knowledge")
	{
		g.GET("/sources", h.ListSources)
		g.POST("/sources", h.Upload)
		g.GET("/sources/:id", h.GetSource)
		g.GET("/sources/:id/outline", h.GetSourceOutline)
		g.POST("/sources/:id/revisions", h.UploadRevision)
		g.POST("/sources/:id/revisions/:revision/rollback", h.RollbackRevision)
		g.DELETE("/sources/:id/revisions/:revision", h.PurgeRevision)
		g.POST("/sources/:id/reindex", h.ReindexSource)
		g.DELETE("/sources/:id", h.DeleteSource)
		g.GET("/jobs", h.ListJobs)
		g.GET("/jobs/:id", h.GetJob)
		g.POST("/jobs/:id/retry", h.RetryJob)
		g.POST("/jobs/:id/cancel", h.CancelJob)
		g.POST("/search", h.Search)
		g.POST("/read", h.Read)
		g.GET("/runtime", h.GetRuntimeStatus)
		g.GET("/slo", h.GetSLOSummary)
		g.GET("/index-health", h.GetIndexOutboxHealth)
		g.GET("/vector-audit", h.AuditVectors)
		g.GET("/vector-orphans", h.AuditOrphanVectors)
		g.GET("/vector-orphans/page", h.AuditOrphanVectorPage)
		g.GET("/vector-orphans/audits", h.ListOrphanAuditSessions)
		g.GET("/vector-orphans/audits/:id/pages", h.ListOrphanAuditPageReceipts)
		g.GET("/vector-orphans/audits/:id/recheck", h.RecheckOrphanAuditLatest)
		g.GET("/vector-orphans/audits/:id/rechecks", h.ListOrphanAuditRechecks)
		g.GET("/vector-orphans/audits/:id/recovery-reviews", security.SharedCoreAdminOnly(), h.ListRecoveryReviews)
		g.POST("/vector-orphans/audits/:id/recovery-reviews", security.SharedCoreAdminOnly(), h.CreateRecoveryReview)
		g.POST("/vector-orphans/recovery-reviews/:id/approve", security.SharedCoreAdminOnly(), h.ApproveRecoveryReview)
		g.POST("/vector-orphans/recovery-reviews/:id/revoke", security.SharedCoreAdminOnly(), h.RevokeRecoveryReview)
		g.GET("/vector-orphans/recovery-reviews/:id/events", security.SharedCoreAdminOnly(), h.ListRecoveryReviewEvents)
		g.GET("/vector-orphans/recovery-reviews/:id/plan", security.SharedCoreAdminOnly(), h.GetRecoveryReviewPlan)
		g.GET("/traces", h.ListRetrievalTraces)
		g.POST("/benchmark", h.RunBenchmark)
		g.GET("/benchmark/runs", h.ListBenchmarkRuns)
		g.POST("/reconcile", h.Reconcile)
		g.POST("/gc", h.GarbageCollect)
	}
}
