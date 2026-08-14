// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) StorageBackups(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStorageBackups())
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
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.StorageImportUserData(body))
}

func (h *Handler) StorageImportAmitia(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		util.ErrorResponse(c, 400, "missing file", nil)
		return
	}
	defer file.Close()

	exportDir := filepath.Join("data", "exports")
	os.MkdirAll(exportDir, 0755)
	dst := filepath.Join(exportDir, header.Filename)
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

	result := h.service.StorageImportUserData(map[string]interface{}{"fileName": header.Filename})
	util.SuccessResponse(c, result)
}

func (h *Handler) StorageExportDownload(c *gin.Context) {
	filename := c.Param("filename")
	path := filepath.Join("data", "exports", filename)
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
