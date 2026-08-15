package securityaudit

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) ListEvents(c *gin.Context) {
	actor := getActor(c)
	if actor == nil {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}

	userID, err := strconv.ParseInt(string(actor.UserID), 10, 64)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "用户身份无效", nil)
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	cursor := c.Query("cursor")

	events, err := h.repo.ListUserEvents(userID, limit, cursor)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}

	result := make([]gin.H, 0, len(events))
	for _, e := range events {
		result = append(result, gin.H{
			"eventId":    e.EventID,
			"eventType":  e.EventType,
			"severity":   e.Severity,
			"outcome":    e.Outcome,
			"sessionId":  e.SessionID,
			"ipAddress":  e.IPAddress,
			"reasonCode": e.ReasonCode,
			"occurredAt": e.OccurredAt,
		})
	}

	util.SuccessResponse(c, result)
}

func getActor(c *gin.Context) *auth.ActorContext {
	if v, exists := c.Get("actorContext"); exists {
		if actor, ok := v.(*auth.ActorContext); ok {
			return actor
		}
	}
	return nil
}

var _ = fmt.Sprintf
