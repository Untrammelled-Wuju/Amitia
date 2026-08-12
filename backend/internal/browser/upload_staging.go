package browser

import (
	"path/filepath"
	"strings"
)

const (
	DefaultMaxUploadBytes = int64(256 * 1024 * 1024)

	UploadStagingDir = "browser-uploads"
)

type UploadPolicy struct {
	MaxBytes        int64
	StagingRootPath string
}

func DefaultUploadPolicy() UploadPolicy {
	return UploadPolicy{
		MaxBytes: DefaultMaxUploadBytes,
	}
}

func SanitizeUploadFilename(name string) string {
	if name == "" {
		return "upload"
	}

	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")

	if name == "" {
		return "upload"
	}

	if len(name) > 255 {
		name = name[:255]
	}

	return name
}

func IsInputTypeFile(localName string, attributes map[string]string) bool {
	if !strings.EqualFold(localName, "input") {
		return false
	}
	typeAttr, ok := attributes["type"]
	if !ok {
		return false
	}
	return strings.EqualFold(typeAttr, "file")
}

func IsAcceptableFileType(accept string, mimeType string) bool {
	if accept == "" {
		return true
	}

	accept = strings.ToLower(accept)
	mimeType = strings.ToLower(mimeType)

	if strings.Contains(accept, "*/*") {
		return true
	}

	parts := strings.Split(accept, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.HasSuffix(part, "/*") {
			category := strings.TrimSuffix(part, "/*")
			if strings.HasPrefix(mimeType, category+"/") {
				return true
			}
			continue
		}

		if part == mimeType {
			return true
		}

		if strings.HasPrefix(mimeType, part) {
			return true
		}
	}

	return false
}
