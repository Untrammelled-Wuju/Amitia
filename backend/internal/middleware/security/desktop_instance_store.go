// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
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

	dir := filepath.Dir(instanceFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf(
			"create desktop instance dir: %w",
			err,
		)
	}

	instanceID := strings.TrimSpace(
		readInstanceFile(instanceFile),
	)
	if instanceID == "" {
		generated, err := newInstanceID()
		if err != nil {
			return nil, fmt.Errorf(
				"generate desktop instance id: %w",
				err,
			)
		}
		if err := writeInstanceFile(
			instanceFile,
			generated,
		); err != nil {
			return nil, fmt.Errorf(
				"write desktop instance id: %w",
				err,
			)
		}
		instanceID = generated
	}

	return &DesktopInstanceStore{
		instanceFile: instanceFile,
		instanceID:   instanceID,
	}, nil
}

func readInstanceFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func newInstanceID() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(
		b,
	), nil
}

func writeInstanceFile(path string, value string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(
		tmp,
		[]byte(value),
		0o600,
	); err != nil {
		return err
	}
	if file, err := os.OpenFile(
		tmp,
		os.O_WRONLY,
		0o600,
	); err == nil {
		_ = file.Sync()
		_ = file.Close()
	}
	return os.Rename(tmp, path)
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
