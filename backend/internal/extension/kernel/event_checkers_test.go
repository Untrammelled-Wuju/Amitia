package kernel

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type mockInstRepo struct {
	inst domain.ExtensionInstallation
	err  error
}

func (m *mockInstRepo) PutInstallation(_ context.Context, _ domain.ExtensionInstallation) error {
	return nil
}

func (m *mockInstRepo) GetInstallation(_ context.Context, _ domain.ExtensionID) (domain.ExtensionInstallation, error) {
	return m.inst, m.err
}

func (m *mockInstRepo) ListInstallations(_ context.Context) ([]domain.ExtensionInstallation, error) {
	return nil, nil
}

func (m *mockInstRepo) DeleteInstallation(_ context.Context, _ domain.ExtensionID) error {
	return nil
}

func TestEventGenerationResolverAdapter_NilRepo_Error(t *testing.T) {
	adapter := NewEventGenerationResolverAdapter(nil)
	_, err := adapter.CurrentGeneration(context.Background(), "ext-1")
	if err == nil {
		t.Fatalf("expected error when installation repository is nil")
	}
}

func TestEventGenerationResolverAdapter_GetInstallationError_Propagated(t *testing.T) {
	repo := &mockInstRepo{err: errors.New("db connection lost")}
	adapter := NewEventGenerationResolverAdapter(repo)
	_, err := adapter.CurrentGeneration(context.Background(), "ext-1")
	if err == nil {
		t.Fatalf("expected error when GetInstallation fails")
	}
}

func TestEventGenerationResolverAdapter_Success_ReturnsGeneration(t *testing.T) {
	repo := &mockInstRepo{inst: domain.ExtensionInstallation{Generation: 3}}
	adapter := NewEventGenerationResolverAdapter(repo)
	gen, err := adapter.CurrentGeneration(context.Background(), "ext-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gen != 3 {
		t.Fatalf("expected generation 3, got %d", gen)
	}
}
