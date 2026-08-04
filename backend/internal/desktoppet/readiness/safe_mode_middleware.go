// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package readiness

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RejectWritesWhenSafeMode(
	controller *SafeModeController,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if controller == nil {
			c.JSON(
				http.StatusServiceUnavailable,
				gin.H{
					"code": 503,
					"msg":  "safe mode controller missing",
				},
			)
			c.Abort()
			return
		}

		active, reason, _ :=
			controller.IsInSafeMode()

		if active {
			c.JSON(
				http.StatusServiceUnavailable,
				gin.H{
					"code": 503,
					"msg":  "desktop pet safe mode",
					"data": gin.H{
						"reason": reason,
					},
				},
			)
			c.Abort()
			return
		}

		c.Next()
	}
}
