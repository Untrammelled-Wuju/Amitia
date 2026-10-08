package coordination_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func historicalTaskProof() coordination.TaskReadProof {
	return coordination.TaskReadProof{TaskRunID: "run", DefinitionFingerprint: strings.Repeat("a", 64), Scope: coordination.ExecutionScope{SpaceID: "core", CoreID: "core", AuthorizationRealm: "core", InitiatorDeviceID: "caller", TargetDeviceID: "device", ProviderEpoch: 1, TargetProviderEpoch: 1, PermissionRevision: 1, TargetPermissionRevision: 1, ModeRevision: 1, RoleRevision: 1, RoleID: "old-role", ResourceOwnerID: "device", RoleOwnerID: "device", RequestID: "request", ExecutionID: "execution", TurnID: "turn"}}
}

func TestHistoricalTaskReadPreservesFrozenOwnerAndRejectsWrites(t *testing.T) {
	proof := historicalTaskProof()
	current := proof.Scope
	current.Coordinated, current.ModeRevision, current.PermissionRevision, current.TargetPermissionRevision = true, 2, 3, 3
	current.RoleOwnerID, current.ResourceOwnerID, current.RoleID, current.RoleRevision = "core", "core", "", 0
	active := true
	ctx := coordination.WithAdditionalGuard(coordination.WithScope(t.Context(), current), func(context.Context) error {
		if !active {
			return coordination.ErrScopeExpired
		}
		return nil
	})
	read, err := coordination.WithTaskReadAuthority(ctx, current, proof)
	if err != nil {
		t.Fatal(err)
	}
	if saved, ok := coordination.FromContext(read); !ok || saved != proof.Scope {
		t.Fatal("历史读取改变了原来的数据所有者或角色")
	}
	if actual, saved, ok := coordination.TaskReadAuthority(read); !ok || actual != current || saved != proof {
		t.Fatal("当前读取权限与原始数据归属没有分别保存")
	}
	if err := coordination.CommitCurrent(read, func() error { t.Fatal("只读历史授权执行了写入"); return nil }); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("历史通道未拒绝写入: %v", err)
	}
	active = false
	if !errors.Is(coordination.ValidateCurrent(read), coordination.ErrScopeExpired) {
		t.Fatal("当前权限撤销后仍能读取历史数据")
	}
	other := current
	other.CoreID, other.SpaceID, other.AuthorizationRealm = "other-core", "other-core", "other-core"
	if !errors.Is(coordination.ValidateTaskReadProof(other, proof), coordination.ErrWrongOwner) {
		t.Fatal("历史读取迁移了原 Core 的专属数据")
	}
}

func TestHistoricalTaskResourcesCannotCrossTaskOwnerOrDefinition(t *testing.T) {
	for _, scenario := range []string{"valid", "checkpoint", "progress", "foreign_task", "foreign_checkpoint", "foreign_scope", "foreign_owner", "foreign_definition", "deleted", "arbitrary_resource", "foreign_progress"} {
		t.Run(scenario, func(t *testing.T) {
			proof := historicalTaskProof()
			id := "task/input/run"
			document := map[string]any{"executionScope": proof.Scope, "definitionFingerprint": proof.DefinitionFingerprint, "taskRunId": proof.TaskRunID}
			resource := &coordination.Resource{OwnerID: "device", RoleID: "old-role", Kind: "checkpoint", Revision: 1}
			switch scenario {
			case "checkpoint", "foreign_checkpoint":
				id = "task/checkpoint/checkpoint-id"
				delete(document, "taskRunId")
				run := proof.TaskRunID
				if scenario == "foreign_checkpoint" {
					run = "other"
				}
				document["checkpoint"] = map[string]any{"taskRunId": run}
			case "progress", "foreign_progress":
				id = "task/progress/run/1/attempt"
				delete(document, "taskRunId")
				if scenario == "foreign_progress" {
					id = "task/progress/other/1/attempt"
				}
			case "foreign_task":
				document["taskRunId"] = "other"
			case "foreign_scope":
				other := proof.Scope
				other.ExecutionID = "other"
				document["executionScope"] = other
			case "foreign_owner":
				resource.OwnerID = "core"
			case "foreign_definition":
				document["definitionFingerprint"] = strings.Repeat("b", 64)
			case "deleted":
				resource.Deleted = true
			case "arbitrary_resource":
				id = "memory/private"
			}
			resource.ID = id
			resource.Body, _ = json.Marshal(document)
			allowed := scenario == "valid" || scenario == "checkpoint" || scenario == "progress"
			if err := coordination.ValidateTaskReadResource(proof, "checkpoint", id, resource); (err == nil) != allowed {
				t.Fatalf("历史资源边界判断错误: %v", err)
			}
		})
	}
}
