// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterRealtimeRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	SetDB(ctx.DB)
	r.GET("/realtime/session", HandleSession)
}
