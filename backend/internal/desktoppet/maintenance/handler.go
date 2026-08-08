// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package maintenance

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet/migration"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type BackupService interface {
	CreateBackup(ctx context.Context) (interface{}, error)
}

type ExportService interface {
	CreateExport(ctx context.Context) (interface{}, error)
}

type DoctorService interface {
	Report(ctx context.Context) (interface{}, error)
}

type Handler struct {
	migrationRunner *migration.Runner
	backupSvc       BackupService
	exportSvc       ExportService
	doctorSvc       DoctorService
}

func NewHandler(runner *migration.Runner, backup BackupService, exportSvc ExportService, doctor DoctorService) *Handler {
	return &Handler{
		migrationRunner: runner,
		backupSvc:       backup,
		exportSvc:       exportSvc,
		doctorSvc:       doctor,
	}
}

func (h *Handler) Doctor(c *gin.Context) {
	if h.doctorSvc == nil {
		util.ErrorResponse(c, response.InternalError, "诊断服务不可用", gin.H{"errorCode": "MAINTENANCE_DOCTOR_UNAVAILABLE"})
		return
	}
	report, err := h.doctorSvc.Report(c.Request.Context())
	if err != nil {
		writeMaintenanceError(c, &maintenanceError{Code: "DOCTOR_REPORT_FAILED", Message: "诊断报告生成失败: " + err.Error(), httpCode: response.InternalError})
		return
	}
	util.SuccessResponse(c, report)
}

func (h *Handler) CreateBackup(c *gin.Context) {
	if h.backupSvc == nil {
		util.ErrorResponse(c, response.InternalError, "备份服务不可用", gin.H{"errorCode": "MAINTENANCE_BACKUP_UNAVAILABLE"})
		return
	}
	result, err := h.backupSvc.CreateBackup(c.Request.Context())
	if err != nil {
		writeMaintenanceError(c, &maintenanceError{Code: "BACKUP_FAILED", Message: "备份创建失败: " + err.Error(), httpCode: response.InternalError})
		return
	}
	util.SuccessMsgResponse(c, "备份已启动", gin.H{"result": result})
}

func (h *Handler) CreateExport(c *gin.Context) {
	if h.exportSvc == nil {
		util.ErrorResponse(c, response.InternalError, "导出服务不可用", gin.H{"errorCode": "MAINTENANCE_EXPORT_UNAVAILABLE"})
		return
	}
	result, err := h.exportSvc.CreateExport(c.Request.Context())
	if err != nil {
		writeMaintenanceError(c, &maintenanceError{Code: "EXPORT_FAILED", Message: "导出创建失败: " + err.Error(), httpCode: response.InternalError})
		return
	}
	util.SuccessMsgResponse(c, "导出已启动", gin.H{"result": result})
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
	var req struct {
		OperationID string `json:"operationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"errorCode": "MAINTENANCE_INVALID_PARAMS"})
		return
	}
	if req.OperationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "operationId 不能为空", gin.H{"errorCode": "CUTOVER_OPERATION_REQUIRED"})
		return
	}
	if err := h.migrationRunner.RequestCutover(c.Request.Context(), req.OperationID, "read"); err != nil {
		writeMaintenanceError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "读切换已请求", gin.H{"direction": "read", "operationId": req.OperationID})
}

func (h *Handler) CutoverWrite(c *gin.Context) {
	var req struct {
		OperationID string `json:"operationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"errorCode": "MAINTENANCE_INVALID_PARAMS"})
		return
	}
	if req.OperationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "operationId 不能为空", gin.H{"errorCode": "CUTOVER_OPERATION_REQUIRED"})
		return
	}
	if err := h.migrationRunner.RequestCutover(c.Request.Context(), req.OperationID, "write"); err != nil {
		writeMaintenanceError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "写切换已请求", gin.H{"direction": "write", "operationId": req.OperationID})
}

func writeMaintenanceError(c *gin.Context, err error) {
	var me *maintenanceError
	if errors.As(err, &me) {
		util.ErrorResponse(c, me.httpCode, me.Message, gin.H{"errorCode": me.Code})
		return
	}
	var re *migration.RunnerError
	if errors.As(err, &re) {
		util.ErrorResponse(c, response.InternalError, re.Message, gin.H{"errorCode": re.Code})
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
