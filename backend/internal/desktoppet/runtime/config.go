// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

type DesktopPetRuntimeConfig struct {
	Enabled               bool
	Path                  string
	LoopbackOnly          bool
	AllowRemote           bool
	HeartbeatIntervalMs   int
	HeartbeatTimeoutMs    int
	MaxMessageBytes       int
	RegisterTimeoutSec    int
	SendQueueSize         int
	CommandTimeoutSec     int
	MaxRetryAttempts      int
	RetryBaseDelayMs      int
	RetryMaxDelayMs       int
	CommandRetentionHours int
	Token                 string
	BackendInstanceID     string
}

func DefaultRuntimeConfig() *DesktopPetRuntimeConfig {
	return &DesktopPetRuntimeConfig{
		Enabled:               true,
		Path:                  "/internal/desktop-pet/runtime/ws",
		LoopbackOnly:          true,
		AllowRemote:           false,
		HeartbeatIntervalMs:   10000,
		HeartbeatTimeoutMs:    30000,
		MaxMessageBytes:       1048576,
		RegisterTimeoutSec:    10,
		SendQueueSize:         64,
		CommandTimeoutSec:     30,
		MaxRetryAttempts:      5,
		RetryBaseDelayMs:      500,
		RetryMaxDelayMs:       30000,
		CommandRetentionHours: 24,
	}
}

func (c *DesktopPetRuntimeConfig) Validate() error {
	if c.HeartbeatIntervalMs <= 0 {
		return fmt.Errorf("heartbeatIntervalMs must be positive")
	}
	if c.HeartbeatTimeoutMs <= c.HeartbeatIntervalMs {
		return fmt.Errorf("heartbeatTimeoutMs must be greater than heartbeatIntervalMs")
	}
	if c.MaxMessageBytes <= 0 {
		return fmt.Errorf("maxMessageBytes must be positive")
	}
	if c.SendQueueSize <= 0 {
		return fmt.Errorf("sendQueueSize must be positive")
	}
	if c.RegisterTimeoutSec <= 0 {
		return fmt.Errorf("registerTimeoutSec must be positive")
	}
	if c.LoopbackOnly && c.AllowRemote {
		return fmt.Errorf("loopbackOnly and allowRemote cannot both be true")
	}
	return nil
}

func (c *DesktopPetRuntimeConfig) EnsureToken() {
	if c.Token == "" {
		b := make([]byte, 32)
		rand.Read(b)
		c.Token = hex.EncodeToString(b)
	}
	if c.BackendInstanceID == "" {
		b := make([]byte, 8)
		rand.Read(b)
		c.BackendInstanceID = "backend_boot_" + hex.EncodeToString(b)
	}
}

func (c *DesktopPetRuntimeConfig) IsLoopbackAddr(remoteAddr string) bool {
	if remoteAddr == "" {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	host = strings.TrimSpace(host)
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func (c *DesktopPetRuntimeConfig) HeartbeatInterval() time.Duration {
	return time.Duration(c.HeartbeatIntervalMs) * time.Millisecond
}

func (c *DesktopPetRuntimeConfig) HeartbeatTimeout() time.Duration {
	return time.Duration(c.HeartbeatTimeoutMs) * time.Millisecond
}

func (c *DesktopPetRuntimeConfig) RegisterTimeout() time.Duration {
	return time.Duration(c.RegisterTimeoutSec) * time.Second
}

func (c *DesktopPetRuntimeConfig) CommandTimeout() time.Duration {
	return time.Duration(c.CommandTimeoutSec) * time.Second
}

func (c *DesktopPetRuntimeConfig) RetryBaseDelay() time.Duration {
	return time.Duration(c.RetryBaseDelayMs) * time.Millisecond
}

func (c *DesktopPetRuntimeConfig) RetryMaxDelay() time.Duration {
	return time.Duration(c.RetryMaxDelayMs) * time.Millisecond
}
