package plugincontract

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrServiceGraphInvalid = errors.New("plugincontract: service graph invalid")
	ErrProviderNotSelected = errors.New("plugincontract: provider not selected")
)

type ServiceGraphInvalidError struct {
	Issues []ServiceGraphIssue
}

func (e *ServiceGraphInvalidError) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		detail := issue.Code
		if issue.Service != "" {
			detail += " service=" + issue.Service
		}
		if issue.Provider != "" {
			detail += " provider=" + issue.Provider
		}
		if issue.Module != "" {
			detail += " module=" + issue.Module
		}
		if issue.Detail != "" {
			detail += ": " + issue.Detail
		}
		parts = append(parts, detail)
	}
	return fmt.Sprintf("plugincontract: service graph invalid (%s)", strings.Join(parts, "; "))
}

func (e *ServiceGraphInvalidError) Unwrap() error { return ErrServiceGraphInvalid }

type ProviderNotSelectedError struct {
	ServiceID  string
	ProviderID string
	Revision   uint64
	Candidates []string
}

func (e *ProviderNotSelectedError) Error() string {
	target := e.ProviderID
	if target == "" {
		target = "<none>"
	}
	return fmt.Sprintf("plugincontract: provider %q is not selected for service %q at revision %d (candidates: %s)", target, e.ServiceID, e.Revision, strings.Join(e.Candidates, ", "))
}

func (e *ProviderNotSelectedError) Unwrap() error { return ErrProviderNotSelected }

type ProviderCandidates struct {
	ServiceID  string
	Candidates []ServiceDescriptor
	Selected   ServiceDescriptor
	Bound      bool
	Revision   uint64
	Epoch      string
	Scope      BindingScope
	Trail      []string
}

func SelectProviderCandidates(resolution ServiceGraphResolution, snapshot *BindingSnapshot, request BindingRequest, serviceID string) (ProviderCandidates, error) {
	trimmedService := strings.TrimSpace(serviceID)
	if trimmedService == "" {
		return ProviderCandidates{}, &BindingInvalidError{Field: "serviceId", Value: serviceID, Reason: "must not be empty"}
	}
	if !resolution.Valid() {
		return ProviderCandidates{}, &ServiceGraphInvalidError{Issues: append([]ServiceGraphIssue(nil), resolution.Issues...)}
	}
	if snapshot == nil {
		return ProviderCandidates{}, ErrSnapshotEmpty
	}
	candidates := graphCandidates(resolution.Graph, trimmedService)
	sortServiceDescriptors(candidates)
	bound, err := snapshot.Resolve(request)
	if err != nil {
		return ProviderCandidates{}, err
	}
	providerIDs := make([]string, len(candidates))
	for index, candidate := range candidates {
		providerIDs[index] = candidate.ProviderID
	}
	var selected ServiceDescriptor
	found := false
	for _, candidate := range candidates {
		if candidate.ProviderID == bound.Binding.ProviderID {
			selected = candidate
			found = true
			break
		}
	}
	if !found {
		return ProviderCandidates{}, &ProviderNotSelectedError{
			ServiceID:  trimmedService,
			ProviderID: bound.Binding.ProviderID,
			Revision:   bound.Revision,
			Candidates: providerIDs,
		}
	}
	ordered := make([]ServiceDescriptor, len(candidates))
	for index, candidate := range candidates {
		ordered[index] = normalizeServiceDescriptor(candidate)
	}
	return ProviderCandidates{
		ServiceID:  trimmedService,
		Candidates: ordered,
		Selected:   normalizeServiceDescriptor(selected),
		Bound:      true,
		Revision:   bound.Revision,
		Epoch:      bound.Epoch,
		Scope:      bound.Scope,
		Trail:      append([]string(nil), bound.Trail...),
	}, nil
}

func graphCandidates(graph ServiceGraph, serviceID string) []ServiceDescriptor {
	seen := make(map[string]bool)
	candidates := make([]ServiceDescriptor, 0, len(graph.SelectedProviders))
	for _, provider := range graph.SelectedProviders {
		if provider.ServiceID != serviceID || seen[provider.ProviderID] {
			continue
		}
		seen[provider.ProviderID] = true
		candidates = append(candidates, provider)
	}
	for _, bindings := range graph.Bindings {
		for _, binding := range bindings {
			if binding.ServiceID != serviceID {
				continue
			}
			for _, provider := range binding.Providers {
				if provider.ServiceID != serviceID || seen[provider.ProviderID] {
					continue
				}
				seen[provider.ProviderID] = true
				candidates = append(candidates, provider)
			}
		}
	}
	return candidates
}

func sortServiceDescriptors(candidates []ServiceDescriptor) {
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].Priority != candidates[right].Priority {
			return candidates[left].Priority > candidates[right].Priority
		}
		return candidates[left].ProviderID < candidates[right].ProviderID
	})
}
