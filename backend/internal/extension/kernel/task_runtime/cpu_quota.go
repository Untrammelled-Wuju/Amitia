//go:build rsi_cpuquota

package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const (
	CPUQuotaPeriodMicros = 100000
	MaxCPUQuotaPercent   = 800.0
)

var (
	ErrCPUQuotaInvalid    = errors.New("task_runtime: cpu quota invalid")
	ErrCPUQuotaNotApplied = errors.New("task_runtime: cpu quota not applied")
)

type CPUQuotaPlan struct {
	QuotaMicros  int64
	PeriodMicros int64
}

func (p CPUQuotaPlan) CgroupValue() string {
	return fmt.Sprintf("%d %d", p.QuotaMicros, p.PeriodMicros)
}

func (p CPUQuotaPlan) Cores() float64 {
	if p.PeriodMicros <= 0 {
		return 0
	}
	return float64(p.QuotaMicros) / float64(p.PeriodMicros)
}

func PlanCPUQuota(percent float64) (CPUQuotaPlan, error) {
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		return CPUQuotaPlan{}, fmt.Errorf("%w: percent=%v not finite", ErrCPUQuotaInvalid, percent)
	}
	if percent <= 0 || percent > MaxCPUQuotaPercent {
		return CPUQuotaPlan{}, fmt.Errorf("%w: percent=%v bounds=(0,%v]", ErrCPUQuotaInvalid, percent, MaxCPUQuotaPercent)
	}
	quota := int64(math.Round(percent / 100 * CPUQuotaPeriodMicros))
	if quota < 1 {
		quota = 1
	}
	return CPUQuotaPlan{QuotaMicros: quota, PeriodMicros: CPUQuotaPeriodMicros}, nil
}

type CPUQuotaFS interface {
	WriteFile(path string, data []byte, perm fs.FileMode) error
	ReadFile(path string) ([]byte, error)
}

type OSCPUQuotaFS struct{}

func (OSCPUQuotaFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (OSCPUQuotaFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func ApplyCPUQuota(ctx context.Context, fsys CPUQuotaFS, directory string, percent float64) (CPUQuotaPlan, error) {
	plan, err := PlanCPUQuota(percent)
	if err != nil {
		return CPUQuotaPlan{}, err
	}
	if err := ctx.Err(); err != nil {
		return CPUQuotaPlan{}, err
	}
	if fsys == nil {
		return CPUQuotaPlan{}, fmt.Errorf("%w: nil filesystem", ErrCPUQuotaInvalid)
	}
	if strings.TrimSpace(directory) == "" {
		return CPUQuotaPlan{}, fmt.Errorf("%w: empty cgroup directory", ErrCPUQuotaInvalid)
	}
	target := filepath.Join(directory, "cpu.max")
	want := plan.CgroupValue()
	if err := fsys.WriteFile(target, []byte(want), 0o644); err != nil {
		return CPUQuotaPlan{}, fmt.Errorf("write cpu.max: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return CPUQuotaPlan{}, err
	}
	got, err := fsys.ReadFile(target)
	if err != nil {
		return CPUQuotaPlan{}, fmt.Errorf("read cpu.max: %w", err)
	}
	if strings.TrimSpace(string(got)) != want {
		return CPUQuotaPlan{}, fmt.Errorf("%w: got=%q want=%q", ErrCPUQuotaNotApplied, strings.TrimSpace(string(got)), want)
	}
	return plan, nil
}
