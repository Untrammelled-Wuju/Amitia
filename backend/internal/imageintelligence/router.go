package imageintelligence

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/pkg/util"
)

func RegisterRouter(group *gin.RouterGroup, service ImageIntelligence) {
	if group == nil || service == nil {
		return
	}
	imageIntelligence := group.Group("/image-intelligence")
	imageIntelligence.GET("/status", func(c *gin.Context) {
		util.SuccessResponse(c, service.Status(c.Request.Context()))
	})
	imageIntelligence.POST("/generate", func(c *gin.Context) {
		actor, ok := auth.FromContext(c.Request.Context())
		if !ok || actor == nil || actor.SpaceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "authenticated space principal is required"})
			return
		}
		var request ImageGenerateRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": ErrInvalid, "message": err.Error()})
			return
		}
		request.OwnerSpaceID = actor.SpaceID.String()
		result, err := service.Generate(c.Request.Context(), request)
		if err != nil {
			status := http.StatusInternalServerError
			var typed *Error
			if errors.As(err, &typed) && typed.HTTPStatus >= 400 && typed.HTTPStatus <= 599 {
				status = typed.HTTPStatus
			}
			c.JSON(status, gin.H{"code": "IMAGE_GENERATION_FAILED", "message": err.Error(), "error": err})
			return
		}
		util.SuccessResponse(c, result)
	})
}
