// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ProcessIdentity struct {
	PID            int
	ExecutablePath string
	StartMarker    string
	CommandLine    []string
}

func (i ProcessIdentity) Validate() error {
	if i.PID <= 0 {
		return fmt.Errorf("qdrantprocess: invalid PID: %d", i.PID)
	}
	if i.ExecutablePath == "" {
		return fmt.Errorf("qdrantprocess: empty executable path")
	}
	if !filepath.IsAbs(i.ExecutablePath) {
		return fmt.Errorf("qdrantprocess: executable path is not absolute: %s", i.ExecutablePath)
	}
	if i.StartMarker == "" {
		return fmt.Errorf("qdrantprocess: empty start marker")
	}
	for idx, arg := range i.CommandLine {
		if strings.ContainsRune(arg, 0) {
			return fmt.Errorf("qdrantprocess: command-line arg %d contains NUL", idx)
		}
	}
	return nil
}

func (i ProcessIdentity) cloneCommandLine() []string {
	if i.CommandLine == nil {
		return nil
	}
	out := make([]string, len(i.CommandLine))
	copy(out, i.CommandLine)
	return out
}

func SameProcessIdentity(expected, actual ProcessIdentity) bool {
	if expected.PID != actual.PID {
		return false
	}
	if expected.StartMarker != actual.StartMarker {
		return false
	}
	if !SameExecutablePath(expected.ExecutablePath, actual.ExecutablePath) {
		return false
	}
	if len(expected.CommandLine) > 0 && len(actual.CommandLine) > 0 {
		if !matchCommandLine(expected.CommandLine, actual.CommandLine) {
			return false
		}
	}
	return true
}

func matchCommandLine(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	for idx := range expected {
		if expected[idx] != actual[idx] {
			return false
		}
	}
	return true
}

func ContainsConfigPath(cl []string, configPath string) bool {
	for idx := 0; idx < len(cl); idx++ {
		if cl[idx] == "--config-path" && idx+1 < len(cl) && cl[idx+1] == configPath {
			return true
		}
	}
	return false
}
