// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type DesktopInstanceStore struct {
	mu           sync.RWMutex
	instanceFile string
	instanceID   string
}

func NewDesktopInstanceStore(
	dataDir string,
) (*DesktopInstanceStore, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New(
			"desktop instance dataDir is required",
		)
	}

	instanceFile := filepath.Join(
		dataDir,
		"security",
		"desktop-instance-id",
	)

	data, err := os.ReadFile(instanceFile)
	if err != nil {
		return nil, fmt.Errorf(
			"read desktop instance id: %w",
			err,
		)
	}

	instanceID := strings.TrimSpace(
		string(data),
	)
	if instanceID == "" {
		return nil, errors.New(
			"desktop instance id is empty",
		)
	}

	return &DesktopInstanceStore{
		instanceFile: instanceFile,
		instanceID:   instanceID,
	}, nil
}

func (
	s *DesktopInstanceStore,
) Validate(
	candidate string,
) bool {
	candidate = strings.TrimSpace(candidate)

	s.mu.RLock()
	expected := s.instanceID
	s.mu.RUnlock()

	if candidate == "" ||
		len(candidate) != len(expected) {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(candidate),
		[]byte(expected),
	) == 1
}

func (
	s *DesktopInstanceStore,
) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instanceID
}
