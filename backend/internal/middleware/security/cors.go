// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type CorsConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int
}

func CorsMiddleware(cfg CorsConfig) gin.HandlerFunc {
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = []string{"Content-Type", "Authorization", "X-Device-Timezone", "X-Amitia-Local-Token", "X-Amitia-Desktop-Session", "X-Amitia-Desktop-Instance", "X-Request-ID"}
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 86400
	}

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")

return func(c *gin.Context) {
	origin := c.GetHeader("Origin")
	allowed := false

	if origin == "" {
		allowed = true
	} else {
		for _, allowedOrigin := range cfg.AllowedOrigins {
			if allowedOrigin == origin {
				allowed = true
				break
			}
		}
	}

	if origin != "" {
		if !allowed {
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin, Access-Control-Request-Headers")
	}

	if allowed {
		c.Header("Access-Control-Allow-Methods", methods)
		c.Header("Access-Control-Allow-Headers", headers)
		c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))
	}

	if c.Request.Method == "OPTIONS" {
		if allowed {
			c.AbortWithStatus(http.StatusNoContent)
		} else {
			c.AbortWithStatus(http.StatusForbidden)
		}
		return
	}

	c.Next()
}
}
