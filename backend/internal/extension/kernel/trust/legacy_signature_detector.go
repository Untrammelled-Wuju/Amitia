package trust

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type LegacySignatureFormat string

const (
	LegacyFormatNone          LegacySignatureFormat = "none"
	LegacyFormatPublisherTree LegacySignatureFormat = "publisher_tree_hash"
	LegacyFormatContentTree   LegacySignatureFormat = "content_tree_manifest_hash"
)

type LegacySignatureInfo struct {
	Format      LegacySignatureFormat
	PublisherID string
	KeyID       string
	SignedAt    time.Time
	HasV2       bool
}

type LegacySignatureDetector struct{}

func NewLegacySignatureDetector() *LegacySignatureDetector {
	return &LegacySignatureDetector{}
}

func (d *LegacySignatureDetector) Detect(v2Signature json.RawMessage, legacySignature interface{}) LegacySignatureInfo {
	info := LegacySignatureInfo{Format: LegacyFormatNone}

	if len(v2Signature) > 0 {
		info.HasV2 = true
	}

	if legacySignature == nil {
		return info
	}

	switch sig := legacySignature.(type) {
	case map[string]interface{}:
		info.Format = detectLegacyFormatFromMap(sig)
		info.PublisherID = getStringFromMap(sig, "publisherId")
		info.KeyID = getStringFromMap(sig, "keyId")
		info.SignedAt = getTimeFromMap(sig, "signedAt")
	case []byte:
		var m map[string]interface{}
		if err := json.Unmarshal(sig, &m); err == nil {
			info.Format = detectLegacyFormatFromMap(m)
			info.PublisherID = getStringFromMap(m, "publisherId")
			info.KeyID = getStringFromMap(m, "keyId")
			info.SignedAt = getTimeFromMap(m, "signedAt")
		}
	}

	return info
}

func (d *LegacySignatureDetector) ShouldReject(info LegacySignatureInfo, allowDevMode bool) (bool, string) {
	if info.HasV2 {
		return false, ""
	}
	if info.Format == LegacyFormatNone {
		return false, ""
	}
	if allowDevMode {
		return false, "legacy signature allowed in dev mode"
	}
	return true, fmt.Sprintf("legacy signature format %s must be re-signed with amitiax-signature-v1", info.Format)
}

func (d *LegacySignatureDetector) Warning(info LegacySignatureInfo) string {
	if info.Format == LegacyFormatNone {
		return ""
	}
	if info.HasV2 {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf(
		"legacy signature detected (format=%s, publisher=%s, key=%s); please re-sign with amitiax sign --v1",
		info.Format, info.PublisherID, info.KeyID,
	))
}

func detectLegacyFormatFromMap(m map[string]interface{}) LegacySignatureFormat {
	if _, ok := m["signature"]; ok {
		if _, ok := m["contentTreeHash"]; ok {
			return LegacyFormatContentTree
		}
		return LegacyFormatPublisherTree
	}
	return LegacyFormatNone
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getTimeFromMap(m map[string]interface{}, key string) time.Time {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case string:
			parsed, err := time.Parse(time.RFC3339, t)
			if err == nil {
				return parsed
			}
		case time.Time:
			return t
		}
	}
	return time.Time{}
}
