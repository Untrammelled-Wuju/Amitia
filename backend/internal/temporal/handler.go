package temporal

import (
	"errors"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) GetUserProfile(c *gin.Context) {
	profile, err := h.service.GetProfile(c.Request.Context(), OwnerSpace, apiSpaceID(c))
	h.respond(c, profile, err)
}

func (h *Handler) UpdateUserProfile(c *gin.Context) {
	var input ProfilePatch
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "时间设置格式无效"})
		return
	}
	profile, err := h.service.PatchProfile(c.Request.Context(), OwnerSpace, apiSpaceID(c), input)
	h.respond(c, profile, err)
}

func (h *Handler) GetCharacterProfile(c *gin.Context) {
	characterID := strings.TrimSpace(c.Param("characterId"))
	if !h.requireCharacterOwner(c, characterID) {
		return
	}
	profile, err := h.service.GetProfile(c.Request.Context(), OwnerCharacter, characterID)
	h.respond(c, profile, err)
}

func (h *Handler) UpdateCharacterProfile(c *gin.Context) {
	var input ProfilePatch
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "角色时间设置格式无效"})
		return
	}
	characterID := strings.TrimSpace(c.Param("characterId"))
	if !h.requireCharacterOwner(c, characterID) {
		return
	}
	profile, err := h.service.PatchProfile(c.Request.Context(), OwnerCharacter, characterID, input)
	h.respond(c, profile, err)
}

func (h *Handler) GetSnapshot(c *gin.Context) {
	if !h.requireCharacterOwner(c, strings.TrimSpace(c.Query("characterId"))) {
		return
	}
	snapshot, err := h.service.ResolveSnapshot(c.Request.Context(), snapshotInput(c))
	h.respond(c, snapshot, err)
}

func (h *Handler) GetDiagnostics(c *gin.Context) {
	if !h.requireCharacterOwner(c, strings.TrimSpace(c.Query("characterId"))) {
		return
	}
	snapshot, err := h.service.ResolveSnapshot(c.Request.Context(), snapshotInput(c))
	if err != nil {
		h.respond(c, nil, err)
		return
	}
	events, err := h.service.ListEvents(c.Request.Context(), apiSpaceID(c), strings.TrimSpace(c.Query("characterId")), 50)
	if err != nil {
		h.respond(c, nil, err)
		return
	}
	h.respond(c, gin.H{"snapshot": snapshot, "core": gin.H{"featureFlags": h.service.FeatureFlags(), "clockSource": "system", "tzdb": "go-embedded-tzdata/" + runtime.Version()}, "relationshipTime": snapshot.RelationshipTime, "promptSections": []gin.H{{"type": "temporal_context", "content": h.service.RenderSnapshot(snapshot)}}, "commitEffects": []interface{}{}, "recentEvents": events, "metrics": h.service.Metrics(), "diagnostics": []string{}, "snapshotVersion": SnapshotVersion}, nil)
}

func (h *Handler) ListAnchors(c *gin.Context) {
	characterID := strings.TrimSpace(c.Query("characterId"))
	if !h.requireCharacterOwner(c, characterID) {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	anchors, err := h.service.ListAnchors(c.Request.Context(), AnchorQuery{SpaceID: apiSpaceID(c), CharacterID: characterID, Status: strings.TrimSpace(c.Query("status")), Limit: limit})
	h.respond(c, anchors, err)
}

func (h *Handler) CreateAnchor(c *gin.Context) {
	var input Anchor
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "时间锚点格式无效"})
		return
	}
	characterID := strings.TrimSpace(input.CharacterID)
	if !h.requireCharacterOwner(c, characterID) {
		return
	}
	anchor, err := h.service.SaveAnchor(c.Request.Context(), apiSpaceID(c), characterID, input)
	h.respond(c, anchor, err)
}

func (h *Handler) UpdateAnchor(c *gin.Context) {
	var input Anchor
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "时间锚点格式无效"})
		return
	}
	input.ID = c.Param("id")
	characterID := strings.TrimSpace(input.CharacterID)
	if !h.requireCharacterOwner(c, characterID) {
		return
	}
	anchor, err := h.service.SaveAnchor(c.Request.Context(), apiSpaceID(c), characterID, input)
	h.respond(c, anchor, err)
}

func (h *Handler) DeleteAnchor(c *gin.Context) {
	characterID := strings.TrimSpace(c.Query("characterId"))
	if !h.requireCharacterOwner(c, characterID) {
		return
	}
	err := h.service.DeleteAnchor(c.Request.Context(), apiSpaceID(c), characterID, c.Param("id"))
	h.respond(c, gin.H{"deleted": err == nil}, err)
}

func (h *Handler) ConfirmAnchor(c *gin.Context) {
	characterID := strings.TrimSpace(c.Query("characterId"))
	if !h.requireCharacterOwner(c, characterID) {
		return
	}
	anchor, err := h.service.ConfirmAnchor(c.Request.Context(), apiSpaceID(c), characterID, c.Param("id"))
	h.respond(c, anchor, err)
}

func (h *Handler) ListEvents(c *gin.Context) {
	characterID := strings.TrimSpace(c.Query("characterId"))
	if !h.requireCharacterOwner(c, characterID) {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	events, err := h.service.ListEvents(c.Request.Context(), apiSpaceID(c), characterID, limit)
	h.respond(c, events, err)
}

func (h *Handler) Recompute(c *gin.Context) {
	recomputed, err := h.service.RecomputeAnchorOccurrences(c.Request.Context())
	if err != nil {
		h.respond(c, nil, err)
		return
	}
	processed, err := h.service.ProcessDueAnchors(c.Request.Context(), false)
	if err != nil {
		h.respond(c, nil, err)
		return
	}
	snapshot, err := h.service.ResolveSnapshot(c.Request.Context(), snapshotInput(c))
	h.respond(c, gin.H{"snapshot": snapshot, "recomputedAnchors": recomputed, "processed": processed}, err)
}

func (h *Handler) AcceptTimezoneSuggestion(c *gin.Context) {
	profile, err := h.service.ResolveTimezoneSuggestion(c.Request.Context(), apiSpaceID(c), true)
	h.respond(c, profile, err)
}
func (h *Handler) RejectTimezoneSuggestion(c *gin.Context) {
	profile, err := h.service.ResolveTimezoneSuggestion(c.Request.Context(), apiSpaceID(c), false)
	h.respond(c, profile, err)
}

func (h *Handler) SuggestTimezone(c *gin.Context) {
	var input struct {
		Timezone string `json:"timezone"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "时区建议格式无效"})
		return
	}
	profile, err := h.service.SuggestTimezone(c.Request.Context(), apiSpaceID(c), input.Timezone)
	h.respond(c, profile, err)
}

func (h *Handler) requireCharacterOwner(c *gin.Context, characterID string) bool {
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		return true
	}
	owned, err := h.service.CharacterOwnedBy(characterID, apiSpaceID(c))
	if err != nil {
		h.respond(c, nil, err)
		return false
	}
	if !owned {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "角色不存在"})
		return false
	}
	return true
}

func (h *Handler) respond(c *gin.Context, data interface{}, err error) {
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": data, "msg": "操作成功"})
		return
	}
	status := http.StatusInternalServerError
	message := "时间服务操作失败"
	switch {
	case errors.Is(err, ErrInvalidTimezone):
		status = http.StatusBadRequest
		message = "IANA 时区无效"
	case errors.Is(err, ErrInvalidOwner), errors.Is(err, ErrScopeMismatch):
		status = http.StatusBadRequest
		message = "时间配置作用域无效"
	case errors.Is(err, ErrAnchorNotFound):
		status = http.StatusNotFound
		message = "时间锚点不存在"
	}
	c.JSON(status, gin.H{"code": status, "msg": message})
}

func apiSpaceID(c *gin.Context) string {
	return requestidentity.ResolveGin(c)
}

func snapshotInput(c *gin.Context) SnapshotInput {
	return SnapshotInput{SpaceID: apiSpaceID(c), CharacterID: strings.TrimSpace(c.Query("characterId")), Channel: strings.TrimSpace(c.Query("channel")), DeviceTimezone: strings.TrimSpace(c.GetHeader("X-Device-Timezone"))}
}
