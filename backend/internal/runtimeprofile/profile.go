// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimeprofile

import "strings"

type Profile string

const (
	ProfileLocal       Profile = "local"
	ProfileCloudCore   Profile = "cloud-core"
	ProfileDeviceAgent Profile = "device-agent"
)

func (p Profile) String() string {
	return string(p)
}

func (p Profile) IsValid() bool {
	switch p {
	case ProfileLocal, ProfileCloudCore, ProfileDeviceAgent:
		return true
	default:
		return false
	}
}

func (p Profile) IsCore() bool {
	switch p {
	case ProfileLocal, ProfileCloudCore:
		return true
	default:
		return false
	}
}

func (p Profile) IsDeviceAgent() bool {
	return p == ProfileDeviceAgent
}

func Default() Profile {
	return ProfileLocal
}

// Parse parses an explicitly supplied runtime profile. It is intentionally
// strict: an empty string is not a profile. Callers that want the default must
// decide that the setting is absent first and call Default themselves.
func Parse(raw string) (Profile, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "local", "desktop-local":
		return ProfileLocal, nil
	case "cloud-core", "cloud", "remote-core", "cloud-web":
		return ProfileCloudCore, nil
	case "device-agent", "device":
		return ProfileDeviceAgent, nil
	default:
		return "", &InvalidProfileError{Raw: raw}
	}
}
