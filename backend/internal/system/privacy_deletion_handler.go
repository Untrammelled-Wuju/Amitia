package system

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

func (h *Handler) privacyDeletionOperation(c *gin.Context, operation string, input coordination.PrivacyDeletionRequest, id string) (map[string]any, error) {
	if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
		return nil, err
	}
	port, ok := h.memoryConsumer.(business.PrivacyDeletionPort)
	if !ok {
		return nil, errors.New("统一数据删除入口不可用")
	}
	return port.PrivacyDeletion(c.Request.Context(), operation, input, id)
}

func (h *Handler) PrivacyDeletionPreview(c *gin.Context) {
	var input coordination.PrivacyDeletionRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if c.ShouldBindJSON(&input) != nil {
		privacyError(c, 400, "删除预览参数无效", nil)
		return
	}
	input.Target = nil
	input.ID = ""
	if input.TargetType == "memory" {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			privacyError(c, 503, "统一记忆删除预览入口不可用", nil)
			return
		}
		targets, err := port.PreviewMemoryDeletion(c.Request.Context(), input.TargetID)
		if err != nil {
			privacyError(c, 409, "无法确认记忆归属", err.Error())
			return
		}
		util.SuccessResponse(c, gin.H{"targets": targets})
		return
	}
	result, err := h.privacyDeletionOperation(c, "preview", input, "")
	if err != nil {
		privacyError(c, 409, "无法确认删除范围", err.Error())
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionRequest(c *gin.Context) {
	var payload struct {
		coordination.PrivacyDeletionRequest
		RequestID string `json:"requestId"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	if c.ShouldBindJSON(&payload) != nil {
		privacyError(c, 400, "删除参数无效", nil)
		return
	}
	if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
		privacyError(c, 409, "删除授权已变化", err.Error())
		return
	}
	if _, err := uuid.Parse(payload.RequestID); err != nil || payload.Target == nil {
		privacyError(c, 400, "删除缺少预览归属或有效请求编号", nil)
		return
	}
	if payload.LegacyDeletionID != "" {
		if err := h.validateLegacyDeletion(c, payload.PrivacyDeletionRequest); err != nil {
			privacyError(c, 409, "旧删除任务迁移未获授权", err.Error())
			return
		}
	}
	if payload.TargetType == "memory" {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			privacyError(c, 503, "统一记忆删除入口不可用", nil)
			return
		}
		result, err := port.DeleteMemoryIntent(c.Request.Context(), payload.TargetID, payload.Selection, *payload.Target, payload.RequestID)
		if err != nil {
			privacyError(c, 409, "记忆删除失败", err.Error())
			return
		}
		if payload.LegacyDeletionID != "" {
			if err := h.markLegacyDeletionMigrated(c, payload.LegacyDeletionID, map[string]any{"id": result.ID, "retrievalBlocked": result.RetrievalBlocked}); err != nil {
				privacyError(c, 409, "记忆删除已提交，旧任务归档失败", err.Error())
				return
			}
		}
		util.SuccessResponse(c, result)
		return
	}
	result, err := h.privacyDeletionOperation(c, "request", payload.PrivacyDeletionRequest, payload.RequestID)
	if err != nil {
		privacyError(c, 409, "删除未获数据持有方确认", err.Error())
		return
	}
	if payload.LegacyDeletionID != "" {
		if err := h.markLegacyDeletionMigrated(c, payload.LegacyDeletionID, result); err != nil {
			privacyError(c, 409, "删除已提交，但旧任务归档失败", gin.H{"result": result, "error": err.Error()})
			return
		}
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionStatus(c *gin.Context) {
	id := c.Param("id")
	if strings.HasPrefix(id, "tomb_") {
		actor := security.GetActor(c)
		if actor == nil || !actor.HasPermission(auth.PermSystemAdmin) {
			privacyError(c, 403, "旧任务归属不明，需要本机管理员确认", nil)
			return
		}
		var legacy mindruntime.DeletionTombstoneModel
		if err := h.db.WithContext(c.Request.Context()).Where("id=?", id).First(&legacy).Error; err != nil {
			privacyError(c, 404, "旧任务不存在", nil)
			return
		}
		if legacy.MigratedTo == "" {
			privacyError(c, 409, "旧任务尚未确认数据归属，请重新预览并迁移", nil)
			return
		}
		id = legacy.MigratedTo
	}
	if strings.HasPrefix(id, "owned-memory-delete.") {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			privacyError(c, 503, "统一记忆删除入口不可用", nil)
			return
		}
		result, err := port.DeletionStatus(c.Request.Context(), id)
		if err != nil {
			privacyError(c, 409, "删除状态读取失败", err.Error())
			return
		}
		util.SuccessResponse(c, result)
		return
	}
	result, err := h.privacyDeletionOperation(c, "status", coordination.PrivacyDeletionRequest{ID: id}, "")
	if err != nil {
		privacyError(c, 409, "删除回执不属于当前数据范围或不可用", err.Error())
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionStats(c *gin.Context) {
	result, err := h.privacyDeletionOperation(c, "stats", coordination.PrivacyDeletionRequest{}, "")
	if err != nil {
		privacyError(c, 503, "删除统计未完整读取", err.Error())
		return
	}
	port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
	if !ok {
		privacyError(c, 503, "记忆删除统计不可用", nil)
		return
	}
	memory, err := port.DeletionStats(c.Request.Context())
	if err != nil {
		privacyError(c, 503, "记忆删除统计读取失败", err.Error())
		return
	}
	for key, value := range memory {
		result[key] = value
	}
	count := func(value any) int64 {
		switch number := value.(type) {
		case int:
			return int64(number)
		case int64:
			return number
		case float64:
			return int64(number)
		}
		return 0
	}
	result["total"] = count(result["tombstones"]) + count(memory["memoryDeletions"])
	result["totalTombstones"] = result["total"]
	result["pending"] = count(result["pending"]) + count(memory["memoryPending"])
	result["completed"] = count(result["completed"]) + count(memory["memoryCompleted"])
	actor := security.GetActor(c)
	if actor != nil && actor.HasPermission(auth.PermSystemAdmin) && h.db.Migrator().HasTable(&mindruntime.DeletionTombstoneModel{}) {
		var records []mindruntime.DeletionTombstoneModel
		if err := h.db.WithContext(c.Request.Context()).Where("COALESCE(migrated_to,'')='' AND status<>?", "completed").Order("requested_at ASC").Limit(128).Find(&records).Error; err != nil {
			privacyError(c, 503, "旧删除任务归档读取失败", err.Error())
			return
		}
		result["legacyRecords"], result["legacyPending"] = records, len(records)
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionCleanup(c *gin.Context) {
	result, err := h.privacyDeletionOperation(c, "cleanup", coordination.PrivacyDeletionRequest{}, "")
	if err != nil {
		privacyError(c, 503, "删除清理未全部完成", err.Error())
		return
	}
	port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
	if !ok {
		privacyError(c, 503, "记忆清理入口不可用", nil)
		return
	}
	count, err := port.CleanupDeletions(c.Request.Context())
	if err != nil {
		privacyError(c, 503, "记忆索引清理未完成", err.Error())
		return
	}
	result["memoryCleaned"] = count
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionSecurityTests(c *gin.Context) {
	var input coordination.PrivacyDeletionRequest
	if c.ShouldBindJSON(&input) != nil {
		privacyError(c, 400, "删除核验参数无效", nil)
		return
	}
	if input.TargetType == "memory" {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			privacyError(c, 503, "记忆核验入口不可用", nil)
			return
		}
		checks, err := port.DeletionChecks(c.Request.Context(), input.TargetID)
		if err != nil {
			privacyError(c, 409, "无法核验真实记忆删除状态", err.Error())
			return
		}
		util.SuccessResponse(c, checks)
		return
	}
	result, err := h.privacyDeletionOperation(c, "checks", input, "")
	if err != nil {
		privacyError(c, 409, "无法核验真实删除结果", err.Error())
		return
	}
	util.SuccessResponse(c, result["checks"])
}

func (h *Handler) validateLegacyDeletion(c *gin.Context, input coordination.PrivacyDeletionRequest) error {
	actor := security.GetActor(c)
	if actor == nil || !actor.HasPermission(auth.PermSystemAdmin) {
		return errors.New("旧任务归属缺失，必须由管理员确认迁移")
	}
	var row mindruntime.DeletionTombstoneModel
	if err := h.db.WithContext(c.Request.Context()).Where("id=?", input.LegacyDeletionID).First(&row).Error; err != nil {
		return err
	}
	if row.MigratedTo != "" || row.TargetID != input.TargetID || row.TargetType != input.TargetType || row.Scope != input.Selection {
		return errors.New("旧任务与本次确认的数据范围不一致")
	}
	return nil
}

func (h *Handler) markLegacyDeletionMigrated(c *gin.Context, id string, result map[string]any) error {
	receipt, _ := result["id"].(string)
	if receipt == "" {
		return errors.New("新入口没有返回持久化回执")
	}
	return h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&mindruntime.DeletionTombstoneModel{}).Where("id=? AND COALESCE(migrated_to,'')=''", id).Updates(map[string]any{"migrated_to": receipt, "status": "migrated", "retrieval_blocked": result["retrievalBlocked"] == true})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errors.New("旧任务状态已变化")
		}
		if err := tx.Model(&mindruntime.OutboxCleanupItemModel{}).Where("id LIKE ? AND status<>?", "outbox_"+id+"_%", "completed").Updates(map[string]any{"status": "dead", "last_error": "migrated:" + receipt}).Error; err != nil {
			return err
		}
		return tx.Model(&mindruntime.RecalculationTaskModel{}).Where("id LIKE ? AND status<>?", "recalc_"+id+"_%", "completed").Updates(map[string]any{"status": "dead", "last_error": "migrated:" + receipt}).Error
	})
}
