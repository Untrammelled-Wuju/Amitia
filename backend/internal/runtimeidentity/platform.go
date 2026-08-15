package runtimeidentity

import (
	"fmt"
	"strings"
)

type Platform string

const (
	PlatformUnknown Platform = ""
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
	PlatformWindows Platform = "windows"
	PlatformLinux   Platform = "linux"
	PlatformDarwin  Platform = "darwin"
	PlatformWeb     Platform = "web"
)

func ParsePlatform(raw string) (Platform, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "macos", "mac":
		return PlatformDarwin, nil
	case "win":
		return PlatformWindows, nil
	case "android", "ios", "windows", "linux", "darwin", "web":
		return Platform(s), nil
	case "":
		return PlatformUnknown, nil
	default:
		return PlatformUnknown, fmt.Errorf("runtimeidentity: unknown platform: %q", raw)
	}
}

func (p Platform) String() string {
	return string(p)
}

func (p Platform) IsKnown() bool {
	switch p {
	case PlatformUnknown, PlatformAndroid, PlatformIOS, PlatformWindows, PlatformLinux, PlatformDarwin, PlatformWeb:
		return true
	}
	return false
}
