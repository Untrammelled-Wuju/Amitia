package generation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

func generateUUID() string {
	return uuid.New().String()
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func computeSHA256Hex(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func computeHashJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return computeSHA256Hex(string(data))
}

func canonicalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}
