// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"errors"
	"net"
)

type SecurityMode string

const (
	SecurityModeNetwork     SecurityMode = "network"
	SecurityModeLocalSingle SecurityMode = "local_single_user"
	SecurityModeMaintenance SecurityMode = "maintenance"
)

type SecurityConfig struct {
	Mode              SecurityMode
	AllowRemoteAccess bool
	ListenAddress     string
	JWTSecret         string
	LocalToken        string
	AllowedOrigins    []string
}

func (s *SecurityConfig) Validate() error {
	switch s.Mode {
	case SecurityModeNetwork:
		if s.JWTSecret == "" {
			return errors.New("security: JWT secret required in network mode")
		}
	case SecurityModeLocalSingle:
		if s.LocalToken == "" {
			return errors.New("security: local token required in local single user mode")
		}
		if s.AllowRemoteAccess {
			return errors.New("security: remote access must be disabled in local single user mode")
		}
		if !isLoopback(s.ListenAddress) {
			return errors.New("security: local single user mode must listen on loopback only")
		}
	case SecurityModeMaintenance:
		if s.JWTSecret == "" && s.LocalToken == "" {
			return errors.New("security: either JWT secret or local token required in maintenance mode")
		}
	default:
		return errors.New("security: invalid security mode")
	}
	return nil
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func (s *SecurityConfig) IsFailOpen() bool {
	return s.FailOpen
}
