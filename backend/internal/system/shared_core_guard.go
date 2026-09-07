// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

func sharedCoreAdminOnly() gin.HandlerFunc {
	return security.SharedCoreAdminOnly()
}
