// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import "time"

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

func NewClock() Clock { return realClock{} }

type fakeClock struct {
	now time.Time
}

func NewFakeClock(now time.Time) Clock { return fakeClock{now: now} }

func (f fakeClock) Now() time.Time { return f.now }
