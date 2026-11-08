package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type RuntimeExecutionGuard func(context.Context, protocol.RuntimeInvokePayload, func(context.Context) (*protocol.RuntimeResultPayload, error)) (*protocol.RuntimeResultPayload, error)

const RemoteBrowserReadHandler = "workbench.browser.observe"

var trustedWorkflowControlHandlers = map[string]struct{}{
	"workflow.catalog":                    {},
	"workflow.tool_catalog":               {},
	"workflow.get":                        {},
	"workflow.upsert":                     {},
	"workflow.delete":                     {},
	"workflow.set_enabled":                {},
	"workflow.run":                        {},
	"workflow.run.start":                  {},
	"workflow.run.list":                   {},
	"workflow.run.get":                    {},
	"workflow.run.steps":                  {},
	"workflow.run.attempts":               {},
	"workflow.run.checkpoints":            {},
	"workflow.run.logs":                   {},
	"workflow.run.pause":                  {},
	"workflow.run.resume":                 {},
	"workflow.run.confirm":                {},
	"workflow.run.cancel":                 {},
	"workflow.run.recover":                {},
	"workflow.run.rerun":                  {},
	"workflow.sync.outbox":                {},
	"workflow.sync.ack":                   {},
	"workflow.sync.state":                 {},
	"workflow.trigger_capabilities":       {},
	"workflow.trigger_app_catalog":        {},
	"workflow.trigger_wake_configs":       {},
	"workflow.trigger_wake_config.create": {},
	"workflow.trigger_secret.create":      {},
	"workflow.android_runtime_health":     {},
}

func trustedUnscopedControlInvocation(invocation protocol.RuntimeInvokePayload) bool {
	if invocation.InvocationID == "" || invocation.AuthorityCallID != "" || len(invocation.OwnedExecutionScope) != 0 {
		return false
	}
	if invocation.Handler == RemoteBrowserReadHandler {
		return invocation.RuntimeType == "browser"
	}
	if invocation.RuntimeType != "workflow" {
		return false
	}
	_, ok := trustedWorkflowControlHandlers[invocation.Handler]
	return ok
}

func credentialMatchesInvocation(credential *StoredCredential, invocation protocol.RuntimeInvokePayload) bool {
	return credential != nil &&
		invocation.SpaceID == credential.SpaceID &&
		invocation.DeviceID == credential.DeviceID &&
		invocation.RuntimeID == credential.RuntimeID &&
		time.Now().Before(credential.ExpiresAt)
}

func NewOwnedToolGuard(db *sql.DB, dataDir string, roleGuards ...coordination.SourceRoleExecutionGuard) RuntimeExecutionGuard {
	credentials := NewCredentialStore(dataDir)
	return func(ctx context.Context, invocation protocol.RuntimeInvokePayload, run func(context.Context) (*protocol.RuntimeResultPayload, error)) (*protocol.RuntimeResultPayload, error) {
		if trustedUnscopedControlInvocation(invocation) {
			credential, err := credentials.LoadCredential()
			if err != nil || !credentialMatchesInvocation(credential, invocation) {
				return nil, coordination.ErrWrongOwner
			}
			result, executeErr := run(ctx)
			refreshed, credentialErr := credentials.LoadCredential()
			if credentialErr != nil || refreshed == nil || refreshed.CredentialID != credential.CredentialID || refreshed.Credential != credential.Credential || !credentialMatchesInvocation(refreshed, invocation) {
				return nil, coordination.ErrWrongOwner
			}
			return result, executeErr
		}
		var scope coordination.ExecutionScope
		if db == nil || invocation.AuthorityCallID == "" || len(invocation.AuthorityCallID) > 512 || len(invocation.OwnedExecutionScope) > 64<<10 || json.Unmarshal(invocation.OwnedExecutionScope, &scope) != nil {
			return nil, coordination.ErrWrongOwner
		}
		credential, err := credentials.LoadCredential()
		if err != nil {
			return nil, err
		}
		if credential == nil || invocation.SpaceID != credential.SpaceID || invocation.DeviceID != credential.DeviceID || invocation.RuntimeID != credential.RuntimeID || scope.SpaceID != invocation.SpaceID.String() || scope.CoreID != scope.SpaceID || scope.AuthorizationRealm != scope.CoreID || scope.TargetDeviceID != invocation.DeviceID.String() || scope.InitiatorDeviceID == "" || scope.RoleID == "" || scope.RequestID == "" {
			return nil, coordination.ErrWrongOwner
		}
		if scope.PermissionRevision < 1 || scope.TargetPermissionRevision < 1 || scope.ModeRevision < 1 || scope.ProviderEpoch < 1 || scope.TargetProviderEpoch < 1 || scope.RoleRevision < 1 || len(scope.RoleID) > 512 || len(scope.RequestID) > 512 || len(scope.InitiatorDeviceID) > 512 {
			return nil, coordination.ErrWrongOwner
		}
		owner := scope.TargetDeviceID
		if scope.Coordinated {
			owner = scope.CoreID
		}
		if scope.ResourceOwnerID != owner || scope.RoleOwnerID != owner {
			return nil, coordination.ErrWrongOwner
		}
		if invocation.RuntimeType == "task" {
			if scope.ExecutionID == "" || scope.TurnID == "" {
				return nil, coordination.ErrWrongOwner
			}
			return executeOwnedTaskGuard(ctx, db, credentials, credential, scope, invocation.AuthorityCallID, roleGuards, run)
		}
		var result *protocol.RuntimeResultPayload
		err = credentials.WithActiveCredential(ctx, credential, func(current context.Context) error {
			if err := coordination.ValidateSourceCall(current, db, scope.SpaceID, invocation.AuthorityCallID); err != nil {
				return err
			}
			if err := coordination.ValidateSourceAuthority(current, db, scope); err != nil {
				return err
			}
			execute := func() error {
				var runErr error
				guarded := coordination.WithAdditionalGuard(coordination.WithScope(current, scope), func(check context.Context) error {
					if err := coordination.ValidateSourceCall(check, db, scope.SpaceID, invocation.AuthorityCallID); err != nil {
						return err
					}
					return coordination.ValidateSourceAuthority(check, db, scope)
				})
				result, runErr = run(guarded)
				if runErr == nil && current.Err() != nil {
					return context.Cause(current)
				}
				return runErr
			}
			if !scope.Coordinated {
				if len(roleGuards) == 0 || roleGuards[0] == nil {
					return coordination.ErrRoleRequired
				}
				return roleGuards[0].WithSourceRole(current, scope, execute)
			}
			return execute()
		})
		return result, err
	}
}
