package packageext

import (
	"path/filepath"
	"strings"
)

const (
	Canonical = ".amitiax"
	Game      = ".gamex"
	Pet       = ".petx"
)

var archiveExtensions = map[string]bool{
	Canonical: true,
	Game:      true,
	Pet:       true,
}

func Normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(filepath.Ext(value)))
}

func IsArchivePath(value string) bool {
	return archiveExtensions[Normalize(value)]
}

func SupportedExtensions() []string {
	return []string{Canonical, Game, Pet}
}
