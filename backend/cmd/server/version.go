// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/u-ai/backend/internal/buildinfo"
)

type versionOutput struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Target    string `json:"target"`
	GoVersion string `json:"goVersion"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
}

func handleVersion() int {
	info := buildinfo.Current()
	out := versionOutput{
		Name:      info.Name,
		Version:   info.Version,
		Commit:    info.Commit,
		Target:    info.Target,
		GoVersion: info.GoVersion,
		GOOS:      info.GOOS,
		GOARCH:    info.GOARCH,
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "version encode error: %v\n", err)
		return 1
	}
	return 0
}
