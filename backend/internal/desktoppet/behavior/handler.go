package behavior

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type Handler struct {
	service *BehaviorService
}

func NewHandler(svc *BehaviorService) *Handler {
	return &Handler{service: svc}
}

type reconcileRequest struct {
	UserID      string `json:"userId"`
	CharacterID string `json:"characterId"`
}

type setModeRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) GetBehaviorState(c *gin.Context) {
	userID := c.Param("userId")
	characterID := c.Param("characterId")
	if userID == "" || characterID == "" {
		util.ErrorResponse(c, response.InvalidParams, "userId 和 characterId 不能为空", nil)
		return
	}
	snapshot, err := h.service.GetBehaviorState(c.Request.Context(), userID, characterID)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	util.SuccessResponse(c, snapshot)
}

func (h *Handler) GetMetrics(c *gin.Context) {
	metrics := h.service.GetMetrics()
	util.SuccessResponse(c, metrics)
}

func (h *Handler) SimulateEvent(c *gin.Context) {
	var event BehaviorEventEnvelope
	if err := c.ShouldBindJSON(&event); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	decision, err := h.service.SimulateEvent(c.Request.Context(), event)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	util.SuccessResponse(c, decision)
}

func (h *Handler) TriggerReconcile(c *gin.Context) {
	var req reconcileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	if req.UserID == "" || req.CharacterID == "" {
		util.ErrorResponse(c, response.InvalidParams, "userId 和 characterId 不能为空", nil)
		return
	}
	if err := h.service.TriggerReconcile(c.Request.Context(), req.UserID, req.CharacterID); err != nil {
		writeBehaviorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "状态恢复已触发", nil)
}

func (h *Handler) SetShadowMode(c *gin.Context) {
	var req setModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	h.service.SetShadowMode(req.Enabled)
	util.SuccessMsgResponse(c, "影子模式已更新", nil)
}

func (h *Handler) SetRuntimeCommand(c *gin.Context) {
	var req setModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	h.service.SetRuntimeCommandEnabled(req.Enabled)
	util.SuccessMsgResponse(c, "运行时命令已更新", nil)
}

func (h *Handler) ListBindings(c *gin.Context) {
	userID := c.Query("userId")
	characterID := c.Query("characterId")
	if userID == "" {
		util.ErrorResponse(c, response.InvalidParams, "userId 不能为空", nil)
		return
	}
	bindings, err := h.service.ListBindings(c.Request.Context(), userID, characterID)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	if bindings == nil {
		bindings = []BehaviorBinding{}
	}
	util.SuccessResponse(c, gin.H{"items": bindings, "total": len(bindings)})
}

func (h *Handler) CreateBinding(c *gin.Context) {
	var binding BehaviorBinding
	if err := c.ShouldBindJSON(&binding); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	if binding.UserID == "" {
		util.ErrorResponse(c, response.InvalidParams, "userId 不能为空", nil)
		return
	}
	if binding.EventType == "" {
		util.ErrorResponse(c, response.InvalidParams, "eventType 不能为空", nil)
		return
	}
	if binding.Semantic == "" {
		util.ErrorResponse(c, response.InvalidParams, "semantic 不能为空", nil)
		return
	}
	if err := h.service.CreateBinding(c.Request.Context(), binding); err != nil {
		writeBehaviorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "绑定已创建", binding)
}

func (h *Handler) GetBinding(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.ErrorResponse(c, response.InvalidParams, "绑定 ID 不能为空", nil)
		return
	}
	binding, err := h.service.GetBinding(c.Request.Context(), id)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	util.SuccessResponse(c, binding)
}

func (h *Handler) UpdateBinding(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.ErrorResponse(c, response.InvalidParams, "绑定 ID 不能为空", nil)
		return
	}
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	if err := h.service.UpdateBinding(c.Request.Context(), id, updates); err != nil {
		writeBehaviorError(c, err)
		return
	}
	updated, err := h.service.GetBinding(c.Request.Context(), id)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "绑定已更新", updated)
}

func (h *Handler) DeleteBinding(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.ErrorResponse(c, response.InvalidParams, "绑定 ID 不能为空", nil)
		return
	}
	if err := h.service.DeleteBinding(c.Request.Context(), id); err != nil {
		writeBehaviorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "绑定已删除", nil)
}

func writeBehaviorError(c *gin.Context, err error) {
	var be *BehaviorError
	if errors.As(err, &be) {
		httpCode := mapBehaviorErrorCode(be.Code)
		util.ErrorResponse(c, httpCode, be.Message, gin.H{"errorCode": be.Code})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		util.ErrorResponse(c, response.NotFound, "记录不存在", nil)
		return
	}
	util.ErrorResponse(c, response.InternalError, err.Error(), nil)
}

func mapBehaviorErrorCode(code string) int {
	switch code {
	case ErrCodeBindingInvalid, ErrCodeBindingActionMissing, ErrCodeEventSchemaInvalid:
		return response.InvalidParams
	case ErrCodeNoActiveInstallation, ErrCodeNoActionAvailable, ErrCodeSnapshotUnavailable:
		return response.NotFound
	case ErrCodeRulesetInvalid, ErrCodeContextConflict, ErrCodeMailboxOverflow:
		return response.BusinessError
	case ErrCodeRuntimeOffline, ErrCodeRuntimeCommandFailed, ErrCodePlaybackFailed:
		return response.BusinessError
	default:
		return response.InternalError
	}
}
