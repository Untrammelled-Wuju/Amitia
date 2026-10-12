package main

import (
	"testing"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/pkg/platform"
)

func TestCloudApplicationRegistrationWithDefaultDesktopPet(t *testing.T) {
	previous := config.AppCfg
	t.Cleanup(func() { config.AppCfg = previous })
	config.AppCfg = &config.Config{}
	config.AppCfg.DesktopPetRuntime.Enabled = true
	services := &AppServices{
		RuntimeProfile: runtimeprofile.ProfileCloudCore,
		RuntimePolicy:  runtimeprofile.PolicyFor(runtimeprofile.ProfileCloudCore),
	}
	bootstrap := &runtimeBootstrap{
		orchestrator: runtimeorchestrator.NewWithProfile(platform.RuntimeDescriptor{}, runtimeprofile.ProfileCloudCore),
	}
	if err := bootstrap.RegisterApplication(services); err != nil {
		t.Fatalf("Cloud Core application registration failed: %v", err)
	}
	desc := newDesktopPetComponent(services).Descriptor()
	if desc.Enabled || desc.Required {
		t.Fatal("Cloud Core requires a device-local pet runtime")
	}
}

func TestDeviceDesktopPetRegistrationHonorsConfiguration(t *testing.T) {
	previous := config.AppCfg
	t.Cleanup(func() { config.AppCfg = previous })
	config.AppCfg = &config.Config{}
	for _, profile := range []runtimeprofile.Profile{runtimeprofile.ProfileLocal, runtimeprofile.ProfileDeviceAgent} {
		for _, enabled := range []bool{false, true} {
			config.AppCfg.DesktopPetRuntime.Enabled = enabled
			services := &AppServices{RuntimeProfile: profile, RuntimePolicy: runtimeprofile.PolicyFor(profile)}
			component := newDesktopPetComponent(services)
			orchestrator := runtimeorchestrator.NewWithProfile(platform.RuntimeDescriptor{}, profile)
			if err := orchestrator.Register(component); err != nil {
				t.Fatalf("%s registration failed: %v", profile, err)
			}
			desc := component.Descriptor()
			if desc.Enabled != enabled || desc.Required != enabled {
				t.Fatalf("%s pet configuration changed: %+v", profile, desc)
			}
		}
	}
}
