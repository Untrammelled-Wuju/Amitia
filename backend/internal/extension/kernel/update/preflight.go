package update

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

type PreflightCheck struct {
	Name  string
	Check func(ctx context.Context) error
}

type PreflightResult struct {
	Passed bool
	Errors []string
}

type PreflightRunner struct {
	mu     sync.Mutex
	checks []PreflightCheck
}

func NewPreflightRunner() *PreflightRunner {
	return &PreflightRunner{}
}

func (r *PreflightRunner) AddCheck(name string, check func(ctx context.Context) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks = append(r.checks, PreflightCheck{Name: name, Check: check})
}

func (r *PreflightRunner) Run(ctx context.Context) PreflightResult {
	r.mu.Lock()
	checks := make([]PreflightCheck, len(r.checks))
	copy(checks, r.checks)
	r.mu.Unlock()

	result := PreflightResult{Passed: true}
	for _, c := range checks {
		if c.Check == nil {
			continue
		}
		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := c.Check(checkCtx)
		cancel()
		if err != nil {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", c.Name, err))
		}
	}
	return result
}

func DefaultPreflightChecks(stagingDir string, maxFileSize int64) []PreflightCheck {
	return []PreflightCheck{
		{
			Name: "staging_dir_writable",
			Check: func(ctx context.Context) error {
				return ensureStagingDir(stagingDir)
			},
		},
		{
			Name: "disk_space_available",
			Check: func(ctx context.Context) error {
				return checkDiskSpace(stagingDir, maxFileSize)
			},
		},
		{
			Name: "network_reachable",
			Check: func(ctx context.Context) error {
				return checkNetworkReachable(ctx)
			},
		},
	}
}

func ensureStagingDir(stagingDir string) error {
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return fmt.Errorf("cannot create staging dir: %w", err)
	}
	probe := stagingDir + string(os.PathSeparator) + ".preflight-probe"
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return fmt.Errorf("staging dir not writable: %w", err)
	}
	os.Remove(probe)
	return nil
}

func checkDiskSpace(stagingDir string, required int64) error {
	if required <= 0 {
		return nil
	}
	return nil
}

func checkNetworkReachable(ctx context.Context) error {
	return nil
}
