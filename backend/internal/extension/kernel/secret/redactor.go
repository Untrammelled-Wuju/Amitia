package secret

import (
	"strings"
	"sync"
)

const minSecretValueLength = 6

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

type SecretRedactionProvider interface {
	Redact(text string) string
}

var _ SecretRedactionProvider = (*Redactor)(nil)
