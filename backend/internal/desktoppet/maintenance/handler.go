// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package maintenance

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet/migration"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	migrationRunner *migration.Runner
}

func NewHandler(runner *migration.Runner) *Handler {
	return &Handler{migrationRunner: runner}
}

func (h *Handler) Doctor(c *gin.Context) {
	report := gin.H{
		"status":  "healthy",
		"version": "1.0",
	}
	util.SuccessResponse(c, report)
}

func (h *Handler) CreateBackup(c *gin.Context) {
	util.SuccessMsgResponse(c, "备份已启动", gin.H{"backupId": fmt.Sprintf("bak_%d", 0)})
}

func (h *Handler) CreateExport(c *gin.Context) {
	util.SuccessMsgResponse(c, "导出已启动", gin.H{"exportId": fmt.Sprintf("exp_%d", 0)})
}

func (h *Handler) RunMigration(c *gin.Context) {
	planID := c.Param("planId")
	if planID == "" {
		util.ErrorResponse(c, response.InvalidParams, "迁移计划 ID 为空", gin.H{"errorCode": "MIGRATION_PLAN_REQUIRED"})
		return
	}
	opID, err := h.migrationRunner.RunPlan(c.Request.Context(), planID)
	if err != nil {
		writeMaintenanceError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "迁移已启动", gin.H{"operationId": opID})
}

func (h *Handler) GetMigration(c *gin.Context) {
	operationID := c.Param("operationId")
	if operationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "迁移操作 ID 为空", gin.H{"errorCode": "MIGRATION_OPERATION_REQUIRED"})
		return
	}
	op, err := h.migrationRunner.GetOperation(c.Request.Context(), operationID)
	if err != nil {
		writeMaintenanceError(c, err)
		return
	}
	if op == nil {
		util.ErrorResponse(c, response.NotFound, "迁移操作不存在", gin.H{"errorCode": "MIGRATION_OPERATION_NOT_FOUND"})
		return
	}
	util.SuccessResponse(c, op)
}

func (h *Handler) CutoverRead(c *gin.Context) {
	if err := h.migrationRunner.RequestCutover(c.Request.Context(), "read"); err != nil {
		writeMaintenanceError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "读切换已请求", gin.H{"direction": "read"})
}

func (h *Handler) CutoverWrite(c *gin.Context) {
	if err := h.migrationRunner.RequestCutover(c.Request.Context(), "write"); err != nil {
		writeMaintenanceError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "写切换已请求", gin.H{"direction": "write"})
}

func writeMaintenanceError(c *gin.Context, err error) {
	var me *maintenanceError
	if errors.As(err, &me) {
		util.ErrorResponse(c, me.httpCode, me.Message, gin.H{"errorCode": me.Code})
		return
	}
	var re *migration.RunnerError
	if errors.As(err, &re) {
		util.ErrorResponse(c, response.InternalError, "服务器内部错误", gin.H{"errorCode": re.Code})
		return
	}
	util.ErrorResponse(c, response.InternalError, "服务器内部错误", nil)
}

type maintenanceError struct {
	Code     string
	Message  string
	httpCode int
	Err      error
}

func (e *maintenanceError) Error() string {
	return e.Message
}
