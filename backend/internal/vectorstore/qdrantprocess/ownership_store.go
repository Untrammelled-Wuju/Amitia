// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type OwnershipStore interface {
	Read(ctx context.Context) (OwnRecord, error)
	Create(ctx context.Context, rec OwnRecord) error
	Update(ctx context.Context, rec OwnRecord) error
	Delete(ctx context.Context) error
	Exists(ctx context.Context) (bool, error)
}

type OwnRecord = OwnershipRecord

type JSONOwnershipStore struct {
	root  string
	fs    FileSystem
	mu    sync.Mutex
	clock Clock
}

func NewOwnershipStore(root string, fs FileSystem, clock Clock) (*JSONOwnershipStore, error) {
	if root == "" {
		return nil, fmt.Errorf("qdrantprocess: empty ownership root")
	}
	if fs == nil {
		fs = NewFileSystem()
	}
	if clock == nil {
		clock = NewClock()
	}
	store := &JSONOwnershipStore{root: root, fs: fs, clock: clock}
	return store, nil
}

func (s *JSONOwnershipStore) ensureRoot() error {
	return s.fs.MkdirAll(s.root, 0700)
}

func (s *JSONOwnershipStore) ensureActive() error {
	return s.fs.Mkdir(activeDirPath(s.root), 0700)
}

func (s *JSONOwnershipStore) Read(ctx context.Context) (OwnRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.fs.ReadFile(ownershipFilePath(s.root))
	if err != nil {
		if os.IsNotExist(err) {
			return OwnRecord{}, ErrOwnershipRecordNotFound
		}
		return OwnRecord{}, err
	}
	var rec OwnRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return OwnRecord{}, fmt.Errorf("%w: %v", ErrOwnershipRecordCorrupted, err)
	}
	if err := rec.Validate(); err != nil {
		return OwnRecord{}, err
	}
	return rec, nil
}

func (s *JSONOwnershipStore) Create(ctx context.Context, rec OwnRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureRoot(); err != nil {
		return err
	}
	if err := s.ensureActive(); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	now := s.clock.Now().UnixMilli()
	rec.CreatedAtEpochMillis = now
	rec.UpdatedAtEpochMillis = now

	data, err := marshalRecord(rec)
	if err != nil {
		return err
	}
	return atomicWriteFile(s.fs, ownershipFilePath(s.root), data, 0600)
}

func (s *JSONOwnershipStore) Update(ctx context.Context, rec OwnRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.readUnlocked()
	if err != nil {
		return err
	}
	if existing.LaunchID != rec.LaunchID {
		return fmt.Errorf("%w: launch ID mismatch", ErrLeaseOwnershipLost)
	}
	rec.UpdatedAtEpochMillis = s.clock.Now().UnixMilli()
	data, err := marshalRecord(rec)
	if err != nil {
		return err
	}
	return atomicWriteFile(s.fs, ownershipFilePath(s.root), data, 0600)
}

func (s *JSONOwnershipStore) Delete(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := activeDirPath(s.root)
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return ErrOwnershipRecordInvalid
	}
	return s.fs.Remove(path)
}

func (s *JSONOwnershipStore) Exists(ctx context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.fs.Stat(ownershipFilePath(s.root))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *JSONOwnershipStore) readUnlocked() (OwnRecord, error) {
	data, err := s.fs.ReadFile(ownershipFilePath(s.root))
	if err != nil {
		if os.IsNotExist(err) {
			return OwnRecord{}, ErrOwnershipRecordNotFound
		}
		return OwnRecord{}, err
	}
	var rec OwnRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return OwnRecord{}, fmt.Errorf("%w: %v", ErrOwnershipRecordCorrupted, err)
	}
	if err := rec.Validate(); err != nil {
		return OwnRecord{}, err
	}
	return rec, nil
}

func marshalRecord(rec OwnRecord) ([]byte, error) {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	return data, nil
}

func ReadWithRetry(store OwnershipStore, maxWait time.Duration) (OwnRecord, error) {
	if maxWait <= 0 {
		maxWait = 500 * time.Millisecond
	}
	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		rec, err := store.Read(context.Background())
		if err == nil {
			return rec, nil
		}
		if err != ErrOwnershipRecordNotFound {
			return OwnRecord{}, err
		}
		lastErr = err
		select {
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			break
		}
	}
	return OwnRecord{}, lastErr
}
