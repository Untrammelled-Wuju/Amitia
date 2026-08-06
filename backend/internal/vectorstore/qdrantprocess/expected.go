// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"fmt"
	"path/filepath"
)

const ComponentID = "builtin.qdrant-process"

type ExpectedProcess struct {
	ComponentID    string
	ExecutablePath string
	ConfigPath     string
}

func NewExpectedProcess(executablePath, configPath string) ExpectedProcess {
	return ExpectedProcess{
		ComponentID:    ComponentID,
		ExecutablePath: executablePath,
		ConfigPath:     configPath,
	}
}

func (e ExpectedProcess) Validate() error {
	if e.ComponentID == "" {
		return fmt.Errorf("qdrantprocess: empty component ID")
	}
	if e.ExecutablePath == "" {
		return fmt.Errorf("qdrantprocess: empty executable path")
	}
	if !filepath.IsAbs(e.ExecutablePath) {
		return fmt.Errorf("qdrantprocess: executable path is not absolute: %s", e.ExecutablePath)
	}
	if e.ConfigPath == "" {
		return fmt.Errorf("qdrantprocess: empty config path")
	}
	if !filepath.IsAbs(e.ConfigPath) {
		return fmt.Errorf("qdrantprocess: config path is not absolute: %s", e.ConfigPath)
	}
	return nil
}
