package coordination_test

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestManagementFencePreservesAuthenticatedMutationUntilOwnerAcknowledges(t *testing.T) {
	for _, acknowledged := range []bool{false, true} {
		t.Run(map[bool]string{false: "unconfirmed", true: "acknowledged"}[acknowledged], func(t *testing.T) {
			db, service := setup(t)
			business, scope, endBusiness, err := service.Begin(t.Context(), "space", "a", "a", "space", "role", "business")
			if err != nil {
				t.Fatal(err)
			}
			defer endBusiness()
			if _, err := service.TrackRemoteAuthority(business, scope, "session", 1); err != nil {
				t.Fatal(err)
			}
			management, _, endManagement, err := service.Begin(t.Context(), "space", "a", "a", "space", "", "management")
			if err != nil {
				t.Fatal(err)
			}
			defer endManagement()
			ordinary, _, endOrdinary, err := service.Begin(t.Context(), "space", "a", "a", "space", "", "ordinary-request")
			if err != nil {
				t.Fatal(err)
			}
			defer endOrdinary()
			ordinary, err = coordination.WithRequestAuthority(ordinary, ordinary)
			if err != nil {
				t.Fatal(err)
			}
			linked, err := coordination.WithManagementRequestAuthority(management)
			if err != nil {
				t.Fatal(err)
			}
			barrierReached := false
			service.SetAuthorityBarrier(func(ctx context.Context, space, device string, revision int64, sources []string) error {
				barrierReached = true
				if context.Cause(business) == nil {
					t.Error("旧业务未在屏障前取消")
				}
				if context.Cause(ordinary) == nil {
					t.Error("普通 WithRequestAuthority 请求被错误排除取消")
				}
				if err := ctx.Err(); err != nil {
					t.Errorf("管理请求在等待 Source ACK 前被自己取消: %v", err)
					return err
				}
				if !acknowledged {
					return coordination.ErrAuthorityUnconfirmed
				}
				return coordination.FenceSourceAuthority(ctx, db, space, device, revision)
			})
			policy, err := service.ChangeMode(linked, "space", "a", 1, true, "role")
			if !barrierReached {
				t.Fatal("未执行真实所有者屏障")
			}
			if acknowledged {
				if err != nil || !policy.Coordinated || policy.ModeRevision != 2 {
					t.Fatalf("Source 已确认但权限修改失败: %+v %v", policy, err)
				}
				if context.Cause(management) == nil {
					t.Fatal("管理变更提交后旧权限 scope 仍然有效")
				}
			} else {
				if !errors.Is(err, coordination.ErrAuthorityUnconfirmed) {
					t.Fatalf("未确认调用没有拦截: %v", err)
				}
				actual, readErr := service.Get(t.Context(), "space", "a")
				if readErr != nil || actual.Coordinated || actual.ModeRevision != 1 {
					t.Fatalf("未确认仍改变权限: %+v %v", actual, readErr)
				}
			}
		})
	}
}
