// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"sync"
	"time"
)

type loginAttempt struct {
	count     int
	expiresAt time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: map[string]loginAttempt{}}
}

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	attempt, exists := l.attempts[key]
	if !exists || now.After(attempt.expiresAt) {
		delete(l.attempts, key)
		return true
	}
	return attempt.count < 5
}

func (l *loginLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	attempt, exists := l.attempts[key]
	if !exists || now.After(attempt.expiresAt) {
		l.attempts[key] = loginAttempt{count: 1, expiresAt: now.Add(10 * time.Minute)}
		return
	}
	attempt.count++
	l.attempts[key] = attempt
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}
