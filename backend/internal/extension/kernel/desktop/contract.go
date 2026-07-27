package desktop

import (
	"context"
	"fmt"
	"sync"
)

type DesktopType string

const (
	DesktopTypeMenuItem       DesktopType = "app.menu.item"
	DesktopTypeMenuSubmenu    DesktopType = "app.menu.submenu"
	DesktopTypeTrayItem      DesktopType = "app.tray.item"
	DesktopTypeTraySubmenu    DesktopType = "app.tray.submenu"
	DesktopTypeAppShortcut    DesktopType = "app.shortcut.application"
	DesktopTypeGlobalShortcut DesktopType = "app.shortcut.global"
)

type ContractStatus string

const (
	ContractStatusActive ContractStatus = "active"
	ContractStatusFrozen ContractStatus = "frozen"
	ContractStatusDeprecated ContractStatus = "deprecated"
)

type DesktopContractDefinition struct {
	ContractID      string         `json:"contractId"`
	Version         int            `json:"version"`
	DesktopType     DesktopType    `json:"desktopType"`
	AllowedTargets  []string       `json:"allowedTargets"`
	Status          ContractStatus `json:"status"`
	Description     string         `json:"description"`
	MaxItemsPerExt  int            `json:"maxItemsPerExt"`
	RequiresPermission bool       `json:"requiresPermission"`
}

type DesktopContractRegistry struct {
	mu         sync.RWMutex
	contracts  map[string]map[int]DesktopContractDefinition
}

func NewDesktopContractRegistry() *DesktopContractRegistry {
	r := &DesktopContractRegistry{
		contracts: make(map[string]map[int]DesktopContractDefinition),
	}
	r.registerBuiltinContracts()
	return r
}

func (r *DesktopContractRegistry) registerBuiltinContracts() {
	builtins := []DesktopContractDefinition{
		{
			ContractID: "app.menu.item", Version: 1,
			DesktopType: DesktopTypeMenuItem,
			AllowedTargets: []string{
				"app.menu.file.extensions", "app.menu.edit.extensions",
				"app.menu.view.extensions", "app.menu.tools.extensions",
				"app.menu.help.extensions",
				"context.chat.message.extensions", "context.chat.composer.extensions",
				"context.extension.detail.extensions",
			},
			Status: ContractStatusActive, MaxItemsPerExt: 20, RequiresPermission: true,
		},
		{
			ContractID: "app.menu.submenu", Version: 1,
			DesktopType: DesktopTypeMenuSubmenu,
			AllowedTargets: []string{
				"app.menu.file.extensions", "app.menu.edit.extensions",
				"app.menu.view.extensions", "app.menu.tools.extensions",
				"app.menu.help.extensions",
				"context.chat.message.extensions", "context.chat.composer.extensions",
				"context.extension.detail.extensions",
			},
			Status: ContractStatusActive, MaxItemsPerExt: 10, RequiresPermission: true,
		},
		{
			ContractID: "app.tray.item", Version: 1,
			DesktopType: DesktopTypeTrayItem,
			AllowedTargets: []string{"tray.quick_actions", "tray.extensions", "tray.status"},
			Status: ContractStatusActive, MaxItemsPerExt: 5, RequiresPermission: true,
		},
		{
			ContractID: "app.tray.submenu", Version: 1,
			DesktopType: DesktopTypeTraySubmenu,
			AllowedTargets: []string{"tray.quick_actions", "tray.extensions"},
			Status: ContractStatusActive, MaxItemsPerExt: 3, RequiresPermission: true,
		},
		{
			ContractID: "app.shortcut.application", Version: 1,
			DesktopType: DesktopTypeAppShortcut,
			AllowedTargets: []string{"window.focused", "page.scope"},
			Status: ContractStatusActive, MaxItemsPerExt: 10, RequiresPermission: true,
		},
		{
			ContractID: "app.shortcut.global", Version: 1,
			DesktopType: DesktopTypeGlobalShortcut,
			AllowedTargets: []string{"global"},
			Status: ContractStatusActive, MaxItemsPerExt: 3, RequiresPermission: true,
		},
	}
	for _, c := range builtins {
		r.registerBuiltin(c)
	}
}

func (r *DesktopContractRegistry) registerBuiltin(def DesktopContractDefinition) {
	if _, ok := r.contracts[def.ContractID]; !ok {
		r.contracts[def.ContractID] = make(map[int]DesktopContractDefinition)
	}
	r.contracts[def.ContractID][def.Version] = def
}

func (r *DesktopContractRegistry) RegisterContract(ctx context.Context, def DesktopContractDefinition) error {
	if def.ContractID == "" || def.Version <= 0 {
		return ErrInvalidContractID
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.contracts[def.ContractID]; ok {
		if existing, vok := r.contracts[def.ContractID][def.Version]; vok {
			if existing.Status == ContractStatusFrozen {
				return fmt.Errorf("%w: %s v%d", ErrContractFrozen, def.ContractID, def.Version)
			}
		}
	} else {
		r.contracts[def.ContractID] = make(map[int]DesktopContractDefinition)
	}
	r.contracts[def.ContractID][def.Version] = def
	return nil
}

func (r *DesktopContractRegistry) GetContract(ctx context.Context, contractID string, version int) (DesktopContractDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions, ok := r.contracts[contractID]
	if !ok {
		return DesktopContractDefinition{}, fmt.Errorf("%w: %s", ErrContractNotFound, contractID)
	}
	def, ok := versions[version]
	if !ok {
		return DesktopContractDefinition{}, fmt.Errorf("%w: %s v%d", ErrContractVersionNotFound, contractID, version)
	}
	return def, nil
}

func (r *DesktopContractRegistry) ListContracts() []DesktopContractDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]DesktopContractDefinition, 0)
	for _, versions := range r.contracts {
		for _, def := range versions {
			result = append(result, def)
		}
	}
	return result
}

func (r *DesktopContractRegistry) IsTargetAllowed(contractID string, version int, target string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions, ok := r.contracts[contractID]
	if !ok {
		return false
	}
	def, ok := versions[version]
	if !ok {
		return false
	}
	for _, t := range def.AllowedTargets {
		if t == target {
			return true
		}
	}
	return false
}

func (r *DesktopContractRegistry) MaxItemsPerExtension(contractID string, version int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions, ok := r.contracts[contractID]
	if !ok {
		return 0
	}
	def, ok := versions[version]
	if !ok {
		return 0
	}
	return def.MaxItemsPerExt
}
