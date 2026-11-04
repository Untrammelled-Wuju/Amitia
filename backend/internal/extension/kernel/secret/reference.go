package secret

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type SecretRef string

const (
	schemeCanonical = "secret://"
)

func ParseRef(raw string) (SecretRef, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrSecretRefInvalid
	}
	if !strings.HasPrefix(trimmed, schemeCanonical) {
		return "", fmt.Errorf("%w: %s", ErrSecretRefInvalid, raw)
	}
	path := strings.TrimPrefix(trimmed, schemeCanonical)
	if path == "" || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return "", fmt.Errorf("%w: %s", ErrSecretRefInvalid, raw)
	}
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("%w: %s", ErrSecretRefInvalid, raw)
	}
	return SecretRef(raw), nil
}

func (r SecretRef) Valid() bool {
	s := string(r)
	if !strings.HasPrefix(s, schemeCanonical) {
		return false
	}
	path := strings.TrimPrefix(s, schemeCanonical)
	if path == "" || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return false
	}
	parts := strings.SplitN(path, "/", 2)
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

func (r SecretRef) String() string {
	return string(r)
}

func (r SecretRef) Canonical() SecretRef {
	return r
}

func FingerprintRef(ref SecretRef) string {
	h := sha256.Sum256([]byte(ref))
	return "secret-ref:" + hex.EncodeToString(h[:8])
}

type Reference struct {
	Ref SecretRef
}

func (r Reference) MarshalJSON() ([]byte, error) {
	return marshalSecretRefJSON(r.Ref)
}

func (r *Reference) UnmarshalJSON(data []byte) error {
	var raw string
	if err := unmarshalSecretRefJSON(data, &raw); err != nil {
		return err
	}
	ref, err := ParseRef(raw)
	if err != nil {
		return err
	}
	r.Ref = ref
	return nil
}

func ParseReferenceValue(v string) (SecretRef, error) {
	if v == "" {
		return "", nil
	}
	return ParseRef(v)
}

func marshalSecretRefJSON(ref SecretRef) ([]byte, error) {
	return []byte(fmt.Sprintf(`{"$secret":%q}`, ref.String())), nil
}

func unmarshalSecretRefJSON(data []byte, raw *string) error {
	var wrapper struct {
		Secret string `json:"$secret"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		var s string
		if err2 := json.Unmarshal(data, &s); err2 == nil {
			*raw = s
			return nil
		}
		return err
	}
	*raw = wrapper.Secret
	return nil
}
