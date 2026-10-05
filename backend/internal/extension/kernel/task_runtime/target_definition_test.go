package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type pinnedTargetProvider struct {
	pin TargetTaskDefinitionPin
	err error
}

func (p *pinnedTargetProvider) TargetTaskDefinition(context.Context, coordination.ExecutionScope, string) (TargetTaskDefinitionPin, error) {
	return p.pin, p.err
}

func TestTargetTaskDefinitionOwnerPinRejectsUpgradeAndConfirmsUnknownAck(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	definition.EntryHash = "sha256:" + strings.Repeat("a", 64)
	definition.InstalledGeneration = 7
	run.TaskDefinitionID = definition.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.Input = json.RawMessage(`{"private":"only-owner"}`)
	run.InputHash = hashBytes(run.Input)
	data := &taskInputData{}
	ctx := coordination.WithScope(t.Context(), authority)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	targetDefinition := *definition
	targetDefinition.InstalledGeneration = 99
	targetDefinition.DefinitionHash = "target-local-declaration"
	full, _ := taskDefinitionFingerprint(&targetDefinition)
	portable, _ := portableTaskDefinitionFingerprint(&targetDefinition)
	provider := &pinnedTargetProvider{pin: TargetTaskDefinitionPin{DeviceID: authority.TargetDeviceID, TaskID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InstalledGeneration: 99, DefinitionFingerprint: full, PortableFingerprint: portable, EntryHash: definition.EntryHash}}
	port := AcknowledgedTaskTargetDefinitionPort{Data: data, Provider: provider}
	data.wrongAck = true
	if _, err := port.Prepare(ctx, run, definition); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("invalid ACK accepted: %v", err)
	}
	data.wrongAck = false
	pin, err := port.Prepare(ctx, run, definition)
	if err != nil || pin != provider.pin {
		t.Fatalf("durable pin confirmation failed: %+v %v", pin, err)
	}
	stored := data.resources["checkpoint/task/target-definition/"+run.TaskRunID]
	if stored == nil || stored.Revision != 1 || stored.OwnerID != authority.ResourceOwnerID || strings.Contains(string(stored.Body), "only-owner") {
		t.Fatal("target pin stored input or changed owner/version")
	}
	if _, err := port.Prepare(ctx, run, definition); err != nil || stored.Revision != 1 {
		t.Fatalf("repeated pin changed version: %v", err)
	}
	provider.pin.InstalledGeneration++
	if _, err := port.Prepare(ctx, run, definition); !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) {
		t.Fatalf("upgrade silently replaced old pin: %v", err)
	}
	provider.pin.InstalledGeneration--
	provider.pin.DeviceID = "other-device"
	if _, err := port.Prepare(ctx, run, definition); !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) {
		t.Fatalf("foreign target accepted: %v", err)
	}
	provider.pin.DeviceID = authority.TargetDeviceID
	provider.err = errors.New("offline")
	if _, err := port.Prepare(ctx, run, definition); !errors.Is(err, provider.err) {
		t.Fatalf("offline reused stale advertisement: %v", err)
	}
}

func TestInstalledTaskDescriptionRechecksInstallationAfterEntryResolution(t *testing.T) {
	definition := &TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module", Entry: "task.cjs", EntryHash: "sha256:" + strings.Repeat("a", 64), InstalledGeneration: 2}
	config := DefaultTaskRuntimeConfig()
	var checks int
	changed := false
	config.InstalledDefinitionValidator = func(context.Context, *TaskDefinition) error {
		checks++
		if changed {
			return coordination.ErrScopeExpired
		}
		return nil
	}
	config.EntryResolver = func(context.Context, *TaskDefinition) (string, error) { return "entry", nil }
	service := NewTaskRuntimeService(&pauseStore{def: definition}, config)
	pin, err := service.DescribeInstalledTask(t.Context(), "task", "device")
	if err != nil || checks != 2 || pin.InstalledGeneration != 2 || !validTaskFingerprint(pin.DefinitionFingerprint) {
		t.Fatalf("description=%+v checks=%d err=%v", pin, checks, err)
	}
	service.config.EntryResolver = func(context.Context, *TaskDefinition) (string, error) { changed = true; return "entry", nil }
	if _, err := service.DescribeInstalledTask(t.Context(), "task", "device"); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("installation replacement accepted: %v", err)
	}
}
