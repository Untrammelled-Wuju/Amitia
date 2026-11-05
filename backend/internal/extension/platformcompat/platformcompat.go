package platformcompat

import (
	"fmt"
	"strings"
)

const (
	Windows = "windows"
	MacOS   = "macos"
	Linux   = "linux"
	Android = "android"
	IOS     = "ios"
	Web     = "web"
	All     = "all"
)

var canonicalOrder = []string{Windows, MacOS, Linux, Android, IOS, Web}

func Normalize(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == "all" || normalized == "*":
		return All
	case normalized == "windows" || normalized == "win" || normalized == "win32" || normalized == "win64":
		return Windows
	case normalized == "macos" || normalized == "mac" || normalized == "osx" || normalized == "darwin" ||
		strings.HasPrefix(normalized, "darwin-"):
		return MacOS
	case normalized == "linux" || strings.HasPrefix(normalized, "linux-"):
		return Linux
	case normalized == "android" || strings.HasPrefix(normalized, "android-"):
		return Android
	case normalized == "ios" || strings.HasPrefix(normalized, "ios-"):
		return IOS
	case normalized == "web":
		return Web
	default:
		return ""
	}
}

func NormalizeList(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := Normalize(value)
		if normalized == "" {
			return nil, fmt.Errorf("unsupported plugin platform: %s", value)
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	if seen[All] {
		return []string{All}, nil
	}
	ordered := make([]string, 0, len(result))
	for _, candidate := range canonicalOrder {
		if seen[candidate] {
			ordered = append(ordered, candidate)
		}
	}
	return ordered, nil
}

func Supports(values []string, current string) bool {
	if len(values) == 0 {
		return true
	}
	target := Normalize(current)
	if target == "" {
		return false
	}
	for _, value := range values {
		normalized := Normalize(value)
		if normalized == All || normalized == target {
			return true
		}
	}
	return false
}

func Summary(values []string) string {
	if len(values) == 0 {
		return "支持全部平台"
	}
	normalized, err := NormalizeList(values)
	if err != nil {
		return err.Error()
	}
	if len(normalized) == 1 && normalized[0] == All {
		return "支持全部平台"
	}
	labels := make([]string, 0, len(normalized))
	for _, value := range normalized {
		switch value {
		case Windows:
			labels = append(labels, "Windows")
		case MacOS:
			labels = append(labels, "macOS")
		case Linux:
			labels = append(labels, "Linux")
		case Android:
			labels = append(labels, "Android")
		case IOS:
			labels = append(labels, "iOS")
		case Web:
			labels = append(labels, "Web")
		}
	}
	return "支持 " + strings.Join(labels, "、")
}
