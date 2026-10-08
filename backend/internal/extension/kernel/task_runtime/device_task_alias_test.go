package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type deviceTaskImportStore struct {
	deviceTaskCatalogStore
	creates int
}

func (s *deviceTaskImportStore) CreateTaskDefinition(_ context.Context, definition *TaskDefinition) (bool, error) {
	for _, existing := range s.definitions {
		if existing.TaskID == definition.TaskID {
			return false, nil
		}
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return false, err
	}
	var copy TaskDefinition
	if err := json.Unmarshal(encoded, &copy); err != nil {
		return false, err
	}
	s.definitions = append(s.definitions, &copy)
	s.creates++
	return true, nil
}

func TestDeviceTaskImportRejectsExpiredAuthorityAndConflictingMetadata(t *testing.T) {
	source, _ := deviceTaskCatalogFixture(t)
	entry, err := source.DescribeInstalledTaskCatalogEntry(t.Context(), "device", "task-00")
	if err != nil {
		t.Fatal(err)
	}
	store := &deviceTaskImportStore{}
	service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	ctx := coordination.WithScope(t.Context(), coordination.ExecutionScope{CoreID: "core", TargetDeviceID: "device"})
	denied := coordination.WithAdditionalGuard(ctx, func(context.Context) error { return coordination.ErrScopeExpired })
	if _, err := service.ImportDeviceTaskDefinition(denied, entry); !errors.Is(err, coordination.ErrScopeExpired) || store.creates != 0 {
		t.Fatalf("过期授权导入了设备任务: %v", err)
	}
	first, err := service.ImportDeviceTaskDefinition(ctx, entry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ImportDeviceTaskDefinition(ctx, entry); err != nil || store.creates != 1 {
		t.Fatalf("目录重试重新保存或拒绝相同定义: %v", err)
	}
	first.InputSchema[0] = 'x'
	if !json.Valid(store.definitions[0].InputSchema) {
		t.Fatal("导入响应共享了持久化定义")
	}
	store.definitions[0].Entry = "foreign.cjs"
	if _, err := service.ImportDeviceTaskDefinition(ctx, entry); !errors.Is(err, coordination.ErrRequestConflict) || store.creates != 1 || store.definitions[0].Entry != "foreign.cjs" {
		t.Fatalf("目录导入覆盖了冲突定义: %v", err)
	}
	if _, err := service.ImportDeviceTaskDefinition(t.Context(), entry); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("无授权请求导入了设备任务: %v", err)
	}
}

func TestCoreDeviceTaskDefinitionAliasesPreserveSourceFingerprintAndFenceAuthority(t *testing.T) {
	service, _ := deviceTaskCatalogFixture(t)
	page, err := service.DescribeInstalledTaskCatalog(t.Context(), "device", DeviceTaskCatalogRequest{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	entry := page.Entries[0]
	definition, err := NewCoreDeviceTaskDefinition("core", "device", entry)
	if err != nil {
		t.Fatal(err)
	}
	if definition.TaskID == entry.Definition.TaskID || definition.InstalledGeneration != 0 || definition.RemoteSource == nil || SourceTaskDefinitionID(definition) != entry.Definition.TaskID || definition.ExecutionPlacement != TaskExecutionPlacementDevice || definition.RemoteSource.Placement != entry.Definition.ExecutionPlacement {
		t.Fatal("Core 目录引用覆盖了来源任务或安装身份")
	}
	if portable, err := portableTaskDefinitionFingerprint(definition); err != nil || portable != entry.Target.PortableFingerprint {
		t.Fatalf("Core 目录身份改变了来源可移植指纹: %v", err)
	}
	if err := ValidateTargetTaskDefinition("device", definition, entry.Target); err != nil {
		t.Fatal(err)
	}
	authority := coordination.ExecutionScope{CoreID: "core", TargetDeviceID: "device"}
	if err := validateDeviceTaskSource(authority, definition); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"core", "device", "namespace", "source", "placement", "input", "installation"} {
		t.Run(scenario, func(t *testing.T) {
			copy := *definition
			source := *definition.RemoteSource
			copy.RemoteSource = &source
			proof := authority
			switch scenario {
			case "core":
				proof.CoreID = "foreign"
			case "device":
				proof.TargetDeviceID = "foreign"
			case "namespace":
				copy.TaskID = "foreign"
			case "source":
				source.Reference.SourceTaskID = "foreign"
			case "placement":
				source.Placement = TaskExecutionPlacementCloud
			case "input":
				copy.InputSchema = []byte(`{"type":"string"}`)
			case "installation":
				copy.InstalledGeneration = 7
			}
			if err := validateDeviceTaskSource(proof, &copy); err == nil {
				t.Fatal("目录任务引用被转交或改变了来源定义")
			}
		})
	}
	run := &TaskRun{TaskDefinitionID: definition.TaskID, DefinitionFingerprint: "", ExecutionTarget: TaskExecutionTarget{SourceTaskDefinitionID: entry.Definition.TaskID}}
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	if err := validateSourceTaskRoot(authority, run, &entry.Definition, entry.Target); err != nil {
		t.Fatal(err)
	}
	run.DefinitionFingerprint = hashBytes([]byte("foreign"))
	if err := validateSourceTaskRoot(authority, run, &entry.Definition, entry.Target); !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) {
		t.Fatal("来源设备接受了被改写的 Core 目录任务")
	}
}

func TestDeviceTaskDefinitionMappingSurvivesTargetNormalizationAndConnectionRefresh(t *testing.T) {
	target := TaskExecutionTarget{SourceTaskDefinitionID: "source-task", SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 1}
	normalized := target.Normalize()
	if normalized.SourceTaskDefinitionID != "source-task" || !normalized.ExactEqual(target) {
		t.Fatal("目标标准化丢失了来源任务映射")
	}
	changed := target
	changed.SourceTaskDefinitionID = "foreign"
	if changed.StableEqual(target) || changed.ExactEqual(target) {
		t.Fatal("其他来源任务借用了已固定执行目标")
	}
	changed = target
	changed.RuntimeSessionID, changed.ConnectionGeneration = "new", 2
	if !changed.StableEqual(target) || changed.ExactEqual(target) {
		t.Fatal("连接刷新改变了稳定任务映射或继续复用旧会话")
	}
}

func TestDeviceTaskDefinitionGuardUsesSourceAuthorityInsteadOfCoreInstallation(t *testing.T) {
	source, _ := deviceTaskCatalogFixture(t)
	entry, err := source.DescribeInstalledTaskCatalogEntry(t.Context(), "device", "task-00")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := NewCoreDeviceTaskDefinition("core", "device", entry)
	if err != nil {
		t.Fatal(err)
	}
	store := &deviceTaskCatalogStore{definitions: []*TaskDefinition{definition}}
	config := DefaultTaskRuntimeConfig()
	config.InstalledDefinitionValidator = func(context.Context, *TaskDefinition) error { return errors.New("Core 未安装来源插件") }
	service := NewTaskRuntimeService(store, config)
	run := &TaskRun{TaskDefinitionID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, ExecutionPlacement: TaskExecutionPlacementDevice, ExecutionTarget: TaskExecutionTarget{SourceTaskDefinitionID: entry.Definition.TaskID}}
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	ctx := coordination.WithScope(t.Context(), coordination.ExecutionScope{CoreID: "core", TargetDeviceID: "device"})
	if err := coordination.ValidateCurrent(service.guardOwnedTaskDefinition(ctx, run)); err != nil {
		t.Fatal(err)
	}
	run.ExecutionTarget.SourceTaskDefinitionID = "other-task"
	if err := coordination.ValidateCurrent(service.guardOwnedTaskDefinition(ctx, run)); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("错误来源映射绕过定义约束: %v", err)
	}
}
