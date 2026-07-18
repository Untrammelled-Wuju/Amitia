package emote

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "60"))
	items, total, err := h.service.List(c.Query("groupId"), c.Query("view"), c.Query("q"), page, pageSize)
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, item)
}

func (h *Handler) Upload(c *gin.Context) {
	header, err := c.FormFile("file")
	if err != nil {
		header, err = c.FormFile("image")
	}
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少表情文件", nil)
		return
	}
	config := parseConfig(c.PostForm("config"))
	result := h.service.Import(header, config)
	if result.Status == "failed" {
		util.ErrorResponse(c, response.InvalidParams, result.ErrorMessage, result)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) BatchUpload(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "批量上传请求无效", nil)
		return
	}
	headers := c.Request.MultipartForm.File["files"]
	if len(headers) == 0 {
		headers = c.Request.MultipartForm.File["file"]
	}
	configs := []ImportConfig{}
	_ = json.Unmarshal([]byte(c.PostForm("configs")), &configs)
	byName := map[string]ImportConfig{}
	for _, cfg := range configs {
		byName[cfg.SourceName] = cfg
	}
	results := make([]ImportResult, 0, len(headers))
	success, duplicates, failed, disabled := 0, 0, 0, 0
	for i, header := range headers {
		cfg := byName[header.Filename]
		if cfg.SourceName == "" && i < len(configs) {
			cfg = configs[i]
		}
		result := h.service.Import(header, cfg)
		results = append(results, result)
		switch result.Status {
		case "success":
			success++
		case "duplicate":
			duplicates++
		default:
			failed++
		}
		if result.AIWasDisabled {
			disabled++
		}
	}
	util.SuccessResponse(c, gin.H{"items": results, "summary": gin.H{"success": success, "duplicates": duplicates, "failed": failed, "aiDisabled": disabled}})
}

func parseConfig(raw string) ImportConfig {
	cfg := ImportConfig{}
	_ = json.Unmarshal([]byte(raw), &cfg)
	return cfg
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	item, err := h.service.Update(c.Param("id"), req)
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, item)
}

func (h *Handler) BatchUpdate(c *gin.Context) {
	var body struct {
		IDs    []string      `json:"ids"`
		Update UpdateRequest `json:"update"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		util.ErrorResponse(c, response.InvalidParams, "请选择表情", nil)
		return
	}
	results := []gin.H{}
	for _, id := range body.IDs {
		item, err := h.service.Update(id, body.Update)
		if err != nil {
			results = append(results, gin.H{"emoteId": id, "status": "failed", "error": err.Error()})
		} else {
			results = append(results, gin.H{"emoteId": id, "status": "success", "item": item})
		}
	}
	util.SuccessResponse(c, gin.H{"items": results})
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"deleted": true})
}

func (h *Handler) SetGroups(c *gin.Context) {
	var body struct {
		GroupIDs []string `json:"groupIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	item, err := h.service.Update(c.Param("id"), UpdateRequest{GroupIDs: body.GroupIDs})
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, item)
}

func (h *Handler) SetRoleScope(c *gin.Context) {
	var body struct {
		RoleScope    string   `json:"roleScope"`
		CharacterIDs []string `json:"characterIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	item, err := h.service.Update(c.Param("id"), UpdateRequest{RoleScope: &body.RoleScope, CharacterIDs: body.CharacterIDs})
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, item)
}

func (h *Handler) Groups(c *gin.Context) {
	items, err := h.service.Groups()
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) CreateGroup(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	item, err := h.service.CreateGroup(body.Name)
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, item)
}

func (h *Handler) UpdateGroup(c *gin.Context) {
	var body struct {
		Name         *string `json:"name"`
		CoverEmoteID *string `json:"coverEmoteId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	if err := h.service.UpdateGroup(c.Param("id"), body.Name, body.CoverEmoteID); err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"updated": true})
}

func (h *Handler) DeleteGroup(c *gin.Context) {
	if err := h.service.DeleteGroup(c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"deleted": true, "emotesPreserved": true})
}

func (h *Handler) ReorderGroups(c *gin.Context) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	if err := h.service.ReorderGroups(body.IDs); err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"updated": true})
}

func (h *Handler) AddGroupEmotes(c *gin.Context) {
	var body struct {
		EmoteIDs []string `json:"emoteIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	if err := h.service.AddToGroup(c.Param("id"), body.EmoteIDs); err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"updated": true})
}

func (h *Handler) RemoveGroupEmote(c *gin.Context) {
	if err := h.service.RemoveFromGroup(c.Param("id"), c.Param("emoteId")); err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"updated": true})
}

func (h *Handler) ManualSend(c *gin.Context) {
	var body struct {
		ConversationID   string  `json:"conversationId"`
		CharacterID      string  `json:"characterId"`
		EmoteID          string  `json:"emoteId"`
		ReplyToMessageID *string `json:"replyToMessageId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.ConversationID) == "" || strings.TrimSpace(body.CharacterID) == "" || strings.TrimSpace(body.EmoteID) == "" {
		util.ErrorResponse(c, response.InvalidParams, "发送参数不完整", nil)
		return
	}
	message, err := h.service.ManualSend(body.ConversationID, body.CharacterID, body.EmoteID, body.ReplyToMessageID)
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, message)
}

func (h *Handler) GetSettings(c *gin.Context) {
	settings, err := h.service.GetSettings(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, settings)
}

func (h *Handler) SaveSettings(c *gin.Context) {
	var body struct {
		Enabled                  bool    `json:"enabled"`
		BaseProbability          float64 `json:"baseProbability"`
		MaxProbability           float64 `json:"maxProbability"`
		MaxPerHour               int     `json:"maxPerHour"`
		MinReplyGap              int     `json:"minReplyGap"`
		SameEmoteCooldownMinutes int     `json:"sameEmoteCooldownMinutes"`
		AllowEmoteOnly           bool    `json:"allowEmoteOnly"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	settings := CharacterSettings{Enabled: boolInt(body.Enabled), BaseProbability: body.BaseProbability, MaxProbability: body.MaxProbability, MaxPerHour: body.MaxPerHour, MinReplyGap: body.MinReplyGap, SameEmoteCooldownMinutes: body.SameEmoteCooldownMinutes, AllowEmoteOnly: boolInt(body.AllowEmoteOnly)}
	saved, err := h.service.SaveSettings(c.Param("id"), settings)
	if err != nil {
		fail(c, err)
		return
	}
	util.SuccessResponse(c, saved)
}

func fail(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		util.ErrorResponse(c, response.DataNotFound, "数据不存在", nil)
		return
	}
	util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
}
