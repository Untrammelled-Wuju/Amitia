package resource

import (
	"math"
	"testing"
)

func TestAddInt64OrCap_Normal(t *testing.T) {
	got, overflow := addInt64OrCap(100, 200)
	if overflow || got != 300 {
		t.Fatalf("expected (300,false), got (%d,%v)", got, overflow)
	}
}

func TestAddInt64OrCap_PositiveOverflow(t *testing.T) {
	got, overflow := addInt64OrCap(math.MaxInt64, 1)
	if !overflow || got != math.MaxInt64 {
		t.Fatalf("expected溢出 capped at MaxInt64, got (%d,%v)", got, overflow)
	}
}

func TestAddInt64OrCap_NegativeOverflow(t *testing.T) {
	got, overflow := addInt64OrCap(math.MinInt64, -1)
	if !overflow || got != math.MinInt64 {
		t.Fatalf("expected overflow capped at MinInt64, got (%d,%v)", got, overflow)
	}
}

func TestSafeLte_WithinLimit(t *testing.T) {
	if !safeLte(100, 200, 500) {
		t.Fatal("expected within limit")
	}
}

func TestSafeLte_OverLimit(t *testing.T) {
	if safeLte(300, 300, 500) {
		t.Fatal("expected surpassed limit")
	}
}

func TestSafeLte_NegativeLimit(t *testing.T) {
	if safeLte(0, 10, -1) {
		t.Fatal("negative limit should deny")
	}
}

func TestSafeLte_NegativeRequest(t *testing.T) {
	if safeLte(0, -5, 100) {
		t.Fatal("negative request should deny")
	}
}

func TestSafeLte_OverflowRejects(t *testing.T) {
	if safeLte(math.MaxInt64, math.MaxInt64, math.MaxInt64) {
		t.Fatal("must reject overflow of sum")
	}
}
