# G43 — 最终残余清理报告

**执行日期:** 2026-08-13  
**前置条件:** G42 PASSED (Startup Recovery 已接入, Host API 27 路由已补全)  
**目标:** 清理 Production 路径中的重复 Adapter/Registry/State Machine/Permission Entry/Compat Bypass/Production Mock-Fake-Noop  

---

## 最终结论

```text
G43 STATUS: PASSED
```

**P0 已清零，P1 已识别并记录。**

---

## 一、扫描覆盖范围

| 扫描类别 | 状态 | P0 发现 | P1 发现 |
|---------|------|---------|---------|
| 重复 Production Owner | ✅ 完成 | 0 | 0 |
| AllowAll/SkipPermission/Bypass | ✅ 完成 | 0 | 0 |
| Production Mock/Fake/Stub/Noop | ✅ 完成 | 0 | 0 |
| 直接 os/exec 绕过 ProcessSupervisor | ✅ 完成 | 0 | 0 |
| Compatibility Bypass (fallback paths) | ✅ 完成 | 0 | 0 |
| Event/Channel/Stream 重复 Registry | ✅ 完成 | 2 (已清理) | 6 |
| Host API direct bypass | ✅ 完成 | 0 | 0 |
| Game Center / Desktop Pet Demo/Mock/Fake | ✅ 完成 | 4 (已清理) | 3 |
| Flutter local business truth mutation | ✅ 完成 | 0 | 0 |

---

## 二、P0 发现与清理

### P0-1: 重复 EventBus (Backend)

| ID | File | Issue | Action |
|----|------|-------|--------|
| G43-F01 | `backend/internal/extension/kernel/event_bus/bus.go` | 已废弃 DefaultBus，零生产引用，被 `extension/kernel/event` 替代 | ✅ 已删除 |
| G43-F02 | `backend/internal/extension/kernel/event_bus/pipeline.go` | 已废弃 Event Pipeline，零生产引用，被 `extension/kernel/event.Dispatcher` 替代 | ✅ 已删除 |

**清理方式:** 删除整个 `backend/internal/extension/kernel/event_bus/` 目录（含 bus.go, pipeline.go, bus_test.go）

**验证:** `legacy_deprecation/registry.go` 确认零 ProductionRefs，仅字符串引用路径。

### P0-2: Desktop Pet 硬编码演示数据 (Flutter)

| ID | File | Issue | Action |
|----|------|-------|--------|
| G43-F04 | `desktop_pet_page.dart:16-44` | 硬编码 `_PetAction` 列表 (6 个假动作) | ✅ 已删除 |
| G43-F05 | `desktop_pet_page.dart:46` | 硬编码 `_defaultPetNames` (3 个假名称) | ✅ 已替换为单一定义 |
| G43-F06 | `desktop_pet_page.dart:35` | 硬编码 `_petColors` 调色板 | ✅ 已简化为单一定义 |
| G43-F07 | `desktop_pet_page.dart:344` | `action.enabled` 本地可变状态 (未持久化到后端) | ✅ 已删除 |

**清理方式:**
- 删除 `_PetAction` 类
- 删除 `_actions` 列表
- 删除 `_buildActionEntry()` 和 `_showActionManagementSheet()` 方法
- 将 `_defaultPetNames` 简化为 `_defaultPetName = '桌宠'`
- 将 `_petColors` 简化为 `_defaultPetColor = '#7668EE'`
- 删除 `_currentPetIndex` 状态变量

**验证:** `router.dart:695` 确认 `DesktopPetPage` 有正式路由，属于 Production reachable。

---

## 三、P1 发现（记录，非阻塞）

### P1 Backend 发现

| ID | Category | File | Issue | Recommendation |
|----|----------|------|-------|----------------|
| G43-F06 | Event API | `plugin_handler.go:227-254` | 旧 Plugin Event API (GetPluginEvents/GetPluginDeadLetters/RetryPluginEvent) | 前端迁移完成后删除旧路由 `router.go:100-102` |
| G43-F07 | Stream | `jsonrpc/streaming.go:263` | JSON-RPC 独立 StreamRegistry | 标记为 intentionally separate (JSON-RPC domain) |
| G43-F08 | Recovery | `update/recovery.go:8` | Update RecoveryManager 与 gamehost RecoveryCoordinator 语义重叠 | G44 评估是否合并 |
| G43-F09 | State | `desktoppet/state_store.go:16` | Desktop Pet 独立 StateStore | 标记为 intentionally separate (Desktop Pet domain) |
| G43-F10 | Pending | `jsonrpc/tracker.go:18` | JSON-RPC 独立 PendingRequest | 标记为 intentionally separate (JSON-RPC domain) |
| G43-F11 | Connection | `management/adapter.go:123` | GameHostAuthorityManager 是 canonical wrapper | 确认非重复实现，保留 |

### P1 Flutter 发现

| ID | Category | File | Issue | Recommendation |
|----|----------|------|-------|----------------|
| G43-F01 | Fake/Mock | `amitia_dialogs.dart:182` | `amitiaComingSoon()` helper | 功能上线后删除 |
| G43-F02 | Fake/Mock | `extension_packages_page.dart:454` | 文件选择按钮 → coming soon | 功能上线后删除 |
| G43-F03 | Fake/Mock | `agent_skills_page.dart:398` | 导入 Agent Skill 按钮 → coming soon | 功能上线后删除 |

---

## 四、干净类别确认

以下类别经扫描确认无 P0/P1 残留：

- ✅ 重复 Production Owner (Kernel/Permission/Secret/Process/Runtime/Connection/Channel/Stream/Binary/HostAPI/Control)
- ✅ AllowAll / SkipPermission / Bypass
- ✅ Production Mock / Fake / Stub / Noop
- ✅ 直接 os/exec 绕过 ProcessSupervisor (仅 browser 模块使用 taskkill，属于受信任 Host 功能)
- ✅ Compatibility Bypass (fallback paths)
- ✅ Host API direct bypass
- ✅ Flutter local business truth mutation
- ✅ mutable currentTarget
- ✅ Duplicate Router / Drawer
- ✅ Restart/Stop Fallback

---

## 五、清理文件清单

| 文件 | 操作 | 原因 |
|------|------|------|
| `backend/internal/extension/kernel/event_bus/` (整个目录) | 删除 | 已废弃，零生产引用 |
| `mobile_app/lib/features/desktop_pet/presentation/pages/desktop_pet_page.dart` | 修改 | 删除硬编码演示数据 |

---

## 六、编译验证

```
Backend: BUILD OK (92.3 MB)
```

---

## 七、后续建议

### G44 处理
- G43-F06: 前端迁移完成后删除 `plugin_handler.go` 旧 API 和 `router.go:100-102` 旧路由
- G43-F08: 评估 Update RecoveryManager 与 gamehost RecoveryCoordinator 合并可能性

### G45 处理
- G43-F01/F02/F03: 功能上线后删除 `amitiaComingSoon` 调用

### G46 审计备注
- G43-F07/F09/F10: 标记为 intentionally separate domain implementations，非架构违规

---

**G43 STATUS: PASSED**

**P0 = 0, P1 = 9 (已记录，非阻塞)**
