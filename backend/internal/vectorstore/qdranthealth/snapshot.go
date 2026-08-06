// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import "time"

type Snapshot struct {
	State          State
	Target         Target
	Identity       Identity
	LastResult     ProbeResult
	LastError      error
	Attempts       int
	FirstAttemptAt time.Time
	LastAttemptAt  time.Time
	ReadyAt        time.Time
	Duration       time.Duration
	Metadata       map[string]string
}

func NewSnapshot(state State, target Target) Snapshot {
	now := time.Now()
	return Snapshot{
		State:          state,
		Target:         target,
		FirstAttemptAt: now,
		LastAttemptAt:  now,
		Metadata:       make(map[string]string),
	}
}

func (s Snapshot) WithLastResult(r ProbeResult) Snapshot {
	s.LastResult = r
	s.LastAttemptAt = r.Timestamp
	if s.Attempts == 0 {
		s.FirstAttemptAt = r.Timestamp
	}
	s.Attempts++
	if r.Err != nil {
		s.LastError = r.Err
	}
	return s
}

func (s Snapshot) WithState(state State) Snapshot {
	s.State = state
	s.LastAttemptAt = time.Now()
	if state == StateReady && s.ReadyAt.IsZero() {
		s.ReadyAt = s.LastAttemptAt
		s.Duration = s.ReadyAt.Sub(s.FirstAttemptAt)
	}
	return s
}

func (s Snapshot) WithMetadata(key, value string) Snapshot {
	if s.Metadata == nil {
		s.Metadata = make(map[string]string)
	}
	s.Metadata[key] = value
	return s
}

func (s Snapshot) IsHealthy() bool {
	return s.State.IsReady() && s.LastResult.IsOK()
}

func (s Snapshot) IsUnhealthy() bool {
	return !s.LastResult.IsOK() && s.LastResult.Err != nil
}

func (s Snapshot) Clone() Snapshot {
	clone := Snapshot{
		State:          s.State,
		Target:         s.Target,
		Identity:       s.Identity,
		LastResult:     s.LastResult,
		LastError:      s.LastError,
		Attempts:       s.Attempts,
		FirstAttemptAt: s.FirstAttemptAt,
		LastAttemptAt:  s.LastAttemptAt,
		ReadyAt:        s.ReadyAt,
		Duration:       s.Duration,
	}
	if s.Metadata != nil {
		clone.Metadata = make(map[string]string, len(s.Metadata))
		for k, v := range s.Metadata {
			clone.Metadata[k] = v
		}
	}
	return clone
}

func (s Snapshot) Reset() Snapshot {
	s.State = StateProcessStarted
	s.LastResult = ProbeResult{}
	s.LastError = nil
	s.Attempts = 0
	s.FirstAttemptAt = time.Now()
	s.LastAttemptAt = s.FirstAttemptAt
	s.ReadyAt = time.Time{}
	s.Duration = 0
	return s
}
