//go:build rsi_routeconformance

package routeconformance

import (
	"fmt"
	"sort"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

const (
	selectionService = "memory.search"
	selectionSpace   = "space-a"
	selectionDevice  = "device-1"
)

func ProviderDescriptors(count int) []plugincontract.ServiceDescriptor {
	providers := make([]plugincontract.ServiceDescriptor, 0, count)
	for index := 0; index < count; index++ {
		providers = append(providers, plugincontract.ServiceDescriptor{
			ProviderID: fmt.Sprintf("provider.%02d", index),
			Module:     plugincontract.ModuleRef{PluginID: fmt.Sprintf("plugin.%02d", index), ModuleID: "module.main"},
			ServiceID:  selectionService,
			Version:    "1.0.0",
			Scopes:     []plugincontract.ServiceScope{plugincontract.ServiceScopeGlobal},
			Placement:  plugincontract.ServicePlacementCore,
			Priority:   count - index,
		})
	}
	return providers
}

func SelectionResolution(providers []plugincontract.ServiceDescriptor) (plugincontract.ServiceGraphResolution, error) {
	resolution := plugincontract.ServiceGraphResolver{}.Resolve(plugincontract.ServiceGraphRequest{
		Providers: providers,
		Consumers: []plugincontract.ServiceConsumer{{
			Module: plugincontract.ModuleRef{PluginID: "consumer.plugin", ModuleID: "module.main"},
			Requirements: []plugincontract.ServiceRequirement{{
				ServiceID:    selectionService,
				VersionRange: ">=1.0.0",
				Selection:    plugincontract.ServiceSelectionAll,
			}},
		}},
	})
	if !resolution.Valid() {
		return plugincontract.ServiceGraphResolution{}, fmt.Errorf("routeconformance: service graph invalid: %v", resolution.Issues)
	}
	return resolution, nil
}

func SelectionSnapshot(providerID string) (*plugincontract.BindingSnapshot, error) {
	return plugincontract.NewBindingSnapshot("conformance-epoch", 11, []plugincontract.Binding{{
		Key:                plugincontract.BindingKey{CapabilityID: selectionService, Scope: plugincontract.BindingScopeGlobal},
		ProviderID:         providerID,
		ProviderInstanceID: providerID + ":1",
	}})
}

func SelectionRequest() plugincontract.BindingRequest {
	return plugincontract.BindingRequest{CapabilityID: selectionService, SpaceID: selectionSpace, DeviceID: selectionDevice}
}

func CandidateProviderIDs(candidates plugincontract.ProviderCandidates) []string {
	ids := make([]string, len(candidates.Candidates))
	for index, candidate := range candidates.Candidates {
		ids[index] = candidate.ProviderID
	}
	return ids
}

func DescendingPriorities(candidates plugincontract.ProviderCandidates) bool {
	priorities := make([]int, len(candidates.Candidates))
	for index, candidate := range candidates.Candidates {
		priorities[index] = candidate.Priority
	}
	return sort.SliceIsSorted(priorities, func(left, right int) bool { return priorities[left] > priorities[right] })
}
