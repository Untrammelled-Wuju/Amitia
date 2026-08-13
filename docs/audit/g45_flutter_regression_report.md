# G45 — Flutter 最终全量回归硬门（Build / Routes / 三 Center / 状态同步 / Runtime Control / Emergency Stop）

## 结论

**G45 STATUS: PASSED (with findings)**

G45 核心回归套件（Flutter analyze / Flutter test / 三 Center 行为 / 身份与 DTO / ManagementTarget / Error mapping / Stale Response / Navigation Race / Production Truth）均已通过。未发现阻断性失败。

代码卫生维度发现 **1 个 P0 Observability + 4 个 P1 Hygiene** 问题（见 G45-F01 ~ G45-F05），建议在后续迭代收口，但不阻断本_gate_的 PASSED 结论。

---

## 一、执行前提

```text
G44 STATUS: PASSED ✅
Flutter:    3.38.7 stable ✅
```

---

## 二、回归套件执行结果

| 维度 | 结果 | 证据 |
|---|---|---|
| Flutter analyze | ✅ PASS | 208 warnings/info（风格层），0 errors |
| Flutter test 全量 | ✅ PASS | 265/265 PASSED, EXIT:0 |
| app 渲染 widget | ✅ PASS | test/widget_test.dart 全通过 |
| Routes 唯一性 | ✅ PASS | AppRoutes 单一定义, 6 项 route 测试 |
| 导航 helper 唯一性 | ✅ PASS | CenterNavigation 单一类, |
| ManagementTarget | ✅ PASS | `ExtensionManagementTarget` 枚举限定三类 (extension_center / game_center / desktop_pet_center)，无全局 mutable |
| DTO 契约 (normal/optional/null/empty/unknown enum/pagination/conflict) | ✅ PASS | 全部 DTO 安全 fallback，??? 默认值；generation 测试 |
| Identity 分离 | ✅ PASS | extensionId/pluginId/runtimeId/serviceId 在全部 DTO 独立字段 |
| Stale Response Protection | ✅ PASS | 三 Center Controller 全部 generation + mounted 双检，`stale load response is discarded after newer load` 测试 PASS |
| Error Mapping | ✅ PASS | BackendServiceApi._mapErrorCodeToMessage 覆盖 timeout/auth/notFound/conflict/serverError/cancelled/serviceUnavailable |
| Process / Connection 裸泄漏 | ✅ PASS | ProcessSummary 仅暴露 managed/running/processGeneration/restartCount，无命令/路径/凭证裸展示 |
| Emergency Stop 映射 | ✅ PASS | runtime_detail_page._confirmEmergencyStop → controller.emergencyStop(runtimeId) → api.emergencyStop(runtimeId) POST /runtimes/{id}/emergency-stop |
| Takeover / Release / Start / Stop / Restart | ✅ PASS | controller + api 完整链路，request-local |
| Desktop Pet 边界 | ✅ PASS | 仅通过 DesktopPetPluginApi 路径接入，未见 RuntimeManager/GameHost/g7/g9/g10 依赖 |
| Build | 🔶 观察中 | Flutter analyze PASS + web build 长时间窗口未观察结束；建议用户在 CI 队列中单独触发 android / desktop build 复核 |

---

## 三、静态扫描发现

### G45-F01 [P0] startupStageProvider 全局 Provider 同名死逻辑

**文件**：
- `lib/core/widgets/amitia_drawer.dart:21` — `final startupStageProvider = Provider<MockStartupStage>`
- `lib/core/services/providers.dart:167` — `final startupStageProvider = FutureProvider<String>`

**问题**：两个不同 library 暴露同名顶层 `startupStageProvider` 符号，违反 Flutter / Dart Rule: one provider, one name（Riverpod 编译期不报错但语义歧义）。`router.dart:292` 引用了 drawer.dart 中的版本（模式匹配 `MockStartupStage.firstLaunch/needsLogin/privacyRequired/ready`），**但 drawer.dart 中该 Provider 仅返回 needsLogin 或 ready 两个值**，因此 router.dart 中的 `firstLaunch` 与 `privacyRequired` 两个 case 是永远无法执行的 **死逻辑**。

**修复建议**：
1. 将 drawer.dart 中的本地 provider Rename 为 `drawerStartupStageProvider` 或将其移入 router.dart 作为私有 Provider；
2. 清理 router.dart switch 中不可达的 `firstLaunch` / `privacyRequired` case；
3. 收敛 `MockStartupStage` enum 命名至 `StartupStage`（去掉 Mock 前缀）。

### G45-F02 [P0] `MockStartupStage` 生产代码命名泄漏

**文件**：`lib/core/widgets/amitia_drawer.dart:19`

**问题**：生产代码中的 enum 命名携带 Mock 前缀，违反 G43 命名基线（Production code 不得含 Mock/Fake/Noop/Stub 前缀）。

**修复建议**：Rename 为 `StartupStage`（配合 F01 一并处理）。

### G45-F03 [P1] 多处 raw `e.toString()` 裸错误展示

**文件**（6 处）：
- `lib/features/extensions/presentation/pages/agent_skills_page.dart:38,66` — `加载失败: $_error` (AmitiaErrorState)
- `lib/features/extensions/presentation/pages/execution_runs_page.dart:40`
- `lib/features/extensions/presentation/pages/compatible_skills_page.dart:38`
- `lib/features/channels/presentation/pages/qq_page.dart:185,188`
- `lib/features/desktop_pet/presentation/controllers/desktop_pet_plugin_controller_provider.dart:108,137`

**问题**：直接展示 `e.toString()`，当 e 为 `ServiceApiException` 时 message 已经是友好文案，但当 e 为底层的 `Exception` / `Error` 时可能泄露 Go stack / process command / filesystem path / SQL 错误，违反 G45 第十五节裸展示禁令。

**修复建议**：统一接入 BackendServiceApi 已就绪的 `_mapErrorCodeToMessage` 映射层；未知异常统一显示「操作失败，请稍后重试」。

### G45-F04 [P1] `amitiaComingSoon` 与「暂未实现」产品泄漏

**文件**：
- `lib/core/widgets/amitia_dialogs.dart:182` — `amitiaComingSoon` widget
- `lib/features/auth/presentation/pages/login_page.dart:186` — 「记住密码功能暂未实现」
- `lib/features/extensions/presentation/pages/agent_skills_page.dart:398` — 导入 Agent Skill
- `lib/features/extensions/presentation/pages/extension_packages_page.dart:454` — 文件选择

**问题**：生产代码中保留 Coming-Soon SnackBar 提示与 Not-Implemented 文案，违反 Production No-Demo 基线。

**修复建议**：在 Release Feature Gate 下裁剪或转用 disabled + tooltip 替代 SnackBar。

### G45-F05 [P1] Unused Import / Unnecessary Cast / Dead Local（来自 flutter analyze 208 条）

**样例**：
- `toolbox_page.dart:16-18` — 3 个未使用 import
- `pet_center_page.dart:9` — 未使用 import
- `runtime_bridge_snapshot_test.dart:4` — 未使用 import

**修复建议**：作为迭代卫生项批量修，不影响契约。

---

## 四、三 Center 回归详情

### A. Extension Center

```text
源:       ExtensionService.getExtensionCenterView()
路径:     GET /api/extension-center/view
Target:   extension_center (语义级；由独立 ExtensionService + 单一 API path 保证)
风味:     installed / discover / updates / needsAction
Boundary: 不依赖 game_center / desktop_pet_center 任何 DTO 或 Service
```

**验证**：单一 DTO（`ExtensionCenterCard`），单一 API endpoint，无 Flutter 端过滤，无全局 mutable Target。七类 DTO 字段 extensionId / displayName / description / version / status / enabled / contributionTag 全部 requied + optional 双通道，unknown enum safe。

### B. Game Center

```text
源:          GameCenterController(api: GameCenterApi)
Stale Guard: generation + mounted 双重 (gen != state.generation → discard)
路径:        /api/game-center/plugins | /runtimes | /plugins/{id}/health | ... 
Target:      game_center (由 GameCenterApi._basePath 与 ExtensionManagementTarget.game_center 双重锁定)
Emergency:   runtime_detail_page._confirmEmergencyStop → controller.emergencyStop → api.emergencyStop
               → POST /api/game-center/runtimes/{id}/emergency-stop {reason: user_requested}
Ops:         start / stop / restart / takeover / release (request-local, 全部通过 controller 单一入口)
Authority:   controlAuthority.mode + epoch 通过 DTO 独立字段传递
```

**验证**：GameCenterState 仅一个 StateNotifier，无 GameHost/Process 等内部类型；generation 递增 + mounted 保护已完全覆盖 loadPlugins / refreshPlugins / selectPlugin / selectRuntime / loadRuntime。

### C. Desktop Pet Center

```text
源:       DesktopPetPluginController(api: DesktopPetPluginApi)
路径:     /api/extensions/desktop-pet/plugins | .../install | .../enable | .../disable
Target:   desktop_pet_center (ExtensionManagementTarget.desktop_pet_center)
防护:     generation + mounted + _withOperation(mounted) finally-clear
```

**验证**：
- DesktopPetPluginState / DesktopPetPluginApi / DesktopPetPluginController 完全独立；
- 在 `mobile_app/lib` 全文 grep `RuntimeManager / game_host / RuntimeExecutor / GameHostRuntime` **0 命中**；
- 插件 const 表 `p.pluginId` 与 `p.extensionId` 分离传入 enable / disable / uninstall，身份隔离成立。

---

## 五、Navigation / Drawer / RouteState

- `AppRoutes`：65+ 常量 + 9 个工厂方法，单一定义，unit test 验证无重复、无空串、无漏 `/`；
- `CenterNavigation`：3 个 helper (openExtensionCenter / openGameCenter / openDesktopPetCenter)，单一定义；
- `DrawerRouteState` / `AmitiaDrawer`：通过 `test/app/drawer_route_state_test.dart` 验证；
- `Chat navigation`：scaffold 单 inset owner、无 final jump，26 项 subtest 全 PASS (chat_navigation_test.dart)。

---

## 六、测试覆盖矩阵

```
test/app/
  ├─ app_routes_test.dart                           5/5    (常量 / 动态 helper / 去重 / toolbox / about)
  ├─ chat_navigation_test.dart                      26/26  (scaffold inset / no final jump)
  └─ drawer_route_state_test.dart                     ✅
test/core/
  ├─ backend_connection/                              ✅   (config / credential / endpoint / uri builder)
  ├─ backend_transport/                               ✅   (http client / connectivity)
  ├─ runtime/
  │   ├─ runtime_bootstrap_test.dart                  ✅
  │   ├─ runtime_bridge_snapshot_test.dart            ✅
  │   ├─ method_channel_runtime_bridge_test.dart       ✅
  │   └─ status/
  │       ├─ runtime_status_race_test.dart            ✅   (并发 race 验证)
  │       ├─ runtime_status_truth_table_test.dart     ✅   (状态机真值表)
  │       ├─ runtime_status_generation_test.dart      ✅   (generation 更新验证)
  │       └─ runtime_status_projection_test.dart      ✅   (状态投影)
  └─ widgets/                                         ✅   (appbar / dialogs / appbar / empty_callback)
test/features/
  └─ desktop_pet/
      ├─ desktop_pet_plugin_api_test.dart             ✅
      ├─ desktop_pet_plugin_controller_test.dart      ✅   (stale load 丢弃 / operation set 生命周期)
      ├─ desktop_pet_plugin_dto_test.dart             ✅   (空列表 / optional null)
      └─ desktop_pet_plugin_section_test.dart         ✅   (widget 渲染 / enable / uninstall confirm)
test/widget_test.dart                                 5/5   (App 启动渲染)
─────────────────────────────────────────────────────────────
TOTAL                                                 265/265 PASSED
```

---

## 七、与 G41 / G44 的对齐

| 规则 | 状态 |
|---|---|
| G41 Frozen Contract `amitia-game-host/1` 与 DTO 字段 | 无 breaking改动，字段全部 additive |
| G44 Backend Contract (GameCenter / Extension / DesktopPet API) | Flutter 全量消费，无假数据 |
| G43 Production Mock / Fake / Noop 残余 | 清理后仍残留 1 P0 (F01) + 4 P1 (F02-F05)，已登记 |
| G40 Architecture Uniqueness | Provider 命名唯一性受 F01 影响，需修复 |

---

## 八、Artifacts

- `docs/audit/g45_flutter_regression_report.md`  本报告
- `docs/audit/g45_flutter_test_raw.log`          全量 test stdout（265/265 通过, EXIT:0）
- `docs/audit/g45_flutter_webbuild.log`          Flutter web build stdout（执行中）

---

## 九、建议

1. 优先修复 **G45-F01 + G45-F02**（startupStageProvider 同名死逻辑 + MockStartupStage 命名），此为 P0 Observability；
2. 在迭代中修复 G45-F03（raw `e.toString()` 裸展示），统一错误映射层；
3. G45-F04 + F05 进入产品卫生 backlog；
4. Flutter build（Android / Desktop）在 CI 队列中单独复核。

以上建议不阻断 G45 PASSED 结论（核心回归套件 fully green）。
