package task_runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type taskPermissionEvaluatorFunc func(context.Context, permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult

func (f taskPermissionEvaluatorFunc) Evaluate(ctx context.Context, request permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
	return f(ctx, request)
}

func sourcePermissionFixture(t *testing.T) (context.Context, *TaskRun, *TaskDefinition, json.RawMessage) {
	t.Helper()
	_, _, authority, run, _ := taskAuthorityFixture(t)
	definition := &TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module", InstalledGeneration: 7, PermissionRequirementStrings: []string{"test.source.read"}}
	input := json.RawMessage(`{"private":"仅用于本次执行"}`)
	run.TaskDefinitionID, run.ExtensionID, run.ModuleID = definition.TaskID, definition.ExtensionID, definition.ModuleID
	run.TaskRunID, run.InvocationID, run.ScopeSnapshotID, run.InputHash = "run", "invocation", "snapshot", hashBytes(input)
	run.ExecutionTarget = TaskExecutionTarget{ProviderID: "provider", ProviderInstanceID: "instance", SpaceID: runtimeidentity.SpaceID(authority.CoreID), DeviceID: runtimeidentity.DeviceID(authority.TargetDeviceID), RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 3}
	return coordination.WithScope(t.Context(), authority), run, definition, input
}

func TestSourceTaskPermissionsBindActualDeviceAndInstalledGeneration(t *testing.T) {
	ctx, run, definition, input := sourcePermissionFixture(t)
	authority, _ := coordination.FromContext(ctx)
	var calls int
	guard := NewSourceTaskPermissionGuard(taskPermissionEvaluatorFunc(func(_ context.Context, request permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
		calls++
		if request.Subject.Type != permission.SubjectModule || request.Subject.ExtensionID != definition.ExtensionID || request.Subject.ModuleID != definition.ModuleID || request.Generation != 7 || request.InvocationID != run.InvocationID || request.ScopeSnapshotID != run.ScopeSnapshotID || request.Target.ID != definition.TaskID || !request.IsBackground || request.ApprovalMode != "" || request.ApprovalRecordID != "" || string(request.Input) != string(input) || request.ExecutionContext.DeviceID.String() != authority.TargetDeviceID || request.ExecutionContext.SpaceID.String() != authority.CoreID || request.ExecutionContext.RuntimeSessionID != "session" || request.ExecutionContext.ProviderInstanceID != "instance" || request.ExecutionContext.Placement != permission.ExecutionPlacementDevice {
			t.Error("资源权限评估未绑定实际设备安装与原调用身份")
		}
		return permission.PermissionEvaluationResult{Decision: permission.DecisionAllow}
	}))
	for _, coordinated := range []bool{false, true} {
		authority.Coordinated = coordinated
		if err := guard(coordination.WithScope(ctx, authority), run, definition, input); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatal("统筹模式绕过了设备资源权限检查")
	}
	for _, changed := range []string{"target", "realm", "module", "input", "installation", "session"} {
		t.Run(changed, func(t *testing.T) {
			current, installed := CloneTaskRun(run), *definition
			switch changed {
			case "target":
				current.ExecutionTarget.DeviceID = "foreign"
			case "realm":
				current.ExecutionTarget.SpaceID = "foreign"
			case "module":
				current.ModuleID = "foreign"
			case "input":
				current.InputHash = hashBytes([]byte("foreign"))
			case "installation":
				installed.InstalledGeneration = 0
			case "session":
				current.ExecutionTarget.RuntimeSessionID = ""
			}
			if err := guard(ctx, current, &installed, input); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
				t.Fatalf("不一致的资源授权身份被接受: %v", err)
			}
		})
	}
}

func TestSourceTaskPermissionsRequireTargetBoundGrantAndStopAfterRevocation(t *testing.T) {
	ctx, run, definition, input := sourcePermissionFixture(t)
	registry := permission.NewPermissionDefinitionRegistry()
	registry.Register(permission.PermissionDefinition{ID: "test.source.read", Category: permission.CategoryFilesystem, RiskLevel: "high", AllowedScopes: []permission.ScopeType{permission.ScopeExtension}, PersistentGrantable: true, BackgroundAllowed: true, DefaultApproval: permission.ApprovalManual, RemoteExecution: permission.RemoteExecutionInherit})
	broker := permission.NewDefaultPermissionBroker(registry, permission.NewMemoryPermissionStorage())
	t.Cleanup(func() { _ = broker.Close() })
	guard := NewSourceTaskPermissionGuard(broker)
	if err := guard(ctx, run, definition, input); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
		t.Fatal("未授权的本机资源被使用")
	}
	request := permission.PermissionGrantRequest{Subject: permission.PermissionSubject{Type: permission.SubjectModule, ID: definition.ModuleID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID}, PermissionID: "test.source.read", Scope: permission.ScopeForExtension(definition.ExtensionID), Decision: permission.DecisionAllowPersistent, IssuedBy: permission.IssuerUser, TargetBinding: &permission.TargetBinding{DeviceID: "foreign"}}
	foreign, err := broker.Grant(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := guard(ctx, run, definition, input); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
		t.Fatal("其他设备的资源授权被借用")
	}
	if err := broker.Revoke(ctx, foreign.GrantID); err != nil {
		t.Fatal(err)
	}
	request.TargetBinding = &permission.TargetBinding{DeviceID: run.ExecutionTarget.DeviceID, RuntimeID: run.ExecutionTarget.RuntimeID, ProviderID: string(run.ExecutionTarget.ProviderID), ProviderInstanceID: string(run.ExecutionTarget.ProviderInstanceID)}
	grant, err := broker.Grant(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := guard(ctx, run, definition, input); err != nil {
		t.Fatalf("当前设备有效资源授权被拒绝: %v", err)
	}
	if err := broker.Revoke(ctx, grant.GrantID); err != nil {
		t.Fatal(err)
	}
	if err := guard(ctx, run, definition, input); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
		t.Fatal("撤销资源权限后继续执行")
	}
	request.Decision = permission.DecisionAllowOnce
	if _, err := broker.Grant(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := guard(ctx, run, definition, input); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
		t.Fatal("未消费绑定的一次性权限被重复使用")
	}
}

func TestSourceTaskPermissionsRejectMissingGuardAndInvalidDeclarations(t *testing.T) {
	ctx, run, definition, input := sourcePermissionFixture(t)
	service := &TaskRuntimeService{}
	if err := service.validateSourceTaskPermissions(ctx, run, definition, input); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
		t.Fatal("缺少本机权限端口仍接受资源声明")
	}
	for _, value := range []string{"", " test.source.read", "test.source.read "} {
		definition.PermissionRequirementStrings = []string{value}
		if _, err := sourceTaskPermissionRequirements(definition); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
			t.Fatal("无效权限编号被接受")
		}
	}
	definition.PermissionRequirementStrings = make([]string, 65)
	if _, err := sourceTaskPermissionRequirements(definition); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
		t.Fatal("权限声明上限未执行")
	}
	definition.PermissionRequirementStrings = nil
	definition.PermissionRequirements = []PermissionRequirement{{PermissionID: "test.source.read", Conditions: json.RawMessage(`invalid`)}}
	if _, err := sourceTaskPermissionRequirements(definition); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
		t.Fatal("错误权限条件被接受")
	}
}

func TestSourceTaskPermissionAcknowledgementRejectsSpoofedScopeAndTarget(t *testing.T) {
	ctx, run, _, input := sourcePermissionFixture(t)
	authority, _ := coordination.FromContext(ctx)
	target := TargetTaskDefinitionPin{DeviceID: authority.TargetDeviceID, TaskID: run.TaskDefinitionID, ExtensionID: run.ExtensionID, ModuleID: run.ModuleID, InstalledGeneration: 7, DefinitionFingerprint: hashBytes([]byte("full")), PortableFingerprint: hashBytes([]byte("portable")), EntryHash: "sha256:" + hashBytes([]byte("entry"))}
	request := SourceTaskPermissionRequest{Scope: authority, Run: *run, Input: input, Target: target}
	confirmed := SourceTaskPermissionAcknowledgement{Scope: authority, TaskRunID: run.TaskRunID, TaskGeneration: 1, ExecutionTarget: run.ExecutionTarget, InputHash: run.InputHash, Target: target, Allowed: true}
	if err := ValidateSourceTaskPermissionAcknowledgement(request, confirmed); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []string{"scope", "task", "generation", "session", "connection", "input", "target", "installation", "entry", "denied"} {
		t.Run(changed, func(t *testing.T) {
			ack := confirmed
			switch changed {
			case "scope":
				ack.Scope.PermissionRevision++
			case "task":
				ack.TaskRunID = "foreign"
			case "generation":
				ack.TaskGeneration++
			case "session":
				ack.ExecutionTarget.RuntimeSessionID = "foreign"
			case "connection":
				ack.ExecutionTarget.ConnectionGeneration++
			case "input":
				ack.InputHash = hashBytes([]byte("foreign"))
			case "target":
				ack.Target.DeviceID = "foreign"
			case "installation":
				ack.Target.InstalledGeneration++
			case "entry":
				ack.Target.EntryHash = "sha256:" + hashBytes([]byte("foreign"))
			case "denied":
				ack.Allowed = false
			}
			if err := ValidateSourceTaskPermissionAcknowledgement(request, ack); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
				t.Fatalf("不一致的权限确认被接受: %v", err)
			}
		})
	}
}

func TestSourceTaskPermissionsCannotUseInstallationGrantToSkipIndependentApproval(t *testing.T) {
	for _, scenario := range []string{"per-use", "remote-approval", "remote-denied", "ordinary"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, run, definition, input := sourcePermissionFixture(t)
			registry := permission.NewPermissionDefinitionRegistry()
			policy := permission.PermissionDefinition{ID: "test.source.read", Category: permission.CategoryFilesystem, RiskLevel: "high", AllowedScopes: []permission.ScopeType{permission.ScopeExtension}, BackgroundAllowed: true, RemoteExecution: permission.RemoteExecutionInherit}
			switch scenario {
			case "per-use":
				policy.RequiresPerUse = true
			case "remote-approval":
				policy.RemoteExecution = permission.RemoteExecutionRequireApproval
			case "remote-denied":
				policy.RemoteExecution = permission.RemoteExecutionDeny
			}
			registry.Register(policy)
			broker := permission.NewDefaultPermissionBroker(registry, permission.NewMemoryPermissionStorage())
			t.Cleanup(func() { _ = broker.Close() })
			broker.InstallationPolicy = func(context.Context, permission.PermissionSubject, permission.PermissionRequirement, permission.PermissionDefinition) (permission.PermissionDecision, bool) {
				return permission.DecisionAllow, true
			}
			err := NewSourceTaskPermissionGuard(broker)(ctx, run, definition, input)
			if scenario == "ordinary" && err != nil || scenario != "ordinary" && !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
				t.Fatalf("安装授权错误替代了独立资源审批: %v", err)
			}
		})
	}
}
