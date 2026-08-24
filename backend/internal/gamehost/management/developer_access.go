package management

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	middlewaresecurity "github.com/u-ai/backend/internal/middleware/security"
)

func gameHostDeveloperModeEnabled() bool {
	enabled, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("AMITIA_EXTENSION_DEV_MODE")))
	return err == nil && enabled
}

func hasGameHostDeveloperAccess(c *gin.Context) bool {
	actor := middlewaresecurity.GetActor(c)
	if actor == nil {
		return false
	}
	if actor.ActorType == auth.ActorTypeLocalAdmin && actor.IsLocalTrusted {
		return true
	}
	isAdmin := actor.ActorType == auth.ActorTypeAdmin || actor.HasRole("admin")
	return isAdmin && gameHostDeveloperModeEnabled()
}

func RequireGameHostDeveloperAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !hasGameHostDeveloperAccess(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": 403,
				"msg":  "game host developer access requires a trusted local admin or admin with extension dev mode enabled",
			})
			return
		}
		c.Next()
	}
}

func GetGameHostDeveloperAccess(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "ok",
		"data": gin.H{"enabled": hasGameHostDeveloperAccess(c)},
	})
}
