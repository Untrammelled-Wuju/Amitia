package coordination_test

import (
	"database/sql"
	"errors"
	"testing"
)

func TestProjectionAuthorityKeepsRegisteredTargetDeviceAndRejectsCoreIdentifier(t *testing.T) {
	_, service := setup(t)
	ctx := t.Context()

	_, scope, finish, err := service.BeginWithOwners(ctx, "space", "a", "b", "core-x", "role", "req", "b", "core-x")
	if err != nil {
		t.Fatalf("已注册目标设备被拒绝: %v", err)
	}
	defer finish()
	if scope.TargetDeviceID != "b" || scope.ResourceOwnerID != "b" || scope.RoleOwnerID != "core-x" {
		t.Fatalf("执行范围归属错位: %+v", scope)
	}
	if err := service.Validate(ctx, scope); err != nil {
		t.Fatalf("已注册目标设备校验失败: %v", err)
	}

	_, _, _, err = service.BeginWithOwners(ctx, "space", "a", "core-x", "core-x", "role", "req", "", "")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("统筹中心标识被当作目标设备时未按原始错误失败: %v", err)
	}
}
