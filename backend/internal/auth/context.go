// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type PrincipalType string

const (
	PrincipalLocalUI       PrincipalType = "local_ui"
	PrincipalTrustedDevice PrincipalType = "trusted_device"
	PrincipalDeviceRuntime PrincipalType = "device_runtime"
	PrincipalSystemWorker  PrincipalType = "system_worker"
	PrincipalAutomation    PrincipalType = "automation"
	PrincipalExtension     PrincipalType = "extension"
	PrincipalMigration     PrincipalType = "migration"
	PrincipalRepair        PrincipalType = "repair"
	PrincipalTest          PrincipalType = "test"
)

type ActorContext struct {
	PrincipalType    PrincipalType
	SpaceID          runtimeidentity.SpaceID
	DeviceID         runtimeidentity.DeviceID
	RuntimeID        runtimeidentity.RuntimeID
	Capabilities     []string
	Permissions      []string
	AuthMethod       string
	SessionID        string
	RuntimeSessionID runtimeidentity.RuntimeSessionID
	CorrelationID    string
	RequestID        string
	IsLocalTrusted   bool
}

type contextKey struct{}

var actorContextKey = contextKey{}

func WithActor(ctx context.Context, actor *ActorContext) context.Context {
	return context.WithValue(ctx, actorContextKey, actor)
}
func FromContext(ctx context.Context) (*ActorContext, bool) {
	actor, ok := ctx.Value(actorContextKey).(*ActorContext)
	return actor, ok
}
func RequireActor(ctx context.Context) (*ActorContext, error) {
	actor, ok := FromContext(ctx)
	if !ok || actor == nil {
		return nil, errors.New("unauthorized: missing actor context")
	}
	return actor, nil
}
func (a *ActorContext) HasPermission(perm string) bool {
	for _, p := range a.Permissions {
		if p == perm || p == "*" {
			return true
		}
	}
	return false
}
func (a *ActorContext) HasCapability(cap string) bool {
	for _, c := range a.Capabilities {
		if c == cap || c == "*" {
			return true
		}
	}
	return false
}
func (a *ActorContext) IsSystemActor() bool {
	return a != nil && (a.PrincipalType == PrincipalSystemWorker || a.PrincipalType == PrincipalMigration || a.PrincipalType == PrincipalRepair)
}
func (a *ActorContext) Clone() *ActorContext {
	if a == nil {
		return nil
	}
	c := *a
	c.Capabilities = append([]string(nil), a.Capabilities...)
	c.Permissions = append([]string(nil), a.Permissions...)
	return &c
}
func (a *ActorContext) RuntimeIdentity() runtimeidentity.Identity {
	if a == nil {
		return runtimeidentity.Identity{}
	}
	return runtimeidentity.Identity{SpaceID: a.SpaceID, DeviceID: a.DeviceID, RuntimeID: a.RuntimeID, RuntimeSessionID: a.RuntimeSessionID}
}
