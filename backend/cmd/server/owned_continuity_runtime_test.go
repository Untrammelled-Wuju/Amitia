package main

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func TestContinuityResolveDistinguishesMissingRoleFromUnreadyService(t *testing.T) {
	unready := &ownedContinuityRuntime{}
	if _, err := unready.Resolve(t.Context(), interaction.EntryOwner{}, "role", "", "", "hello", "", "", "request"); err == nil || errors.Is(err, coordination.ErrRoleRequired) {
		t.Fatalf("unready runtime error=%v", err)
	}
	ready := &ownedContinuityRuntime{engine: &business.Engine{}}
	_, err := ready.Resolve(t.Context(), interaction.EntryOwner{}, "", "", "", "hello", "", "", "request")
	if !errors.Is(err, coordination.ErrRoleRequired) {
		t.Fatalf("missing role error=%v", err)
	}
	if _, err := ready.Resolve(t.Context(), interaction.EntryOwner{}, "role", "", "", "", "", "", "request"); err != nil {
		t.Fatalf("empty message rejected: %v", err)
	}
}
