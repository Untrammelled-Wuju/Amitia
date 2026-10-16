package workbenchtasks

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type ConversationQuery struct {
	ConversationID string
	OwnerID        string
	CharacterID    string
	TargetDeviceID string
	ExpectedCoreID string
}

type QueryContext struct {
	CanReadRecord      func(context.Context, *interaction.InteractionRecord) (bool, error) `json:"-"`
	ExecutionScope     coordination.ExecutionScope                                         `json:"executionScope"`
	ConversationOrigin business.ConversationOrigin                                         `json:"conversationOrigin"`
	ConversationID     string                                                              `json:"conversationId"`
}

type ConversationAccess func(context.Context, *auth.ActorContext, ConversationQuery, func(context.Context, QueryContext) error) error

func (h *Handler) WithConversationAccess(access ConversationAccess) *Handler {
	h.access = access
	return h
}

func validResourceID(id string) bool {
	return id != "" && len(id) <= 512 && utf8.ValidString(id) && !strings.ContainsAny(id, "\x00\r\n") && strings.TrimSpace(id) == id && !strings.HasPrefix(id, "meshconv1:")
}

func (h *Handler) readConversation(c *gin.Context, read func(context.Context, QueryContext) error) {
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil || actor.SpaceID == "" || actor.DeviceID == "" || (actor.PrincipalType != auth.PrincipalLocalUI && actor.PrincipalType != auth.PrincipalTrustedDevice) {
		c.JSON(http.StatusForbidden, gin.H{"error": "任务查询需要有效设备身份", "errorCode": "access_denied"})
		return
	}
	query := ConversationQuery{ConversationID: c.Query("conversationId"), OwnerID: c.Query("conversationOwnerId"), CharacterID: c.Query("characterId"), TargetDeviceID: c.Query("targetDeviceId"), ExpectedCoreID: c.Query("expectedCoreId")}
	if !validResourceID(query.ConversationID) || !validResourceID(query.OwnerID) || !validResourceID(query.CharacterID) || !validResourceID(query.ExpectedCoreID) || (query.TargetDeviceID != "" && !validResourceID(query.TargetDeviceID)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务查询缺少有效的会话归属、角色或执行 Core", "errorCode": "invalid_context"})
		return
	}
	if h.access == nil || h.records == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "任务归属查询服务暂不可用", "errorCode": "context_unavailable"})
		return
	}
	err := h.access(c.Request.Context(), actor, query, read)
	if err == nil {
		return
	}
	status, code, message := http.StatusServiceUnavailable, "context_unavailable", "任务所属执行服务或数据所有者暂不可用"
	switch {
	case errors.Is(err, coordination.ErrWrongOwner), errors.Is(err, coordination.ErrCapabilityGrant), errors.Is(err, coordination.ErrRoleRequired), errors.Is(err, coordination.ErrRoleSelection):
		status, code, message = http.StatusForbidden, "access_denied", "没有当前会话任务的访问权限"
	case errors.Is(err, coordination.ErrScopeExpired), errors.Is(err, coordination.ErrResourceVersion), errors.Is(err, coordination.ErrRequestConflict):
		status, code, message = http.StatusConflict, "scope_changed", "会话归属或执行 Core 已变化，请重新加载"
	case errors.Is(err, business.ErrConversationUnavailable):
		status, code, message = http.StatusNotFound, "conversation_unavailable", "当前执行 Core 下没有此会话，请返回原执行 Core 查看任务"
	}
	c.JSON(status, gin.H{"error": message, "errorCode": code})
}

type scopedRecordReader struct {
	reader  RecordReader
	role    string
	canRead func(context.Context, *interaction.InteractionRecord) (bool, error)
}

func (r scopedRecordReader) ListByScope(ctx context.Context, scope interaction.InteractionScope) ([]*interaction.InteractionRecord, error) {
	return r.ListRecentWorkbenchTaskRecords(ctx, scope, 121)
}

func (r scopedRecordReader) ListRecentWorkbenchTaskRecords(ctx context.Context, scope interaction.InteractionScope, limit int) ([]*interaction.InteractionRecord, error) {
	scope.CharacterID = r.role
	var rows []*interaction.InteractionRecord
	var err error
	if bounded, ok := r.reader.(BoundedRecordReader); ok {
		rows, err = bounded.ListRecentWorkbenchTaskRecords(ctx, scope, limit)
	} else {
		rows, err = r.reader.ListByScope(ctx, scope)
	}
	if err != nil {
		return nil, err
	}
	filtered := make([]*interaction.InteractionRecord, len(rows))
	for i, row := range rows {
		if row == nil || row.Scope.CharacterID != r.role {
			continue
		}
		if r.canRead == nil {
			continue
		}
		allowed, err := r.canRead(ctx, row)
		if err != nil {
			return nil, err
		}
		if allowed {
			filtered[i] = row
		}
	}
	return filtered, nil
}
