// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import "time"

type Backoff struct {
	initial  time.Duration
	max      time.Duration
	multiplier float64
	current  time.Duration
	count    int
}

func NewBackoff(initial, max time.Duration, multiplier float64) *Backoff {
	return &Backoff{
		initial:    initial,
		max:        max,
		multiplier: multiplier,
		current:    initial,
	}
}

func NewBackoffFromPolicy(policy Policy) *Backoff {
	return NewBackoff(policy.InitialDelay, policy.MaxDelay, policy.Multiplier)
}

func (b *Backoff) Next() time.Duration {
	delay := b.current
	if b.current < b.max {
		b.current = time.Duration(float64(b.current) * b.multiplier)
		if b.current > b.max {
			b.current = b.max
		}
	}
	b.count++
	return delay
}

func (b *Backoff) Reset() {
	b.current = b.initial
	b.count = 0
}

func (b *Backoff) Count() int {
	return b.count
}

func (b *Backoff) Current() time.Duration {
	return b.current
}

type FixedBackoff struct {
	delay time.Duration
}

func NewFixedBackoff(delay time.Duration) *FixedBackoff {
	return &FixedBackoff{delay: delay}
}

func (b *FixedBackoff) Next() time.Duration {
	return b.delay
}

func (b *FixedBackoff) Reset() {}

func (b *FixedBackoff) Count() int {
	return 0
}

func (b *FixedBackoff) Current() time.Duration {
	return b.delay
}
