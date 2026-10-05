package security

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func DeviceExecutionAuthority(service *coordination.Service, coreID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		if service == nil || actor == nil || actor.IsLocalTrusted || actor.DeviceID == "" || c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if path != "/api/chat" && !strings.HasPrefix(path, "/api/agent/") && !strings.HasPrefix(path, "/api/interaction/") {
			c.Next()
			return
		}
		ctx, scope, finish, err := service.Begin(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String(), "", coreID, "", actor.RequestID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": "mesh.scope_unavailable", "message": "无法确定本次请求的数据归属"})
			return
		}
		defer finish()
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Amitia-Provider-Epoch", strconv.FormatInt(scope.ProviderEpoch, 10))
		c.Header("X-Amitia-Mode-Revision", strconv.FormatInt(scope.ModeRevision, 10))
		c.Next()
	}
}
