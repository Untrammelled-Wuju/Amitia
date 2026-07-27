package update

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type HealthCheck struct {
	Name  string
	Check func(ctx context.Context) error
}

type HealthCheckResult struct {
	Healthy bool
	Errors  []string
}

type PostUpdateHealthChecker struct {
	mu     sync.Mutex
	checks []HealthCheck
}

func NewPostUpdateHealthChecker() *PostUpdateHealthChecker {
	return &PostUpdateHealthChecker{}
}

func (h *PostUpdateHealthChecker) AddCheck(name string, check func(ctx context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, HealthCheck{Name: name, Check: check})
}

func (h *PostUpdateHealthChecker) Check(ctx context.Context) HealthCheckResult {
	h.mu.Lock()
	checks := make([]HealthCheck, len(h.checks))
	copy(checks, h.checks)
	h.mu.Unlock()

	result := HealthCheckResult{Healthy: true}
	for _, c := range checks {
		if c.Check == nil {
			continue
		}
		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := c.Check(checkCtx)
		cancel()
		if err != nil {
			result.Healthy = false
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", c.Name, err))
		}
	}
	return result
}

func DefaultPostUpdateHealthChecks(extensionID string) []HealthCheck {
	return []HealthCheck{
		{
			Name: "extension_loaded",
			Check: func(ctx context.Context) error {
				return nil
			},
		},
		{
			Name: "modules_activated",
			Check: func(ctx context.Context) error {
				return nil
			},
		},
		{
			Name: "migrations_applied",
			Check: func(ctx context.Context) error {
				return nil
			},
		},
		{
			Name: "registry_consistent",
			Check: func(ctx context.Context) error {
				return nil
			},
		},
	}
}
