package coordination

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

const PolicySchema = `CREATE TABLE IF NOT EXISTS kernel_device_coordination (
	space_id TEXT NOT NULL, device_id TEXT NOT NULL,
	coordinated INTEGER NOT NULL DEFAULT 0, mode_revision INTEGER NOT NULL DEFAULT 1,
	administrator INTEGER NOT NULL DEFAULT 0, permission_revision INTEGER NOT NULL DEFAULT 1,
	selected_role TEXT NOT NULL DEFAULT '', provider_epoch INTEGER NOT NULL DEFAULT 1,
	PRIMARY KEY(space_id, device_id))`

var ErrRevision = errors.New("设备状态已变化，请刷新后重试")
var ErrAdministratorMode = errors.New("管理员设备必须先开启统筹模式")
var ErrScopeExpired = errors.New("服务提供者、统筹模式或权限已变化，当前回复已中断")
var ErrRoleRequired = errors.New("没有可用角色，拒绝调用")
var ErrRoleSelection = errors.New("存在多个角色，请先指定调用角色")

type Policy struct {
	DeviceID           string `json:"deviceId"`
	Coordinated        bool   `json:"coordinated"`
	ModeRevision       int64  `json:"modeRevision"`
	Administrator      bool   `json:"administrator"`
	PermissionRevision int64  `json:"permissionRevision"`
	SelectedRole       string `json:"selectedRole"`
	ProviderEpoch      int64  `json:"providerEpoch"`
}

type ExecutionScope struct {
	SpaceID                  string `json:"spaceId"`
	InitiatorDeviceID        string `json:"initiatorDeviceId"`
	TargetDeviceID           string `json:"targetDeviceId"`
	CoreID                   string `json:"coreId"`
	ProviderEpoch            int64  `json:"providerEpoch"`
	Coordinated              bool   `json:"coordinated"`
	ModeRevision             int64  `json:"modeRevision"`
	PermissionRevision       int64  `json:"permissionRevision"`
	RoleID                   string `json:"roleId"`
	TargetPermissionRevision int64  `json:"targetPermissionRevision,omitempty"`
	TargetProviderEpoch      int64  `json:"targetProviderEpoch,omitempty"`
	RoleRevision             int64  `json:"roleRevision,omitempty"`
	AuthorizationRealm       string `json:"authorizationRealm,omitempty"`
	TurnID                   string `json:"turnId,omitempty"`
	ExecutionID              string `json:"executionId,omitempty"`
	RoleOwnerID              string `json:"roleOwnerId"`
	ResourceOwnerID          string `json:"resourceOwnerId"`
	RequestID                string `json:"requestId"`
}

type scopeKey struct{}
type authorityServiceKey struct{}
type activeExecutionKey struct{}
type managementExecutionKey struct{}

type activeExecution struct {
	service *Service
	id      uint64
}
type guardKey struct{}
type commitKey struct{}
type additionalGuardKey struct{}
type commitDependenciesKey struct{}

func WithoutAdditionalGuard(ctx context.Context) context.Context {
	return context.WithValue(ctx, additionalGuardKey{}, func(context.Context) error { return nil })
}

func WithCommitDependencies(ctx context.Context, dependencies func(context.Context) ([]ResourceVersion, error)) context.Context {
	return context.WithValue(ctx, commitDependenciesKey{}, dependencies)
}

func CommitDependencies(ctx context.Context) ([]ResourceVersion, error) {
	if dependencies, ok := ctx.Value(commitDependenciesKey{}).(func(context.Context) ([]ResourceVersion, error)); ok {
		return dependencies(ctx)
	}
	return nil, nil
}

func WithAdditionalGuard(ctx context.Context, guard func(context.Context) error) context.Context {
	previous, _ := ctx.Value(additionalGuardKey{}).(func(context.Context) error)
	return context.WithValue(ctx, additionalGuardKey{}, func(current context.Context) error {
		if previous != nil {
			if err := previous(current); err != nil {
				return err
			}
		}
		return guard(current)
	})
}

func WithScope(ctx context.Context, scope ExecutionScope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func FromContext(ctx context.Context) (ExecutionScope, bool) {
	scope, ok := ctx.Value(scopeKey{}).(ExecutionScope)
	return scope, ok
}

func ValidateCurrent(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if guard, ok := ctx.Value(additionalGuardKey{}).(func(context.Context) error); ok {
		if err := guard(ctx); err != nil {
			return err
		}
	}
	guard, ok := ctx.Value(guardKey{}).(func(context.Context) error)
	if !ok {
		return nil
	}
	return guard(ctx)
}

func CommitCurrent(ctx context.Context, commit func() error) error {
	if _, _, readOnly := TaskReadAuthority(ctx); readOnly {
		return ErrWrongOwner
	}
	if guard, ok := ctx.Value(additionalGuardKey{}).(func(context.Context) error); ok {
		if err := guard(ctx); err != nil {
			return err
		}
	}
	if guarded, ok := ctx.Value(commitKey{}).(func(context.Context, func() error) error); ok {
		return guarded(ctx, commit)
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return commit()
}

type Service struct {
	db               *sql.DB
	mu               sync.Mutex
	active           map[string]map[uint64]context.CancelCauseFunc
	next             uint64
	authorityBarrier AuthorityBarrier
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, active: make(map[string]map[uint64]context.CancelCauseFunc)}
}

func (s *Service) InitializeCoreConsole(ctx context.Context, space, device string) error {
	if space == "" || device == "" {
		return ErrWrongOwner
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_coordination(space_id,device_id,coordinated,administrator) VALUES(?,?,1,1) ON CONFLICT DO NOTHING`, space, device)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count > 0 {
		granted := true
		if err := meshaudit.QueueTx(ctx, tx, space, device, "device_mesh.core_console_initialized", meshaudit.Details{Coordinated: &granted, Administrator: &granted, ModeRevision: 1, PermissionRevision: 1, ProviderEpoch: 1}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) Get(ctx context.Context, space, device string) (Policy, error) {
	p := Policy{DeviceID: device, ModeRevision: 1, PermissionRevision: 1, ProviderEpoch: 1}
	err := s.db.QueryRowContext(ctx, `SELECT coordinated, mode_revision, administrator, permission_revision, selected_role, provider_epoch FROM kernel_device_coordination WHERE space_id=? AND device_id=?`, space, device).Scan(&p.Coordinated, &p.ModeRevision, &p.Administrator, &p.PermissionRevision, &p.SelectedRole, &p.ProviderEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	return p, err
}

func (s *Service) ChangeMode(ctx context.Context, space, device string, expected int64, coordinated bool, role string) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validateRequestAuthorities(ctx); err != nil {
		return Policy{}, err
	}
	previous, err := s.Get(ctx, space, device)
	if err != nil {
		return Policy{}, err
	}
	if previous.ModeRevision != expected {
		return Policy{}, ErrRevision
	}
	if err := s.fenceAuthorityLocked(ctx, space, device, previous.PermissionRevision); err != nil {
		return Policy{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Policy{}, err
	}
	defer tx.Rollback()
	if err := ValidateRequestAuthorityTx(ctx, tx); err != nil {
		return Policy{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO kernel_device_coordination(space_id,device_id) VALUES(?,?) ON CONFLICT DO NOTHING`, space, device); err != nil {
		return Policy{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE kernel_device_coordination SET coordinated=?, selected_role=?, mode_revision=mode_revision+1, administrator=CASE WHEN ? THEN administrator ELSE 0 END, permission_revision=permission_revision+1 WHERE space_id=? AND device_id=? AND mode_revision=?`, coordinated, role, coordinated, space, device, expected)
	if err != nil {
		return Policy{}, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return Policy{}, ErrRevision
	}
	administrator := previous.Administrator && coordinated
	if err := meshaudit.QueueTx(ctx, tx, space, device, "device_mesh.mode_changed", meshaudit.Details{ModeRevision: expected + 1, PermissionRevision: previous.PermissionRevision + 1, ProviderEpoch: previous.ProviderEpoch, Coordinated: &coordinated, Administrator: &administrator}); err != nil {
		return Policy{}, err
	}
	if err = tx.Commit(); err != nil {
		return Policy{}, err
	}
	s.cancelLocked(space, device)
	previous.Coordinated = coordinated
	previous.SelectedRole = role
	previous.ModeRevision++
	previous.PermissionRevision++
	previous.Administrator = administrator
	return previous, nil
}

func (s *Service) GrantAdministrator(ctx context.Context, space, device string, expected int64, grant bool) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validateRequestAuthorities(ctx); err != nil {
		return Policy{}, err
	}
	p, err := s.Get(ctx, space, device)
	if err != nil {
		return p, err
	}
	if p.PermissionRevision != expected {
		return p, ErrRevision
	}
	if grant && !p.Coordinated {
		return p, ErrAdministratorMode
	}
	if err := s.fenceAuthorityLocked(ctx, space, device, p.PermissionRevision); err != nil {
		return p, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	if err := ValidateRequestAuthorityTx(ctx, tx); err != nil {
		return p, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE kernel_device_coordination SET administrator=?, permission_revision=permission_revision+1 WHERE space_id=? AND device_id=? AND permission_revision=?`, grant, space, device, expected)
	if err != nil {
		return p, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return p, ErrRevision
	}
	if err := meshaudit.QueueTx(ctx, tx, space, device, "device_mesh.administrator_changed", meshaudit.Details{PermissionRevision: expected + 1, ModeRevision: p.ModeRevision, ProviderEpoch: p.ProviderEpoch, Administrator: &grant, Coordinated: &p.Coordinated}); err != nil {
		return p, err
	}
	if err := tx.Commit(); err != nil {
		return p, err
	}
	s.cancelLocked(space, device)
	p.Administrator = grant
	p.PermissionRevision++
	return p, nil
}

func (s *Service) ResetTx(ctx context.Context, tx *sql.Tx, space, device string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_coordination(space_id,device_id) VALUES(?,?) ON CONFLICT DO NOTHING`, space, device); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE kernel_device_coordination SET administrator=0, permission_revision=permission_revision+1, provider_epoch=provider_epoch+1 WHERE space_id=? AND device_id=?`, space, device)
	return err
}

func (s *Service) RevokeDevice(ctx context.Context, space, device string, revoke func(*sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validateRequestAuthorities(ctx); err != nil {
		return err
	}
	policy, err := s.Get(ctx, space, device)
	if err != nil {
		return err
	}
	if err := s.fenceAuthorityLocked(ctx, space, device, policy.PermissionRevision); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ValidateRequestAuthorityTx(ctx, tx); err != nil {
		return err
	}
	if err := revoke(tx); err != nil {
		return err
	}
	if err := s.ResetTx(ctx, tx, space, device); err != nil {
		return err
	}
	if err := meshaudit.QueueTx(ctx, tx, space, device, "device_mesh.device_revoked", meshaudit.Details{PermissionRevision: policy.PermissionRevision + 1, ProviderEpoch: policy.ProviderEpoch + 1}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.cancelLocked(space, device)
	return nil
}

func (s *Service) Invalidate(space, device string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelLocked(space, device)
}

func (s *Service) cancelLocked(space, device string) {
	key := space + "\x00" + device
	for _, cancel := range s.active[key] {
		cancel(ErrScopeExpired)
	}
	delete(s.active, key)
}

func SelectRole(requested string, available []string) (string, error) {
	if requested != "" {
		for _, role := range available {
			if role == requested {
				return role, nil
			}
		}
		return "", ErrRoleRequired
	}
	if len(available) == 0 {
		return "", ErrRoleRequired
	}
	if len(available) != 1 {
		return "", ErrRoleSelection
	}
	return available[0], nil
}

func (s *Service) Begin(ctx context.Context, space, device, target, core, role, request string) (context.Context, ExecutionScope, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.Get(ctx, space, device)
	if err != nil {
		return ctx, ExecutionScope{}, nil, err
	}
	if target == "" {
		target = device
	}
	targetPolicy, err := s.Get(ctx, space, target)
	if err != nil {
		return ctx, ExecutionScope{}, nil, err
	}
	owner := target
	if targetPolicy.Coordinated {
		owner = core
	}
	scope := ExecutionScope{SpaceID: space, AuthorizationRealm: core, InitiatorDeviceID: device, TargetDeviceID: target, CoreID: core, ProviderEpoch: p.ProviderEpoch, TargetProviderEpoch: targetPolicy.ProviderEpoch, Coordinated: targetPolicy.Coordinated, ModeRevision: targetPolicy.ModeRevision, PermissionRevision: p.PermissionRevision, TargetPermissionRevision: targetPolicy.PermissionRevision, RoleID: role, RoleOwnerID: owner, ResourceOwnerID: owner, RequestID: request}
	if err := s.Validate(ctx, scope); err != nil {
		return ctx, ExecutionScope{}, nil, err
	}
	guarded := context.WithValue(WithScope(ctx, scope), guardKey{}, func(current context.Context) error { return s.Validate(current, scope) })
	guarded = context.WithValue(guarded, authorityServiceKey{}, s)
	guarded = context.WithValue(guarded, commitKey{}, func(current context.Context, commit func() error) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.validateRequestAuthorities(current); err != nil {
			return err
		}
		if err := s.Validate(current, scope); err != nil {
			return err
		}
		return commit()
	})
	child, cancel := context.WithCancelCause(guarded)
	key := space + "\x00" + device
	s.next++
	id := s.next
	child = context.WithValue(child, activeExecutionKey{}, activeExecution{service: s, id: id})
	if s.active[key] == nil {
		s.active[key] = make(map[uint64]context.CancelCauseFunc)
	}
	s.active[key][id] = cancel
	targetKey := space + "\x00" + target
	if targetKey != key {
		if s.active[targetKey] == nil {
			s.active[targetKey] = make(map[uint64]context.CancelCauseFunc)
		}
		s.active[targetKey][id] = cancel
	}
	finish := func() {
		s.mu.Lock()
		delete(s.active[key], id)
		delete(s.active[targetKey], id)
		s.mu.Unlock()
		cancel(nil)
	}
	return child, scope, finish, nil
}

func (s *Service) Validate(ctx context.Context, scope ExecutionScope) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	provider, _, err := s.Provider(ctx)
	if err != nil {
		return err
	}
	if provider != "" && provider != scope.CoreID {
		return ErrScopeExpired
	}
	p, err := s.Get(ctx, scope.SpaceID, scope.InitiatorDeviceID)
	if err != nil {
		return err
	}
	if p.ProviderEpoch != scope.ProviderEpoch || p.PermissionRevision != scope.PermissionRevision {
		return ErrScopeExpired
	}
	target := scope.TargetDeviceID
	if target == "" {
		target = scope.InitiatorDeviceID
	}
	targetPolicy, err := s.Get(ctx, scope.SpaceID, target)
	if err != nil {
		return err
	}
	if targetPolicy.ModeRevision != scope.ModeRevision || targetPolicy.Coordinated != scope.Coordinated || (scope.TargetPermissionRevision != 0 && targetPolicy.PermissionRevision != scope.TargetPermissionRevision) || (scope.TargetProviderEpoch != 0 && targetPolicy.ProviderEpoch != scope.TargetProviderEpoch) {
		return ErrScopeExpired
	}
	owner := target
	if targetPolicy.Coordinated {
		owner = scope.CoreID
	}
	if scope.ResourceOwnerID != owner || scope.RoleOwnerID != owner {
		return ErrWrongOwner
	}
	var trusted string
	err = s.db.QueryRowContext(ctx, `SELECT trust_state FROM kernel_devices WHERE space_id=? AND device_id=?`, scope.SpaceID, scope.InitiatorDeviceID).Scan(&trusted)
	if err != nil {
		return err
	}
	if trusted != "trusted" {
		return fmt.Errorf("%w: %s", ErrScopeExpired, runtimeidentity.DeviceID(scope.InitiatorDeviceID))
	}
	if target != scope.InitiatorDeviceID {
		if err := s.db.QueryRowContext(ctx, `SELECT trust_state FROM kernel_devices WHERE space_id=? AND device_id=?`, scope.SpaceID, target).Scan(&trusted); err != nil {
			return err
		}
		if trusted != "trusted" {
			return ErrScopeExpired
		}
	}
	return nil
}
