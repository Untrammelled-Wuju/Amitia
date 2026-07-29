package agent_skill

import (
	"fmt"
	"sync"
)

type CatalogFilter struct {
	Scope   AgentSkillScope
	Enabled *bool
	Query   string
}

type AgentSkillCatalog struct {
	mu    sync.RWMutex
	items map[string]AgentSkillDefinition
}

func NewAgentSkillCatalog() *AgentSkillCatalog {
	return &AgentSkillCatalog{
		items: map[string]AgentSkillDefinition{},
	}
}

func (c *AgentSkillCatalog) Register(definition AgentSkillDefinition) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.items[definition.ExtensionID]; exists {
		return fmt.Errorf("agent skill %s already registered", definition.ExtensionID)
	}
	c.items[definition.ExtensionID] = definition
	return nil
}

func (c *AgentSkillCatalog) Unregister(extensionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, extensionID)
	return nil
}

func (c *AgentSkillCatalog) Get(extensionID string) (AgentSkillDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[extensionID]
	return item, ok
}

func (c *AgentSkillCatalog) List(filter CatalogFilter) []AgentSkillDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]AgentSkillDefinition, 0, len(c.items))
	for _, item := range c.items {
		if filter.Scope != "" && item.Scope != filter.Scope {
			continue
		}
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		if filter.Query != "" && !containsQuery(item.Name, item.Description, filter.Query) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func (c *AgentSkillCatalog) SetEnabled(extensionID string, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[extensionID]
	if !ok {
		return nil
	}
	item.Enabled = enabled
	c.items[extensionID] = item
	return nil
}

func (c *AgentSkillCatalog) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func containsQuery(name, description, query string) bool {
	return contains(name, query) || contains(description, query)
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && matchSubstring(s, substr)
}

func matchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
