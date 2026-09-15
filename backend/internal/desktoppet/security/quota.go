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
	QuotaKeySpaceConcurrentGeneration   QuotaKey = "space_concurrent_generation"
	QuotaKeySpaceConcurrentRegeneration QuotaKey = "space_concurrent_regeneration"
	QuotaKeySpaceConcurrentProcessing   QuotaKey = "space_concurrent_processing"
	QuotaKeySpaceConcurrentQuality      QuotaKey = "space_concurrent_quality"
	QuotaKeySpaceConcurrentReleaseBuild QuotaKey = "space_concurrent_release_build"
	QuotaKeySpaceSSEConnections         QuotaKey = "space_sse_connections"
	QuotaKeySpaceImportBytes            QuotaKey = "space_import_bytes"
	QuotaKeySpaceStorageBytes           QuotaKey = "space_storage_bytes"
)

type QuotaLimits struct {
	SpaceConcurrentGeneration   int
	SpaceConcurrentRegeneration int
	SpaceConcurrentProcessing   int
	SpaceConcurrentQuality      int
	SpaceConcurrentReleaseBuild int
	SpaceSSEConnections         int
	SpaceImportBytes            int64
	SpaceStorageBytes           int64
	GlobalStorageBytes          int64
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
	case QuotaKeySpaceConcurrentGeneration:
		limit = int64(s.limits.SpaceConcurrentGeneration)
	case QuotaKeySpaceConcurrentRegeneration:
		limit = int64(s.limits.SpaceConcurrentRegeneration)
	case QuotaKeySpaceConcurrentProcessing:
		limit = int64(s.limits.SpaceConcurrentProcessing)
	case QuotaKeySpaceConcurrentQuality:
		limit = int64(s.limits.SpaceConcurrentQuality)
	case QuotaKeySpaceConcurrentReleaseBuild:
		limit = int64(s.limits.SpaceConcurrentReleaseBuild)
	case QuotaKeySpaceSSEConnections:
		limit = int64(s.limits.SpaceSSEConnections)
	default:
		return true
	}
	if maximum > 0 && limit > maximum {
		limit = maximum
	}
	current := s.usage[key][owner]
	return current < limit
}

func (s *QuotaService) CheckStorage(spaceID string, requestedBytes int64) error {
	if requestedBytes < 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.usage[QuotaKeySpaceStorageBytes][spaceID]
	if requestedBytes > s.limits.SpaceStorageBytes-current {
		return ErrStorageQuotaExceeded
	}
	return nil
}

var DefaultQuotaLimits = QuotaLimits{
	SpaceConcurrentGeneration:   3,
	SpaceConcurrentRegeneration: 2,
	SpaceConcurrentProcessing:   3,
	SpaceConcurrentQuality:      3,
	SpaceConcurrentReleaseBuild: 1,
	SpaceSSEConnections:         5,
	SpaceImportBytes:            500 * 1024 * 1024,
	SpaceStorageBytes:           20 * 1024 * 1024 * 1024,
	GlobalStorageBytes:          50 * 1024 * 1024 * 1024,
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
