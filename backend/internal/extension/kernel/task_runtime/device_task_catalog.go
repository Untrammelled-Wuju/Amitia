package task_runtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
)

type DeviceTaskCatalogRequest struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type DeviceTaskCatalogEntry struct {
	Definition TaskDefinition          `json:"definition"`
	Target     TargetTaskDefinitionPin `json:"target"`
}

type DeviceTaskCatalogPage struct {
	Capabilities SourceTaskCapabilities   `json:"capabilities"`
	DeviceID     string                   `json:"deviceId"`
	Revision     string                   `json:"revision"`
	Entries      []DeviceTaskCatalogEntry `json:"entries"`
	NextCursor   string                   `json:"nextCursor,omitempty"`
}

type DeviceTaskDefinitionReference struct {
	CatalogID           string `json:"catalogId"`
	CoreID              string `json:"coreId"`
	DeviceID            string `json:"deviceId"`
	SourceTaskID        string `json:"sourceTaskId"`
	PortableFingerprint string `json:"portableFingerprint"`
}

type deviceTaskCatalogCursor struct {
	Revision    string `json:"revision"`
	AfterTaskID string `json:"afterTaskId"`
}

func NewDeviceTaskDefinitionReference(coreID, deviceID string, entry DeviceTaskCatalogEntry) (DeviceTaskDefinitionReference, error) {
	if coreID == "" || deviceID == "" || len(coreID) > 256 || len(deviceID) > 256 || strings.TrimSpace(coreID) != coreID || strings.TrimSpace(deviceID) != deviceID || entry.Definition.TaskID == "" || len(entry.Definition.TaskID) > 256 {
		return DeviceTaskDefinitionReference{}, NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录身份无效")
	}
	if err := ValidateTargetTaskDefinition(deviceID, &entry.Definition, entry.Target); err != nil {
		return DeviceTaskDefinitionReference{}, err
	}
	full, err := taskDefinitionFingerprint(&entry.Definition)
	if err != nil || full != entry.Target.DefinitionFingerprint || entry.Definition.InstalledGeneration != entry.Target.InstalledGeneration {
		return DeviceTaskDefinitionReference{}, NewTaskError(ErrTaskDefinitionInvalid, "任务目录元数据与设备安装指纹不一致")
	}
	encoded, err := json.Marshal([]string{coreID, deviceID, entry.Definition.TaskID, entry.Target.PortableFingerprint})
	if err != nil {
		return DeviceTaskDefinitionReference{}, err
	}
	return DeviceTaskDefinitionReference{CatalogID: "mesh-task-" + hashBytes(encoded), CoreID: coreID, DeviceID: deviceID, SourceTaskID: entry.Definition.TaskID, PortableFingerprint: entry.Target.PortableFingerprint}, nil
}

func (s *TaskRuntimeService) installedTaskCatalog(ctx context.Context) ([]TaskDefinition, string, error) {
	if err := validateSourceTaskExecutionAvailable(ctx); err != nil {
		return nil, "", err
	}
	if s.config.InstalledDefinitionValidator == nil || s.config.EntryResolver == nil {
		return nil, "", NewTaskError(ErrTaskDependencyUnavailable, "设备已安装任务查询端口不完整")
	}
	definitions, err := s.store.ListTaskDefinitions(ctx, "")
	if err != nil {
		return nil, "", err
	}
	if len(definitions) > 256 {
		return nil, "", NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录超过当前上限")
	}
	items := make([]TaskDefinition, 0, len(definitions))
	seen := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		if definition == nil || definition.InstalledGeneration < 1 {
			continue
		}
		if !definition.ExecutionPlacement.Normalize().IsValid() || definition.ExecutionPlacement == TaskExecutionPlacementCloud {
			continue
		}
		if err := validateSourceTaskDeclaredCapabilities(definition); err != nil {
			continue
		}
		if definition.TaskID == "" || len(definition.TaskID) > 256 || definition.ExtensionID == "" || len(definition.ExtensionID) > 256 || definition.ModuleID == "" || len(definition.ModuleID) > 256 || seen[definition.TaskID] || !validTaskFingerprint(strings.TrimPrefix(definition.EntryHash, "sha256:")) {
			return nil, "", NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录包含无效安装身份")
		}
		seen[definition.TaskID] = true
		if err := s.config.InstalledDefinitionValidator(ctx, definition); err != nil {
			continue
		}
		encoded, err := json.Marshal(definition)
		if err != nil || len(encoded) > 64<<10 {
			return nil, "", NewTaskError(ErrTaskDefinitionInvalid, "设备任务定义超过目录上限")
		}
		var item TaskDefinition
		if json.Unmarshal(encoded, &item) != nil {
			return nil, "", NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录无法解析")
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].TaskID < items[j].TaskID })
	encoded, err := json.Marshal(items)
	if err != nil || len(encoded) > 4<<20 {
		return nil, "", NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录正文超过上限")
	}
	if err := ctx.Err(); err != nil {
		return nil, "", context.Cause(ctx)
	}
	return items, hashBytes(encoded), nil
}

func (s *TaskRuntimeService) DescribeInstalledTaskCatalog(ctx context.Context, deviceID string, request DeviceTaskCatalogRequest) (DeviceTaskCatalogPage, error) {
	page := DeviceTaskCatalogPage{}
	if deviceID == "" || len(deviceID) > 256 || len(request.Cursor) > 1024 || request.Limit < 0 || request.Limit > 8 {
		return page, NewTaskError(ErrTaskInputInvalid, "设备任务目录分页参数无效")
	}
	capabilities := CurrentSourceTaskCapabilities()
	if err := validateSourceTaskCapabilities(capabilities); err != nil {
		return page, err
	}
	ctx = context.WithValue(ctx, sourceTaskPreflightKey{}, true)
	limit := request.Limit
	if limit == 0 {
		limit = 8
	}
	definitions, revision, err := s.installedTaskCatalog(ctx)
	if err != nil {
		return page, err
	}
	start := 0
	if request.Cursor != "" {
		encoded, err := base64.RawURLEncoding.DecodeString(request.Cursor)
		var cursor deviceTaskCatalogCursor
		if err != nil || len(encoded) > 512 || json.Unmarshal(encoded, &cursor) != nil || cursor.Revision != revision || cursor.AfterTaskID == "" {
			return page, NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录已变化，请重新加载")
		}
		found := false
		for index := range definitions {
			if definitions[index].TaskID == cursor.AfterTaskID {
				start, found = index+1, true
				break
			}
		}
		if !found {
			return page, NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录游标不属于当前安装")
		}
	}
	page.DeviceID, page.Revision = deviceID, revision
	page.Capabilities = capabilities
	page.Entries = make([]DeviceTaskCatalogEntry, 0, limit)
	end := min(start+limit, len(definitions))
	for index := start; index < end; index++ {
		definition := definitions[index]
		pin, err := s.DescribeInstalledTask(ctx, definition.TaskID, deviceID)
		if err != nil {
			return DeviceTaskCatalogPage{}, err
		}
		expected, err := taskDefinitionFingerprint(&definition)
		if err != nil || pin.DefinitionFingerprint != expected {
			return DeviceTaskCatalogPage{}, NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录读取期间已更新")
		}
		page.Entries = append(page.Entries, DeviceTaskCatalogEntry{Definition: definition, Target: pin})
	}
	_, current, err := s.installedTaskCatalog(ctx)
	if err != nil {
		return DeviceTaskCatalogPage{}, err
	}
	if current != revision {
		return DeviceTaskCatalogPage{}, NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录读取期间安装已变化")
	}
	if end < len(definitions) {
		encoded, _ := json.Marshal(deviceTaskCatalogCursor{Revision: revision, AfterTaskID: definitions[end-1].TaskID})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	return page, nil
}

func (s *TaskRuntimeService) DescribeInstalledTaskCatalogEntry(ctx context.Context, deviceID, taskID string) (DeviceTaskCatalogEntry, error) {
	entry := DeviceTaskCatalogEntry{}
	if taskID == "" || len(taskID) > 256 || deviceID == "" || len(deviceID) > 256 {
		return entry, NewTaskError(ErrTaskInputInvalid, "设备任务目录身份无效")
	}
	definition, err := s.store.GetTaskDefinition(ctx, taskID)
	if err != nil {
		return entry, err
	}
	if definition == nil || definition.RemoteSource != nil || !definition.ExecutionPlacement.Normalize().IsValid() || definition.ExecutionPlacement == TaskExecutionPlacementCloud || definition.InstalledGeneration < 1 {
		return entry, NewTaskError(ErrTaskDefinitionInvalid, "目标设备没有此已安装任务")
	}
	encoded, err := json.Marshal(definition)
	if err != nil || len(encoded) > 64<<10 || json.Unmarshal(encoded, &entry.Definition) != nil {
		return entry, NewTaskError(ErrTaskDefinitionInvalid, "设备任务定义超过目录上限")
	}
	entry.Target, err = s.DescribeInstalledTask(ctx, taskID, deviceID)
	if err != nil {
		return DeviceTaskCatalogEntry{}, err
	}
	fingerprint, err := taskDefinitionFingerprint(&entry.Definition)
	if err != nil || fingerprint != entry.Target.DefinitionFingerprint || entry.Definition.InstalledGeneration != entry.Target.InstalledGeneration {
		return DeviceTaskCatalogEntry{}, NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录读取期间已更新")
	}
	return entry, nil
}
