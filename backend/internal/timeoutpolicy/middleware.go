package timeoutpolicy

import (
	"github.com/gin-gonic/gin"
	"strconv"
)

func Headers() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, _ := Current()
		c.Header("X-Amitia-Timeout-Disabled", strconv.FormatBool(settings.Disabled))
		c.Header("X-Amitia-Timeout-Seconds", strconv.Itoa(settings.Seconds))
		c.Next()
	}
}
