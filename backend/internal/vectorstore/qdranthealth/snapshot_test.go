// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"testing"
	"time"
)

func TestNewSnapshot(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	s := NewSnapshot(StateProcessStarted, target)

	if s.State != StateProcessStarted {
		t.Errorf("State = %v", s.State)
	}
	if s.Target.BaseURL != target.BaseURL {
		t.Errorf("Target = %v", s.Target)
	}
	if s.Attempts != 0 {
		t.Errorf("Attempts = %d", s.Attempts)
	}
	if len(s.Metadata) != 0 {
		t.Errorf("Metadata should be empty, got %v", s.Metadata)
	}
}

func TestSnapshotWithLastResult(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	s := NewSnapshot(StateProcessStarted, target)

	result := ProbeResult{Status: StatusOK}
	s = s.WithLastResult(result)

	if s.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", s.Attempts)
	}
	if s.LastResult.Status != StatusOK {
		t.Errorf("LastResult.Status = %s", s.LastResult.Status)
	}

	errResult := ProbeResult{Status: StatusUnreachable, Err: ErrProbeTimeout}
	s = s.WithLastResult(errResult)

	if s.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", s.Attempts)
	}
	if s.LastError != ErrProbeTimeout {
		t.Errorf("LastError = %v", s.LastError)
	}
}

func TestSnapshotWithState(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	s := NewSnapshot(StateProcessStarted, target)
	time.Sleep(10 * time.Millisecond)

	s = s.WithState(StateIdentityConfirmed)
	if s.State != StateIdentityConfirmed {
		t.Errorf("State = %v", s.State)
	}

	s = s.WithState(StateReady)
	if s.State != StateReady {
		t.Errorf("State = %v", s.State)
	}
	if s.ReadyAt.IsZero() {
		t.Error("ReadyAt should be set")
	}
	if s.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestSnapshotWithMetadata(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	s := NewSnapshot(StateProcessStarted, target)

	s = s.WithMetadata("key1", "value1")
	if s.Metadata["key1"] != "value1" {
		t.Errorf("Metadata = %v", s.Metadata)
	}
}

func TestSnapshotIsHealthy(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	s := NewSnapshot(StateReady, target)
	s.LastResult = ProbeResult{Status: StatusOK}

	if !s.IsHealthy() {
		t.Error("expected healthy snapshot")
	}

	s.State = StateLive
	if s.IsHealthy() {
		t.Error("live snapshot should not be healthy")
	}
}

func TestSnapshotIsUnhealthy(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	s := NewSnapshot(StateLive, target)
	s.LastResult = ProbeResult{Status: StatusUnreachable, Err: ErrProbeFailed}

	if !s.IsUnhealthy() {
		t.Error("expected unhealthy snapshot")
	}
}

func TestSnapshotClone(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	s := NewSnapshot(StateReady, target)
	s.Metadata["key"] = "value"

	clone := s.Clone()
	if clone.State != s.State {
		t.Errorf("Clone State = %v", clone.State)
	}
	if clone.Metadata["key"] != "value" {
		t.Errorf("Clone Metadata = %v", clone.Metadata)
	}

	clone.Metadata["key"] = "modified"
	if s.Metadata["key"] != "value" {
		t.Error("modifying clone should not affect original")
	}
}

func TestSnapshotReset(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	s := NewSnapshot(StateReady, target)
	s.Attempts = 5

	s = s.Reset()
	if s.State != StateProcessStarted {
		t.Errorf("State = %v", s.State)
	}
	if s.Attempts != 0 {
		t.Errorf("Attempts = %d", s.Attempts)
	}
}
