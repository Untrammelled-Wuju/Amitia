package browser

import (
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	DefaultMaxDownloadBytes = int64(512 * 1024 * 1024)
	DefaultDownloadTimeout  = 120 * time.Second
	MaxDownloadTimeout      = 10 * time.Minute

	DownloadBehaviorAllowAndName = "allowAndName"
	DownloadBehaviorAllow        = "allow"
	DownloadBehaviorDeny         = "deny"
)

type DownloadPolicy struct {
	MaxBytes        int64
	Timeout         time.Duration
	Behavior        string
	StagingRootPath string
}

func DefaultDownloadPolicy() DownloadPolicy {
	return DownloadPolicy{
		MaxBytes: DefaultMaxDownloadBytes,
		Timeout:  DefaultDownloadTimeout,
		Behavior: DownloadBehaviorAllowAndName,
	}
}

func SanitizeFilename(name string) string {
	if name == "" {
		return "download"
	}

	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")

	var builder strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) || r == 0 {
			continue
		}
		builder.WriteRune(r)
	}

	result := builder.String()
	result = strings.TrimSpace(result)

	if result == "" {
		return "download"
	}

	if len(result) > 255 {
		result = result[:255]
	}

	return result
}

func IsAllowedScheme(scheme string, allowed []string) bool {
	for _, s := range allowed {
		if strings.EqualFold(s, scheme) {
			return true
		}
	}
	return false
}
