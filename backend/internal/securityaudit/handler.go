package securityaudit

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"strconv"
)

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }
func (h *Handler) ListEvents(c *gin.Context) {
	actor := getActor(c)
	if actor == nil || actor.SpaceID == "" {
		util.ErrorResponse(c, response.Unauthorized, "未认证", nil)
		return
	}
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, e := strconv.Atoi(l); e == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	events, err := h.repo.ListSpaceEvents(actor.SpaceID.String(), limit, c.Query("cursor"))
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, events)
}
func getActor(c *gin.Context) *auth.ActorContext {
	if v, ok := c.Get("actorContext"); ok {
		if a, ok := v.(*auth.ActorContext); ok {
			return a
		}
	}
	return nil
}
