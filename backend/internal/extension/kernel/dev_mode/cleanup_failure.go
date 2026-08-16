package dev_mode

import (
	"context"
	"errors"
	"time"
)

type CleanupFailureStatus string

const (
	CleanupFailurePending   CleanupFailureStatus = "pending"
	CleanupFailureRetrying  CleanupFailureStatus = "retrying"
	CleanupFailureResolved  CleanupFailureStatus = "resolved"
	CleanupFailureExhausted CleanupFailureStatus = "exhausted"
)

type CleanupFailureRecord struct {
	FailureID     string
	WorkspaceID   WorkspaceID
	ExtensionID   string
	OldInstanceID string
	OldGeneration int64
	NewInstanceID string
	NewGeneration int64
	ErrorCode     string
	ErrorMessage  string
	RetryCount    int
	MaxRetries    int
	NextRetryAt   time.Time
	Status        CleanupFailureStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CleanupFailureStore interface {
	Save(ctx context.Context, record *CleanupFailureRecord) error
	ListPending(ctx context.Context) ([]*CleanupFailureRecord, error)
	ListAll(ctx context.Context) ([]*CleanupFailureRecord, error)
	Delete(ctx context.Context, failureID string) error
	UpdateRetry(ctx context.Context, failureID string, retryCount int, nextRetryAt time.Time, status CleanupFailureStatus) error
	Count(ctx context.Context) (int64, error)
}

var ErrNoopCleanupFailureStore = errors.New("dev_mode: NoopCleanupFailureStore does not persist data")

type NoopCleanupFailureStore struct{}

func (NoopCleanupFailureStore) Save(context.Context, *CleanupFailureRecord) error {
	return ErrNoopCleanupFailureStore
}
func (NoopCleanupFailureStore) ListPending(context.Context) ([]*CleanupFailureRecord, error) {
	return nil, ErrNoopCleanupFailureStore
}
func (NoopCleanupFailureStore) ListAll(context.Context) ([]*CleanupFailureRecord, error) {
	return nil, ErrNoopCleanupFailureStore
}
func (NoopCleanupFailureStore) Delete(context.Context, string) error {
	return ErrNoopCleanupFailureStore
}
func (NoopCleanupFailureStore) UpdateRetry(context.Context, string, int, time.Time, CleanupFailureStatus) error {
	return ErrNoopCleanupFailureStore
}
func (NoopCleanupFailureStore) Count(context.Context) (int64, error) {
	return 0, ErrNoopCleanupFailureStore
}
