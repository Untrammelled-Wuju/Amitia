package runtimeorchestrator

import (
	"context"

	"github.com/u-ai/backend/internal/runtimeprofile"
)

type ComponentID string

const (
	ComponentSQLite          ComponentID = "core.sqlite"
	ComponentVectorStore     ComponentID = "provider.vector-store"
	ComponentGraphStore      ComponentID = "provider.graph-store"
	ComponentSidecars        ComponentID = "component.channel-sidecars"
	ComponentExtensionKernel ComponentID = "component.extension-kernel"
	ComponentTaskRuntime     ComponentID = "component.task-runtime"
	ComponentDesktopPet      ComponentID = "component.desktop-pet"
	ComponentDesktopPetMesh  ComponentID = "component.desktop-pet-behavior-mesh"
	ComponentBrowser         ComponentID = "component.browser"
)

type ComponentPhase string

const (
	PhaseInfrastructure ComponentPhase = "infrastructure"
	PhaseApplication    ComponentPhase = "application"
)

type ComponentDescriptor struct {
	ID           ComponentID
	Phase        ComponentPhase
	Enabled      bool
	Required     bool
	Dependencies []ComponentID
	Capabilities []string
	Profiles     []runtimeprofile.Profile
}

func (d ComponentDescriptor) SupportsProfile(profile runtimeprofile.Profile) bool {
	if len(d.Profiles) == 0 {
		return true
	}
	for _, p := range d.Profiles {
		if p == profile {
			return true
		}
	}
	return false
}

type ManagedComponent interface {
	Descriptor() ComponentDescriptor
	Start(context.Context) error
	Ready(context.Context) error
	Stop(context.Context) error
}

func validateDescriptor(desc ComponentDescriptor, profile runtimeprofile.Profile) error {
	if desc.ID == "" {
		return invalidDescriptorErr("component ID is empty")
	}
	if desc.Phase != PhaseInfrastructure && desc.Phase != PhaseApplication {
		return invalidDescriptorErr("unknown phase: " + string(desc.Phase))
	}
	if desc.Required && !desc.Enabled {
		return invalidDescriptorErr("required component must be enabled")
	}
	if desc.Required && desc.Enabled && !desc.SupportsProfile(profile) {
		return invalidDescriptorErr("required component not supported by profile: " + string(profile))
	}
	for i, dep := range desc.Dependencies {
		if dep == desc.ID {
			return invalidDescriptorErr("self-dependency: " + string(dep))
		}
		for j := i + 1; j < len(desc.Dependencies); j++ {
			if desc.Dependencies[j] == dep {
				return invalidDescriptorErr("duplicate dependency: " + string(dep))
			}
		}
	}
	return nil
}

func cloneCapabilities(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneDependencies(src []ComponentID) []ComponentID {
	if src == nil {
		return nil
	}
	dst := make([]ComponentID, len(src))
	copy(dst, src)
	return dst
}
