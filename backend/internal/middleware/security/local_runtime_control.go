package security

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

func LocalRuntimeControlMiddleware(store *LocalCredentialStore, listenAddr string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			host = strings.Trim(c.Request.RemoteAddr, "[]")
		}

		if !isLoopbackAddr(host) {
			c.AbortWithStatusJSON(403, gin.H{"code": "forbidden", "message": "loopback only"})
			return
		}

		if listenAddr != "" {
			lhost, _, lerr := net.SplitHostPort(listenAddr)
			if lerr != nil {
				lhost = strings.Trim(listenAddr, "[]")
			}
			if !isLoopback(lhost) && lhost != "0.0.0.0" && lhost != "::" {
				c.AbortWithStatusJSON(403, gin.H{"code": "forbidden", "message": "listen address not loopback"})
				return
			}
		}

		token := c.GetHeader("X-Amitia-Local-Token")
		if token == "" {
			token = c.Query("token")
		}
		if token == "" {
			c.AbortWithStatusJSON(401, gin.H{"code": "unauthorized", "message": "missing local token"})
			return
		}

		if store == nil || !store.Validate(token) {
			c.AbortWithStatusJSON(401, gin.H{"code": "unauthorized", "message": "invalid local token"})
			return
		}

		c.Next()
	}
}

func isLoopbackAddr(host string) bool {
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
