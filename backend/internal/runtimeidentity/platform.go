package runtimeidentity

import "strings"

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

func ParsePlatform(raw string) Platform {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "macos", "mac":
		return PlatformDarwin
	case "win":
		return PlatformWindows
	default:
		return Platform(s)
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
