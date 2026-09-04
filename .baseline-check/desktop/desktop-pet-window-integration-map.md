# Amitia 第 5 步·Electron 桌宠窗口重构 — 集成映射与现状审计

- 文档版本:1.0
- 编写日期:2026-07-30
- 对应规范:`Amitia桌宠系统_第5步_Electron桌宠窗口重构完整实施文档.md`
- Electron 版本:`^43.0.0`(package.json)
- 审计性质:只读,未修改任何源码

## 一、关键前提澄清

规范文档基于「上传基线 backend-src(2).zip 只含 Go 后端、不含 Electron 源码」的假设,认为「桌面执行端仍缺失」。实际项目现状与该假设**存在重大偏差**,必须在动工前明确:

1. **Electron 端已存在可工作的桌宠实现**:`desktop/src/main/pet/` 下已有 16 个模块(约 5400 行),`DesktopPetManager`(manager.ts,1937 行)以 **HTTP REST 胖客户端**方式直接驱动后端,本地自主管理窗口生命周期、动作播放、点击穿透、拖拽、位置持久化与崩溃恢复。该实现**不符合**第 5 步架构要求(无八态状态机、无 Registry、无 Runtime Client、sandbox 未启用、无 full 穿透模式、无 long_press、无 PetVisualSurface)。
2. **第 4 步 Runtime Bridge 协议在后端「已写未接线」**:`backend/internal/desktoppet/contracts/runtime_protocol.go` 与 `backend/internal/desktoppet/runtime/`(9 个文件)有完整协议契约与核心组件,但 **WS 路由未挂载**(`cmd/server/router.go:92-94` 未注册)、**无命令下发编排层**(dispatcher)、**状态上报不落库**(`handler.go` 只打日志)、**三张运行时表迁移未注册**且 **baseline.sql 缺失**。Electron 端**完全没有** Runtime Client。
3. 因此第 5 步「消费第 4 步 Runtime Bridge 协议」这一前置条件**尚未满足**。第 5 步实际工作量 = 补第 4 步接线层 + 重构 Electron 为 Runtime Client 架构 + 后端 Settings 修复 + 迁移 + 测试。

## 二、第 5 步文档 5.1 节要求定位的现有文件(定位结果)

| 规范要求定位项 | 实际项目文件 | 现状摘要 |
|---|---|---|
| Electron 主进程启动入口 | `desktop/src/main/index.ts` | `app.whenReady`→`enterMainApp()`;单实例锁;`before-quit` 走 `shutdownInProgress` |
| 主窗口创建函数 | `desktop/src/main/window.ts` | 主窗口 `webPreferences`:contextIsolation=true、nodeIntegration=false、**sandbox=false**、webSecurity=true |
| App 生命周期与单实例锁 | `desktop/src/main/index.ts:68-110,246-273` | `requestSingleInstanceLock`;`second-instance` 恢复窗口;退出前 `desktopPetManager.shutdown()` |
| preload 构建入口 | `desktop/src/preload/index.ts` → `dist/preload/index.cjs` | 暴露 `window.amitiaDesktop`(约 30 方法)+ `window.electronWindowApi`;**pet 窗口与主窗口共享同一 preload** |
| Renderer 路由和静态资源构建 | `desktop/vite.config.ts`(Renderer root=`../front`) | Vite + vite-plugin-electron;dev 端口 5178 代理 `/api` 到 18899 |
| CSP 与自定义协议注册 | `desktop/src/main/protocol.ts` | `amitia-extension://` 协议:路径遍历防护、MIME 白名单、CSP 注入;**主窗口无 CSP** |
| 自动更新与重启 | `desktop/src/main/update-manager.ts` | electron-updater,generic provider→`https://amitia.untrammelled.top/amitia` |
| 用户数据目录解析 | `desktop/src/main/path-manager.ts`、`config-store.ts` | `app.getPath('userData')` + 部署配置 |
| 日志系统 | `desktop/src/main/logger.ts`、`desktop/src/main/pet/logger.ts` | electron-log;pet logger 含敏感信息脱敏 |
| WebSocket 客户端依赖 | **无**(当前用 HTTP fetch) | Runtime Bridge WS 客户端完全缺失 |
| 测试框架和 E2E 工具 | `desktop/src/main/pet/__tests__/`(Vitest) | 仅 ResourceLoader/ActionScheduler/ActionPlayer/WindowAdapter 有单测;核心控制器无测试 |
| TypeScript 配置及路径别名 | `desktop/tsconfig.json` | target ES2022、strict、module ESNext、Bundler |
| 打包工具配置 | `desktop/electron-builder.yml`、`desktop/vite.config.ts` | electron-builder v26,compression normal,differentialPackage true |

## 三、规范逻辑文件 → 实际项目文件映射

规范建议的 `<electron-root>/desktop-pet/` 逻辑结构,映射到现有 `desktop/src/main/pet/` 与新增模块。原则:**原位增强现有模块,禁止并行保留两套实现**。

### 3.1 已有模块(原位增强)

| 规范逻辑文件 | 现有文件 | 现状 → 需补强 |
|---|---|---|
| `window/desktop-pet-window-manager.ts` | `pet/manager.ts`(1937 行) | 胖客户端 HTTP 驱动 → 需引入状态机、Registry、命令路由,改为 Runtime Client 驱动 |
| `window/window-registry.ts` | **无** | 需新增:petInstanceId/installationId/browserWindowId 三向索引 |
| `window/window-factory.ts` | `pet/window-adapter.ts`(492 行,内含创建) | 创建参数基本符合;需抽出 Factory,集中 BrowserWindow 创建;**sandbox 改 true** |
| `window/lifecycle-machine.ts` | **无** | 现有仅 5 态 `uninitialized/ready/enabled/disabled/invalid` → 需八态机 |
| `window/recovery-supervisor.ts` | `pet/manager.ts` 内 `scheduleRecovery/recoverRuntime` | 有 800ms 防抖+布尔锁 → 需滑动窗口限流(10min/3 次)+指数退避+failed 态 |
| `geometry/coordinate-space.ts` | `pet/window-adapter.ts` 部分 | 未显式区分 DIP → 需统一 DIP 语义 |
| `geometry/display-topology.ts` | `pet/window-adapter.ts` `listScreens/findScreenById` | 有多显示器支持 → 需补 display-added/removed/metrics-changed 防抖处理 |
| `geometry/display-fingerprint.ts` | **无** | 仅用 `display.id` → 需指纹(label+尺寸+scaleFactor+rotation+relative) |
| `geometry/bounds-resolver.ts` | `pet/window-adapter.ts` 部分 | 需抽出纯函数:recommended DIP、scale、placement |
| `geometry/bounds-clamper.ts` | `pet/window-adapter.ts` `ensureVisible`(VISIBLE_MARGIN=20) | 有基础夹取 → 需补最低可见面积(25%)、负坐标、竖屏 |
| `geometry/position-persistence.ts` | `pet/manager.ts` `persistRuntimePosition` + `pet/drag-controller.ts` | 防抖 500ms → 需改为 revision CAS + 拓扑恢复写 + 退出 flush |
| `interaction/click-through-controller.ts` | `pet/click-through-controller.ts`(110 行) | alpha 轮询模式(16ms cursor 轮询) → 需补 full 模式、迟滞阈值、Renderer 上报模式 |
| `interaction/alpha-hit-test.ts` | `pet/click-hit-test.ts`(127 行) | 有 alpha 数据命中 + boundingBox → 需补 contain/letterbox 偏移、generation token |
| `interaction/pointer-gesture-machine.ts` | `pet/event-bridge.ts`(242 行) | 有 click/double_click/hover(300ms/3000ms) → **缺 long_press**;需 DIP 阈值、cancel 清理 |
| `interaction/drag-controller.ts` | `pet/drag-controller.ts`(240 行) | Main 驱动、读真实 bounds → 基本符合;需补 cancel 路径、alpha 锁恢复 |
| `interaction/interaction-events.ts` | `pet/event-bridge.ts` 部分 | 需补 UUID/sequence/screenPoint/displayFingerprint |
| `preload/desktop-pet-preload.ts` | **无(共享主 preload)** | 需新增专用 preload:仅暴露 ready/visual-ready/hit-state/pointer/snapshot |
| `renderer/pet-visual-surface.ts` | **无** | 需新增接口 + 静态实现(preview/首帧 + alpha mask) |
| `renderer/static-visual-surface.ts` | **无** | 当前帧通过 IPC dataURL 直送 renderer → 需 PetVisualSurface 适配 |
| `renderer/alpha-mask.ts` | `pet/click-hit-test.ts` 部分 | 需移到 renderer 侧,低分辨率采样 |
| `diagnostics/snapshot.ts` | **无** | 需新增只读诊断快照 |
| `diagnostics/dev-harness.ts` | **无** | 需新增测试命令面板(仅 dev 构建) |

### 3.2 需新增模块(Electron 端)

| 规范逻辑文件 | 说明 |
|---|---|
| `contracts/commands.ts` `events.ts` `settings.ts` `geometry.ts` `capabilities.ts` | 共享 TypeScript 类型,放 `desktop/src/main/pet/contracts/` 或 `desktop/src/shared/pet/` |
| `runtime/runtime-client.ts` | **核心缺口**:WS 连接 `/internal/desktop-pet/runtime/ws` + register/welcome 握手 + 心跳 + 重连 |
| `runtime/command-router.ts` | 命令映射:pet.spawn→ensureInstance 等 |
| `runtime/actual-state-reporter.ts` | 上报 PetInstanceSummary + 心跳 |
| `runtime/reconnect-controller.ts` | resumeSessionId + fullSyncRequired reconcile |
| `platform/platform-window-adapter.ts` | Windows/macOS/Linux 置顶/穿透/焦点差异封装 |

## 四、后端现状与差距(规范第三章 3.2~3.10)

RuntimeSettings 实际归属:`backend/internal/desktoppet/installation/`(非顶层 `desktoppet/`)。

| 规范问题编号 | 问题 | 现状 | 修复方向 |
|---|---|---|---|
| 3.2 | click_through_mode 默认值冲突 | 迁移 SQL 默认 `'alpha'`,`service.go` 常量 `"off"` | 统一为单一值 + 迁移历史数据 |
| 3.3 | 设置更新缺类型/范围校验 | `filterRuntimeSettingsUpdates` 仅校验列名(snake_case 白名单),无类型/范围 | 强类型 DTO + 校验 |
| 3.4 | 设置接口未接受 camelCase | JSON 输出 camelCase,PATCH 仅接受 snake_case | 边界归一化层 |
| 3.5 | UpdateRuntimeSettings 缺用户归属 | 无 userId 参数,仅校验 installation 存在 | 补 userId + `inst.UserID != userId` |
| 3.6 | Recenter 缺用户归属 | 同上 | 同上 |
| 3.7 | Recenter 写 0 无几何语义 | `position_x=0/position_y=0/screen_id=""` | 改命令语义,Electron 计算后回报 |
| 3.8 | screen_id 不稳定 | 仅 `String(display.id)` | 补 display_fingerprint + 归一化位置 |
| 3.9 | 无窗口宽高/视觉尺寸 | 仅 scale | 补 last_window_width/height + recommended DIP |
| 3.10 | 无位置 revision | 无 CAS | 补 settings_revision + CAS |
| — | 缺实际窗口状态字段 | 无 | 由第 4 步 actual_states 表承载(需接线) |

### 后端需新增字段(desktop_pet_runtime_settings)

`settings_revision`、`restore_on_app_start`、`position_mode`、`display_fingerprint`、`relative_x`、`relative_y`、`last_window_width`、`last_window_height`、`position_updated_at`(当前全部不存在)。

### 后端 Runtime Bridge 接线缺口(第 4 步未完成)

| 缺口 | 位置 | 说明 |
|---|---|---|
| WS 路由未挂载 | `cmd/server/router.go:92-94` | 未注册 `runtime.Handler.ServeHTTP`,`/internal/desktop-pet/runtime/ws` 不可达 |
| runtime 包未实例化 | `cmd/server/services.go:439-468` | 未 `NewAuth/NewRegistry/NewPendingTracker/NewHandler/NewCommandStore/NewStateStore` |
| 无命令下发编排层 | runtime 包内无 dispatcher | 无代码构造 `pet.spawn` 等命令并 `Send` |
| 状态上报不落库 | `runtime/handler.go:207-215` | `HandleEvent/HandleHeartbeat` 只打日志,未调 `state_store.UpsertActualState` |
| 三表迁移未注册 | `migration/migrations.go` `DefaultMigrations()` | `DesktopPetRuntimeClientsMigration`/`CommandsMigration`/`ActualStatesMigration` 文件存在但未注册 |
| 三表 baseline 缺失 | `migration/baseline.sql` | actual_states/clients/commands 三张 CREATE TABLE 缺失 |

## 五、安全配置现状(规范第十五章)

| 配置项 | 主窗口 | 桌宠窗口 | 规范要求 | 差距 |
|---|---|---|---|---|
| contextIsolation | true | true | true | ✅ |
| nodeIntegration | false | false | false | ✅ |
| sandbox | **false** | **false** | true | ❌ P0 |
| webSecurity | true | true | true | ✅ |
| IPC 白名单 | 无 | 无 | 有 | ❌ P0 |
| 专用 pet preload | 共享主 preload | 共享主 preload | 独立最小 API | ❌ P1 |
| 主窗口 CSP | 无 | — | 有 | ❌ P1 |
| 本地资源协议 | `amitia-extension://` | — | 受控协议 | ✅(可复用) |
| token 获取 | `executeJavaScript` 读 localStorage | — | Main 持有 | ❌ P1 |

## 六、实施顺序建议(对照规范第二十三章)

1. **后端基础先行**:注册三表迁移 + 补 baseline + Settings 强类型/归属/revision/迁移 → 确保 desired/actual 数据模型可用
2. **第 4 步接线**:挂载 WS 路由 + 实例化 runtime 组件 + dispatcher + 状态落库 → 让协议真正跑通
3. **Electron contracts + 状态机 + Registry + Factory**:原位改造 manager.ts,引入八态机,Factory 集中创建,sandbox=true
4. **Geometry/Display/DPI**:补 fingerprint、归一化位置、越界夹取、拓扑事件防抖
5. **Preload/Renderer 静态视觉**:专用 preload + PetVisualSurface 静态实现 + alpha mask
6. **Click-through + 手势**:补 full 模式、long_press、迟滞、事件去重
7. **Runtime Client + 命令映射 + actual state 上报**:WS 连接、reconcile、revision
8. **Recovery + diagnostics + E2E**:限流熔断、诊断快照、测试用例
9. **交接文档**:第 13 步交接清单

## 七、已知限制与环境约束

- 当前开发环境为 Windows,macOS/Linux 验收需标记 capability degraded
- 现有 pet 模块是可工作状态,重构期间需保证可编译、可回滚
- 前端有热更新,后端改 Go 需重启;Electron main 改动需重启 Electron
