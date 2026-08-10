package secret

import (
	"regexp"
	"strings"
	"sync"
)

const minSecretValueLength = 6

var (
	secretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
		regexp.MustCompile(`ghp_[a-zA-Z0-9]{20,}`),
		regexp.MustCompile(`xox[bpras]-[a-zA-Z0-9-]+`),
		regexp.MustCompile(`Bearer\s+[a-zA-Z0-9._-]{20,}`),
		regexp.MustCompile(`[A-Za-z0-9+/=]{32,}`),
	}
)

type Redactor struct {
	mu      sync.Mutex
	secrets map[string]struct{}
}

func NewRedactor() *Redactor {
	return &Redactor{
		secrets: make(map[string]struct{}),
	}
}

func (r *Redactor) Add(value []byte) {
	if len(value) < minSecretValueLength {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets[string(value)] = struct{}{}
}

func (r *Redactor) Remove(value []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.secrets, string(value))
}

func (r *Redactor) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets = make(map[string]struct{})
}

func (r *Redactor) Redact(text string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for secret := range r.secrets {
		text = strings.ReplaceAll(text, secret, "[redacted]")
	}
	return text
}

func RedactLogLine(text string) string {
	for _, pattern := range secretPatterns {
		text = pattern.ReplaceAllString(text, "[redacted]")
	}
	return text
}

type SecretRedactionProvider interface {
	Redact(text string) string
}

var _ SecretRedactionProvider = (*Redactor)(nil)
