package plugincontract

import (
	"context"
	"fmt"
)

type AdapterConformanceCase struct {
	Descriptor   AdapterDescriptor
	Module       ModuleRef
	Contribution ContributionDescriptor
}

type AdapterConformanceReport struct {
	Completed []string
	Issues    []string
}

func (r AdapterConformanceReport) Valid() bool {
	return len(r.Issues) == 0
}

func (r AdapterConformanceReport) Err() error {
	if r.Valid() {
		return nil
	}
	return fmt.Errorf("adapter conformance failed: %v", r.Issues)
}

func RunAdapterConformance(
	ctx context.Context,
	registry *AdapterRegistry,
	router *ContributionRouter,
	testCase AdapterConformanceCase,
) AdapterConformanceReport {
	var report AdapterConformanceReport
	fail := func(step string, err error) AdapterConformanceReport {
		report.Issues = append(report.Issues, fmt.Sprintf("%s: %v", step, err))
		return report
	}
	if registry == nil || router == nil {
		return fail("setup", fmt.Errorf("adapter registry and contribution router are required"))
	}
	if err := testCase.Descriptor.Validate(); err != nil {
		return fail("descriptor", err)
	}
	registered, ok := registry.Get(testCase.Descriptor.ID, testCase.Descriptor.ContractVersion)
	if !ok {
		return fail("registry", fmt.Errorf("adapter %s v%d is not registered", testCase.Descriptor.ID, testCase.Descriptor.ContractVersion))
	}
	if registered.ID != testCase.Descriptor.ID || registered.ContractVersion != testCase.Descriptor.ContractVersion {
		return fail("registry", fmt.Errorf("registered adapter identity does not match conformance descriptor"))
	}
	report.Completed = append(report.Completed, "registry")
	if testCase.Module.PluginID == "" || testCase.Module.ModuleID == "" {
		return fail("module", fmt.Errorf("module identity is required"))
	}
	if testCase.Contribution.Adapter != testCase.Descriptor.ID ||
		testCase.Contribution.ContractVersion != testCase.Descriptor.ContractVersion {
		return fail("contribution", fmt.Errorf("contribution adapter and contract version must match the descriptor"))
	}
	handler, err := router.Resolve(testCase.Contribution)
	if err != nil {
		return fail("route", err)
	}
	report.Completed = append(report.Completed, "route")
	registration, err := handler.Build(ctx, testCase.Module, testCase.Contribution)
	if err != nil {
		return fail("build", err)
	}
	if registration == nil {
		return fail("build", fmt.Errorf("handler returned nil registration"))
	}
	if registration.ID() == "" {
		return fail("build", fmt.Errorf("registration id is empty"))
	}
	report.Completed = append(report.Completed, "build")
	if err := handler.Activate(ctx, registration); err != nil {
		_ = handler.Discard(ctx, registration)
		return fail("activate", err)
	}
	report.Completed = append(report.Completed, "activate")
	if err := handler.Deactivate(ctx, registration); err != nil {
		_ = handler.Discard(ctx, registration)
		return fail("deactivate", err)
	}
	report.Completed = append(report.Completed, "deactivate")
	if err := handler.Discard(ctx, registration); err != nil {
		return fail("discard", err)
	}
	report.Completed = append(report.Completed, "discard")
	return report
}
