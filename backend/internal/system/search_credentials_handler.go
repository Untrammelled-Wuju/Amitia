package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

type searchCredentialRequest struct {
	Value string `json:"value"`
}

func (h *Handler) SearchCredentialList(c *gin.Context) {
	if h.searchCredentials == nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "搜索凭据服务不可用", nil)
		return
	}
	items, err := h.searchCredentials.List(c.Request.Context())
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "读取搜索凭据失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items})
}

func (h *Handler) SearchCredentialSave(c *gin.Context) {
	if h.searchCredentials == nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "搜索凭据服务不可用", nil)
		return
	}
	var body searchCredentialRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "请求内容无效", nil)
		return
	}
	item, err := h.searchCredentials.Set(c.Request.Context(), c.Param("engineId"), body.Value)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "搜索凭据保存失败", nil)
		return
	}
	util.SuccessResponse(c, item)
}

func (h *Handler) SearchCredentialDelete(c *gin.Context) {
	if h.searchCredentials == nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "搜索凭据服务不可用", nil)
		return
	}
	if err := h.searchCredentials.Delete(c.Request.Context(), c.Param("engineId")); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "搜索凭据清除失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"deleted": true})
}
