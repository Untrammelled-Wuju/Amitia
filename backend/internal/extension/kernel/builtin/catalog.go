package builtin

import (
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type Catalog struct {
	mu          sync.RWMutex
	definitions map[domain.ExtensionID]Definition
}

func NewCatalog() *Catalog {
	return &Catalog{
		definitions: make(map[domain.ExtensionID]Definition),
	}
}

func (c *Catalog) Register(def Definition) error {
	if def.Extension.ID == "" {
		return fmt.Errorf("builtin catalog: extension ID is required")
	}
	if string(def.Extension.ID)[:len(PrefixBuiltin)] != PrefixBuiltin {
		return fmt.Errorf("builtin catalog: extension ID must use %q prefix, got %q", PrefixBuiltin, def.Extension.ID)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.definitions[def.Extension.ID] = def
	return nil
}

func (c *Catalog) Get(id domain.ExtensionID) (Definition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	def, ok := c.definitions[id]
	return def, ok
}

func (c *Catalog) List() []Definition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Definition, 0, len(c.definitions))
	for _, def := range c.definitions {
		out = append(out, def)
	}
	return out
}

func (c *Catalog) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.definitions)
}

func (c *Catalog) Unregister(id domain.ExtensionID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.definitions, id)
}
