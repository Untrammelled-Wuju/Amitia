package rpc

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

var reservedMetadataPrefixes = []string{
	"rpc.",
}

func FingerprintMetadata(metadata map[string]json.RawMessage) string {
	if len(metadata) == 0 {
		return ""
	}

	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		if isReservedKey(k) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(strings.ToLower(k))
		buf.WriteByte('=')
		buf.Write(metadata[k])
		buf.WriteByte(';')
	}
	return buf.String()
}

func isReservedKey(key string) bool {
	lower := strings.ToLower(key)
	for _, prefix := range reservedMetadataPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
