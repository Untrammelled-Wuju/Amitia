package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

func TestHostCommandRegistry_RegisterAndExecute(t *testing.T) {
	registry := NewHostCommandRegistry()
	executed := false

	err := registry.Register(HostCommandDefinition{
		CommandID:   "test.echo",
		Description: "Test echo command",
		Permission:  "ui.navigate",
		Scope:       HostCommandScopeGlobal,
		Risk:        host_api.RiskLow,
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
			executed = true
			return json.RawMessage(`{"echo":true}`), nil
		},
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	result, err := registry.Execute(
		context.Background(),
		"test.echo",
		HostCommandExecContext{ExtensionID: "ext-1"},
		nil,
	)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !executed {
		t.Fatal("handler was not called")
	}

	var out map[string]any
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if out["echo"] != true {
		t.Fatalf("expected echo=true, got %v", out["echo"])
	}
}

func TestHostCommandRegistry_UnregisteredCommandReturnsNotFound(t *testing.T) {
	registry := NewHostCommandRegistry()

	_, err := registry.Execute(
		context.Background(),
		"app.nonexistent",
		HostCommandExecContext{ExtensionID: "ext-1"},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for unregistered command, got nil")
	}

	var hcErr *HostCommandError
	if !AsHostCommandError(err, &hcErr) {
		t.Fatalf("expected HostCommandError, got %T: %v", err, err)
	}
	if hcErr.Code != ErrCodeHostCommandNotFound {
		t.Fatalf("expected code %s, got %s", ErrCodeHostCommandNotFound, hcErr.Code)
	}
}

func TestHostCommandRegistry_RegisterValidation(t *testing.T) {
	tests := []struct {
		name string
		def  HostCommandDefinition
		err  string
	}{
		{
			name: "empty command ID",
			def: HostCommandDefinition{
				Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
					return nil, nil
				},
				Permission: "ui.navigate",
			},
			err: "command_id is required",
		},
		{
			name: "nil handler",
			def: HostCommandDefinition{
				CommandID:  "test.nil",
				Permission: "ui.navigate",
			},
			err: "handler is required",
		},
		{
			name: "empty permission",
			def: HostCommandDefinition{
				CommandID: "test.noperm",
				Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
					return nil, nil
				},
			},
			err: "permission is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewHostCommandRegistry()
			err := registry.Register(tt.def)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !contains(err.Error(), tt.err) {
				t.Fatalf("expected error containing %q, got %q", tt.err, err.Error())
			}
		})
	}
}

func TestHostCommandRegistry_DuplicateRegistration(t *testing.T) {
	registry := NewHostCommandRegistry()
	handler := func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
		return nil, nil
	}

	def := HostCommandDefinition{
		CommandID:  "test.dup",
		Permission: "ui.navigate",
		Handler:    handler,
	}

	if err := registry.Register(def); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := registry.Register(def)
	if err == nil {
		t.Fatal("expected error for duplicate registration, got nil")
	}
	if !contains(err.Error(), "already registered") {
		t.Fatalf("expected 'already registered' error, got %q", err.Error())
	}
}

func TestHostCommandRegistry_List(t *testing.T) {
	registry := NewHostCommandRegistry()
	handler := func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
		return nil, nil
	}

	cmds := []string{"cmd.a", "cmd.b", "cmd.c"}
	for _, id := range cmds {
		_ = registry.Register(HostCommandDefinition{
			CommandID:  id,
			Permission: "ui.navigate",
			Handler:    handler,
		})
	}

	list := registry.List()
	if len(list) != len(cmds) {
		t.Fatalf("expected %d commands, got %d", len(cmds), len(list))
	}

	if registry.Count() != len(cmds) {
		t.Fatalf("expected count %d, got %d", len(cmds), registry.Count())
	}
}

func TestHostCommandRegistry_Unregister(t *testing.T) {
	registry := NewHostCommandRegistry()
	_ = registry.Register(HostCommandDefinition{
		CommandID:  "test.tmp",
		Permission: "ui.navigate",
		Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
			return nil, nil
		},
	})

	if !registry.IsRegistered("test.tmp") {
		t.Fatal("expected command to be registered")
	}

	registry.Unregister("test.tmp")

	if registry.IsRegistered("test.tmp") {
		t.Fatal("expected command to be unregistered")
	}

	_, err := registry.Execute(context.Background(), "test.tmp", HostCommandExecContext{}, nil)
	if err == nil {
		t.Fatal("expected error after unregister")
	}
}

func TestHostCommandRegistry_Get(t *testing.T) {
	registry := NewHostCommandRegistry()
	_ = registry.Register(HostCommandDefinition{
		CommandID:   "test.get",
		Description: "test description",
		Permission:  "ui.navigate",
		Scope:       HostCommandScopeGlobal,
		Risk:        host_api.RiskLow,
		Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
			return nil, nil
		},
	})

	def, ok := registry.Get("test.get")
	if !ok {
		t.Fatal("expected command to be found")
	}
	if def.Description != "test description" {
		t.Fatalf("expected description 'test description', got %q", def.Description)
	}
	if def.Permission != "ui.navigate" {
		t.Fatalf("expected permission 'ui.navigate', got %q", def.Permission)
	}
	if def.Scope != HostCommandScopeGlobal {
		t.Fatalf("expected scope %s, got %s", HostCommandScopeGlobal, def.Scope)
	}
	if def.Risk != host_api.RiskLow {
		t.Fatalf("expected risk %s, got %s", host_api.RiskLow, def.Risk)
	}

	_, ok = registry.Get("nonexistent")
	if ok {
		t.Fatal("expected command to not be found")
	}
}

func TestSetupDefaultHostCommands(t *testing.T) {
	registry := NewHostCommandRegistry()
	gateway := host_api.NewDefaultGateway()

	err := SetupDefaultHostCommands(registry, gateway)
	if err != nil {
		t.Fatalf("SetupDefaultHostCommands failed: %v", err)
	}

	expectedCmds := []struct {
		id         string
		permission string
		risk       host_api.RiskLevel
	}{
		{"app.open.settings", "ui.navigate", host_api.RiskLow},
		{"app.open.extension.detail", "ui.navigate", host_api.RiskLow},
		{"app.refresh.current_view", "ui.notify", host_api.RiskLow},
		{"app.show.notification_center", "ui.notify", host_api.RiskLow},
	}

	if registry.Count() != len(expectedCmds) {
		t.Fatalf("expected %d commands, got %d", len(expectedCmds), registry.Count())
	}

	for _, expected := range expectedCmds {
		def, ok := registry.Get(expected.id)
		if !ok {
			t.Fatalf("expected command %s to be registered", expected.id)
		}
		if def.Permission != expected.permission {
			t.Fatalf("command %s: expected permission %s, got %s", expected.id, expected.permission, def.Permission)
		}
		if def.Risk != expected.risk {
			t.Fatalf("command %s: expected risk %s, got %s", expected.id, expected.risk, def.Risk)
		}
		if def.Handler == nil {
			t.Fatalf("command %s: handler is nil", expected.id)
		}
		if len(def.InputSchema) == 0 {
			t.Fatalf("command %s: input schema is empty", expected.id)
		}
		if def.Description == "" {
			t.Fatalf("command %s: description is empty", expected.id)
		}
	}
}

func TestSetupDefaultHostCommands_NilArgs(t *testing.T) {
	err := SetupDefaultHostCommands(nil, host_api.NewDefaultGateway())
	if err == nil {
		t.Fatal("expected error for nil registry")
	}

	err = SetupDefaultHostCommands(NewHostCommandRegistry(), nil)
	if err == nil {
		t.Fatal("expected error for nil gateway")
	}
}

func TestHostCommandDoesNotEnterToolRegistry(t *testing.T) {
	hostCmdRegistry := NewHostCommandRegistry()
	toolRegistry := capability.NewToolRegistry()

	_ = hostCmdRegistry.Register(HostCommandDefinition{
		CommandID:  "app.open.settings",
		Permission: "ui.navigate",
		Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
			return nil, nil
		},
	})

	tools := toolRegistry.List(context.Background(), capability.ToolFilter{})
	if len(tools) != 0 {
		t.Fatalf("expected 0 tools in ToolRegistry, got %d", len(tools))
	}

	if hostCmdRegistry.Count() != 1 {
		t.Fatalf("expected 1 command in HostCommandRegistry, got %d", hostCmdRegistry.Count())
	}
}

func TestHostCommandError_TypeAssertion(t *testing.T) {
	directErr := NewHostCommandError(ErrCodeHostCommandNotFound, "not found", nil)

	var hcErr *HostCommandError
	if !AsHostCommandError(directErr, &hcErr) {
		t.Fatal("expected AsHostCommandError to return true for direct HostCommandError")
	}
	if hcErr.Code != ErrCodeHostCommandNotFound {
		t.Fatalf("expected code %s, got %s", ErrCodeHostCommandNotFound, hcErr.Code)
	}

	wrappedErr := errors.Join(directErr)
	if !AsHostCommandError(wrappedErr, &hcErr) {
		t.Fatal("expected AsHostCommandError to return true for wrapped HostCommandError")
	}

	plainErr := errors.New("some other error")
	if AsHostCommandError(plainErr, &hcErr) {
		t.Fatal("expected AsHostCommandError to return false for plain error")
	}
}

func TestHostCommandRegistry_InputInvalidError(t *testing.T) {
	registry := NewHostCommandRegistry()

	_ = registry.Register(HostCommandDefinition{
		CommandID:  "test.invalidinput",
		Permission: "ui.navigate",
		Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
			return nil, NewHostCommandError(ErrCodeHostCommandInputInvalid, "bad input", nil)
		},
	})

	_, err := registry.Execute(context.Background(), "test.invalidinput", HostCommandExecContext{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var hcErr *HostCommandError
	if !AsHostCommandError(err, &hcErr) {
		t.Fatalf("expected HostCommandError, got %T", err)
	}
	if hcErr.Code != ErrCodeHostCommandInputInvalid {
		t.Fatalf("expected code %s, got %s", ErrCodeHostCommandInputInvalid, hcErr.Code)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
