// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import "github.com/gin-gonic/gin"

func withRequestContextFallback(engine *gin.Engine) *gin.Engine {
	engine.ContextWithFallback = true
	return engine
}
