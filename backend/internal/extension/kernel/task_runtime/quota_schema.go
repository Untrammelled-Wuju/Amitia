//go:build rsi_quota

package task_runtime

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/u-ai/backend/internal/platform/process"
)

const (
	QuotaSchemaVersion = 1
	MinQuotaCPUPercent = 1.0
)

var (
	ErrQuotaRejected       = errors.New("task_runtime: resource quota rejected")
	ErrQuotaBudgetExceeded = errors.New("task_runtime: resource quota budget exceeded")
)

type ResourceQuota struct {
	MemoryBytes int64   `json:"memoryBytes,omitempty"`
	CPUPercent  float64 `json:"cpuPercent,omitempty"`
	Processes   int64   `json:"processes,omitempty"`
	Concurrency int64   `json:"concurrency,omitempty"`
	DiskBytes   int64   `json:"diskBytes,omitempty"`
}

func (ResourceQuota) SchemaVersion() int {
	return QuotaSchemaVersion
}

func (q ResourceQuota) IsZero() bool {
	return q.MemoryBytes == 0 && q.CPUPercent == 0 && q.Processes == 0 && q.Concurrency == 0 && q.DiskBytes == 0
}

type QuotaCeiling struct {
	MemoryBytes int64   `json:"memoryBytes"`
	CPUPercent  float64 `json:"cpuPercent"`
	Processes   int64   `json:"processes"`
	Concurrency int64   `json:"concurrency"`
	DiskBytes   int64   `json:"diskBytes"`
}

func DefaultQuotaCeiling() QuotaCeiling {
	return QuotaCeiling{
		MemoryBytes: 512 << 20,
		CPUPercent:  50,
		Processes:   int64(taskProcessPIDBudget(runtime.GOOS)),
		Concurrency: 4,
		DiskBytes:   256 << 20,
	}
}

func (c QuotaCeiling) Validate() error {
	values := []struct {
		name  string
		value float64
	}{
		{"memoryBytes", float64(c.MemoryBytes)},
		{"cpuPercent", c.CPUPercent},
		{"processes", float64(c.Processes)},
		{"concurrency", float64(c.Concurrency)},
		{"diskBytes", float64(c.DiskBytes)},
	}
	violations := make([]QuotaViolation, 0, len(values))
	for _, item := range values {
		if math.IsNaN(item.value) || math.IsInf(item.value, 0) {
			violations = append(violations, QuotaViolation{Dimension: item.name, Value: item.value, Reason: "not_finite"})
			continue
		}
		if item.value <= 0 {
			violations = append(violations, QuotaViolation{Dimension: item.name, Value: item.value, Reason: "not_positive"})
		}
	}
	if len(violations) > 0 {
		return &QuotaRejection{Violations: violations}
	}
	return nil
}

type QuotaViolation struct {
	Dimension string  `json:"dimension"`
	Value     float64 `json:"value"`
	Reason    string  `json:"reason"`
}

type QuotaRejection struct {
	Violations []QuotaViolation `json:"violations"`
}

func (e *QuotaRejection) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, violation := range e.Violations {
		parts = append(parts, fmt.Sprintf("%s=%g(%s)", violation.Dimension, violation.Value, violation.Reason))
	}
	return fmt.Sprintf("%v: %v", ErrQuotaRejected, parts)
}

func (e *QuotaRejection) Is(target error) bool {
	return target == ErrQuotaRejected
}

func (e *QuotaRejection) Unwrap() error {
	return WrapTaskError(ErrTaskDefinitionInvalid, "resource quota rejected", ErrQuotaRejected)
}

func ValidateQuota(quota ResourceQuota) []QuotaViolation {
	values := []struct {
		name  string
		value float64
	}{
		{"memoryBytes", float64(quota.MemoryBytes)},
		{"cpuPercent", quota.CPUPercent},
		{"processes", float64(quota.Processes)},
		{"concurrency", float64(quota.Concurrency)},
		{"diskBytes", float64(quota.DiskBytes)},
	}
	violations := make([]QuotaViolation, 0, len(values))
	for _, item := range values {
		if math.IsNaN(item.value) || math.IsInf(item.value, 0) {
			violations = append(violations, QuotaViolation{Dimension: item.name, Value: item.value, Reason: "not_finite"})
			continue
		}
		if item.value < 0 {
			violations = append(violations, QuotaViolation{Dimension: item.name, Value: item.value, Reason: "negative"})
		}
	}
	return violations
}

func NormalizeQuota(quota ResourceQuota, ceiling QuotaCeiling) (ResourceQuota, []string, error) {
	if err := ceiling.Validate(); err != nil {
		return ResourceQuota{}, nil, fmt.Errorf("quota ceiling: %w", err)
	}
	if violations := ValidateQuota(quota); len(violations) > 0 {
		return ResourceQuota{}, nil, &QuotaRejection{Violations: violations}
	}
	notes := make([]string, 0, 5)
	normalized := quota
	normalized.MemoryBytes, notes = normalizeIntQuota("memoryBytes", quota.MemoryBytes, ceiling.MemoryBytes, notes)
	normalized.CPUPercent, notes = normalizeCPUQuota(quota.CPUPercent, ceiling.CPUPercent, notes)
	normalized.Processes, notes = normalizeIntQuota("processes", quota.Processes, ceiling.Processes, notes)
	normalized.Concurrency, notes = normalizeIntQuota("concurrency", quota.Concurrency, ceiling.Concurrency, notes)
	normalized.DiskBytes, notes = normalizeIntQuota("diskBytes", quota.DiskBytes, ceiling.DiskBytes, notes)
	sort.Strings(notes)
	return normalized, notes, nil
}

func normalizeIntQuota(name string, value, ceiling int64, notes []string) (int64, []string) {
	switch {
	case value == 0:
		return ceiling, append(notes, name+"=default")
	case value > ceiling:
		return ceiling, append(notes, name+"=clamped")
	default:
		return value, notes
	}
}

func normalizeCPUQuota(value, ceiling float64, notes []string) (float64, []string) {
	switch {
	case value == 0:
		return ceiling, append(notes, "cpuPercent=default")
	case value < MinQuotaCPUPercent:
		return MinQuotaCPUPercent, append(notes, "cpuPercent=floor")
	case value > ceiling:
		return ceiling, append(notes, "cpuPercent=clamped")
	default:
		return value, notes
	}
}

func ApplyQuotaSupport(quota ResourceQuota, support process.ResourceLimitSupport) (ResourceQuota, []string) {
	notes := make([]string, 0, 3)
	applied := quota
	if !support.Memory && applied.MemoryBytes != 0 {
		applied.MemoryBytes = 0
		notes = append(notes, "memoryBytes=unenforced")
	}
	if !support.CPU && applied.CPUPercent != 0 {
		applied.CPUPercent = 0
		notes = append(notes, "cpuPercent=unenforced")
	}
	if !support.Processes && applied.Processes != 0 {
		applied.Processes = 0
		notes = append(notes, "processes=unenforced")
	}
	sort.Strings(notes)
	return applied, notes
}

type QuotaBudgetError struct {
	Dimension string  `json:"dimension"`
	Requested float64 `json:"requested"`
	Used      float64 `json:"used"`
	Budget    float64 `json:"budget"`
}

func (e *QuotaBudgetError) Error() string {
	return fmt.Sprintf("%v: %s requested=%g used=%g budget=%g", ErrQuotaBudgetExceeded, e.Dimension, e.Requested, e.Used, e.Budget)
}

func (e *QuotaBudgetError) Is(target error) bool {
	return target == ErrQuotaBudgetExceeded
}

type QuotaLedger struct {
	mu     sync.Mutex
	budget QuotaCeiling
	used   ResourceQuota
}

func NewQuotaLedger(budget QuotaCeiling) (*QuotaLedger, error) {
	if err := budget.Validate(); err != nil {
		return nil, fmt.Errorf("quota budget: %w", err)
	}
	return &QuotaLedger{budget: budget}, nil
}

func (l *QuotaLedger) Admit(quota ResourceQuota) error {
	if violations := ValidateQuota(quota); len(violations) > 0 {
		return &QuotaRejection{Violations: violations}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	checks := []struct {
		name      string
		requested float64
		used      float64
		budget    float64
	}{
		{"memoryBytes", float64(quota.MemoryBytes), float64(l.used.MemoryBytes), float64(l.budget.MemoryBytes)},
		{"cpuPercent", quota.CPUPercent, l.used.CPUPercent, l.budget.CPUPercent},
		{"processes", float64(quota.Processes), float64(l.used.Processes), float64(l.budget.Processes)},
		{"concurrency", float64(quota.Concurrency), float64(l.used.Concurrency), float64(l.budget.Concurrency)},
		{"diskBytes", float64(quota.DiskBytes), float64(l.used.DiskBytes), float64(l.budget.DiskBytes)},
	}
	for _, check := range checks {
		if check.used+check.requested > check.budget+1e-9 {
			return &QuotaBudgetError{Dimension: check.name, Requested: check.requested, Used: check.used, Budget: check.budget}
		}
	}
	l.used.MemoryBytes += quota.MemoryBytes
	l.used.CPUPercent += quota.CPUPercent
	l.used.Processes += quota.Processes
	l.used.Concurrency += quota.Concurrency
	l.used.DiskBytes += quota.DiskBytes
	return nil
}

func (l *QuotaLedger) Release(quota ResourceQuota) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.used.MemoryBytes = releaseInt(l.used.MemoryBytes, quota.MemoryBytes)
	l.used.CPUPercent = releaseFloat(l.used.CPUPercent, quota.CPUPercent)
	l.used.Processes = releaseInt(l.used.Processes, quota.Processes)
	l.used.Concurrency = releaseInt(l.used.Concurrency, quota.Concurrency)
	l.used.DiskBytes = releaseInt(l.used.DiskBytes, quota.DiskBytes)
}

func releaseInt(used, released int64) int64 {
	if released >= used {
		return 0
	}
	return used - released
}

func releaseFloat(used, released float64) float64 {
	if released >= used {
		return 0
	}
	return used - released
}

func (l *QuotaLedger) Snapshot() ResourceQuota {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.used
}
