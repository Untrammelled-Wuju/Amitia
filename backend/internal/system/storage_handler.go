// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) StorageBackup(c *gin.Context) { util.SuccessResponse(c, h.service.StorageBackup()) }

func (h *Handler) StorageBackupEncrypted(c *gin.Context) {
	util.SuccessResponse(c, h.service.StorageBackupEncrypted())
}

func (h *Handler) StorageBackups(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStorageBackups())
}

func (h *Handler) StorageDeleteBackup(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteStorageBackup(c.Param("name")))
}

func (h *Handler) StorageDeleteAll(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteAllStorage())
}

func (h *Handler) StorageRestore(c *gin.Context) {
	util.SuccessResponse(c, h.service.StorageRestore(c.Param("name")))
}

func (h *Handler) StorageRestoreEncrypted(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.StorageRestoreEncrypted(body))
}

func (h *Handler) StorageRestoreVerify(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.StorageRestoreVerify(body))
}

func (h *Handler) StorageExportUserData(c *gin.Context) {
	util.SuccessResponse(c, h.service.StorageExportUserData())
}

func (h *Handler) StorageImportUserData(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.StorageImportUserData(body))
}

func (h *Handler) StorageInfo(c *gin.Context) { util.SuccessResponse(c, h.service.GetStorageInfo()) }

func (h *Handler) StorageMigrations(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStorageMigrations())
}

func (h *Handler) StorageMigrationsCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.CheckStorageMigrations())
}
