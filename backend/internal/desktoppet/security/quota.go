// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"errors"
	"sync"
)

var (
	ErrConcurrentLimitExceeded = errors.New("security: concurrent operation limit exceeded")
	ErrStorageQuotaExceeded    = errors.New("security: storage quota exceeded")
	ErrDiskCriticalReached     = errors.New("security: disk critical watermark reached")
)

type QuotaKey string

const (
	QuotaKeyUserConcurrentGeneration   QuotaKey = "user_concurrent_generation"
	QuotaKeyUserConcurrentRegeneration QuotaKey = "user_concurrent_regeneration"
	QuotaKeyUserConcurrentProcessing   QuotaKey = "user_concurrent_processing"
	QuotaKeyUserConcurrentQuality      QuotaKey = "user_concurrent_quality"
	QuotaKeyUserConcurrentReleaseBuild QuotaKey = "user_concurrent_release_build"
	QuotaKeyUserSSEConnections         QuotaKey = "user_sse_connections"
	QuotaKeyUserImportBytes            QuotaKey = "user_import_bytes"
	QuotaKeyUserStorageBytes           QuotaKey = "user_storage_bytes"
)

type QuotaLimits struct {
	UserConcurrentGeneration   int
	UserConcurrentRegeneration int
	UserConcurrentProcessing   int
	UserConcurrentQuality      int
	UserConcurrentReleaseBuild int
	UserSSEConnections         int
	UserImportBytes            int64
	UserStorageBytes           int64
	GlobalStorageBytes         int64
}

type QuotaUsage struct {
	Current      int64 `json:"current"`
	Limit        int64 `json:"limit"`
	CurrentBytes int64 `json:"currentBytes,omitempty"`
	LimitBytes   int64 `json:"limitBytes,omitempty"`
}

type QuotaService struct {
	mu     sync.RWMutex
	limits QuotaLimits
	usage  map[QuotaKey]map[string]int64
}

func NewQuotaService(limits QuotaLimits) *QuotaService {
	return &QuotaService{
		limits: limits,
		usage:  make(map[QuotaKey]map[string]int64),
	}
}

func (s *QuotaService) Increment(key QuotaKey, owner string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.usage[key]; !ok {
		s.usage[key] = make(map[string]int64)
	}
	s.usage[key][owner] += delta
}

func (s *QuotaService) Decrement(key QuotaKey, owner string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.usage[key]; !ok {
		s.usage[key] = make(map[string]int64)
	}
	s.usage[key][owner] -= delta
	if s.usage[key][owner] < 0 {
		s.usage[key][owner] = 0
	}
}

func (s *QuotaService) Get(key QuotaKey, owner string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.usage[key]; !ok {
		return 0
	}
	return s.usage[key][owner]
}

func (s *QuotaService) CanIncrement(key QuotaKey, owner string, maximum int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var limit int64
	switch key {
	case QuotaKeyUserConcurrentGeneration:
		limit = int64(s.limits.UserConcurrentGeneration)
	case QuotaKeyUserConcurrentRegeneration:
		limit = int64(s.limits.UserConcurrentRegeneration)
	case QuotaKeyUserConcurrentProcessing:
		limit = int64(s.limits.UserConcurrentProcessing)
	case QuotaKeyUserConcurrentQuality:
		limit = int64(s.limits.UserConcurrentQuality)
	case QuotaKeyUserConcurrentReleaseBuild:
		limit = int64(s.limits.UserConcurrentReleaseBuild)
	case QuotaKeyUserSSEConnections:
		limit = int64(s.limits.UserSSEConnections)
	default:
		return true
	}
	if maximum > 0 && limit > maximum {
		limit = maximum
	}
	current := s.usage[key][owner]
	return current < limit
}

func (s *QuotaService) CheckStorage(userID string, requestedBytes int64) error {
	if requestedBytes < 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.usage[QuotaKeyUserStorageBytes][userID]
	if requestedBytes > s.limits.UserStorageBytes-current {
		return ErrStorageQuotaExceeded
	}
	return nil
}

var DefaultQuotaLimits = QuotaLimits{
	UserConcurrentGeneration:   3,
	UserConcurrentRegeneration: 2,
	UserConcurrentProcessing:   3,
	UserConcurrentQuality:      3,
	UserConcurrentReleaseBuild: 1,
	UserSSEConnections:         5,
	UserImportBytes:            500 * 1024 * 1024,
	UserStorageBytes:           20 * 1024 * 1024 * 1024,
	GlobalStorageBytes:         50 * 1024 * 1024 * 1024,
}

type DiskUsage struct {
	UsedBytes                int64   `json:"usedBytes"`
	PendingBytes             int64   `json:"pendingBytes"`
	TrashBytes               int64   `json:"trashBytes"`
	QuotaBytes               int64   `json:"quotaBytes"`
	HighWatermarkPercent     float64 `json:"highWatermarkPercent"`
	CriticalWatermarkPercent float64 `json:"criticalWatermarkPercent"`
}

func (d *DiskUsage) Status() string {
	ratio := float64(d.UsedBytes) / float64(d.QuotaBytes)
	if ratio >= d.CriticalWatermarkPercent {
		return "critical"
	}
	if ratio >= d.HighWatermarkPercent {
		return "warning"
	}
	return "ok"
}

func (d *DiskUsage) CanWrite(bytesToAdd int64) error {
	ratio := float64(d.UsedBytes+bytesToAdd) / float64(d.QuotaBytes)
	if ratio >= d.CriticalWatermarkPercent {
		return ErrDiskCriticalReached
	}
	return nil
}
