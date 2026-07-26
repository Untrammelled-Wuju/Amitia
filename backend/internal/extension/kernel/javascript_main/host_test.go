package javascript_main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/runtime"
)

func makeTestHost(t *testing.T) *PluginHost {
	t.Helper()
	host, err := NewPluginHost(PluginHostConfig{
		InstanceID:        "inst-1",
		ExtensionID:       "com.example/weather",
		ModuleID:          "main",
		BootstrapSpec: runtime.BootstrapSpec{
			InstanceID:     "inst-1",
			ExtensionID:    "com.example/weather",
			ModuleID:       "main",
			Entry:          "modules/main/dist/index.js",
			HostAPIVersion: "1",
			SessionToken:   "token-1",
			ResourceLimits: runtime.DefaultResourceLimits(),
		},
		ProcessBoundary: runtime.DefaultProcessBoundary(),
		DefinitionHash:  "sha256:abc",
		HostAPIVersion:  "1",
		AllowedContributions: []AllowedContribution{
			{ContributionID: "get_weather", EntryType: "tool", EntryName: "get_weather"},
			{ContributionID: "filter_msg", EntryType: "hook", EntryName: "filter_msg"},
		},
	})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	return host
}

func TestPluginHostStart(t *testing.T) {
	host := makeTestHost(t)
	if host.State() != HostStateCreated {
		t.Fatalf("expected created, got %s", host.State())
	}
	result := host.Start(context.Background())
	if !result.Success {
		t.Fatalf("expected success, got %s: %s", result.Reason, result.Steps)
	}
	if host.State() != HostStateReady {
		t.Fatalf("expected ready, got %s", host.State())
	}
	if host.Session() == nil {
		t.Fatal("expected session")
	}
	if !host.Session().Ready {
		t.Fatal("expected session ready")
	}
}

func TestPluginHostStartRejectsMissingSessionToken(t *testing.T) {
	_, err := NewPluginHost(PluginHostConfig{
		InstanceID:     "inst-1",
		ExtensionID:    "com.example",
		ModuleID:       "main",
		BootstrapSpec:  runtime.BootstrapSpec{
			InstanceID:  "inst-1",
			Entry:       "entry.js",
		},
		DefinitionHash: "sha256:abc",
		HostAPIVersion: "1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestPluginHostStop(t *testing.T) {
	host := makeTestHost(t)
	host.Start(context.Background())
	if err := host.Stop(context.Background(), "test stop"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if host.State() != HostStateStopped {
		t.Fatalf("expected stopped, got %s", host.State())
	}
}

func TestPluginHostCannotStopFromCreated(t *testing.T) {
	host := makeTestHost(t)
	err := host.Stop(context.Background(), "test stop")
	if err == nil {
		t.Fatal("expected error stopping from created state")
	}
}

func TestPluginHostMarkCrashed(t *testing.T) {
	host := makeTestHost(t)
	host.Start(context.Background())
	host.MarkCrashed("panic in handler")
	if host.State() != HostStateCrashed {
		t.Fatalf("expected crashed, got %s", host.State())
	}
	if host.CrashCount() != 1 {
		t.Fatalf("expected crash count 1, got %d", host.CrashCount())
	}
	if host.LastError() != "panic in handler" {
		t.Fatalf("expected error message, got %s", host.LastError())
	}
}

func TestHandlerRegistryBindAllowed(t *testing.T) {
	registry := NewHandlerRegistry([]AllowedContribution{
		{ContributionID: "get_weather", EntryType: "tool", EntryName: "get_weather"},
	})
	err := registry.Bind(HandlerTypeTool, "get_weather", func(input interface{}, ctx InvocationContext) (interface{}, error) {
		return map[string]string{"status": "ok"}, nil
	})
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if !registry.IsBound(HandlerTypeTool, "get_weather") {
		t.Fatal("expected bound")
	}
	if registry.BoundCount() != 1 {
		t.Fatalf("expected bound count 1, got %d", registry.BoundCount())
	}
}

func TestHandlerRegistryRejectsUndeclared(t *testing.T) {
	registry := NewHandlerRegistry([]AllowedContribution{
		{ContributionID: "get_weather", EntryType: "tool", EntryName: "get_weather"},
	})
	err := registry.Bind(HandlerTypeTool, "undeclared_tool", func(input interface{}, ctx InvocationContext) (interface{}, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected entry_not_declared error")
	}
}

func TestHandlerRegistryRejectsDuplicate(t *testing.T) {
	registry := NewHandlerRegistry([]AllowedContribution{
		{ContributionID: "get_weather", EntryType: "tool", EntryName: "get_weather"},
	})
	registry.Bind(HandlerTypeTool, "get_weather", func(input interface{}, ctx InvocationContext) (interface{}, error) {
		return nil, nil
	})
	err := registry.Bind(HandlerTypeTool, "get_weather", func(input interface{}, ctx InvocationContext) (interface{}, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestHandlerRegistryVerifyCompleteness(t *testing.T) {
	registry := NewHandlerRegistry([]AllowedContribution{
		{ContributionID: "get_weather", EntryType: "tool", EntryName: "get_weather"},
		{ContributionID: "filter_msg", EntryType: "hook", EntryName: "filter_msg"},
	})
	registry.Bind(HandlerTypeTool, "get_weather", func(input interface{}, ctx InvocationContext) (interface{}, error) {
		return nil, nil
	})
	missing := registry.VerifyCompleteness()
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing, got %d", len(missing))
	}
}

func TestInvocationDispatcherSucceeded(t *testing.T) {
	dispatcher := NewInvocationDispatcher(runtime.DefaultResourceLimits())
	deadline := time.Now().Add(10 * time.Second)
	result := dispatcher.Dispatch(context.Background(), HandlerTypeTool, "test", "input", "inv-1", deadline, func(input interface{}, ctx InvocationContext) (interface{}, error) {
		return map[string]string{"result": "ok"}, nil
	})
	if result.Status != InvocationStatusSucceeded {
		t.Fatalf("expected succeeded, got %s: %s", result.Status, result.Error)
	}
}

func TestInvocationDispatcherFailed(t *testing.T) {
	dispatcher := NewInvocationDispatcher(runtime.DefaultResourceLimits())
	deadline := time.Now().Add(10 * time.Second)
	result := dispatcher.Dispatch(context.Background(), HandlerTypeTool, "test", "input", "inv-1", deadline, func(input interface{}, ctx InvocationContext) (interface{}, error) {
		return nil, errors.New("handler error")
	})
	if result.Status != InvocationStatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if result.Error != "handler error" {
		t.Fatalf("expected error message, got %s", result.Error)
	}
}

func TestInvocationDispatcherCancelled(t *testing.T) {
	dispatcher := NewInvocationDispatcher(runtime.DefaultResourceLimits())
	deadline := time.Now().Add(10 * time.Second)
	done := make(chan struct{})
	go func() {
		result := dispatcher.Dispatch(context.Background(), HandlerTypeTool, "test", "input", "inv-1", deadline, func(input interface{}, ctx InvocationContext) (interface{}, error) {
			select {
			case <-ctx.CancelSignal.Done():
				return nil, errors.New("cancelled")
			case <-time.After(5 * time.Second):
				return nil, nil
			}
		})
		if result.Status != InvocationStatusCancelled {
			t.Errorf("expected cancelled, got %s", result.Status)
		}
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	dispatcher.Cancel("inv-1", "user requested")
	<-done
}

func TestInvocationDispatcherTimedOut(t *testing.T) {
	dispatcher := NewInvocationDispatcher(runtime.DefaultResourceLimits())
	deadline := time.Now().Add(200 * time.Millisecond)
	result := dispatcher.Dispatch(context.Background(), HandlerTypeTool, "test", "input", "inv-1", deadline, func(input interface{}, ctx InvocationContext) (interface{}, error) {
		time.Sleep(2 * time.Second)
		return nil, nil
	})
	if result.Status != InvocationStatusTimedOut {
		t.Fatalf("expected timed_out, got %s", result.Status)
	}
}

func TestInvocationDispatcherRejectsAfterShutdown(t *testing.T) {
	dispatcher := NewInvocationDispatcher(runtime.DefaultResourceLimits())
	dispatcher.RejectNewInvocations()
	deadline := time.Now().Add(10 * time.Second)
	result := dispatcher.Dispatch(context.Background(), HandlerTypeTool, "test", "input", "inv-1", deadline, func(input interface{}, ctx InvocationContext) (interface{}, error) {
		return nil, nil
	})
	if result.Status != InvocationStatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestRuntimeFactoryCreate(t *testing.T) {
	factory := NewRuntimeFactory()
	host, err := factory.Create(context.Background(), CreateHostRequest{
		ExtensionID:    "com.example/weather",
		ModuleID:       "main",
		Entry:          "modules/main/dist/index.js",
		DefinitionHash: "sha256:abc",
		HostAPIVersion: "1",
		SessionToken:   "token-1",
		AllowedContributions: []AllowedContribution{
			{ContributionID: "get_weather", EntryType: "tool", EntryName: "get_weather"},
		},
		ResourceLimits: runtime.DefaultResourceLimits(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if host.State() != HostStateCreated {
		t.Fatalf("expected created, got %s", host.State())
	}
	host.Start(context.Background())
	if factory.ActiveCount() != 1 {
		t.Fatalf("expected 1 active, got %d", factory.ActiveCount())
	}
}

func TestRuntimeFactoryStopAll(t *testing.T) {
	factory := NewRuntimeFactory()
	host1, _ := factory.Create(context.Background(), CreateHostRequest{
		ExtensionID:    "com.example/ext1",
		ModuleID:       "main",
		Entry:          "entry.js",
		DefinitionHash: "h1",
		HostAPIVersion: "1",
		SessionToken:   "t1",
		ResourceLimits: runtime.DefaultResourceLimits(),
	})
	host2, _ := factory.Create(context.Background(), CreateHostRequest{
		ExtensionID:    "com.example/ext2",
		ModuleID:       "main",
		Entry:          "entry.js",
		DefinitionHash: "h2",
		HostAPIVersion: "1",
		SessionToken:   "t2",
		ResourceLimits: runtime.DefaultResourceLimits(),
	})
	host1.Start(context.Background())
	host2.Start(context.Background())
	errs := factory.StopAll(context.Background(), "shutdown")
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
	if host1.State() != HostStateStopped {
		t.Fatalf("expected host1 stopped, got %s", host1.State())
	}
	if host2.State() != HostStateStopped {
		t.Fatalf("expected host2 stopped, got %s", host2.State())
	}
}

func TestWatchdogReport(t *testing.T) {
	w := NewWatchdog("inst-1")
	report := w.Report()
	if !report.Healthy {
		t.Fatal("expected healthy by default")
	}
}

func TestHealthReport(t *testing.T) {
	host := makeTestHost(t)
	host.Start(context.Background())
	report := host.Health()
	if report.InstanceID != "inst-1" {
		t.Fatalf("expected inst-1, got %s", report.InstanceID)
	}
	if report.State != HostStateReady {
		t.Fatalf("expected ready, got %s", report.State)
	}
}
