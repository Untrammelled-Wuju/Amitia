package packageformat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

func CanonicalJSON(v interface{}) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %w", err)
	}

	var generic interface{}
	dec := json.NewDecoder(jsonReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	canonical := canonicalizeValue(generic)
	return json.Marshal(canonical)
}

func canonicalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		ordered := make([]struct {
			Key   string      `json:"k"`
			Value interface{} `json:"v"`
		}, len(keys))
		for i, k := range keys {
			ordered[i].Key = k
			ordered[i].Value = canonicalizeValue(val[k])
		}
		return ordered

	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = canonicalizeValue(item)
		}
		return result

	default:
		return v
	}
}

func ManifestHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

type bytesReader struct {
	data []byte
	pos  int
}

func jsonReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
