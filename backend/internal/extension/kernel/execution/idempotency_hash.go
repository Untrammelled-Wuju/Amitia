package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func BuildIdempotencyKeySHA(identity IdempotencyIdentity) string {
	var b strings.Builder
	b.WriteString("v1")
	b.WriteByte('|')
	b.WriteString(identity.ToolID)
	b.WriteByte('|')
	b.WriteString(fmt.Sprintf("%d", identity.Generation))
	b.WriteByte('|')
	b.WriteString(identity.UserID)
	b.WriteByte('|')
	b.WriteString(identity.CharacterID)
	b.WriteByte('|')
	b.WriteString(identity.ConversationID)
	b.WriteByte('|')
	b.WriteString(string(identity.Source))
	b.WriteByte('|')
	b.WriteString(identity.CallerKey)
	return sha256Hex([]byte(b.String()))
}

func BuildIdempotencyIdentity(toolID string, inv capability.ToolInvocationContext, callerKey string) IdempotencyIdentity {
	return IdempotencyIdentity{
		ToolID:         toolID,
		Generation:     inv.Generation,
		UserID:         inv.UserID,
		CharacterID:    inv.CharacterID,
		ConversationID: inv.ConversationID,
		Source:         inv.Source,
		CallerKey:      callerKey,
	}
}

func BuildRequestFingerprintSHA(input json.RawMessage, toolVersion capability.ToolVersion, generation int64) string {
	canonical, err := CanonicalInputHash(input)
	if err != nil {
		canonical = sha256Hex(input)
	}
	var b strings.Builder
	b.WriteString(canonical)
	b.WriteByte('|')
	b.WriteString(fmt.Sprintf("%d", toolVersion.SchemaVersion))
	b.WriteByte('|')
	b.WriteString(toolVersion.Revision)
	b.WriteByte('|')
	b.WriteString(fmt.Sprintf("%d", generation))
	return sha256Hex([]byte(b.String()))
}

func CanonicalInputHash(input json.RawMessage) (string, error) {
	if len(input) == 0 {
		return "", fmt.Errorf("idempotency: empty input")
	}
	raw, err := canonicalize(input)
	if err != nil {
		return "", err
	}
	return sha256Hex(raw), nil
}

func canonicalize(input json.RawMessage) ([]byte, error) {
	var generic any
	dec := json.NewDecoder(jsonReader(input))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("idempotency: decode input: %w", err)
	}
	if dec.More() {
		_, err := io.Copy(io.Discard, dec.Buffered())
		_ = err
	}
	sorted := sortValue(generic)
	out, err := json.Marshal(sorted)
	if err != nil {
		return nil, fmt.Errorf("idempotency: marshal canonical: %w", err)
	}
	return out, nil
}

func sortValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]byte, 0, 64)
		out = append(out, '{')
		for i, k := range keys {
			if i > 0 {
				out = append(out, ',')
			}
			out = append(out, json.RawMessage([]byte(fmt.Sprintf("%q:", k)))...)
			out = append(out, json.RawMessage(mustJSON(sortValue(val[k])))...)
		}
		out = append(out, '}')
		return json.RawMessage(out)
	case []any:
		for i, item := range val {
			val[i] = sortValue(item)
		}
		return val
	default:
		return v
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}

func jsonReader(b []byte) *strings.Reader {
	return strings.NewReader(string(b))
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
