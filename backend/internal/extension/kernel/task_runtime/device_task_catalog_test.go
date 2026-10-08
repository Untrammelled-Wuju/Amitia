package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

type deviceTaskCatalogStore struct {
	TaskStore
	definitions []*TaskDefinition
}

func (s *deviceTaskCatalogStore) ListTaskDefinitions(context.Context, string) ([]*TaskDefinition, error) {
	return s.definitions, nil
}

func (s *deviceTaskCatalogStore) GetTaskDefinition(_ context.Context, id string) (*TaskDefinition, error) {
	for _, definition := range s.definitions {
		if definition.TaskID == id {
			encoded, _ := json.Marshal(definition)
			var copy TaskDefinition
			_ = json.Unmarshal(encoded, &copy)
			return &copy, nil
		}
	}
	return nil, errors.New("task not found")
}

func deviceTaskCatalogFixture(t *testing.T) (*TaskRuntimeService, *deviceTaskCatalogStore) {
	t.Helper()
	store := &deviceTaskCatalogStore{}
	for index := 0; index < 10; index++ {
		store.definitions = append(store.definitions, &TaskDefinition{TaskID: fmt.Sprintf("task-%02d", 9-index), ExtensionID: "extension", ModuleID: "module", Entry: "task.cjs", EntryHash: "sha256:" + hashBytes([]byte("installed-entry")), InstalledGeneration: 2, InputSchema: json.RawMessage(`{"type":"object"}`)})
	}
	config := DefaultTaskRuntimeConfig()
	config.InstalledDefinitionValidator = func(_ context.Context, definition *TaskDefinition) error {
		if definition.ModuleID == "disabled" {
			return errors.New("disabled")
		}
		return nil
	}
	config.EntryResolver = func(context.Context, *TaskDefinition) (string, error) { return "task.cjs", nil }
	return NewTaskRuntimeService(store, config), store
}

func TestDeviceTaskCatalogPaginationPinsInstalledVersionAndSkipsUninstalled(t *testing.T) {
	service, store := deviceTaskCatalogFixture(t)
	store.definitions = append(store.definitions, &TaskDefinition{TaskID: "uninstalled"})
	store.definitions[0].ModuleID = "disabled"
	page, err := service.DescribeInstalledTaskCatalog(t.Context(), "device", DeviceTaskCatalogRequest{})
	if err != nil || len(page.Entries) != 8 || page.Entries[0].Definition.TaskID != "task-00" || page.NextCursor == "" || !validTaskFingerprint(page.Revision) {
		t.Fatalf("设备任务目录分页未固定版本: %+v %v", page, err)
	}
	next, err := service.DescribeInstalledTaskCatalog(t.Context(), "device", DeviceTaskCatalogRequest{Cursor: page.NextCursor})
	if err != nil || len(next.Entries) != 1 || next.Entries[0].Definition.TaskID != "task-08" || next.Revision != page.Revision || next.NextCursor != "" {
		t.Fatalf("设备任务后续分页不一致: %+v %v", next, err)
	}
	page.Entries[0].Definition.InputSchema[0] = 'x'
	if !json.Valid(store.definitions[9].InputSchema) {
		t.Fatal("目录响应共享了源定义正文")
	}
	store.definitions[1].InstalledGeneration++
	if _, err := service.DescribeInstalledTaskCatalog(t.Context(), "device", DeviceTaskCatalogRequest{Cursor: page.NextCursor}); !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) {
		t.Fatal("旧安装游标继续分页")
	}
}

func TestDeviceTaskCatalogReferencesSeparateCoreDeviceAndPortableVersions(t *testing.T) {
	service, _ := deviceTaskCatalogFixture(t)
	page, err := service.DescribeInstalledTaskCatalog(t.Context(), "device", DeviceTaskCatalogRequest{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	entry := page.Entries[0]
	first, err := NewDeviceTaskDefinitionReference("core", "device", entry)
	if err != nil || first.CatalogID == "" {
		t.Fatal(err)
	}
	again, _ := NewDeviceTaskDefinitionReference("core", "device", entry)
	if first != again {
		t.Fatal("同一设备任务目录身份不稳定")
	}
	foreignCore, _ := NewDeviceTaskDefinitionReference("foreign", "device", entry)
	foreignEntry := entry
	foreignEntry.Target.DeviceID = "foreign-device"
	foreignDevice, _ := NewDeviceTaskDefinitionReference("core", "foreign-device", foreignEntry)
	if first.CatalogID == foreignCore.CatalogID || first.CatalogID == foreignDevice.CatalogID {
		t.Fatal("跨 Core 或同名设备任务相互覆盖")
	}
	entry.Definition.Version = "updated"
	entry.Target.PortableFingerprint, _ = portableTaskDefinitionFingerprint(&entry.Definition)
	entry.Target.DefinitionFingerprint, _ = taskDefinitionFingerprint(&entry.Definition)
	updated, err := NewDeviceTaskDefinitionReference("core", "device", entry)
	if err != nil || updated.CatalogID == first.CatalogID {
		t.Fatal("不同任务版本复用了原目录身份")
	}
	entry.Target.EntryHash = "sha256:" + hashBytes([]byte("foreign"))
	if _, err := NewDeviceTaskDefinitionReference("core", "device", entry); !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) {
		t.Fatal("目录允许不匹配的入口版本")
	}
}

func TestDeviceTaskCatalogRejectsMutationDuringReadAndMalformedCursor(t *testing.T) {
	service, store := deviceTaskCatalogFixture(t)
	for _, request := range []DeviceTaskCatalogRequest{{Limit: 9}, {Limit: -1}, {Cursor: "invalid"}} {
		if _, err := service.DescribeInstalledTaskCatalog(t.Context(), "device", request); err == nil {
			t.Fatal("错误任务目录分页被接受")
		}
	}
	service.config.EntryResolver = func(context.Context, *TaskDefinition) (string, error) {
		store.definitions[9].Version = "changed"
		return "task.cjs", nil
	}
	if _, err := service.DescribeInstalledTaskCatalog(t.Context(), "device", DeviceTaskCatalogRequest{Limit: 1}); !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) {
		t.Fatalf("更新期间目录返回了混合版本: %v", err)
	}
}
