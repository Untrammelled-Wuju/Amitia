//go:build rsi_memquota

package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	MinMemoryQuotaBytes = 4 << 20
	MaxMemoryQuotaBytes = 8 << 30
)

var (
	ErrMemoryQuotaInvalid    = errors.New("task_runtime: memory quota invalid")
	ErrMemoryQuotaNotApplied = errors.New("task_runtime: memory quota not applied")
)

type MemoryQuotaPlan struct {
	Bytes     int64
	Unlimited bool
}

func (p MemoryQuotaPlan) CgroupValue() string {
	if p.Unlimited {
		return "max"
	}
	return strconv.FormatInt(p.Bytes, 10)
}

func PlanMemoryQuota(bytes int64) (MemoryQuotaPlan, error) {
	if bytes == 0 {
		return MemoryQuotaPlan{Unlimited: true}, nil
	}
	if bytes < MinMemoryQuotaBytes || bytes > MaxMemoryQuotaBytes {
		return MemoryQuotaPlan{}, fmt.Errorf("%w: bytes=%d bounds=[%d,%d]", ErrMemoryQuotaInvalid, bytes, MinMemoryQuotaBytes, MaxMemoryQuotaBytes)
	}
	return MemoryQuotaPlan{Bytes: bytes}, nil
}

type MemoryQuotaFS interface {
	WriteFile(path string, data []byte, perm fs.FileMode) error
	ReadFile(path string) ([]byte, error)
}

type OSMemoryQuotaFS struct{}

func (OSMemoryQuotaFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (OSMemoryQuotaFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func ApplyMemoryQuota(ctx context.Context, fsys MemoryQuotaFS, directory string, bytes int64) (MemoryQuotaPlan, error) {
	plan, err := PlanMemoryQuota(bytes)
	if err != nil {
		return MemoryQuotaPlan{}, err
	}
	if err := ctx.Err(); err != nil {
		return MemoryQuotaPlan{}, err
	}
	if fsys == nil {
		return MemoryQuotaPlan{}, fmt.Errorf("%w: nil filesystem", ErrMemoryQuotaInvalid)
	}
	if strings.TrimSpace(directory) == "" {
		return MemoryQuotaPlan{}, fmt.Errorf("%w: empty cgroup directory", ErrMemoryQuotaInvalid)
	}
	target := filepath.Join(directory, "memory.max")
	want := plan.CgroupValue()
	if err := fsys.WriteFile(target, []byte(want), 0o644); err != nil {
		return MemoryQuotaPlan{}, fmt.Errorf("write memory.max: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return MemoryQuotaPlan{}, err
	}
	got, err := fsys.ReadFile(target)
	if err != nil {
		return MemoryQuotaPlan{}, fmt.Errorf("read memory.max: %w", err)
	}
	if strings.TrimSpace(string(got)) != want {
		return MemoryQuotaPlan{}, fmt.Errorf("%w: got=%q want=%q", ErrMemoryQuotaNotApplied, strings.TrimSpace(string(got)), want)
	}
	return plan, nil
}

func ReadOOMKills(ctx context.Context, fsys MemoryQuotaFS, directory string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if fsys == nil {
		return 0, fmt.Errorf("%w: nil filesystem", ErrMemoryQuotaInvalid)
	}
	data, err := fsys.ReadFile(filepath.Join(directory, "memory.events"))
	if err != nil {
		return 0, fmt.Errorf("read memory.events: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "oom_kill" {
			continue
		}
		count, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse oom_kill: %w", err)
		}
		return count, nil
	}
	return 0, nil
}
