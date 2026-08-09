package domain

import "fmt"

type DomainFilter struct {
	Domains []ExtensionDomain
}

func (f DomainFilter) Matches(domain ExtensionDomain) bool {
	if len(f.Domains) == 0 {
		return true
	}
	for _, d := range f.Domains {
		if d == domain {
			return true
		}
	}
	return false
}

type ManagementFilter struct {
	Targets []ManagementTarget
}

func (f ManagementFilter) Matches(target ManagementTarget) bool {
	if len(f.Targets) == 0 {
		return true
	}
	for _, t := range f.Targets {
		if t == target {
			return true
		}
	}
	return false
}

func FilterByDomain(extensions []ExtensionDefinition, filter DomainFilter) []ExtensionDefinition {
	if len(filter.Domains) == 0 {
		result := make([]ExtensionDefinition, len(extensions))
		copy(result, extensions)
		return result
	}
	var result []ExtensionDefinition
	for _, ext := range extensions {
		if filter.Matches(ext.Domain) {
			result = append(result, ext)
		}
	}
	return result
}

func FilterByManagementTarget(extensions []ExtensionDefinition, filter ManagementFilter) ([]ExtensionDefinition, error) {
	if len(filter.Targets) == 0 {
		result := make([]ExtensionDefinition, len(extensions))
		copy(result, extensions)
		return result, nil
	}
	var result []ExtensionDefinition
	for _, ext := range extensions {
		target, err := ManagementTargetForDomain(ext.Domain)
		if err != nil {
			return nil, fmt.Errorf("filter: extension %s has invalid domain %s: %w", ext.ID, ext.Domain, err)
		}
		if filter.Matches(target) {
			result = append(result, ext)
		}
	}
	return result, nil
}

func FilterExtensionCenter(extensions []ExtensionDefinition) []ExtensionDefinition {
	return FilterByDomain(extensions, DomainFilter{Domains: []ExtensionDomain{ExtensionDomainGeneral}})
}

func FilterGameCenter(extensions []ExtensionDefinition) []ExtensionDefinition {
	return FilterByDomain(extensions, DomainFilter{Domains: []ExtensionDomain{ExtensionDomainGame}})
}

func FilterDesktopPetCenter(extensions []ExtensionDefinition) []ExtensionDefinition {
	return FilterByDomain(extensions, DomainFilter{Domains: []ExtensionDomain{ExtensionDomainDesktopPet}})
}
