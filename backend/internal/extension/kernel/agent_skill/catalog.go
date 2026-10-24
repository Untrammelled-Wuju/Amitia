package agent_skill

import (
	"fmt"
	"strings"
	"sync"
)

type CatalogFilter struct {
	Enabled        *bool
	Query          string
	SpaceID        string
	CharacterID    string
	ConversationID string
	ExtensionID    string
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
	key := catalogKey(definition)
	if _, exists := c.items[key]; exists {
		return fmt.Errorf("agent skill %s already registered", definition.StateID())
	}
	c.items[key] = definition
	return nil
}

func (c *AgentSkillCatalog) Unregister(extensionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, item := range c.items {
		if item.ExtensionID == extensionID {
			delete(c.items, key)
		}
	}
	return nil
}

func (c *AgentSkillCatalog) Get(extensionID string) (AgentSkillDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, item := range c.items {
		if item.OwnerSpaceID == "" && (item.ExtensionID == extensionID || item.ID == extensionID || item.ContributionID == extensionID) {
			return item, true
		}
	}
	return AgentSkillDefinition{}, false
}

func (c *AgentSkillCatalog) GetScoped(spaceID, skillID string) (AgentSkillDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var global AgentSkillDefinition
	foundGlobal := false
	for _, item := range c.items {
		if !skillMatchesSpace(item, spaceID) {
			continue
		}
		if item.ExtensionID != skillID && item.ID != skillID && item.ContributionID != skillID {
			continue
		}
		if item.OwnerSpaceID == spaceID && spaceID != "" {
			return item, true
		}
		if item.OwnerSpaceID == "" {
			global = item
			foundGlobal = true
		}
	}
	return global, foundGlobal
}

func (c *AgentSkillCatalog) GetScopedState(spaceID, stateID string) (AgentSkillDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, item := range c.items {
		if item.StateID() == stateID && skillMatchesSpace(item, spaceID) {
			return item, true
		}
	}
	return AgentSkillDefinition{}, false
}

func (c *AgentSkillCatalog) List(filter CatalogFilter) []AgentSkillDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]AgentSkillDefinition, 0, len(c.items))
	for _, item := range c.items {
		if !skillMatchesSpace(item, filter.SpaceID) {
			continue
		}
		if filter.ExtensionID != "" && item.ExtensionID != filter.ExtensionID {
			continue
		}
		if !item.MatchesScope(filter.SpaceID, filter.CharacterID, filter.ConversationID) {
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
	for key, item := range c.items {
		if item.OwnerSpaceID != "" || item.ExtensionID != extensionID && item.ID != extensionID && item.ContributionID != extensionID {
			continue
		}
		item.Enabled = enabled
		c.items[key] = item
	}
	return nil
}

func (c *AgentSkillCatalog) SetEnabledForSpace(spaceID, extensionID, skillID string, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, item := range c.items {
		if item.OwnerSpaceID != spaceID {
			continue
		}
		if item.ExtensionID != extensionID {
			continue
		}
		if item.ID != skillID && item.ContributionID != skillID {
			continue
		}
		item.Enabled = enabled
		c.items[key] = item
	}
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

func catalogKey(definition AgentSkillDefinition) string {
	return strings.TrimSpace(definition.OwnerSpaceID) + "\x00" + strings.TrimSpace(definition.StateID())
}

func skillMatchesSpace(item AgentSkillDefinition, spaceID string) bool {
	return item.OwnerSpaceID == "" || spaceID != "" && item.OwnerSpaceID == spaceID
}
