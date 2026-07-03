package health

import (
	"sync"
	"time"
)

type DependencyStatus string

const (
	DependencyUp       DependencyStatus = "up"
	DependencyDown     DependencyStatus = "down"
	DependencyDegraded DependencyStatus = "degraded"
)

type HealthStatus struct {
	Overall      string                    `json:"overall"`
	Dependencies map[string]DependencyInfo `json:"dependencies"`
	CheckedAt    time.Time                 `json:"checkedAt"`
}

type DependencyInfo struct {
	Name      string           `json:"name"`
	Status    DependencyStatus `json:"status"`
	LastCheck time.Time        `json:"lastCheck"`
	Latency   time.Duration    `json:"latency"`
	Error     string           `json:"error,omitempty"`
}

type Checker struct {
	mu     sync.RWMutex
	deps   map[string]DependencyInfo
	checks map[string]func() error
}

func NewChecker() *Checker {
	return &Checker{
		deps:   map[string]DependencyInfo{},
		checks: map[string]func() error{},
	}
}

func (c *Checker) Register(name string, check func() error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
	c.deps[name] = DependencyInfo{Name: name, Status: DependencyDown}
}

func (c *Checker) RunAll() HealthStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	allUp := true
	now := time.Now().UTC()
	result := map[string]DependencyInfo{}

	for name, check := range c.checks {
		start := time.Now()
		err := check()
		latency := time.Since(start)

		info := DependencyInfo{
			Name:      name,
			LastCheck: now,
			Latency:   latency,
		}

		if err != nil {
			info.Status = DependencyDown
			info.Error = err.Error()
			allUp = false
		} else {
			info.Status = DependencyUp
		}

		c.deps[name] = info
		result[name] = info
	}

	overall := "healthy"
	if !allUp {
		overall = "unhealthy"
	}

	return HealthStatus{
		Overall:      overall,
		Dependencies: result,
		CheckedAt:    now,
	}
}

func (c *Checker) GetStatus() HealthStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := map[string]DependencyInfo{}
	for name, info := range c.deps {
		result[name] = info
	}

	return HealthStatus{
		Overall:      "unknown",
		Dependencies: result,
		CheckedAt:    time.Now().UTC(),
	}
}
