package behavior

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/desktoppet/behavior/bindings"
	"github.com/u-ai/backend/internal/middleware"
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
	userID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	characterID := c.Param("characterId")
	if characterID == "" {
		util.ErrorResponse(c, response.InvalidParams, "characterId 不能为空", nil)
		return
	}
	snapshot, err := h.service.GetBehaviorState(c.Request.Context(), userID, characterID)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	util.SuccessResponse(c, snapshot)
}

func requireAdmin(c *gin.Context) (string, bool) {
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return "", false
	}
	if !actor.HasPermission(auth.PermDesktopPetBehaviorAdmin) {
		util.ErrorResponse(c, response.Forbidden, "需要管理员权限", gin.H{"errorCode": "FORBIDDEN"})
		return "", false
	}
	return actor.UserID, true
}

func (h *Handler) GetMetrics(c *gin.Context) {
	if _, ok := requireAdmin(c); !ok {
		return
	}
	metrics := h.service.GetMetrics()
	util.SuccessResponse(c, metrics)
}

func (h *Handler) SimulateEvent(c *gin.Context) {
	if _, ok := requireAdmin(c); !ok {
		return
	}
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
	actorID, ok := requireAdmin(c)
	if !ok {
		return
	}
	var req reconcileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	if req.CharacterID == "" {
		util.ErrorResponse(c, response.InvalidParams, "characterId 不能为空", nil)
		return
	}
	if req.UserID == "" {
		req.UserID = actorID
	}
	if err := h.service.TriggerReconcile(c.Request.Context(), req.UserID, req.CharacterID); err != nil {
		writeBehaviorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "状态恢复已触发", nil)
}

func (h *Handler) SetShadowMode(c *gin.Context) {
	if _, ok := requireAdmin(c); !ok {
		return
	}
	var req setModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	h.service.SetShadowMode(req.Enabled)
	util.SuccessMsgResponse(c, "影子模式已更新", nil)
}

func (h *Handler) SetRuntimeCommand(c *gin.Context) {
	if _, ok := requireAdmin(c); !ok {
		return
	}
	var req setModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	h.service.SetRuntimeCommandEnabled(req.Enabled)
	util.SuccessMsgResponse(c, "运行时命令已更新", nil)
}

func (h *Handler) ListBindings(c *gin.Context) {
	userID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	characterID := c.Query("characterId")
	items, err := h.service.ListBindings(c.Request.Context(), userID, characterID)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	if items == nil {
		items = []bindings.BehaviorBinding{}
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) CreateBinding(c *gin.Context) {
	var binding bindings.BehaviorBinding
	if err := c.ShouldBindJSON(&binding); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	binding.UserID = actorID
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
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	existing, err := h.service.GetBinding(c.Request.Context(), id)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	if existing.UserID != actorID {
		util.ErrorResponse(c, response.Forbidden, "无权访问该绑定", gin.H{"errorCode": "BINDING_NOT_OWNED"})
		return
	}
	util.SuccessResponse(c, existing)
}

var behaviorUpdateWhitelist = map[string]bool{
	"event_type":       true,
	"conditions_json":  true,
	"semantic":         true,
	"preferred_action": true,
	"priority_offset":  true,
	"cooldown_ms":      true,
	"enabled":          true,
}

func filterBehaviorUpdates(updates map[string]interface{}) (map[string]interface{}, []string) {
	filtered := make(map[string]interface{})
	var rejected []string
	for k, v := range updates {
		if behaviorUpdateWhitelist[k] {
			filtered[k] = v
		} else {
			rejected = append(rejected, k)
		}
	}
	return filtered, rejected
}

func (h *Handler) UpdateBinding(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.ErrorResponse(c, response.InvalidParams, "绑定 ID 不能为空", nil)
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	existing, err := h.service.GetBinding(c.Request.Context(), id)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	if existing.UserID != actorID {
		util.ErrorResponse(c, response.Forbidden, "无权操作该绑定", gin.H{"errorCode": "BINDING_NOT_OWNED"})
		return
	}
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), nil)
		return
	}
	filtered, rejected := filterBehaviorUpdates(updates)
	if len(rejected) > 0 {
		util.ErrorResponse(c, response.InvalidParams, "包含不允许更新的字段", gin.H{"errorCode": "FIELD_NOT_ALLOWED", "fields": rejected})
		return
	}
	if err := h.service.UpdateBinding(c.Request.Context(), id, filtered); err != nil {
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
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	existing, err := h.service.GetBinding(c.Request.Context(), id)
	if err != nil {
		writeBehaviorError(c, err)
		return
	}
	if existing.UserID != actorID {
		util.ErrorResponse(c, response.Forbidden, "无权操作该绑定", gin.H{"errorCode": "BINDING_NOT_OWNED"})
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
