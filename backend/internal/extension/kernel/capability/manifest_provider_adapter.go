package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

var ErrManifestProviderIDCollision = errors.New("capability: manifest provider id collision")

type ManifestProviderError struct {
	ExtensionID string
	ModuleID    string
	CapabilityID CapabilityID
	Err         error
}

func (e ManifestProviderError) Error() string {
	return fmt.Sprintf("manifest provider error: ext=%s mod=%s cap=%s: %v",
		e.ExtensionID, e.ModuleID, e.CapabilityID, e.Err)
}

func BuildManifestProviderID(extensionID domain.ExtensionID, moduleID domain.ModuleID, providerMeta *domain.ProviderMetadata, capabilityID CapabilityID) ProviderID {
	var seed string
	if providerMeta != nil && providerMeta.ID != "" {
		seed = string(extensionID) + "|" + string(moduleID) + "|" + providerMeta.ID + "|" + string(capabilityID)
	} else {
		seed = string(extensionID) + "|" + string(moduleID) + "|" + string(capabilityID)
	}
	sum := sha256.Sum256([]byte(seed))
	return ProviderID("extprov_" + hex.EncodeToString(sum[:]))
}

func runtimeTypeFromManifest(runtimeType domain.RuntimeType, placement domain.ModulePlacement) RuntimeType {
	switch runtimeType {
	case domain.RuntimeTypeBuiltin:
		return RuntimeTypeBuiltin
	case domain.RuntimeTypeJavaScript:
		if placement == domain.ModulePlacementDevice {
			return RuntimeTypePluginJS
		}
		return RuntimeTypeJavaScript
	case domain.RuntimeTypeWASM:
		return RuntimeTypeWASM
	case domain.RuntimeTypeService:
		return RuntimeTypePluginService
	case domain.RuntimeTypeMCP:
		return RuntimeTypeMCP
	default:
		return RuntimeTypeLegacy
	}
}

func runtimeBindingFromManifestModule(mod domain.ModuleDefinition) RuntimeBinding {
	if mod.Runtime == nil {
		return RuntimeBinding{
			RuntimeType: RuntimeTypeLegacy,
			RuntimeID:   string(mod.ID),
			HandlerName: string(mod.ID),
		}
	}
	rt := runtimeTypeFromManifest(mod.Runtime.Type, mod.Placement)
	metadata := make(map[string]any, len(mod.Runtime.Env))
	for k, v := range mod.Runtime.Env {
		metadata[k] = v
	}
	return RuntimeBinding{
		RuntimeType: rt,
		RuntimeID:   string(mod.ID),
		HandlerName: mod.Runtime.EntryPoint,
		Endpoint:    mod.Runtime.EntryPoint,
		Metadata:    metadata,
	}
}

func ProviderDefinitionsFromExtension(def domain.ExtensionDefinition) ([]CapabilityProviderDefinition, error) {
	var results []CapabilityProviderDefinition
	seen := make(map[ProviderID]bool)

	for _, mod := range def.Modules {
		if mod.Provider == nil && len(mod.ProvidedCapabilities) == 0 {
			continue
		}

		providerMeta := mod.Provider
		capabilities := mod.ProvidedCapabilities
		if len(capabilities) == 0 && providerMeta != nil {
			continue
		}

		binding := runtimeBindingFromManifestModule(mod)
		placement := ProviderPlacementCore
		if mod.Placement == domain.ModulePlacementDevice {
			placement = ProviderPlacementDevice
		}

		for _, pc := range capabilities {
			var providerID ProviderID
			var priority int
			var providerMetadata map[string]any
			var platforms []runtimeidentity.Platform

			if providerMeta != nil && providerMeta.ID != "" {
				providerID = ParseProviderID(providerMeta.ID)
				priority = providerMeta.Priority
				providerMetadata = providerMeta.Metadata
			} else {
				providerID = BuildManifestProviderID(def.ID, mod.ID, nil, CapabilityID(pc.ID))
				priority = 0
			}

			if seen[providerID] {
				return nil, ManifestProviderError{
					ExtensionID: string(def.ID),
					ModuleID:    string(mod.ID),
					CapabilityID: CapabilityID(pc.ID),
					Err:         ErrManifestProviderIDCollision,
				}
			}
			seen[providerID] = true

			capID := ParseCapabilityID(pc.ID)

			providerDef := CapabilityProviderDefinition{
				ID:           providerID,
				CapabilityID: capID,
				Kind:         ProviderKindExtension,
				Placement:    placement,
				ExtensionID:  string(def.ID),
				ModuleID:     string(mod.ID),
				Runtime:      binding,
				Priority:     priority,
				Platforms:    platforms,
				Metadata:     providerMetadata,
			}
			if pc.Metadata != nil {
				if providerDef.Metadata == nil {
					providerDef.Metadata = make(map[string]any)
				}
				for k, v := range pc.Metadata {
					providerDef.Metadata[k] = v
				}
			}

			results = append(results, providerDef)
		}
	}

	return results, nil
}
