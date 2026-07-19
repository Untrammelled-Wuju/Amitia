// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
)

func (h *Handler) ImportsBatches(c *gin.Context) {
	h.GetImportsBatches(c)
}

func (h *Handler) ImportsBatchDetail(c *gin.Context) {
	h.GetImportsBatchDetail(c)
}

func (h *Handler) ImportsBatchSummary(c *gin.Context) {
	h.GetImportsBatchSummary(c)
}

func (h *Handler) ImportsBatchMemoryCandidates(c *gin.Context) {
	h.GetImportsBatchMemoryCandidates(c)
}

func (h *Handler) ImportsBatchDelete(c *gin.Context) {
	h.DeleteImportsBatch(c)
}

func (h *Handler) ImportsBatchGenerateSummary(c *gin.Context) {
	h.GenerateImportsBatchSummary(c)
}

func (h *Handler) ImportsBatchConfirmMemories(c *gin.Context) {
	h.ConfirmImportsBatchMemories(c)
}

func (h *Handler) ImportsUpload(c *gin.Context) {
	h.UploadImports(c)
}

func (h *Handler) ImportsParseText(c *gin.Context) {
	h.ParseImportsText(c)
}

func (h *Handler) ImportsConfirm(c *gin.Context) {
	h.ConfirmImports(c)
}

func (h *Handler) ImportData(c *gin.Context) {
	h.DoImportData(c)
}
