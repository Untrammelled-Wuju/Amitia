// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"testing"
	"time"
)

func TestNewBackoff(t *testing.T) {
	b := NewBackoff(100*time.Millisecond, 5*time.Second, 2.0)
	if b.initial != 100*time.Millisecond {
		t.Errorf("initial = %v", b.initial)
	}
	if b.max != 5*time.Second {
		t.Errorf("max = %v", b.max)
	}
	if b.multiplier != 2.0 {
		t.Errorf("multiplier = %v", b.multiplier)
	}
}

func TestBackoffNext(t *testing.T) {
	b := NewBackoff(100*time.Millisecond, 1*time.Second, 2.0)

	expected := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1 * time.Second,
		1 * time.Second,
	}

	for i, want := range expected {
		got := b.Next()
		if got != want {
			t.Errorf("iteration %d: got %v, want %v", i, got, want)
		}
	}
}

func TestBackoffReset(t *testing.T) {
	b := NewBackoff(100*time.Millisecond, 5*time.Second, 2.0)
	b.Next()
	b.Next()
	b.Reset()
	if b.current != 100*time.Millisecond {
		t.Errorf("current after reset = %v", b.current)
	}
	if b.count != 0 {
		t.Errorf("count after reset = %d", b.count)
	}
}

func TestBackoffCount(t *testing.T) {
	b := NewBackoff(100*time.Millisecond, 5*time.Second, 2.0)
	for i := 0; i < 5; i++ {
		b.Next()
	}
	if b.Count() != 5 {
		t.Errorf("Count() = %d, want 5", b.Count())
	}
}

func TestBackoffFromPolicy(t *testing.T) {
	p := NewPolicy()
	b := NewBackoffFromPolicy(p)
	if b.initial != p.InitialDelay {
		t.Errorf("initial = %v", b.initial)
	}
	if b.max != p.MaxDelay {
		t.Errorf("max = %v", b.max)
	}
	if b.multiplier != p.Multiplier {
		t.Errorf("multiplier = %v", b.multiplier)
	}
}

func TestFixedBackoff(t *testing.T) {
	b := NewFixedBackoff(500 * time.Millisecond)
	if b.Next() != 500*time.Millisecond {
		t.Errorf("Next() = %v", b.Next())
	}
	b.Reset()
	if b.Count() != 0 {
		t.Error("FixedBackoff Count should be 0")
	}
	if b.Current() != 500*time.Millisecond {
		t.Errorf("Current() = %v", b.Current())
	}
}
