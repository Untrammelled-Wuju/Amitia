package secret

import (
	"context"
	"encoding/json"
	"time"
)

type PreparedSecrets struct {
	LeaseIDs     []LeaseID
	RuntimeInput json.RawMessage
	Environment  []string
	cleanup      func()
}

type Injector interface {
	Prepare(ctx context.Context, tool interface{}, invocation interface{}, input json.RawMessage) (PreparedSecrets, error)
}

type SecretBinding struct {
	RefPath        SecretRef
	EnvironmentKey string
}

type ToolSecretBindings struct {
	Bindings []SecretBinding
}

func DefaultSecretBindings(leaseID LeaseID, ref SecretRef) []SecretBinding {
	return []SecretBinding{
		{RefPath: ref, EnvironmentKey: "AMITIA_SECRET_LEASE"},
	}
}

func BuildLeaseEnvironment(leaseID LeaseID, existing []string) []string {
	env := make([]string, 0, len(existing)+1)
	for _, e := range existing {
		if len(e) > len("AMITIA_SECRET_LEASE=") && e[:len("AMITIA_SECRET_LEASE=")] == "AMITIA_SECRET_LEASE=" {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "AMITIA_SECRET_LEASE="+string(leaseID))
	return env
}

func ComputeLeaseTTL(invocationDeadline *time.Time) time.Duration {
	defaultTTL := 5 * time.Minute
	if invocationDeadline == nil {
		return defaultTTL
	}
	remaining := time.Until(*invocationDeadline)
	if remaining <= 0 {
		return time.Second * 30
	}
	if remaining < defaultTTL {
		return remaining
	}
	return defaultTTL
}

func IsSensitiveEnvKey(key string) bool {
	upper := toUpper(key)
	sensitiveTokens := []string{
		"TOKEN", "SECRET", "PASSWORD", "PASSWD", "API_KEY", "APIKEY",
		"AUTHORIZATION", "COOKIE", "PRIVATE_KEY", "CREDENTIAL",
	}
	for _, token := range sensitiveTokens {
		if containsToken(upper, token) {
			return true
		}
	}
	return false
}

func IsProtectedSystemEnv(key string) bool {
	protected := []string{
		"PATH", "HOME", "TMP", "TEMP", "SystemRoot", "NODE_OPTIONS",
		"LD_PRELOAD", "DYLD_INSERT_LIBRARIES", "USERPROFILE", "PATHEXT",
	}
	for _, p := range protected {
		if upperEqual(key, p) {
			return true
		}
	}
	return false
}

func ValidateEnvEntry(key string) error {
	if IsSensitiveEnvKey(key) {
		return ErrSecretRefInvalid
	}
	if IsProtectedSystemEnv(key) {
		return ErrSecretRefInvalid
	}
	return nil
}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsToken(s, token string) bool {
	return indexOf(s, token) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func upperEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]
		if ca >= 'a' && ca <= 'z' {
			ca = ca - 'a' + 'A'
		}
		if cb >= 'a' && cb <= 'z' {
			cb = cb - 'a' + 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
