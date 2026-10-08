// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) StorageBackups(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStorageBackups())
}

func (h *Handler) StorageCreateBackup(c *gin.Context) {
	result := h.service.CreatePhysicalSafetySnapshot()
	if ok, exists := result["ok"].(bool); exists && !ok {
		util.ErrorResponse(c, http.StatusInternalServerError, "创建备份失败", result)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) StorageRestoreBackup(c *gin.Context) {
	name := filepath.Base(c.Param("name"))
	if name == "." || name == "" || name != c.Param("name") {
		util.ErrorResponse(c, http.StatusBadRequest, "无效备份名称", nil)
		return
	}
	result := h.service.RestorePhysicalSafetySnapshot(name)
	if ok, exists := result["ok"].(bool); exists && !ok {
		util.ErrorResponse(c, http.StatusInternalServerError, "恢复备份失败", result)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) StorageDeleteBackup(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteStorageBackup(c.Param("name")))
}

func (h *Handler) StorageDeleteAll(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteAllStorage())
}

func (h *Handler) StorageExportAmitia(c *gin.Context) {
	var body struct {
		Scope       string `json:"scope"`
		CharacterID string `json:"characterId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || (body.Scope != "all" && body.Scope != "character") {
		util.ErrorResponse(c, 400, "scope must be 'all' or 'character'", nil)
		return
	}
	if body.Scope == "character" && body.CharacterID == "" {
		util.ErrorResponse(c, 400, "characterId is required when scope is 'character'", nil)
		return
	}
	util.SuccessResponse(c, h.service.StorageExportAmitia(body.Scope, body.CharacterID))
}

func (h *Handler) StorageImportUserData(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	h.respondStorageImport(c, h.storageImport(c.Request.Context(), body))
}

func (h *Handler) StorageImportAmitia(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		util.ErrorResponse(c, 400, "missing file", nil)
		return
	}
	defer file.Close()

	info := h.service.GetStorageInfo()
	dataDir, _ := info["path"].(string)
	if strings.TrimSpace(dataDir) == "" {
		util.ErrorResponse(c, 500, "storage data directory unavailable", nil)
		return
	}
	exportDir := filepath.Join(dataDir, "exports")
	os.MkdirAll(exportDir, 0755)
	dst := filepath.Join(exportDir, filepath.Base(header.Filename))
	out, err := os.Create(dst)
	if err != nil {
		util.ErrorResponse(c, 500, err.Error(), nil)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		util.ErrorResponse(c, 500, err.Error(), nil)
		return
	}
	out.Close()

	result := h.storageImport(c.Request.Context(), map[string]interface{}{"fileName": filepath.Base(header.Filename)})
	h.respondStorageImport(c, result)
}

func (h *Handler) storageImport(ctx context.Context, body map[string]interface{}) map[string]interface{} {
	if scoped, ok := h.service.(interface {
		StorageImportUserDataContext(context.Context, map[string]interface{}) map[string]interface{}
	}); ok {
		return scoped.StorageImportUserDataContext(ctx, body)
	}
	return map[string]interface{}{"imported": false, "error": "当前存储服务不支持可撤销的配置导入"}
}

func (h *Handler) respondStorageImport(c *gin.Context, result map[string]interface{}) {
	if imported, _ := result["imported"].(bool); !imported {
		message, _ := result["error"].(string)
		util.ErrorResponse(c, 409, message, result)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) StorageExportDownload(c *gin.Context) {
	filename := filepath.Base(c.Param("filename"))
	if filename == "." || filename == "" || filename != c.Param("filename") {
		util.ErrorResponse(c, http.StatusBadRequest, "invalid filename", nil)
		return
	}
	info := h.service.GetStorageInfo()
	dataDir, _ := info["path"].(string)
	if strings.TrimSpace(dataDir) == "" {
		util.ErrorResponse(c, 500, "storage data directory unavailable", nil)
		return
	}
	path := filepath.Join(dataDir, "exports", filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "file not found")
		return
	}
	c.FileAttachment(path, filename)
}

func (h *Handler) StorageInfo(c *gin.Context) { util.SuccessResponse(c, h.service.GetStorageInfo()) }

func (h *Handler) StorageMigrations(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStorageMigrations())
}

func (h *Handler) StorageMigrationsCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.CheckStorageMigrations())
}
