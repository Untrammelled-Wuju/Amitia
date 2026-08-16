package integration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/installation"
)

type fakeResourcePort struct {
	mu       sync.RWMutex
	bindings map[string]ExistingPetResourceBinding
	listErr  error
}

func newFakeResourcePort() *fakeResourcePort {
	return &fakeResourcePort{bindings: make(map[string]ExistingPetResourceBinding)}
}

func (f *fakeResourcePort) AttachPluginResource(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	handle := extensionID + "/" + contributionID
	f.mu.Lock()
	defer f.mu.Unlock()
	if existing, ok := f.bindings[handle]; ok && existing.Revision >= revision {
		return existing.Handle, nil
	}
	f.bindings[handle] = ExistingPetResourceBinding{
		Handle:         handle,
		ExtensionID:    extensionID,
		ContributionID: contributionID,
		Revision:       revision,
		Definition:     definition,
	}
	return handle, nil
}

func (f *fakeResourcePort) DetachPluginResource(ctx context.Context, handle string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.bindings[handle]; !ok {
		return fmt.Errorf("not found: %s", handle)
	}
	delete(f.bindings, handle)
	return nil
}

func (f *fakeResourcePort) ListAttachedResources(ctx context.Context, extensionID string) ([]ExistingPetResourceBinding, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []ExistingPetResourceBinding
	for _, b := range f.bindings {
		if b.ExtensionID == extensionID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (f *fakeResourcePort) RebuildFromExisting() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings = make(map[string]ExistingPetResourceBinding)
	return nil
}

type fakeRepo struct {
	installation.Repository
}

func TestProductionResourceAttach_CallsExistingOwner(t *testing.T) {
	port := newFakeResourcePort()
	repo := &fakeRepo{}
	cap := NewProductionResourceCapability(repo, port)

	req := PluginResourceAttachRequest{
		ExtensionID:    "com.example/pet",
		PluginID:       "pet.plugin",
		ContributionID: "wave_asset",
		Revision:       1,
		Definition:     map[string]any{"assetKind": "image"},
	}

	handle, err := cap.AttachPluginResource(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle == "" {
		t.Fatal("expected non-empty handle")
	}

	port.mu.RLock()
	count := len(port.bindings)
	port.mu.RUnlock()
	if count != 1 {
		t.Errorf("expected 1 binding in Existing owner, got %d", count)
	}
}

func TestProductionResourceAttach_NilPort_Fails(t *testing.T) {
	repo := &fakeRepo{}
	cap := NewProductionResourceCapability(repo, nil)

	req := PluginResourceAttachRequest{
		ExtensionID:    "com.example/pet",
		ContributionID: "wave_asset",
	}

	_, err := cap.AttachPluginResource(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when port is nil")
	}
	if !strings.Contains(err.Error(), "Existing pet resource port unavailable") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProductionResourceDetach_CallsExistingOwner(t *testing.T) {
	port := newFakeResourcePort()
	repo := &fakeRepo{}
	cap := NewProductionResourceCapability(repo, port)

	req := PluginResourceAttachRequest{
		ExtensionID:    "com.example/pet",
		ContributionID: "wave_asset",
		Revision:       1,
	}

	handle, err := cap.AttachPluginResource(context.Background(), req)
	if err != nil {
		t.Fatalf("attach error: %v", err)
	}

	err = cap.DetachPluginResource(context.Background(), handle)
	if err != nil {
		t.Fatalf("detach error: %v", err)
	}

	port.mu.RLock()
	count := len(port.bindings)
	port.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 bindings after detach, got %d", count)
	}
}

func TestProductionResourceCache_IsNotSourceOfTruth(t *testing.T) {
	port := newFakeResourcePort()
	repo := &fakeRepo{}
	cap := NewProductionResourceCapability(repo, port).(*productionResourceCapability)

	req := PluginResourceAttachRequest{
		ExtensionID:    "com.example/pet",
		ContributionID: "wave_asset",
		Revision:       1,
	}

	_, err := cap.AttachPluginResource(context.Background(), req)
	if err != nil {
		t.Fatalf("attach error: %v", err)
	}

	port.mu.Lock()
	delete(port.bindings, "com.example/pet/wave_asset")
	port.mu.Unlock()

	err = cap.RebuildFromExisting()
	if err != nil {
		t.Fatalf("rebuild error: %v", err)
	}
}

func TestProductionFloatingWindow_NilPublisher_FailsFast(t *testing.T) {
	cap := NewProductionFloatingWindowCapability(nil)

	req := PluginFloatingWindowAttachRequest{
		ExtensionID:    "com.example/pet",
		ContributionID: "main_window",
	}

	_, err := cap.AttachPluginFloatingWindow(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when publisher is nil")
	}
	if !strings.Contains(err.Error(), "composition error") {
		t.Errorf("expected composition error, got: %v", err)
	}
}

func TestTransactionalAttach_Rollback_OnStepFailure(t *testing.T) {
	resPort := newFakeResourcePort()
	actionPort := &fakeActionPort{failOnAttach: true}
	rtPort := &fakeRuntimePort{}
	winPort := &fakeWindowPort{}

	caps := DesktopPetPluginCapabilities{
		Resource:       NewProductionResourceCapability(&fakeRepo{}, resPort),
		Action:         NewProductionActionCapability(actionPort, &productionActionTargetResolver{}),
		Runtime:        NewProductionRuntimeCapability(rtPort),
		FloatingWindow: NewProductionFloatingWindowCapability(winPort),
	}

	req := AttachTransactionRequest{
		ExtensionID:    "com.example/pet",
		PluginID:       "pet.plugin",
		ContributionID: "wave_full",
		Revision:       1,
		Target:         ExistingPetActionTarget{InstallationID: "inst_1"},
		Definition:     map[string]any{"actionKey": "wave"},
	}

	_, err := caps.TransactionalAttach(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when action fails")
	}

	resPort.mu.RLock()
	resCount := len(resPort.bindings)
	resPort.mu.RUnlock()
	if resCount != 0 {
		t.Errorf("expected 0 resource bindings after rollback, got %d", resCount)
	}
}

type fakeActionPort struct {
	mu           sync.RWMutex
	bindings     map[string]ExistingPetActionBinding
	failOnAttach bool
	failOnDetach bool
}

func newFakeActionPort() *fakeActionPort {
	return &fakeActionPort{bindings: make(map[string]ExistingPetActionBinding)}
}

func (f *fakeActionPort) AttachPluginAction(ctx context.Context, extensionID, contributionID string, revision int, target ExistingPetActionTarget, definition map[string]any) (string, error) {
	if f.failOnAttach {
		return "", fmt.Errorf("simulated action attach failure")
	}
	handle := extensionID + "/" + contributionID
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings[handle] = ExistingPetActionBinding{
		Handle:         handle,
		ExtensionID:    extensionID,
		ContributionID: contributionID,
		Revision:       revision,
		Target:         target,
	}
	return handle, nil
}

func (f *fakeActionPort) DetachPluginAction(ctx context.Context, handle string) error {
	if f.failOnDetach {
		return fmt.Errorf("simulated action detach failure")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bindings, handle)
	return nil
}

func (f *fakeActionPort) ListAttachedActions(ctx context.Context, extensionID string) ([]ExistingPetActionBinding, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []ExistingPetActionBinding
	for _, b := range f.bindings {
		if b.ExtensionID == extensionID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (f *fakeActionPort) RebuildFromExisting() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings = make(map[string]ExistingPetActionBinding)
	return nil
}

type fakeRuntimePort struct {
	mu       sync.RWMutex
	bindings map[string]ExistingPetRuntimeBinding
}

func newFakeRuntimePort() *fakeRuntimePort {
	return &fakeRuntimePort{bindings: make(map[string]ExistingPetRuntimeBinding)}
}

func (f *fakeRuntimePort) AttachPluginRuntime(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	handle := extensionID + "/" + contributionID
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings[handle] = ExistingPetRuntimeBinding{
		Handle:         handle,
		ExtensionID:    extensionID,
		ContributionID: contributionID,
		Revision:       revision,
	}
	return handle, nil
}

func (f *fakeRuntimePort) DetachPluginRuntime(ctx context.Context, handle string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bindings, handle)
	return nil
}

func (f *fakeRuntimePort) ListAttachedRuntimes(ctx context.Context, extensionID string) ([]ExistingPetRuntimeBinding, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []ExistingPetRuntimeBinding
	for _, b := range f.bindings {
		if b.ExtensionID == extensionID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (f *fakeRuntimePort) RebuildFromExisting() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings = make(map[string]ExistingPetRuntimeBinding)
	return nil
}

type fakeWindowPort struct {
	mu       sync.RWMutex
	bindings map[string]ExistingPetWindowBinding
}

func newFakeWindowPort() *fakeWindowPort {
	return &fakeWindowPort{bindings: make(map[string]ExistingPetWindowBinding)}
}

func (f *fakeWindowPort) PublishFloatingWindowContribution(ctx context.Context, extensionID, contributionID string, definition map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings[extensionID+"/"+contributionID] = ExistingPetWindowBinding{
		ExtensionID:    extensionID,
		ContributionID: contributionID,
	}
	return nil
}

func (f *fakeWindowPort) RetractFloatingWindowContribution(ctx context.Context, extensionID, contributionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bindings, extensionID+"/"+contributionID)
	return nil
}

func (f *fakeWindowPort) ListAttachedWindows(ctx context.Context, extensionID string) ([]ExistingPetWindowBinding, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []ExistingPetWindowBinding
	for _, b := range f.bindings {
		if b.ExtensionID == extensionID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (f *fakeWindowPort) RebuildFromExisting() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings = make(map[string]ExistingPetWindowBinding)
	return nil
}

func TestTransactionalAttach_Success_AllSteps(t *testing.T) {
	resPort := newFakeResourcePort()
	actionPort := newFakeActionPort()
	rtPort := newFakeRuntimePort()
	winPort := newFakeWindowPort()

	caps := DesktopPetPluginCapabilities{
		Resource:       NewProductionResourceCapability(&fakeRepo{}, resPort),
		Action:         NewProductionActionCapability(actionPort, &productionActionTargetResolver{}),
		Runtime:        NewProductionRuntimeCapability(rtPort),
		FloatingWindow: NewProductionFloatingWindowCapability(winPort),
	}

	req := AttachTransactionRequest{
		ExtensionID:    "com.example/pet",
		PluginID:       "pet.plugin",
		ContributionID: "wave_full",
		Revision:       1,
		Target:         ExistingPetActionTarget{InstallationID: "inst_1"},
		Definition:     map[string]any{"actionKey": "wave"},
	}

	result, err := caps.TransactionalAttach(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResourceHandle == "" {
		t.Error("expected resource handle")
	}
	if result.ActionHandle == "" {
		t.Error("expected action handle")
	}
	if result.RuntimeHandle == "" {
		t.Error("expected runtime handle")
	}
	if result.FloatingWindowHandle == "" {
		t.Error("expected floating window handle")
	}
}

func TestTransactionalDetach_ReverseOrder(t *testing.T) {
	resPort := newFakeResourcePort()
	actionPort := newFakeActionPort()
	rtPort := newFakeRuntimePort()
	winPort := newFakeWindowPort()

	caps := DesktopPetPluginCapabilities{
		Resource:       NewProductionResourceCapability(&fakeRepo{}, resPort),
		Action:         NewProductionActionCapability(actionPort, &productionActionTargetResolver{}),
		Runtime:        NewProductionRuntimeCapability(rtPort),
		FloatingWindow: NewProductionFloatingWindowCapability(winPort),
	}

	attachReq := AttachTransactionRequest{
		ExtensionID:    "com.example/pet",
		PluginID:       "pet.plugin",
		ContributionID: "wave_full",
		Revision:       1,
		Target:         ExistingPetActionTarget{InstallationID: "inst_1"},
		Definition:     map[string]any{"actionKey": "wave"},
	}

	result, err := caps.TransactionalAttach(context.Background(), attachReq)
	if err != nil {
		t.Fatalf("attach error: %v", err)
	}

	detachReq := DetachTransactionRequest{
		ExtensionID:    "com.example/pet",
		ContributionID: "wave_full",
		ResourceHandle: result.ResourceHandle,
		ActionHandle:   result.ActionHandle,
		RuntimeHandle:  result.RuntimeHandle,
		WindowHandle:   result.FloatingWindowHandle,
	}

	err = caps.TransactionalDetach(context.Background(), detachReq)
	if err != nil {
		t.Fatalf("detach error: %v", err)
	}

	winPort.mu.RLock()
	winCount := len(winPort.bindings)
	winPort.mu.RUnlock()
	rtPort.mu.RLock()
	rtCount := len(rtPort.bindings)
	rtPort.mu.RUnlock()
	actionPort.mu.RLock()
	actionCount := len(actionPort.bindings)
	actionPort.mu.RUnlock()
	resPort.mu.RLock()
	resCount := len(resPort.bindings)
	resPort.mu.RUnlock()

	if winCount != 0 {
		t.Errorf("expected 0 window bindings, got %d", winCount)
	}
	if rtCount != 0 {
		t.Errorf("expected 0 runtime bindings, got %d", rtCount)
	}
	if actionCount != 0 {
		t.Errorf("expected 0 action bindings, got %d", actionCount)
	}
	if resCount != 0 {
		t.Errorf("expected 0 resource bindings, got %d", resCount)
	}
}
