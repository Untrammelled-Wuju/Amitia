// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) ImportsBatches(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetImportsBatches())
}

func (h *Handler) ImportsBatchDetail(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetImportsBatchDetail(c.Param("id")))
}

func (h *Handler) ImportsBatchSummary(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetImportsBatchSummary(c.Param("id")))
}

func (h *Handler) ImportsBatchMemoryCandidates(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetImportsBatchMemoryCandidates(c.Param("id")))
}

func (h *Handler) ImportsBatchDelete(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteImportsBatch(c.Param("id")))
}

func (h *Handler) ImportsBatchGenerateSummary(c *gin.Context) {
	util.SuccessResponse(c, h.service.GenerateImportsBatchSummary(c.Param("id")))
}

func (h *Handler) ImportsBatchConfirmMemories(c *gin.Context) {
	util.SuccessResponse(c, h.service.ConfirmImportsBatchMemories(c.Param("id")))
}

func (h *Handler) ImportsUpload(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UploadImports(body))
}

func (h *Handler) ImportsParseText(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ParseImportsText(body))
}

func (h *Handler) ImportsConfirm(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ConfirmImports(body))
}

func (h *Handler) ImportData(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ImportData(body))
}
