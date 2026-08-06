package runtimeorchestrator

import "context"

type ComponentID string

const (
	ComponentSQLite          ComponentID = "core.sqlite"
	ComponentVectorStore     ComponentID = "provider.vector-store"
	ComponentGraphStore      ComponentID = "provider.graph-store"
	ComponentSidecars        ComponentID = "component.channel-sidecars"
	ComponentExtensionKernel ComponentID = "component.extension-kernel"
	ComponentTaskRuntime     ComponentID = "component.task-runtime"
	ComponentDesktopPet      ComponentID = "component.desktop-pet"
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
}

type ManagedComponent interface {
	Descriptor() ComponentDescriptor
	Start(context.Context) error
	Ready(context.Context) error
	Stop(context.Context) error
}

func validateDescriptor(desc ComponentDescriptor) error {
	if desc.ID == "" {
		return invalidDescriptorErr("component ID is empty")
	}
	if desc.Phase != PhaseInfrastructure && desc.Phase != PhaseApplication {
		return invalidDescriptorErr("unknown phase: " + string(desc.Phase))
	}
	if desc.Required && !desc.Enabled {
		return invalidDescriptorErr("required component must be enabled")
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
