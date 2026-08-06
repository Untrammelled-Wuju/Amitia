// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package buildinfo

import "runtime"

var (
	Version = "dev"
	Commit  = "unknown"
	Target  = "development"
)

type Info struct {
	Name      string
	Version   string
	Commit    string
	Target    string
	GoVersion string
	GOOS      string
	GOARCH    string
}

func Current() Info {
	return Info{
		Name:      "amitia-server",
		Version:   Version,
		Commit:    Commit,
		Target:    Target,
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
	}
}
