package health

import (
	"errors"
	"testing"
)

func TestNewCheckerEmpty(t *testing.T) {
	c := NewChecker()
	status := c.RunAll()
	if status.Overall != "healthy" {
		t.Errorf("expected healthy, got %s", status.Overall)
	}
	if len(status.Dependencies) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(status.Dependencies))
	}
}

func TestCheckerRegisterAndRunAllHealthy(t *testing.T) {
	c := NewChecker()
	c.Register("db", func() error { return nil })
	c.Register("cache", func() error { return nil })
	status := c.RunAll()
	if status.Overall != "healthy" {
		t.Errorf("expected healthy, got %s", status.Overall)
	}
	if len(status.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(status.Dependencies))
	}
	db := status.Dependencies["db"]
	if db.Status != DependencyUp {
		t.Errorf("expected db up, got %s", db.Status)
	}
	cache := status.Dependencies["cache"]
	if cache.Status != DependencyUp {
		t.Errorf("expected cache up, got %s", cache.Status)
	}
}

func TestCheckerRunAllUnhealthy(t *testing.T) {
	c := NewChecker()
	c.Register("db", func() error { return nil })
	c.Register("cache", func() error { return errors.New("connection refused") })
	status := c.RunAll()
	if status.Overall != "unhealthy" {
		t.Errorf("expected unhealthy, got %s", status.Overall)
	}
	cache := status.Dependencies["cache"]
	if cache.Status != DependencyDown {
		t.Errorf("expected cache down, got %s", cache.Status)
	}
	if cache.Error != "connection refused" {
		t.Errorf("expected error message, got %s", cache.Error)
	}
}

func TestCheckerGetStatusBeforeRun(t *testing.T) {
	c := NewChecker()
	c.Register("db", func() error { return nil })
	status := c.GetStatus()
	if status.Overall != "unknown" {
		t.Errorf("expected unknown, got %s", status.Overall)
	}
	db := status.Dependencies["db"]
	if db.Status != DependencyDown {
		t.Errorf("expected pre-run db status down, got %s", db.Status)
	}
}

func TestCheckerDependencyLatency(t *testing.T) {
	c := NewChecker()
	c.Register("db", func() error { return nil })
	status := c.RunAll()
	db := status.Dependencies["db"]
	if db.Latency < 0 {
		t.Error("expected non-negative latency")
	}
}
