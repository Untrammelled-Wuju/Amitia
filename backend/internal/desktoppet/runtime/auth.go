// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"

	"github.com/u-ai/backend/log"
)

type Auth struct {
	config      *DesktopPetRuntimeConfig
	mu          sync.RWMutex
	token       string
	allowLegacy bool
}

func NewAuth(config *DesktopPetRuntimeConfig) *Auth {
	return &Auth{
		config:      config,
		token:       config.Token,
		allowLegacy: false,
	}
}

func (a *Auth) Token() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.token
}

func (a *Auth) SetToken(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = token
	log.Logger.Info("runtime auth: token rotated")
}

func (a *Auth) SetAllowLegacy(allow bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.allowLegacy = allow
}

func (a *Auth) ValidateRequest(r *http.Request) error {
	if a.config.LoopbackOnly {
		remoteAddr := r.RemoteAddr
		if !a.config.IsLoopbackAddr(remoteAddr) {
			log.Logger.Warnf("runtime auth: non-loopback origin rejected remoteAddr=%s", remoteAddr)
			return NewRuntimeError(ErrCodeRuntimeForbiddenOrigin, "non-loopback origin rejected", ErrRuntimeForbiddenOrigin)
		}
	}
	if !a.config.AllowRemote && !a.config.LoopbackOnly {
		remoteAddr := r.RemoteAddr
		if !a.config.IsLoopbackAddr(remoteAddr) {
			return NewRuntimeError(ErrCodeRuntimeForbiddenOrigin, "remote origin rejected", ErrRuntimeForbiddenOrigin)
		}
	}
	origin := r.Header.Get("Origin")
	if origin != "" && !a.isAllowedOrigin(origin) {
		return NewRuntimeError(ErrCodeRuntimeForbiddenOrigin, "origin not allowed: "+origin, ErrRuntimeForbiddenOrigin)
	}
	token := a.extractToken(r)
	if token == "" {
		return NewRuntimeError(ErrCodeRuntimeUnauthorized, "missing runtime token", ErrRuntimeUnauthorized)
	}
	a.mu.RLock()
	currentToken := a.token
	allowLegacy := a.allowLegacy
	a.mu.RUnlock()
	if allowLegacy && subtle.ConstantTimeCompare([]byte(token), []byte(currentToken)) == 1 {
		return nil
	}
	return NewRuntimeError(ErrCodeRuntimeUnauthorized, "invalid runtime token", ErrRuntimeUnauthorized)
}

func (a *Auth) extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
		if strings.HasPrefix(auth, "Token ") {
			return strings.TrimPrefix(auth, "Token ")
		}
	}
	token := r.Header.Get("X-Runtime-Token")
	if token != "" {
		return token
	}
	sec := r.Header.Get("Sec-WebSocket-Protocol")
	if sec != "" {
		parts := strings.Split(sec, ", ")
		for _, p := range parts {
			if strings.HasPrefix(p, "amitia-runtime.") {
				return strings.TrimPrefix(p, "amitia-runtime.")
			}
		}
	}
	return ""
}

func (a *Auth) isAllowedOrigin(origin string) bool {
	if a.config.LoopbackOnly {
		lower := strings.ToLower(origin)
		if strings.HasPrefix(lower, "http://127.0.0.1") ||
			strings.HasPrefix(lower, "http://localhost") ||
			strings.HasPrefix(lower, "http://[::1]") {
			return true
		}
		return false
	}
	return true
}

func (a *Auth) CheckOrigin(r *http.Request) bool {
	if a.config.LoopbackOnly {
		remoteAddr := r.RemoteAddr
		if !a.config.IsLoopbackAddr(remoteAddr) {
			return false
		}
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return a.isAllowedOrigin(origin)
}

type BootstrapTokenInfo struct {
	SchemaVersion     int    `json:"schemaVersion"`
	Endpoint          string `json:"endpoint"`
	Token             string `json:"token"`
	Protocol          string `json:"protocol"`
	BackendInstanceID string `json:"backendInstanceId"`
}

func (a *Auth) BootstrapTokenInfo(endpoint string) BootstrapTokenInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return BootstrapTokenInfo{
		SchemaVersion:     1,
		Endpoint:          endpoint,
		Token:             a.token,
		Protocol:          "amitia-desktop-pet.v1",
		BackendInstanceID: a.config.BackendInstanceID,
	}
}
