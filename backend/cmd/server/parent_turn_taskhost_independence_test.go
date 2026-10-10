package main

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/interaction"
)

func TestParentRecoveryIndependentOfTaskHostAndMultiAgent(t *testing.T) {
	before := config.AppCfg.Components.TaskHost.Enabled
	config.AppCfg.Components.TaskHost.Enabled = false
	defer func() { config.AppCfg.Components.TaskHost.Enabled = before }()
	svc := &AppServices{
		UnifiedEntry:    &interaction.UnifiedEntry{},
		KernelContainer: &kernel.Container{},
	}
	component := newTaskRuntimeComponent(svc)
	if !component.enabled || component.taskHostEnabled {
		t.Fatalf("parent recovery disabled alongside TaskHost: %+v", component)
	}
	descriptor := component.Descriptor()
	if !descriptor.Enabled || len(descriptor.Capabilities) != 1 ||
		descriptor.Capabilities[0] != "interaction.parent_recovery" {
		t.Fatalf("disabled TaskHost falsely advertised or suppressed parent recovery: %+v", descriptor)
	}
	if err := component.Start(context.Background()); err != nil {
		t.Fatalf("parent recovery watcher failed without TaskHost service: %v", err)
	}
	if component.parentRecovery == nil || component.reconcileDone == nil {
		t.Fatal("ordinary parent recovery watcher did not start")
	}
	if err := component.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := component.Stop(ctx); err != nil {
		t.Fatalf("disabled TaskHost parent recovery watcher failed to stop: %v", err)
	}
}
