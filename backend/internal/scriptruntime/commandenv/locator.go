// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

import (
	"os/exec"
	"path/filepath"
)

type ExecutableLocator interface {
	LookPath(string) (string, error)
}

type defaultLocator struct{}

func (defaultLocator) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func newDefaultLocator() ExecutableLocator {
	return defaultLocator{}
}

func toAbsolutePath(locator ExecutableLocator, raw string) (string, error) {
	trimmed := filepath.Clean(raw)
	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}
	resolved, err := locator.LookPath(trimmed)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return resolved, nil
	}
	return abs, nil
}
