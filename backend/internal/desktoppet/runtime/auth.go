// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"net/http"
	"strings"

	"github.com/u-ai/backend/log"
)

type Auth struct {
	config *DesktopPetRuntimeConfig
}

func NewAuth(config *DesktopPetRuntimeConfig) *Auth {
	return &Auth{config: config}
}

func (a *Auth) ValidateRequest(r *http.Request) error {
	if a.config == nil {
		return NewRuntimeError(ErrCodeRuntimeUnauthorized, "runtime auth config is missing", ErrRuntimeUnauthorized)
	}

	if a.config.LoopbackOnly && !a.config.IsLoopbackAddr(r.RemoteAddr) {
		log.Logger.Warnf("runtime auth: non-loopback request rejected remoteAddr=%s", r.RemoteAddr)
		return NewRuntimeError(ErrCodeRuntimeForbiddenOrigin, "non-loopback runtime request rejected", ErrRuntimeForbiddenOrigin)
	}

	if !a.config.AllowRemote && !a.config.IsLoopbackAddr(r.RemoteAddr) {
		return NewRuntimeError(ErrCodeRuntimeForbiddenOrigin, "remote runtime request rejected", ErrRuntimeForbiddenOrigin)
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" && !a.isAllowedOrigin(origin) {
		return NewRuntimeError(ErrCodeRuntimeForbiddenOrigin, "runtime origin not allowed", ErrRuntimeForbiddenOrigin)
	}

	if !containsRuntimeProtocol(r.Header.Get("Sec-WebSocket-Protocol")) {
		return NewRuntimeError(ErrCodeRuntimeProtocolMismatch, "runtime websocket protocol is missing", ErrRuntimeProtocolMismatch)
	}

	return nil
}

func containsRuntimeProtocol(header string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(part) == "amitia-desktop-pet.v1" {
			return true
		}
	}
	return false
}

func (a *Auth) isAllowedOrigin(origin string) bool {
	if a.config == nil {
		return false
	}
	if a.config.LoopbackOnly {
		lower := strings.ToLower(origin)
		return strings.HasPrefix(lower, "http://127.0.0.1") ||
			strings.HasPrefix(lower, "http://localhost") ||
			strings.HasPrefix(lower, "http://[::1]")
	}
	return true
}

func (a *Auth) CheckOrigin(r *http.Request) bool {
	if a.config == nil {
		return false
	}
	return a.ValidateRequest(r) == nil
}
