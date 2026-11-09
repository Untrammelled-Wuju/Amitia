package plugincontract

import (
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type ModuleRef struct {
	PluginID string `json:"pluginId"`
	ModuleID string `json:"moduleId"`
}

func (m ModuleRef) Key() string {
	return strings.TrimSpace(m.PluginID) + "/" + strings.TrimSpace(m.ModuleID)
}

type ServiceDescriptor struct {
	ProviderID   string           `json:"providerId"`
	Module       ModuleRef        `json:"module"`
	ServiceID    string           `json:"serviceId"`
	Version      string           `json:"version"`
	ContractHash string           `json:"contractHash,omitempty"`
	Scopes       []ServiceScope   `json:"scopes"`
	Placement    ServicePlacement `json:"placement"`
	Priority     int              `json:"priority,omitempty"`
	Disabled     bool             `json:"disabled,omitempty"`
}

type ServiceConsumer struct {
	Module       ModuleRef            `json:"module"`
	Requirements []ServiceRequirement `json:"requirements"`
}

type ServiceGraphRequest struct {
	Providers []ServiceDescriptor `json:"providers"`
	Consumers []ServiceConsumer   `json:"consumers"`
}

type ServiceEdge struct {
	Consumer string `json:"consumer"`
	Provider string `json:"provider"`
	Service  string `json:"service"`
}

type ServiceGraphIssue struct {
	Code     string `json:"code"`
	Module   string `json:"module,omitempty"`
	Service  string `json:"service,omitempty"`
	Provider string `json:"provider,omitempty"`
	Detail   string `json:"detail"`
}

type ServiceBinding struct {
	ServiceID string              `json:"serviceId"`
	Providers []ServiceDescriptor `json:"providers"`
}

type ServiceGraph struct {
	ActivationOrder   []string                     `json:"activationOrder"`
	ReverseOrder      []string                     `json:"reverseOrder"`
	Bindings          map[string][]ServiceBinding  `json:"bindings"`
	Edges             []ServiceEdge                `json:"edges"`
	SelectedProviders map[string]ServiceDescriptor `json:"selectedProviders"`
}

type ServiceGraphResolution struct {
	Graph  ServiceGraph        `json:"graph"`
	Issues []ServiceGraphIssue `json:"issues,omitempty"`
}

func (r ServiceGraphResolution) Valid() bool {
	return len(r.Issues) == 0
}

type ServiceGraphResolver struct{}

func (ServiceGraphResolver) Resolve(request ServiceGraphRequest) ServiceGraphResolution {
	var result ServiceGraphResolution
	result.Graph.Bindings = make(map[string][]ServiceBinding)
	result.Graph.SelectedProviders = make(map[string]ServiceDescriptor)
	providersByService := make(map[string][]ServiceDescriptor)
	providerIDs := make(map[string]bool)
	nodes := make(map[string]bool)
	edgesByConsumer := make(map[string]map[string]bool)

	for index := range request.Providers {
		provider := normalizeServiceDescriptor(request.Providers[index])
		if issue := validateProvider(provider); issue != nil {
			result.Issues = append(result.Issues, *issue)
			continue
		}
		if providerIDs[provider.ProviderID] {
			result.Issues = append(result.Issues, ServiceGraphIssue{
				Code:     "duplicate_provider",
				Service:  provider.ServiceID,
				Provider: provider.ProviderID,
				Detail:   "provider id is duplicated",
			})
			continue
		}
		providerIDs[provider.ProviderID] = true
		nodes[provider.Module.Key()] = true
		if !provider.Disabled {
			providersByService[provider.ServiceID] = append(providersByService[provider.ServiceID], provider)
		}
	}

	consumerIDs := make(map[string]bool)
	for index := range request.Consumers {
		consumer := request.Consumers[index]
		consumerKey := consumer.Module.Key()
		if consumer.Module.PluginID == "" || consumer.Module.ModuleID == "" {
			result.Issues = append(result.Issues, ServiceGraphIssue{
				Code:   "invalid_consumer",
				Module: consumerKey,
				Detail: "consumer module identity is required",
			})
			continue
		}
		if consumerIDs[consumerKey] {
			result.Issues = append(result.Issues, ServiceGraphIssue{
				Code:   "duplicate_consumer",
				Module: consumerKey,
				Detail: "consumer module is duplicated",
			})
			continue
		}
		consumerIDs[consumerKey] = true
		nodes[consumerKey] = true
		result.Graph.Bindings[consumerKey] = make([]ServiceBinding, 0)

		for requirementIndex := range consumer.Requirements {
			requirement := normalizeServiceRequirement(consumer.Requirements[requirementIndex])
			selected, issues := resolveServiceRequirement(consumerKey, requirement, providersByService[requirement.ServiceID])
			result.Issues = append(result.Issues, issues...)
			if len(selected) == 0 {
				continue
			}
			result.Graph.Bindings[consumerKey] = append(result.Graph.Bindings[consumerKey], ServiceBinding{
				ServiceID: requirement.ServiceID,
				Providers: append([]ServiceDescriptor(nil), selected...),
			})
			for _, provider := range selected {
				result.Graph.SelectedProviders[provider.ProviderID] = provider
				providerKey := provider.Module.Key()
				if providerKey == consumerKey {
					continue
				}
				if edgesByConsumer[consumerKey] == nil {
					edgesByConsumer[consumerKey] = make(map[string]bool)
				}
				if edgesByConsumer[consumerKey][providerKey] {
					continue
				}
				edgesByConsumer[consumerKey][providerKey] = true
				result.Graph.Edges = append(result.Graph.Edges, ServiceEdge{
					Consumer: consumerKey,
					Provider: providerKey,
					Service:  requirement.ServiceID,
				})
			}
		}
	}

	result.Graph.ActivationOrder = activationOrder(nodes, edgesByConsumer)
	if len(result.Graph.ActivationOrder) != len(nodes) {
		result.Issues = append(result.Issues, ServiceGraphIssue{
			Code:   "service_dependency_cycle",
			Detail: "service dependency graph contains a cycle",
		})
		return result
	}
	result.Graph.ReverseOrder = reverseStrings(result.Graph.ActivationOrder)
	sort.Slice(result.Graph.Edges, func(i, j int) bool {
		if result.Graph.Edges[i].Provider != result.Graph.Edges[j].Provider {
			return result.Graph.Edges[i].Provider < result.Graph.Edges[j].Provider
		}
		if result.Graph.Edges[i].Consumer != result.Graph.Edges[j].Consumer {
			return result.Graph.Edges[i].Consumer < result.Graph.Edges[j].Consumer
		}
		return result.Graph.Edges[i].Service < result.Graph.Edges[j].Service
	})
	return result
}

func validateProvider(provider ServiceDescriptor) *ServiceGraphIssue {
	if strings.TrimSpace(provider.ProviderID) == "" {
		return &ServiceGraphIssue{Code: "invalid_provider", Detail: "provider id is required"}
	}
	if provider.Module.PluginID == "" || provider.Module.ModuleID == "" {
		return &ServiceGraphIssue{Code: "invalid_provider", Provider: provider.ProviderID, Detail: "provider module identity is required"}
	}
	if !serviceIDPattern.MatchString(provider.ServiceID) {
		return &ServiceGraphIssue{Code: "invalid_service", Provider: provider.ProviderID, Detail: "service id is invalid"}
	}
	if _, err := domain.ParseVersion(provider.Version); err != nil {
		return &ServiceGraphIssue{Code: "invalid_service_version", Provider: provider.ProviderID, Service: provider.ServiceID, Detail: err.Error()}
	}
	if len(provider.Scopes) == 0 {
		return &ServiceGraphIssue{Code: "invalid_service_scope", Provider: provider.ProviderID, Service: provider.ServiceID, Detail: "service scopes are required"}
	}
	for _, scope := range provider.Scopes {
		if !scope.Valid() {
			return &ServiceGraphIssue{Code: "invalid_service_scope", Provider: provider.ProviderID, Service: provider.ServiceID, Detail: "service scope is invalid"}
		}
	}
	if !provider.Placement.Valid() {
		return &ServiceGraphIssue{Code: "invalid_service_placement", Provider: provider.ProviderID, Service: provider.ServiceID, Detail: "service placement is invalid"}
	}
	return nil
}

func resolveServiceRequirement(consumer string, requirement ServiceRequirement, providers []ServiceDescriptor) ([]ServiceDescriptor, []ServiceGraphIssue) {
	if !serviceIDPattern.MatchString(requirement.ServiceID) {
		return nil, []ServiceGraphIssue{{Code: "invalid_service_requirement", Module: consumer, Service: requirement.ServiceID, Detail: "service id is invalid"}}
	}
	versionRange, err := dependency.ParseRange(requirement.VersionRange)
	if err != nil {
		return nil, []ServiceGraphIssue{{Code: "invalid_version_range", Module: consumer, Service: requirement.ServiceID, Detail: err.Error()}}
	}
	var matched []ServiceDescriptor
	for _, provider := range providers {
		version, err := domain.ParseVersion(provider.Version)
		if err != nil || !versionRange.Satisfies(version) {
			continue
		}
		if requirement.ContractHash != "" && provider.ContractHash != requirement.ContractHash {
			continue
		}
		matched = append(matched, provider)
	}
	if len(matched) == 0 {
		if requirement.Optional {
			return nil, nil
		}
		return nil, []ServiceGraphIssue{{
			Code:    "required_service_unavailable",
			Module:  consumer,
			Service: requirement.ServiceID,
			Detail:  "no provider satisfies the declared service contract",
		}}
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Priority != matched[j].Priority {
			return matched[i].Priority > matched[j].Priority
		}
		left, _ := domain.ParseVersion(matched[i].Version)
		right, _ := domain.ParseVersion(matched[j].Version)
		if comparison := left.Compare(right); comparison != 0 {
			return comparison > 0
		}
		return matched[i].ProviderID < matched[j].ProviderID
	})
	switch requirement.Selection {
	case ServiceSelectionAll:
		return matched, nil
	case ServiceSelectionExclusive:
		if len(matched) == 1 {
			return matched, nil
		}
		return nil, []ServiceGraphIssue{{
			Code:    "exclusive_service_conflict",
			Module:  consumer,
			Service: requirement.ServiceID,
			Detail:  "exclusive service requirement matched multiple providers",
		}}
	default:
		if len(matched) > 1 && matched[0].Priority == matched[1].Priority {
			return nil, []ServiceGraphIssue{{
				Code:    "service_provider_conflict",
				Module:  consumer,
				Service: requirement.ServiceID,
				Detail:  "multiple providers share the highest priority",
			}}
		}
		return matched[:1], nil
	}
}

func activationOrder(nodes map[string]bool, edgesByConsumer map[string]map[string]bool) []string {
	indegree := make(map[string]int, len(nodes))
	consumersByProvider := make(map[string][]string)
	for node := range nodes {
		indegree[node] = 0
	}
	for consumer, providers := range edgesByConsumer {
		for provider := range providers {
			indegree[consumer]++
			consumersByProvider[provider] = append(consumersByProvider[provider], consumer)
		}
	}
	var ready []string
	for node, degree := range indegree {
		if degree == 0 {
			ready = append(ready, node)
		}
	}
	sort.Strings(ready)
	var order []string
	for len(ready) > 0 {
		node := ready[0]
		ready = ready[1:]
		order = append(order, node)
		consumers := consumersByProvider[node]
		sort.Strings(consumers)
		for _, consumer := range consumers {
			indegree[consumer]--
			if indegree[consumer] == 0 {
				ready = append(ready, consumer)
				sort.Strings(ready)
			}
		}
	}
	return order
}

func normalizeServiceDescriptor(provider ServiceDescriptor) ServiceDescriptor {
	provider.ProviderID = strings.TrimSpace(provider.ProviderID)
	provider.Module.PluginID = strings.TrimSpace(provider.Module.PluginID)
	provider.Module.ModuleID = strings.TrimSpace(provider.Module.ModuleID)
	provider.ServiceID = strings.TrimSpace(provider.ServiceID)
	provider.Version = strings.TrimSpace(provider.Version)
	provider.ContractHash = strings.TrimSpace(provider.ContractHash)
	provider.Scopes = append([]ServiceScope(nil), provider.Scopes...)
	return provider
}

func normalizeServiceRequirement(requirement ServiceRequirement) ServiceRequirement {
	requirement.ServiceID = strings.TrimSpace(requirement.ServiceID)
	requirement.VersionRange = strings.TrimSpace(requirement.VersionRange)
	requirement.ContractHash = strings.TrimSpace(requirement.ContractHash)
	if requirement.Selection == "" {
		requirement.Selection = ServiceSelectionSingle
	}
	return requirement
}

func reverseStrings(values []string) []string {
	result := append([]string(nil), values...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}
