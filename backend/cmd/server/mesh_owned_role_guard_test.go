package main

import (
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestDeviceToolRoleChangesWaitForExecutingSnapshot(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3}
	entered, release := make(chan struct{}), make(chan struct{})
	executed := make(chan error, 1)
	go func() {
		executed <- p.WithSourceRole(t.Context(), scope, func() error { close(entered); <-release; return nil })
	}()
	<-entered
	changed := make(chan error, 1)
	go func() {
		unlock := character.LockRoleSource(p.services.DB)
		defer unlock()
		changed <- p.services.DB.Model(&character.Character{}).Where("id=?", "one").Update("revision", 4).Error
	}()
	select {
	case err := <-changed:
		close(release)
		t.Fatalf("role changed during tool execution: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-executed; err != nil {
		t.Fatal(err)
	}
	if err := <-changed; err != nil {
		t.Fatal(err)
	}
	if err := p.WithSourceRole(t.Context(), scope, func() error { t.Fatal("obsolete role executed"); return nil }); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("obsolete role accepted: %v", err)
	}
	scope.RoleID = "missing"
	if err := p.WithSourceRole(t.Context(), scope, func() error { t.Fatal("missing role executed"); return nil }); !errors.Is(err, coordination.ErrRoleRequired) {
		t.Fatalf("missing role accepted: %v", err)
	}
}
