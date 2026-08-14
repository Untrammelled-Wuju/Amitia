// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package auth

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ActorType string

const (
	ActorTypeUser          ActorType = "user"
	ActorTypeLocalUser     ActorType = "local_user"
	ActorTypeLocalAdmin    ActorType = "local_admin"
	ActorTypeAdmin         ActorType = "admin"
	ActorTypeSystemWorker  ActorType = "system_worker"
	ActorTypeRuntimeClient ActorType = "runtime_client"
	ActorTypeMigration     ActorType = "migration"
	ActorTypeRepair        ActorType = "repair"
	ActorTypeTest          ActorType = "test"
)

type ActorContext struct {
	ActorType        ActorType
	UserID           runtimeidentity.UserID
	DeviceID         runtimeidentity.DeviceID
	RuntimeID        runtimeidentity.RuntimeID
	Roles            []string
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
		if p == perm {
			return true
		}
	}
	return false
}

func (a *ActorContext) HasRole(role string) bool {
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (a *ActorContext) IsSystemActor() bool {
	switch a.ActorType {
	case ActorTypeSystemWorker, ActorTypeMigration, ActorTypeRepair:
		return true
	}
	return false
}

func (a *ActorContext) Clone() *ActorContext {
	if a == nil {
		return nil
	}
	clone := *a
	if a.Roles != nil {
		clone.Roles = append([]string(nil), a.Roles...)
	}
	if a.Permissions != nil {
		clone.Permissions = append([]string(nil), a.Permissions...)
	}
	return &clone
}

func (a *ActorContext) RuntimeIdentity() runtimeidentity.Identity {
	if a == nil {
		return runtimeidentity.Identity{}
	}
	return runtimeidentity.Identity{
		UserID:           a.UserID,
		DeviceID:         a.DeviceID,
		RuntimeID:        a.RuntimeID,
		RuntimeSessionID: a.RuntimeSessionID,
	}
}
