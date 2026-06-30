// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterMemoryRouter(r *gin.RouterGroup, ctx *app.AppContext, graphSvc graph.Service) *Handler {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx, graphSvc)
	handler := NewHandler(svc)

	r.DELETE("/memories", handler.DeleteAll)
	r.GET("/memories", handler.List)
	r.POST("/memories", handler.Create)
	r.PUT("/memories/:id", handler.Update)
	r.DELETE("/memories/:id", handler.Delete)
	r.POST("/memories/search", handler.Search)
	r.POST("/memories/vector-search", handler.VectorSearch)
	r.POST("/memories/hybrid-search", handler.HybridSearch)
	r.POST("/memories/:id/use", handler.RecordUse)
	r.GET("/memories/vector-status", handler.VectorStatus)
	r.GET("/memories/timeline", handler.Timeline)
	r.POST("/memories/check-conflict", handler.CheckConflict)
	r.POST("/memories/resolve-conflict", handler.ResolveConflict)
	r.POST("/memories/extract-candidates", handler.ExtractCandidates)
	r.POST("/memories/rebuild-index", handler.RebuildIndex)
	r.POST("/memories/rebuild-embeddings", handler.RebuildEmbeddings)

	r.POST("/memories/batch-verify", handler.BatchVerify)
	r.POST("/memories/batch-importance", handler.BatchSetImportance)
	r.GET("/memories/ranked", handler.GetRankedMemories)

	r.GET("/memory-candidates", handler.ListCandidates)
	r.PUT("/memory-candidates/:id", handler.UpdateCandidate)
	r.DELETE("/memory-candidates/:id", handler.DeleteCandidate)
	r.POST("/memory-candidates/:id/accept", handler.AcceptCandidate)
	r.POST("/memory-candidates/:id/reject", handler.RejectCandidate)
	r.POST("/memory-candidates/batch-accept", handler.BatchAcceptCandidates)
	r.POST("/memory-candidates/generate", handler.GenerateCandidates)

	return handler
}
