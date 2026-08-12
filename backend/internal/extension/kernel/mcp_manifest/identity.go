package mcp_manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var serverIDSafePattern = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func BuildServerID(extensionID, moduleID, contributionID string) string {
	raw := extensionID + "\x00" + moduleID + "\x00" + contributionID
	hash := sha256.Sum256([]byte(raw))
	hashPrefix := hex.EncodeToString(hash[:])[:12]
	slug := slugify(extensionID) + "_" + slugify(moduleID) + "_" + slugify(contributionID)
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return "mcp_" + slug + "_" + hashPrefix
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = serverIDSafePattern.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return s
}

func IsServerIDPathSafe(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r >= 127 {
			return false
		}
		switch r {
		case '/', '?', '#', '%':
			return false
		}
		if r < 32 {
			return false
		}
	}
	return true
}
