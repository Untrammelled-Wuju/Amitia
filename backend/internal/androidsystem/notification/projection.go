package notification

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
)

type refMapping struct {
	key       string
	generation uint64
}

type ProjectionStore struct {
	mu          sync.RWMutex
	generation  uint64
	refToKey    map[string]refMapping
	keyToRef    map[string]string
	ownRefToTag map[string]string
	tagToOwnRef map[string]string
}

func NewProjectionStore() *ProjectionStore {
	return &ProjectionStore{
		refToKey:    make(map[string]refMapping),
		keyToRef:    make(map[string]string),
		ownRefToTag: make(map[string]string),
		tagToOwnRef: make(map[string]string),
	}
}

func (s *ProjectionStore) Generation() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generation
}

func (s *ProjectionStore) BumpGeneration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation++
	s.refToKey = make(map[string]refMapping)
	s.keyToRef = make(map[string]string)
}

func (s *ProjectionStore) AssignNotificationRef(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ref, ok := s.keyToRef[key]; ok {
		return ref
	}
	ref := generateOpaqueRef(NotificationRefPrefix)
	s.keyToRef[key] = ref
	s.refToKey[ref] = refMapping{key: key, generation: s.generation}
	return ref
}

func (s *ProjectionStore) LookupNotification(ref string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mapping, ok := s.refToKey[ref]
	if !ok {
		return "", false
	}
	if mapping.generation != s.generation {
		return "", false
	}
	return mapping.key, true
}

func (s *ProjectionStore) InvalidateNotification(ref string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mapping, ok := s.refToKey[ref]; ok {
		delete(s.refToKey, ref)
		delete(s.keyToRef, mapping.key)
	}
}

func (s *ProjectionStore) AssignOwnRef(tag string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ref, ok := s.tagToOwnRef[tag]; ok {
		return ref
	}
	ref := generateOpaqueRef(OwnNotificationPrefix)
	s.tagToOwnRef[tag] = ref
	s.ownRefToTag[ref] = tag
	return ref
}

func (s *ProjectionStore) LookupOwnTag(ref string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tag, ok := s.ownRefToTag[ref]
	return tag, ok
}

func (s *ProjectionStore) InvalidateOwnRef(ref string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tag, ok := s.ownRefToTag[ref]; ok {
		delete(s.ownRefToTag, ref)
		delete(s.tagToOwnRef, tag)
	}
}

func generateOpaqueRef(prefix string) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	encoded := base64.RawURLEncoding.EncodeToString(buf)
	return fmt.Sprintf("%s%s", prefix, encoded)
}

func Truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}

func TruncateString(s string, maxLen int) string {
	return Truncate(s, maxLen)
}

func IsSensitiveExtra(key string) bool {
	upper := strings.ToUpper(key)
	sensitive := []string{
		"PASSWORD", "PASSWD", "TOKEN", "SECRET", "OTP", "CODE",
		"CREDENTIAL", "AUTH", "CVV", "SSN", "ACCOUNT",
	}
	for _, s := range sensitive {
		if strings.Contains(upper, s) {
			return true
		}
	}
	return false
}

func AllowedExtrasKeys() []string {
	return []string{
		"android.title",
		"android.text",
		"android.subText",
		"android.bigText",
	}
}

func IsRuntimeServiceNotification(packageName string, channelID string) bool {
	if packageName == "com.amitia.amitia_app" && channelID == "runtime_service" {
		return true
	}
	if channelID == "runtime_service" {
		return true
	}
	return false
}
