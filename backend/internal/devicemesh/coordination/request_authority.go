package coordination

import (
	"context"
	"database/sql"
	"errors"
)

func ValidateRequestAuthorityTx(ctx context.Context, tx *sql.Tx) error {
	requests, _ := ctx.Value(requestAuthoritiesKey{}).([]context.Context)
	for _, request := range requests {
		if err := context.Cause(request); err != nil {
			return err
		}
		scope, ok := FromContext(request)
		if !ok {
			return ErrScopeExpired
		}
		var provider string
		if err := tx.QueryRowContext(ctx, `SELECT core_id FROM kernel_device_provider_binding WHERE id=1`).Scan(&provider); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if provider != "" && provider != scope.CoreID {
			return ErrScopeExpired
		}
		for _, device := range []string{scope.InitiatorDeviceID, scope.TargetDeviceID} {
			var trusted string
			if err := tx.QueryRowContext(ctx, `SELECT trust_state FROM kernel_devices WHERE space_id=? AND device_id=?`, scope.SpaceID, device).Scan(&trusted); err != nil {
				return err
			}
			if trusted != "trusted" {
				return ErrScopeExpired
			}
			policy := Policy{ModeRevision: 1, PermissionRevision: 1, ProviderEpoch: 1}
			err := tx.QueryRowContext(ctx, `SELECT coordinated,mode_revision,permission_revision,provider_epoch FROM kernel_device_coordination WHERE space_id=? AND device_id=?`, scope.SpaceID, device).Scan(&policy.Coordinated, &policy.ModeRevision, &policy.PermissionRevision, &policy.ProviderEpoch)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if device == scope.InitiatorDeviceID && (policy.PermissionRevision != scope.PermissionRevision || policy.ProviderEpoch != scope.ProviderEpoch) {
				return ErrScopeExpired
			}
			if device == scope.TargetDeviceID && (policy.ModeRevision != scope.ModeRevision || policy.Coordinated != scope.Coordinated || policy.PermissionRevision != scope.TargetPermissionRevision || policy.ProviderEpoch != scope.TargetProviderEpoch) {
				return ErrScopeExpired
			}
		}
	}
	return nil
}

type requestAuthoritiesKey struct{}

func CommitRequestCurrent(ctx context.Context, commit func() error) error {
	if _, _, readOnly := TaskReadAuthority(ctx); readOnly {
		return ErrWrongOwner
	}
	requests, _ := ctx.Value(requestAuthoritiesKey{}).([]context.Context)
	if len(requests) == 0 {
		return commit()
	}
	return CommitCurrent(ctx, commit)
}

func WithRequestAuthority(ctx, request context.Context) (context.Context, error) {
	if ctx == nil || request == nil {
		return ctx, ErrScopeExpired
	}
	service, ok := request.Value(authorityServiceKey{}).(*Service)
	if !ok || service == nil {
		return ctx, ErrScopeExpired
	}
	if err := ValidateCurrent(request); err != nil {
		return ctx, err
	}
	if current, ok := ctx.Value(authorityServiceKey{}).(*Service); ok && current != service {
		return ctx, ErrWrongOwner
	}
	previous, _ := ctx.Value(requestAuthoritiesKey{}).([]context.Context)
	if len(previous) >= 8 {
		return ctx, ErrPendingLimit
	}
	requests := append(append([]context.Context(nil), previous...), request)
	guarded := context.WithValue(ctx, requestAuthoritiesKey{}, requests)
	if _, ok := ctx.Value(commitKey{}).(func(context.Context, func() error) error); !ok {
		guarded = context.WithValue(guarded, commitKey{}, request.Value(commitKey{}))
	}
	return WithAdditionalGuard(guarded, func(context.Context) error { return ValidateCurrent(request) }), nil
}

func (s *Service) validateRequestAuthorities(ctx context.Context) error {
	requests, _ := ctx.Value(requestAuthoritiesKey{}).([]context.Context)
	for _, request := range requests {
		service, ok := request.Value(authorityServiceKey{}).(*Service)
		scope, owned := FromContext(request)
		if !ok || service != s || !owned {
			return ErrWrongOwner
		}
		if err := s.Validate(request, scope); err != nil {
			return err
		}
	}
	return nil
}
