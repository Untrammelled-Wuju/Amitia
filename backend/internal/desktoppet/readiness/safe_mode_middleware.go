// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package readiness

import (
	"net/http"
	"time"

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

		method := c.Request.Method
		isWrite := method == http.MethodPost ||
			method == http.MethodPut ||
			method == http.MethodPatch ||
			method == http.MethodDelete

		if !isWrite {
			c.Next()
			return
		}

		if active, reason, enteredAt := controller.IsInSafeMode(); active {
			c.JSON(
				http.StatusServiceUnavailable,
				gin.H{
					"code": 503,
					"msg":  "desktop pet safe mode",
					"data": gin.H{
						"reason":    reason,
						"enteredAt": enteredAt.Format(time.RFC3339),
					},
				},
			)
			c.Abort()
			return
		}

		c.Next()
	}
}
