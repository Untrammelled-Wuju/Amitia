package dataportability

import (
	"sync"
)

type ImportIdentityMap struct {
	mu            sync.RWMutex
	Characters    map[string]string `json:"characters"`
	Conversations map[string]string `json:"conversations"`
	Messages      map[string]string `json:"messages"`
	Memories      map[string]string `json:"memories"`
	Resources     map[string]string `json:"resources"`
	Configs       map[string]string `json:"configs"`
}

func NewImportIdentityMap() *ImportIdentityMap {
	return &ImportIdentityMap{
		Characters:    make(map[string]string),
		Conversations: make(map[string]string),
		Messages:      make(map[string]string),
		Memories:      make(map[string]string),
		Resources:     make(map[string]string),
		Configs:       make(map[string]string),
	}
}

func (m *ImportIdentityMap) AddCharacter(src, dst string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Characters[src] = dst
}

func (m *ImportIdentityMap) AddConversation(src, dst string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Conversations[src] = dst
}

func (m *ImportIdentityMap) AddMessage(src, dst string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages[src] = dst
}

func (m *ImportIdentityMap) AddMemory(src, dst string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Memories[src] = dst
}

func (m *ImportIdentityMap) AddResource(src, dst string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Resources[src] = dst
}

func (m *ImportIdentityMap) AddConfig(src, dst string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Configs[src] = dst
}

func (m *ImportIdentityMap) GetCharacter(src string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Characters[src]
	return v, ok
}

func (m *ImportIdentityMap) GetConversation(src string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Conversations[src]
	return v, ok
}

func (m *ImportIdentityMap) GetMessage(src string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Messages[src]
	return v, ok
}

func (m *ImportIdentityMap) GetMemory(src string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Memories[src]
	return v, ok
}

func (m *ImportIdentityMap) GetResource(src string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Resources[src]
	return v, ok
}

func (m *ImportIdentityMap) GetConfig(src string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.Configs[src]
	return v, ok
}

func (m *ImportIdentityMap) RemapMessageRef(src string) string {
	if src == "" {
		return ""
	}
	if dst, ok := m.GetMessage(src); ok {
		return dst
	}
	return src
}

func (m *ImportIdentityMap) RemapConversationRef(src string) string {
	if src == "" {
		return ""
	}
	if dst, ok := m.GetConversation(src); ok {
		return dst
	}
	return src
}

func (m *ImportIdentityMap) RemapCharacterRef(src string) string {
	if src == "" {
		return ""
	}
	if dst, ok := m.GetCharacter(src); ok {
		return dst
	}
	return src
}
