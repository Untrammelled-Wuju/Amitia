# DSH 插件兼容与 Amitia 通用 UI 扩展开发方案

状态：执行版

日期：2026-11-13

目标 DSH ABI：`0.2.2-rc.2`

容器插件：`com.amitia/dsh-compatible-host`

源码目录：`plugins-code/dsh-compatible-host`

发行目录：`plugin/`

当前结论：方案可行。当前已经完成通用 UI 扩展底座和 DSH 兼容容器的主体骨架，但尚未达到目标版本 100% 兼容，不能提前宣称完成。

## 1. 执行摘要

本项目最终交付一个普通、独立的 Amitiax 插件包，用户只需在 Amitia 中安装 `com.amitia/dsh-compatible-host`。DSH 插件随后安装在该容器的私有插件数据目录中，由容器完成 DSH Host ABI、Client ABI、服务、事件、工具、提示词、Session、Agent、Subagent、MCP 和 UI 到 Amitia 通用插件能力的映射。

最终架构必须同时守住四条边界：

1. Amitia 核心只提供通用宿主能力，不增加 DSH 专用路由、页面、Adapter、数据库表或业务分支。
2. 不运行 DeepSeek Harness 本体。
3. 不把 Cordis 作为 Amitia 核心依赖，也不运行 Cordis 引擎。
4. 所有 DSH 专用逻辑都封装在普通 Amitiax 容器插件内部。

“UI 补齐万物皆可扩展”是 DSH UI 全量兼容的必要条件，但不是 DSH 插件 100% 兼容的充分条件。完成 UI 底座后，仍必须完成 Host ABI、Client ABI、服务深映射、包生命周期、状态恢复、错误链、官方夹具和真实安装验证。

100% 兼容只能绑定固定 DSH 版本和明确交付平台。当前冻结目标为 `0.2.2-rc.2`；在该版本的完整公开 ABI、UI Slot、生命周期和测试矩阵全部闭环前，项目状态只能标记为“部分兼容”。

## 2. 核心需求

### 2.1 用户核心要求

| 编号 | 要求 |
| --- | --- |
| UR-01 | 交付物必须是普通、独立的 Amitiax 插件包，不是系统内置插件。 |
| UR-02 | 用户只安装一个 Amitiax 容器插件，DSH 插件在该容器内部安装和管理。 |
| UR-03 | DSH 子插件不能注册成独立 Amitia 扩展，不能出现在 Amitia 扩展中心。 |
| UR-04 | DSH Host API、Client ABI、服务、事件、工具、Session、Agent、Subagent、MCP 和 UI 都要映射到 Amitia。 |
| UR-05 | DSH 的 React UI 和 Slot UI 要能够进入 Amitia 页面、聊天区、侧栏、输入区、消息区、设置、工具结果和浮层。 |
| UR-06 | Amitia 只提供通用宿主能力，DSH 映射由独立容器插件完成。 |
| UR-07 | UI 侧最终达到“所有宿主明确声明的 Surface 均可扩展”。 |
| UR-08 | 最终达到固定 DSH 版本的 100% 兼容，不接受“能加载几个示例”作为完成标准。 |
| UR-09 | 不运行 DeepSeek Harness 本体。 |
| UR-10 | 不把 Cordis 引入 Amitia 核心。 |
| UR-11 | Android、iOS 与桌面端使用同一个普通 Amitiax 容器和同一 DSH ABI；手机端也必须能在容器内安装、启停和卸载 DSH 插件及扩展页面。 |

### 2.2 功能需求

| 编号 | 需求 | 验收定义 |
| --- | --- | --- |
| FR-01 | 普通容器安装 | 扩展中心只安装 `com.amitia/dsh-compatible-host`。 |
| FR-02 | DSH 子插件内部安装 | DSH 包只进入容器 `plugin_data`，不修改已安装 `.amitiax` 内容。 |
| FR-03 | 多来源包管理 | 支持固定版本 npm、本地 tarball、本地目录和固定 Git 来源。 |
| FR-04 | 多入口包 | 支持 DSH bundle、多个 Host 子路径和多个 Client 子路径。 |
| FR-05 | Host ABI | 支持 `apply(ctx)`、effect、事件、waterfall、bail、tools、服务注入和 disposer。 |
| FR-06 | Client ABI | 支持 ModuleLoader、factory、require、React、Slot、locale、store、inject、hooks 和 children。 |
| FR-07 | 提示词映射 | 支持 `systemPrompt.section/context/tools/variable` 进入 Amitia 提示词链路。 |
| FR-08 | Session 映射 | 支持会话读取、事件追加、事件持久化、projection 重算和恢复。 |
| FR-09 | Agent 映射 | 支持 get、resume、followup、steer、inject、cancel 和模型选择。 |
| FR-10 | Subagent 映射 | 支持 provider registry、activation、children、interrupt、send 和等待语义。 |
| FR-11 | MCP 映射 | 支持 stdio/HTTP MCP、工具发现和调用、resources、prompts、sampling、elicitation。 |
| FR-12 | 工具映射 | 支持 DSH 工具 schema 转换、动态 Amitia Tool contribution、调用、取消和错误传播。 |
| FR-13 | UI 映射 | 每个目标 DSH Slot 都有唯一 Amitia Slot、平台状态和自动化测试。 |
| FR-14 | HOC 和 children | 支持多层包裹、owner content 透传、顺序、遮蔽和重挂载。 |
| FR-15 | Portal | 支持 overlay、modal、drawer、toast、menu、context menu 和生命周期清理。 |
| FR-16 | 插件自定义 Slot | 普通插件可声明 Slot，其他插件可注册、组合并在停用时递归清理。 |
| FR-17 | 子插件隔离 | 一个 DSH 插件失败不影响 Amitia 核心和其他 DSH 插件。 |
| FR-18 | 完整生命周期 | 支持安装、启用、停用、更新、回滚、崩溃恢复和卸载。 |
| FR-19 | 状态恢复 | 重启后恢复子插件、Session、配置和 UI 状态，且不重复注册。 |
| FR-20 | 兼容矩阵 | 可查询每项 ABI、Slot、平台和测试的当前覆盖状态。 |
| FR-21 | 原始错误 | API、任务、流式响应、渲染和安装失败均保留完整原始错误。 |
| FR-22 | 独立版本 | Amitia、容器插件、目标 DSH ABI 和 DSH 子插件版本彼此独立。 |
| FR-23 | 移动端同能力 | Android/iOS 支持容器安装、DSH 子插件安装、Host 映射、Client UI、启用、停用、恢复和卸载，功能状态必须与桌面一致。 |
| FR-24 | 移动端 UI 映射 | DSH React UI 在移动 WebView Runtime 中执行，并通过 Flutter 通用 Slot、Surface、Portal 和资源面板宿主进入 Amitia 页面。 |

### 2.3 非功能需求

| 编号 | 需求 | 验收定义 |
| --- | --- | --- |
| NFR-01 | 稳定性 | 容器或单个 DSH 插件失败不得阻止 Amitia 核心启动。 |
| NFR-02 | 隔离性 | DSH 包、依赖、状态、日志和缓存不进入 Amitia 核心业务数据。 |
| NFR-03 | 可恢复性 | 崩溃、重启和更新失败后可恢复到一致状态。 |
| NFR-04 | 性能 | 安装和停用不阻塞宿主主线程；Slot、贡献、模块和 React Root 有数量预算。 |
| NFR-05 | 可观测性 | 日志包含插件 ID、版本、入口、阶段、调用 ID、会话 ID 和原始错误。 |
| NFR-06 | 可维护性 | DSH 专有逻辑只存在于 `plugins-code/dsh-compatible-host`。 |
| NFR-07 | 可测试性 | 每个映射都能用单元测试、夹具、集成测试或真实运行证据验证。 |
| NFR-08 | 可打包性 | 源码只从 `plugins-code/` 构建，发行物只输出到 `plugin/`。 |
| NFR-09 | 可审计性 | ABI Catalog、Slot Catalog、测试报告和兼容矩阵随版本保存。 |
| NFR-10 | 错误真实性 | 不吞错、不截断、不用固定文案覆盖原始错误、不返回假成功。 |
| NFR-11 | 平台一致性 | Manifest 声明的平台必须与实际能力、测试证据一致。 |
| NFR-12 | 扩展一致性 | 新增宿主 Surface 通过通用 Slot 完成，不新增 DSH 专用核心分支。 |
| NFR-13 | 移动端一致性 | 同一能力在手机端的支持、失败、清理和原始错误状态必须可查询，禁止静默禁用或伪装成功。 |

## 3. 100% 兼容的严格定义

本项目中的“100% 兼容”必须满足以下全部条件：

1. 只针对明确冻结的 DSH 版本 `0.2.2-rc.2`。
2. 覆盖该版本全部公开 Host Plugin API。
3. 覆盖该版本全部公开 Client Plugin API。
4. 覆盖该版本全部公开 UI Slot 和 UI 上下文。
5. 覆盖官方包、bundle、多入口、依赖、locale、配置和生命周期。
6. 覆盖安装、启用、停用、更新、回滚、恢复和卸载。
7. 覆盖工具、事件、Hook、服务、提示词、Session、Agent、Subagent 和 MCP。
8. 覆盖所有声明支持的平台和宿主形态。
9. 所有公开 ABI 均至少有一条成功路径、一条失败路径和一条清理路径证据。
10. 不存在被默认回退、启发式猜测或空实现掩盖的未映射能力。

不纳入 100% 兼容口径的内容：

1. 非目标 DSH 版本的历史 ABI。
2. DSH Harness 本体自身的主进程、根界面和内部实现细节。
3. 目标 DSH 版本未公开的私有模块。
4. 绕过 Amitia 生命周期、权限和贡献系统直接操纵宿主 DOM。
5. 未在容器 Manifest 中声明支持的平台。

### 3.1 兼容等级

| 等级 | 能力 | 可以对外表述 |
| --- | --- | --- |
| C0 | 能发现 DSH 包 | 已识别 |
| C1 | 能安装、启停、卸载 | 可管理 |
| C2 | Host ABI 可执行 | Host 可用 |
| C3 | Client ABI 和 UI 可渲染 | Client 可用 |
| C4 | 生命周期、状态、错误和清理完整 | 兼容 |
| C5 | 目标 ABI、Slot、平台和测试矩阵全部闭环 | 目标版本 100% 兼容 |

### 3.2 当前兼容状态

当前项目处于 C3 到 C4 之间：

| 领域 | 当前状态 | 尚缺内容 |
| --- | --- | --- |
| 普通容器包 | 基础完成 | 正式在线安装和完整重启验证 |
| DSH 包安装 | 基础完成 | 事务化安装、更新、回滚和失败恢复 |
| Host ABI | 主体完成 | Agent setup/seed/fork、完整 Subagent 和 MCP 高级语义 |
| Client ABI | 主体完成 | jobs、remote、sessions、resources、sidebar 等真实服务 |
| UI 映射 | 部分完成 | 目标 Slot Catalog 全量映射，不能依赖启发式 fallback |
| 生命周期 | 部分完成 | 崩溃恢复、残留审计和完整清理证明 |
| 官方夹具 | 部分完成 | 目标版本全量官方包和场景矩阵 |
| 跨平台 | 未完成 | 实际 Electron/Core 和平台矩阵 |
| 100% 兼容 | 未完成 | 上述门禁全部闭环 |

## 4. 已冻结的架构决策

| 决策 ID | 决策 |
| --- | --- |
| AD-01 | DSH 兼容由普通 Amitiax 插件 `com.amitia/dsh-compatible-host` 完成。 |
| AD-02 | Amitia 核心只扩展通用 Host API、Slot、Context、Portal 和 Foreign Component 能力。 |
| AD-03 | 不运行 DeepSeek Harness 本体。 |
| AD-04 | Amitia 核心不安装、不启动、不依赖 Cordis。 |
| AD-05 | DSH Client ABI 所需的 Cordis 符号只在容器包内做最小符号 shim。 |
| AD-06 | DSH 子插件不是 Amitia 扩展，不进入 Amitia 扩展生命周期。 |
| AD-07 | DSH 包和依赖只安装到容器 `plugin_data/dsh-packages`。 |
| AD-08 | DSH UI 通过通用 Slot、HOC、Portal、UI Context 和 Foreign Component 渲染。 |
| AD-09 | 每个 DSH 能力必须有直接映射、组合映射、容器内兼容实现或明确失败，不允许静默空实现。 |
| AD-10 | 兼容声明绑定固定 DSH 版本和固定平台矩阵。 |
| AD-11 | 单一版本策略：不保留旧 DSH ABI、旧协议、旧包格式或旧映射分支。 |
| AD-12 | 所有源码修改只发生在源码目录，不修改编译后产物。 |

## 5. 总体架构

```text
用户
  │
  ├─ 安装普通插件
  ▼
Amitia 扩展中心
  │
  └─ com.amitia/dsh-compatible-host
       │
       ├─ DSH 子插件安装与管理
       ├─ DSH Host ABI Shim
       ├─ DSH Client ABI Shim
       ├─ DSH UI Slot Mapper
       ├─ DSH Host Service Mapper
       └─ plugin_data/dsh-packages

Amitia 核心
  ├─ 通用插件安装、权限和生命周期
  ├─ 通用 Host Gateway
  ├─ 通用动态贡献和服务
  ├─ 通用 Slot Kernel
  ├─ 通用 UI Context 和 Surface Catalog
  ├─ 通用 Portal
  └─ 通用 Foreign Component ABI
```

### 5.1 运行链路

```text
安装 Amitiax 容器
  → 启用容器运行时和客户端模块
  → 在容器页面安装 DSH 包
  → 解析 DSH entries
  → 写入容器状态
  → 启用 DSH 子插件
  → 加载 Host entries
  → 注册 Amitia 动态 Tool
  → 注入 PromptExtension
  → 加载 Client entries
  → 映射 Slot、locale、store 和上下文
  → 停用时反序释放全部贡献和运行时
```

### 5.2 架构边界

1. Amitia 后端不新增 `/dsh` 专用业务路由。
2. Amitia 前端不新增 DSH 专用页面分支。
3. Amitia Manifest 不新增 DSH 专用字段。
4. Amitia 核心不依赖 `@deepseek-ai/cordis`。
5. Amitia 不启动 Harness 主进程。
6. DSH 子插件不能绕过容器直接注册全局贡献。
7. DSH UI 不能在未经声明的情况下直接操纵宿主 DOM。
8. 停用或卸载后不能遗留 Tool、Slot、Listener、Service、Job、React Root 或进程。

## 6. 包与数据布局

### 6.1 容器源码

```text
plugins-code/dsh-compatible-host/
├─ amitia-extension.json
├─ package.json
├─ README.md
├─ scripts/
│  ├─ build-package.mjs
│  ├─ test-official-plugin.mjs
│  ├─ test-official-client.mjs
│  └─ test-official-mcp.mjs
├─ src/
│  ├─ runtime/
│  │  ├─ index.mjs
│  │  ├─ abi.mjs
│  │  ├─ host-abi.mjs
│  │  ├─ host-services.mjs
│  │  ├─ package-manager.mjs
│  │  ├─ state.mjs
│  │  ├─ tool-mapping.mjs
│  │  └─ compat/
│  ├─ client-abi/
│  │  ├─ module-loader.mjs
│  │  ├─ react-adapter.mjs
│  │  └─ slot-map.mjs
│  └─ client-ui/
│     └─ index.mjs
├─ ui/
│  └─ schema/desktop.json
└─ tests/
   ├─ fixtures/
   └─ *.test.mjs
```

### 6.2 发行包

```text
plugin/
└─ amitia-dsh-compatible-host-<container-version>.amitiax
   ├─ manifest.json
   ├─ modules/dsh-compatible-runtime/
   ├─ modules/dsh-compatible-ui/
   ├─ integrity/files.json
   └─ integrity/content-tree.json
```

发行包禁止包含：

1. `node_modules` 缓存。
2. 测试数据库和运行数据。
3. DSH 子插件安装内容。
4. 日志和诊断转储。
5. DeepSeek Harness 本体。
6. Cordis 运行时。

### 6.3 容器数据

```text
plugin_data/
├─ dsh-packages/
│  └─ <source-hash>/
│     ├─ package.json
│     ├─ node_modules/
│     └─ package/
├─ state/
├─ sessions/
├─ configuration/
├─ diagnostics/
└─ logs/
```

所有删除操作必须先解析真实路径，并确认目标仍位于 `plugin_data/dsh-packages` 下。

## 7. Amitia 通用 UI 扩展工作流

UI 工作流必须先行或至少与容器骨架并行。DSH UI 不能通过修改 DSH 专用核心分支完成。

### UI-01 Slot Kernel

目标：建立可承载 DSH 复杂组合语义的通用 Slot 内核。

工作内容：

1. `single`、`list`、`keyed`、`chain`、`hoc` 五种 cardinality。
2. 多层 HOC 包裹和 owner content 透传。
3. `priority`、`order`、`entryKey`、`cellId` 和注册顺序。
4. `root`、`session-maybe`、`session` 作用域。
5. store、inject、hooks、selector、getSnapshot 和 subscribe。
6. 动态 Slot 声明、子 Slot 和嵌套渲染。
7. 冲突检测、循环检测、错误边界和递归卸载。
8. 插件 owner、贡献 owner 和 Slot owner 校验。

当前状态：基础完成。

验收：

1. 五种 Slot 均有正反向测试。
2. HOC 顺序、遮蔽、children、重挂载和卸载测试通过。
3. 循环声明、重复 owner、跨 scope 逃逸和残留全部失败关闭。
4. 旧 Slot 行为无语义回归。

### UI-02 UI Context 与 Surface Catalog

目标：插件通过稳定上下文访问宿主状态，不依赖具体 Vue 页面内部实现。

统一上下文：

```text
route
surface
platform
host
theme
locale
session
conversation
message
character
workspace
permissions
capabilities
actions
navigation
```

Surface Catalog 必须记录：

```text
surfaceId
routePrefixes / exactRoutes
host platform
scope
available slots
capabilities
lifecycle
```

当前状态：基础上下文和显式路由解析已完成，完整 Surface 清单尚未完成。

验收：

1. 所有可扩展宿主表面都能从 Catalog 查询。
2. 每个 Surface 的 Slot、owner props、scope 和平台能力有测试。
3. 会话切换、无会话、嵌套会话和路由变化有确定行为。
4. Context 不暴露未授权的会话、消息或权限字段。

### UI-03 Portal 和宿主挂载点

必须覆盖：

1. overlay。
2. modal。
3. drawer。
4. toast。
5. menu。
6. context menu。
7. toolbar。
8. sidebar。
9. page。
10. workspace tab。
11. desktop window。
12. tool result。
13. composer。
14. message action。
15. settings section。

当前状态：overlay、modal、drawer、toast 已有基础实现；其余部分尚未完成。

验收：

1. Portal 绑定插件、Surface、会话和组件实例生命周期。
2. 关闭、切换会话、停用、崩溃和卸载后无残留。
3. 多插件同时使用时有明确排序和冲突策略。
4. 焦点恢复、键盘关闭和宿主核心交互不被抢占。

### UI-04 Foreign Component ABI

目标：在 Vue Slot 中安全承载 React、DSH UI 和其他外部组件。

工作内容：

1. 框架无关的 adapter registry。
2. mount、update、unmount 和 context 更新协议。
3. React Root adapter。
4. Vue owner Slot 与 React island 共存。
5. selector/ref 到 React props 的桥接。
6. React children 到 Vue slot 的反向桥接。
7. CSS 隔离、Shadow DOM 或明确的样式命名空间策略。
8. 错误边界、原始错误展示和重复挂载检测。

当前状态：React mount/update/unmount、Vue/React 双向桥接已完成基础实现；样式隔离和多 adapter 仍需补齐。

验收：

1. Vue 和 React 同时挂载、更新和卸载。
2. React hooks 不因 Vue 重渲染丢失状态。
3. slot owner content 经 HOC 和 children 后仍可渲染。
4. 组件异常进入错误边界，原始错误可复制。
5. adapter 释放后无 DOM、Root、事件和样式残留。

### UI-05 插件自定义 Slot 和跨插件组合

场景：

```text
插件 A 声明 Slot
插件 B 向 A 的 Slot 注册组件
插件 C 读取并组合 A 的 Slot
```

验收：

1. A 停用后，B 的相关贡献被回收。
2. B 停用不影响 A 的 Slot 和其他贡献者。
3. 不存在悬挂 Slot、孤儿贡献或错误 owner 清理。
4. Slot 深度、数量和递归都有预算。

### UI-06 全客户端与平台验收

必须执行：

1. Web 真实浏览器。
2. Desktop Electron。
3. Windows。
4. macOS。
5. Linux。
6. Android/iOS 仅在明确纳入交付范围后执行。

Android 和 iOS 已纳入最终交付范围。移动端不得通过下载任意 Dart/原生代码执行 UI；DSH React Client ABI 必须运行在 WebView Runtime，并由 Flutter 通用 Slot/Web Provider 宿主挂载。

UI 通过标准：

1. 页面、侧栏、聊天头、消息、输入区、设置、工具结果、扩展中心、菜单和浮层都有扩展测试。
2. Vue、React 和外部组件共存测试通过。
3. HOC、Portal、动态 Slot、跨插件组合和作用域切换通过。
4. 插件停用、卸载、重启和恢复后无残留。
5. 所有声明平台与 Manifest、Catalog 和实际测试一致。

“万物皆可扩展”的准确定义是：所有宿主明确声明的 Surface 都能由普通插件注册、组合、包裹和卸载；新增需求通过通用 Slot 声明完成，不需要 DSH 专用核心分支。它不等于允许插件任意操纵宿主 DOM。

## 8. 开发阶段与顺序

总体顺序：

```text
P0 兼容基线与版本冻结
  ↓
P1-P6 Amitia 通用 UI 基础
  ↓
P7 普通容器骨架
  ↓
P8 DSH 包管理器
  ↓
P9 Host ABI
  ↓
P10 Client ABI
  ↓
P11 Host Service 深映射
  ↓
P12 DSH UI 全量映射
  ↓
P13 Client 交互服务
  ↓
P14 生命周期、状态和恢复
  ↓
P15 错误、审计和 Cordis 边界
  ↓
P16 官方夹具与兼容矩阵
  ↓
P17 平台适配
  ↓
P18 构建、真实安装和端到端验证
  ↓
P19 比较矩阵、发布材料和回滚验证
  ↓
P20 最终兼容审计
```

虽然代码骨架可能并行存在，正式验收顺序不能颠倒。

### P0：兼容基线与接口冻结

依赖：无。

需求：UR-08、FR-20、FR-22、NFR-09。

目标：冻结目标 DSH 版本、公开 ABI、UI Slot、平台和不允许支持的范围。

工作内容：

1. 固定 DSH 版本为 `0.2.2-rc.2`。
2. 导出 Host API Catalog。
3. 导出 Client API Catalog。
4. 导出 UI Slot Catalog。
5. 导出事件、服务、Agent、Subagent、MCP、配置和生命周期清单。
6. 对照 Amitia 当前能力形成差异矩阵。
7. 每一项标记为：直接映射、组合映射、容器内实现、平台不适用、待实现。
8. 冻结首批平台：Windows、macOS、Linux、Desktop Web。
9. 移动端作为独立平台门禁，不在无证据时纳入 v1 支持声明。

交付物：

1. `DSH ABI Catalog v1`。
2. `DSH UI Slot Catalog v1`。
3. `Amitia generic capability gap list v1`。
4. `Platform support matrix v1`。
5. `Compatibility matrix v1`。

验收：

1. 每项公开能力有唯一状态和阶段编号。
2. 每个 Slot 有目标 Amitia Surface 或明确失败原因。
3. 没有无编号的“待补充”项。
4. 版本变更必须重新打开 P0。

当前状态：Catalog v1 已完成。目标版本官方包已导出 Host Service 69 项、Host Event 74 项、Host Tool 77 项、Host Configuration 163 项、Session Lifecycle Event 63 项、Client Service 8 项、Client Event 4 项和 UI Slot/Factory 120 项；每项均有唯一编号、阶段和映射状态。Client Service 8 项中 7 项已实现，`DSH-CLIENT-SERVICE-008 workspaces` 依赖核心项目 create/rename/delete 与会话 archive/unarchive/insertSessionBefore 的通用 Host 能力，当前如实标记未实现。UI Slot/Factory 当前 89 项明确映射、31 项按 P12 明确待实现；生产 Slot 路由已删除启发式 fallback。

### P1：通用 Slot 内核

依赖：P0。

需求：FR-13、FR-14、FR-16、NFR-12。

工作内容：

1. 完成五种 Slot cardinality。
2. 完成 HOC、children、多层包裹和重挂载。
3. 完成 store、inject、hooks、selector 和 observable。
4. 完成动态 Slot 注册、注销、父子关系和递归清理。
5. 完成 owner、scope、priority、order 和 entryKey 校验。
6. 完成错误边界、冲突处理和无 owner 渲染策略。

验收：

1. Slot 正反向测试全部通过。
2. 跨 scope、循环声明和重复 owner 失败关闭。
3. 插件停用后 Slot 和贡献全部注销。
4. 原有 Amitia UI 无回归。

当前状态：基础完成，需补齐跨插件和真实页面覆盖。

### P2：UI Context 与 Surface Catalog

依赖：P1。

需求：FR-13、FR-21、NFR-09、NFR-12。

工作内容：

1. 完成 route、surface、platform、session、conversation、message、character、workspace、permission、capability、action 和 navigation 上下文。
2. 建立 Context source、selector、snapshot、subscribe 和 action 调用。
3. 为所有核心页面声明 Surface。
4. 建立 Catalog 自动校验。
5. 统一 Web、Desktop 和移动端的 Surface 能力声明。

验收：

1. 所有扩展表面在 Catalog 中存在。
2. 未声明 Slot 直接失败。
3. 会话切换和缺失会话有确定行为。
4. 上下文不泄漏无权数据。

当前状态：基础完成，Surface 清单覆盖不足。

### P3：Portal 与宿主挂载点

依赖：P1、P2。

需求：FR-15、FR-17、NFR-12。

工作内容：

1. 完成 overlay、modal、drawer、toast。
2. 完成 menu、context menu、toolbar、sidebar、workspace tab。
3. 完成 desktop window、tool result、composer、message action 和 settings section。
4. 为 Portal 建立插件、会话、Surface 和实例 owner。
5. 建立层级、排序、焦点和关闭规则。

验收：

1. 停用、崩溃、会话切换和卸载无 Portal 残留。
2. 多插件同时使用可解释。
3. 不允许 Portal 强占宿主根交互。

当前状态：四类基础 Portal 已完成，其余挂载点未完成。

### P4：Foreign Component ABI

依赖：P1、P2。

需求：FR-06、FR-13、FR-21。

工作内容：

1. 完成通用 adapter 注册协议。
2. 完成 mount、update、unmount、context 更新和错误上报。
3. 完成 React Root adapter。
4. 完成 Vue/React children 和 selector 桥接。
5. 完成样式隔离。
6. 完成多 adapter、并发挂载和重复挂载检测。

验收：

1. React 组件真实挂载、更新、卸载通过。
2. Hooks 状态不丢失。
3. 样式不污染全局。
4. 组件失败显示完整原始错误。

当前状态：基础 React adapter 完成，样式隔离和多 adapter 未完成。

### P5：插件自声明 Slot

依赖：P1、P2。

需求：FR-16、NFR-12。

工作内容：

1. 插件声明自有 Slot。
2. 其他插件向该 Slot 注册。
3. 插件组合、递归渲染和 owner 校验。
4. 深度、数量、循环和性能预算。
5. 停用时递归注销。

验收：

1. A 声明、B 贡献、C 组合的端到端测试通过。
2. A 停用时 B 的相关贡献被回收。
3. B 停用不影响 A 的其他使用者。
4. 不存在悬挂 Slot 或孤儿贡献。

当前状态：部分完成，跨插件组合和递归清理仍需完整验证。

### P6：Amitia 通用 UI 总验收

依赖：P1-P5。

需求：UR-07、FR-13、FR-14、FR-15、FR-16。

工作内容：

1. Web 和 Desktop 全表面回归。
2. Vue、React、外部组件共存测试。
3. HOC、Portal、动态 Slot 和跨插件组合测试。
4. 会话切换、插件启停、卸载和重启测试。
5. 平台能力矩阵测试。

出口标准：

1. 所有声明的 Surface 都有通用扩展点。
2. 未声明 Surface 明确失败。
3. 停用和卸载无残留。
4. 不依赖 DSH 专用核心分支。

注意：本阶段通过只表示“Amitia UI 已具备通用可扩展底座”，不等于 DSH UI 已 100% 兼容。

### P7：普通 Amitiax 容器骨架

依赖：P0、P6 可并行开始，P6 完成后进入完整 UI 验收。

需求：UR-01、UR-02、UR-03、FR-01、FR-02、NFR-06。

工作内容：

1. Manifest、Runtime、Client module 和管理页面。
2. 容器 Host API 边界。
3. 私有的数据目录。
4. 容器状态、诊断和版本信息。
5. 动态 Adapter、权限和贡献预算。

验收：

1. 容器可独立安装、启用、停用和卸载。
2. 扩展中心不出现 DSH 子插件。
3. 停止容器后无工具、页面、进程和监听器残留。
4. 容器包不含 Harness 本体和 Cordis。

当前状态：已完成骨架和管理页。

### P8：DSH 包管理器

依赖：P7。

需求：FR-03、FR-04、FR-18、NFR-02、NFR-03。

工作内容：

1. 支持固定 npm、tarball、本地目录和固定 Git。
2. 解析 `package.json`、`dsh` 字段、Host/Client 入口和 bundle 子路径。
3. 安装到容器 `plugin_data/dsh-packages`。
4. 支持依赖安装、锁文件和来源记录。
5. 实现安装 staging、校验、原子切换和失败清理。
6. 支持启用、停用、更新、回滚和卸载。
7. 支持图标、locale、README、配置和诊断信息。
8. 校验入口不越界。

验收：

1. 安装失败无半安装目录。
2. 删除严格遵守目录边界。
3. 更新失败可恢复原状态。
4. 不同来源可复现。
5. 不含旧 Runtime、旧包或缓存污染。

当前状态：基础安装、路径校验、staging、原子切换和失败保留旧版本已完成；可持久恢复的历史版本回滚仍未完成。

### P9：DSH Host ABI Shim

依赖：P7、P8。

需求：FR-05、FR-07、FR-12、FR-17、FR-21。

工作内容：

1. 加载 Host bundle 和全部公开子路径。
2. 执行 `apply(ctx)`。
3. 实现 effect、disposer 和反序清理。
4. 实现 `provide/get/set` 和服务注入。
5. 实现 `on/emit/waterfall/bail` 等事件语义。
6. 实现 Tool 注册、schema、调用、取消、超时和结果适配。
7. 实现 Prompt section/context/tools/variable。
8. 实现动态 Amitia Tool contribution。
9. 保留原始错误、cause 和堆栈。

验收：

1. Host tool、event、hook、service、prompt 和 bundle 夹具通过。
2. 单个入口失败可被隔离并保留原始错误。
3. 停用后 Tool、Listener 和 Service 全部释放。
4. 不运行 Harness 或 Cordis。

当前状态：主体完成，官方 Todo、Ask User、Present、Subagent、Subagent Control、Schedule 等包已验证。

### P10：DSH Client ABI Shim

依赖：P7、P8、P6。

需求：FR-06、FR-13、FR-14、FR-21。

工作内容：

1. 安装 `window.__ModuleLoader__`。
2. 实现 factory、require、重复注册和缺失依赖错误。
3. 注入 React、JSX Runtime、ReactDOM、Primitives、品牌和 invariant shim。
4. 实现 Client `apply(ctx)`、effect、事件和服务。
5. 实现 Slot register、inject、store、hooks、children 和 locale。
6. 注入 session、conversation、message、workspace 和 permission。
7. 实现多 Client 子路径入口。
8. 实现 React Root、CSS、监听器和模块记录清理。

验收：

1. Client fixture 可加载、渲染、更新和卸载。
2. 多入口 bundle 可分别加载。
3. Slot 注册进入正确 Amitia Surface。
4. 停用后无 DOM、Root、listener、store 和 Slot 残留。

当前状态：主体完成，官方 Jobs、Plan、Sidebar Client 包已通过加载和映射冒烟。

### P11：DSH Host Service 深映射

依赖：P9。

需求：FR-08、FR-09、FR-10、FR-11、NFR-10。

必须逐项完成：

1. network。
2. workspace。
3. memory。
4. resources。
5. events。
6. schedule。
7. approval。
8. sessions。
9. llm。
10. characters。
11. runtime。
12. jobs。
13. agents。
14. subagents。
15. MCP。
16. filesystem。
17. user questions。
18. working directory。

重点语义：

1. `agent.get/resume/followup/steer/inject`。
2. `agent.cancel` 通过通用 `host.conversation.turn.interrupt` 找到当前授权的 active turn/request 并发出真实中断。
3. `agent.whenIdle` 必须基于真实 turn 完成状态。
4. `agent.model.current/select` 必须绑定当前会话授权模型。
5. `sessions.create` 必须使用 Amitia 现有会话系统，不创建第二套 Session。
6. Subagent provider 必须覆盖本地 provider、外部 provider、children、send、interrupt 和 drain。
7. MCP 必须覆盖 stdio/HTTP、工具、resources、prompts、sampling、elicitation 和审批。

Agent cancel 的行业标准设计：

1. 在 Amitia Host API 增加通用 `host.conversation.turn.interrupt`。
2. 在 kernel 定义通用 `ConversationTurnController` 接口。
3. 由后端适配器解析授权 ExecutionScope、Conversation、Character 和当前 active turn/request。
4. 复用 `conversationstream.Manager.Interrupt`、Owned Business Interrupt 和现有 runtime interrupt 路由。
5. 只有确认已发出中断时返回成功。
6. 没有 active turn、scope 不匹配或无权限时原样返回原始错误。
7. DSH `agent.cancel()` 只调用该通用 Host API，不在容器中伪造取消。

验收：

1. 每个服务方法在 ABI Catalog 中有状态。
2. 不允许返回空对象冒充成功。
3. Agent cancel、session create、model select、Subagent 和 MCP 有真实测试或明确失败测试。
4. 取消、超时、审批和错误链路可追踪。

当前状态：Agent create、官方 `resume({resumeSessionId})`、`AgentHandle.dispose()`、cancel、whenIdle、model.current/select、runMaintenance 和 Session create 已闭环。dispose 只停止并等待 Agent 静止、注销 live Agent，不删除持久会话；创建时签发的目标会话能力快照同时约束后续 followup/steer/inject/cancel/wait，未持快照的跨会话调用继续返回 scope_denied。`setup`、seed/fork history、session meta 和完整 Subagent/MCP 高级能力仍未闭环，继续明确失败。

### P12：DSH UI 全量映射

依赖：P0、P6、P10。

需求：FR-13、FR-14、FR-15、FR-21。

工作内容：

1. 以 P0 的 DSH UI Slot Catalog 为唯一输入。
2. 建立 `DSH Slot -> Amitia Slot -> scope -> owner props -> platform -> test -> status` 矩阵。
3. 页面和普通内容映射到 `list/single`。
4. 包裹行为映射到 `chain/hoc`。
5. 浮层映射到 Portal。
6. 未知 Surface 通过通用动态 Slot 补齐。
7. 对每个 Slot 注入真实 session、message、conversation、actions 和 locale。
8. 所有映射必须支持更新、会话切换和卸载。

当前映射类别：

1. 页面和 provider page。
2. Shell overlay、modal、drawer、toast。
3. Sidebar 和 workspace sidebar。
4. Conversation header 和 actions。
5. Composer 和 input activity。
6. Message、turn tail、assistant/user action。
7. Tool call 和 tool result。
8. Plugin detail、settings 和 extension center。
9. Locale、store、inject 和 children。

验收：

1. 目标 Slot Catalog 没有未分类项。
2. 生产映射不得依赖 `includes("sidebar")` 一类启发式猜测。
3. 未映射 Slot 直接失败并保留原始 ID。
4. 每个 Slot 至少在 JSDOM 或真实宿主中完成注册、渲染、更新和卸载。
5. 会话切换、插件停用和重启后 UI 状态正确。

当前状态：已建立 alias 和多类映射，并新增 Composer top/bottom/left/right/overlay/model/model.label/model.options/permission 九个独立通用 Surface、Header leading/lineage/corner 三个单贡献 Surface、助手和用户消息图片组两个独立单贡献 Surface、全局额度通知链和全宽底部内容两个 Shell Surface、右键菜单内容和图标两个 keyed Surface、文件预览和变更文件两个工作台操作 Surface、模型 Provider 卡和模型列表 footer 两个 Settings Surface、右侧根栏/会话栏/文档/Guide 等 Surface，以及设置壳、标题、操作、关闭、侧栏启动器、通知、引导、分节图标、底部触发和模型登录位。`conversation.input.dock`、`conversation.composer.dock`、`conversation.input.left`、`conversation.input.right`、`conversation.input.model`、`conversation.input.model.label`、`conversation.input.model.options`、`conversation.input.overlay`、`conversation.input.permission`、`conversation.chat.message.images`、`conversation.chat.user.images`、`conversation.header.leading`、`conversation.session.header.lineage`、`conversation.session.header.corner`、`shell.bottom`、`shell.quota-notice`、`rightbar`、`rightbar.session`、`contextMenu.content`、`contextMenu.icon`、`deliverables.file.actions`、`deliverables.review.file.actions`、`settings.models.provider-card`、`settings.models.footer`、`sidebar.settings`、`settings.action`、`settings.close`、`settings.header`、`settings.launcher`、`settings.launcher.notifications`、`settings.models.sign-in`、`settings.onboarding`、`settings.section.icon`、`settings.trigger`、`sidebar.right.tab.document`、`sidebar.right.tab.document.action`、`sidebar.right.tab.document.actions`、`sidebar.right.tab.document.office.pdf`、`sidebar.right.tab.document.unpreviewable`、`sidebar.right.tab.files.actions`、`sidebar.right.tab.guide`、`sidebar.right.tab.guide.actions`、`sidebar.right.tab.guide.entry` 和 `sidebar.right.tab.menu.item` 已精确映射。Composer、图片和 Shell 槽位在 Web、Electron、Android、iOS 挂载；Context Menu、Workbench、模型 Settings、设置壳和右侧栏槽位按真实宿主能力限定为 Windows、macOS、Linux、Web。模型 Provider 卡按 `settingsNs` 执行 keyed dispatch，并区分 saved 与 draft；footer 位于模型配置列表和新增控件之后。当前 120 项中 89 项明确映射、31 项 pending。

### P13：DSH Client 交互服务

依赖：P10、P11。

需求：FR-09、FR-10、FR-11、FR-13。

必须完成：

1. Jobs：行状态、observe、watchRows、kill 和 stream 更新。
2. Remote commands：真实执行、错误和取消。
3. Remote sessions：事件流、分页和恢复。
4. Sessions binding：当前 session、subagent address 和切换。
5. UI conversation：事件注册、binding、target snapshot。
6. Resources：注册、打开、读取和释放。
7. Sidebar right：mounted、openResource 和 tab 注册。
8. Layout：selectPanel、toggleSidebar 和状态同步。
9. Shortcuts：catalog、触发和权限。
10. UI workspace：startSession、工作区和 panel。

验收：

1. 官方 Jobs、Plan 和 Sidebar 包不只是“加载成功”，还要完成真实交互。
2. observe、kill、session 切换、资源打开和侧栏 tab 有真实测试。
3. 所有暂未实现服务必须返回完整明确错误，不能返回空成功。

当前状态：Jobs Client、resource provider、右侧资源面板和侧栏 tab/title 已闭环；`layout` 已映射官方 Conversation、Plugins、Schedules 面板；`sessions.create`、`uiWorkspace.startSession` 和 `uiWorkspace.openSession` 已接入真实会话事务与导航；官方 Schedule Client 已可加载。`theme` 已提供主题快照、`theme/change` 事件、内置 light/dark 与第三方主题注册、`overrideTokens` 分层叠加、三档字号与字体族写入，`--dsw-*` 基础 token 映射 Amitia `--tp-*`，宿主调色板通过 `MutationObserver` 与 `prefers-color-scheme` 实时同步；`timer` 已提供 Cordis 风格 `timeout/interval/throttle/debounce` 并在模块上下文注入 `sleep`。`workspaces` 仍缺少核心项目 CRUD 与会话归档/排序 Host 能力，未做假实现。Amitia 唯一 Schedule 执行器已新增通用 `session_prompt` Target，并完成固定 Session、`new_once`、`new` 目标、`after/at/every/daily/weekly/cron/rrule`、IANA 本地绝对时间、create/catalog/list/view/update/delete/run/deleteInactive 和 delivery history 映射；`new` 的 `cwd`、`agentPreset` 自定义创建参数仍未闭环。Remote、WorkspaceId、fork 和 Session retain/projection 的深语义仍不完整。

### P14：生命周期、状态与恢复

依赖：P8-P13。

需求：FR-17、FR-18、FR-19、NFR-01、NFR-02、NFR-03。

工作内容：

1. 容器启动恢复。
2. DSH 包状态版本校验。
3. 启用顺序和依赖顺序。
4. 安装事务、更新事务和回滚。
5. 配置、Session、日志和用户数据隔离。
6. 崩溃隔离、超时和健康检查。
7. Tool、Slot、Service、Job、Listener 和 React Root 递归清理。
8. 失败状态持久化和重试边界。

验收：

1. 容器崩溃不影响 Amitia 核心启动。
2. 一个 DSH 插件失败不影响其他插件。
3. 重启后无重复注册。
4. 恢复后的贡献数量与启用记录一致。
5. 停用和卸载后无任何运行时残留。

当前状态：基础状态 CAS、Session 事件持久化、63 项官方生命周期事件信封与 surface 投影语义、Stop/Remove 清理已完成；事务化恢复和完整残留审计未完成。

### P15：错误、审计和 Cordis 边界

依赖：P7-P14。

需求：FR-21、NFR-05、NFR-10、AD-04、AD-05。

工作内容：

1. 统一错误附加规则。
2. 审计所有 catch、Host 调用、任务、流式响应和桥接代码。
3. 保留 name、message、code、cause、stack、details 和上游字段。
4. UI 显示用户解释时同时提供原始错误。
5. 扫描核心依赖、前端 bundle、容器包和安装结果中的 Cordis。
6. 扫描 Harness 主进程和 DSH 专用核心分支。
7. 扫描空实现、假成功和默认 fallback。

验收：

1. 无吞错和固定错误覆盖。
2. 无无 cause 重抛和原始错误截断。
3. 无 `@deepseek-ai/cordis` 核心依赖。
4. 无 Harness 进程。
5. 无 DSH 专用后端路由、前端页面和 Manifest 字段。

当前状态：已建立明确失败原则和依赖边界测试，仍需全仓交叉审计。

### P16：官方夹具和兼容矩阵

依赖：P0、P8-P15。

需求：FR-20、FR-22、NFR-07、NFR-09。

夹具类型：

1. Host-only。
2. Client-only。
3. Host + Client。
4. Bundle 多 Host 入口。
5. Bundle 多 Client 入口。
6. 多 Slot。
7. HOC/children。
8. Locale/store/inject。
9. Session/Agent/Subagent。
10. MCP stdio/HTTP。
11. 安装、更新、回滚和卸载。
12. 崩溃和恢复。
13. 原始错误。
14. 恶意路径和越界包。

当前已验证官方包：

Host：

1. `@deepseek-ai/dsh-tool-todo@0.2.2-rc.2`。
2. `@deepseek-ai/dsh-tool-ask-user@0.2.2-rc.2`。
3. `@deepseek-ai/dsh-tool-present@0.2.2-rc.2`。
4. `@deepseek-ai/dsh-tool-subagent@0.2.2-rc.2`。
5. `@deepseek-ai/dsh-tool-subagent-control@0.2.2-rc.2`。
6. `@deepseek-ai/dsh-tool-schedule@0.2.2-rc.2`。

Client：

1. `@deepseek-ai/dsh-client-ui-jobs@0.2.2-rc.2`。
2. `@deepseek-ai/dsh-client-ui-plan@0.2.2-rc.2`。
3. `@deepseek-ai/dsh-client-ui-sidebar@0.2.2-rc.2`。

MCP：

1. `@deepseek-ai/dsh-mcp-client@0.2.2-rc.2`。
2. `@modelcontextprotocol/server-everything@2.3.0`。
3. 真实 stdio 连接、22 个工具发现和 `echo` 调用通过。
4. `resources/list` 和 `resources/read` 通过。

测试分级：

| 等级 | 内容 | 失败条件 |
| --- | --- | --- |
| T0 | Manifest、依赖、哈希和 ABI 静态检查 | 任一未分类或缺失 |
| T1 | 容器单元测试 | Tool、状态、映射或清理失败 |
| T2 | DSH 夹具 ABI 测试 | Host/Client 无法执行 |
| T3 | Amitia 集成测试 | Slot、Host 调用、Prompt 或贡献链路失败 |
| T4 | Electron/真实浏览器 | 真实安装、渲染、会话或卸载失败 |
| T5 | 平台发布验收 | Manifest 声明与实际不一致 |

验收：

1. 每个 ABI 项至少一条 T1/T2 证据。
2. 每个 Slot 至少一条 T2/T3 证据。
3. 每个关键场景包含成功、失败和清理路径。
4. 不允许只测加载成功。
5. 兼容矩阵中的“已支持”必须能追溯到测试。

### P17：平台适配

依赖：P16 基础通过。

需求：NFR-01、NFR-11。

平台范围：

| 平台 | v1 状态 | 验收 |
| --- | --- | --- |
| Windows | 声明支持 | 真实 Electron/Core、文件、进程、网络和卸载 |
| macOS | 声明支持 | 真实 Electron/Core、文件、进程、网络和卸载 |
| Linux | 声明支持 | 真实 Electron/Core、文件、进程、网络和卸载 |
| Desktop Web | 开发与验证面 | 浏览器安装、渲染、会话和卸载 |
| Android | 必须支持 | Runtime Package、Node 容器、DSH 子插件安装、WebView Client Runtime、Slot/Portal 和完整生命周期 |
| iOS | 必须支持 | 本地 Runtime、Node 容器、DSH 子插件安装、WebView Client Runtime、Slot/Portal 和完整生命周期 |

验收：

1. Manifest 平台与实际能力一致。
2. 不支持平台显示完整原因。
3. 不得通过平台静默降级伪造成功。
4. 所有声明平台至少有一条真实端到端证据。

### P18：构建、真实安装和端到端验证

依赖：P15-P17。

需求：FR-01、FR-18、NFR-08、NFR-09。

工作内容：

1. 使用项目内 Node 构建容器。
2. 生成 `plugin/amitia-dsh-compatible-host-<version>.amitiax`。
3. 校验 Manifest、权限、Adapter、入口、资源、CSS、文件和内容树哈希。
4. 在干净 Amitia 环境安装普通容器。
5. 从容器页面安装至少一个真实 DSH Host 插件和一个真实 DSH Client 插件。
6. 执行工具、Prompt、Session、UI、MCP 和错误场景。
7. 执行停用、完整重启、恢复、更新、回滚和卸载。
8. 检查 Amitia 核心、Qdrant、SurrealDB、plugin-host 和已安装插件健康状态。

验收：

1. 正式安装包构建成功。
2. 包内无依赖缓存、日志、测试数据、DSH 子插件或旧 Runtime。
3. 用户旅程完整通过。
4. 完整重启后恢复成功。
5. 停用和卸载无残留。
6. 包哈希与记录一致。

当前阻塞：

```text
desktop/scripts/build-amitiacore.mjs
freeze manifest does not match canonical scope
```

该问题来自当前工作区冻结清单与已有未提交改动不一致。处理前必须先确认改动归属，不能把用户已有改动混入发布基线，也不能通过修改编译产物或绕过门禁伪装通过。

### P19：兼容矩阵、发布材料和回滚验证

依赖：P18。

需求：FR-20、FR-22、NFR-09。

交付：

1. 容器包。
2. 目标 DSH ABI Catalog。
3. UI Slot Catalog。
4. Host/Client/Service 映射矩阵。
5. 平台支持矩阵。
6. 测试清单和原始结果。
7. 容器版本、目标 DSH 版本和包哈希。
8. 安装、更新、回滚和卸载说明。
9. 已知限制和完整原始错误说明。

验收：

1. 所有矩阵与包版本一致。
2. 不存在未分类项。
3. 不存在“代码有入口但无测试”的公开发布能力。
4. 发布说明不得把部分兼容描述成 100%。

### P20：最终兼容审计

依赖：P0-P19。

必须回答：

1. 目标 DSH 版本是什么。
2. 哪些 Host API 是直接映射。
3. 哪些 Host API 是容器内实现。
4. 哪些 Client API 是直接映射或 shim。
5. 哪些 UI Slot 映射到哪些 Amitia Surface。
6. Agent、Session、Subagent、MCP、Approval 和 Jobs 是否有真实测试。
7. 安装、更新、回滚、恢复和卸载是否有真实证据。
8. 是否存在 Cordis、Harness 本体或 DSH 专用核心分支。
9. 是否存在启发式默认回退掩盖未映射能力。
10. 所有声明平台是否与测试一致。

最终通过条件：

1. 目标公开 ABI 覆盖率为 100%。
2. 目标 UI Slot 覆盖率为 100%。
3. 所有平台门禁通过。
4. 所有错误和清理路径通过。
5. 不存在未分类跳过项。
6. 不存在假成功和静默降级。

## 9. DSH Host API 映射责任

| DSH 能力 | Amitia 通用责任 | 容器责任 | 禁止做法 |
| --- | --- | --- | --- |
| 包发现和 Manifest | 文件权限、Plugin Data 和 Host API | 解析 DSH Package、入口和 bundle | 在核心写 DSH 路由 |
| `apply(ctx)` | 通用插件运行时 | 创建 Shim Context 并调用 | 启动 Harness 本体 |
| effect/disposer | 通用生命周期和清理 | 记录并反序释放 | 忽略 disposer |
| inject/service | 通用 Service Graph | DSH 服务名和语义适配 | 缺失时继续假运行 |
| events/waterfall/bail | 通用事件或调用桥 | 实现 DSH 事件语义 | 吞掉异常 |
| tools/defineTool | 动态 Tool contribution 和执行管线 | schema、参数、输出、取消和错误 | 只注册名称不执行 |
| systemPrompt | Prompt Extension / Slot | 收集 section/context/tools/variable | 核心写 DSH 专用拼接 |
| agents | 现有会话、Turn、模型和多 Agent 能力 | 身份、idle、cancel、model 语义 | 新建第二套 Agent 系统 |
| sessions | 现有会话、消息和持久化 | resume、append、send、projection | 新建第二套 Session |
| subagents | 现有多 Agent 或通用 provider | activation、children、interrupt | 绕过授权任意创建 |
| jobs | 通用后台任务 | 状态、取消和观测 | 遗留定时器 |
| MCP | 通用 MCP 服务 | DSH MCP client/resources 映射 | 核心写 DSH MCP 分支 |
| approval | `host.ui.dialog` 和权限 | DSH 审批语义适配 | 固定自动同意 |

## 10. DSH Client/UI 映射责任

| DSH 能力 | Amitia 通用责任 | 容器责任 | 验收 |
| --- | --- | --- | --- |
| ModuleLoader | 通用 Client/Foreign Component 宿主 | factory、require、注册和释放 | 模块加载和卸载 |
| React | Foreign Component ABI | ReactDOM adapter | Root 生命周期 |
| Primitives | 依赖注入 | 固定版本打包并注入 | 真实组件渲染 |
| slots.register | Slot Registry | DSH Slot ID 精确映射 | 注册到正确 Surface |
| slots.inject | Slot lifecycle/inject | 适配 DSH inject | 初始化和销毁 |
| store/hooks | Slot store/observable | DSH store 语义适配 | 更新和清理 |
| children | Slot children/island | Vue/React 桥接 | owner content 不丢失 |
| locale | Locale namespace | DSH locale 注入 | 切换语言 |
| session context | UI Context | conversation/message 注入 | 会话切换 |
| actions/navigation | 通用 Action/Navigation | 权限内适配 | 未授权动作失败 |
| CSS | 样式隔离 | 完整 CSS 和资源 | 不污染全局 |
| Portal/HOC | 通用 Slot 能力 | 选择 inline、HOC、Portal | 停用后无残留 |

## 11. 错误处理规范

1. 安装、解析、加载、执行、渲染和卸载错误必须保留原始错误对象。
2. 保留 `name`、`message`、`code`、`cause`、`stack`、`details` 和上游附加字段。
3. 禁止空 catch、吞错、无 cause 重抛和固定文案覆盖。
4. API、任务、流式响应、RPC、原生桥接和插件桥接必须向调用方传播原始错误。
5. 只有在无法返回对象时才序列化错误，并且必须保留完整内容。
6. UI 可以增加易懂解释，但不得替代原始错误。
7. 日志必须附加上下文，不能修改原始 message 和 stack。
8. 日志上下文至少包含容器 ID、DSH 包 ID、版本、入口、阶段、toolCallId、conversationId 和 traceId。

## 12. 状态与数据规范

1. 容器状态使用单一版本化 Schema。
2. DSH 包记录包含 ID、版本、来源、安装根、Host entries、Client entries、启用状态和映射摘要。
3. Session 事件按 conversation 隔离。
4. 事件写入使用 CAS，最终成功前必须等待持久化。
5. Session projection 必须可以从持久事件重新计算。
6. 更新前记录旧状态快照。
7. 回滚只针对当前版本安装事务，不建立旧 ABI 兼容层。
8. 卸载策略必须明确：保留用户数据或删除数据。
9. 禁止把 DSH 数据写入 Amitia 固定业务表。
10. 禁止写回已安装 `.amitiax` 包目录。

## 13. 性能与稳定性规范

1. 容器启动不能阻塞 Amitia 核心启动。
2. 单个 DSH 插件失败不能阻止其他插件启用。
3. Slot、Contribution、Tool、Route、Service 和 React Root 都有数量预算。
4. 大量插件启用时，Host 加载和 Client 渲染应分批或按需执行。
5. 安装、更新和卸载任务可取消并报告状态。
6. Tool、Job、Portal 和 React Root 必须有泄漏检查。
7. 容器进程必须有超时、健康检查和恢复策略。
8. UI Slot 的递归深度和组件嵌套必须有上限。

## 14. 构建与发布规范

1. 源码位于 `plugins-code/dsh-compatible-host`。
2. 包产物只输出到项目根目录 `plugin/`。
3. 使用项目内 Node 二进制构建。
4. 不修改编译后产物。
5. 不把 DSH 子插件打入 Amitiax。
6. 不把 `node_modules`、日志、测试数据库或运行数据打入 Amitiax。
7. 构建后验证 Manifest、权限、Adapter、入口、资源、CSS、文件哈希和内容树哈希。
8. 验证包内不存在 Cordis 和 Harness 本体。
9. 发布前执行对应阶段定向测试，不默认运行无关全量测试。
10. 未收到明确上传指令时，不推送 Git，不上传发布服务器。

## 15. 验收矩阵

| ID | 场景 | 必须证明 | 阶段 |
| --- | --- | --- | --- |
| AC-01 | 安装普通容器 | 扩展中心只有一个 Amitiax 容器 | P7/P18 |
| AC-02 | 容器内安装 DSH Host 插件 | 包只进入 plugin_data，工具动态注册 | P8/P9 |
| AC-03 | 容器内安装 DSH Client 插件 | Client ABI 加载、React 渲染和 Slot 注册 | P10/P12 |
| AC-04 | DSH bundle 多入口 | 每个入口独立 apply、注册和清理 | P9/P10 |
| AC-05 | 工具调用 | schema、参数、输出、取消和原始错误 | P9/P11 |
| AC-06 | 事件与 Hook | 顺序、waterfall、bail、异常和释放 | P9 |
| AC-07 | systemPrompt | section/context/tools/variable 进入真实提示词链路 | P9/P11 |
| AC-08 | Session | 读取、追加、发送、持久化和 projection 恢复 | P11/P14 |
| AC-09 | Agent cancel | 真实 active turn 中断，无 active turn 或越权失败 | P11 |
| AC-10 | Agent create/model | 真实创建和模型读取/选择，或明确不属于目标 API | P11 |
| AC-11 | Subagent | provider、children、send、interrupt 和 drain | P11 |
| AC-12 | MCP | stdio/HTTP、tools、resources、prompts、sampling、elicitation | P11/P16 |
| AC-13 | UI 全量 Slot | 每个目标 Slot 有映射、平台和测试 | P12/P16 |
| AC-14 | HOC/children | owner content、层级、顺序和重挂载 | P1/P4/P12 |
| AC-15 | Portal | overlay/modal/drawer/toast/menu 生命周期 | P3/P12 |
| AC-16 | locale/store/inject | 更新、持久化、作用域和清理 | P1/P10/P13 |
| AC-17 | Client 交互服务 | Jobs、remote、sessions、resources、sidebar 和 workspace | P13 |
| AC-18 | 停用和卸载 | Tool、Slot、Root、listener、service、job 和目录清理 | P8/P14 |
| AC-19 | 崩溃和重启 | 失败隔离、恢复、无重复注册 | P14 |
| AC-20 | 原始错误 | 对象、cause、stack、details 可查看和复制 | P15 |
| AC-21 | Cordis 边界 | 核心和包依赖审计通过 | P15 |
| AC-22 | 无 DSH 专用核心分支 | 核心无 DSH 路由、页面、表、字段和 Adapter | P15/P20 |
| AC-23 | 跨平台 | Manifest 与实际行为一致 | P17 |
| AC-24 | 构建发布 | 包白名单、哈希、安装、回滚和发布材料一致 | P18/P19 |

## 16. 里程碑

| 里程碑 | 完成条件 | 可以宣称 |
| --- | --- | --- |
| M0 基线冻结 | P0 完成 | 兼容范围可证明 |
| M1 UI 通用底座 | P1-P6 完成 | Amitia UI 具备通用可扩展能力 |
| M2 容器可管理 | P7-P8 完成 | 普通 Amitiax 可管理 DSH 插件 |
| M3 Host 可用 | P9、P11 完成 | DSH Host 侧可用 |
| M4 Client/UI 可用 | P10、P12、P13 完成 | DSH Client 和 UI 可用 |
| M5 生命周期可交付 | P14、P15 完成 | 可停用、恢复、卸载和审计 |
| M6 兼容可证明 | P16 完成 | 固定目标版本覆盖率可计算 |
| M7 平台与安装通过 | P17-P18 完成 | 声明平台有真实证据 |
| M8 100% 兼容 | P19-P20 完成 | 仅可对目标 DSH 版本宣称 100% 兼容 |

## 17. 当前实现证据

截至本文档更新时，源码已有以下证据：

1. 通用 UI 定向测试包含 HOC、UI Context、Foreign Component 等用例。
2. 容器测试 48 项通过。
3. `go test ./internal/interaction` 通过。
4. 官方 Host、Client 和 MCP 冒烟脚本通过。
5. 已生成 `plugin/amitia-dsh-compatible-host-0.1.0.amitiax`。
6. `host.conversation.turn.interrupt` 已接入通用 Host API、Owner Scope 校验、active Turn 查找和真实 runtime 取消。
7. Agent cancel、whenIdle、DSH Jobs Client 和事务化包安装已接入真实容器链路。
8. Client locale 已接入真实字典和多语言回退，兼容状态可查询且明确区分 implemented、partial 和 unavailable。
9. 当前粗粒度能力矩阵为 31 项能力中 19 项 implemented、11 项 partial、1 项 unavailable，不能据此宣称目标 ABI 全量覆盖。
10. 移动端通用 `mobile_web_runtime` Provider 已完成 SDK、后端校验、Provider 解析、Flutter WebView 宿主、UI Bridge Session 和应用级 Portal Host 基础链路。
11. 当前包 `amitia-dsh-compatible-host-0.1.41.amitiax` SHA-256：`1b8e1925fb505a2ede1e2ddac3a709de42c82ff84b709503acde8b0a9fa6e3b7`。
12. 当前包 Tree Hash：`56ec54b93749b15b8ffc8bd691f54720b14f3c9ba9e3db40c19dcb27f0c6a445`。
13. `DSH ABI Catalog v1`、`DSH UI Slot Catalog v1`、`Amitia generic capability gap list v1`、`Platform support matrix v1` 和 `Compatibility matrix v1` 已由目标版本官方包生成，位于 `plugins-code/dsh-compatible-host/catalog/`。
14. Catalog 生成器会从 `@deepseek-ai/dsh@0.2.2-rc.2` 与 `@deepseek-ai/dsh-cordis-client-runner@0.2.2-rc.2` 读取官方生成目录，连续两次生成的目录哈希一致。
15. Catalog 定向测试校验版本、来源包、唯一编号、Slot 目标、缺口数量、平台声明和目录生成快照，当前 5 项全部通过。
16. DSH Client `sessions.create`、`uiWorkspace.startSession/openSession` 已通过通用 `host.conversation.create` 创建并打开真实 Amitia 会话；不能安全映射的 Workspace、fork 和 Remote 继续返回带 P13 原因的完整失败。
17. 官方 `@deepseek-ai/dsh-client-ui-schedule@0.2.2-rc.2` 已通过 Client ABI、Remote Service、RemoteAccess、Slot/Factory 和侧栏依赖加载；尚未实现语义的 Schedule 操作未被伪造成成功。
18. Amitia Schedule 已新增通用 `session_prompt` Target，执行时从创建时冻结的 Scope Snapshot 恢复 Owner Scope，再通过现有 `ConversationMessageAppender` 向同一会话投递 prompt，不创建第二套定时任务系统。
19. Schedule Host 服务已补 `create/update/run/cancel/list/history` 与会话归属检查；DSH Schedule Remote 已覆盖固定 Session、`new_once`、`new` 目标的 create/catalog/list/view/update/delete/run/deleteInactive/history。`after` 已修正为单次任务，`at` 支持 RFC 3339 与 IANA `LocalAtInput`，`rrule` 通过有界 RFC 5545 evaluator 接入唯一 Schedule 执行器；删除支持 `expectedUpdatedAt` 冲突检查，历史记录保存不可变 prompt。
20. 已用真实 Core 完成 RRULE + `new_once` 端到端验收：首次运行创建真实会话并投递用户角色消息，第二次运行复用同一会话；两次 Run 均为 completed，目标会话仅 1 个且累计 2 条消息，历史接口返回真实 message ID、target session 和 prompt。`new_once` 更新为 `new` 后官方目录返回正确的 `{kind:"new",create:{}}`。
21. 修复 UI Bridge Scope 的通用持久化与执行上下文链路：非设备授权快照保存完整 Space/Owner，跨模块派生保留 Owned Execution Scope，会话携带角色修订和 Memory Scope，并允许当前运行上下文中的 `promptExtensions`。当前插件 83 项测试通过，后端 `host_api`、`schedule`、`kernel`、`scope`、SQLite persistence、coordination、business worker 及 `cmd/server` 定向测试通过；开发 Core 已从当前源码重建并以 SHA-256 `cd502f0485cb2a689ec39c66708da99fa32e911d3365ae41d66a985c29414e87` 重启。
22. 官方 Jobs、Plan、Sidebar、Schedule 四个 Client 包在最终源码上全部通过 Client ABI、ModuleLoader、Remote Service 和 Slot 注册冒烟；开发 Core `/api/health` 与 `/readyz` 均为 ready，Vite、Qdrant、SurrealDB 健康接口正常。
23. `host.conversation.create` 现在签发与扩展、模块、Space、角色和目标会话绑定的通用能力快照。嵌套 Host 调用只有携带该快照时才能切换目标会话；日志会包含具体 `host.call <method>`，原始错误码和错误文本继续向上传播。
24. 真实 Core 已完成 Agent create/resume/inject/dispose 链路：创建 `dsh-agent-e2e-66121565-685b-4dbc-a8ff-5c9f4f3f1081` 后 dispose，重启 Runtime 后通过官方 `resumeSessionId` 恢复；`inject` 写入真实 `user` 消息，dispose 后 conversation 与两条 message 均保留且 `deleted=0`，live Agent 列表为空。无 active turn 时只忽略明确的 `state_conflict` 无活动轮次错误，其他原始错误不吞掉、不改写。
25. 最终门禁通过：插件 83/83、Runtime Host 1/1、`go mod verify`、`go vet ./...`、`go build ./cmd/server`、Kernel/Host API/JavaScript runtime 定向测试及 `cmd/server` Owner 会话定向测试；官方 Host、Desktop Client、MCP 和 Mobile Client 冒烟均通过。
26. 真实 `conversation-delete` 已使用创建时目标能力快照完成删除事务，并同步删除 `dsh-session` 状态和 Host 目标能力快照；验证结果分别为 conversation 删除标记、状态键 0 条、能力快照 0 条，清理后 `/readyz` 仍为 ready。
27. P12 Composer 第一批已完成：通用 Slot Registry 新增 `chat.composer.dock.top`、`chat.composer.dock.bottom`、`chat.composer.left`、`chat.composer.right`、`chat.composer.overlay`；Web/Electron 输入区与 Android/iOS 输入区完成对应挂载；Dock、左右工具栏位和 Session/Input owner context 已进入 DSH Client bridge。插件 83/83、Composer 前端定向 6/6、移动 Surface Catalog 3/3、Slot Registry 后端定向测试和 Vite 生产构建通过；运行中 Slot 快照共 71 项，5 个新 Surface 全部存在。
28. P12 Composer 第二批已完成：新增单贡献替换型 `chat.composer.model` 与 `chat.composer.permission`，Web/Electron 使用 Standard Slot 包裹原生控件，Android/iOS 使用带 fallback 的移动 Slot 保留默认模型与权限控件；DSH `conversation.input.model` 和 `conversation.input.permission` 已精确映射。当前运行中 Slot 快照共 73 项，7 个 Composer 新 Surface 全部存在，`/api/health` 与 `/readyz` 均为 ready。
29. P12 Header 批次已完成：新增 `chat.header.leading`、`chat.header.lineage`、`chat.header.corner`，分别保持 root 导航、会话标题替换和三处不同 Header 座位；Web/Electron 与 Android/iOS 均完成挂载，DSH leading、lineage、corner 三个 Slot 已精确映射。运行中 Slot 快照增至 76 项，10 个本轮新增 Composer/Header Surface 全部存在；Composer 前端 6/6、Header 前端 4/4、移动 Surface Catalog 3/3、插件 83/83、Slot Registry 后端定向测试和 Vite 生产构建通过。
30. P12 Composer 第三批已完成：新增 `chat.composer.model.label` 单贡献标签槽与 `chat.composer.model.options` 菜单列表槽，并补全移动端 `chat.composer.overlay` 卡片内宿主。DSH `conversation.input.model.label`、`conversation.input.model.options` 和 `conversation.input.overlay` 已精确映射；Catalog 提升至 58/120，剩余 62 项。插件 83/83、Composer 前端 6/6、Header 前端 4/4、移动 Surface Catalog 3/3、Go 定向测试、`go mod verify`、`go vet ./...`、Vite 生产构建和官方 Host/Desktop Client/MCP/Mobile Client 冒烟均通过。Core 从当前源码重建，SHA-256 为 `8c30ac2d1e6f7de8c19d5846263967f3f60a74c47501df749a7ae09e252f9410`；`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。`0.1.36` 已完成真实包更新并启用，generation 为 `generation-d2789e85-d602-41be-80cc-d57616e70b50`；运行中 Slot 快照 78 项，三个目标 Surface 均存在 `com.amitia/dsh-compatible-host` contribution。
31. P12 消息图片批次已完成：新增 `chat.message.images` 与 `chat.message.user.images` 单贡献槽，连续图片统一分组成画廊；Web 的流式与气泡两种消息样式均保留 `AmitiaImageBlock` fallback，移动端按消息角色选择槽并保留原生图片卡片和预览。DSH `conversation.chat.message.images` 和 `conversation.chat.user.images` 已精确映射；Catalog 提升至 60/120，剩余 60 项。插件 83/83、前端消息相关测试 31/31、移动 Surface Catalog 3/3、Go 定向测试、`go mod verify`、`go vet ./...`、Vite 生产构建和官方 Host/Desktop Client/MCP/Mobile Client 冒烟均通过。Core 从当前源码重建，SHA-256 为 `ad9c590a87538499a31459806b6f81e60bd395157f444852cfe71e95b6cd0bc4`；`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。`0.1.37` 已完成真实包更新并启用，generation 为 `generation-11ec3998-ee8b-45e1-a401-bcb712656dfc`；运行中 Slot 快照 80 项，两个目标 Surface 均存在 `com.amitia/dsh-compatible-host` contribution。
32. P12 Shell 批次已完成：新增 `app.shell.quota_notice` 根级链式通知槽与 `app.shell.bottom` 根级单贡献底部槽；Web/Electron 在应用根布局中按“通知链、主壳、底部内容”顺序挂载，Android/iOS 在路由 Shell 中以相同顺序保留主内容。DSH `shell.quota-notice` 和 `shell.bottom` 已精确映射；Catalog 提升至 62/120，剩余 58 项。插件 83/83、前端定向 22/22、移动 Surface Catalog 3/3、Go 定向测试、`go mod verify`、`go vet ./...`、Vite 生产构建和官方 Host/Desktop Client/MCP/Mobile Client 冒烟均通过。Core 从当前源码重建，SHA-256 为 `0d6ad0376bd6c586d041b5735b24b7f72f27b44712bf0edffb33f532e1da8cc2`；`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。`0.1.38` 已完成真实包更新并启用，generation 为 `generation-844eb48b-eb8b-42e7-a2bd-d77b67f2f1b4`；运行中 Slot 快照 82 项，两个目标 Surface 均存在正确 kind、root scope 和 `com.amitia/dsh-compatible-host` contribution。
33. P12 Context Menu 批次已完成：新增 `desktop.context_menu.content` 和 `desktop.context_menu.icon` 两个 root-scope keyed 槽；内容按 action ID dispatch，图标按独立 `iconKey` dispatch，根菜单和子菜单均传入 action、owner、图标、快捷键、层级和菜单标题上下文。DSH `contextMenu.content` 和 `contextMenu.icon` 已精确映射；槽位平台声明限定 Windows、macOS、Linux、Web，不要求 Android/iOS 伪造右键菜单 contribution。Catalog 提升至 64/120，剩余 56 项。插件 83/83、前端定向 27/27、Go 定向测试、`go mod verify`、`go vet ./...`、Vite 生产构建和官方 Host/Desktop Client/MCP/Mobile Client 冒烟均通过。Core 从当前源码重建，SHA-256 为 `17c70300287f9862267e494db209ce11cf60755772fae39a2e4ed31a60f562b0`；`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。`0.1.39` 已完成真实包更新并启用，generation 为 `generation-035330d9-a6e3-4dd3-9112-69302103af04`；运行中 Slot 快照 84 项，两个目标 Surface 均为 root scope、keyed dispatch 且带四平台白名单。当前环境未安装注册这两个 keyed 槽的外部 DSH Client 插件，因此运行态 contribution 数为 0，渲染和 owner context 由前端定向测试覆盖。
34. P12 Workbench 文件操作批次已完成：新增 `workbench.file.actions` 与 `workbench.review.file.actions` 两个 session-scope 列表槽，分别挂在文件预览工具栏和每个 Git 变更文件行。DSH `deliverables.file.actions` 与 `deliverables.review.file.actions` 已按真实文件事件坐标映射；上下文包含 conversationId、characterId、workspaceUri、resourceUri、相对路径、文件名和事件坐标，槽位平台限定 Windows、macOS、Linux、Web。Catalog 提升至 66/120，剩余 54 项。插件 83/83、前端定向 29/29、Go 定向测试、`go mod verify`、`go vet ./...`、Vite 生产构建和官方 Host/Desktop Client/MCP/Mobile Client 冒烟均通过。Core 从当前源码重建，SHA-256 为 `17d411a3d914de39eb2476e01042ab9c5073832395da66c582f23815508c2ab8`；`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。`0.1.40` 已完成真实包更新并启用，generation 为 `generation-0a779abb-282d-46b6-94a4-385b888ced5e`；运行中 Slot 快照 86 项，两个目标 Surface 均为 session scope、list dispatch 且带四平台白名单。当前环境未安装注册这两个操作槽的外部 DSH Client 插件，因此运行态 contribution 数为 0，挂载位置与上下文由组件测试覆盖。
35. P12 Models Settings 批次已完成：新增 `settings.model.provider` keyed 槽与 `settings.model.footer` 列表槽。Provider 槽在已保存模型卡和新增/编辑草稿中都以 `settingsNs = apiType[:protocol]` dispatch，并投影 provider、configId、模型、Base URL、凭据状态、启用状态和测试结果；footer 位于模型 Provider 列表和新增控件之后。DSH `settings.models.provider-card` 与 `settings.models.footer` 已精确映射，平台限定 Windows、macOS、Linux、Web。Catalog 提升至 68/120，剩余 52 项。插件 83/83、前端定向 30/30、Go 定向测试、`go mod verify`、`go vet ./...`、Vite 生产构建和官方 Host/Desktop Client/MCP/Mobile Client 冒烟均通过。Core 从当前源码重建，SHA-256 为 `4081623ef32ecb00be50dabb01df58848db2dd72569ad7c834304d38e5c08125`；`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。`0.1.41` 已完成真实包更新并启用，generation 为 `generation-3fbee3ec-bc16-4bfb-8244-14838346305e`；运行中 Slot 快照 88 项，provider 槽为 root scope、keyed dispatch，footer 为 root scope、list dispatch，均带四平台白名单。当前环境未安装注册这两个 Settings 槽的外部 DSH Client 插件，因此运行态 contribution 数为 0，saved/draft provider 上下文由组件测试覆盖。
36. P12 右侧文档批次已完成：新增 `ui.sidebar.right.pane.document`、`ui.sidebar.right.pane.document.action`、`ui.sidebar.right.pane.document.actions`、`ui.sidebar.right.pane.document.pdf` 和 `ui.sidebar.right.pane.document.unpreviewable` 五个 session-scope 槽，分别承载文档正文、渲染器专属操作、文件操作列表、Office PDF 内容和不可预览处理。DSH 五个对应 Slot 已精确映射；嵌套 `renderSlot`、`renderSlotChain`、`renderFactorySlot` 会先映射 DSH key，再在 owner 授权范围内调用 Amitia child Surface。补充 Client `useContextMenu` 的 file/url descriptor、`reflect.provide`、`configForms`/`remote.settings`、浏览器 `fileDrag`，并实现 `remote.workspaceFiles.read/readBytes/stat`：DSH `sessionId` 经 Owner 会话解析到项目主工作区，文本按行分页，完整字节经 Host 返回，stat 提供绝对路径与版本，路径穿越直接失败。文件 watch、目录列表和搜索仍明确返回不可用。Catalog 提升至 73/120，剩余 47 项。插件全量测试 86/86、前端定向 15/15、Go `internal/extension/kernel` 与 `internal/devicemesh/business` 测试、`go mod verify`、`go vet ./...`、`cmd/server` 编译、Vite 生产构建、官方 Host/Desktop Jobs Client/文档预览 Client/MCP/Mobile Client 冒烟均通过。文档预览官方包注册 `ui.sidebar.right.pane.tab`、title 和五个文档子槽。Core 从当前源码重建，SHA-256 为 `1a6783fabe0388efd492c573424019b52ca0b4d3b49cb5efdd9144ba373d4758`，Source Gate 为 `04b37164c749de74827f503ddde7a584fa8877dc0e51c3ef0143ad4a90ed3e5a`；冻结清单 11263 项校验通过。`0.1.42` 包 SHA-256 为 `db218e01d054d8f7f9be65100703ad3a72ef62fdfd31ca299701d07e5ad91351`，content tree hash 为 `d9d1d83546893b586a623eeb79b8a1077355a9887d674ee6d46849cf7de19c08`，已完成真实包更新并启用，generation 为 `generation-d5f1cf88-f0da-4d6f-aabe-2f5d5978be24`；运行中 Slot 快照 93 项，五个文档 Surface 均为 session scope 并带 Windows、macOS、Linux、Web 四平台白名单。`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。
37. P12 右侧栏 Files/Guide 批次已完成：新增 `app.rightbar`、`app.rightbar.session`、`ui.sidebar.right.pane.files.actions`、`ui.sidebar.right.pane.guide`、`ui.sidebar.right.pane.guide.actions`、`ui.sidebar.right.pane.guide.entry` 和 `ui.sidebar.right.pane.tab.menu.item`。右栏根槽与 session 槽包裹现有固定右侧面板；Files 操作位于刷新控件之后；Guide 使用 chain 正文、root-scope More 菜单行和 keyed provider 卡片；页签菜单接收 tab 与 dismiss 并保持注册顺序。新增显式 `allowRootChild` Slot 契约，允许 DSH 的 root-scope Guide 操作挂在 session Guide 容器内，同时普通未知或越权子槽仍直接失败。Client ModuleLoader 现会跟踪异步 `ctx.effect` disposer；官方 `sidebar-right` 与 `sidebar-files` 均完成加载，Files 的三个官方 Slot Factory 使用独立 `ui.dsh.factory.*` 适配面并在销毁时回收。Client 宿主同时把管理页 UI Context 的 `sessionId` 同步到 DSH `uiSession`，右栏打开、Guide 导航和 Files 操作不再依赖空会话。Catalog 提升至 80/120，剩余 40 项。插件全量测试 87/87、前端右栏/HOC/UI Context 定向测试 12/12、Kernel Slot Registry 与完整 `internal/extension/kernel` 测试、`go mod verify`、`go vet ./...`、Vite 生产构建，以及官方 Host、Jobs Client、Sidebar Right、Sidebar Files、文档预览、MCP、Mobile Client 冒烟均通过。Core 从当前源码重建，SHA-256 为 `d8e65177d0204e913b6416a1bda131aecbf0c455805dd0f30067e6492019f7b8`，Source Gate 为 `28a50a1f41136c930fe6beb72f40dad25096342450c4e1c00371eebc76231117`；冻结清单 11263 项校验通过。`0.1.44` 包 SHA-256 为 `4f1b09ba5940ccce1cb3b7531b615551b6683c1030ffec26d50745b1a684981a`，content tree hash 为 `dfc87c9e3ec66f0f0980f8ddf6ea13540b9de42981433319fa155a11483d500f`，已完成真实包更新并启用，generation 为 `generation-7bf175b0-4487-49bf-acc8-73e2238c56c5`；运行中 Slot 快照 100 项，新增七槽均带 Windows、macOS、Linux、Web 四平台白名单，Guide actions 保留 root scope 与显式跨作用域授权。Core endpoint 为 `127.0.0.1:65505`，`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。
38. P12 Settings Shell 批次已完成：新增 `settings.shell`、`settings.shell.header`、`settings.shell.action`、`settings.shell.close`、`settings.shell.launcher`、`settings.shell.launcher.notifications`、`settings.onboarding`、`settings.section.icon`、`settings.shell.trigger` 和 `settings.model.sign-in`。设置内容页由根单槽包裹，标题位于设置侧栏，操作和关闭按钮位于内容头部；设置启动器、通知与触发行嵌入侧栏账号菜单；分节图标按 settings section id 过滤；模型登录位位于模型配置页凭据编辑器之前。新增可选 `settingsSchema` 同步路径服务，并补充 `remote.credentials`、`remote.llm` 和 `remote.session.modelCatalog` 的显式状态；这些远端读写未映射时继续返回完整不可用错误，不伪造成功。Catalog 提升至 89/120，剩余 31 项。插件全量测试 88/88、前端 Settings/Model/Right Pane/UI Context 定向测试 15/15、完整 `internal/extension/kernel` 测试、`go mod verify`、`go vet ./...`、Vite 生产构建，以及官方 Host、Jobs Client、Sidebar Right、Sidebar Files、文档预览、Settings Models、MCP、Mobile Client 冒烟均通过。Core 从当前源码重建，SHA-256 为 `6a9e012de719db60e761c60d98ace524ee0280bc563060cd39bdc57914bb7169`，Source Gate 为 `99ad1fd8575272ddc7076106a8c1f469003d5bc64ed262d08188382be39e69eb`；冻结清单 11264 项校验通过。`0.1.45` 包 SHA-256 为 `539c518672d360b1d8fcbd19e8ddcd8d945c8229aeda9240ce1b833c790007da`，content tree hash 为 `0057fd6c2b87306ea0c8f2a642db97ad7e7ed374ff8f8ae03e65c3ac0d4bdbc5`，已完成真实包更新并启用，generation 为 `generation-da135461-c298-4030-b80f-d9284aed7a30`；运行中 Slot 快照 110 项，新增十槽均带 Windows、macOS、Linux、Web 四平台白名单。Core endpoint 为 `127.0.0.1:49817`，`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。
39. P12 Plugins Manager 批次已完成：新增 `extension.plugin.tab`、`extension.plugin.item`、`extension.plugin.bundle.activation`、`extension.plugin.bundle.config`、`extension.plugin.row.config` 和 `extension.settings.tab` 六个宿主 Slot。插件页签追加到扩展包页签栏，官方插件条目位于已安装列表之后，Bundle 激活引导按扩展 ID keyed 分派，Bundle 配置和单项配置分别使用 keyed Slot，Settings 插件页签挂载到设置内容页；扩展头部操作、卡片徽标、详情操作和详情分节也补入现有扩展详情界面。DSH `plugins.tab`、`plugins.item`、`plugins.bundle.activation`、`plugins.bundle.config`、`plugins.row.config` 和 `settings.plugins.tab` 均映射到对应 Amitia Slot，其中 `plugins.row.config` 改为独立 keyed 面，避免复用 list 型设置分节造成 kind 冲突。新增 `remote.pluginManager`、`remote.pluginInventory` 和 `remote.pluginRegistryProbe` 客户端服务；包列表、Bundles/Plugins 投影、Bundle/单项启停、安装和移除接入现有 DSH 子插件状态与包生命周期，未具备的预安装检查、版本目录、详情文档和兼容豁免返回带完整原因的显式失败，不伪造成功。Catalog 保持 94/120，剩余 26 项。插件全量测试 89/89、前端 Plugin Manager/Settings/Right Pane/UI Context/App Shell 定向测试 13/13、`go vet ./...`、完整 `internal/extension/kernel` 测试和 Vite 生产构建通过；官方 Host、Plugin Manager、Settings Models、Sidebar Right、Sidebar Files、文档预览、MCP、Mobile Client 冒烟均通过，Plugin Manager 实际注册 `ui.portal.overlay`、`chat.sidebar.panel` 和 `provider.page.provider`。冻结清单已刷新并验证 11265 项；Core 从当前源码重建，SHA-256 为 `eaa157747a43b6ca2b740a5448b6b511f0f6a1f6c6c4b25284bdc75af5adab5e`，Source Gate 为 `722fe290b3703caf8c2c0ea5452c3713dfcc07f9cff1da7c7cace077b3293c49`。`0.1.46` 包 SHA-256 为 `6c7418bf635c7fa8d568408054c1997f34aa2f130a7443e9dc52c164c6c3bdb4`，content tree hash 为 `72dda2fb41a964888fba4eddc6a8f656f7645192cfe3f1130427ad3a7abcfcbf`，已完成真实包更新并启用，generation 为 `generation-f57a9143-52f6-49f7-9aa0-a23398d2e113`；运行中 Slot 快照 116 项，新增宿主面均带 Windows、macOS、Linux、Web 四平台白名单。Core endpoint 为 `127.0.0.1:51437`，`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。
40. P12 Conversation Shell/Composer/Hero 批次已完成：新增 `chat.conversation.root`、`chat.conversation.session`、`chat.conversation.header`、`chat.conversation.session.header`、`chat.conversation.subagent.footer`、`chat.conversation.views`、`chat.composer.root`、`chat.composer.bar`、`chat.composer.attachments`、`chat.empty.brand_mark`、`chat.empty.frame`、`chat.empty.options`、`chat.empty.workspace`、`chat.empty.workspace.directory_flow`、`chat.empty.agent_preset`、`chat.message.user.text` 和 `app.workspace.toast` 十七个宿主 Slot。会话根与 Session 主体包裹现有聊天页；Header、Session Header、子代理 footer、View 列表、Composer chain、Composer bar、附件轨道以及空会话品牌、工作区、目录流、代理预设和选项均挂入真实界面。补齐 `fileUpload`、`remote.directoryPicker`、`connection`、`shortcuts`、Cordis `ctx.plugin`、`uiConversation.events/views/groups` 与 `uiSession` 标准源接口；未具备的 DSH 附件 receipt 和目录列表继续返回完整原始不可用原因。Catalog 提升至 111/120，剩余 9 项；官方 Conversation 与 Workspace 客户端加载通过，Plugin Manager 继续通过。
41. P12 Chat/Tool/Trajectory 收尾批次已完成：新增 `chat.approval.detail`、`chat.message.command`、`chat.conversation.flow`、`chat.message.markdown.fence`、`chat.message.reasoning.body`、`chat.message.text.interaction`、`chat.message.trajectory.images`、`chat.message.tool.images` 和 `chat.message.tool.cordis` 九个宿主 Slot。审批关联工具详情进入审批弹窗；命令消息按命令名 keyed 渲染；消息流、推理正文、Markdown 语言 HOC、文本交互、轨迹图片、工具图片和 `cordis_run` 包内交互分别接入现有消息组件。补齐 `@tanstack/react-virtual` 客户端模块、`require.asset`、`remote.toolResults` 和 `remote.$stream` 边界。Catalog 达到 120/120，剩余 0 项。官方 Conversation、Workspace、Chat、Approval、Tool、Trajectory、Plugin Manager 和 Settings Models 客户端均加载通过，Tool Client 注册 56 个 `chat.message.custom_renderer` 贡献，Chat Client 注册消息流、View、Markdown fence、审批详情和 Composer dock。插件全量测试 89/89、前端定向测试 21/21、`go vet ./...`、完整 `internal/extension/kernel` 测试、Vite 生产构建和冻结清单 11265 项校验通过。Core 从当前源码重建，SHA-256 为 `51696a7046091a7d9959c7c2a2b1dc02e88d47f399d8f47d8aefe2e1c6d9ccb2`，Source Gate 为 `a87998a20537c0eb308a0e8eaab18f3de70709ffd818c16cdbd34919dfe02cd2`。`0.1.47` 包 SHA-256 为 `9d86e0d996b7347e18eab4c74ae78ab5aac3c44a0b0d58ae70bbff0d5160c41c`，content tree hash 为 `7b1d1ff903d19fe96030287aceba0e6c4b6bbeac0255820e6379407fd7b41ec7`，已完成真实包更新并启用，generation 为 `generation-634f8ff4-8bf2-49ff-8373-8286126132c7`；运行中 Slot 快照 142 项。Core endpoint 为 `127.0.0.1:53258`，`/api/health` 与 `/readyz` 均 ready（7 ready、0 degraded、0 blocking、0 failed）。完整 `vue-tsc` 仍仅被既有 `front/src/views/creative-workshop/PetInstallationsView.vue` 缺字段阻断，与本轮无关。
42. P14 运行态桥接收口已完成：`0.1.48` 至 `0.1.69` 依次完成真实 Conversation/Workspace 数据接入、只读 `host.conversation.list`、Session/Workspace 文件能力、桌面目录 IPC、Extension Page 会话身份修正、动作角色上下文、`layout.beginNavigation()`、分块 runtime module 读取和 capability-signed module 路由。最终修复 React 适配器与 DSH wrapper 的契约错位，统一为扁平 props 并兼容 envelope；官方 `useToast`、`useRenameRequest`、`useArchiveRequest` 和 `dismissToast` 均能真实注入。observable hooks 改用 `useSyncExternalStore`，Vue VNode 通过稳定容器转换为 React 节点，React 卸载后不再访问已清空 ref。通用 Slot 运行时的 `parentSlotId = root` 现按根级预声明处理，允许 contribution 显式声明并独占 claim `app.workspace.toast`，同时继续拒绝具体非 root 父 Slot 的越权声明；DSH 在 `session-maybe` 容器下显式声明的 root child 翻译为 `allowRootChild`。嵌套 Slot owner 会继承父 Slot 上下文，避免 capability、scope 和 Session 信息丢失。
43. `0.1.69` 最终验证已完成：安装包 `plugin/amitia-dsh-compatible-host-0.1.69.amitiax`，14321524 字节，SHA-256 `5560310275b355839ad3ba79c1a52df2c20a0b45031b1d51dcf5055f3cafa53d`，content tree hash `f155602c29e557884573f901e2ae5f1a0b5ed5591453f5a9982037ee652f49ff`。经真实 update 操作安装并启用，generation 为 `generation-a4e19310-f9a6-42ac-866d-0499c9ee9481`；完整重启后目标页面加载 generation 96，运行态有 41 个 Client contribution、覆盖 19 个 Slot，`workspace.row-toast` 授权包含 `app.workspace.toast`。官方 Conversation Client（704183 字节，SHA-256 `3d166eab312749bf8c528dd7d26d95340df75a27fec668b3510e4558fe9b343b`）和 Workspace Client（204486 字节，SHA-256 `77da49d837c2503c45ea197129c53d9f679e99bf36737c23eaef457c66f4ee8c`）均通过 runtime module 路由返回 200、`text/javascript; charset=utf-8`、CORS `*`。插件全量测试 93/93、前端 Slot/React Host/页面上下文定向测试 31/31、UI Slot Catalog 120/120 通过；前端全量 `vue-tsc` 仍只被既有 `PetInstallationsView.vue` 缺字段阻断。Core endpoint 为 `127.0.0.1:52267`，PID `69527`，SHA-256 `5a48317960295915b6bcd0f01aa1f370474462ade121c159ae345d3df5a5e480`，Core 构建 Source Gate 为 `81b2d0dc25e9c69a2b0ae611b3e6352a59c2f6e3cecebb505f71236757c00c72`；本轮修改后的冻结清单 11267 项，当前 Source Gate 为 `46c73bd5226c1c234b768658a0737b820366f6213add45be12925834d258c409`。`/api/health` 与 `/readyz` 为 ready（7 ready、0 degraded、0 blocking、0 failed）。
44. `0.1.70` 文件上传回执批次已完成：`fileUpload.upload/uploadImage` 复用现有 `/api/artifacts/v1`，没有建立第二套附件存储；上传完成后以 Amitia Artifact `artifactId` 作为 DSH `receiptId`，支持 `AbortSignal`、XHR 上传进度和完整原始错误/cause。真实 Core 已验证文件与图片上传、内容读取和删除，内容哈希一致。安装包 SHA-256 为 `cc7c99693eac885706f6173952d49fe57b1e011fff1ec8f643fa3078a6088a03`，generation 为 `generation-5c3cfa28-6e9f-4979-aa5f-4210f59c65d8`；插件测试 95/95。
45. `0.1.71` 工作区搜索与变更批次已完成：`remote.workspaceFiles.search` 通过通用 `host.workspace.read/file_search` 接入会话项目主工作区，支持名称或路径查询、大小写不敏感匹配、祖先目录补齐、隐藏项、VCS 元数据、`.gitignore`、字面排除、有界文件和结果上限，并在远程工作区无法完整应用忽略规则时返回 `incomplete=true`。`remote.workspaceFiles.changes` 以 750ms 有界轮询比较文件版本或目录快照，生成 `ready/change/absent` DSH 帧，非“文件不存在”的读取错误继续原样抛出。补全 Command Manifest 中此前缺失的 workspace stat/read/readBytes/search 枚举，并在 runtime module 源发送前删除 `sourceMappingURL`，消除浏览器对不可公开 `.map` 的 403。该中间包被下一批 CSP 修复替换。
46. `0.1.72` 最终验证已完成：安装包 `plugin/amitia-dsh-compatible-host-0.1.72.amitiax`，14339514 字节，SHA-256 `4fa6331ac37c479c92512e6dbabe78a4831a26c52463420d43d52574b77ce0b1`，构建 content tree hash `3ed2b445dec27efe8d73e213969313cc82761f111c834e872e2d0e38545df3c6`，安装 tree hash `95b98615bc1098859832fe5a2fedb259619ba63db38fd1f3553b30db6dbb42dc`。经真实 package preview、confirm 和 update 事务安装并启用，generation 为 `generation-068e221e-1afb-4e40-ad0f-4142663db702`，扩展中心运行 generation 为 102。页面在清空 DevTools 后重载，Issues 从“1 page error”降为 `0 page errors`：Source Map 403 已消失，Zod 在任何 DSH Client 插件加载前启用 `jitless`，不再执行 CSP 禁止的 `new Function("")` 探测。`start-all-services.sh` 现在会枚举本项目进程、等待退出、必要时强制终止，并在 5178 释放后才启动新实例；完整重启未再出现端口竞争。插件全量测试 96/96、`internal/extension/kernel` 全包测试、Kernel `go vet`、冻结清单 11267 项校验通过。Core 从当前源码重建，endpoint 为 `127.0.0.1:53794`，PID `76344`，SHA-256 `44e5d1ef28937e8078c5d6efe909b5a28c84b4990f68c0a50df1c31fff9bd3f8`，Source Gate 为 `1da0656f5e41ceb16a1fcce7802d4f2efb581205abf6cc5e16148c5824c70b0b`；`/api/health` 与 `/readyz` 均为 ready（7 ready、0 degraded、0 blocking、0 failed）。

47. P13 `0.1.73` 接入 `remote.toolResults.watch` 和 Schedule 工作区映射。本机安装后，真实 UI Bridge 拒绝了包含 `seq` 的命令，原始错误为 `input schema instance_mismatch: / additionalProperties(seq)`。
48. `0.1.74` 在现有命令 Schema 声明非负安全整数 `seq`，完成本机包更新并启用。插件测试 103/103。经真实 UI Bridge 验证结果缺失、取消订阅和负序号错误。Schedule 经真实角色上下文验证未知工作区拒绝、创建延迟任务和取消。工具结果从未就绪到可用、已知 `cwd` 到 `projectId` 的映射均通过测试夹具，待用真实 DSH 会话和项目工作区完成验证。

49. `0.1.75` 修复已有 Session 在历史恢复完成前运行工具的问题，避免工具结果序号与历史事件撞号。工具执行与持久化同时失败时保留两个原始错误和堆栈。插件测试 105/105，真实包更新完成，已通过 UI Bridge 验证运行状态和工具结果缺失。协调模式下，经正常接口成功建立的临时设备项目无法通过 DSH Bridge 创建绑定会话，Core 返回「项目不存在或不属于当前角色」。项目 API 明确选择设备作为资源归属，Bridge 的无显式 Scope 查询在协调模式默认选择 Core。临时资源已清理，真实工作区调度和真实会话工具结果转换仍待验证。

50. 协调模式下的无显式快照桥接操作与项目接口统一使用设备数据归属。新建会话调度保存发起设备作为资源所有者，投递前将 Host 会话上下文设置为实际创建的会话 ID。Go 会话创建、宿主读写和 Schedule Session Prompt 定向测试通过。macOS Core 从源码重建并完整重启，7 个组件就绪。通过 `0.1.75` 的真实 UI Bridge 创建临时项目会话、解析 `cwd`、创建并手动触发 Schedule，调度历史确认投递，设备消息接口读到对应消息，新会话归属临时项目并保留工作区路径。临时任务、会话、项目和挂载均已清理。

51. `0.1.76` 完成 `remote.toolResults.watch` 的真实会话验证。本机 Core 安装并启用临时 DSH Host 插件，使用 agents API 创建真实会话，工具通过 Session API 追加并等待 `tool/result` 持久化。UI Bridge 订阅收到从 `available: false` 到 `true` 的转换，序号和结果内容匹配，取消正常结束。修复取消后仍可能发出在途响应的问题，插件测试 106/106，本机包更新完成，临时插件与会话已清理。模型发起工具调用和官方 Client 界面链路仍待单独验证。

52. `0.1.77` 完成 P13 的 `theme` 与 `timer` 两项 Client Service。`theme` 按目标版本官方契约实现 `THEME_PREFERENCES`、`FONT_ROLES`、`FONT_SIZE_SPECS`、`DEFAULT_FONT_SIZES`、`DEFAULT_FONT_FAMILIES` 和 14 项 `--dsw-*` token 检查清单；`DSH_BASE_TOKEN_SOURCES` 把基础层直接映射到 Amitia `--tp-page/panel/panel-soft/float/border/border-strong/primary/text/text-secondary/danger/text-muted/success/warning/shell`，第三方主题 token 与 `overrideTokens` 按 seq 分层叠加在基础层之上；宿主 `html.dark`/`data-theme` 与实际调色板由 `MutationObserver`、`prefers-color-scheme` 同步，`ai-companion-theme` 提供持久化偏好；字号与字体族按角色校验范围后写入 `--dsh-content-font-size`、`--dsh-code-font-size`、`--dsh-terminal-font-size` 和 `--dsh-font-family-*`；`overrideTokens` 不接受裸字符串，`register` 拒绝重复与保留标识；宿主未暴露主题切换动作时 `setTheme` 原文透传 `DshOperationUnavailableError`（含 operation、phase 与 reason），未用固定文案替换。`timer` 提供 Cordis 风格 `timeout`、`interval`、`throttle`、`debounce` 双签名，`createDshClientContext` 额外注入四个方法与 `sleep`，释放时清空挂起任务并拒绝新任务。桌面与移动端每个模块各自创建 `DshClientEventBus`，运行时服务 `emit` 与 `loadDshClientPlugin({eventBus})` 共用同一总线，`ctx.on('theme/change')` 可直接收到快照。安装包 `plugin/amitia-dsh-compatible-host-0.1.77.amitiax`，14398922 字节，SHA-256 `67c672e30a20c96ebd44e84c68b1f66a7d725ede15be12014a19fe472706e75e`，content tree hash `38400e60a0a568b94361feb695e4bc65f6ef5237ed0f912576f8dd48f078602f`；插件测试 122/122 通过；官方 `@deepseek-ai/dsh-client-ui-theme@0.2.2-rc.2` 在本机 Client ABI 容器加载成功并注册两处 `system.settings.section`，官方包以 `ctx.provide('theme')` 覆盖容器注入。`DSH-CLIENT-SERVICE-008 workspaces` 仍因核心缺少项目 create/rename/delete 与会话 archive/unarchive/insertSessionBefore 的通用 Host 能力而保持未实现，Catalog 记录完整原因，未以空成功充当实现。

53. macOS 本机验证模型发起 DSH 工具调用。修复 Owner 上下文接收端缺少 `promptExtensions`、动态工具只安装未激活和缺少能力提供者的问题。JavaScript 运行宿主传递真实会话和工具调用 ID。本地模拟模型通过正常聊天接口收到工具定义并发出调用，工具返回真实会话 ID，模型端收到结果，`tool-result-watch` 从同一会话读到持久结果。临时模型、插件和会话已清理。后端和 JavaScript 宿主定向测试通过，项目从源码重建并完整重启，7 个组件就绪。官方 Tool Client 加载和 Slot 注册通过，真实界面渲染仍待验证。

54. `0.1.78` 完成 P14 生命周期事件与恢复语义。官方 `persistence-catalog` 的 63 项 Session Lifecycle Event（58 项 log-only + 5 项 surface）已从冻结目录进入 `src/runtime/lifecycle-events.generated.mjs`，并由 `src/runtime/session-events.mjs` 统一产出具名信封 `{ type, seq, time, data, ignorable?, surfaceOp?, sourceEventSeqs? }`。写入侧拒绝未声明且未标 `ignorable` 的事件（`DSH_SESSION_EVENT_UNDECLARED`），log-only 事件携带 `surfaceOp`/`sourceEventSeqs` 直接失败；5 项 surface 事件的 `replace` 两端点必须是当前 surface 节点且 `sourceEventSeqs` 覆盖全部被遮蔽节点（`DSH_SESSION_EVENT_SURFACE_RANGE_MISSING` / `DSH_SESSION_EVENT_SOURCE_SEQS_INCOMPLETE`），`agent.session.surfaceEvents()` 输出投影后的 surface 视图。恢复侧对未声明必需事件报 `DSH_SESSION_EVENT_UNRECOGNIZED_REQUIRED`，带 `ignorable` 的未知事件保留并计入容忍计数；会话状态版本提升到 2 并拒绝旧版本（`DSH_SESSION_STATE_VERSION_UNSUPPORTED`），`slice(-512)` 静默截断改为 100000 条上限（`DSH_SESSION_EVENT_LIMIT_EXCEEDED`）。安装包 `plugin/amitia-dsh-compatible-host-0.1.78.amitiax`，14401391 字节，SHA-256 `db0c358d6fa0d98c2eb38074812335f9721f14248ed7126ae2b30c47cfaacabb`，content tree hash `af4ca94174c33ab05c4d3a169902e2db01fff27ef639a98ad3a975ee663b7a17`；插件测试 137/137 通过；`scripts/install-package.mjs` 真实安装后扩展状态为 `installed`、`enabled`、`failureCount: 0`，generation 为 `generation-698f27ed-14ff-48dc-9a78-08b4ae8a0abe`，generation 目录内 `session-events.mjs` 与 `lifecycle-events.generated.mjs` 均已就位。重新生成 Catalog 后 `amitia-gap.v1.json` 的 `lifecycleEvents` 缺口为 0，剩余 57 项 Host Service 与 1 项 Client Service（`workspaces`）。

55. `0.1.79` 完成 P11 首批 Host Service 桥接。新增 `src/runtime/host-service-bridges.mjs`，在 `host-services.mjs` 中挂载 `credentials`、`commands` 与 `sessionQuery` 三个服务对象。`credentials` 完整实现引用解析（先 `host.state` 凭据存储、后进程环境变量，分别标记 `source: 'file'` 与 `source: 'env'`）、`describe` 掩码、`set` 拒绝空值、`unset` 幂等，并以 `host.state` 版本化 CAS 作为 record 半边唯一写路径（`modifyRecord` 冲突重试 8 次后抛 `DSH_STATE_WRITE_CONFLICT`）。`commands` 完整实现注册校验、冻结定义、重名拒绝、按名排序描述符、唯一 file receipt resolver 及 `execute` 的成对 `command/run`/`command/done` 事件，handler 抛错时以不可枚举属性保留原始错误对象并将原始 message 原样写入 `text`，附件不被定义接受时在 handler 前结算为 error 结果且不写事件。`sessionQuery` 落地 16 个方法中的 7 个（`listSessions`、`readSession`、`listEvents`、`readSurface`、`readTitle`、`readTitleSnapshot`、`readTitleSnapshots`），其余 9 个因缺少对应 Amitia 通用能力原样抛 `DSH_HOST_OPERATION_UNAVAILABLE`。`agentPresets` 经核对官方契约后移出首批：`mount`/`recompose` 需要按会话将 preset 的 plugins 组合为 Cordis 子组合，官方消费方还依赖 `agent.ctx` 与 `agent.session.header`，容器与 Core 均无对应能力。安装包 `plugin/amitia-dsh-compatible-host-0.1.79.amitiax`，14426191 字节，SHA-256 `ecf2218b2a0c0359b8bc9a699e961a1fc634534a5f04d63ba877091a6dbe6441`，content tree hash `aa310a58f4beaa978714264dba2dfc676728ef3ed3965e66e45fdcd4ae990502`；插件测试 144/144 通过；`scripts/install-package.mjs` 真实安装后扩展状态为 `installed`、`enabled`、`failureCount: 0`，最新 generation 为 `generation-b35ef574-2bc9-4641-8747-e5b5b7ecccd3`，目录内 `host-service-bridges.mjs`、`session-events.mjs` 与 `lifecycle-events.generated.mjs` 均已就位；`/api/health` 与 `/readyz` 均为 ready（7 ready、0 degraded、0 blocking、0 failed）。重新生成 Catalog 后 `lifecycleEvents` 缺口仍为 0，剩余 54 项 Host Service 与 1 项 Client Service（`workspaces`）。

56. `0.1.89` 完成 P13：`workspaces` Client Service 映射到通用项目与会话归档能力。Core 新增 `host.project.list|create|update|delete` 与 `host.conversation.archive`，新增权限 `project.read`（RiskLow/PersistentGrantable）与 `project.manage`（RiskHigh/RequiresPerUse/ApprovalManual/ChildDeny），`conversation.archive` 归入既有 `conversation.manage`；五个方法常量在 `DefaultPermissionMapping()`、`RoutePermissionForMethod`、`RouteScopeForMethod`、`IsDataRouteMethod`、`RouteRiskForMethod`、`RouteSideEffectForMethod` 全部登记。`host.project.create` 增加 `directoryPath`，复用 `workspace.Service.RegisterLocalMount` 生成真实本地挂载后交给 `business.CreateProject`，与显式 `workspaces` 互斥，不新增第二套项目系统。插件 runtime 新增 `conversation-archive`、`project-catalog`、`project-workspaces`、`project-create`、`project-update`、`project-delete` 六个 action；`projectScopeInput` 透传可选 `spaceId`/`characterId`；`conversation-archive` 优先请求内 `scopeSnapshotId`，回退 `dsh-session:<id>` 持久化快照后走 `host.callTargeted`；新增 `dsh-workspace-index` 状态（`host.state.cas` 最多 8 次重试）并在 `sessionCatalog.refresh` 中与会话派生项按 `workspaceId` 合并，使新建项目立即出现在 DSH Workspace 目录。`ctx.workspaces` 的 `create`/`rename`/`delete`/`archiveSession`/`unarchiveSession` 五个方法落地，`rename`/`delete` 缺少已验证 `ProjectId` 映射时抛原始错误，`insertSessionBefore` 保持 `unsupported("P13")`。Manifest 增加 `project.read`/`project.manage` 权限与 6 个 action、8 个输入属性。catalog 中 `workspaces` 由 `unavailable` 变为 `partial`（`mapping: composite`），client services 8 项为 implemented 4、partial 4，host services 69 项为 delegated 27、partial 9、implemented 6、pending 14、unavailable 13，`amitia-gap.v1.json` 的 client 缺口只剩 `insertSessionBefore` 排序一项。安装包 `plugin/amitia-dsh-compatible-host-0.1.89.amitiax`，14711483 字节，SHA-256 `598854d3a0e01a8a4a99e3d46dfed404d0f9fdf2bfb930dda42e1583dea106ec`，treeHash `49645f85d38af11cc17d546726220cfd4455ef09dd956a52f11448fc4a36808e`；插件测试 164/164 通过；`scripts/install-package.mjs` 安装后 `version: 0.1.89`、`enabled: true`、`failureCount: 0`。以当前源码重建 darwin/arm64 `AmitiaCore` 并重启完整服务后 `/api/health` 为 `ready: true`、runtime `state: ready`（7 ready、0 degraded、0 blocking、0 failed）。真实链路探测确认 `project-catalog` 已进入 Core 的 `host.project.list` 并原样返回业务侧 `input_invalid`；带 page session 的 UI 端到端验证与 `insertSessionBefore` 仍待完成。

57. `0.1.96` 完成 P11 批次 D 前半段（§23.14）。shared container realm 让 `storage`/`storageDomain` 对兄弟包可见；`provider-wiring.mjs` 把 `dsh-storage-json`、`dsh-session-persistence-jsonl`、`dsh-spill-local` 的落盘目录统一接到 `plugin-data/dsh-data/{storage,sessions,spill}`，并为 `dsh-storage-domain` 注入必填 `backend: "json"`；`host-abi.mjs` 新增声明型契约包判定，`@deepseek-ai/dsh-session-persistence` 不再被当插件挂载并抢占 `sessionPersistence`，`@deepseek-ai/dsh-lsp` 因其原型方法保留为真 seam；npm 安装参数追加 `--ignore-scripts`，`koffi` 走平台预编译包正常加载。安装包 `plugin/amitia-dsh-compatible-host-0.1.97.amitiax`，14716593 字节，SHA-256 `6aa0136f1613692ef92bbd59ed867f069c4bb53db63f498ff5f2a25b5bcbae65`，treeHash `f9f73da1423ec6f35fd21ccf7d1e0a6180bb22d62c3ef00b215198c1282c6ed3`；插件测试 168/168 通过。0.1.97 修复声明型判定误读原型 getter 导致的 `Cannot read properties of undefined (reading 'view')`。真实 Core 启用后 `dsh-storage`、`dsh-storage-json`、`dsh-storage-domain`、`dsh-session-persistence-jsonl`、`dsh-typert-registry`、`dsh-api-job-controller` 均为 `active: true`，`dsh-data/storage` 与 `dsh-data/sessions` 目录已创建；`dsh-workspace` 越过 `storageDomain` 检查后停在原始错误 `fsync API is disabled when Permission Model is enabled.`，该错误在 Node v26.12.1 任何 `--permission` 组合下均可复现，属当前阻塞项，决策见 §23.14.6。

这些证据证明当前主体链路可用，不证明目标版本 100% 兼容。

## 18. 当前主要缺口

| 缺口 | 影响 | 下一阶段 |
| --- | --- | --- |
| DSH ABI Catalog v1 已冻结，但实现覆盖率仍不足 | 目录可证明缺口，尚不能证明兼容 | P11/P13 |
| 生命周期事件已按官方信封全部落地，但未在官方真实会话历史上回放验证 | `surfaceOp.replace` 与 `sourceEventSeqs` 的正确性仅在单元测试和容器内会话验证 | P16 |
| UI Slot Catalog v1 已冻结，120 项均有目标映射 | 仍需继续验证全部 Slot 的跨平台真实渲染 | P12/P16/P17 |
| Agent setup/seed/fork 未映射 | 无法创建带任意 Session projection、fork history 或 scoped setup 的 Agent | P11 |
| `agentPresets` 未实现 | preset 的 mount/recompose 需要按会话组合 plugins，官方消费方还依赖 `agent.ctx` 与 `agent.session.header`，容器与 Core 均无对应能力 | P11（转入需 Core 新增通用能力的一批） |
| `sessionQuery` 只实现 16 个方法中的 7 个 | observe/inspect/search/filter/trace 与单事件读取缺少对应 Amitia 通用能力 | P11 |
| Subagent send/drain 不完整 | 多 Agent 语义不完整 | P11 |
| MCP prompts/sampling/elicitation 未完成 | MCP 只覆盖 subset | P11/P16 |
| Remote、Sessions、Workspace service 仍不完整 | 持久计划读取、会话切换和新建会话仍受限制 | P13 |
| Client `workspaces` 未实现 | 项目 create/rename/delete 与会话 archive/unarchive/insertSessionBefore 缺核心通用 Host 能力，依赖核心新增项目与会话归档 API 后再闭环 | P13 |
| DSH Schedule 高级语义未完成 | 自定义 `cwd` 已通过 macOS 协调模式的真实项目投递验证。`agentPreset` 映射尚未实现 | P13/P16 |
| 包管理未保存可恢复历史版本 | 更新失败可保留旧版本，但无法主动回滚已成功的新版本 | P8/P14 |
| 样式隔离未完成 | React CSS 可能污染宿主 | P4 |
| 动态 Slot 跨插件清理未完整验证 | 可能出现悬挂贡献 | P5/P14 |
| 官方夹具矩阵不完整 | 无法宣称 100% | P16 |
| 正式发布、跨平台真机和完整用户旅程未完成 | 不能宣称目标版本 100% 兼容或可发布 | P17/P18 |
| 跨平台未验证 | 不能确认平台一致 | P17 |

P11 阶段状态：首批已落地 `credentials`（完整）、`commands`（完整）与 `sessionQuery`（16 个方法中 7 个），Host Service 缺口由 57 项降至 54 项。`agentPresets` 经契约核对后不满足“纯映射”前提，转入需要 Core 新增通用能力的一批；`sessionQuery` 剩余 9 个方法与 163 项 config 缺口同样依赖 Core 侧通用能力，需先出方案再实施。

P11 优先级依据（对 `@deepseek-ai/dsh@0.2.2-rc.2` 完整依赖树 339 个 `@deepseek-ai/*` 包做静态 `ctx.*` 引用统计，计数包含类型声明与方法调用，仅用于排序）：54 项 Host Service 缺口中有 42 项被官方包引用，12 项未被引用。引用量前列为 `sandboxPolicy`（109 次、22 包）、`workspaceRegistry`（101/7）、`credentials`（94/10）、`shell`（94/11）、`web`（92/7）、`subprocess`（87/16）、`sessionQuery`（85/13）、`remoteAccess`（80/15）、`connection`（58/14）、`sessionPersistence`（58/15）、`goals`（57/6）、`terminals`（57/5）、`typert`（57/5）、`sandbox`（48/10）、`skills`（45/9）、`agentPresets`（35/7）、`webServer`（34/9）、`authorization`（33/4）、`spillStore`（30/5）、`directoryPicker`（29/5）。按消费方集中度看，`dsh-api-session-controller`、`dsh-api-workspace-controller`、`dsh-tool-cordis`、`dsh-deepseek-account`、`dsh-tool-fs-search`、`dsh-tool-web`、`dsh-subagent`、`dsh-schedule` 是主要调用方。未被任何官方包引用的 12 项为 `agentLoop`、`credentialsController`、`directoryPickerController`、`jobController`、`sessionFileReferences`、`sessionLogDeepSeek`、`sessionReferenceResolver`、`sessionSkillCatalog`、`sessionTelemetry`、`settingsController`、`subagentModelSelection`、`workspaceController`。

## 19. 主要风险与处理

| 风险 | 影响 | 处理 |
| --- | --- | --- |
| DSH Slot 语义偏离 Amitia | 无法完整渲染 | 固定版本、精确 Catalog、禁止生产启发式 fallback |
| React/Vue 生命周期差异 | 状态丢失或泄漏 | Foreign Component Host、StrictMode/卸载测试 |
| Service 语义不完全等价 | Host 插件行为不一致 | 直接映射、组合映射或容器内兼容实现 |
| Cordis API 被依赖 | 无 shim 时无法加载 | 包内最小符号 shim，持续测官方包 |
| 空实现伪装成功 | 兼容矩阵失真 | 明确失败和假成功审计 |
| 已有未提交改动 | 发布门禁阻塞或基线污染 | 先确认归属，不混入发布基线 |
| 移动端能力不足 | 无法跨端一致 | 平台单独门禁，不静默降级 |
| 大量动态贡献 | 性能下降 | 预算、延迟加载、压力和泄漏测试 |
| 包来源不可复现 | 无法审计 | 固定版本、lockfile、来源哈希和安装记录 |

## 20. 变更控制

任何新增 DSH 能力、Slot、服务或平台必须同步更新：

1. ABI Catalog。
2. UI Slot Catalog。
3. 容器映射。
4. Amitia 通用能力差异矩阵。
5. 平台支持矩阵。
6. 成功、失败和清理测试。
7. 错误与权限路径。
8. 发布说明。

以下情况必须重新打开 P0：

1. DSH 目标版本变化。
2. DSH Slot ID 或语义变化。
3. Client UI Primitives 版本变化。
4. 新增平台。
5. 把某个明确失败项改成已支持。
6. 新增 Host Service 或 Agent/MCP 语义。

## 21. 需求与阶段追踪

| 需求 | 主要负责阶段 | 最终证据 |
| --- | --- | --- |
| UR-01 | P0、P7、P18 | 正式 Amitiax 包及真实安装记录 |
| UR-02 | P0、P7、P8 | 容器内 DSH 包列表和 plugin_data 路径 |
| UR-03 | P0、P7 | 扩展中心无 DSH 子插件 |
| UR-04 | P0、P9-P13 | Host/Client/Service/UI 映射矩阵 |
| UR-05 | P1-P6、P10、P12、P13 | 真实 Surface 渲染和多插件测试 |
| UR-06 | P0-P6、P15 | 核心通用能力审计和 DSH 分支扫描 |
| UR-07 | P1-P6 | 全 Surface、Slot、HOC、Portal 和跨插件验收 |
| UR-08 | P0、P16、P20 | ABI/Slot Catalog 和 100% 覆盖报告 |
| UR-09 | P7、P15 | 无 Harness 进程和依赖审计 |
| UR-10 | P7、P15、P20 | 无 Cordis 核心依赖和运行引擎 |
| FR-01 | P7、P18 | 普通容器安装记录 |
| FR-02 | P8、P14 | 子插件私有目录和卸载证据 |
| FR-03 | P8、P16 | npm、tarball、目录和固定 Git 测试 |
| FR-04 | P8-P10、P16 | bundle 多入口夹具 |
| FR-05 | P9、P16 | Host ABI 夹具结果 |
| FR-06 | P4、P10、P16 | Client ABI 和 React 夹具结果 |
| FR-07 | P9、P11、P16 | systemPrompt 到真实 Prompt 链路 |
| FR-08 | P11、P14、P16 | Session 持久化和 projection 恢复 |
| FR-09 | P11、P16 | Agent 生命周期测试 |
| FR-10 | P11、P16 | Subagent provider 和 children 测试 |
| FR-11 | P11、P16 | MCP 全部能力矩阵 |
| FR-12 | P9、P11、P16 | Tool schema、调用、取消和错误测试 |
| FR-13 | P1-P6、P10、P12、P16 | Slot Catalog 和渲染测试 |
| FR-14 | P1、P4、P12 | HOC/children/重挂载测试 |
| FR-15 | P3、P12 | Portal 生命周期测试 |
| FR-16 | P5、P14 | 自定义 Slot 和递归清理测试 |
| FR-17 | P9、P10、P14 | 故障隔离和残留检查 |
| FR-18 | P8、P14、P18 | 安装到卸载完整旅程 |
| FR-19 | P14、P18 | 完整重启恢复证据 |
| FR-20 | P0、P16、P19 | 版本化兼容矩阵 |
| FR-21 | P9-P15 | 原始错误链审计 |
| FR-22 | P0、P19 | 三类独立版本和发布记录 |
| NFR-01 | P7、P14、P17 | 故障隔离和平台健康检查 |
| NFR-02 | P8、P12、P14 | 数据路径和包内容审计 |
| NFR-03 | P8、P14 | 事务、回滚和恢复测试 |
| NFR-04 | P1、P4、P13、P14 | 预算、延迟加载和泄漏测试 |
| NFR-05 | P14、P15 | 结构化和可追踪错误日志 |
| NFR-06 | P7、P15、P20 | 核心无 DSH 专用代码 |
| NFR-07 | P16 | 自动化测试和夹具矩阵 |
| NFR-08 | P18 | 包白名单和构建产物审计 |
| NFR-09 | P19 | Catalog、报告、矩阵和哈希归档 |
| NFR-10 | P15 | 全局错误交叉审计 |
| NFR-11 | P17 | Manifest 与真实平台证据一致 |
| NFR-12 | P1-P6、P15 | 通用 Surface 扩展审计 |

## 22. 最终完成定义

满足以下全部条件才可标记完成：

1. Amitia 通用 UI 已具备 Slot、Context、Portal、Foreign Component、自声明 Slot 和跨插件组合。
2. 目标 DSH 全部公开 Host API 已映射或有容器内兼容实现。
3. 目标 DSH 全部公开 Client API 已映射。
4. 目标 DSH 全部公开 UI Slot 已映射。
5. 普通容器插件可独立安装、启用、停用、恢复和卸载。
6. DSH 子插件不注册为 Amitia 扩展。
7. Amitia 核心不依赖 Cordis。
8. Amitia 核心不运行 Harness 本体。
9. Amitia 核心不存在 DSH 专用业务分支。
10. Agent、Session、Subagent、MCP、Jobs 和 Approval 有真实证据。
11. 原始错误链完整。
12. 目标 ABI 和 UI Slot 覆盖率为 100%。
13. 所有声明平台通过真实安装和端到端验证。
14. 不存在未分类跳过项、启发式掩盖项和空实现成功项。

最终结论：

1. 当前方案可行，但还没有达到 100% 兼容。
2. UI 补齐后，Amitia UI 可以作为通用可扩展宿主，并且具备承载 DSH UI 的必要底座。
3. DSH 100% 兼容仍必须由独立普通 Amitiax 插件完成 Host ABI、Client ABI、服务、UI、生命周期和测试映射。
4. 不引入 Cordis、不运行 Harness 本体、不新增 DSH 专用核心分支，是项目的固定架构边界。

## 23. P11 批次 B 方案（待确认）：DSH 服务容器与官方服务包加载

### 23.1 现状证据

1. `src/runtime/host-abi.mjs:210-236` 的 `loadDshHostPlugin` 为每个 DSH 包入口创建独立上下文，`services` 直接取自调用方传入的固定表。
2. `src/runtime/index.mjs:88-98` 的 `activatePackage` 用 `createAmitiaDshHostServices` 一次性生成固定服务表，同一 `record` 下的多个 `entry` 共享该表，不同包的 `record` 互不可见。
3. `src/runtime/host-abi.mjs:117-131` 的 `ctx.provide()` 只写入当前插件上下文对象，不会进入其他包的上下文。
4. `src/runtime/host-abi.mjs:60-67` 的 `assertServices` 在加载前用固定表断言 `inject`，缺服务直接抛错。
5. 官方把大量能力实现为“服务包”：`@deepseek-ai/dsh-lsp` 的 `lib/types/index.d.ts` 中 `class Lsp extends Service implements LspService` 本身就是 `ctx.lsp` 的定义与实现，`apply` 装载即注册服务；`@deepseek-ai/dsh-shell-env` 的 `ShellEnvRegistry extends Service` 同理。
6. 运行时验证（0.1.79，本机 Node）：包 A 调用 `ctx.provide("probeService")` 后，A 自身上下文可见该服务，而传给 `loadDshHostPlugin` 的服务表仍为空（`[]`）；随后包 B 声明 `inject: ["probeService"]` 直接失败，原始报错为 `DSH 插件依赖的服务 probeService 不可用`。即跨包服务在容器当前实现下确定不可见。

结论：剩余 54 项 Host Service 缺口里，相当一部分不是“缺 Amitia 通用能力”，而是“容器没有跨包服务容器”，导致官方服务包注册的服务无法被其他 DSH 包注入。按 23.4 逐项分类，54 项中 27 项属于这一类。

### 23.2 行业标准对照

Cordis 的服务语义是：服务由插件的 `Service` 子类或 `ctx.provide` 注册到当前 fiber 的上下文，`inject` 声明依赖并在依赖就绪后才执行，同一层重复提供同名服务是错误，子上下文可遮蔽父上下文服务，fiber 释放时服务随之注销。容器要支持官方服务包，必须实现同一套语义，而不是给每个包发一份互相隔离的服务表。

### 23.3 设计

> 本节为初版设计，已被 23.8 修订取代：实测确认容器可直接运行真实 `@deepseek-ai/cordis`，因此不再自建注册表、自写 generator effect 与 Proxy 视图。本节与 23.3.1、23.3.5、23.3.6 保留作为调研记录与“跨包服务不可见”的证据，实施以 23.8 为准。

1. 新增容器级 `DshServiceRegistry`：保存每个服务的提供者栈（容器内置层 + 各 DSH 包层）。DSH 包读取服务时按“自身层 → 容器内置层”解析，未命中时按 23.3.3 报错。
2. 每个 DSH 包得到子上下文：`provide` 写入该包自己的层并登记释放；包停用时逆序注销该包全部服务，父层服务不受影响。
3. 同一层重复 `provide` 同名服务抛 `DSH_SERVICE_DUPLICATE`，错误包含服务名与两个提供者；子层遮蔽容器内置服务需要显式声明，未声明时抛错而不是静默覆盖。
4. `inject` 断言改为在加载前向注册表解析；缺失时抛 `DSH_SERVICE_UNAVAILABLE`，包含服务名、包名与“无提供者/提供者未启用”两类原因，禁止空实现顶替。
5. 生命周期：服务包与消费包按 manifest 声明顺序加载，消费包停用不影响服务包；服务包停用而其服务仍被已启用包引用时拒绝停用并原样报错，不做级联静默停用。
6. 权限边界：容器进程仍以 `--permission` 运行。服务包需要的文件、进程、网络能力只能经 `host.resource.*`、`host.network.*`、Core 通用能力代理，禁止为提高兼容性放宽 Node 权限。
7. 不新增 DSH 专用核心分支：注册表完全位于插件运行时，Core 无感知。

### 23.3.1 实施改动清单

1. 新增 `src/runtime/service-registry.mjs`：导出 `createDshServiceRegistry()`，提供 `register(ownerId, name, value, options)`、`resolve(name, requesterId)`、`releaseOwner(ownerId)`、`providers(name)`、`assertAvailable(names, requesterId)`、`snapshot()`；错误码固定为 `DSH_SERVICE_DUPLICATE`、`DSH_SERVICE_UNAVAILABLE`、`DSH_SERVICE_SHADOW_DENIED`、`DSH_SERVICE_IN_USE`，message 保留原始上下文，不做文案替换。
2. `src/runtime/host-abi.mjs` 的 `createDshHostContext` 增加 `registry` 选项；`provide`（117-132 行）改为向注册表登记并把释放函数挂到本上下文 disposers；服务属性通过 `Object.defineProperty(context, name, { get })` 暴露，读取走注册表解析，`set` 与直接赋值统一转到注册表，避免两套存储。
3. `src/runtime/host-abi.mjs` 199-201 行的固定表复制改为把容器内置服务注册到注册表的内置层；`assertServices`（60-66 行）改为 `registry.assertAvailable(inject, pluginId)`。
4. `src/runtime/host-abi.mjs` 的 `loadDshHostPlugin`（226-240 行）在加载失败时调用 `registry.releaseOwner(pluginId)`，不留下半注册服务。
5. `src/runtime/index.mjs` 的 `activatePackage` 提升注册表为实例级单例并在包之间共享；`deactivatePackage` 调用 `releaseOwner`，若服务仍被已启用包引用则抛 `DSH_SERVICE_IN_USE`，不做级联静默停用。
6. `src/runtime/compat/cordis.mjs` 不改：`Service` 构造函数已经调用 `ctx.provide`，语义随第 2 条自动生效。
7. 新增测试覆盖：跨包注入成功、同层重复提供、未声明遮蔽、缺失服务、消费包停用不影响服务包、服务包停用被引用。测试断言错误码与原始 message，不降级断言。

### 23.3.2 兼容层前置条件（本轮实测新增）

只用服务注册表还不够，官方服务包还依赖两件兼容层能力，本轮已用真实官方包验证：

1. **官方服务包可以被容器实例化**：把 `@deepseek-ai/dsh-lsp@0.2.2-rc.2` 的 `lib/index.js` 交给 `loadDshHostPlugin` 后，`default` 类被正确实例化，`context.lsp.constructor.name` 为 `Lsp`，`registerProvider` 为函数。它只导入 `@deepseek-ai/cordis` 与 `@deepseek-ai/dsh-llm`，现有 compat 映射足以满足导入。
2. **`ctx.effect` 缺 generator 语义，且是静默失败**：`Lsp.registerProvider` 用 `this.ctx.effect(function* () { …; yield () => {…} }, "lsp.registerProvider()")` 提交登记。容器 `host-abi.mjs:150-155` 的 `effect` 直接 `factory()` 取返回值，generator 调用只返回迭代器，函数体从未执行。实测结果：`registerProvider` 返回了 disposer，但 `providerIds` 为 `[]`、`routes` 为 `[]`，随后 `query({filePath:"/tmp/probe.ts"})` 报 `LspError LSP_UNAVAILABLE | no LSP provider handles "/tmp/probe.ts"`。注册“成功”却什么都没登记，属于静默失败。
3. 由此新增两项兼容层改动：`effect` 必须支持 generator（执行到首个 yield，收集 yield 出的 disposer，`return` 时逆序释放并调用 `iterator.return()`），非 generator 保持现有行为；`ctx.effect` 不接受空 label 与无效工厂，异常原样抛出。
4. 官方包依赖的真实模块走 npm 安装，不自造 shim：`Lsp` 只需要 compat 的 `cordis` 与 `dsh-llm`；`@deepseek-ai/dsh-shell-env` 还需 `@deepseek-ai/dsh-error`、`@deepseek-ai/dsh-home-paths`、`@deepseek-ai/dsh-shell`、`@deepseek-ai/schemastery`，这些是官方发布的真实包，由 `package-manager.mjs` 的 npm 安装流程提供，禁止为它们写第二套语义。本轮已实测该流程可用：`npm install @deepseek-ai/dsh-shell-env@0.2.2-rc.2` 成功装入 48 个包。
5. **兼容映射会整包短路，导致官方包 import 直接失败**：真实 `@deepseek-ai/dsh-shell/lib/index.js` 从 `@deepseek-ai/dsh-subprocess` 导入 `DSH_ENV_PREFIX`，而容器把该 specifier 映射到 `src/runtime/compat/dsh-subprocess.mjs`，该 shim 只导出 `scrubbedParentEnv`，于是加载 `dsh-shell-env` 时抛出 `SyntaxError: The requested module '@deepseek-ai/dsh-subprocess' does not provide an export named 'DSH_ENV_PREFIX'`，整条 import 链失败。实测扫描：官方包从 `@deepseek-ai/dsh-subprocess` 只导入 `DSH_ENV_PREFIX` 一个符号，说明该包本不该被映射。
6. 由此确定兼容映射的两条硬规则：只映射“必须由 Amitia 提供实现”的包（LLM、工具、会话、作用域、附件等），纯基类或纯常量包（`dsh-subprocess`、`dsh-shell`、`dsh-spill`、`dsh-sandbox` 一类）一律装真实包；每个被映射包的导出面必须是官方包导出面的超集，并加守卫测试直接比对官方 `lib/index.js` 的导出名与 shim 的导出名，缺失即失败。

### 23.3.3 兼容映射修正清单

1. `src/runtime/host-abi.mjs` 的 `compatModules`（68-82 行）移除 `@deepseek-ai/dsh-subprocess`，改由真实包解析；其余映射逐包核对导出面。
2. 新增守卫测试：遍历每个被映射的 specifier，读取官方包 `lib/index.js` 的导出名集合，断言 shim 导出名覆盖该集合，缺失时输出缺失符号清单。
3. 新增 A 类第二试点：真实 `@deepseek-ai/dsh-shell-env` 在容器内 `apply` 后 `ctx.shellEnv.register/collect/list` 可用，作为“官方服务包 + 真实依赖”的验收证据。

### 23.3.4 实施改动清单补充

1. `src/runtime/host-abi.mjs` 的 `effect`（150-155 行）增加 generator 分支，与 23.3.1 的注册表改动同批落地。
2. 新增测试：generator effect 立即执行到首个 yield、yield 出的 disposer 在释放时逆序调用、generator 抛错原样透传、非 generator 行为不变。
3. 新增 A 类试点用例：用真实 `@deepseek-ai/dsh-lsp` 包注册 provider 后 `query` 能命中路由，作为 A 类可用的验收证据。

### 23.3.5 原型验证结果（未改产品源码）

为确认 23.3 的设计能真正点亮官方包，我在 `.tmp-spec3/proto` 里复制一份 `host-abi.mjs` 做原型（新增 `service-registry.mjs`、移除 `dsh-subprocess` 映射、`effect` 加 generator 分支、上下文加注册表代理），跑通以下试点；原型目录不入库，实施落地后随临时目录一并删除。

1. 跨包提供与注入：包 A `provide("probeService")` 后注册表为 `{"probeService":["probe-provider"]}`，包 B 声明 `inject:["probeService"]` 通过，`ctx.probeService.ping()` 返回 `pong`。
2. 服务仍被引用时停用：`DSH_SERVICE_IN_USE`，详细信息为 `[{"service":"probeService","consumers":["probe-consumer"]}]`。
3. 提供方自身释放后注册表回到 `{}`，消费方不再解析到该服务。
4. 真实 `@deepseek-ai/dsh-lsp`：`registerProvider` 后 `routes` 为 `[".ts"]`，`query({filePath:"/tmp/probe.ts"})` 返回 `{"ok":true,"languageId":"typescript"}`，disposer 执行后 `routes` 回到 `[]`。
5. 真实 `@deepseek-ai/dsh-shell-env`：`register` 后 `list()` 为 `[{"contributor":"probe","description":"probe","key":"DSH_PROBE"}]`，`collect({})` 返回 `{"DSH_HOME":"…/.dsh","DSH_NODE_EXECUTABLE":"…","DSH_PROBE":"1","DSH_SHELL":"1"}`。

原型阶段还暴露一个设计要点，必须并入实施：**插件上下文要能动态看到后注册的服务**。只把服务写进 `context` 对象时，先加载的消费包读不到后加载服务包提供的服务（试点第 2 项首次结果为 `null`）。做法是把返回给插件的上下文包成 Proxy：自身属性优先，未命中时向注册表按 `pluginId` 解析。加上代理后试点第 2 项立即返回 `pong`。

### 23.3.6 实施改动清单（合并版）

1. 新增 `src/runtime/service-registry.mjs`（见 23.3.1 第 1 条）。
2. `host-abi.mjs`：`provide` 写注册表、`assertServices` 走 `registry.assertAvailable`、`effect` 支持 generator、内置服务注册为 `builtin` 层、插件上下文改为 Proxy、`loadDshHostPlugin` 失败时回滚该包服务。
3. `host-abi.mjs` 的 `compatModules` 移除 `@deepseek-ai/dsh-subprocess`。
4. `index.mjs`：注册表单例化并跨包共享，`deactivatePackage` 走 `releaseOwner`。
5. 测试：注册表六类语义、generator effect 四项、Proxy 动态可见性、lsp 与 shell-env 两个真实包试点、兼容映射导出面守卫。

### 23.4 缺口重分类

按“该服务实现需要什么”分三类，依据官方文档每个服务章节的 Source 包路径与章节内声明的依赖：

1. A 类（27 项，官方包实现）：实现包本身就是官方 DSH 插件，容器只要具备跨包服务层与加载顺序即可提供服务，无需新能力。
2. B 类（24 项，需宿主代理）：官方实现要求文件、进程、终端、网络、持久化、系统对话框或沙箱，容器必须代理到通用 Host 能力，或在对应能力不存在时按失败关闭明确报错。
3. C 类（3 项，需 Core 能力）：服务语义绑定 Amitia 产品对象，必须先由 Core 新增通用 API。

#### 23.4.1 逐项清单

| 序号 | 服务 | 官方实现包 | 类别 | 依据 |
| --- | --- | --- | --- | --- |
| 1 | `agentDefaultModel` | `packages/core/agent-default-model` | A 官方包实现 | 官方读取 Config 的默认模型选择，无宿主依赖 |
| 2 | `agentLoop` | `packages/core/agent-loop` | A 官方包实现 | 官方 agent 工厂与驱动，依赖官方 llm 与 session 包 |
| 3 | `agentPresets` | `packages/preset/agent-preset-registry` | A 官方包实现 | 官方注册表；mount/recompose 需容器支持动态子组合 |
| 4 | `authorization` | `packages/credentials/authorization` | A 官方包实现 | 官方凭据流程注册表，记录写入走已实现的 credentials |
| 5 | `clientModules` | `packages/client/modules` | A 官方包实现 | 官方 Web 插件表，桌面端不消费 |
| 6 | `compaction` | `packages/compaction/compaction` | A 官方包实现 | 官方 seam 定义，具体实现 compaction-basic 同属官方包 |
| 7 | `connection` | `packages/client/connection` | A 官方包实现 | 官方 wire 组合，桌面端仅需服务可见性 |
| 8 | `credentialsController` | `packages/api/settings-controller` | A 官方包实现 | 官方 Host API 控制器，底层 credentials 已实现 |
| 9 | `deepseekAccount` | `packages/credentials/deepseek-account` | B 需宿主代理 | 登录需访问官方账号服务，需网络代理与凭据注入 |
| 10 | `deepseekLlmApiExtensions` | `packages/llm/deepseek-llm-api-extensions` | A 官方包实现 | 官方请求字段注册表，纯内存 |
| 11 | `directoryPicker` | `packages/host/directory-picker` | B 需宿主代理 | 需 OS 目录选择，代理到通用 ui.dialog |
| 12 | `directoryPickerController` | `packages/api/workspace-controller` | B 需宿主代理 | 依赖 directoryPicker 的 OS 选择能力 |
| 13 | `fileReferences` | `packages/context/file-reference` | B 需宿主代理 | 需遍历工作目录，代理到通用文件读取 |
| 14 | `goals` | `packages/goal/goal` | A 官方包实现 | 官方实现，以会话日志为唯一来源 |
| 15 | `jobController` | `packages/api/job-controller` | B 需宿主代理 | 需作业运行时与进程信息 |
| 16 | `lsp` | `packages/lsp/lsp` | B 需宿主代理 | 需拉起语言服务器子进程 |
| 17 | `permissionPresets` | `packages/interaction/permission-presets` | B 需宿主代理 | 官方声明需要 ctx.shell 执行器与 ctx.approval |
| 18 | `planMode` | `packages/plan/plan-mode` | A 官方包实现 | 官方实现，状态写入会话日志 |
| 19 | `ptcRuntime` | `packages/ptc-runtime/ptc-runtime` | B 需宿主代理 | 需隔离执行程序，依赖子进程与沙箱 |
| 20 | `remoteAccess` | `packages/api/remote-access` | A 官方包实现 | 官方访问策略注册表，纯内存 |
| 21 | `sandbox` | `packages/sandbox/sandbox` | B 需宿主代理 | 必须由宿主给出 confining 实现或按失败关闭 |
| 22 | `sandboxPolicy` | `packages/sandbox/sandbox-policy` | A 官方包实现 | 官方策略服务，执行侧才需要 sandbox |
| 23 | `sessionController` | `packages/api/session-controller` | A 官方包实现 | 官方 Host API 控制器，依赖会话持久化与标题 |
| 24 | `sessionFileReferences` | `packages/api/session-controller` | B 需宿主代理 | 依赖 fileReferences 的工作目录枚举 |
| 25 | `sessionLogDeepSeek` | `packages/session/session-log-deepseek` | A 官方包实现 | 官方日志贡献者，纯内存过滤规则 |
| 26 | `sessionPersistence` | `packages/session/session-persistence` | B 需宿主代理 | 需持久化后端，须映射到 Amitia 会话存储 |
| 27 | `sessionProjectionCache` | `packages/session/session-projection-cache` | B 需宿主代理 | 官方声明打开 storage 域，依赖持久化后端 |
| 28 | `sessionReferenceResolver` | `packages/context/session-reference` | A 官方包实现 | 官方实现，读取会话日志精确内容 |
| 29 | `sessionSkillCatalog` | `packages/api/session-controller` | C 需 Core 能力 | 技能目录来自 Amitia 技能体系，需 Core 通用能力 |
| 30 | `sessionTelemetry` | `packages/session/session-telemetry` | B 需宿主代理 | 需遥测上报通道，Amitia 无对应能力时明确失败 |
| 31 | `sessionTitle` | `packages/session/session-title` | A 官方包实现 | 官方实现，标题以会话日志折叠为准 |
| 32 | `settings` | `packages/settings/settings` | A 官方包实现 | 官方 Config 投影，纯内存与 Config 文档 |
| 33 | `settingsController` | `packages/api/settings-controller` | A 官方包实现 | 官方 Host API 控制器，底层 settings 由官方提供 |
| 34 | `shell` | `packages/shell/shell` | B 需宿主代理 | 需真实进程执行，受容器 Node 权限约束 |
| 35 | `shellEnv` | `packages/shell/shell-env` | A 官方包实现 | 官方注册表，已确认为独立服务包 |
| 36 | `skills` | `packages/skill/skill` | A 官方包实现 | 官方分层注册表；provider 取 Amitia 技能目录时为 C |
| 37 | `spillStore` | `packages/spill/spill` | B 需宿主代理 | 需落盘，须映射到插件数据目录写入 |
| 38 | `storage` | `packages/storage/storage` | B 需宿主代理 | 需注册持久化后端，Amitia 无对应能力 |
| 39 | `storageDomain` | `packages/storage/storage-domain` | B 需宿主代理 | 依赖 storage 后端路由 |
| 40 | `subagentModelSelection` | `packages/subagent/tool-subagent` | A 官方包实现 | 官方 settings 贡献项，纯内存记录 |
| 41 | `subprocess` | `packages/subprocess/subprocess` | B 需宿主代理 | 需真实子进程，受容器 Node 权限约束 |
| 42 | `terminalController` | `packages/api/terminal-controller` | B 需宿主代理 | 依赖 terminals 与真实 PTY |
| 43 | `terminals` | `packages/terminal/terminal` | B 需宿主代理 | 官方注册表需 PTY 后端，后端需子进程 |
| 44 | `tokenMeter` | `packages/llm/token-meter` | A 官方包实现 | 官方实现，用于本地估算与 usage 折叠 |
| 45 | `toolResultPruner` | `packages/compaction/compaction-tool-result-pruner` | A 官方包实现 | 官方确定性裁剪，纯计算 |
| 46 | `typert` | `packages/typert/registry` | A 官方包实现 | 官方 schema 注册表，纯内存 |
| 47 | `typertGateway` | `packages/api/gateway` | A 官方包实现 | 官方网关，活跃客户端通道就绪后可用 |
| 48 | `web` | `packages/web/web` | B 需宿主代理 | 需搜索与抓取 provider，依赖网络与凭据 |
| 49 | `webApp` | `packages/bundle/web-app` | A 官方包实现 | 官方 Web 包，桌面端不消费 |
| 50 | `webServer` | `packages/host/webserver` | B 需宿主代理 | 需 HTTP 服务端，端口与路由须由宿主提供 |
| 51 | `workflowEngine` | `packages/workflow/workflow` | B 需宿主代理 | 官方 seam，具体实现 workflow-ptc 需隔离执行 |
| 52 | `workspaceController` | `packages/api/workspace-controller` | C 需 Core 能力 | 项目对象来自 Amitia 项目语义 |
| 53 | `workspaceFiles` | `packages/api/workspace-files` | B 需宿主代理 | 枚举与浏览可走 `host.workspace.read` 的 `file_list`/`browse`/`file_search` |
| 54 | `workspaceRegistry` | `packages/workspace/workspace` | C 需 Core 能力 | 项目注册表来自 Amitia 项目语义 |

#### 23.4.2 分类结论

1. A 类是当前投入产出最高的一批：54 项中有 27 项只需共享服务层加官方包，不需要 Core 改动，也不需要放宽容器权限。
2. B 类能否闭合取决于容器是否获得代理能力：文件与持久化可以映射到既有通用能力，进程、终端与 HTTP 服务端在容器 Node 权限不变的前提下只能明确失败。
3. C 类共 3 项，收敛到 2 个 Core 通用能力：项目注册与会话排序（`workspaceRegistry`、`workspaceController`）、技能目录读取（`sessionSkillCatalog`）。工作区文件枚举经核查可走 `host.workspace.read`，已从 C 改判 B。
4. 逐项判定以官方文档的 Source 包路径为证据；实施每一项前仍需按该包的实际 exports 与 construction 依赖复核一次，复核结果回写到本表。

### 23.5 C 类需要 Core 新增的通用能力草案

按 23.4 复核后 C 类只剩 3 项，收敛到 2 个 Core 通用能力：

| 通用能力 | 对应缺口 | 关键契约 |
| --- | --- | --- |
| `host.project.list/create/rename/delete` + `host.conversation.order/archive/pin` | `workspaceRegistry`、`workspaceController` | 复用 Amitia 项目对象与 Owner Scope；删除走既有软删除语义；会话排序/归档/置顶与会话列表同源，不新增 DSH 专用字段 |
| `host.skill.list/read` | `sessionSkillCatalog` | 读取 Amitia 现有技能目录与技能正文，权限沿用技能现有作用域 |

`skills`、`agentPresets`、`sessionTitle`、`sessionController.rename` 原计划列入 C 类，复核后确认由官方包自行实现（标题以会话日志折叠为准、preset 注册表与技能分层注册表均为官方包），不再需要 Core 新增能力；`shell`/`subprocess`/`terminals`/`sandbox` 归入 B 类失败关闭，不新增 `host.exec.*`。

上述每项都需要单独方案与确认，不在本批次内直接实施。

> 23.9.4 补充：实测 `@deepseek-ai/dsh-workspace` 仅 inject `storageDomain` 与 `sessionPersistence`，两者都是 seam；只要提供对应 provider，`workspaceRegistry` 与 `workspaceController` 可不必新增 Core API。`sessionSkillCatalog` 由官方 `api-session-controller` 从 `ctx.skills` 派生，而 `skills` 的官方 provider 主要走已提供的 `ctx.fs`。因此本节是否成立取决于 provider 落盘口径的选择。

### 23.6 验收标准

1. `@deepseek-ai/dsh-lsp` 与 `@deepseek-ai/dsh-shell-env` 作为官方服务包真实安装并加载后，消费包可 `inject` 到 `ctx.lsp`、`ctx.shellEnv` 并完成一次真实调用。
2. 缺失服务、重复提供、未声明遮蔽、服务包停用被引用四种情况各自返回原始错误与稳定错误码，测试覆盖。
3. 包停用后服务与其贡献全部回收，父层服务仍可用。
4. Catalog 重新生成后，A 类缺口从 Host Service 缺口计数中移除，B、C 类带具体原因，且总数与 54 项一致。
5. 插件全量测试、lint、真实安装、`/api/health` 与 `/readyz` 全部通过。
6. `ctx.effect` 支持 generator：登记立即生效、释放逆序执行、generator 内异常原样抛出；真实 `@deepseek-ai/dsh-lsp` 注册 provider 后 `query` 命中路由（当前实测为 `LSP_UNAVAILABLE`）。
7. 兼容映射导出面守卫通过：被映射的 specifier 不得缺少官方包导出名；真实 `@deepseek-ai/dsh-shell-env` 在容器内 `ctx.shellEnv.register/collect/list` 可用（当前实测为 `DSH_ENV_PREFIX` 缺失导致 import 失败）。

### 23.7 B 类代理映射设计（24 项）

逐项给出代理目标或失败关闭理由：12 项可代理，12 项在容器 Node 权限不变的前提下明确失败。

> 本节依据 23.9 的官方能力接缝权威表修订：`lsp`、`shell`、`subprocess`、`terminals`、`sandbox`、`web`、`storage`、`spillStore`、`fileReferences`、`attachments`、`directoryPicker`、`sessionPersistence`、`sessionTelemetry`、`workflowEngine`、`ptcRuntime`、`compaction`、`skills`、`authorization`、`deepseekLlmApiExtensions`、`sessionTitle` 均为官方 seam，实施形态改为“安装官方 seam + 按需选择 provider”，不再一律失败关闭。

| 服务 | 代理目标或失败理由 |
| --- | --- |
| `fileReferences` | `host.workspace.read` 的 `file_list` / `file_search` |
| `workspaceFiles` | `host.workspace.read` 的 `file_list` / `browse` |
| `sessionFileReferences` | 同 `fileReferences`，按会话工作目录取候选 |
| `spillStore` | `host.resource.open/write`（`scope=data`）+ `host.resource.stat`，返回 locator 与字节数 |
| `sessionPersistence` | 会话事件序列走 `host.state.*`（按会话分键，追加用 CAS，冲突重试后原样报错），不新增 Core 表 |
| `sessionProjectionCache` | 同 `sessionPersistence` 的 `host.state` 键空间，冷快照与 checkpoint 分别落键 |
| `storage` / `storageDomain` | 域设施基于 `host.state`（KV）+ `host.resource`（blob）实现；官方后端的专有语义不代理 |
| `web` | `host.network.request` 取搜索/抓取结果，凭据走 `host.secret.get`；provider 仍由 DSH 侧注册 |
| `deepseekAccount` | `host.network.request` + `host.secret.get`；官方账号 API 语义不在容器内重建 |
| `directoryPicker` / `directoryPickerController` | `host.ui.dialog`；若对话框能力不支持目录选择则失败关闭 |
| `shell` / `subprocess` | 明确失败：Core 无进程执行通用能力，容器不放宽 `node --permission` |
| `terminals` / `terminalController` | 明确失败：同上，PTY 后端需真实进程 |
| `jobController` | 明确失败：无作业运行时通用能力 |
| `ptcRuntime` / `workflowEngine` | 明确失败：需隔离执行程序 |
| `sandbox` | 明确失败：无 confine 通用能力，禁止无沙箱直通 |
| `permissionPresets` | 明确失败：官方要求 `ctx.shell` 执行器，shell 已失败关闭 |
| `lsp` | 明确失败：需拉起语言服务器子进程 |
| `sessionTelemetry` | 明确失败：无遥测上报通道，禁止静默丢弃 |
| `webServer` | 明确失败：无 HTTP 服务端通用能力 |

失败关闭的统一形态：抛 `DSH_HOST_OPERATION_UNAVAILABLE`，错误里带服务名、方法名、缺失的通用能力名，不返回空实现或假成功。

### 23.8 方案修订（本轮实测决定）：以真实 `@deepseek-ai/cordis` 作为容器插件运行时

23.3 的原设计是在容器里自建服务注册表、自写 generator effect、加 Proxy 视图，本质是用几百行重实现 cordis 语义。本轮用真实官方包实测后确认：容器可以直接运行真实 `@deepseek-ai/cordis`，官方包原生的服务注册、inject 排布、generator effect、冲突检测与释放语义全部可用。因此本批次实施方案改为“真实 cordis + Amitia 服务层”，23.3.1、23.3.5、23.3.6 中的自建注册表、自写 effect、Proxy 视图全部取消。

#### 23.8.1 修订原因

1. 官方包就是 cordis 插件：默认导出的类继承 `Service` 或 `TypertRemoteService`，构造签名是 `constructor(ctx)`，服务用 `super(ctx, "<name>")` 注册，登记内容用 `this.ctx.effect(function* () { … yield () => {…} })` 释放。
2. 自建注册表即使跑通原型，也只是 cordis 的一个子集，后续每遇到一个上游语义就要再补一次，长期必然偏移。
3. 真实 cordis 已经是官方发布物（`@deepseek-ai/cordis@4.1.0`），与目标 ABI 同源，不需要我们实现。

#### 23.8.2 实测证据（全部使用真实官方包）

| 探针 | 结果 |
| --- | --- |
| `ctx.plugin(TypertRegistry)` 后读 `ctx.typert` | `constructor.name === "TypertRegistry"`，`list()` 返回 `[]` |
| 同一上下文重复 `ctx.plugin(TypertRegistry)` | 原始错误 `service "typert" has been registered at <TypertRegistry>` |
| 先 `ctx.plugin(cordis-plugin-loader)` 再 `ctx.plugin(dsh-client-modules)` | `ctx.loader = Loader`，`ctx.clientModules = ClientModuleRegistry`，`graph` 可调用 |
| `inject: ["notARealService"]` 的插件 | `apply` 未执行，无假服务 |
| `ctx.plugin(dsh-lsp)` 后 `registerProvider({ id, extensionToLanguage, query })` | `query({ filePath: "/tmp/probe.ts" })` 返回 `{ languageId: "typescript", filePath: "/tmp/probe.ts" }` |
| 重复注册同一扩展名 | 原始错误 `LspError: extension ".ts" is already handled by another LSP provider [LSP_CONFLICT]` |
| 调 `registerProvider` 返回的 disposer 后再 `query` | 原始错误 `LspError: no LSP provider handles "/tmp/probe.ts" [LSP_UNAVAILABLE]` |
| Amitia 侧 `class AmitiaWorkspace extends Service { super(ctx, "workspace") }` | `ctx.workspace` 可见且方法可调用；`ctx.provide`、`ctx.set` 均为 function |
| `dsh-lsp` 内部 `ctx.effect(function* () { … })` | 生成器体同步执行完毕，disposer 延迟到释放时执行，与官方语义一致 |

对照 23.3.5 原型期实测：当时 `registerProvider` 返回 disposer 但 `providerIds=[]`、`routes=[]`，`query` 报 `LSP_UNAVAILABLE`；现在同样代码在真实 cordis 下直接命中路由。

#### 23.8.3 设计（修订版）

```text
DSH 包运行层：真实 @deepseek-ai/cordis Context
├─ 官方包：ctx.plugin(官方包)          // inject 由 cordis 自动等待与排布
├─ Amitia 服务层：每个通用能力一个 Service 子类
│    class AmitiaWorkspace extends Service { constructor(ctx) { super(ctx, "workspace"); } }
│    方法内部仍走 host.call，原始错误原样抛出
├─ 事件与工具：ctx.on/emit/waterfall/bail、ctx.tools 由 Amitia 侧注入
└─ import 钩子：只保留 Amitia 必须自定义的 specifier 映射
```

与现有实现的差别只在“谁来托管插件”：仍是同一个 plugin-host 子进程、同一套 `node --permission` 边界、同一套 `host.call` 桥接，Core 与 ABI 不变。

#### 23.8.4 改动清单（取代 23.3.1 与 23.3.6）

1. 运行时依赖：`@deepseek-ai/cordis@~4.1.0`、`@deepseek-ai/cordis-plugin-loader@~1.1.0` 与官方包同源安装，走容器包管理器真实安装，不预打包进插件产物。
2. `src/runtime/compat/cordis.mjs`：删除自写 `Service`，改为转发真实 cordis，或在导出面守卫通过后直接移除该映射。
3. `src/runtime/host-abi.mjs`：`createDshHostContext` 改为创建真实 cordis Context；`ctx.on/emit/waterfall/bail`、`ctx.tools`、`ctx.call` 由 Amitia 侧注入；`assertServices` 删除（inject 由 cordis 原生处理）。
4. `src/runtime/host-services.mjs`：`createAmitiaDshHostServices` 返回的服务集合逐个注册为 cordis 服务，服务名保持不变（`systemPrompt` … `mcp`，共 25 个）。
5. `src/runtime/index.mjs`：`activatePackage` 改用 `ctx.plugin(...)`，停用走 fiber dispose，工具绑定与回收逻辑保持现状。
6. 测试：原生语义四态（重复提供、inject 不满足不激活、释放后回收、父层遮蔽）、`dsh-lsp` 与 `dsh-shell-env` 双试点、兼容映射导出面守卫、停用回收。

#### 23.8.5 与旧方案的关系

23.3.1 的 `service-registry.mjs`、23.3.5 的自写 generator effect 与 Proxy 视图不再实施；23.3.5 的原型只作为“跨包服务不可见”的证据保留，原型目录在实施完成后连同 `.tmp-spec3` 一并删除。

#### 23.8.6 本轮新发现的依赖闭包

实测 26 个 A 类官方包的 `inject` 后确认，A 类不能单包安装：

1. 已由容器提供、可满足：`agents`、`credentials`、`fs`、`llm`、`sessionProjections`、`sessionQuery`、`sessions`、`subagents`、`systemPrompt`。
2. 属于 54 项缺口、需先补齐：`agentDefaultModel`、`deepseekLlmApiExtensions`、`remoteAccess`、`tokenMeter`、`typert`、`typertGateway`、`workspaceRegistry`。
3. 未在文档化服务清单内、需确认提供者的 6 个：`loader`（已实测由 `cordis-plugin-loader` 提供）、`tools`、`attachments`、`configEditor`、`profileContext`、`fileUploads`。
4. 典型闭环示例：`ctx.sessionController` 需要 13 个服务（`agentDefaultModel`、`agents`、`attachments`、`fileUploads`、`fs`、`llm`、`remoteAccess`、`sessions`、`sessionProjections`、`sessionQuery`、`typert`、`typertGateway`、`workspaceRegistry`），必须按闭包安装。

同时确认 `dsh-web-app`（`packages/bundle/web-app`）有 145 个依赖且属 Web 端专用，桌面端不安装、不加载，`webApp` 服务在桌面端保持 unavailable 并标注原因。

#### 23.8.7 验收标准（修订）

1. 真实 cordis Context 内 `ctx.typert`、`ctx.shellEnv`、`ctx.lsp`、`ctx.clientModules` 均由官方包提供且方法可调用。
2. Amitia 服务在 `ctx.workspace` 等位置可见，内部仍走 `host.call`，原始错误原样透传。
3. `inject` 不满足的官方包不激活，且不出现假服务或空实现。
4. 官方包停用后其服务与贡献全部回收，抛出的仍是官方原始错误与错误码。
5. 插件全量测试、lint、真实安装、`/api/health` 与 `/readyz` 全部通过。

#### 23.8.8 落地实测结论（0.1.81）

以下每一条都是在真实 cordis 4.1.0 与本机 Core 上实测得到，不是按文档推断。

1. 依赖落位：plugin-host 子进程不提供 `node_modules`，官方 `@deepseek-ai/cordis` 与 `@deepseek-ai/cosmokit` 以 vendor 形式随插件产物发布在 `dist/vendor/`；cordis 对 cosmokit 的裸导入已改为相对导入，否则 vendor 内解析失败。自写 `Service` 与转发层已删除（`src/runtime/compat/cordis.mjs` 无任何引用，按单版本策略移除），`@deepseek-ai/cordis` 直接映射到 vendor 包。
2. 托管形态：`createDshHostContext` 创建真实 `new Context()`；每个包入口在 `root.isolate("tools")` 内 `extend({ pluginId, scope: { extensionId } })` 后调用 `ctx.plugin(entry, config)`，Amitia 的 `tools` 门面按入口注入，入口之间互不共享工具视图。
3. 服务名保留字：cordis 自身占用 `events` 与 `runtime`，Amitia 侧同名服务注册为 `amitiaEvents` 与 `amitiaRuntime`，其余 25 个服务名保持不变。
4. inject 语义完全交给 cordis：依赖不满足时 fiber 停在 state 0 且 `apply` 不执行，cordis 本身不抛错；容器在 `await fiber.await()` 之后用 `assertActive` 读取 `fiber.state` 判定，抛 `DSH_HOST_SERVICE_MISSING`，并在错误里列出未满足的服务名。同一包内存在“提供者入口 + 消费者入口”时用 `deferInjectCheck` 延后到全部入口加载完成后由 `assertDshHostPluginActive` 统一断言，避免顺序脆弱。
5. 服务提供者释放后的实测行为：依赖方 fiber 回到 state 0，该服务立即不可见，不存在“保留旧值”的过渡态；此后新加载同一 inject 的包直接得到 `DSH_HOST_SERVICE_MISSING`。不为这一状态新增“服务被回收”错误码，按 cordis 原生 state 0 语义如实呈现。
6. 错误面：cordis 原始报错对象、message 与 stack 原样抛出，仅通过 `defineProperty` 附加稳定 code（`DSH_HOST_SERVICE_MISSING`、`DSH_HOST_SERVICE_CONFLICT`、`DSH_HOST_SERVICE_UNINJECTED`），不替换文案、不包装 cause。实测原文为 `service "workspace" has been registered at <root>` 与 `cannot get property "X" without inject`。
7. compat 映射改为“真实包优先”：先走正常解析，只在 `ERR_MODULE_NOT_FOUND` 时回退容器 shim，避免 shim 短路真实官方包。导出面守卫改为扫描已安装官方包的真实具名导入反向校验映射，本轮因此发现 `@deepseek-ai/dsh-subprocess` 缺 `DSH_ENV_PREFIX`（官方 `dsh-shell` 依赖它），并发现 `scrubbedParentEnv` 语义错误（原实现直接复制父环境），已按官方常量与 `DSH_` 前缀加敏感名过滤重写。
8. 双试点实测：`ctx.lsp` 走官方 `Lsp`，重复注册抛 `LSP_CONFLICT`、无 provider 时抛 `LSP_UNAVAILABLE`、释放后服务不可见；`ctx.shellEnv` 走官方模块，`register/list/collect/release/dispose` 全通，`DSH_SHELL` 为 `"1"`、`DSH_HOME` 取自传入的 `dshHome`。
9. ESM 图禁止顶层 await：plugin-host 以 `require()` 加载扩展入口（`loadESMFromCJS`），图内任何顶层 await 都会让整图成为 async 模块并抛 `ERR_REQUIRE_ASYNC_MODULE`。vendor cordis 因此改为静态导入，后续运行时模块同样不得引入顶层 await。
10. 本机 Core 实测（0.1.81）：安装成功，`/api/health` 200、`/readyz` 200（7 ready、0 degraded、0 failed），容器 `status` 返回 4 个已启用 DSH 包（含官方 `@deepseek-ai/dsh-tool-schedule` 映射 4 个贡献），工具经 `POST /api/extension/action/dsh-compatible-page/command` 正常执行，插件测试 152 项全通过。

### 23.9 官方能力接缝权威表：按 kind 与 provider 重新定位 B/C 类

官方文档 `docs/capability-seams.md` 给出全部 109 个 `ctx.*` 服务的权威定义：`kind`（`core` / `seam` / `service` / `bundle`）、Definition 包、Provider 包、消费者。这和 23.4 按“文档 Source 行”做的推断相比是更强的一手证据：**seam 类服务由官方提供接口定义包，真正的实现是独立 provider 包**，容器完全可以只装 seam，再由 Amitia 侧提供一个 provider。

缺口相关（54 项 + 6 个此前未文档化服务）共 59 项的权威分类是：`core` 37 项、`seam` 20 项、`service` 1 项、`bundle` 1 项。

#### 23.9.1 逐项权威表

| 服务 | kind | Definition 包 | Provider 包 |
| --- | --- | --- | --- |
| `ctx.agentDefaultModel` | core | `agent-default-model` | - |
| `ctx.agentPresets` | core | `agent-preset-registry` | - |
| `ctx.clientModules` | core | `client-modules` | - |
| `ctx.configEditor` | core | `config-editor` | - |
| `ctx.connection` | core | `client-connection` | - |
| `ctx.credentialsController` | core | `api-settings-controller` | - |
| `ctx.directoryPickerController` | core | `api-workspace-controller` | - |
| `ctx.fileUploads` | core | `client-file-upload` | - |
| `ctx.goals` | core | `goal` | - |
| `ctx.jobController` | core | `api-job-controller` | - |
| `ctx.permissionPresets` | core | `permission-presets` | - |
| `ctx.planMode` | core | `plan-mode` | - |
| `ctx.profileContext` | core | `app-boot` | - |
| `ctx.remoteAccess` | core | `api-remote-access` | - |
| `ctx.sandboxPolicy` | core | `sandbox-policy` | - |
| `ctx.sessionController` | core | `api-session-controller` | - |
| `ctx.sessionFileReferences` | core | `api-session-controller` | - |
| `ctx.sessionLogDeepSeek` | core | `session-log-deepseek` | - |
| `ctx.sessionProjectionCache` | core | `session-projection-cache` | - |
| `ctx.sessionReferenceResolver` | core | `session-reference` | - |
| `ctx.sessionSkillCatalog` | core | `api-session-controller` | - |
| `ctx.settings` | core | `settings` | - |
| `ctx.settingsController` | core | `api-settings-controller` | - |
| `ctx.shellEnv` | core | `shell-env` | - |
| `ctx.storageDomain` | core | `storage-domain` | - |
| `ctx.subagentModelSelection` | core | `tool-subagent` | - |
| `ctx.terminalController` | core | `api-terminal-controller` | - |
| `ctx.tokenMeter` | core | `token-meter` | - |
| `ctx.toolResultPruner` | core | `compaction-tool-result-pruner` | - |
| `ctx.tools` | core | `tools` | - |
| `ctx.typert` | core | `typert-registry` | - |
| `ctx.typertGateway` | core | `api-gateway` | - |
| `ctx.webApp` | core | `web-app` | - |
| `ctx.webServer` | core | `host-webserver` | - |
| `ctx.workspaceController` | core | `api-workspace-controller` | - |
| `ctx.workspaceFiles` | core | `api-workspace-files` | - |
| `ctx.workspaceRegistry` | core | `workspace` | - |
| `ctx.attachments` | seam | `attachment` | `attachment-local` |
| `ctx.authorization` | seam | `authorization` | - |
| `ctx.compaction` | seam | `compaction` | `compaction-basic` |
| `ctx.deepseekLlmApiExtensions` | seam | `deepseek-llm-api-extensions` | `session-log-deepseek`、`plugin-package-inventory-deepseek` |
| `ctx.directoryPicker` | seam | `host-directory-picker` | `host-directory-picker-native`、`host-directory-picker-browse` |
| `ctx.fileReferences` | seam | `file-reference` | `file-reference-local` |
| `ctx.lsp` | seam | `lsp` | `lsp-stdio` |
| `ctx.ptcRuntime` | seam | `ptc-runtime` | `ptc-runtime-node`、`experimental-ptc-runtime-python` |
| `ctx.sandbox` | seam | `sandbox` | `sandbox-local`、`sandbox-ssh` |
| `ctx.sessionPersistence` | seam | `session-persistence` | `session-persistence-jsonl` |
| `ctx.sessionTelemetry` | seam | `session-telemetry` | `session-telemetry-otel` |
| `ctx.sessionTitle` | seam | `session-title` | `session-title-first-prompt-llm`、`session-title-all-prompts-llm` |
| `ctx.shell` | seam | `shell` | `bash-local`、`bash-sandbox`、`pwsh-local` |
| `ctx.skills` | seam | `skill` | `sandbox-windows-acl`、`skill-badge`、`skill-filesystem`、`skill-office` |
| `ctx.spillStore` | seam | `spill` | `spill-local` |
| `ctx.storage` | seam | `storage` | `storage-json`、`storage-sqlite` |
| `ctx.subprocess` | seam | `subprocess` | `subprocess-local`、`subprocess-ssh` |
| `ctx.terminals` | seam | `terminal` | `terminal-bash`、`terminal-panel` |
| `ctx.web` | seam | `web` | `web-search-deepseek`、`web-deepseek-api`、`web-fetch-http` |
| `ctx.workflowEngine` | seam | `workflow` | `workflow-ptc` |
| `ctx.deepseekAccount` | service | `deepseek-account` | - |
| `ctx.agentLoop` | bundle | `agent-loop` | - |

#### 23.9.2 结论一：seam 类不再“失败关闭”，改为“官方 seam + 选定 provider”

23.7 把 `lsp`、`shell`、`subprocess`、`terminals`、`sandbox`、`web`、`storage`、`spillStore`、`fileReferences`、`attachments`、`directoryPicker`、`sessionPersistence`、`sessionTelemetry`、`workflowEngine`、`ptcRuntime`、`compaction`、`skills`、`authorization`、`deepseekLlmApiExtensions`、`sessionTitle` 归入“明确失败”，依据是“需要真实进程/文件/网络”。权威表显示它们都是 seam：**接口与语义在官方包里，容器只需要装 seam；需要真实能力的是 provider**。因此正确形态是：

1. seam 包一律安装并加载，服务可见，语义完全由官方实现（例如 `ctx.lsp.query` 在没有 provider 时抛官方 `LSP_UNAVAILABLE`，这就是标准行为，不需要我们编造错误）。
2. provider 逐项决策：能映射到 Amitia 通用能力就提供 Amitia provider，不能就保持“无 provider”的官方失败态。
3. 只有 `core` 类服务才可能真的需要 Core 新能力。

#### 23.9.3 结论二：provider 的能力依赖决定实现路线（实测）

| provider 包 | 依赖形态 | 路线 |
| --- | --- | --- |
| `dsh-subprocess-local` | `node:fs` / `node:os` / `node:timers` 直接起进程 | 需要真实子进程，容器权限不变时不可用 |
| `dsh-lsp-stdio` | `ctx.subprocess` + `ctx.fs` | 只要 `subprocess` provider 存在即可用 |
| `dsh-skill-filesystem` | `ctx.fs` 为主，少量 `node:fs` | 走 Amitia `fs` 服务即可，**不需要新增 Core 技能 API** |
| `dsh-storage-json` / `dsh-session-persistence-jsonl` / `dsh-spill-local` / `dsh-file-reference-local` | 直接 `node:fs` 落盘 | 需要确定落盘目录与权限口径，见 23.9.4 |

#### 23.9.4 结论三：C 类可以不由 Core 新增 API 闭合，但需要先定落盘口径

实测 `@deepseek-ai/dsh-workspace`（即 `ctx.workspaceRegistry`）的依赖是 `static inject = ["storageDomain", "sessionPersistence"]`。两者都是 seam，官方提供 `storage-json`、`session-persistence-jsonl` 作为 provider。也就是说：

1. `workspaceRegistry` 不必等 Core 新增“项目 API”，提供 `storageDomain` 与 `sessionPersistence` 的 provider 即可运行。
2. `sessionSkillCatalog` 由官方 `api-session-controller` 从 `ctx.skills` 派生；`skills` 的官方 provider `skill-filesystem` 主要走 `ctx.fs`，而 `fs` 服务容器已经提供，因此也不必然需要 Core 新增技能 API。
3. 剩下的真实决策点是：官方 provider 用 `node:fs` 直接落盘，落在哪里、是否纳入 Amitia 备份与清理、是否受 `node --permission` 放行。可选两条：
   - 方案 A：允许官方 provider 落盘到插件数据目录，由 Amitia 备份与清理统一覆盖；需要把该目录纳入 `--permission` 允许写入范围，并在容器启动时登记。
   - 方案 B：自写 provider，全部经 `host.state`（KV）与 `host.resource`（blob）转发，不产生容器外文件；实现量更大，但数据全部归 Amitia 管理，不依赖 Node 权限放行。
4. 这一条直接影响 3 个 C 类项与 4 个 B 类项，需要你选定方向后再实施，不在本批次直接动手。

### 23.10 批次 C 落地记录（0.1.83）：54 项缺口逐项路由 + 官方包证据

#### 23.10.1 目标与结果

批次 C 的原始假设是“零引用服务保持 `unavailable`”。落地前先做了引用测量，结论是**该假设不成立**：官方 0.2.2-rc.2 依赖闭包内没有任何 catalog 服务是零引用的，因此不存在“因无人使用而直接判死”的服务。批次 C 的实际交付因此改为：**给 54 项缺口逐项写出“由谁提供 / 缺什么能力 / 为什么现在不可用”，并把这些判断变成可复算的产物**。

#### 23.10.2 测量工具：`scripts/scan-official-references.mjs`

不依赖任何临时目录，只依赖一个 `@deepseek-ai` 包树（本轮用 `npm install @deepseek-ai/dsh@0.2.2-rc.2 --ignore-scripts` 得到的 672 个包的闭包），输出 `catalog/official-references.v1.json`。为避免“漏认”导致误判零引用，扫描刻意为**过度近似**：

1. `literal`：任意文件里出现带引号的 catalog 服务名字面量（覆盖 `inject` 数组、`ctx.get("x")`、`super(ctx, "x")`）。
2. `property`：出现 `ctx.<name>` / `this.ctx.<name>` / `context.<name>` 属性访问。
3. `definition`：出现 `super(<owned context>, "<name>")` / `set("<name>")` / `provide("<name>")` 的定义点。

实测结果：扫描 416 个 `@deepseek-ai` 包，69 项 catalog 服务全部被引用，`zeroReferenceServices` 为空。定义点也都定位到了具体官方包（例：`typert` → `dsh-typert-registry`，`shell` → `dsh-shell`，`workspaceRegistry` → `dsh-workspace`，`sessionSkillCatalog` → `dsh-api-session-controller`）。唯一没有定义点的是 `lsp`：官方文档与 API catalog 声明了 `ctx.lsp` seam，但 0.2.2-rc.2 的依赖闭包里没有随附该 seam 包，这一点已按实测写入路由说明，不再沿用先前“官方包已可加载”的说法。

#### 23.10.3 路由表：`scripts/host-service-routes.mjs`

54 项逐项给出 `route`（`official-package` / `amitia-service`）、`provider`（`none` / `amitia` / `unavailable` / `container`）与一句可核对的说明。`provider` 经 `HOST_SERVICE_PROVIDER_STATUS` 映射为 catalog 状态：

| provider | catalog status | 含义 | 本次数量 |
| --- | --- | --- | --- |
| `none` | `delegated` | 官方包自带该服务，插件声明 inject 并自带依赖后由真实 cordis 原生托管，容器不复制实现 | 26 |
| `amitia` | `pending` | seam 由官方包提供，但 provider 需要一项 Amitia 已有或可映射的通用能力，待实现 | 15 |
| `unavailable` | `unavailable` | provider 需要真实进程 / PTY / 隔离执行 / 外部网络或账号，容器 Node 权限不变时维持官方失败态 | 13 |

- `delegated` 26 项即 §23.4.1 的 A 类，按 §23.6 第 4 条从“Host Service 实现缺口”中移除，但仍逐条保留在产物里可追溯。
- `pending` 15 项即 B/C 类中可经 provider 闭合的部分，包含 `storage`、`storageDomain`、`spillStore`、`sessionPersistence`、`sessionProjectionCache`、`fileReferences`、`sessionFileReferences`、`workspaceFiles`、`directoryPicker`、`directoryPickerController`、`jobController`、`skills`、`sessionSkillCatalog`、`workspaceRegistry`、`workspaceController`。其中 `workspaceRegistry`、`workspaceController`、`sessionSkillCatalog` 与 §23.9.4 的实测一致：它们只依赖 `storageDomain` + `sessionPersistence` 两个 seam，不需要 Core 新增项目 API。
- `unavailable` 13 项为 `shell`、`subprocess`、`terminals`、`terminalController`、`sandbox`、`ptcRuntime`、`workflowEngine`、`permissionPresets`、`web`、`webServer`、`deepseekAccount`、`sessionTelemetry`、`lsp`，原因逐条写入 catalog，不做“统一文案”兜底。

#### 23.10.4 Catalog 产物变化

1. `catalog/official-references.v1.json`：新增证据文件，记录 416 个包、69 项服务的字面引用、属性引用、定义包与消费者数量。
2. `catalog/amitia-gap.v1.json`：`host` 由“只列 pending”改为列出全部 54 项，每项新增 `status`、`route`、`provider`、`officialPackages`、`consumerCount`，`reason` 换成路由表里的具体原因。
3. `catalog/dsh-abi.v1.json` 与 `catalog/compatibility.v1.json`：`host.services` 增加 `route`、`provider`、`reason`（abi 另含 `officialPackages`、`consumerCount`）。
4. `src/runtime/catalog-summary.generated.mjs`：新增 `hostServiceDelegatedCount`、`hostServiceProviderPendingCount`、`hostServiceUnavailableCount`、`officialReferencePackageCount`，容器 `status` 动作直接可见这几项计数。
5. `scripts/generate-catalogs.mjs` 读取证据文件时对未知 `provider` 直接抛错，避免静默降级成通用文案。

#### 23.10.5 验证

1. `node --test tests/*.test.mjs`：153 项全部通过（新增 1 项 54 项路由一致性测试，会校验每项的 `route`/`provider`/`reason`/`consumerCount`，并与 `dsh-abi` 的 status 逐项比对）。
2. 重新生成的 catalog 计数：`hostServiceCount` 69、`hostEventCount` 74、`hostToolCount` 77、`hostConfigurationCount` 163、`lifecycleEventCount` 63、`clientServiceCount` 8、`clientEventCount` 4、`uiSlotCount` 120，与生成前一致；54 = 26 + 15 + 13。
3. 打包 `amitia-dsh-compatible-host-0.1.83.amitiax`（14693266 字节，SHA-256 96da0cec…，treeHash e1def340…）并安装到本机 Core（previousVersion 0.1.82）：`/api/health` 200（`ready: true`、`model: configured`），`/readyz` 200（`state: ready`，7 ready、0 degraded、0 blocking、0 failed），容器 `status` 动作返回 4 个官方插件启用、catalog 计数含上述四项新字段。

#### 23.10.6 剩余工作

1. `pending` 15 项需要按 §23.9.4 先定落盘口径（方案 A：允许官方 provider 写插件数据目录并纳入备份清理；方案 B：自写 provider 全走 `host.state` / `host.resource`），选定后作为批次 D 实施。
2. `unavailable` 13 项在容器 Node 权限与宿主能力不变的前提下保持失败关闭，不引入兼容层或降级实现。
3. `lsp` 需在官方补齐 seam 包发布后重新测量，届时从 `unavailable` 重新评估。

### 23.11 P13 方案：`workspaces` Client Service 的通用能力缺口（待确认后实施）

#### 23.11.1 DSH 侧要求

官方 Client ABI 的 `ctx.workspaces`（catalog 记为 `DSH-CLIENT-SERVICE-008`，当前 `unavailable`）共 6 个方法：

| 方法 | 语义 |
| --- | --- |
| `create({ path })` | 以路径建项目，返回 `WorkspaceView` |
| `rename(workspaceId, title)` | 改项目标题 |
| `delete(workspaceId)` | 删项目 |
| `archiveSession(sessionId, { stopActivity? })` | 归档会话，可选先停活动 |
| `unarchiveSession(sessionId)` | 取消归档 |
| `insertSessionBefore(workspaceId, sessionId, beforeSessionId?)` | 把会话插到另一会话之前（手动排序） |

#### 23.11.2 Amitia 现状（已核对源码与基线 schema）

1. 数据模型已具备：`projects(id, space_id, name, workspace_id, device_id, root_uri, pinned_at, revision)`，`conversations` 具备 `project_id`、`archived_at`、`pinned_at`、`workspace_id`。索引 `idx_conversations_project_updated`、`idx_conversations_sidebar_archived` 已存在。
2. 写入通道已具备：设备归属业务资源 `edit.go` 同时支持 `conversation` 与 `project` 两种 kind（标题、归档、置顶、删除），`mesh_project_router.go` 提供 `GET /projects` 与 `POST /projects`（按设备与执行域校验）。
3. 缺口在暴露层：宿主 API（`backend/internal/extension/kernel/host_api`）只有 `host.conversation.list/read/search/create/delete/message.*/turn.*/logs.*`，**没有** `host.project.*`，也没有会话归档与排序方法；权限表里同样没有对应条目。
4. `insertSessionBefore` 无对应字段：会话排序当前是 `pinned_at` + `updated_at`，没有手动序号，无法精确插入到指定会话之前。

结论：5 个方法缺的是“把既有 Core 能力以通用形式开放给所有插件”，第 6 个方法（`insertSessionBefore`）缺的是数据模型。

#### 23.11.3 选项

**选项 A（推荐）：新增通用宿主能力 `host.project.*` + 会话归档，排序另立一条**

1. 新增 `host.project.list` / `host.project.create` / `host.project.update`（含 `title`、`pinnedAt`、`deleted`）/ `host.project.delete`，权限归入 `project.read` 与 `project.manage`（新增权限 ID），实现复用 `business.Edit` 与 `CreateProject`，不做第二套项目系统。
2. 新增 `host.conversation.archive`（`archived` 布尔，语义等价 `archived_at` 置空/置值），权限归入既有 `conversation.manage`；`stopActivity` 映射为“先中断在途 turn 再归档”，复用 `host.conversation.turn.interrupt` 的同一执行路径。
3. `insertSessionBefore` 单独立项：需要给会话增加手动排序值（候选：`conversations.manual_order` 整数或 `project_sessions.position`），涉及基线 schema、排序查询、前端与手机端列表排序，改动面明显大于前两条，建议单独一批，并在未落地前让该方法按原始 unsupported 错误失败，不做“近似排序”。
4. 这些能力按项目规则一律做成通用能力（`host.project.*` 对所有插件可用），DSH 容器只做映射，不新增强制绑定 DSH 的接口。

**选项 B：只用现有宿主 API 组合，`workspaces` 保持不可用**

1. `list` 可由 `host.conversation.list` 拼出只读视图，但 `create`/`rename`/`delete`/`archive*` 无通道。
2. 结果是 `workspaces` 长期停在 `unavailable`，任何依赖它的官方 Client 插件（官方 `api-workspace-controller` 系）在桌面端都不可用。

#### 23.11.4 影响与验收

1. 选项 A 的前两条（项目 CRUD 暴露 + 会话归档）改动集中在宿主 API 方法表、权限映射、kernel 处理器与插件权限声明，不新增业务逻辑，符合“优先改原系统、不造第二套”。
2. 验收：新方法具备方法级测试（成功、权限不足、参数非法、原始错误透传），catalog 中 `workspaces` 由 `unavailable` 变为 `partial`（`insertSessionBefore` 明确列为未支持），`dsh-abi` 与 `amitia-gap` 同步更新，`workspaces` 的 `reason` 只保留排序这一项真实缺口。
3. 该选项涉及 Core 新能力与权限 ID，按项目规则需确认后再实施；在确认前不改动 Core 任何代码。

#### 23.11.5 与 23.9.4 的关系

23.9.4 的落盘口径（方案 A：官方 provider 写插件数据目录并纳入备份清理；方案 B：自写 provider 全走 `host.state` / `host.resource`）决定 `pending` 15 项中 storage 系 9 项的实现路线，与本节互不替代：本节解决“项目与会话写操作如何进入 Core”，23.9.4 解决“官方 provider 的落盘位置”。

### 23.12 批次 D 落地记录（0.1.86）：Host 入口语义、激活隔离与安装器 peer 闭包

#### 23.12.1 目标与结果

批次 D 的入口是“按官方包实测结果重判 `jobController`”。实测过程中先暴露出三个真实缺陷，修复后才拿到可信结论：**Host 入口只能是包根导出**、**单个包激活失败不能污染持久化状态、也不能阻断宿主启动**、**安装器必须补齐 peerDependencies 闭包**。修完之后 `jobController` 由官方包在严格注入检查下直接激活，catalog 的 `delegated` 由 26 增至 27。

#### 23.12.2 缺陷一：把子路径导出当成 Host 入口

1. 原实现把 `exports` 里的每个子路径都当作可激活入口，于是 `./types`、`./typert`、`./remote` 这些纯类型 / FaceModel 数据模块也会被 `ctx.plugin(...)` 调用。真实 cordis 4.1.0 直接抛 `invalid plugin, expect function or object with an "apply" method, received object`，且该失败会让整个宿主启动失败。
2. 实测依据：官方 `@deepseek-ai/dsh-api-job-controller@0.2.2-rc.2` 的 `exports` 含 `"."`、`"./client"`、`"./types"`、`"./typert"` 等九个键，其中只有 `"."` 是可执行插件，其余是类型声明、FaceModel 协议与 locale。官方 loader 只取包根导出，容器必须一致。
3. 修复：`packageEntries` 只取 `exports["."]`（`string` 或 `import`/`default`/`node`/`require` 任一），缺失时回退 `module`、`main`，且必须命中 `.mjs|.cjs|.js`；同时允许“只有 `./client` 的纯前端包”通过描述阶段（`entry` 为空、`clientEntries` 非空），不再误报“缺少主入口”。
4. 测试口径：`tests/fixtures/multi-entry-plugin` 改成真实官方包形态（根导出是插件，`./types`、`./review`、`./search/client` 是子路径数据），原“发现并激活全部子路径入口”的用例改写为“只激活包根入口、其余子路径不动”，并新增“纯 client 包可被接受”用例。没有删除用例，也没有放宽断言。

#### 23.12.3 缺陷二：激活失败污染状态并阻断启动

1. 原实现先写 `enabled=true` 再激活，激活失败后持久化状态仍是“已启用”，于是每次重启都在同一处失败；`activate(context)` 又是串行 `for` 循环且不隔离异常，一个坏包会让整个宿主插件启动失败，最终表现为整机 Core `runtime.initialize` 报 `invalid plugin ...`（0.1.84 安装时就踩到这条）。
2. 修复：
   - `set-enabled` 先按状态解析记录并激活 / 反激活，成功后才 `mutateState(... setPackageEnabled ...)` 落库；失败直接抛出原始错误，不落库。
   - `activate(context)` 对每个已启用包单独 try/catch，失败时把 `{name, message, stack}` 记入 `activationFailures`，并用 `console.error("[dsh-activate-failed] plugin=... <stack>")` 打出完整堆栈后继续下一个包。
   - 包摘要新增 `active` 与 `activationError`，问题包在状态里可见，不再是黑盒。
3. 测试口径：新增“激活失败不落库、错误原样抛出”与“启用记录已损坏时宿主仍能启动并保留 `activationError`”两条回归用例。

#### 23.12.4 缺陷三：安装器不装 peerDependencies，且目标包名解析会被 peer 覆盖

1. 官方 seam provider 的依赖形态是 peer：`dsh-jobs-local` 声明 `dsh-jobs`、`dsh-agent`、`dsh-scope`、`dsh-timeout` 四个同源 peer。安装器为隔离私有依赖用了 `--legacy-peer-deps`，npm 因此一个 peer 都不装，加载即 `ERR_MODULE_NOT_FOUND: Cannot find package '@deepseek-ai/dsh-jobs'`。
2. 修复：把 npm 调用抽成可复用的 `runNpm`，首次安装后用 `missingPeerSpecs` 扫描 `node_modules` 里已装包的 `peerDependencies`，对缺失项按声明范围补装，最多 4 轮直到不动点；`@deepseek-ai/cordis` 列入 `HOST_PROVIDED_PACKAGES` 永不由 npm 安装，避免与容器自带的 vendored cordis 形成双实例；失败时保留 npm 原始输出与 `cause`，不换文案。
3. 补装 peer 会往私有 `package.json` 的 `dependencies` 追加键，而原实现用“第一个依赖键”反推目标包，补装后首个键会变成 `@deepseek-ai/dsh-agent` 之类的 peer，导致 `pluginId`、`packageDir` 全部指错包（实测报 `service "agents" has been registered`）。修复：`runNpmInstall` 在首次安装后记录目标包名并随结果返回，`installDshPackage` 优先使用它，旧路径仅作兜底；新增回归用例固定该行为。

#### 23.12.5 官方链路实测（`scripts/test-official-host-chain.mjs`）

脚本按加载顺序逐个安装、严格校验注入（`deferInjectCheck: false`）后加载，逐行输出 JSON，最后打印 `typert` / `jobs` / `jobController` 的可见性：

| spec | 结果 |
| --- | --- |
| `@deepseek-ai/dsh-typert-registry@0.2.2-rc.2` | `ok`，根入口 `./lib/index.js`，1 个 client 入口 |
| `@deepseek-ai/dsh-jobs-local@0.2.2-rc.2` | `ok: false`，`DSH_HOST_SERVICE_CONFLICT`：`service "jobs" has been registered` |
| `@deepseek-ai/dsh-api-job-controller@0.2.2-rc.2` | `ok`，根入口 `./lib/index.js`，1 个 client 入口 |

结论：`jobs` 由 Amitia `host-services` 已实现的作业运行时提供，官方 `jobs-local` 与之互斥，容器不安装它；`typert` 走官方 `typert-registry`，`jobController` 走官方 `api-job-controller`。三项服务在严格注入检查后均可见，因此 `jobController` 的路由由 `provider: "amitia"` 改为 `provider: "none"`，`note` 只保留实测事实。

#### 23.12.6 Catalog 与验证

1. 重新生成 catalog：`hostServiceCount` 69、`hostServiceDelegatedCount` 27、`hostServiceProviderPendingCount` 14、`hostServiceUnavailableCount` 13（合计 54），其余计数与批次 C 一致。
2. `node --test tests/*.test.mjs`：157 项全部通过（批次 C 为 153 项；本轮新增 4 项：纯 client 包、激活失败不落库、启用记录损坏仍可启动、目标包名解析；另把 2 项“子路径入口”用例改写为根入口语义用例）。
3. 打包 `amitia-dsh-compatible-host-0.1.86.amitiax`（treeHash `9a71b15a…`）并安装到本机 Core：`/api/health` 200（`ready: true`、`model: configured`、version 26.2.0-beta.22），`/readyz` 200（`state: ready`，7 ready、0 degraded、0 blocking、0 failed），Core 日志无 `[dsh-activate-failed]`。
4. 清理：探针目录 `.probe/`（两个官方 tgz、解包目录、临时探针脚本）已删除，测量能力保留在 `scripts/test-official-host-chain.mjs`。

#### 23.12.7 剩余工作

1. `pending` 14 项按 §23.9.4 方案 A 实施（官方 provider 落盘到插件数据目录并纳入备份清理）。
2. `workspaces` Client Service 按 §23.11 方案 A 实施（`host.project.*` + `host.conversation.archive`）。
3. `unavailable` 13 项与 `lsp` 的处置不变。

### 23.13 P13 落地记录（0.1.89）：`workspaces` 映射到通用项目与会话归档能力

#### 23.13.1 目标与结果

按 §23.11 选项 A 实施：Core 新增通用宿主能力 `host.project.list|create|update|delete` 与 `host.conversation.archive`，DSH 容器把 `ctx.workspaces` 的 6 个方法映射到这些通用能力。结果：`workspaces` 由 `unavailable` 变为 `partial`，6 个方法中 5 个落地，`insertSessionBefore` 保持原始 unsupported 失败，不做近似排序。

#### 23.13.2 Core 侧

1. 方法登记：`MethodProjectList`、`MethodProjectCreate`、`MethodProjectUpdate`、`MethodProjectDelete`、`MethodConversationArchive` 五个常量进入 `host_api`，并在 `DefaultPermissionMapping()`、`RoutePermissionForMethod`、`RouteScopeForMethod`、`IsDataRouteMethod`、`RouteRiskForMethod`、`RouteSideEffectForMethod` 全部登记；未登记权限映射的路由在注册阶段即被拒绝。
2. 权限定义：新增 `project.read`（`RiskLow`、`PersistentGrantable`）与 `project.manage`（`RiskHigh`、`RequiresPerUse`、`DefaultApproval: ApprovalManual`、`ChildDeny`）；`host.conversation.archive` 归入既有 `conversation.manage`。
3. 作用域解析：`resolveProjectHostScope` 依次合并 scope snapshot、`CallRequest` 的 `SpaceID`/`CharacterID` 与输入里的 `spaceId`/`characterId`，任一来源与已有值不一致返回 `scope_denied`，全部为空返回 `input_invalid: spaceId and characterId are required`。list / create / update / delete 四个处理器都走同一解析，不新增旁路。
4. 项目创建：`host.project.create` 增加 `directoryPath`，与显式 `workspaces` 互斥；Core 适配器调用既有 `workspace.Service.RegisterLocalMount` 生成真实本地挂载，再以该挂载作为项目主工作区交给 `business.CreateProject`。没有第二套项目系统，也没有伪造工作区记录。
5. 写入通道：update 只接受 `title`、`pinned`、`hidden` 与显式 `workspaces` 重建，delete 走 `business.Edit` 的删除语义；archive 用 `archived` 布尔映射 `archived_at` 置空/置值，`stopActivity` 先中断在途 turn 再归档。
6. 测试口径：新增 `TestProjectCreateRouteForwardsDirectoryPath`、`TestProjectCreateRouteRejectsDirectoryPathWithWorkspaces`、`TestProjectRoutesAcceptMatchingInputScopeAndRejectMismatch`（匹配作用域成功、四类不匹配一律 `scope_denied` 且端口不被调用）；`go build ./...`、`go vet ./internal/extension/kernel/` 与 kernel 定向测试全部通过。

#### 23.13.3 插件侧

1. runtime 新增 6 个 action：`conversation-archive`、`project-catalog`、`project-workspaces`、`project-create`、`project-update`、`project-delete`；`projectScopeInput` 把可选 `spaceId`/`characterId` 透传给宿主，`conversation-archive` 优先用请求里的 `scopeSnapshotId`，缺失时回退 `dsh-session:<conversationId>` 持久化快照并走 `host.callTargeted`。
2. 新增 `dsh-workspace-index` 插件状态：`project-create` 写入 `{workspaceId, projectId, path, title}`，`project-update` 同步标题，`project-delete` 剔除，写入走 `host.state.cas` 最多 8 次重试。
3. `sessionCatalog.refresh` 把索引项与会话派生项按 `workspaceId` 合并（会话派生项覆盖 `projectId`/`path`/`title`，缺失路径回退索引项），解决“新建项目在 DSH Workspace 目录不可见”。
4. `ctx.workspaces` 落地：`create({path})` 走 `project-create` 并刷新目录，`rename`/`delete` 必须先有已验证的 `ProjectId` 映射否则抛原始错误，`archiveSession`/`unarchiveSession` 映射 `conversation-archive`（`stopActivity` 原样透传），`insertSessionBefore` 抛 `unsupported("P13")`。
5. Manifest：新增 `project.read`、`project.manage` 权限声明与 `requiredPermissions`；`command` 工具 `enum` 增加 6 个 action，`properties` 增加 `archived`、`stopActivity`、`cursor`、`directoryPath`、`pinned`、`hidden`、`spaceId`、`characterId`。
6. 测试：`node --test tests/*.test.mjs` 164 项通过（0.1.86 为 157 项），新增 workspaces 方法级 4 项、runtime 项目与归档 3 项，并为测试夹具补 `project-*` 分支；未删除用例、未放宽断言。

#### 23.13.4 Catalog

`dsh-abi.v1.json` 的 client services 8 项分布为 implemented 4、partial 4（`workspaces` 由 `unavailable` 改为 `partial`、`mapping` 为 `composite`）；host services 69 项分布为 delegated 27、partial 9、implemented 6、pending 14、unavailable 13。`amitia-gap.v1.json` 的 client 缺口只剩 1 项（`workspaces` 的 `insertSessionBefore` 排序能力）。

#### 23.13.5 验证

1. 打包 `plugin/amitia-dsh-compatible-host-0.1.89.amitiax`，14711483 字节，SHA-256 `598854d3a0e01a8a4a99e3d46dfed404d0f9fdf2bfb930dda42e1583dea106ec`，treeHash `49645f85d38af11cc17d546726220cfd4455ef09dd956a52f11448fc4a36808e`。
2. `scripts/install-package.mjs` 安装到本机 Core：`version: 0.1.89`、`enabled: true`、`failureCount: 0`，generation 目录 `generation-54754b2e-f0b8-489b-8c6e-a7085f79c5f1`。
3. 以当前源码重建 darwin/arm64 `AmitiaCore` 并重启完整服务：`/api/health` 返回 `ready: true`、`runtime` 为 `state: ready`（7 ready、0 degraded、0 blocking、0 failed），前端 5178 返回 200。
4. 真实链路探测：`POST /api/extension/action/dsh-compatible-page/command` 调用 `project-catalog` 已进入 Core 的 `host.project.list`，返回的是业务侧 `input_invalid: spaceId and characterId are required`，说明方法注册、权限映射与原始错误透传均生效；直接 HTTP 调用没有 page session，作用域为空时的拒绝属于预期边界。
5. 未完成项：带 page session 的真实 UI 上下文里执行 create / rename / archive 的端到端验证；`insertSessionBefore` 依赖会话手动排序值，未落地前保持原始 unsupported。

#### 23.13.6 剩余工作

1. `insertSessionBefore` 需要会话手动排序字段（基线 schema、排序查询、桌面与手机列表排序同步改造），单独立项。
2. `pending` 14 项按 §23.9.4 方案 A 实施（官方 provider 落盘到插件数据目录并纳入备份清理）。
3. 在真实 UI 会话上下文补一次 `workspaces` 端到端验证。

### 23.14 P11 批次 D 落地记录（0.1.90~0.1.97）：共享容器、provider 落盘接线、声明型契约包

本批次目标是 §23.9.4 方案 A 的前半段：让官方 provider 真正在插件数据目录下完成落盘，且与 Amitia 的备份、清理口径一致。实现过程中按真实 Core 的失败原文逐层推进，结论如下。

#### 23.14.1 共享容器 realm

`0.1.90` 之前每个 DSH 包各自 `new Context`，导致 `dsh-storage` 提供的 `storage` 服务对 `dsh-storage-json` 不可见。改为同一 realm 后确认 cordis 的可见性规则：

1. `ctx.provide(name, value)` 写入 root store（key 为 `ctx.root[symbols.isolate][name]`），因此兄弟 fiber 可见。
2. `ctx.isolate("tools")` 只隔离 `tools` 一项，不影响其他服务。
3. `ctx.inject(deps, cb)` 产生的子 fiber 内调用 `provide` 仍写 root store，所以 `dsh-storage-domain` 在 `ctx.inject(["storage.backend.json"], ...)` 内部 `provide("storageDomain")` 后，`dsh-workspace` 能看到 `storageDomain`；前提是注入的 backend 服务已存在。
4. 同一名字重复注册会抛 `service "X" has been registered at <Y>`，可直接用于判定冲突来源。

据此确认官方 seam/provider 的接线口径：seam 只提供 hub（`dsh-storage`→`storage`，`dsh-storage-domain`→`storageDomain`），provider 注册后端（`dsh-storage-json`→`ctx.provide(storageBackendServiceKey("json"))`，`storageBackendServiceKey = name => "storage.backend." + name`）。`workspaces` 需要的 `workspaceRegistry`/`workspaceController` 确实只依赖 `storageDomain` 与 `sessionPersistence` 两个 seam，与 §23.9.4 的实测一致。

#### 23.14.2 provider 落盘接线

`src/runtime/provider-wiring.mjs` 现在承担两件事：

1. `providerDataDirectories`：`@deepseek-ai/dsh-storage-json`→`dsh-data/storage`，`@deepseek-ai/dsh-session-persistence-jsonl`→`dsh-data/sessions`，`@deepseek-ai/dsh-spill-local`→`dsh-data/spill`。目录在激活前 `mkdir -p`，配置以 `{ root }` 注入，全部位于插件数据目录内，因此天然落在 Amitia 的插件数据边界里，不需要第二套路径规则。
2. `seamConfigs`：`@deepseek-ai/dsh-storage-domain` → `{ backend: "json" }`。该包 `Config.backend` 为必填，此前注入 `{}` 会让所有依赖 `storageDomain` 的包报 `DSH 插件依赖的服务 storageDomain 不可用`，注入静态配置后消失。

实测证据：`AmitiaData/extensions-v2/plugin-data/com.amitia__dsh-compatible-host/dsh-data/storage` 与 `dsh-data/sessions` 均已被真实创建。

#### 23.14.3 声明型契约包不再被挂载

`@deepseek-ai/dsh-session-persistence` 不是插件，只导出一个抽象 `SessionPersistence extends Service`（构造内 `super(ctx, "sessionPersistence")`，无原型方法）。此前的加载器把默认导出统一当插件挂载，于是该抽象类被实例化并抢占 `sessionPersistence`，`dsh-session-persistence-jsonl` 再注册时报 `service "sessionPersistence" has been registered at <SessionPersistence>`。

`src/runtime/host-abi.mjs` 新增 `isDeclarationOnlyModule`：默认导出为函数、无 `apply`、无 `name`，且原型上除 `constructor` 外没有任何方法时才判定为声明型，加载后早退为 `{ plugin: null, declarationOnly: true }`，并由 `assertDshHostPluginActive` 短路放行。

首版判定误伤 `@deepseek-ai/dsh-lsp`（`tests/native-semantics.test.mjs` 报 `Cannot read properties of null (reading 'state')`）：`Lsp` 类带 `registerProvider`/`query` 原型方法，是真 seam，不能当库跳过，因此判定改为按原型方法数。另有一处错误是拿 `typeof candidate.apply === "function"` 判 `apply`，而 `Function.prototype.apply` 恒真，已删除该判断。新增夹具 `tests/fixtures/declaration-only-plugin` 与回归用例，断言该包 `active: true` 且构造函数从未执行。

#### 23.14.4 npm 安装脚本策略改为 `--ignore-scripts`

`koffi` 3.1.1 带 `install` 脚本 `node ./cnoke.cjs -P . -D src/koffi --prebuild --release`。该脚本的 `checkPrebuild()` 会用 `node -e "require(pkg)"` 探测平台预编译包 `@koromix/koffi-<os>-<arch>`，平台包存在时应为 no-op，但在受限宿主里执行构建脚本仍然失败，报 `npm error command sh -c node ./cnoke.cjs`。`package-manager.mjs` 的安装参数改为追加 `--ignore-scripts` 后安装成功，`import('koffi')` 正常加载平台预编译产物。第三方包在受限进程里执行任意安装脚本本身也不符合沙箱口径，该改动与安全姿态一致。

#### 23.14.5 实测状态（0.1.97）

安装包 `plugin/amitia-dsh-compatible-host-0.1.97.amitiax`，14716593 字节，SHA-256 `6aa0136f1613692ef92bbd59ed867f069c4bb53db63f498ff5f2a25b5bcbae65`，treeHash `f9f73da1423ec6f35fd21ccf7d1e0a6180bb22d62c3ef00b215198c1282c6ed3`，插件测试 168/168 通过，已通过 `scripts/install-package.mjs` 安装到本机 Core（`version: 0.1.97`）。

启用官方包的实测结果：

1. `@deepseek-ai/dsh-storage`、`@deepseek-ai/dsh-storage-json`、`@deepseek-ai/dsh-storage-domain`、`@deepseek-ai/dsh-session-persistence-jsonl`、`@deepseek-ai/dsh-typert-registry`、`@deepseek-ai/dsh-api-job-controller` 均为 `active: true`，`activationError` 为 `null`。
2. `dsh-data/storage` 与 `dsh-data/sessions` 目录真实创建。
3. `@deepseek-ai/dsh-workspace` 已越过 `storageDomain` 可用性检查，重新启用时命令返回 `{"ok":false,"error":{"code":"runtime_unavailable","message":"action command failed: execution_failed: tool execution failed: failed (execution_failed: javascript runtime: invoke error: jsonrpc: -32603: fsync API is disabled when Permission Model is enabled.)"}}`，包状态为 `activationError.message = "fsync API is disabled when Permission Model is enabled."`。

#### 23.14.6 阻塞项：Node Permission Model 禁用 fsync

在 Node v26.12.1 上，只要启用 `--permission`（任何 `--allow-fs-*` 组合，含仅放行单个可写目录），`fs.fsyncSync` 即抛 `fsync API is disabled when Permission Model is enabled.`。而 `dsh-storage-json`、`dsh-session-persistence-jsonl`、`dsh-workspace` 的落盘协议都是 temp-write → `fsync` → rename → `fsync(dir)`，因此所有 durable provider 在真实写入时必然失败。Core 侧 `backend/internal/extension/kernel/javascript_main/host.go:450` 的 `args := []string{"--permission"}` 是硬编码，461/474 只追加 `--allow-fs-read=`/`--allow-fs-write=`。

三条可选路径：

1. **方案 A（放行）**：对声明了 `hostAccess.filesystem` 的运行时去掉 `--permission`，改由 Amitia 的路径白名单与进程边界约束。代价是安全姿态变更，需改 `host.go`、重建 Core 并重启完整服务。
2. **方案 B（保留沙箱 + fsync 委托）**：在 `installDshImportHooks()` 中为 DSH 包（按 `parentURL` 限定）把 `node:fs`/`node:fs/promises` 映射到 shim，仅把 `fsync`/`fdatasync`/`FileHandle.sync` 委托给 helper 子进程。`registerHooks().resolve` 可以拦截并替换核心模块，`FileHandle` 实例也允许用自有属性覆盖 `sync`（探针 `overrideAssigned: true`），但实测**委托目标拿不到 fsync**：Node v26.12.1 在权限模型下会把自身的权限参数写进子进程的 `NODE_OPTIONS`（探针中子进程 `NODE_OPTIONS` 为 `--permission --allow-fs-read=... --allow-fs-write=... --allow-child-process`），显式传 `NODE_OPTIONS: ""` 也会被覆盖；经 `/bin/sh -c` 间接启动的 node 同样继承，子进程执行 `fsyncSync` 仍报同一条错误。即纯 Node 的 helper 子进程无法脱离权限模型，方案 B 在不引入非 Node 助手或 `sh -c 'unset NODE_OPTIONS'` 这类绕开注入的做法时不可行；后者本质等同于方案 A 且更脆弱，不作为候选。
3. **方案 C（记录为不支持）**：在 catalog 标注原始错误，Amitia 侧继续用 `host.state` CAS 提供会话持久化，官方 durable provider 保持 `pending`。

方案 B 经实测被排除后，推荐**收窄后的方案 A**：只对声明了 `hostAccess.process.mode == "direct"` 的运行时不再传 `--permission`，其余运行时保持现状。理由是这类运行时已经拿到 `--allow-child-process`，Node 自身在启动该参数时即警告「could invalidate the permission model」，且它可以启动非 Node 子进程（例如 `/bin/sh`）完成任意文件写入，权限模型对其本就不构成有效边界；对这类运行时继续保留 `--permission` 只会带来「看似受限、实际可绕」的错觉，同时把官方 provider 的 durable 落盘整体挡死。收窄方案 A 需要改 `backend/internal/extension/kernel/javascript_main/host.go` 的 `nodeLaunchArgs()` 并重建 Core、重启完整服务，属安全姿态变更，实施前需明确确认。方案 C 保留为兜底：若确认维持现有沙箱姿态不变，则官方 durable provider 继续 `pending`，catalog 记录原始错误，Amitia 侧仍以 `host.state` CAS 提供会话持久化。

#### 23.14.7 声明型包判定误读原型 getter（0.1.97 已修）

`@deepseek-ai/dsh-typert-registry` 在 0.1.95/0.1.96 报 `Cannot read properties of undefined (reading 'view')`，`dsh-api-job-controller` 因缺 `typert` 连带失败。堆栈指向 `isDeclarationOnlyModule`：判定用 `typeof prototype[key] === "function"` 遍历原型属性名，而该类的原型带访问器（`get remotes()`，内部读 `...view`），读取即执行 getter 并按内部状态抛错。根因是判定代码自己触发了被测对象的访问器，与陈旧 entries 无关。0.1.97 改为 `Object.getOwnPropertyDescriptor` 逐个检查 `value`/`get`，不再读取原型属性值；两个包随即恢复 `active: true`。新增夹具 `tests/fixtures/getter-prototype-plugin`（原型 getter 直接抛错）与回归用例，断言加载期间不读取 getter、包正常激活。

#### 23.14.8 收尾项当时状态（0.1.97）

1. `dsh-spill` + `dsh-spill-local` 安装启用与 `dsh-data/spill` 真实写入验证，被 fsync 阻塞。
2. catalog 中 `storage`、`storageDomain`、`sessionPersistence`、`projectionCache`、`spillStore`、`workspaceRegistry` 的状态推进，需同步 `scripts/generate-catalogs.mjs` 产物与 `tests/catalog.test.mjs`；`catalog/platform-support.v1.json` 由生成器写入，不手工编辑。
3. Core 侧把 plugin-data 纳入备份与卸载清理：`backend/internal/extension/backup_contributor.go` 当前只归档 `agent_skill` 的 `.amitiax`（NDJSON + base64），不含 plugin-data。

#### 23.14.9 durableSync 能力落地（0.1.98）

收窄方案 A 的最终实现形式是 manifest 显式 opt-in，而不是按 `process.mode` 推断：

1. manifest 新增 `hostAccess.filesystem.durableSync`，`plugincontract` 侧 `FilesystemAccess.DurableSync` 透传到 `domain.RuntimeFilesystemAccess.DurableSync`，再由 `supervisor_adapter.resolveHostAccess` 写入 `PluginHostAccess.FilesystemDurableSync`。
2. `validate.go` 增加约束：`durableSync` 必须与 `filesystem.mode == "read_write"` 同时出现，否则报 `durableSync requires filesystem mode read_write`。
3. `javascript_main/host.go` 的 `nodeLaunchArgs()` 首行改为：声明 `durableSync` 时只返回 `[pluginHostPath]`，不传 `--permission` 与任何 `--allow-fs-*`；其余运行时行为逐字节不变，因此 wechat / qq-personal / qq-clawbot 三个 direct 插件的沙箱姿态未变。
4. 新增 `TestNodeLaunchArgsSkipPermissionModelForDurableSyncRuntimes`，断言 `len(args) == 1 && args[0] == "/runtime/plugin-host/dist/index.js"`；`go vet` 与 `go test ./internal/extension/kernel/javascript_main/ ./internal/extension/kernel/plugincontract/` 通过。
5. Core 由官方脚本 `desktop/scripts/build-amitiacore.mjs` 从当前源码重建（同时写入 `desktop/resources/core/AmitiaCore` 与 `desktop/resources/core/macos-arm64/AmitiaCore`，SHA-256 `24203616a1aa53cda49a2acbdae9c8f3fdf59c261f9717a1c393795af31bf4b8`），`./start-all-services.sh` 重启后 `/api/health` 为 ready（7 ready / 0 degraded / 0 blocking / 0 failed）。
6. 运行实例证据：`ps` 中出现两个插件宿主进程——plugin-market 仍带 `--permission --allow-fs-read=...`，DSH 宿主为 `node .../AmitiaData/plugin-host/dist/index.js`，**没有任何权限参数**，证明 opt-in 只对声明方生效。
7. 直接后果：`@deepseek-ai/dsh-session-persistence` 启用成功，`fsync API is disabled when Permission Model is enabled.` 不再出现。

构建路径上有一个必须记录的坑：`start-all-services.sh` 的「准备核心」步骤会用 `desktop/resources/core/AmitiaCore`（根路径）覆盖 `desktop/resources/core/<target>/AmitiaCore`。手工只替换目标目录的二进制会被覆盖回旧版，表现为新字段解析失败 `kernel: manifest v2 parse: plugincontract: decode manifest: json: unknown field "durableSync"`（HTTP 422）。正确做法是走 `scripts/build-amitiacore.mjs`，它同时写两处并落 `.amitiacore-build.json` 时间戳；重建前若源码改动未进 freeze 清单，脚本会以 `freeze manifest does not match canonical scope` 失败，需先 `scripts/build-freeze-scope.mjs --write`。

#### 23.14.10 `sessions` 服务归属修正与 workspace 激活（0.1.99）

`dsh-workspace` 的激活错误从 fsync 变成 `sessions.list is not a function`，继续排查后确认是宿主自造的 `sessions` 服务遮蔽了官方契约：

1. 官方 `@deepseek-ai/dsh-session` 在 `lib/index.js:1820` 执行 `super(ctx, "sessions")`，它才是 `sessions` 的唯一契约提供者；官方 `dsh-session-persistence` 提供的是 `sessionPersistence`，两者不是同一服务。
2. 宿主 `createAmitiaDshHostServices` 早先为实现 `host.conversation.*` 桥接，自带了一个只有 `read` / `messages` / `resume` / `create` 的 `sessions` 对象，注册在 root store，于是 `dsh-workspace` 的 `ctx.get("sessions").list()` 拿到的是这个残缺对象。
3. 直接启用 `@deepseek-ai/dsh-session` 时报原始冲突：`service "sessions" has been registered at <root>`。
4. 修正方式沿用宿主既有命名空间约定（`events`→`amitiaEvents`、`runtime`→`amitiaRuntime`），在 `host-abi.mjs` 的 `reservedServiceNames` 增加 `sessions`→`amitiaSessions`。宿主对象本身保留（供桥接与用例使用），但不再占用官方服务名，也不再构成第二套 `sessions` 实现。
5. 0.1.99 打包（treeHash `e69901c6dbe4464f782426802e76eafc526af329d1e49d47654a80c00832d2fc`）并安装后，`@deepseek-ai/dsh-session` 与 `@deepseek-ai/dsh-workspace` 均 `active: true`、`activationError: null`。
6. 真实落盘证据：`plugin-data/com.amitia__dsh-compatible-host/dsh-data/storage/workspace.json` 由官方 storage-json provider 写出（`0600`，227 字节），内容为 `{"unit":{"name":"workspace","version":2},"global":{"initialized":true,...},"tables":{"workspaces":{}}}`，说明 durable 协议（temp-write → fsync → rename → fsync(dir)）在真实 Core 运行时中完整跑通，且 `dsh-workspace` 的默认工作区初始化路径也走完了 `sessions.list()`。

#### 23.14.11 catalog 状态推进

`catalog` 之前把「provider 已落地」和「provider 未实现」一律记为 `pending`，会把已完成的工作继续显示成缺口。0.1.99 在 `scripts/host-service-routes.mjs` 引入两个新的 provider 口径，并由 `HOST_SERVICE_PROVIDER_STATUS` 映射成已有状态值：

| provider | status | 含义 |
| --- | --- | --- |
| `amitia-landed` | `implemented` | Amitia provider 已在真实运行中落盘并有文件证据 |
| `amitia-wired` | `partial` | provider 已接线并激活，真实写入证据待补 |

据此推进：`storage`、`storageDomain`、`workspaceRegistry` → `amitia-landed`；`sessionPersistence`、`spillStore` → `amitia-wired`。`catalog/amitia-gap.v1.json` 新增 `resolved` 段列出这两类条目（含 `reason` 与 `consumerCount`），不再把它们从缺口清单里删掉了事；summary 增加 `hostServiceImplementedCount`（9）与 `hostServicePartialCount`（11），缺口清单从 54 降到 49（delegated 27、pending 9、unavailable 13）。`tests/catalog.test.mjs` 同步更新计数并新增用例 `records landed and wired Amitia providers instead of hiding them as gaps`，逐项断言五个能力出现在 `resolved`、不再出现在缺口清单、状态与 provider 匹配；`node --test tests/catalog.test.mjs` 7 项全部通过。

`dsh-spill` + `dsh-spill-local` 已安装并 `active: true`，`dsh-data/spill` 目录随 provider 激活创建，但尚无真实溢出文件，因此保持 `amitia-wired` 而不冒充 `implemented`。

#### 23.14.12 仍未完成（前两项已由 §23.14.15 闭环）

1. ~~`sessionPersistence` 与 `spillStore` 的真实写入证据~~：已闭环。官方 `sessions.create` 只在官方 agent/session 链路被调用时触发，Amitia 侧原 `agents.create` 走 `host.conversation.*`；§23.14.15 让出 `agents`/`sessionProjections` 两个官方服务名后，官方 agent-loop 真实执行，产出 `sessions/<cwd 编码>/amitia-probe-agent-3/session.v4.jsonl.zstd`（208 字节）与 `spill/session-55ce772d1796/2941c753b331-probe-0105.txt`（960 字节）。
2. ~~`sessionProjectionCache` 的真实 checkpoint 写入证据~~：已闭环。`storage/session_projcache/sessions/amitia-probe-agent-3.json`（858 字节、`version: 7`）已由官方 `@deepseek-ai/dsh-session-projection-cache` 写出。
3. 仍未完成：真实模型 turn 内的自动溢出与 `turn/end` checkpoint。上述证据分别来自会话创建切点与 `spillStore.saveText` 直调，需先由官方 `llm` adapter 接上 Amitia 模型网关。

#### 23.14.13 plugin-data 纳入备份与卸载清理（0.1.100）

0.1.99 之前 `backend/internal/extension/backup_contributor.go` 只归档 `agent_skill` 的 `.amitiax`，DSH 的 `plugin-data` 目录完全在备份之外；`data.uninstall=delete` 也没有对应实现路径。0.1.100 补齐这两条链路。

**契约层（`internal/extension/kernel/plugincontract`）**

1. `DataContract` 新增 `PluginData *PluginDataContract`，字段为 `include []string`；`validate.go` 要求 `data.pluginData` 必须与 `data.backup=true` 同时出现、`include` 至少一条。
2. `pluginDataIncludeIssue` 拒绝空串、前后空格、`/` 或 `\` 开头的绝对路径、反斜杠、`path.Clean` 后落到 `.`/`..`/`../` 的越界路径，以及未规范化路径；每条拒绝原因写入报告，不做静默纠正。
3. `domain_projection.go` 把 `include` 投影为 `ExtensionDefinition.Metadata["data.pluginData.include"]`；DB 往返后类型为 `[]any`，读取侧 `metadataPathList` 同时接受 `[]string` 与 `[]any`，不做类型断言兜底。

**备份组件（`internal/extension/plugin_data_backup.go`）**

1. 组件 id `extension.plugin_data.v1`、logical name `extension.plugin_data`，与既有 `extension` 组件一起在 `ComponentIDs()` 声明（`validateManifestComponents` 按白名单校验，未声明即失败）。
2. 物理根为 `PluginDataRoot(extRoot, extensionID) = <ExtRoot>/plugin-data/<safeID>`，`safePluginDataID` 把 `/`、`\` 映射为 `__`、`:` 映射为 `_`、`..` 映射为 `_`；DSH 实际目录为 `com.amitia__dsh-compatible-host`。
3. 只导出 manifest 声明路径：`ExtensionPluginDataBackupPaths` 在 `data.backup` 为假时返回 nil，因此未声明的插件不会产生该组件条目（`Plan` 的 `ItemCount` 为 0）。DSH 声明 `include: ["dsh-data"]`，`dsh-packages`（npm 缓存的 220MB 可重装产物）不进入备份；包注册表状态在 kernel `host.state`，本就不在 plugin-data。
4. `collectPluginDataFiles` 用 `WalkDir` 收集，拒绝 symlink 与非普通文件、按路径排序保证归档稳定，单文件上限 64MiB、总量上限 512MiB；缺失的 include 目录直接跳过而不报错。
5. 记录结构 `{extensionId, path, mode, size, sha256, contentBase64}`，恢复侧 `writePluginDataRecord` 逐条校验：路径必须被 manifest 声明（`pluginDataPathDeclared`）、`size` 与 `sha256` 必须与内容一致，否则返回 `ErrBackupComponentFailed`；写入先 `MkdirAll 0700`，落到 `*.restore.tmp`（0600）后 chmod 到记录中的 mode 再 rename，不覆盖半成品。
6. `ExtensionBackupContributor` 的 `Plan`/`Export`/`PreviewImport`/`Import` 全部接入：`Import` 先 `RestoreExtensions` 再 `restorePluginData`，顺序保证定义先于数据存在。

**卸载清理**

1. `Runtime.Uninstall`（`lifecycle_ops.go`）在 delete/export 分支追加 `deleteExtensionPluginData`。
2. `ExecutePackageUninstall` 的卸载 saga（`package_rollback_uninstall_saga.go`）在事务成功后按同一策略删除；清理失败时 `SetOperation(..., "cleanup_plugin_data", "PACKAGE_PLUGIN_DATA_CLEANUP_FAILED", cleanupErr.Error(), ...)` 并 `errors.Join(cleanupErr, persistErr)` 透传原始错误。
3. `retain` 策略不动 `plugin-data/<safeID>`，与 DSH manifest 的 `data.uninstall=retain` 一致。

**定向测试**

`contract_test.go` 新增投影与六种非法 include 拒绝（含缺 `data.backup`）；`package_data_policy_test.go` 覆盖 retain 保留、delete/export 删除并新增 `TestExtensionPluginDataBackupPathsRequireBackupFlag`（含 safeID 映射断言）；`plugin_data_backup_test.go` 覆盖声明路径遍历（排除 `dsh-packages`）、symlink 拒绝、记录往返保持 0600、未声明路径与篡改内容拒绝、越界路径拒绝；`plugin_data_backup_contributor_test.go` 用 stub 仓库与 `kernel.NewRuntime` + `SetContainer` 覆盖 plan/export/restore 往返与未声明插件的跳过。

**真实环境证据**

Core 由当前源码重建：Source Gate `414d3597cd9939e0f3ed609526bcab3ea0311d7ff01610e8fd02bd9288df69fc`，`AmitiaCore` SHA-256 `d584af9d149cede3dc14f42b291fede8a59d145619b38f131251a1e9de12b9ed`；重启后 `/api/health` 为 `ready: true`（7 ready、0 degraded/blocking/failed），`/readyz` 返回 200。插件 0.1.100（14721520 字节、SHA-256 `c444aba354dfcd5a14ed608e2aadb4552302706962f58ed2fa70d72edee63966`、treeHash `5d070bf686c0a4c264ce06dbd61ddf08871fb509c0f60276863270d39edde30d`）安装后，定义元数据落库为 `{"data.backup":true,"data.export":true,"data.pluginData.include":["dsh-data"],"data.schemaVersion":1,"data.scopes":["extension"],"data.uninstall":"retain"}`。以口令创建真实备份 `backup-2712fa24-…`（18 组件），解密后 `datasets/extension.plugin_data.v1.ndjson` 仅 1 条记录：`dsh-data/storage/workspace.json`，227 字节，sha256 `8332e072a55f6c6ac287e41ac1a57640d630c2143cd55eeba907a3a33fb3240e`（与磁盘一致），mode `384`（0600）；`dsh-packages` 未出现在归档中；`restore`/`preview` 均返回 200 且组件可解析。验证产生的临时解密目录与备份文件已删除。

#### 23.14.14 `sessionProjectionCache` 安装接线（0.1.101~0.1.102）

`sessionProjectionCache` 是 0.1.99 之后剩下的 `pending` 项之一，缺口清单里它的 `provider` 记为 `amitia`（待实现）。本轮按官方契约把它接到官方包上，没有写第二套缓存实现。

1. 安装 `@deepseek-ai/dsh-session-projection-cache@0.2.2-rc.2` 后首次激活报原始错误 `invalid config:\n  - $.writeEveryEvents missing required value (at writeEveryEvents)` 与 `... $.writeIntervalMs ...`。
2. 查包内 `lib/index.js:120`：`Config = z.object({ writeEveryEvents: z.natural().min(1).required(), writeIntervalMs: z.natural().min(1).required() })`，即两项都是必填而不是有默认值。官方 README 的示例配置为 `writeEveryEvents: 200`、`writeIntervalMs: 5000`。
3. 容器的配置注入点已有 `seamConfigs`（此前只有 `@deepseek-ai/dsh-storage-domain` → `{backend: "json"}`），0.1.101 在 `src/runtime/provider-wiring.mjs` 追加 `@deepseek-ai/dsh-session-projection-cache` → `{writeEveryEvents: 200, writeIntervalMs: 5000}`，取值来自官方 README 示例，不另造默认值。
4. 新增 `tests/provider-wiring.test.mjs` 四个用例：seam 包拿到必填配置且不创建 provider 目录、provider 包拿到已创建的 `dsh-data/<name>` 根目录、未知包拿到空配置、`pluginDataRootFromPackageDir` 能从 `dsh-packages` 逐级解析到插件数据根（并在根目录本身传入时返回空串）。
5. 插件 0.1.101（treeHash `baf6f5216abf488fa43857adcc262ffe26f183a1c0e9ec6ee80fe3e4cc5ee171`）安装后 `active: true`、`activationError: null`；`session_projcache` 存储域在 init 阶段打开。
6. catalog 推进：`scripts/host-service-routes.mjs` 的 `sessionProjectionCache` 由 `amitia`（pending）改为 `amitia-wired`（partial），原因写明注入的两项配置与三个 checkpoint 切点；`generate-catalogs.mjs` 重新生成后 `hostServicePartialCount` 11→12、`hostServiceProviderPendingCount` 9→8、缺口 49→48，`catalog/amitia-gap.v1.json` 的 `resolved` 新增 `ctx.sessionProjectionCache`。`tests/catalog.test.mjs` 同步更新缺口总数、pending 计数，并在「records landed and wired Amitia providers」用例里新增该项的 provider 与 status 断言。
7. 插件 0.1.102（14722201 字节、SHA-256 `a3bf206efc59497de383eb49f17308a1ce6da03b9bda79968af9425d008eb6c4`、treeHash `1ee936b1bc35581d85c9efddeed0c97c172213cb39b8deb906c1d3634b2d19de`）安装后 `enabledCount 16`、`packageCount 16`，`@deepseek-ai/dsh-session`、`@deepseek-ai/dsh-spill-local`、`@deepseek-ai/dsh-session-projection-cache` 均 `enabled: true`、`active: true`；status 返回的 catalog 计数与生成结果一致（delegated 27、implemented 9、partial 12、pending 8、unavailable 13、缺口 48）。插件测试 173/173 通过。
8. 仍未落盘：`dsh-data` 下只有 `storage/workspace.json`，`sessions`、`spill` 为空，投影缓存也未写出 checkpoint——三者依赖同一条官方会话链路，与 §23.14.12 第 1 条同一阻塞。（该条已在 §23.14.15 闭环：会话文件、checkpoint 与 spill 文件均已真实落盘。）

#### 23.14.15 官方 agent 链路探路与服务名归位（0.1.103~0.1.105）

§23.14.12 的两个未完成项都卡在「官方会话链路没被调用」。本轮不打补丁式伪造，而是让官方 agent-loop 真正跑通 `agents.create()`，代价是又暴露两处容器门面占用官方服务名的问题。

1. `agents` 归位（0.1.103）。官方 `@deepseek-ai/dsh-agent` 以 `super(ctx, "agents")` 注册服务，`@deepseek-ai/dsh-agent-loop` 注入 `agents` 并调用 `agents.setFactory`。容器为桥接 `host.conversation.*` 自造的 `agents` 门面没有 `setFactory`、`turnBoundaryOf`，属于第二套实现。按既有命名空间约定在 `src/runtime/host-abi.mjs` 的 `reservedServiceNames` 追加 `agents`→`amitiaAgents`，夹具 `official-style-plugin`、`official-agent-create-plugin` 同步改注入名；新增夹具 `official-agents-plugin` 做反向断言：未安装官方 agent 包时按官方名注入必须报 `DSH_HOST_SERVICE_MISSING`，证明容器不再冒充官方服务。`tests/host-abi.test.mjs` 新增对应用例。

2. `sessionProjections` 归位（0.1.104）。`agents` 让名后 `ctx.agents.create()` 直穿官方 agent-loop，`initializeAgent` 在 `modelProjectionHandle.readerFor(...)` 抛原始错误 `invoke error: jsonrpc: -32603: this.modelProjectionHandle.readerFor is not a function`。根因是容器的 `sessionProjections` 门面 `register()` 只返回带 `stateOf` 的 disposer、没有 `readerFor`，而官方 `@deepseek-ai/dsh-session-projection` 才是 `sessionProjections` 的契约提供者。处理：`reservedServiceNames` 追加 `sessionProjections`→`amitiaSessionProjections`，安装并启用官方 `@deepseek-ai/dsh-session-projection@0.2.2-rc.2`。`agent-loop` 若因 retry 顺序仍停在失败态，用 `set-enabled false` 再 `true` 重激活即可。测试侧新增夹具 `official-session-projections-plugin`（`inject = ["sessionProjections"]`）验证官方名不被容器占用，并在 `host-abi.test.mjs` 用例里先断言容器 facade 改名为 `amitiaSessionProjections` 后仍能注册 `todo` 投影，再断言官方名缺失时报 `DSH_HOST_SERVICE_MISSING`。

3. 落盘证据（0.1.105）。探针夹具 `official-agent-create-plugin`（版本 1.0.1，`inject = ["agents", "spillStore", "tools"]`）暴露三个工具：`official_agent_create_probe`（`ctx.agents.create({sessionId, meta: {cwd}})` → `{agentId, live}`）、`official_agent_dispose_probe`（`handle.dispose()`）、`official_spill_save_probe`（`ctx.spillStore.saveText({owner: {sessionId}, suggestedName, content})`）。经宿主命令通道 `POST /api/extension/action/dsh-compatible-page/command`（header `X-Amitia-Local-Token`）调用后，三处文件均为官方 provider 直写，容器不参与写盘：

| 证据 | 路径 | 大小 | SHA-256 |
| --- | --- | --- | --- |
| 官方会话 | `dsh-data/sessions/<cwd 编码>/amitia-probe-agent-3/session.v4.jsonl.zstd`（附 `session.lock`） | 208 | `11c6b23422472a93b08b9e03a1fc5fc98ef22b0aa3502493f268872fa2e9c6c9` |
| 投影 checkpoint | `dsh-data/storage/session_projcache/sessions/amitia-probe-agent-3.json`（`version: 7`，`rows` 含 `turnBoundary`/`modelSelection`/`inbox`） | 858 | `ba8daa18d93be1d9fddeedc6d99b7d09e0ce2d0a9750ba38f9f39d32b81387c7` |
| spill 溢出 | `dsh-data/spill/session-55ce772d1796/2941c753b331-probe-0105.txt`（mode 600，内容逐字节一致） | 960 | `b4d0ad6c1d9dd2271818948ceeee5a7722c2055355b3bc60cb36f6751dc4caa4` |

会话文件 zstd 解压后为 `{"type":"session","version":4,"id":"amitia-probe-agent-3","cwd":<绝对路径>,"isSeeded":false,"delegationDepth":0}`，即 `sessions.prepare` → `createStoredSession` → `sessionPersistence.create(header)` 全链路执行；`meta.cwd` 必须是绝对路径，否则官方 `agents.create` 直接拒绝。宿主侧 20 个官方包全部 `active: true`、`activationError: null`，`mappedContributionCount` 为 3（`turnBoundary`、`modelSelection`、`inbox`）。

4. catalog 推进：`sessionPersistence`、`sessionProjectionCache`、`spillStore` 由 `amitia-wired` 升为 `amitia-landed`，`sessionProjections` 新登记为 `official-package` + `amitia-landed`；`hostServiceImplementedCount` 9→12、`hostServicePartialCount` 12→9，缺口并集 47（delegated 27、pending 8、unavailable 13）。`tests/catalog.test.mjs` 的落地清单新增 `ctx.sessionProjections`，三项状态断言由 `partial` 改 `implemented`。

5. 版本与校验：0.1.103（14722231 字节、SHA-256 `856f42ae878362e687f6a001e5b56bae9903a45ff386dba6ffe337e87871dd64`）、0.1.104（14722285 字节、SHA-256 `7698b051ce992e55585aef53009df315afff216e68e97bd01955208f8c6d0254`）、0.1.105（14724099 字节、SHA-256 `08ddea02f7602b0645380d3faa2fb6e7eabdc2dd6636bd1e4846512a3ee19c9c`、treeHash `4d50d20afb114698a1a481aaa2b57dd7ee4a70e33914c7e54860a67efc06124a`）。插件全量测试 175/175 通过（catalog 7/7、host-abi 10/10、host-services 11/11）。

6. 仍未打通：真实模型 turn。官方 `llm` adapter 接上 Amitia 模型网关后，`agentLoop` 才会产生模型调用、上下文溢出与 `turn/end`；因此本轮 spill 证据来自 `spillStore.saveText` 直调，checkpoint 证据来自会话创建切点，都不是 turn 内自动触发。容器门面与官方同名的服务还剩 `tools`（`host-abi.mjs` 每插件 `provide("tools")`）、`llm`、`approval`、`commands`、`systemPrompt`、`typert`，需逐个确认归属：官方有对应契约包的让名，官方没有的才由容器自持。

#### 23.14.16 真实模型 turn 打通：官方 agent-loop 驱动、同名门面让名与宿主定向授权（0.1.106~0.1.117）

§23.14.15 第 6 条列出的缺口本轮闭环：官方 `@deepseek-ai/dsh-agent-loop` 现在真正驱动 Amitia 模型，`turn/start` 到 `turn/end` 全链路事件、投影 checkpoint 与 spill 均由官方实现产出。

1. 模型边界（Go Core）。`host.model.invoke` 成为插件侧模型调用的唯一出口，复用 `internal/modelprotocol` 与当前激活模型配置，不再让插件直连模型网关。

2. 官方 LlmAdapter 契约（0.1.107）。`src/runtime/model-adapter.mjs` 把官方 `GenerateOptions` 译为 `host.model.invoke` 请求：`prepareCall(provider, model, signal)` 与自研实现一致，`stream` 产出 `block-start`/`text-delta`/`reasoning-delta`/`tool-call-delta`/`block-end`/`usage`/`finish`；`registerAdapter(["amitia"], adapter)` 注册到容器 `ctx.llm`。

3. 同名门面继续归位（0.1.114~0.1.115）。`reservedServiceNames` 追加 `tools`→`amitiaTools`、`systemPrompt`→`amitiaSystemPrompt`；官方 `@deepseek-ai/dsh-tools`、`@deepseek-ai/dsh-system-prompt` 安装并启用后，`agent-loop` 才能在 `loopCtx.tools.resolve` 与 `loopCtx.systemPrompt.assemble` 上拿到官方实现。每插件的工具门面原先用 `isolated.provide("tools")`，归位后在共享容器 realm 内会与 root 服务名冲突（`service "amitiaTools" has been registered at <root>`），改为挂在插件上下文属性 `entryContext.amitiaTools`，不再占用服务名，夹具随之从 `inject` 列表移除该名。

4. 真实 turn 驱动方式（0.1.112~0.1.117）。`diagnose` 动作改用官方 `ctx.agents.create({sessionId, meta: {cwd}, agentOptions: {provider, model}})`，`meta.cwd` 必须是绝对路径；`agent.followup(...)` 的用户消息必须带 `role: "user"` 与 `source: {kind: "user"}`，否则官方消息校验读到 `undefined`；`agent.whenIdle()` 之后用 `session.snapshotEvents()` 与 `session.deriveMessages()` 取证，`spillStore.saveText` 写入 turn 文本。

5. 宿主授权层补齐目标快照（Go Core）。`host.conversation.read` 等会话级路由的 `RequireRoles` 原本只认 `ScopeSnapshotID`，导致 `callTargeted(method, input, scopeSnapshotId)` 一律 `required roles [conversation] not satisfied`。`host_api` 新增 `TargetScopeChecker` 可选接口，`DefaultGateway` 在 `TargetScopeSnapshotID` 非空时改走 `CheckTarget`，未实现该接口的 checker 直接 fail-closed 报错；`ManagerScopeChecker.CheckTarget` 校验调用方自身快照、目标快照存在与未过期，并逐项比对 extension/module/generation/space/character/conversation，最后用目标快照做角色判定。新增 `scope_adapter_target_test.go` 覆盖 7 个用例。

6. 实测证据（0.1.117，Core `http://127.0.0.1:52358`，模型 `deepseek-chat`）。`POST /api/game-center-debug/tools/invoke` 执行 `{action: "diagnose", cwd: "/Users/untrammelled/Desktop/Project/U-AI"}` 返回 `Status: success`，`DurationMS: 2742`：

| 证据 | 值 |
| --- | --- |
| 会话事件 | `agent/inbox/spliced` 2、`turn/start` 1、`step/start` 1、`system/message` 1、`user/message` 1、`request/header` 1、`request/context` 1、`assistant/message` 1、`step/end` 1、`turn/end` 1，共 11 条 |
| turn 结果 | `turnStart: 1`、`turnEnd: 1`、`assistantMessages: 1`、`messageCount: 3` |
| 模型返回 | `收到，DSH 兼容宿主自检通过。✅\n\n当前状态正常，可以继续交流或执行后续任务。请告诉我你需要做什么。` |
| token 用量 | `inputTokens: 18`、`outputTokens: 119`、`totalTokens: 137` |
| 官方会话文件 | `plugin-data/com.amitia__dsh-compatible-host/dsh-data/sessions/--Users-untrammelled-Desktop-Project-U-AI--/dsh-diagnose-mwhy8qb1/session.v4.jsonl.zstd` |
| 投影 checkpoint | `dsh-data/storage/session_projcache/sessions/dsh-diagnose-mwhy8qb1.json`，914 字节；`turnBoundary` 为 `{"openTurnStartSeq": null, "lastStepStartSeq": 3, "lastStepBoundary": {"kind": "end", "seq": 9}, "lastTurn": 1}`，`modelSelection.lastUsed` 为 `{"provider": "amitia", "model": "deepseek-chat"}`，`inbox` 为空 |
| spill 落盘 | `dsh-data/spill/session-e6b923c03f4d/8d4b29ef6cfc-diagnose.txt`，141 字节，内容与 `assistantText` 逐字节一致 |

7. 官方包启用清单（运行时 `status` 显示 23 个包、21 个 `active`）：`dsh-agent-loop`、`dsh-agent`、`dsh-llm`、`dsh-tools`、`dsh-system-prompt`、`dsh-tool-executor-native`、`dsh-session`、`dsh-session-projection`、`dsh-session-persistence`、`dsh-session-persistence-jsonl`、`dsh-session-projection-cache`、`dsh-scope`。插件 `package.json` 同步声明这些依赖，保证本地测试与运行时解析一致。

8. turn 内工具调用已打通（0.1.118）。`diagnose` 支持 `toolProbe`：在官方 `agents.create` 的 `setup(agentCtx)` 中通过 `agentCtx.tools.register` 注册 `amitia_probe_<sessionId>`（`parameters` 为 open object、`output.schema` 为 `{ok: boolean}`、`execute` 返回 `{ok: true}`），提示词要求模型调用该工具。真实 Core（`http://127.0.0.1:54918`）返回 `Status: success`，`DurationMS: 2222`，`eventCount: 16`：

| 证据 | 值 |
| --- | --- |
| 事件 | `tool/call` 1、`tool/result` 1、`step/start` 2、`step/end` 2、`assistant/message` 2、`turn/start` 1、`turn/end` 1 |
| 探针工具 | `amitia_probe_dsh_diagnose_mwhykxtz` |
| 最终回答 | `工具返回的 ok 值为 true。` |
| 工具结果回灌 | 第二个 step 的 `assistant/message` 已包含工具返回值，证明 `dsh-tool-executor-native` 执行与官方批次回灌生效 |
| checkpoint | `dsh-diagnose-mwhykxtz.json` 的 `turnBoundary.lastStepStartSeq` 为 12、`lastStepBoundary` 为 `{"kind": "end", "seq": 14}`、`openTurnStartSeq` 为 null |
| spill | `spill/session-684bf0a4e437/efd865f8d6ba-diagnose.txt`，33 字节，内容 `工具返回的 ok 值为 true。` |

9. 同名门面全部归位（0.1.119~0.1.120）。`reservedServiceNames` 追加 `approval`→`amitiaApproval`、`commands`→`amitiaCommands`，并安装启用官方 `@deepseek-ai/dsh-user-approval@0.2.2-rc.2`（服务名 `approval`，`Config.policy` 默认 `ask`）与 `@deepseek-ai/dsh-commands@0.2.2-rc.2`（服务名 `commands`），两者 `active: true`、`activationError: null`；`typert` 容器从未占用（官方 `dsh-typert-registry` 注册 `typert`），因此无需让名。让名后复测 `diagnose`（`toolProbe: true`）仍为 `Status: success`，`tool/call` 1、`tool/result` 1、`step/start` 2、`assistantText` 为 `ok 的值为 true。`；同时移除两条失效的 `@amitia-test/*` 探针包（它们缺官方 `output {schema, render}` 声明），运行时最终为 24 个包全部 `active`。

10. 上下文自动压缩已打通（0.1.121~0.1.125）。`chat.ModelGatewayResult` 与 `kernel.ModelInvokeResult` 新增 `contextWindow`（取自当前激活模型配置的 `context_window`），adapter 在 `host.model.invoke` 返回后缓存，并在 `resolveModel` 里返回官方 `LlmModelContext`（`resolveModel` 是 `LlmRuntime.prepareCall` 计算 `context` 的唯一来源，`adapter.prepareCall` 上的 `context` 会被丢弃）。安装并启用 `@deepseek-ai/dsh-token-meter@0.2.2-rc.2` 与 `@deepseek-ai/dsh-compaction-basic@0.2.2-rc.2`，`provider-wiring.mjs` 为其注入 `auto: true`、`summarizationProvider: "amitia"`、`summarizationModel: "deepseek-chat"`；模型配置补全 `context_window: 128000`。验证期使用 `thresholdRatio: 0.0005` 触发，真实 turn 记录两次压缩：`compaction/start` → `compaction/summary`（`shadowedRange` 5–6，`shadowedTokenCount` 699）→ `compaction/end`（`error: null`），以及第二次 `shadowedRange` 15–15、`shadowedTokenCount` 464、`compaction/end` `error: null`；`request/context` 事件同时出现 `contextWindow: 128000`。验证完成后配置恢复为官方默认阈值，仅保留摘要模型绑定。运行时 26 个官方/夹具包全部 `active`。

11. spill 自动写入已打通（0.1.126~0.1.127）。官方 `dsh-spill` 只是 seam，消费者是 `dsh-spill-policy`、`dsh-tool-fs-search`、`dsh-tool-cordis`、`dsh-session-reference`。安装并启用 `@deepseek-ai/dsh-spill-policy@0.2.2-rc.2`、`@deepseek-ai/dsh-tool-fs-search@0.2.2-rc.2`、`@deepseek-ai/dsh-subprocess@0.2.2-rc.2`（seam）与 `@deepseek-ai/dsh-subprocess-local@0.2.2-rc.2`（provider）。期间暴露并修复一处宿主缺陷：`normalizeHostPlugin` 在把官方模块包转成插件对象时丢弃了 `Config`，导致 cordis 不做 schemastery 解析、默认值不生效，`fs-search` 报 `globMaxResults must be a positive integer`；改为保留 `Config`（函数式插件同样保留）后 `active: true`。`provider-wiring.mjs` 为 `fs-search` 注入必填的 `sampleOverCapGlobResults: false`。真实 turn（`diagnose` + 提示词要求用 grep 搜索 `backend` 下的 `func `）返回 `Status: success`，`tool/call` 1、`tool/result` 1，模型回答 `42645`；`dsh-data/spill/session-f695d585c80d/9ec63dc2d7c4-grep-results.txt` 为官方 `spill-policy` 自动写入，4231146 字节、52809 行、SHA-256 `032fc1ff06c2f943fb7a84828ff2020207391d997c69823c82d06d274a7c752e`，文件头为 `Found 42645 matches` 加逐行匹配；同目录 `75ecd7521415-diagnose.txt`（5 字节）仍是 `diagnose` 显式写入。运行时 30 个包全部 `active`。

12. approval 应答链路已接到 Amitia 原生审批系统（0.1.128~0.1.135）。新增 `host.approval.request` 宿主边界（权限 `approval.request`，允许作用域含 extension/module/character/conversation），内部走既有 `execution.ApprovalBroker.Await`，把 DSH 的 `approval/request` 交给 Amitia 的审批通道而非原生 `host.ui.dialog`：`src/runtime/approval-bridge.mjs` 注册 cordis waterfall 监听器并在 `diagnose` 的 `setup` 中安装，`diagnose` 支持 `approvalProbe`（先 `host.conversation.create` 取 `scopeSnapshotId`，再用 `host.callTargeted` 定向发起）。为容纳人工决策等待，`runtime/plugin-host` 的 `host.call`/`host.callTargeted` 新增可选 `options.timeoutMs`（`rpc.sendRequest` 早已支持自定义超时），`runtime.DefaultResourceLimits().SingleCallTimeout` 由 `30s` 调整为 `5m`。Amitia 侧把 `requestId` 为空的审批视为会话级审批：`ownedRuntimeApprovals` 会列出它们（`scope: "conversation"`、`turnId: ""`），`chat-runtime/approval` 决议对会话级审批放宽 `requestId` 匹配，前端 `AgentApprovalGuard` 允许 `turnId` 为空并对齐结果校验。实测：`diagnose` 的审批请求真实进入 `ApprovalBroker` 并持续等待 179 秒后在诊断路径上超时 fail-closed（工具返回 `unavailable`），证明 host→broker 全链路可达；`go test ./internal/extension/kernel` 覆盖批准/拒绝/缺 broker/缺会话/空工具名 5 项，`front` tsc 通过。现已闭环（0.1.135 + Core Source Gate `1b5c409f…`）：外部复现前端请求的两个关键点已定位——单聊会话**不能**在 `expectedExecutionScope` 里带 `conversationRoleId`（带上会触发 group 校验并返回 `409 禁止将设备专属数据保存到其他设备`），以及会话级审批的 `requestId` 因 `omitempty` 在 JSON 中缺失（`nil` 而非 `""`），决议匹配必须按"字段不存在"处理。修正后用真实审批链路实测：`chat-runtime/snapshot` 返回 `pending` 审批项（`arguments` 为 `插件请求执行工具：amitia_approval_…`），`POST chat-runtime/approval` 返回 `200 {"approved": true}`，模型回答 `outcome 为 allowed-once（本次已获准一次）`，`approval/asked` 与 `approval/decided` 各 1 条，`turn/end` 正常闭合。至此 DSH 审批的发起、等待、用户决议、结果回写四段全部打通。渲染层也补了证据：`front/src/__tests__/agentApprovalGuard.test.ts` 先前的 3/4 失败来自测试环境缺失（未安装 `createPinia`、未 stub 审批弹窗内渲染的 `ExtensionSlot`），补齐后 4/4 通过；新增 `resolves an approval item that has no turn binding` 用例，断言会话级审批（无 `turnId`）的弹窗可见，点击"本次允许"触发 `ownedChatRuntimeRequest("approval", { conversationId, turnId: "", approvalId, approved: true })`，该文件 5/5 通过，`front` 的 `tsc --noEmit` 通过。 返回的 `executionScope` 稍作补丁后调用 `business/chat-runtime/snapshot`，会返回 `409 禁止将设备专属数据保存到其他设备`；对比 `front/src/runtime/owned-chat-management.ts` 的 `prepareOwnedChatRequest` 可知真实载荷还包含 `conversationOwnerId`、`requestId`（每次新生成）、`targetDeviceId`（取 `expected.targetDeviceId`）以及 `chatScope` 拼出的 `conversationRoleId`，并且 `expectedExecutionScope` 字段名不能写成 `expectedScope`（写错会退化为 `400 聊天运行控制参数无效`）。下一步应复用前端 `prepareOwnedChatRequest` 的构造路径（或在真实前端页面操作）完成决议验证。

13. 启动脚本修复：`start-all-services.sh` 的 `terminate_tree` 原先只发一次 `SIGTERM`，vite 未在父进程退出前收敛，导致连续多次 `Port 5178 is already in use`。改为 TERM 后轮询等待最多 2 秒再 `kill -9`，修复后连续启动均一次成功。

#### 23.14.17 官方 DSH 能力落地矩阵（截至 0.1.135）

| 能力 | 状态 | 权威证据 |
| --- | --- | --- |
| 官方包发现与冻结 ABI 校验 | 已落地 | 30 个包全部 `active`，`@deepseek-ai/dsh@0.2.2-rc.2` 版本断言在 `native-semantics.test.mjs` |
| 官方 `dsh-llm` + Amitia adapter | 已落地 | `registerAdapter(["amitia"])` 注册成功，`llm.stream` 产出 `text-delta` 并回传真实模型文本 |
| 官方 `dsh-agent` / `agent-loop` turn | 已落地 | `turn/start` → `step/start` → `assistant/message` → `step/end` → `turn/end` 全链路，`turnBoundary.openTurnStartSeq = null` |
| 官方 `dsh-tools` 注册 | 已落地 | `agentCtx.tools.register` 注册探针工具成功；`normalizeHostPlugin` 保留 `Config` 后官方默认值生效 |
| 官方 `dsh-tool-executor-native` | 已落地 | `tool/call` 1 + `tool/result` 1，模型基于工具返回值作答 |
| 官方 `dsh-tool-fs-search` | 已落地 | grep 搜出 42645 处匹配，模型回答 `42645` |
| 官方 `dsh-compaction-basic` | 已落地 | `compaction/summary` 两次，`shadowedTokenCount` 699 / 464，`compaction/end` 均 `error: null` |
| 官方 `dsh-spill-policy` | 已落地 | `9ec63dc2d7c4-grep-results.txt` 4231146 字节、52809 行由官方自动写入 |
| 官方 `dsh-session-persistence` / `-projection-cache` | 已落地 | `session.v4.jsonl.zstd` 与 `session_projcache/<id>.json` 落盘，含 `turnBoundary` / `modelSelection` / `inbox` |
| 官方 `dsh-user-approval` | 已落地 | 发起→等待→决议→回写四段闭环，脚本决议返回 `200 {"approved": true}`，模型回答 `allowed-once` |
| 官方 `dsh-commands` + `dsh-command-compact` + `dsh-command-goal` + `dsh-goal` | 已落地 | `diagnose` 新增 `commandLine`，在官方 `agents.create` 的 `setup` 里保存容器上下文与 agent，回合结束后调用官方 `commands.execute(agent, line, [], signal)`。`/compact`：`commandId` `cmd-bcf67dfd-1`、`kind=error`、`Compaction could not produce a useful summary. The attempt is recorded in the session log.`；`/goal`：`cmd-bcf67dfd-2`、`kind=success`、`No goal is currently set.` + 官方用法；`/goal 完成 DSH 兼容层验证`：`cmd-bcf67dfd-3`、`Goal created` / `Status: active` / `Rounds: 0/256`，目标域经 `dsh-goal` 持久化 |
| 官方沙箱链（`dsh-sandbox` / `-local` / `-policy` / `dsh-bash-sandbox` / `dsh-shell-env` / `dsh-tool-bash`） | 已落地 | 六包启用后运行时 37 包 36 `active`（`dsh-shell` 与 `dsh-bash-sandbox` 抢注服务 `shell`，保留后者）；`dsh-tool-bash` 依赖 `ctx.jobs` 的 registry，容器 jobs 门面缺 `kill` 契约导致首发报 `registry.kill is not a function`，补上 `kill(jobId, owner, reason)` 后模型真实调用 `bash` 执行 `echo dsh-sandbox-ok`，`tool/call` 1、`tool/result` 1，回答 `dsh-sandbox-ok`。写操作路径已推进到"审批触发"：补 `wait`/`read`/`remove` 与 `start` 返回 `id` 后，`jobs.read` 改回官方 `{ chunks, lossy, job }` 结构（chunk 带 `data/from/to`），`touch /tmp/dsh-sandbox-write-check` 真实触发沙箱升级审批——事件包含 `approval/asked` 1 与 `approval/decided` 1，工具返回 `sandbox escalation to "danger-full-access" requires approval, but no approval channel is available`。说明沙箱的升级审批请求已进入 DSH 审批服务，但容器注入的 answerer 未能应答该路径（探针路径可应答），后续用脚本决议验证闭环：`approval/asked` 1 + `approval/decided` 1，`chat-runtime/approval` 返回 200，`host.approval.request` 审计 `status=success`，且 `/tmp/dsh-sandbox-write-check` 在 09:20 被真实创建——沙箱升级审批（拒绝 → 升级 → 用户批准 → 放行写入）全链路成立。遗留：工具层输出采集仍报"命令未产生任何输出"。0.1.142 尝试按官方 `output` pull source 契约实现（`start` 接收 `options.output`，`read` 逐 source 调 `read(fromByte)` 累积 chunk），`echo sandbox-output-check` 仍返回空错误 `Error:`——说明工具在 job 已完成分支走的是另一条读取路径（`job.output` 结构或 consuming 语义），`jobs.read` 尚未完全对齐官方 ring 契约。0.1.143 按 `dsh-jobs` 类型定义重写：`kill` 返回 `requested`/`already-finished`，`read` 返回 `{ chunks, lossy, result, next, job }`（settlement 后首次携带 producer 结果），新增 `readAt(id, from, caller)`，chunk 采用 `{ channel, text, from, to, at }`，`start` 接收 `options.output` pull source 并在读取时推进 cursor；`echo` 仍返回空错误 —— 失败点已不在 `read` 契约表面。0.1.144 在 `start`/`read`/source 三处加诊断日志后定位到：`[dsh-jobs] start id=dsh-job-1 kind=bash sources=...` 有输出，但**没有任何 `read` 日志**——工具根本没有调用 `registry.read`。0.1.145 把 `get`/`list` 改为返回官方 `JobView` 投影后仍无 `read` 调用，说明工具因 `view.status` 已不是 `running`/`stopping` 而走了 job 已结算分支，该分支的输出渲染路径（`job.output` 形状）才是下一步要读的代码。0.1.146 给 `jobView` 补 `detail`（原缺失导致失败分支抛 `new Error(undefined)` 的空错误），`echo` 的输出随即恢复：`echo sandbox-output-check`、`echo run-check-1`、`echo run-check-2` 三条命令的 stdout 均原样回给模型。0.1.147 修掉激活顺序缺陷：包激活原本单轮遍历，`dsh-bash-sandbox` 依赖的 `sandboxPolicy` 在其后激活失败后不再重试，导致重启后 `dsh-tool-bash` 报"依赖的服务 shell 不可用"、`bash` 全部失效；改为最多 4 轮重试（每轮尝试所有未激活的 enabled 包，无进展即止），修后 `echo retry-check` 正常返回。写命令（`touch && echo`）仍失败：审批 200、文件被创建，但两次执行输出皆空，escalation 重试路径未走到 `canonicalBashResult`。0.1.148 又定位一处契约错位：`registry.wait(id, timeoutMs, caller?, signal?)` 的返回类型是 `Promise<JobView>`，而容器返回的是 job 的 run 结果（`{done, cancel}`），导致 `view.status` 为 `undefined`、工具误判 job 未结算；改为等待 `job.done` 后返回 `jobView(job)`（并支持 `timeoutMs` 与 `signal.throwIfAborted`）。修后写命令的输出已经能回传——模型回答里出现 `wrote-ok`——但插桩显示 `view.status=completed` 时 `attached.process()` 仍是 `undefined`、`failure=null`，工具因此仍把结果标成失败。插桩显示 `view.status=completed` 时 `attached.process()` 仍为 `undefined`。0.1.149 找到真正原因：官方 `run` 返回的是 handle（`{done, cancel}`），**结算信号在 `handle.done`**，而容器把 `run` 的返回值直接当成 job 完成信号并立即标 `completed`，导致工具在 `proc` 还没赋值时就读 job，永远是 `undefined`。改为：`run` 返回值带 `done.then` 时先 `await value.done` 再置 `completed`，并把 `handle` 记入 job 供 `cancel` 转发。修后写命令闭环：审批 200、`/tmp/dsh-write-ok` 创建、模型回答 `wrote-ok`（无失败措辞）；只读复合命令（`echo ... && ls /tmp | head -1`）输出同样正常 |
| DSH Client 渲染（插件页） | 已落地 | 0.1.158 在真实 Electron 页面（`#/extension/page/dsh-compatible-page`）渲染 42 行插件表，无错误文本与控制台错误；页面内 `停用`/`启用` 点击后宿主状态分别变为 `enabled:false` 与 `enabled:true`/`active:true`（39 包 active）。过程中修复：包级 client 子路径依赖解析、client-source 未登记包回退、`remote.job` 服务、类型声明型客户端入口 |

#### 23.14.19 客户端 UI 边界澄清（0.1.158）

真实 Electron 页面核验得到的边界：

1. DSH 插件页（`#/extension/page/dsh-compatible-page`）是 DSH 客户端模块的宿主面：42 行插件表由 `@deepseek-ai/dsh-client-ui-*` 模块经运行时模块通道加载后渲染，页面内 `停用`/`启用` 会真实回写宿主状态（实测启用后表格 1274ms 内翻转为 `已启用`，宿主 `enabled/active` 同步为 true）。
2. Amitia 原生会话页（`#/chat`）不承载 DSH 客户端模块：会话页暴露的槽位是 Amitia 自己的 `chat.conversation.header`、`chat.composer.permission`、`chat.composer.model` 等，页面内 `[data-contribution-id]` 计数为 0，`extension-error-boundary` 也为 0。也就是说「会话页里的请求批准 / 模型选择 / 权限档位」来自 Amitia 原生组件，不是 DSH 卡片。
3. 因此「DSH 消息与工具结果 UI」要在 DSH 自己的会话面上验证（需要 DSH conversation 页宿主），不能以 Amitia 原生会话页代替。本项目当前未提供该宿主面，属于未完成项，不能用插件页的绿测替代。

#### 23.14.20 应用级客户端运行时通道（0.1.164~0.1.168）

桌面端此前没有"扩展客户端模块随应用启动装载"的通道：`ExtensionSlot` 只渲染 schema 贡献，`client_module` 只在 `ExtensionPageHost`（页面）里加载。移动端有 `mobile_web_runtime`，桌面端缺失，导致 DSH 的 Amitia 槽位贡献（1080 条映射已就绪）在聊天页永远不出现。

本轮补齐：

1. 后端新增 `POST /api/extensions/ui/app-module-ticket`（`backend/internal/extension/kernel/ui_handler/app_module_ticket.go`），校验贡献存在、`sandbox=client_module`、声明 `command` 动作，复用 `pageModuleCapability` 签发 1 小时令牌，返回 `entryPath/entryHash/generation/accessToken`。已用本地令牌实测返回 200 与完整字段。
2. 前端新增 `front/src/composables/useAppClientModules.ts`：从 `/api/extensions/ui/contributions` 筛出 `sandbox=client_module` 且槽位属于 `ui.portal.*` / `app.shell.*` / `provider.app.*` 的贡献，逐个取票据后用 `pageModuleComponent` 生成组件，并在 `AppLayout.vue` 渲染（带 `ui-actions` 注入）。
3. 插件新增 `dsh-desktop-shell-host` 贡献（`ui_page` + dispatch `web_page` + `client_module`，槽位 `ui.portal.overlay`，声明 `command` 动作），入口 `modules/dsh-compatible-ui/desktop/shell.mjs`：挂载时按宿主返回的包清单装载已启用 DSH 客户端模块。

实测证据（Electron + CDP，0.1.168）：会话页控制台出现 `[dsh-shell] DSH 客户端模块已装载 0/0`，证明通道全链路打通（票据 → 模块 import → 组件挂载 → onMounted 执行 → 调用宿主 `command`）。

**0.1.169~0.1.170 续修**：`/api/extension/action/<contributionId>/command` 实测返回 200（此前页面里 404 是缺鉴权头与 baseURL 差异），返回体是 MCP 形态 `{ok,result:{content:[{text:"<json>"}]}}`；`shell.mjs` 增加 `content[0].text` 解析，`useAppClientModules` 按 `ExtensionPageHost` 的 `extensionPageActionResult` 同款解包（`result.structured ?? result`），并在装载后注入 `globalThis.__dshHostCommand`。

**已达成（实证）**：Electron 会话页出现 `[dsh-shell] DSH 客户端模块已装载 4/41`，官方客户端插件把贡献真实渲染进了 Amitia 页面：

- `client:@deepseek-ai/dsh-client-ui-workspace:ui.portal.overlay:workspace.session-rename`
- `client:@deepseek-ai/dsh-client-ui-workspace:ui.portal.overlay:workspace.session-archive`
- `client:@deepseek-ai/dsh-client-ui-workspace:ui.portal.overlay:workspace.row-toast`
- `client:@deepseek-ai/dsh-client-ui-conversation:chat.conversation.root:chat.conversation.root:12`

**当前阻塞（下一步）**：

1. ~~壳层 action 调用缺会话作用域~~ **已解决**：根因是 app 级 action 的 `context` 为空，`invokeHostGuardedAction` 走 `AuthorizeSession(def, "", "")` 只授予 extension/module 作用域，数据面按扩展设备比对报 `ErrWrongOwner`。在 `useAppClientModules.buildActions` 注入 `characterId`（`appStore.currentCharacter.id`）后，页面内实测 `globalThis.__dshHostCommand({action:"conversation-list",limit:3})` 返回真实数据：`{archivedSessionIds:[],hasMore:true,items:[{displayTitle:"新对话",createdAt:"2026-11-16T03:59:24Z",...}]}`，`ErrWrongOwner` 不再出现。
2. 子槽位渲染授权：`client contribution ... is not authorized to render child slot provider.conversation.messages`，DSH `conversation.root` 渲染子槽位时被 `BrowserSlotRuntime.assertRenderAuthority` 拦截。0.1.173/0.1.174 已尝试：前端新增 `authorizeChildSlot(contributionId, childSlotId)`（校验槽位存在、`parentSlotId`/`ownerEntryId` 归属，插入 `childSlotIds`），插件在 `renderMappedSlot` 渲染前调用，但实测无效——页面里 `globalThis.__dshActiveContribution` 的实际值是 `client:@deepseek-ai/dsh-client-ui-workspace:chat.header.action:pin`，全局单值被后渲染的贡献覆盖，授权记到了错误贡献上。正确做法是用 React context 建立渲染栈传递当前贡献 id，而不是全局变量。 0.1.175 已改为按贡献绑定：`DshReactRuntimeBridge` 渲染时用 `componentProps.amitiaContributionId` 包装 `renderSlot` / `renderSlotChain` / `renderFactorySlot`，调用前执行 `authorizeChildRender(contributionId, slotId)`，并移除全局变量方案。0.1.176 进一步兼容「组件直接传 Amitia 槽位名」的情况（`mapDshSlot` 失败时按原名授权，修掉了 `[dsh-slot] chat.header.leading 未在映射目录中`）。但 `provider.conversation.messages` 仍被拒，根因收敛为**目录映射的命名空间不一致**：DSH `conversation.root` 映射到 `chat.conversation.root`，而其子槽位 `conversation.messages` 映射到 `provider.conversation.messages`，两者父级不在同一命名空间，`isAuthorizedChild` 的 `parentSlotId` 判据恒不成立。下一步需要重新生成 `slot-catalog`，让同一 DSH 槽位子树的映射落到同一 Amitia 命名空间。【已解决 0.1.178】根因不在 catalog：`provider.conversation.messages` 的定义确实是 `parentSlotId=provider.conversation.shell`，而贡献在 `chat.conversation.root`，两个命名空间都是会话区域。前端 `clientPluginRuntime` 新增 `sharesSessionSlotArea`（`chat.conversation.*` 与 `provider.conversation.*` 视为同一逻辑区域），并**同时**用于 `authorizeChildSlot` 与 `isAuthorizedChild`——此前只改了前者，`assertRenderAuthority` 仍走严格比较，所以授权成功但渲染依旧被拒（页面探针 `authorizeChildSlot` 返回 true 且 `childSlotIds` 已含该槽位，却仍报未授权，正是这个原因）。修复后页面贡献数 5→7，`provider.conversation.messages` 上的官方 `ConversationContent` 成功渲染。

同轮补齐 `useFactorySlot(slotName, fallback)`（官方 `ConversationContent` 用它取会话视图，缺该能力时报 `useFactorySlot is not a function`），实现为返回 `componentProps.factorySlots[name]` 或 fallback。

**当前唯一剩余错误**：`Cannot read properties of undefined (reading 'items')`（`ConversationContent` 与 `WorkspacePickFlow`）——会话快照未注入，`useSession()` / workspace 快照为空，需要把 `dsh-session-projection` 的客户端投影接到 Amitia 的 `conversation-list` / 消息数据上。

**0.1.179 进展**：在 `DshReactRuntimeBridge` 里为缺失的 `workspaces` / `sessions` / `session` 注入最小占位快照（`{ items: [], ready: false }`）。实测 `ConversationContent` 的 `items` 错误消失（官方会话内容组件不再崩溃），但 `WorkspacePickFlow` 仍报 `reading 'items'`。

`WorkspacePickFlow` 的入参是 `useWorkspaces` hook（不是数据对象），其 `.items` 来自 hook 返回值；bridge 注入的 `componentProps.workspaces` 未被该调用链读到，说明这个组件拿到的 `useWorkspaces` 来自 workspace 包内部的 provider，而不是 bridge 的 props 透传。下一步要在 workspace 的 provider 层提供真实/占位 `workspaces` 快照，或把 `useWorkspaces` 改为读我们注入的 store。

真正接入需要 `dsh-workspace` 的客户端投影：现在只是占位，**不代表 workspace UI 已经跑通**。

**0.1.180 回归与回滚（重要）**：在 hook 内提供默认快照（`useWorkspaces` / `useSession` / `useSessions` 返回 `{ items: [] }`）后，聊天页出现 `Cannot read properties of undefined (reading 'catch')`，且**原生输入框（composer）消失**（`document.querySelectorAll('textarea,[contenteditable=true]')` 从 1 变 0）。回滚到 0.1.179 后该现象仍在，说明不是单版本问题，而是 DSH 客户端装载到应用壳层后影响了原生聊天 UI。

**恢复措施**：在前端 `useAppClientModules` 加开关 `APP_CLIENT_MODULES_ENABLED = false` 并重启完整服务，聊天页恢复（composer=1，显示空态欢迎语，无 `.catch` 报错）。

**结论**：应用级装载 DSH 客户端目前会破坏 Amitia 原生聊天页，属于必须解决的前置问题。下一轮要先定位 `.catch` 的来源（某个 DSH 组件在非 Promise 返回值上调用 `.catch`），并限制装载影响面，之后再重开开关继续验证 DSH 会话 UI。在此之前，`dsh-shell` 的应用级装载视为**实验性、默认关闭**。

**`.catch` 根因（0.1.180 之后定位）**：在 `dsh-client-ui-workspace/lib/client.js` 中，`this.sessions.refreshProjections(named).catch(...)`、`navigation.forkSession(target.id).catch(...)` 等调用依赖 **sessions/navigation store** 的方法返回 Promise；我们的壳层没有对应的 sessions 服务实现，这些方法返回 `undefined`，直接触发 `Cannot read properties of undefined (reading 'catch')`，并连带让 composer 槽位渲染失败、原生输入框被卸载。

下一步要做的是补齐 sessions / navigation 客户端服务（`refreshProjections`、`forkSession` 等返回 Promise 并接到宿主 `conversation-list` / `conversation.read`），而不是继续注入占位快照——占位快照只让组件从"空态分支"进入"操作分支"，反而把缺失的服务暴露成运行时崩溃。

**0.1.181 证伪**：给 `componentProps.sessions` 补 `refreshProjections`（返回 Promise）与 `list.getSnapshot`、给 `componentProps.navigation` 补 `forkSession`，打开开关实测**无效**——composer 仍为 0，`Cannot read properties of undefined (reading 'catch')` 依旧。说明 workspace 组件里的 `this.sessions` 不是 `componentProps.sessions`，而是 workspace 包内部构造的 store，其依赖注入点尚未定位。开关已关回，聊天页恢复（composer=1）。

**当前结论**：应用级装载的前置条件是"找到 workspace 内部 sessions / navigation store 的真实依赖注入点"，继续在 bridge props 上补方法属于盲补，已知两次无效。

**0.1.183~0.1.184 突破（两 agent 并行）**：

1. 崩溃真因修复：`react-adapter` 的挂载句柄 `update/unmount` 原本返回 `undefined`，而调用处 `.catch(...)`，导致 `Cannot read properties of undefined (reading 'catch')` 打断 Vue 卸载链、连带移除 composer。改为 `async` 返回 Promise 后该错误彻底消失；`.catch` 内改用 `rawError()` 输出完整 message+stack+cause。
2. 服务补齐：`sessions.retain/using/scopeOf/sessionOf/searchResultLimit`、`workspaces.initializeDefault/pinSession/unpinSession`、`remoteAccess.source/whenResourceAccess`、`ctx.waterfall`，全部复用既有 sessionCatalog/workspaceCatalog，无第二套实现。定向测试 33/33 通过。
3. 作用域隔离（0.1.184）：DSH 的 `conversation.root` 贡献原先映射到 Amitia 原生 `chat.conversation.root` 并整块替换会话区（渲染空白、composer 消失）。现改为**跳过对原生会话根槽位的覆盖**，只保留 header / messages / portal 等子槽位贡献。

实测（0.1.184，开关开启）：`composer = 1`、贡献 6 个，其中官方 DSH 组件真实渲染：`client:@deepseek-ai/dsh-client-ui-conversation:chat.conversation.header`、`client:@deepseek-ai/dsh-client-ui-workspace:chat.empty.workspace`、以及 workspace 的 `session-rename` / `session-archive` / `row-toast` portal 贡献。原生聊天页 UI 同时正常。截图 `artifacts/dsh-ui-cdp/39-dsh-root-takeover.png` 为替换态，本轮新截图为隔离后的正常态。
3. `WorkspacePickFlow: Cannot read properties of undefined (reading 'items')`：官方 workspace 组件缺上下文数据。
4. ~~`ArchiveSessionRowButton: useSessionAccess is not a function`~~ **已解决**：0.1.171 先在 `componentProps` 注入（无效，已排除 props 顶层路径）；0.1.172 改为在 `renderMappedSlot` 里把 `useSessionAccess` / `useWorkspaceAccess` 合入渲染时传给 Amitia 的 **owner 上下文**（`{...capabilities, ...owner}`），重载后该错误消失。说明官方 workspace 组件的这类能力钩子走的是槽位渲染上下文（owner → 组件 props），不是模块级 props。

**0.1.185 进展**：把 `conversation.root` 的目标槽位从「跳过」改为「重定向到 `ui.portal.overlay`」，让根视图有承载面而不覆盖原生会话区。

实测：`composer = 1`（原生输入框保留），贡献 7 个，其中新增官方会话头部动作 `client:@deepseek-ai/dsh-client-ui-workspace:chat.header.action:archive` 与 `:pin`（归档 / 置顶按钮已挂到 Amitia 会话头部），加上此前的 `chat.empty.workspace`、3 个 workspace portal 贡献与 shell host。截图 `artifacts/dsh-ui-cdp/40-root-in-overlay.png`。

仍缺：`conversation.root` 的 DSH 会话视图本体没有出现在 overlay 贡献列表里（只注册了叶子贡献），完整消息流与输入区尚未可见。【0.1.185 复核修正】root 视图并非未注册——用 `amitiaClientPlugins.slots.contributionsById` 直接查运行时注册表（不经 DOM），DSH 已成功注册大量贡献：

- `dsh-client-ui-conversation`：`chat.message.renderer`、`chat.message.user.text`（×2）、`chat.composer.dock.top`（todo / queue）、`system.settings.section:composer-enter`
- `dsh-client-ui-workspace`：`provider.app.workspace.sidebar`、`desktop.context_menu.content/icon`、`chat.empty.workspace`、`app.workspace.toast`、`chat.header.action`、`ui.portal.overlay`

聊天页 DOM 实际渲染的 DSH 槽位容器包括 `chat.conversation.root`、`provider.conversation.messages`、`chat.composer.root`、`chat.composer.bar`、`chat.header.action`、`chat.empty.workspace`，`[data-contribution-id]` 数量由 7 增至 **14**。

结论：真正缺的不是"注册"，而是**在没有消息的会话里，`chat.message.renderer` 没有内容可渲染**。下一步验证应走"真实发一条消息 → 检查消息是否由 DSH renderer 渲染"，而不是继续折腾 root 槽位映射。截图 `artifacts/dsh-ui-cdp/41-dsh-slots-live.png`。

**消息渲染验证（0.1.185，真实发送）**：在真实会话里通过 UI 输入并发送「你好，这是一条渲染验证消息」，模型正常回复（"你好呀，收到啦～渲染这边看起来是正常的…"），DOM 出现
`client:@deepseek-ai/dsh-client-ui-conversation:chat.message.renderer:chat.message.renderer:0`，
贡献总数 18，composer 保留（`composer = 1`）。这是官方 DSH 消息渲染器首次在 Amitia 真实会话里渲染真实模型回复。截图 `artifacts/dsh-ui-cdp/42-dsh-message-rendered.png`。

**版本取舍记录（重要）**：
- 0.1.184 跳过 `conversation.root`：无消息时可用，有消息后 `composer=0`、renderer 不渲染
- 0.1.185 把 root 重定向到 `ui.portal.overlay`：`composer=1`、contribs=18、消息渲染成功，但遗留两条非致命控制台错误 `list slot ui.portal.overlay requires id`
- 0.1.186 给 `cellId`/`entryKey` 补 key 兜底：更糟（`composer=0`、contribs=12），已回滚
- 0.1.187 回到「跳过 root」：有消息场景 `composer=0`，已回滚

当前保留 **0.1.185**。遗留问题：`ui.portal.overlay` 要求贡献带 id，root 贡献缺 id，导致注册报错；正确修法是为 root 单独声明一个专属 overlay 槽位（而不是复用 workspace 的 overlay），避免 cellId 冲突。

**0.1.188 再证伪**：给 root 传固定 `id`/`entryKey`（`dsh-conversation-root`）并注册到 `ui.portal.overlay`，结果 `composer=0`、contribs=12，仍然报 `list slot ui.portal.overlay requires id`，且新增 `client contribution ...:dsh-conversation-root is not authorized`。说明 root 的真实注册路径不是 562 行的 `renderSlot`，而是 599-615 的 `registerFactory`（用 `factorySurfaceId`/`mapDshSlot`），在 562 行做重定向对 root 无效；对 root 的任何改写都会连带影响原生会话容器，导致 composer 消失。

**固化结论**：0.1.185 是当前唯一同时满足「composer 可用 + 消息渲染 + 18 个贡献」的版本，已回滚到此版本并保持开启。任何 root 侧改动都必须先确认它走的是 `registerFactory` 路径，并单独验证 composer 与消息渲染两项，不能只看「不报错」。

**root 承载面六次尝试汇总（全部实测）**：

| 版本 | 方案 | 结果 |
| --- | --- | --- |
| 0.1.184 | 跳过 root | 无消息可用；有消息 `composer=0` |
| 0.1.185 | root → `ui.portal.overlay`（原 options，无 id） | **可用**：`composer=1`、contribs 18、message renderer 渲染；遗留 `requires id` 错误 |
| 0.1.186 | `cellId`/`entryKey` 补 key 兜底 | `composer=0`、contribs 12 |
| 0.1.187 | 回到跳过 | `composer=0` |
| 0.1.188 | root → overlay + 固定 id `dsh-conversation-root` | `composer=0`、contribs 12、新增 unauthorized |
| 0.1.189 | 声明专属槽位 `ui.dsh.conversation`（kind single）后注册 | `composer=0`、contribs 10 |
| 0.1.190 | 0.1.185 root 行为 + `scopeOf` 按契约返回字符串 | `composer=0`、contribs 12 |
| 0.1.191 | 回调 `scopeOf` 返回对象（与 0.1.185 同代码） | 仍 `composer=0`、contribs 12 |

**关键结论**：0.1.191 与 0.1.185 名义代码相同却表现不同，说明还存在未定位的运行时状态差异（装载竞态或缓存代际），不能把「改回同样的代码」当作回滚成功。因此**发布态锁定 0.1.185 产物**，源码中 0.1.186~0.1.191 的实验改动需要单独整理，暂不再以此链路继续实验。

**竞态根因确认（0.1.192/0.1.193）**：

- 0.1.192（跳过 root 注册 + `cellId/entryKey` 回退 `|| undefined`）：清缓存重载后采样 `composer=1`、contribs=12、**console 无错误**（`requires id` 消失）；同一产物再次采样变成 `composer=0`、contribs=10。
- 0.1.193（`sessions.binding(id)` 补 `session`（含 `getSnapshot/subscribe`）与 `ctx`，修掉 `InputHub.shellFor` 的 `reading 'projections'`）：`composer=0`、contribs=10。
- 结论：**同一产物多次采样结果不一致**，证明应用级装载 DSH 客户端与 Amitia 原生会话页之间存在**非确定性竞态**——异步装载在会话视图挂载期间争抢槽位，导致 composer 被卸载。补服务（binding/session/scopeOf）会让 DSH 更倾向接管会话 UI，反而更容易触发该竞态。

**处置**：应用级装载开关置为 `false`（`APP_CLIENT_MODULES_ENABLED = false`），恢复稳定：`composer=1`、贡献 2（shell host 等）、console 无错误。DSH 客户端仍在插件页正常装载（42 行表格已验证）。

**后续方向（必须做作用域隔离）**：应用级装载要成立，DSH 客户端贡献必须与 Amitia 原生槽位空间隔离（独立 slot 命名空间 + 独立渲染面），而不是共用 `chat.*`。在此之前的任何「补服务 + 改 root 映射」都无法消除竞态。

**作用域隔离的正确落点（本轮发现）**：在 DSH 插件页内（`#/extension/page/dsh-compatible-page`）检查运行时注册表，共 **40 个**客户端贡献，其中包含 `client:@deepseek-ai/dsh-client-ui-conversation:provider.page.provider:conversation`（active），该槽位定义为 `kind=chain`、`supportedKinds=[panel, schema_page, web_page]`、`scope=root`。

也就是说 **DSH 自带一个 conversation 页面 provider**，它才是官方会话 UI 的正当入口。正确做法不是把 `conversation.root` 塞进原生 `chat.*`，而是让 Amitia 前端把 `provider.page.provider` 上的 client 贡献当作可打开的页面渲染。

**Amitia 侧缺口**：前端目前只在服务端贡献（`/api/extensions/ui/contributions`）里列页面，`provider.page.provider` 的 **client 贡献没有被页面入口消费**（`grep provider.page.provider` 只命中类型契约，无渲染点）。因此需要新增：把 client page 贡献接入扩展页面列表与页面宿主，页面渲染时以 `scope=root` 隔离，官方会话 UI 即独立成页，不再与原生聊天页争抢槽位。

**0.1.193 后前端实验（未完成）**：新增独立页面 `front/src/views/extensions/DshConversationView.vue` + 路由 `/dsh-conversation`，页面内用 `<ExtensionSlot slot-id="provider.page.provider">` 承载 DSH 会话 provider。

实测两步结果：

1. 不加过滤：渲染出的是**别的扩展**的页面 provider（工作流页，`extension-workflows-page`），因为 `provider.page.provider` 是所有页面 provider 的公共槽位。
2. 加 `contribution-id="client:@deepseek-ai/dsh-client-ui-conversation:provider.page.provider:conversation"` 过滤后：页面渲染为空（`contribs: ["dsh-desktop-shell-host"]`、`text: ""`），DSH conversation 贡献没有被渲染。

结论：`provider.page.provider` 上的 client 贡献不是"作为列表项渲染"的语义，需要 provider 消费方（页面宿主）提供 `CLIENT_SLOT_RUNTIME_CONTEXT` 与页面级上下文；单纯用 `ExtensionSlot` 指定 contribution 不足以把它渲染出来。该路由目前是空页面（不影响现有功能），后续要么补页面宿主对 client page provider 的消费，要么改由 DSH 页面模块内部渲染。

**最终定位（缺的是公开查询 API）**：`CLIENT_SLOT_RUNTIME_CONTEXT` 由 `ClientSlotContributionHost` 自身 `provide`（`inject(..., null)` 允许为空），**不是**渲染阻塞点。真正缺口是：`BrowserClientPluginRuntime`（`front/src/ui-runtime/clientPluginRuntime.ts:1182`）对外只有 `define*` / `inspect()` 等方法，**没有按槽位查询贡献的公开 API**——`listContributions(slotId)` 属于内部 `SlotRuntime`（同文件 796 行），`inspect()` 只返回 plugins/fibers/services/events，不含槽位贡献。因此页面宿主无法以命令式方式取出 `provider.page.provider` 上的 client 贡献去渲染。

实验页面 `DshConversationView.vue` 与路由 `/dsh-conversation` 已移除，代码库保持干净（`pnpm typecheck` 无相关错误）。

**下一步（唯一可行路径）**：给 `BrowserClientPluginRuntime` 增加公开的 `listSlotContributions(slotId): ClientSlotContribution[]`（转发内部 SlotRuntime），并让它参与响应式通知；随后页面宿主即可消费 `provider.page.provider` 的 client 贡献，官方会话 UI 才能真正独立成页。

**2026-11-17 源码核对与实施状态**：当时插件 manifest 版本为 `0.1.220`，README 所写 `0.1.193` 已过时。`BrowserClientPluginRuntime.slots` 已公开，`BrowserSlotRuntime.listContributions` 与 `revision` 已可用于响应式查询，无需新增查询 API。现存实验页 `/dsh-conversation` 已改为精确选择 `client:@deepseek-ai/dsh-client-ui-conversation:provider.page.provider:conversation`，不再渲染 `chat.conversation.root`，并移除每秒轮询。官方 `dsh-client-ui-conversation` 将 root 作用域 `main` 贡献注册为 `key=conversation`，其中 `main.conversation` 为 `session-maybe`；页面因此允许在尚无会话时显示官方导航容器。插件管理页现提供“打开官方会话页面”入口。新增页面宿主定向测试覆盖精确贡献选择、注册表变化、撤销卸载与无会话入口，2/2 通过；相关 Slot 测试另有 5/5 通过，插件 `client-ui.test.mjs` 4/4 通过。全局 typecheck 仍被 `PetInstallationsView.vue` 中与本次改动无关的缺失字段和函数阻断。当前前端服务未运行，独立页面的真实挂载、会话切换以及原生聊天输入框稳定性尚未完成页面验收。

**2026-11-17 运行验证补充**：`start-all-services.sh` 的 `ensure_web_port_free` 在端口无人监听时受 `set -e` 中断，已修正该空端口分支并通过 `bash -n`。按项目脚本前台模式启动后，前端 5178、Core 健康接口和 readyz 均返回 200；运行时 6 ready、1 个可选浏览器组件 degraded、0 blocking、0 failed。Electron 原生聊天页面可见输入框。独立 DSH 页面尚未完成真实挂载验证：普通浏览器被 Web 访问密码页拦截，当前插件源码新增入口也尚未重新打包安装。

**2026-11-17 桌面页面续测**：`0.1.221` 已由当前源码打包并安装到本地 Core，扩展状态 `installed`、`enabled`，管理页显示当前已启用的 DSH 子插件。Electron 中点击管理页“打开官方会话页面”后，路由进入 `/dsh-conversation`，但页面没有会话内容。定位到 `useAppClientModules` 默认只筛选应用外壳前缀，未加载位于 `ui.portal.overlay` 的 `dsh-desktop-shell-host`。`0.1.222` 源码改为该独立页精确装载 `dsh-desktop-shell-host`，Shell 注入导航、动作和会话上下文，避免影响默认关闭的应用级加载。当前仍需对 `0.1.222` 完成打包安装、真实挂载和会话切换验收。

**2026-11-17 React 挂载续测**：`0.1.222` 已安装，独立页开始装载 DSH 客户端模块（4/41 个有客户端入口），Electron 控制台暴露 `React.useRef` / `React.useSyncExternalStore` 的 null dispatcher。原因是管理页 `index.mjs` 和 Shell `shell.mjs` 分别打包 React，但共用同一个 ForeignComponent 适配器 ID，导致组件使用 Shell React、渲染器却来自管理页 React。`0.1.223` 为每个模块实例分配独立适配器 ID，插件 `client-ui.test.mjs` 4/4 通过，打包 treeHash `809073ce1357bdb3c579feb342b15cd32644a908b13331cf8fa0aa758cb156de`，本地更新操作完成，状态 `installed`、`enabled`、`failureCount: 0`。Electron 当前窗口仍保留旧 generation 424 的模块与历史错误；Core 中新贡献 generation 为 428。新 generation 的页面渲染、会话切换与原生聊天回归尚未完成真实验收。

**2026-11-17 官方页面阶段验收**：`0.1.224` 解决 React 实例冲突及子槽位渲染授权，随后 Electron 暴露官方 `InputBar` 对 `session.pendingSubmissions.length` 的读取错误。`0.1.225` 补齐会话快照后，下一处错误为官方 `resolveDraftAttachments(input.attachmentIds)` 的 `ids is not iterable`；`0.1.226` 按官方 `InputState.compose()` 补齐 `attachmentIds`、`draftRev`、`phase` 和 `queue`。`0.1.226` 插件包 treeHash 为 `725b92bfa4014084c23511782909071e3fee06982a0f42422de8fa0b784fcda2`，本地 Core 更新完成，状态 `installed`、`enabled`、`failureCount: 0`。完整项目重启后，前端 5178 与 Core 健康接口可访问；Electron `#/dsh-conversation` 实际显示官方欢迎区、工作区选择器和禁用态输入栏，客户端模块装载 4/41，前述 React、授权、会话数组与附件数组异常均未再出现。无会话入口仍记录 `sessions.sessionOf` 无可解析 Session 作用域告警；带真实会话作用域的消息流、发送、会话切换及原生聊天回归尚未完成。插件客户端定向测试 4/4、独立页定向测试 2/2 通过。

**2026-11-17 会话导航修正**：官方 `uiWorkspace.openSession/startSession` 原先跳到 Amitia 原生 `/chat`，无法继续使用 DSH 自有会话页。`0.1.227` 改为 `/dsh-conversation?conversationId=<编码 ID>`，并更新运行时服务测试，25/25 通过；插件客户端测试 4/4 通过。插件包 treeHash `250a5cdfbcd2a66a3c0336997a543aee165132327a035fb79e11578a1173e996`，本地更新完成，`installed`、`enabled`、`failureCount: 0`。完整项目重启后 Core 与前端接口可访问。真实会话选择、消息区投影和发送尚待下一轮 Electron 验收。

**里程碑状态（0.1.184，221/221 测试通过，health ready / runtime ready）**：应用级装载开启后，官方 DSH 客户端组件与 Amitia 原生 UI 可共存。

**剩余工作（按依赖排序）**：

1. **DSH 自有会话页承载 `conversation.root`**：现在为保护原生 UI 跳过该槽位，导致 DSH 完整会话视图（消息流 + 输入区）不可见。做法是把 root 贡献映射到扩展自有页面（新增一个 `ui_page` + `dsh.conversation` 私有槽位命名空间），页面内用现有 `renderMappedSlot` 渲染，并在页面里提供会话切换入口。
2. **`workspaces.pinSession` / `unpinSession`**：Amitia 只有 `conversation-list` 的 `pinnedSessionIds` 读取端，缺写入通道；需在 Core 增加会话置顶写入接口（或复用现有会话更新接口）后再接。
3. **`sessions.scopeOf` 签名对齐**：当前返回 `{ sessionId }`，官方若期望 `AgentContext` 需再核对一次。
4. **`chat.empty.workspace` 交互验证**：空态工作区组件已渲染，但"新建工作区"路径会触发 `workspaces.create`（真实建项目），需确认该副作用在产品上是否允许。

结论：截至本记录，官方 DSH 的执行面（模型、回合、工具、压缩、spill、会话持久化、审批、沙箱、斜杠命令）已在 Amitia 内真实跑通并有可复核证据；渲染面仍未在 Electron 页面确认，因此不宣称"完全兼容"。

#### 23.14.18 共享依赖树与斜杠命令闭环（0.1.151~0.1.152）

官方组合依赖模块单例身份：`/compact` 的预期失败映射用 `error instanceof ManualCompactionError` 判定，而 `dsh-command-compact` 与 `dsh-compaction-basic` 若各自解析到不同副本的同版本 `@deepseek-ai/dsh-compaction`，该判定恒为 false，官方错误被当成未知异常直接抛出（实测表现为工具调用失败并原样回传 `manual compaction could not produce a smaller summary`）。根因是容器按「每个包一套 npm 工作区」安装，同一依赖在每棵树里各有一份。

0.1.151 改为**一个共享依赖树**：`<plugin_data>/dsh-tree/package.json` 合并全部依赖后统一 `npm install`，包目录统一为 `dsh-tree/node_modules/<name>`，卸载只删该包目录与依赖声明，失败时回滚 `package.json`。收益：模块单例身份成立（`instanceof`、Brand 标记）、安装体积从 881MB 降到 143MB、安装不再复制同一依赖 39 次。同轮修复两处同源缺陷：`jobView` 二次包装丢失 `owner`（`jobs-list` 的 `owner` 恒为空）、`pluginDataRootFromPackageDir` 仍按旧目录名识别数据根（导致 `dsh-storage-json` 等 provider 拿不到 `root`）。

0.1.152 修复启用意图丢失：`set-enabled` 原先「激活成功后才写 `enabled`」，激活失败时意图被丢弃，依赖提供者后到也无法靠重试收敛（实测 39 个包需要连续三轮 `set-enabled` 才全部激活）。改为先持久化 `enabled: true` 再激活，失败保留 `enabled: true` + `activationError` 并立即触发一次重试。实测禁用 `dsh-tools`（依赖 `systemPrompt`）与 `dsh-system-prompt` 后单轮启用二者：`dsh-tools` 首次失败、`systemPrompt` 就位后自动收敛，39 包全部 `active`。

斜杠命令证据（0.1.152 运行实例，`diagnose` 报告的 `command.execution`）：

1. `/compact`：`{"commandId":"cmd-bcf67dfd-1","result":{"kind":"error","text":"Compaction could not produce a useful summary. The attempt is recorded in the session log."}}`——官方预期失败被正确映射为 `CommandResult`，不再抛内部异常。
2. `/goal`：`{"commandId":"cmd-bcf67dfd-2","result":{"kind":"success","text":"No goal is currently set.\nUsage: /goal [<objective>|clear|edit <objective>|pause|resume]"}}`。
3. `/goal 完成 DSH 兼容层验证`：`{"commandId":"cmd-bcf67dfd-3","result":{"kind":"success","text":"Goal created\nStatus: active\nObjective: 完成 DSH 兼容层验证\nRounds: 0/256\nActivation: armed\n\nCommands: /goal edit <objective>, /goal pause, /goal clear"}}`。

已知限制：`diagnose` 复用 `conversationId` 时会命中官方 `agents.create` 的「session 已存在」，因此同一会话内的 `/goal` 回读还没有真实用例；`/compact` 在极短会话上必然返回官方 summary 失败，成功压缩需要更长的历史。

## 附录 A：Android/iOS 同能力交付方案

### A.1 交付目标

手机端不是桌面端的降级版本，也不是第二套 DSH 兼容插件。Android、iOS 和桌面端共用：

1. 同一个插件 ID `com.amitia/dsh-compatible-host`。
2. 同一个 `.amitiax` Manifest 和版本号。
3. 同一个 DSH 目标 ABI `0.2.2-rc.2`。
4. 同一套 DSH Host ABI、Client ABI、Service、Slot 和生命周期映射。
5. 同一个容器私有 `plugin_data/dsh-packages` 数据结构。
6. 同一套兼容矩阵和原始错误语义。

### A.2 移动端运行架构

```text
Flutter Amitia
├─ 通用插件安装与 UISnapshot
├─ 通用 Dynamic Slot / Surface
├─ WebView Provider Host
└─ 普通 Amitiax 容器插件
   ├─ DSH 管理页面（WebView）
   ├─ DSH 子插件安装命令
   └─ DSH 页面 Provider

Android Runtime Package / iOS Runtime
├─ Node
├─ AmitiaCore
├─ plugin-host
├─ task-host
└─ DSH 容器 Runtime
   ├─ Host ABI Shim
   ├─ 包管理器
   ├─ plugin_data/dsh-packages
   └─ Tool / Prompt / Session / Agent / MCP 映射

WebView DSH Client Runtime
├─ ModuleLoader
├─ React / ReactDOM
├─ DSH Client ABI
├─ Slot/HOC/Portal 渲染
└─ AmitiaNativeBridge
```

移动端 DSH UI 不直接注册 Flutter Widget。容器为每个可扩展 Surface 提供一个受控 Web Provider，WebView 内运行 DSH Client ABI，并把当前 Surface 的 Slot Context 注入 DSH React 组件。Flutter 负责宿主区域、尺寸、生命周期、权限和原生交互。

### A.3 移动端核心需求

| 编号 | 需求 |
| --- | --- |
| MOB-01 | Android 和 iOS 均能安装普通 `com.amitia/dsh-compatible-host` Amitiax 包。 |
| MOB-02 | 手机端容器页面能够安装、启用、停用、更新和卸载 DSH 子插件。 |
| MOB-03 | DSH 子插件只进入容器 `plugin_data/dsh-packages`，不注册为独立 Amitia 扩展。 |
| MOB-04 | 手机 Runtime 中的 DSH Host ABI 与桌面使用同一实现和同一 ABI 版本。 |
| MOB-05 | DSH Client ABI 在 WebView Runtime 中执行，不在 Flutter AOT 中动态加载任意代码。 |
| MOB-06 | DSH 页面、侧栏、消息、输入区、设置、工具结果、Portal 和右侧资源面板映射到 Flutter 通用 Slot/Provider Surface。 |
| MOB-07 | HOC、children、store、inject、locale、resource provider、Portal 和 UI Context 在手机端具备与桌面一致的语义。 |
| MOB-08 | WebView 与 Flutter/Node 之间的所有 Host 调用经过 Amitia Native Bridge、权限和生命周期校验。 |
| MOB-09 | 应用进入后台、Runtime 重启、WebView 重建和插件恢复后能够恢复 DSH Session、模型选择和 UI 状态。 |
| MOB-10 | 手机端无法执行的能力必须返回完整原始错误；不得因为平台是 mobile 而静默隐藏或假成功。 |

### A.4 移动端通用 Amitia 能力补齐

开始 DSH 移动 UI 映射前，Amitia 手机端必须先补齐：

1. Dynamic Slot 的 `single/list/keyed/chain/hoc` 完整 dispatch。
2. Mobile UI Context：route、surface、session、conversation、message、character、permission、capability、actions 和 navigation。
3. Mobile Web Slot Host：按 Slot、session 和 surface 创建独立 WebView Runtime。
4. Mobile Foreign Component Surface：WebView 内的 DSH React DOM 生命周期与 Flutter Widget 生命周期同步。
5. Mobile Portal Host：overlay、modal、drawer、toast、menu、tool result 和 right pane。
6. 插件自声明 Slot、跨插件 Slot 和递归 owned cleanup。
7. Surface Catalog 平台字段支持 android、ios，并驱动真实 Provider 选择。
8. WebView 重建、前后台切换、插件停用、会话切换和卸载时无残留。

### A.5 移动端 DSH UI 桥接

WebView 启动参数至少包含：

```text
pluginId
containerVersion
targetDshVersion
slotId
surfaceId
sessionId
conversationId
characterId
messageId
locale
platform
host
theme
permissions
capabilities
ownerProps
```

WebView 输出能力至少包含：

1. `slot.register`：DSH Client 在 WebView 内注册 Slot 组件。
2. `slot.render`：根据 Flutter Surface 参数渲染目标 Slot。
3. `slot.resize`：主动向 Flutter 报告内容尺寸。
4. `action.invoke`：调用 Host 声明并授权的通用 Action。
5. `navigation.request`：请求宿主导航。
6. `portal.request`：请求宿主打开 overlay/modal/drawer/toast。
7. `resource.open`：打开右侧资源面板。
8. `error.report`：上报完整原始错误和组件边界。

Flutter 只接受声明式消息，不允许 WebView 直接操纵宿主 DOM、导航栈或未授权 Native API。

### A.6 移动端分阶段实施

#### MOB-P0：平台与 ABI 基线

工作：

1. 在平台矩阵中加入 Android 和 iOS。
2. 检查 Runtime Package 中的 Node、plugin-host、task-host 与本地 DSH 包安装能力。
3. 明确 Android/iOS 文件系统、网络、进程和后台限制。
4. 为移动端生成独立 Surface Catalog 和 Slot 支持矩阵。

门禁：

1. 手机端 Runtime 能运行同一容器 Host ABI。
2. `plugin_data/dsh-packages` 可读写和执行。
3. 平台不支持项有明确原始错误。

#### MOB-P1：移动端 Dynamic Slot 全语义

当前实现状态：

1. 已完成 `single/list/keyed/chain/hoc` 调度、严格优先级冲突和动态 `entryKey` 选择。
2. 已完成动态 Slot 子声明、跨插件贡献组合、父包移除后的声明失效和递归渲染入口。
3. 已完成应用导航、聊天消息区、消息渲染器、记忆、任务、Portal 和右侧面板等通用 Surface 挂载。
4. 已完成 Android/iOS/mobile Slot 宿主类型和移动 Surface Catalog。
5. 待完成 store、inject、hooks、完整 children 跨 WebView 组合、每个页面的真机行为验收。

工作：

1. Flutter 实现 `single/list/keyed/chain/hoc` 与桌面一致的排序、冲突和卸载。
2. 实现 session scope、store、inject、hooks 和 children。
3. 实现动态 Slot 和插件自有 Slot。
4. 实现 Portal 和 Surface Catalog。

门禁：

1. 手机端与桌面 Slot 契约测试一致。
2. HOC、children、Portal 和跨插件清理通过。
3. 插件停用和会话切换无残留。

#### MOB-P2：移动端 Web Slot Runtime

当前实现状态：

1. 已完成通用 `mobile_web_runtime` Provider、内容哈希、短时模块票据、样式路径和 WebView 生命周期。
2. 已完成普通 Slot 的独立 `contribution-module` 资源通道，不要求把 Slot 伪装成 Provider。
3. 已完成 `AmitiaHost` 的 Action、导航、Toast、右侧面板、尺寸、上下文订阅和原始错误 Bridge。
4. 已实现 DSH 手机容器管理页并通过 jsdom 定向测试。
5. 已实现 DSH Client 插件在手机 WebView 内的加载、React Slot Registry、children、store、inject、locale、resource 和 Jobs 运行。
6. 已完成全部 DSH Slot 别名的手机通用 Slot 映射，共声明 22 个手机宿主贡献，包括工作区侧栏。
7. 待完成跨 WebView 共享状态、Android/iOS React fixture 真机和恢复验收。

工作：

1. Flutter 新增通用 Web Module/Slot WebView Host。
2. 每个 Slot 可独立加载受控 HTML/JS Runtime。
3. 支持 ModuleLoader、React、ReactDOM、CSS、locale 和 resource provider。
4. 实现 NativeBridge 的 action、navigation、dialog、resize、error 和 lifecycle 协议。
5. 实现多个 WebView 共享同一 Session/Container 状态。

门禁：

1. DSH React fixture 在 Android/iOS WebView 中真实挂载、更新和卸载。
2. WebView 重建后 DSH store、Session projection 和 UI 状态可恢复。
3. 非目标 origin、未授权方法和跨 session 调用失败关闭。

#### MOB-P3：容器管理页面

当前实现状态：

1. 已完成手机状态、刷新、安装、启停和移除。
2. 已完成独立 Android、iOS、mobile Provider 入口与统一路由。
3. 待完成更新、回滚、本地文件选择、后台任务视图和真机生命周期验收。

工作：

1. 为手机端提供独立 `web_page` 管理入口。
2. 页面支持 DSH npm、Git、tarball 和本地包安装。
3. 列表展示 Host/Client/UI 映射、兼容状态和原始错误。
4. 支持启停、更新、回滚、移除和重置。
5. 页面与 Runtime Host 通过受控 Bridge 通信。

门禁：

1. 用户可在手机端完成完整 DSH 子插件生命周期。
2. DSH 子插件不出现在 Amitia 扩展中心。
3. 管理页关闭、App 后台或 Runtime 重启后状态一致。

#### MOB-P4：移动端 Host 映射验收

必须逐项验证：

1. Tool / Prompt / Event / Hook。
2. Session / Agent / Agent cancel / whenIdle / model selection / maintenance。
3. Subagent 和 provider。
4. MCP tools、resources、prompts、sampling、elicitation。
5. Jobs 列表、观察和取消。
6. Session 恢复和 projection。

门禁：

1. 同一 DSH Host 夹具在桌面和手机得到一致语义结果。
2. 移动平台差异返回完整原因，不产生假成功。

#### MOB-P5：移动端 DSH UI 全量映射

工作：

1. 页面 Provider。
2. 聊天头和状态项。
3. 侧栏和右侧资源面板。
4. 消息节点、消息操作、自定义渲染器和工具结果。
5. Composer action、hint 和 attachment。
6. 设置、扩展页、菜单、Modal、Drawer、Overlay 和 Toast。
7. HOC、children、locale、store、inject 和 resource provider。

门禁：

1. 每个目标 DSH Slot 在手机 Surface Catalog 中有唯一映射或明确平台错误。
2. 手机尺寸、横竖屏、键盘、安全区和后台恢复通过。
3. WebView 不覆盖宿主原生安全确认和关键导航。
4. Slot 更新不导致键盘、滚动位置、焦点或会话状态丢失。

#### MOB-P6：生命周期、离线与恢复

工作：

1. App 前后台切换。
2. Runtime 重启与升级。
3. WebView 进程崩溃恢复。
4. DSH 子插件安装事务恢复。
5. Session、Agent、Job、Portal 和 React Root 清理。

门禁：

1. 任意单点崩溃不影响 Amitia 主界面。
2. 恢复后不重复注册 Tool、Slot 和服务。
3. 卸载后不遗留 WebView、缓存、Session、任务或子插件目录。

#### MOB-P7：移动端发布验收

1. 从当前后端源码重新构建 Runtime Package。
2. 使用同一源码构建 Android/iOS 所需核心和 Node Runtime。
3. 禁止使用旧 Runtime Package 或旧后端产物。
4. 在干净首次安装环境验证。
5. 安装真实官方 DSH Host、Client UI 和 MCP 包。
6. 验证完整生命周期和兼容矩阵。

### A.7 移动端验收矩阵

| 验收 ID | 场景 | 必须证明 |
| --- | --- | --- |
| MOB-AC-01 | 安装普通容器 | 手机扩展中心只出现一个 Amitiax 容器 |
| MOB-AC-02 | 安装 DSH 子插件 | 手机容器可安装并只写入插件私有目录 |
| MOB-AC-03 | DSH Host Tool | 手机 Runtime 能执行并返回真实结果 |
| MOB-AC-04 | DSH Session/Agent | 创建、恢复、取消、idle、模型选择和 maintenance 正确 |
| MOB-AC-05 | DSH React UI | WebView 中真实挂载、更新和销毁组件 |
| MOB-AC-06 | Slot/HOC | 页面、侧栏、消息、输入区和 HOC children 正确 |
| MOB-AC-07 | Portal/Right Pane | overlay、modal、drawer、toast 和资源面板正确 |
| MOB-AC-08 | Resource Provider | 手机 WebView 能读取资源并更新 |
| MOB-AC-09 | Jobs | 手机端能观察和取消后台任务 |
| MOB-AC-10 | MCP | 工具、资源和高级请求通过或返回明确失败 |
| MOB-AC-11 | 前后台/重启 | 恢复后状态一致且无重复注册 |
| MOB-AC-12 | 停用/卸载 | 无 WebView、Slot、任务、目录和缓存残留 |
| MOB-AC-13 | 平台差异 | Manifest 平台声明与实际能力一致 |
| MOB-AC-14 | 原始错误 | Flutter、WebView、Node 三层错误链完整 |

### A.8 完成口径

只有 Android、iOS、Windows、macOS 和 Linux 全部满足各自声明的能力矩阵，才能把容器插件标记为完成。

手机端达到“能用”的最低门槛不能替代最终目标：

1. 仅能打开管理页面不等于兼容。
2. 仅能安装 DSH 包不等于兼容。
3. 仅能渲染单个 React 组件不等于 UI 兼容。
4. 手机端必须与桌面端使用同一目标 ABI、同一兼容矩阵和同一清理标准。

### 23.20 P13 推进记录（0.1.239，未完成会话消息渲染）

本轮把 `/dsh-conversation` 从「只有 header + composer」推进到「DSH 原生会话骨架完整挂载、viewArea 已建立」，消息流仍未渲染，逐项证据如下。

已落地的桥接能力：

1. `src/client-ui/session-events.mjs`：新增 `projectSessionEvents` 与 `createSessionEventFeed`，把 `host.conversation.read` 的 `{messages:[{conversationId,role,timestamp,contentSummary}],hasMore}` 投影为 DSH `SessionEventWindow`（`turn/start`、`step/start`、`user/message`、`assistant/message`、`step/end`、`turn/end`，`surfaceOp: 'append'`，`revision` 单调递增，变更时 `change.kind = 'replace'`）。
2. `src/client-abi/runtime-services.mjs`：`sessions.binding(sessionId).eventSource` 由空实现改为真实 `SessionEventFeed`；`session` 增加 `loadOlder`/`consumeOlder`/`loadThrough`（原始失败透传）、`beginSubmission`、`prompt`（`conversation-message-send` 真实投递，成功后刷新 feed，`RemoteResult` 结构返回）。这是 DSH `ConversationAssembly.binding()` → `new BoundConversation(binding.eventSource, assembler)` 的唯一合法入口，桥接自建的会话快照不再参与官方渲染路径。
3. `src/client-ui/index.mjs`：
   - 删除重复的 `createUiSessionService`，改为模块级共享 `createSessionSourceRegistry()`（`provide`/`binding`/`resolve`/`registerBinding`/`refresh`），解决每模块各建一份 `uiSession` 导致 chat 的 `hooks:['chat']` 贡献丢失的问题。
   - `registerBinding` 注入各模块真实 `runtimeServices.sessions.binding`，让 DSH 的 `resolve(binding)` 收到带 `ctx`/`sessionId`/`eventSource` 的 `SessionBinding`（此前传 `{sessionId}` 直接抛 `Cannot read properties of undefined (reading 'effect')`）。
   - 贡献 `inject` 按 DSH ABI 适配：调用 `declared(sessionId, actions)`，不再把 Amitia 的 `injectContext` 对象塞进 `sessionId`。
   - `hooks` 与 `keyedHooks` 两个 compartment 都合成 `use<Name>` 选择器 Hook；会话级标准 Hook 优先于兜底。
   - `useStore` 按 DSH ABI 由桥接自建的 DSH store 实例驱动（`defineStore` handle `create(scopeKey)` → `getSnapshot`/`subscribe`/`actions`），不再沿用 Amitia 返回 ComputedRef 的 `useStore`。
   - `uiConversation.interactions`、`binding()` 指向 `conversation-data.mjs` 的会话投影；`slots.subscribe(slotId, listener)` 由空实现改为注册/卸载真实通知（这是视图台账 `conversationViews` 能刷新的前提）。
4. `src/runtime/index.mjs` + `amitia-extension.json`：新增通用 `conversation-read`、`conversation-message-send` 两个 action 与 `content` 输入属性。

插件 `0.1.239` 已安装（`generation-...`，`version: 0.1.239`）。CDP 实测（`artifacts/dsh-acceptance-20261117/step60-probe-flow.mjs`、`step61-console.mjs`）当前状态：

- `.dsh-conversation-root` 从 0 高度恢复为 50px，`wSkVaW_root` 已挂载，header 与 composer 正常。
- `wSkVaW_viewArea` 已建立但 `innerHTML.length === 0`，`[data-chat-flow]` 不存在，`[data-chat-node]` 数量为 0。
- 剩余阻塞是 `InputBar`（`Cannot read properties of undefined (reading 'length')`）与 `QueueDock`（`Cannot read properties of undefined (reading 'filter')`）在标准 `input` Hook 尚未就绪时对 `undefined` 取字段，异常中断了审阅组件的提交，导致 `conversation.view` 台账仍为空。

下一步（按依赖顺序）：补齐 `input` 标准 Hook 的初始快照形态（`queue`、`attachments`、`occurrences` 等字段在 `shell.state` 未就绪时不能是 `undefined`）→ 确认 `conversationViews` 非空后 `DefaultConversationViews` 会以 `active.id === 'chat'` 渲染 `conversation.view` → 再验证 `ChatView`/`ChatFlow`/`ChatNodeSeat` 消费 `SessionEvent` 投影出的消息节点 → 最后验证 `session.prompt` 的真实模型往返与原生聊天页回归。

#### 23.20 续：0.1.242 / 0.1.244（视图台账打通、宿主新增通用会话作用域能力）

1. 视图不渲染的真正收口点是**跨模块槽位通知**：`createSlotBridge` 原先把 `slotListeners` 放在每个模块自己的闭包里，`dsh-client-ui-conversation` 注册的 `slots.subscribe('conversation.view', refreshViews)` 收不到 `dsh-client-ui-chat` 的视图注册事件，`conversationViews` 永远为空。把监听表提升为模块级 `dshSlotListeners` 后，`0.1.242` 实测 `wSkVaW_viewArea` 的 `innerHTML` 由 0 变为 2085，`[data-chat-flow]` 出现，ChatView/ChatFlow/ChatNodeSeat 链路已挂载。
2. `0.1.241` 修掉 `useSession` 兜底把字符串 owner 当成快照对象传入选择器（`s.pendingSubmissions.length` 抛 `Cannot read properties of undefined (reading 'length')`），并让会话兜底快照始终合并 `DSH_EMPTY_SESSION_SNAPSHOT`。
3. `0.1.244` 把桥接内的 `useSyncExternalStore` 调用全部下沉到选择器 Hook 体内，恢复 `client-ui.test.mjs`（4/4）与 `client-abi.test.mjs`（8/8）通过；桥接函数在无 React 渲染器时不再触发 Hook。
4. 消息节点仍为空，根因已定位到**数据面权限**而非渲染：`host.conversation.read` 需要 `conversation` 角色作用域，`dsh-session:<id>` 状态里没有 `scopeSnapshotId` 时只能走普通 `host.call`，被内核拒绝（原始错误 `scope_denied: required roles [conversation] not satisfied`）。内核此前只有 `host.conversation.create` 会铸造会话目标快照，缺少针对**已存在会话**的通用能力。
5. 为此在内核新增通用方法 `host.conversation.scope`（`internal/extension/kernel/host_api/types.go`、`permission_mapping.go` 的权限/作用域/数据路由/副作用四张表、`capability_matrix.go`、`host_api_routes.go` 路由），入参 `{conversationId}`，复用既有 `createConversationTargetSnapshot` 生成带 `conversation` 角色的会话目标快照，返回 `{conversationId, scopeSnapshotId}`；权限沿用 `conversation.read`，风险 `RiskLow`，副作用 `ReadOnly`，不新增第二套作用域体系。`go build ./cmd/server` 与 `go vet ./internal/extension/kernel/...` 通过。
6. 插件侧新增 `conversation-scope` action，`conversationScopeSnapshotId` 在持久化状态缺失时回退调用该方法，取不到快照时原样抛出带完整返回体的错误。安装包 `plugin/amitia-dsh-compatible-host-0.1.244.amitiax`。

当前运行中的 Core 仍是旧二进制，实测 `host.conversation.scope` 返回 `method_not_found: method host.conversation.scope v1 not found`（原始错误已透传）。下一步必须先按项目规则重建并重启完整服务（前端 + Core），再复测 `host.conversation.read` 是否返回真实消息、`ChatFlow` 是否出现消息节点，最后验证 `session.prompt` 的真实模型往返。

#### 23.20 续二：0.1.246–0.1.252（数据面打通、会话事件契约对齐、定位消息节点空渲染）

1. 内核 `host.conversation.scope` 已随完整服务重启生效：重建 darwin/arm64 `AmitiaCore`（SHA-256 `a13f5d4016ea94af7a81b307184ff8b53b8a194292621eca9122f9c0fd93e324`），flat 与 `macos-arm64/` 两份一致，`./start-all-services.sh` 重启后 Core 监听 64310、`/api/health` 正常。重启后 `conversation-read` 从 `scope_denied` 变为成功：`15321b8f-…` 返回 2 条、`536b868e-…` 返回 2 条真实消息。
2. 事件窗口结构修正：`ConversationNodeAssembler.replaceWindow` 接受的是 `SessionEventLikeEntry`（`{type:'event', event}`）而不是裸 `SessionEvent`，0.1.246 起按此结构投影；`turn/start`、`step/start`、`user/message`、`assistant/message`、`step/end`、`turn/end` 全部带 `seq`/`time`/`data`，消息类事件带 `surfaceOp:'append'`。
3. 会话 id 兜底：嵌套槽位（`conversation.view` 内的 ChatView、`conversation.chat.flow`）拿不到 `sessionId` 属性，桥接改为回退 `globalThis.__dshCurrentSessionId`，会话快照由此可解析。
4. 当前实测（0.1.252，`536b868e…`）：`wSkVaW_root[data-phase=active]`、`wSkVaW_viewArea` 存在（2085 字节）、ChatView 的 `frame/root/scroll/column` 与 `[data-chat-flow]` 全部挂载、`conversation.chat.flow` 贡献已渲染；桥接侧实测进入 flow 槽位的 `entries` 为 2，会话 `chat` 快照 `order` 为 `["user:1","assistant-step:0"]`。但 flow 的 React 根仍为空，`[data-chat-node]` 为 0。
5. 剩余缺口定位：`ChatFlow` 用 `entries` 正常，但 `ChatNodeSeat` 走的是 `renderSlot('conversation.chat.node', …, { entryKey: kind })` 的 **keyed** 派发；宿主把 `chat.conversation.node` 预声明为 **list** 槽位（启动日志：`child slot chat.conversation.node is predeclared by the host: dsh=keyed/session host=list/session`），keyed 的 `entryKey` 在 list 槽位上无法命中，节点渲染返回空。同类告警还出现在 `chat.message.badge`（dsh=chain / host=list）。
6. 该项属于宿主槽位契约（`front/src/ui-runtime/slotContract.ts` 的 `chat.conversation.node` 种类）与原生聊天渲染共用，改动会影响现有原生会话渲染，必须先给出方案再确认，不在本轮擅自修改。
7. 已移除本轮所有临时探针（`__dshDebugSessionSources`、flow-debug 日志、`__dshSessionEventDebug`），`client-ui.test.mjs` 4/4、`client-abi.test.mjs` 8/8 通过，安装包 `plugin/amitia-dsh-compatible-host-0.1.252.amitiax`。

#### 23.20 续三：0.1.253–0.1.257（座位崩溃根因修复、槽位订阅初始同步、渲染稳定性待解）

1. **座位崩溃根因已修复**：`turnProcessAlwaysOpen` 读取 `location.turn.end?.data.reason.kind`，而桥接自建的 `conversation-data` 投影把 `TurnLocation.start/end` 写成了 `{seq, time}` 裸对象，缺 `data`。改为事件形状 `{type:'turn/start'|'turn/end', seq, time, data:{turn[, reason]}}` 并补 `steps` 后，`Cannot read properties of undefined (reading 'reason')` 在 `ChatNodeSeat` 路径上不再出现（0.1.256 实测控制台无该异常）。
2. **注入面实测确认**：`conversation.chat.flow` 贡献的 `keyedHooks` 为 `chatNode/chatNodeBottom/chatNodeProcess/chatGroup`，`hooks` 为 `presentation/connectionState`，`sessionId` 正确解析；`useChatNode('user:1')` 与 `useChatNode('assistant-step:0')` 均返回真实节点（`kind` 分别为 `user` / `assistant-step`）。即 keyed Hook 与节点数据链路本身是通的。
3. **槽位订阅补初始同步**：`slots.subscribe(slotId, listener)` 在注册监听后追加一次微任务同步回调，避免「视图先注册、会话后订阅」时台账不刷新。
4. **实测仍不稳定**：连续两次同条件复测（同一会话 `536b868e…`、同一脚本、各等待 12s）出现 `viewAreaLength` 0 / 2085 交替，`[data-chat-node-key]` 始终为 0。说明视图台账或 React 桥接的更新时机存在竞态，尚未收敛；`conversation.chat.node` 的 DSH keyed 派发与宿主 list 声明不一致这条契约冲突仍未解决（见 23.20 续二第 5 点）。
5. 当前干净安装版本 `0.1.257`；`client-ui.test.mjs` 4/4、`client-abi.test.mjs` 8/8。全部临时探针（`dsh-flow-props`、`dsh-keyed`、`__dshDebugSessionSources`、`__dshSessionEventDebug`）已移除。

#### 23.20 续四：0.1.258–0.1.262（DSH 消息流已真实渲染）

本轮把 `/dsh-conversation` 的会话消息流真正渲染出来了，实测证据：`536b868e-a381-5a1f-b568-882fe1df17cd` 页面 `[data-chat-node-key]` 为 2（`user:1`、`assistant-step:0`），`[data-chat-flow]` 的 `innerText` 为真实模型回复（"你好呀，收到啦～渲染这边看起来是正常的…"），页面控制台无异常。

关键修复：

1. **DSH slot-level hook 工厂**：`CHAT_FLOW_INJECT` 这类 `inject.hooks` 成员是 `SlotHookFactory`（`(standard, hookContext) => Hook`），不是 observable。桥接原先只处理 observable，导致 `useGroupAction is not a function` 直接打断 `ChatFlow` 渲染。现在按工厂语义调用并传入 `hookContext`（由 `renderMappedSlot` 透传 `options.hookContext`）。
2. **子槽位 `commonInject` 合并**：`conversation.view` 注册时声明的 `children['conversation.chat.flow'].inject`（`CHAT_FLOW_INJECT`）需要合并进子贡献的注入面；桥接新增 `dshCommonInjects` 表并在渲染时按 `slotId` 合并。
3. **keyed → list 派发翻译**：`ChatNodeSeat` 用 `renderSlot('conversation.chat.node', …, { entryKey: kind })` 做 keyed 派发，而宿主把该槽位声明为 `list`。未修改宿主契约（`front/src/ui-runtime/slotContract.ts` 的 `chat.conversation.node` 被 `MessagesArea.vue`、`ConversationProjectionHost.vue` 以列表语义使用，改成 keyed 会破坏原生聊天渲染），改为桥接层在宿主槽位非 keyed 时把 `entryKey` 翻译成 `only` 过滤，命中我们以 `cellId/key` 注册的同名贡献。这一改动让"未知 surface 事件：undefined"消失，节点正文开始渲染。
4. **turn 边界事件形状**：`conversation-data` 的 `TurnLocation.start/end` 由 `{seq,time}` 改为 `{type,seq,time,data:{turn[,reason]}}` 并补 `steps`，修掉 `turnProcessAlwaysOpen` 的 `Cannot read properties of undefined (reading 'reason')`。
5. **user 消息 `source.form: null`**：`ContextBody` 的 `form` 闭集对 `undefined` 直接 `unreachable context form: undefined` 抛错，补 `form: null` 后该异常消失。

遗留缺口（本轮未解决，未验证）：

1. 用户消息节点 `user:1` 只渲染出时间戳，正文（"你好，这是一条渲染验证消息"）未出现；该节点内出现两个 `chat.message.renderer` 子槽位但无文本，说明用户正文走的 renderer 映射仍未对齐（助手消息走 markdown 路径正常）。
2. `session.prompt` 的真实发送链路（composer 回车 → `conversation-message-send` → 真实模型 turn → 事件回流刷新）尚未做端到端验证；发送会在用户会话里产生真实消息，需用户明确同意后再执行。
3. 原生聊天页回归未跑（本轮未改宿主代码，风险较低，但仍需在改动宿主槽位契约时补测）。

当前安装版本 `0.1.262`；`client-ui.test.mjs` 4/4、`client-abi.test.mjs` 8/8 通过。

#### 23.20 续五：0.1.263–0.1.265（工厂槽位定向渲染、用户正文仍为空）

1. **工厂槽位定向命中**：`registerFactory` 在官方映射存在时（如 `conversation.chat.user.content → chat.message.renderer`、`conversation.user.text → chat.message.renderer`）只把贡献注册进宿主槽位，却没有可辨识的 id，导致 `renderFactorySlot` 在 list 槽位上无 key 命中。现在工厂注册统一使用稳定 id `dsh-factory:<dshSlotId>`，并在 `renderFactorySlot` 中把它翻译成 `only` 过滤，`chat.message.renderer` 里能正确渲染出 `dsh-factory:conversation.user.text` 贡献。
2. **实测确认工厂 props 正确**：`conversation.user.text` 贡献收到的 `content` 为 `{text, attachments, extraBlocks}`，`text` 长度 13（即「你好，这是一条渲染验证消息」），说明 owner props 传递无误。
3. **用户正文仍为空**：该贡献最终渲染为 `<span class="BNl2La_root"><span class="user_text_plainRun"></span></span>`，`text` 未进入 `plainRun`。该工厂内部还会注册 keyed 子槽位 `conversation.user.text.node`（`text`/`session` 两个 key），推测断点在工厂 → keyed 子槽位的二次派发或子槽位 owner props 传递上。
4. 另一处待查：同一 `chat.message.renderer` 槽位内出现两个完全相同的 `dsh-factory:conversation.user.text` 贡献，怀疑工厂被注册两次（模块重复装载或别名重复设置），需与子槽位问题一并排查。
5. 当前安装版本 `0.1.265`，`client-ui.test.mjs` 4/4；真实会话 `536b868e…` 实测 `[data-chat-node-key]` 2 个、助手正文正常、控制台无异常，用户消息仅显示时间戳。

#### 23.20 续六：0.1.267–0.1.268（预声明槽位的 inject 兜底，用户正文仍未渲染）

1. `createSlotBridge.inject` 原实现只依赖宿主 `runtime.inject` 的**声明事件**，而 `chat.message.user.text` 等槽位是宿主预声明的（启动日志明确记录 `predeclared by the host`），声明事件不会再触发，导致 DSH 侧 `slots.inject(...)` 里的注册（`PlainUserText`、`SessionReference`）永远不执行。现在若 `getDefinition(mapped)` 已存在，直接以该定义执行注入回调，否则回退到声明事件订阅。
2. 该项修复后用户正文仍未出现，`chat.message.user.text` 对应的贡献既没有渲染也没有日志（该槽位没有贡献挂载），说明 `conversation.user.text` 工厂内部对子槽位的派发仍未走到。断点继续收敛在「工厂组件 → keyed 子槽位 `conversation.user.text.node`（`text`/`session`）」这一层。
3. 当前干净安装版本 `0.1.268`；`client-ui.test.mjs` 4/4；真实会话 `536b868e…` 实测 `[data-chat-node-key]` 2 个、助手正文 110 字、控制台无异常，用户消息仅显示时间戳。
4. 未做：`session.prompt` 真实发送的端到端验证（会在用户会话产生真实消息）；原生聊天页回归（本轮未改宿主槽位契约，只新增了内核方法 `host.conversation.scope`）。

#### 23.20 续七：0.1.269–0.1.270（用户气泡空白的根因定位）

1. 实测确认 `chat.message.user.text` 槽位已正确注册两个 keyed 贡献（`text`、`session`，`kind: keyed`、`scope: session`），说明子槽位注册链路是通的。
2. 但该槽位在 DOM 中完全没有出现（`[data-slot-id="chat.message.user.text"]` 不存在），即 `conversation.user.text` 工厂组件 `UserText` 从未渲染子槽位。
3. 对 `dsh-factory:conversation.user.text` 贡献注入探针后拿到关键错误：`user-text rule "session" failed to evaluate`。对应 `session-reference-rule.ts` 的 `match()` 在遇到 `@` 时会调用 `context.read('sessionLabels')`，该读取抛错导致整条 user-text 评估失败，`UserText` 因此拿不到任何文本片段，最终渲染出空的气泡（`<span class="user_text_plainRun"></span>`）。
4. 结论：用户气泡正文缺失与槽位派发无关，根因在 **user-text facts（`sessionLabels`）没有传进工厂**。DSH 由 `MessageItem` 通过 `renderFactorySlot(..., { facts })` 传入 `facts`，还需确认这一字段在桥接的 owner props 透传中是否丢失、以及 `UserTextFacts` 读取器在缺 key 时应返回空数组而不是抛错。
5. 安装版本 `0.1.270`，`client-ui.test.mjs` 4/4；真实会话实测 `[data-chat-node-key]` 2 个、助手正文 110 字、控制台无异常。全部临时探针已移除。

#### 23.20 续八：0.1.271（工厂派发的谓词命中问题）

1. `chat.message.renderer` 里实测有 3 个互不相同的工厂贡献：`dsh-factory:conversation.chat.reasoning.content`、`dsh-factory:conversation.chat.user.content`（chat 包）、`dsh-factory:conversation.user.text`（conversation 包），`key`/`entryKey`/`cellId` 均为各自工厂 id，没有 id 冲突。
2. 用户行里实际挂载的却是一直是 `dsh-factory:conversation.user.text`，而不是 `conversation.chat.user.content`；同时给工厂 owner 补默认 `facts: { sessionLabels: [], skillNames: [] }` 之后 `session` 规则不再抛错，但用户正文仍为空、`area` 尺寸不变。
3. 复核宿主实现后确认 `only` 的语义没有问题：`dispatchContributions` 对 list 槽位按 `entry.cellId === listOnly` 精确过滤（`front/src/ui-runtime/clientPluginRuntime.ts:845-855`），我们的工厂贡献 `cellId` 正是各自工厂 id，过滤本身能命中唯一一条。
4. 因此剩下的可能是**派发用的 DSH 槽位名与实际渲染的贡献不一致**：用户行外层请求的是 `conversation.chat.user.content`，但实际挂载的是 `dsh-factory:conversation.user.text`。需要确认 `MessageItem` 在该路径下究竟调用哪个工厂名、以及 `factorySlotAliases` 是否被后注册的 `conversation.user.text` 覆盖（两个工厂映射到同一宿主槽位 `chat.message.renderer`）。
5. 安装版本 `0.1.271`，`client-ui.test.mjs` 4/4；助手消息正文正常，用户气泡仍空。

#### 23.20 续九：0.1.275（用户消息正文修复，消息流双向完整渲染）

1. 关键发现：宿主把 `chat.message.renderer` 声明为 **chain** 槽位（不是 list），而在 chain 分支里 `dispatchContributions` 是按各贡献的 `select(owner)` 决定归属的。桥接注册工厂贡献时用的是默认 `select: owner => owner`（恒真），于是三个工厂贡献（chat 的 `conversation.chat.user.content`、`conversation.chat.reasoning.content`、conversation 的 `conversation.user.text`）里先注册的那一个永远胜出 —— 用户行实际渲染的是 `conversation.user.text`，`conversation.chat.user.content` 从未执行，用户气泡自然没有正文。
2. 修复：工厂贡献的 `select` 改为按调用方标记选择 —— `renderFactorySlot` 在 owner 上打 `__dshFactory` 标记，`select` 只在「有标记且不是自己」时返回 null，无标记时保持原有的恒真语义（避免影响 `conversation.content` 这类同样走 chain 的工厂）。
3. 实测（`0.1.275`，两次连续复测结果一致）：
   - `536b868e-a381-5a1f-b568-882fe1df17cd`：`[data-chat-node-key]` 2 个，flow 文本 124 字，依次为「你好，这是一条渲染验证消息」→ 时间戳 → 助手回复，`viewArea` 7953 字节，控制台 0 异常。
   - `15321b8f-d0bf-59de-9279-03a8c2ce454f`：同样 2 个节点、124 字、0 异常，`viewArea` 6178 字节。
4. 该轮同时修正了此前文档里关于 `only` 过滤的错误结论：list 的 `only` 按 `entry.cellId` 精确匹配，机制本身正确；真正的问题是 chain 的 `select` 谓词。
5. 至此 `/dsh-conversation` 的用户消息与助手消息均由官方 `dsh-client-ui-chat` / `dsh-client-ui-conversation` 组件真实渲染，数据来自 `host.conversation.read`，事件经 `SessionEventWindow` 投影。
6. 未做：`session.prompt` 真实发送的端到端验证（会在用户会话产生真实消息，需用户确认）；原生聊天页回归（本轮未改宿主代码）。

#### 23.20 续十：0.1.276–0.1.282（真实发送链路打通并验证）

1. **发送契约修正**：`session.prompt` 原先向 runtime 的 `conversation-message-send` 多传了 `mode`，被 manifest 的 `additionalProperties` 拒绝（原始错误：`input schema instance_mismatch: / additionalProperties(mode)`）。移除该字段后发送成功。
2. **发送后实时刷新**：只刷新 `SessionEventFeed` 不够——ChatView 实际消费的是桥接自建的 `conversation-data` 投影。改为在发送成功后同时刷新会话投影，并在 400/1200/3000/6000ms 追加几次延时刷新，覆盖宿主异步落库与新回合生成。
3. **刷新注册表去除全局**：新建 `registerConversationData` / `refreshConversationData` 于 `conversation-data.mjs`，`runtime-services` 直接 import 调用，不再依赖 `globalThis` 临时钩子；验证用的 `__dshPromptProbe` 已删除。
4. **实测（0.1.279 / 0.1.281 / 0.1.282）**：在会话 `536b868e-a381-5a1f-b568-882fe1df17cd` 通过 `session.prompt` 真实发送消息，宿主接受并落库（`conversation-read` 计数 2 → 3 → …），界面无需刷新即实时追加节点（`[data-chat-node-key]` 3 → 4 → 5 → 6），`viewArea` 12793 字节，控制台 0 异常，`client-ui.test.mjs` 4/4。
5. **原生聊天页回归**：切到 `#/chat` 重载后输入框存在、消息气泡正常渲染、控制台 0 异常，内核新增方法未影响原生会话页。
6. 说明：DSH 编辑器的回车提交依赖受信任键盘事件，CDP 合成的 `Input.dispatchKeyEvent` 无法触发（草稿会累积但不提交），因此发送验证走的是 composer 同一条 `session.prompt` seam，而不是合成键盘事件；后续如需验证键盘路径，只能在真实窗口里手工回车。
7. 验证过程中在测试会话里留下了 4 条真实测试消息（「DSH 桥接发送验证（自动测试）」「DSH 桥接实时刷新验证」×3），可随时删除。

#### 23.20 续十一：界面发送路径端到端验证通过

1. 之前只用 `session.prompt` seam 验证过发送，本轮补上了**界面路径**：在 composer 输入草稿后点击 `button[aria-label="发送消息"]`。
2. 实测（`0.1.282`）：草稿被清空、`[data-chat-node-key]` 6 → 7、`conversation-read` 6 → 7、控制台 0 异常，界面无需重载即时追加节点。
3. 说明两点差异：`Input.dispatchKeyEvent` 合成的回车与 `Input.dispatchMouseEvent` 合成的点击都没有触发提交（草稿保留），而直接 `button.click()` 生效 —— 说明 DSH 的提交按钮走的是 React 合成事件，键盘提交则依赖受信任按键，后者仍只能人工验证。
4. 至此 `/dsh-conversation` 的「显示真实消息」与「通过界面真实发送消息」两条主链路都已闭环并留下实测证据。

#### 23.20 续十二：空会话（hero）态输入器的真实原因

1. 在空会话 `8a6cbe2f…` 上实测：`data-phase="hero"`，渲染出 `chat.conversation.root`、`chat.conversation.session`、`chat.composer.root/bar/attachments/permission/model` 等槽位，但**没有 `contenteditable`**，因此无法直接输入首条消息。
2. 根因不是桥接丢槽位：hero 态的空状态文案是「选择一个工作区开始」，`chat.empty.workspace` 在等用户选工作区；`InputBar` 的守卫是 `live = input && keyboard && inputActions`，工作区未定时 `keyboard` 尚未就绪，于是输入器渲染为惰性占位。这属于 DSH 自身的 hero 交互，不是桥接缺失。
3. 尝试过在贡献 `inject` 中为空 `sessionId` 回退 `globalThis.__dshCurrentSessionId`，实测 hero 态输入器仍未出现，且该改动会改变 root 作用域注入语义，已回退（`0.1.284`）。
4. 结论：DSH 页的「新会话首条消息」依赖 Workspaces 映射完整（当前 `workspaces` 仍为 partial，`insertSessionBefore` 未实现，`agentPreset` 未映射），属于后续 P13 工作项，不在本轮扩大范围。
5. 当前安装版本 `0.1.284`：会话 `536b868e…` 7 个节点、`viewArea` 14046 字节、控制台 0 异常，`client-ui.test.mjs` 4/4。

#### 23.20 续十三：把投影/事件契约固化成回归测试

1. 新增 `tests/conversation-data.test.mjs`（4 项）。此前这些行为只在真实渲染里被间接验证，一旦改动 `conversation-data.mjs` 或 `session-events.mjs` 很容易静默回退。
2. 覆盖点：
   - `projectConversationMessages`：node key 使用消息原始下标（`user:0` / `assistant-step:1`）、`data.source.form === null`（`ContextBody` 闭集要求）、`location.turn.start/end` 为完整事件形状且 `end.data.reason.kind === 'completed'`、`steps` 为数组。
   - `projectSessionEvents`：每个 entry 都是 `{type:'event', event}`，事件序列为 `turn/start → step/start → user/message → assistant/message → step/end → turn/end`，消息事件带 `surfaceOp: 'append'`，`turn/end` 带 `reason`。
   - `createConversationData`：初始 order 为空、`refresh` 走 `conversation-read`、`target('other')` 返回 undefined、`activate('chat')` 记录 activeView 且 session 快照 `openState === 'open'`、`ready === true`。
   - `send`：空内容抛「发送内容不能为空」；正常发送按 `conversation-message-send → conversation-read` 顺序执行。
3. 测试结果：`conversation-data` 4/4、`client-ui` 4/4、`client-abi` 8/8。重新打包得到相同 `treeHash`（`4f1d7674…`），确认测试不进入发布产物。

#### 23.20 续十四：渲染噪声消失与验证数据的准确说明

1. 复测两个真实会话的节点种类，已只剩 `user` / `assistant-step` 两类，早期出现过的「系统提示词 / 上下文已压缩 / 已压缩 undefined 条历史记录 / 本轮运行失败 / 已达到输出 token 上限 / 未知 surface 事件」等噪声节点不再出现（那是 0.1.259 之前的旧投影路径产物）。
2. 更正 23.20 续十一的一条描述：界面发送验证时，点击发送按钮提交的内容是我此前多次 `Input.insertText` 累积下来的整段草稿（三段测试文案拼在一起），而不是单条「DSH 界面发送验证」。提交行为本身成立（草稿清空、消息数 +1、节点 +1），但该条消息内容是拼接产物。
3. 因此测试会话 `536b868e…` 里现有 7 条消息中，最后一条是被污染的长文本；如需干净环境，建议整段会话删除重建（当前没有按条删除的通用能力）。
4. 交付状态不变：插件 `0.1.284`，`conversation-data` 4/4、`client-ui` 4/4、`client-abi` 8/8，会话页渲染无异常。

#### 23.20 里程碑小结：/dsh-conversation 会话页可用（0.1.284）

交付能力：

1. 用官方 `@deepseek-ai/dsh-client-ui-conversation` + `dsh-client-ui-chat` 渲染真实会话消息（用户气泡 + 助手回复），数据来自 `host.conversation.read`。
2. 通过界面按钮或 `session.prompt` 真实发送消息，宿主落库、界面实时追加节点。
3. 会话切换、空会话 hero 态、原生聊天页均行为正确且无控制台异常。

关键实现点（按依赖顺序）：

| 层 | 内容 |
|---|---|
| 内核 | 新增通用 `host.conversation.scope`（四张权限表 + capability matrix + 路由），复用 `createConversationTargetSnapshot` |
| 运行时 | `conversation-scope` / `conversation-read` / `conversation-message-send` 三个 action；`sessions.binding().eventSource` 接真实 `SessionEventFeed`；`session.prompt` 接真实投递 |
| 事件投影 | `session-events.mjs` 输出 `SessionEventLikeEntry`（`turn/start…turn/end`，`surfaceOp: 'append'`） |
| 数据投影 | `conversation-data.mjs` 输出 ChatSnapshot / ConversationSnapshot / SessionSnapshot，`TurnLocation` 用完整事件形状 |
| 桥接 | hook 工厂调用、子槽位 `commonInject` 合并、keyed→`only` 翻译、chain 工厂 `select` 谓词、工厂贡献稳定 id、预声明槽位 inject 兜底 |

验证证据：

- 真实会话 `536b868e…` 7 节点 / 242 字、`15321b8f…` 2 节点 / 161 字、空会话 hero 态，均 0 异常。
- 发送：`conversation-read` 计数随发送递增，节点同步 +1，无需重载。
- 测试：插件全量 225/225（含新增 `conversation-data` 4 项）。
- 原生聊天页：输入框与气泡正常、0 异常。

遗留（需后续 P13 工作项或人工确认）：

1. 键盘回车提交未自动验证（受信任按键限制，按钮路径已验证）。
2. 空会话 hero 态输入器需先选工作区，依赖 `workspaces` 映射补全（当前 partial）。
3. `session.loadOlder` 未实现（>100 条消息时「加载更早」会抛 `unsupported`）。
4. 测试会话 `536b868e…` 内有 7 条验证用消息，可按需清理。

#### 23.21 一级侧边栏入口与会话页点击失效修复

1. **点击全页无反应**：宿主 `UIPortalLayer` 内 `.extension-slot` 容器铺满 1280×820，且实际计算出的 `pointer-events` 为 `auto`，这一层 z-index 2400 盖在整个应用之上，吞掉了全部点击。修复：图层元素加内联 `pointer-events: none`，`:deep()` 把 `.extension-slot`、`.extension-slot__contribution` 一并设为 `none`，只让真正的内容 `> *` 接收事件；插件侧补 `syncHitArea()`，React 渲染为空时挂载点也置 `pointer-events: none`。实测 DSH 会话页点击输入框可聚焦、原生聊天页 `elementFromPoint` 直接命中 `TEXTAREA.input-field`。
2. **插件无法自行注册该入口**：`/dsh-conversation` 是宿主内置路由，`providerRoutes()` 对 `shadowsProtectedRoute(path)` 的路径一律跳过（`BUILTIN_ROUTE_PATHS` 命中即受保护），因此扩展声明的 `/dsh-conversation` 路由永远不会成为 effective route，`collectExtensionNavigationItems` 的 `hasEffectiveRoute` 检查会把对应导航项过滤掉。实测把导航项换成 `/dsh-compatible-host` 立刻出现、换回 `/dsh-conversation` 立刻消失，验证了这条判定链。
3. **改为宿主内置条件入口**：`navigationRegistry.resolveBuiltinNavigationItems` 在扩展贡献存在（`extensionId === "com.amitia/dsh-compatible-host"`）时追加一条 `placement: "primary"` 的内置导航项「DSH 会话」（路由 `/dsh-conversation`、图标 chat、order 90）。这与宿主已有的 `DshConversationView` 保持一致，且扩展未安装时该入口自动消失。
4. 插件侧 manifest 恢复原契约（`dsh-compatible-host` 导航项 + 单一路由），插件全量测试回到 225/225。
5. 实测：一级侧边栏顺序为「主页 / 角色 / 记忆 / 任务 / 游戏模式 / 创意工坊 / 扩展中心 / DSH 会话 / 本地用户」，点击「DSH 会话」跳转 `#/dsh-conversation` 并渲染真实消息，控制台 0 异常。

#### 23.22 架构纠正：取消独立 DSH 对话页面

结论（用户判定）：兼容插件的职责是把 DSH 的功能**映射进 Amitia 自己的界面**，独立 DSH 对话页属于重复造第二套 UI，必须删除。

已执行：
1. 删除 `front/src/views/extensions/DshConversationView.vue`、路由 `/dsh-conversation`（`front/src/router/builtinRoutes.ts`）、对应宿主测试 `front/src/__tests__/dshConversationView.test.ts`。
2. 移除刚加的侧边栏条件入口（`ui-runtime/navigationRegistry.ts` 的 `hasDshConversationHost` 与追加项）。
3. 插件侧 DSH 会话导航由 `/dsh-conversation?conversationId=…` 改为原生 `/chat?conversationId=…`（`src/client-abi/runtime-services.mjs`、`src/client-ui/index.mjs`），测试断言同步。
4. 实测：`#/dsh-conversation` 已 404（`.dsh-conversation-root` 不存在），`#/chat?conversationId=…` 原生页正常渲染消息与输入框，控制台 0 异常。

后续必须补齐的映射（本轮未完成）：
- 原生聊天页不加载 DSH 客户端模块，因此 DSH 的消息节点渲染器、composer 动作、面板贡献当前不会出现；需要在聊天页装载 DSH shell 模块。
- 装载前必须先取消 shell 级槽位映射（`conversation.view → chat.conversation.views`、`conversation.chat.flow → chat.conversation.flow`），否则 DSH 的整页 ChatView 会与原生消息区重复渲染。
- 目标形态：Amitia 负责布局与会话数据，DSH 只贡献节点渲染器/动作/面板，实现「一套界面、两种内容」。

#### 23.23 全量装载 DSH 客户端导致原生聊天页失效（已回退）

做法：在 `front/src/components/AppLayout.vue` 用 `useAppClientModules({ enabled: true, contributionIds: ["dsh-desktop-shell-host"] })` 应用级装载 DSH 外壳（shell 组件 `shell.mjs` 渲染 `null`，只做注册），期望 DSH 贡献直接进入原生槽位。

实测：
- 装载后 `#/chat?conversationId=…` 的原生页出现 **23 个 DSH 贡献**、`chat.conversation.flow` 内 22 个贡献，消息文本只出现 1 次（无重复），`#/settings` 也有 8 个贡献。
- 但同时出现 `TypeError: tabs.get is not a function`（4–5 次），随后原生聊天页 **输入框与消息全部消失**（`editableCount: 0`、`messageMentions: 0`）。
- 回退 `AppLayout.vue` 后：`editableCount: 1`、`messageMentions: 1`、`tabs.get` 异常消失、控制台 0 异常。

结论：
1. DSH 外壳一次性挂到应用层不可行——它会把自己的整套槽位贡献注入原生页，与原生渲染路径冲突并导致页面失效；其中 `sidebarRightTabs` 的 `get` 调用还暴露出服务对象被替换的问题。
2. 正确做法是**按槽位定向接入**：只把需要的 DSH 贡献（节点渲染器、composer 动作、面板）逐类接入，并按原生 owner props 做适配，而不是整包挂载。这需要逐槽位设计与验证，不能一次铺开。
3. 当前状态：原生聊天页健康、DSH 独立页已删除、DSH 客户端模块不再挂载（界面处于休眠），插件 `0.1.297`，测试 276/276。

#### 23.24 二次尝试：扩充外壳槽位屏蔽后仍然失效（已回退）

在 23.23 基础上把 `conversation.header`、`conversation.session.header`、`conversation.composer`、`conversation.composer.bar`、`conversation.composer.dock` 一并加入屏蔽列表（`0.1.298`），再次在 `AppLayout.vue` 装载 DSH 外壳。

实测：`#/chat` 仍然 `editableCount: 0`、`messageMentions: 0`，且控制台 **0 异常**——页面被静默清空。回退 `AppLayout.vue` 后立刻恢复（`editableCount: 1`、`messageMentions: 1`、0 异常）。

结论：问题不是槽位接管，而是**装载 DSH 外壳本身**就会让原生聊天页渲染失效（无异常抛出，疑似 React 根与 Vue 页面渲染冲突或模块初始化副作用）。在未定位该冲突前，禁止再尝试应用级挂载；下一步必须先用最小实验（只装载一个 DSH 包而非整壳）逐级定位，再决定接入形态。

#### 23.25 DSH 客户端接入原生聊天页成功（根因：main.conversation 抢占根槽位）

二分定位过程（用临时 localStorage 过滤器逐包装载，定位后已移除）：
1. 只装载 `dsh-client-ui-conversation` → 原生页 `editable:0 / mentions:0`（复现清空）。
2. 单独装载 `dsh-client-ui-chat` → 页面存活（`editable:1`）。
3. 结论：清空由 `dsh-client-ui-conversation` 造成，且不是 composer 槽位接管（屏蔽 composer/input 全家族后仍然清空）。
4. 真正原因：该包把 `main.conversation` 映射到 `chat.conversation.root`（`single/session-maybe`），直接顶掉原生会话根容器；同类的消息渲染映射（`conversation.chat.node`、`conversation.user.text*`、`conversation.chat.*.content`）会顶掉原生消息渲染。

修复（插件 `src/client-ui/index.mjs` 的 `DSH_PAGE_SHELL_SLOTS` 屏蔽集）：把**页面级外壳 + 原生已独占的渲染槽位**全部排除注册——
`main.conversation`、`conversation.content/session/session.header/header/view/chat.flow`、`conversation.composer(+bar/dock)`、`conversation.input.*`（12 项）、`conversation.chat.node`、`conversation.user.text(+node)`、`conversation.chat.user.content`、`conversation.chat.reasoning.content`。

接入方式（宿主 `AppLayout.vue`）：`useAppClientModules({ enabled: true, contributionIds: ["dsh-desktop-shell-host"] })`，注入 `actions`/`ui-context`（含 `conversationId`）/`ui-navigate`。shell 组件渲染 `null`，只做注册。

实测（插件 `0.1.303`，全量包装载）：
- `#/chat`：`editable:1`、消息文本出现 1 次（无重复）、DSH 贡献 6 个、`bodyLen:630`、控制台 0 异常；
- `#/characters`、`#/settings/system`、`#/memory` 均正常渲染、0 异常；
- 插件测试 276/276。

含义：DSH 客户端模块现已常驻原生界面，Amitia 负责会话布局/消息/composer，DSH 只贡献附加能力；DSH 自己的消息渲染因与原生重复而明确不注册。

#### 23.26 节点级映射的宿主前提（未解决）

实测槽位语义：
- `chat.conversation.node` 在宿主是 **list** 槽位（`kind: "list"`），原生 `MessagesArea` 会把该槽位所有可见贡献按节点循环渲染；
- DSH 的节点渲染器（`conversation.chat.node` → `chat.conversation.node`）是 **按 kind keyed** 的（`user` / `assistant-step` / 工具节点等）。

后果：一旦注册 DSH 节点渲染器，它们会对原生**每个**节点渲染一次，把原生消息顶掉（23.25 中实测 `mentions` 由 1 变 0）。

因此节点级映射必须先在宿主侧引入「按节点 kind 分派」：`MessagesArea` 渲染 `chat.conversation.node` 贡献时按 `entryKey === node.kind` 过滤，再放开插件侧对 `conversation.chat.node` 的屏蔽。本轮未做，因为它是宿主渲染契约变更，影响原生消息渲染回归。

当前已注册的 DSH 贡献（实测）：`chat.header.action` 2 个（`archive`、`pin`）；`chat.conversation.node`、`chat.message.renderer`、`chat.message.badge` 均为 0（被屏蔽或未注册）。

#### 23.27 阶段完成度快照（catalog 口径）

| 维度 | 数值 | 完成度 |
| --- | --- | --- |
| UI Slot 映射 | 120 / 120（0 pending） | 100% |
| Client Service | 4 implemented + 4 partial / 8 | 75% |
| Host Service | 12 implemented + 27 delegated + 9 partial / 69（另 8 pending、13 unavailable） | 63% |
| 加权合计 | 等权三项 | **79.3%** |

说明：catalog 未随本轮 agent 接线同步刷新（workspaces 的 create/rename/delete/archive 已接真实 action，Host Service 的 workspaceFiles/fileReferences/skills 等已接 `host.workspace.read`），因此上表是**保守口径**；实际可用面高于该数字，但未逐项复核前不并入统计。

进入稳定态后的验证：连续 3 次重载 `editable:1 / 消息 1 次 / DSH 贡献 6 / 0 异常`；插件测试 276/276；前端 5178、Core 64310 正常。

剩余必须的宿主改动（未做，需确认）：`MessagesArea` 的 `chat.conversation.node` 由 list 全量渲染改为按 `node.kind` 分派，否则 DSH 节点渲染器无法与原生消息共存。

#### 23.28 兼容性清单对齐（只登记已验证的接线）

依据本轮 agent 的实际代码与测试证据，更新 `plugins-code/dsh-compatible-host/catalog/compatibility.v1.json`：

| 服务 | 变化 | 证据 |
| --- | --- | --- |
| `fileReferences`（017） | pending → implemented | `src/runtime/host-services.mjs` 中 10 处 `host.workspace.read` 调用（search/stat/read/read_bytes） |
| `sessionFileReferences`（032） | pending → implemented | 同上，按会话作用域调用 |
| `workspaceFiles`（068） | pending → implemented | 同上（file_list/search/stat/read/read_bytes） |
| `skills`（047） | pending → partial | 本地注册表 register/registerProvider/get 已实现；`list`/`snapshot` 仍缺 `host.skill.list`，明确失败 |

更新后统计：implemented 15、delegated 27、partial 10、pending 4、unavailable 13（共 69）。

完成度（等权三项）：UI 120/120 = 100%；Client (4 + 4×0.5)/8 = 75%；Host (15 + 27 + 10×0.5)/69 = 68.1%；**合计 81.0%**。

仍未闭环：`directoryPicker`、`directoryPickerController`、`sessionSkillCatalog`、`workspaceController`（4 项 pending，均需宿主新增通用能力）；13 项宿主不具备；P8 历史回滚、P14 事务化恢复与残留审计、跨平台矩阵未做。

#### 23.29 DSH 头部动作真实链路验证（发现后端缺陷）

验证方式：在原生聊天页点击 DSH 贡献的「归档会话」按钮，抓取完整调用链。

真实链路（已确认连通）：
1. `dsh-client-ui-workspace` → `UiWorkspaceService.archiveSession`
2. → 桥接 `workspaces.archiveSession` → `workspaceHostCall`
3. → 宿主 action `conversation-archive` → `host.conversation.archive`

失败点（后端）：
```
host.conversation.archive: internal_error: 数据版本已变化，请重新加载
```
对应 `backend/cmd/server/kernel_host_adapters.go:586` 的 `ArchiveConversation`：`ReadResource(ctx, ownedRequest, "conversation", id)` 返回 `nil` 或 `Deleted` 时直接 `return coordination.ErrResourceVersion`，经 `hostRouteInternal` 转成该文案。也就是说**资源读取没有拿到会话记录**，被当成版本冲突上报。

结论：桥接侧与 DSH 侧链路正确，缺陷在后端适配器——归档走 device-mesh `ReadResource` 时未取到 conversation 资源，需要用与消息发送相同的读取通道（或先确认资源归属键）后再 `Edit`。

修复方向（下一轮）：`ArchiveConversation` 改为与 `conversationMessageSendRouteHandler` 相同的资源读取路径；若确实不存在则返回明确的 not-found，而不是误报版本冲突。

#### 23.30 归档缺陷的对照结论

对比原生与宿主 ABI 两条路径：

| 路径 | 实现 | 结果 |
| --- | --- | --- |
| 原生「归档对话」 | `front/src/stores/chat.ts:234` `owned.edit("conversation", id, {archived:true})` | 正常 |
| 宿主 ABI `host.conversation.archive` | `backend/cmd/server/kernel_host_adapters.go:586` `engine.ReadResource(...)` + `engine.Edit(...)` | `ReadResource` 返回 nil → 误报 `ErrResourceVersion` |

即：两条路径都在做 device-mesh 的 owned edit，但宿主适配器多了一步 `ReadResource`，而这一步在当前 scope 下取不到 conversation 资源，于是把「没读到」错报成「版本冲突」。

修复方案（下一轮，需要重建 Core + 重启完整服务）：
1. 去掉 `ReadResource` 前置，直接按 `owned.edit` 的语义调用 `engine.Edit`（revision 由引擎内部处理）；
2. 若引擎要求显式 revision，则改用原生同款的 owned-resource 读取通道获取 revision；
3. 资源确实不存在时返回明确的 not-found，而不是 `ErrResourceVersion`。

#### 23.31 归档修复的最终定位

对照三个适配器对 `engine.ReadResource` 的使用：

| 适配器 | 资源类型 | 现状 |
| --- | --- | --- |
| `projectAdapter.UpdateProject`（kernel_host_adapters.go:877） | `project` | 可用 |
| `conversationCreatorAdapter.ArchiveConversation`（:598） | `conversation` | `ReadResource` 返回 nil |
| `conversationCreatorAdapter.DeleteConversation`（:558） | 走 `engine.ManageChat` | 可用 |

结论：`conversation` 不是 device-mesh 的 `ReadResource` 资源（它由 chat runtime 管理），所以归档适配器拿不到 revision，被 `ErrResourceVersion` 掩盖。

推荐修复（下一轮执行）：
1. `ArchiveConversation` 不再使用 `ReadResource`，改走与原生 `owned.edit` 等价的通道：要么在 chat management 层新增 `conversation/archive` 操作并复用 `ManageChat`，要么直接调用 chat runtime 的 owned edit 入口；
2. 两者都不具备时，返回明确的 `host unavailable`，而不是版本冲突。

该改动涉及后端 + 重建 Core + 重启完整服务，属于一个完整的四步循环，本轮预算不足以安全完成（当前剩余轮次 4）。

#### 23.32 归档修复的落地阻塞点（需新增管理操作）

核实结果：
1. `engine.Edit`（`internal/devicemesh/business/edit.go:33`）要求 `ExpectedRevision >= 1`，不接受 0。
2. `engine.ReadResource(ctx, req, "conversation", id)` 对会话返回 nil（会话不是 mesh 资源）。
3. chat management 通道（`cmd/server/mesh_local_chat_management.go:323`）只支持 `conversations / conversation/create / conversation/read / conversation/rename / conversation/delete`，**没有 archive / update**。

因此归档无法在现有通道内完成，必须新增其一：
- 方案 A（推荐）：在 chat management 增加 `conversation/archive` 操作，复用 `ManageChat`（与 delete 同构），归档适配器改为调用它；
- 方案 B：为会话提供可读取的 revision 通道（查询返回 revision），适配器再走 `Edit`。

两者都是后端改动 + 重建 Core + 重启完整服务，本轮剩余预算 2 轮，未执行。

### 23.33 归档链路打通（原生页面内点击生效）

DSH 工作区的「归档会话 / 取消归档」按钮此前只走到宿主 `host.conversation.archive` 就报错，原因与修复如下：

- 缺少管理操作：`ChatManagementRequest` 增加 `Archived *bool`，`conversation/archive` 纳入会话操作分支，落库字段使用系统既有的 `archivedAt`（空串表示未归档），写入沿 `conversation/rename` 的 `p.store.Apply` + `target.Revision` 通道，请求号命名空间为 `archive-chat/<sha256>`。
- 请求号缺失：原生 UI 调用不带 `requestId`，而会话管理对非列表操作要求请求号非空，宿主适配层在为空时生成 uuid，避免把「缺参数」误报成内部错误。
- 列表语义：`host.conversation.list` 增加 `archived` 入参并透传到 `ArchivedOnly`；默认列表排除已归档会话，`archived: true` 只返回已归档会话。
- 客户端状态：工作区目录的 `archivedSessionIds` 改为以宿主为准（刷新时查询归档列表），不再依赖持久化快照，按钮文案随之从「归档会话」切到「取消归档」。

验证（Electron + 原生 `/chat` 页面真实点击）：初始 active、点击「归档会话」后归档列表为 1 且从活跃列表移除、按钮变「取消归档」、再次点击回到活跃列表且归档列表归零；插件测试 276/276、内核包测试通过。`cmd/server` 的 `TestOwnedChatCleanup*` 三个用例为既有陈旧用例（空数据下期望清理预览成功），与本次改动无关。

### 23.34 置顶链路打通（原生页面内点击生效）

- 宿主新增 `host.conversation.pin`：请求结构 `ConversationPinRequest`、`ConversationPinner` 接口、路由 handler 与注册项，权限沿用 `conversation.manage`，能力矩阵与 `AllRegisteredMethods` 同步登记。
- 会话管理新增 `conversation/pin` 操作与 `ChatManagementRequest.Pinned`，落库使用系统既有 `pinnedAt` 时间戳（空串表示未置顶），写入通道与 `conversation/rename` 一致；宿主适配层在请求号为空时生成 uuid。
- 插件侧 `workspaces.pinSession / unpinSession` 由「明确失败并提示缺少 host action」改为真实调用 `conversation-pin`；工作区目录的 `pinnedSessionIds` 改由宿主列表条目的 `pinnedAt` 推导，`archivedSessionIds` 由归档列表查询，两者都不再依赖持久化快照。
- 列表投影 `catalogSession` 的 `pinnedAt / archivedAt` 为唯一真值来源，原生侧边栏与 DSH 按钮读取同一字段。

验证（Electron + 原生 `/chat` 真实点击）：置顶后 `pinnedAt` 写入、按钮 `aria-label` 从「置顶会话」变为「取消置顶」且图标切换为实心；取消置顶后 `pinnedAt` 清空并回到「置顶会话」。归档与置顶互不影响，验证结束后会话回到未归档未置顶状态。插件测试 276/276 通过。

### 23.35 技能目录宿主方法 host.skill.list

- 内核新增 `host.skill.list`：请求 `{query?, enabled?, limit?}`，按当前会话作用域调用扩展内核既有的 `agent_skill` 目录，返回 `{items:[{skillId,id,name,displayName,description,version,extensionId,contributionId,enabled}]}`，不新建第二套技能系统。
- 权限沿用 `conversation.read`：技能目录是会话上下文，`extensions.read` 为 `ScopeGlobal`，扩展运行时调用会被判 `scope_not_allowed`，实测确认该映射不可用后改用会话读取权限。
- 能力矩阵与 `AllRegisteredMethods` 登记为复用现有能力；插件新增 `skill-list` 动作并写入 manifest 入参枚举，运行时通过 `host.call("host.skill.list")` 调用。
- 实测：动作调用返回 `{"items":[]}`（当前环境未安装任何提供 SKILL.md 的扩展，目录为空属预期），无权限错误、无参数错误，链路可用。

仍未完成（保持目标未达成）：DSH 自有消息节点渲染（`conversation.chat.node` keyed → 会话节点投影）、`host.directory.pick/list/createDirectory` 三项交互式目录能力、P8 历史回滚、P14 事务恢复与残留审计、跨平台能力矩阵。

### 23.36 skills 服务接入宿主技能目录

- `createSkillsService` 由「list/snapshot 直接抛缺口」改为持有宿主目录缓存：`refresh()` 调 `host.skill.list`（limit 200）把条目规范化为 DSH 技能描述，`list()` 返回本地注册与宿主条目的合并数组（本地同名优先），`snapshot()` 返回 `{items,state,error}`，`get(name)` 先查本地注册再查宿主条目。
- 目录拉取失败时把原始 error 原样存入 `snapshot().error` 并向上抛出，不做文案替换。
- 测试改写为真实合并语义并断言 `host.skill.list` 被调用；插件测试 276/276 通过；实测插件 0.1.312 生效，`skill-list` 动作返回空目录（当前环境无提供 SKILL.md 的扩展），核心 health 正常。

仍未完成（目标保持未达成）：`conversation.chat.node` keyed 节点渲染、`host.directory.pick/list/createDirectory`、P8 历史回滚、P14 事务恢复与残留审计、跨平台能力矩阵。

### 23.37 节点渲染试验结论（已回退）

- 试验内容：解除 `conversation.chat.node` / `conversation.user.text.node` 的 skip 名单，在槽位注册时改走 `globalThis.amitiaClientPlugins.conversationNodes.register(key, component, {eventTypes:[key], nodeType:key}, {id, key, ordering})`。
- 结果：节点注册链路本身成立——页面出现 `[data-conversation-node-id]` 节点并渲染了 DSH 的 `PlainUserText`；但 owner props 契约不匹配，抛出 `TypeError: Cannot read properties of undefined (reading 'display')`，说明 DSH 节点渲染器需要 `content.display` 这类节点级字段，而 Amitia 现在传入的是 `{eventType, conversationNode, conversationId}`。
- 处理：本次试验已完整回退（恢复 skip 名单、移除注册分支），插件 0.1.314 生效；复核页面 `messages=14 / composer=1 / header=[归档会话,置顶会话]`，无异常。
- 下一步：需要先拿到 DSH 节点 owner 的字段契约（`@deepseek-ai/dsh-client-ui-conversation` 的节点类型定义），再在桥接层构造 `content.display` 等字段；在此之前保持 skip，避免原生气泡被未适配的节点渲染器顶掉。

仍未完成（目标保持未达成）：`conversation.chat.node` 正式适配、`host.directory.pick/list/createDirectory`、P8 历史回滚、P14 事务恢复与残留审计、跨平台能力矩阵。

### 23.38 P8/P14/跨平台矩阵结论（2026-11-18）

完整取证文档：`docs/architecture/dsh-p8-p14-crossplatform-20261118.md`。本轮新增 `backend/internal/extension/kernel/p14_transaction_recovery_drill_test.go` 与只读审计脚本 `scripts/audit/dsh-plugin-residual-audit.py`；未改插件 `src/**`、未改后端业务逻辑、未重启服务、未安装插件、未做 Git 操作。

#### 23.38.1 P8 历史回滚

**现状**

1. 「宿主扩展（含 DSH 宿主本身）的历史版本回滚」**已经存在且持久化**，且有生产路由：`POST /api/extensions/versions/rollback/preview|confirm|rollback`。
2. 「DSH 子包（`plugin_data/dsh-tree`）的历史版本回滚」**不存在**：npm workspace 单树原地安装，失败只回写 `package.json`。
3. 插件数据备份存在，但 DSH 宿主的 `data.pluginData.include` 只声明了 `dsh-data`，**不含 `dsh-tree`**。

**证据**

- 回滚点持久表与数量：`backend/internal/extension/kernel/persistence/sqlite/schema.go:1764`（`extension_package_rollback_points`）；实测

  ```text
  com.amitia/dsh-compatible-host  points=306 active=306 versions=0.1.23..0.1.313 dangling=0
  ```

  即回滚目标代次目录全部在盘上，没有「回滚点悬空」。
- 代次目录与指针：`AmitiaData/extensions-v2/installations/com.amitia__dsh-compatible-host/{current.json,previous.json,generations/}`，current=0.1.314、previous=0.1.313，DB `current_generation_id` 与 `current.json` 一致（审计输出 `match`）。
- 回滚 API：`backend/internal/extension/package_distribution_api.go:29-31`；挂载前缀 `backend/internal/extension/router.go:27` + `backend/cmd/server/router.go:741`。
- 回滚点只存元数据与代次路径，不含插件数据文件：`resource_snapshot_json` 14 字节、`migration_state_snapshot_json` 13 字节（`sqlite3 … SELECT length(...)` 实测）。
- 子包安装器无历史版本：`plugins-code/dsh-compatible-host/src/runtime/package-manager.mjs:216-283`（原地 `npm install`）、`:266-271`（失败仅回写 `package.json`）；`ls -a …/dsh-tree` 仅 `.amitia-npm node_modules package-lock.json package.json`。
- 备份范围不含子包：`plugins-code/dsh-compatible-host/amitia-extension.json:2949-2953`（`include: ["dsh-data"]`）+ `backend/internal/extension/plugin_data_backup.go:250-268`（`pluginDataPathDeclared` 对 include 前缀外路径直接拒绝）。

**结论**

DSH 插件回滚应复用两条既有链路，不新造第三套：宿主版本回滚走扩展包代次 + 回滚点（链路 A，已可用），插件数据走 `extension.plugin_data.v1` 备份组件（链路 B）。子包（`dsh-tree`）两级都覆盖不到，是真实缺口；补齐需要插件侧状态同步，超出「不改 `src/**`」的本轮约束，因此未实现，不做「已完成」表述。

**下一步**

1. `amitia-extension.json` 的 `data.pluginData.include` 扩为 `["dsh-data","dsh-tree/package.json","dsh-tree/package-lock.json"]`（只备份清单，不备份 `node_modules`），改后需重新打包并重装生效。
2. 子包级回滚在插件内实现（`dsh-data/storage` 已装记录 + lock 快照 + `npm ci` 回滚），对外只暴露可失败动作。
3. 宿主版本回滚验收补一条「子包与目标版本的 DSH ABI 兼容性判定」，不一致必须明确失败。

#### 23.38.2 P14 事务恢复与残留审计

**现状**

1. 项目里有两套 journal，只有一套在生产链路上：`update` 包的 `extension_lifecycle_journal`/`extension_rollback_plans`/`extension_rollback_steps`/`extension_side_effect_assessments` 实测 **0 行**，且只有 `GET /api/extensions/recovery/scan` 无 Execute 路由；真实链路是 `package_*_saga` + `RecoverPackageOperations`，启动时自动执行。
2. 崩溃恢复语义（指针原子切换、staging、CAS）实现完整，本轮演练全部通过。
3. 真实数据存在 1 条**永不终结**的 `requires_recovery` 陈旧操作，日志已重复 209 次报错。

**证据**

- 死链 vs 真链：`backend/internal/extension/update_api.go:37,100-112`（只有 scan）；`backend/cmd/server/services.go:920`、`backend/internal/extension/runtime.go:45`（启动即 `RecoverPackageOperations`）；`update/recovery.go:171-183`（`executor == nil` 时仍写 completed 步骤）。
- 演练（本轮真实执行）：

  ```text
  $ cd backend && go test ./internal/extension/kernel -run 'TestP14|TestRecoverPackageOperations' -v
  --- PASS: TestP14GenerationPointerCrashRecoversPreviousGeneration (0.10s)
  --- PASS: TestP14StaleGenerationSwitchIsRejectedAfterRecovery (0.09s)
  --- PASS: TestP14InterruptedPrepareLeavesOnlyItsOwnStaging (0.02s)
  --- PASS: TestRecoverPackageOperationsScansIncompleteWithClaimsAndHash (0.34s)
  --- PASS: TestRecoverPackageOperationsPreservesUnownedStaging (0.37s)
  ok  	github.com/u-ai/backend/internal/extension/kernel	1.920s

  $ cd backend && AMITIA_RUN_PACKAGE_FAULT_MATRIX=1 go test ./internal/extension/kernel -run 'TestPackageFaultMatrixRealJournalCracksRecoverDeterministically' -v
  --- PASS: TestPackageFaultMatrixRealJournalCracksRecoverDeterministically (1.94s)
      (install / update / rollback / uninstall 四个子用例全 PASS)
  ```
- 残留审计（只读脚本，`findings=1`，退出码 1）：非终态步骤 0、租约 0、一致性告警 0、staging 全空、回滚点悬空 0，唯一残留是 1 条 `requires_recovery` 操作。
- 该操作的 8 个持久化步骤**全部 completed**，却因「当前代次 ≠ 目标代次」被判 `recovery_manual`：`error_detail=update generation reconciliation failed: unexpected current generation generation-ab60876e-…`，`target_generation=generation-6a1e7649-…`（0.1.84），而实例已到 0.1.314。
- 反复重试证据：`grep -c 'package operation recovery failed'` 在 `AmitiaData/logs/electron.20261115.002.log`…`20261118.001.log` 合计 209 条；最近一条 `2026-11-18T05:09:49.754Z`，后随 `Core continuing with package operations quarantined for recovery`。

**结论**

P14 的「事务恢复」机制本身可用，且 Core 启动不被残留阻塞（符合验收第 1 条）；但恢复器缺少「该操作已被后续更新超越」的终结判定，导致 (a) 每次启动重复报错、(b) 操作表永久留非终态记录、(c) 「无残留」验收无法通过。`update` 包 journal 属并行死链，不应作为 P14 验收对象。

**下一步**

1. 在 `recoverPackageOperation` 增加「代次已超越 + 全部步骤 completed ⇒ 以 `superseded` 终结（保留原始 error_detail）」判定，并补二次扫描无残留用例。
2. 把 `scripts/audit/dsh-plugin-residual-audit.py` 的检查项接入内核自检，让非终态操作/步骤/租约/指针不一致/悬空回滚点/staging 残留成为可回归失败项。
3. 清理 `update` 包从未写入的 journal 死链与 `executor == nil` 的假成功路径。

#### 23.38.3 跨平台能力矩阵

**现状**

1. 插件声明平台 `windows/linux/macos/android`（无 iOS），与 `catalog/platform-support.v1.json` 的 ios `not-declared` 一致。
2. core 侧 Host Service **无平台分支**：`host_api_routes.go` 内 `runtime.GOOS` 出现 0 次。
3. 平台差异集中在四处：Node 二进制布局、扩展 UI 入口类型（`web_module` vs `mobile_web_runtime`）、目录选择的实现位置（Electron IPC vs SAF，且都未暴露给扩展）、Android 的 PRoot 文件系统语义。

**证据**

| 能力 | macOS arm64 | Windows x64 | Android arm64 | 证据 |
| --- | --- | --- | --- | --- |
| `host.conversation.*`（list/read/search/create/delete/scope/message.send/append/turn.interrupt/await/archive/pin） | 可用（archive/pin 已在原生页真实点击验证） | 可用（代码级，未实机） | 可用（代码级，PRoot 内设备本地会话） | `host_api/types.go:58-69`；路由 `host_api_routes.go:2736,2788,2830,2882,4855,4862,4869,4876,4883,4890,4897,4911`；`runtime.GOOS` 计数 0 |
| `host.skill.list` | 可用（实测返回 `{"items":[]}`） | 可用（代码级） | 可用（代码级） | `host_api/types.go:72`、`host_api_routes.go:4904`、`permission_mapping.go:66` |
| `host.directory.list/create` | 可用（宿主 fs） | 可用（宿主 fs） | **语义不同**：解析 PRoot 客户机 fs，非 SAF 文档树 | `host_api/types.go:70-71`、`host_api_routes.go:4918,4925`、`kernel_host_adapters.go:1265,1308`（`os.ReadDir`/`os.Mkdir`） |
| `host.directory.pick`（交互式） | 缺 | 缺 | 缺 | `backend` 全仓无该方法；插件侧明确失败 `src/runtime/host-services.mjs:363-369` |
| 平台内已有目录选择 | Electron 原生对话框（未暴露给扩展） | 同左（未实机） | SAF `workspace.PickSAFDirectory`（仅 workspace 层） | `desktop/src/main/ipc-handlers.ts:190,228,284`、`desktop/src/preload/index.ts:52-59`；`backend/internal/workspace/service.go:80-97`、`backend/cmd/server/services.go:363`（仅 `AndroidNativeBridge != nil` 时装配） |
| `host.ui.dialog` | 可用 | 可用（同一渲染进程，未实机） | 可用 | `front/src/composables/useUIHostSSE.ts:103,171`；`mobile_app/lib/core/ui_runtime/mobile_ui_host_event_client.dart:311`、`mobile_app/lib/core/services/extension_service.dart:362` |
| plugin-host 子进程 | 可用（Core pid 10058 运行中） | 可用（代码级） | 可用（Runtime 包内 linux-arm64 node） | `javascript_main/host.go:394,450-486`；`script_host/resolver.go:172-180`；`nodeenv/candidates.go:17-104`；`device-runtime-manifest.json` |
| task-host 子进程 | 可用（代码级） | 可用（代码级） | 可用（代码级） | `desktop/electron-builder.yml:28-31`；`task_runtime/process_transport.go:100-135` |
| 子进程沙箱 | 三平台一致：`durableSync: true` ⇒ 不加 Node `--permission` | 同左 | 同左 | `amitia-extension.json:62-70`；`javascript_main/host.go:451-453` |
| 扩展 UI 渲染形态 | `web_module`（desktop 入口） | `web_module` | `mobile_web_runtime`（WebView） | `amitia-extension.json:464-505`；`mobile_app/lib/core/ui_runtime/plugin_surface_resolver.dart:17` |

**结论**

DSH 宿主能力在 Amitia core 侧是平台同源的，跨平台风险不在 core 逻辑，而在三处接入面：Node 二进制布局（`node.exe` vs `bin/node`、`node.exe.zip` vs `node.zip`）、扩展 UI 入口分派（桌面渲染进程 vs Android WebView）、目录选择能力（宿主有实现、扩展 ABI 未开，且 Android 语义为 PRoot 客户机路径）。catalog 里 `DSH-HOST-SERVICE-047 skills` 的 reason 仍写「list/snapshot … 当前明确失败」，与 0.1.312 起接入 `host.skill.list` 的实现不一致（`catalog/compatibility.v1.json:497` vs `src/runtime/host-services.mjs:340-356`），属文案滞后。

**下一步**

1. Windows x64 需在真实机器上补一条端到端证据（Electron/Core/plugin-host/会话/UI 槽位），当前只有代码级与打包配置证据。
2. Android 需真机补 DSH 子插件安装 + `mobile_web_runtime` 渲染 + `host.directory.*` 对真实存储可达性；本轮 `adb devices` 无设备。
3. 若要开 `host.directory.pick`，桌面接 Electron `selectWorkspaceDirectory`、Android 接 `PickSAFDirectory`，统一为一个可失败 Host 方法（而不是两套平台特例）。
4. 刷新 `compatibility.v1.json` 中 `skills` 的 reason 文案。

仍未完成（目标保持未达成）：`conversation.chat.node` 正式适配、`host.directory.pick`（交互式目录选择；`host.directory.list/create` 已在 core 侧存在但插件未接线）、DSH 子包（`dsh-tree`）历史回滚与备份覆盖、陈旧 `requires_recovery` 操作终结判定、Windows x64 与 Android 真机端到端证据。

### 23.39 并行推进结果（2026-11-18）

四个并行子代理的产出与集成结论：

- 宿主目录能力：内核新增 `host.directory.list`（`{path?,limit?}` → `{path,entries[{name,path,kind,size,modifiedAt}]}`）与 `host.directory.create`（`{path,name?}` → `{path}`），复用既有 `workspace.read`/`workspace.write` 权限，拒绝相对路径与 `..` 逃逸，错误原样返回。实测 `directory-list` 返回 `/Users/untrammelled` 前 5 项，真实可用。
- 桌面目录选择：新增通用 IPC `amitia:dialog:select-directory`（`showOpenDialog` + openDirectory/createDirectory），预加载暴露 `window.amitiaDesktop.selectDirectory()`，前端类型与 `uiWorkspace.pickDirectory` 均接上；实测页面上该函数存在。
- 运行时宿主服务：`agents/approval/fs/llm/schedule/sessionQuery/userQuestions` 由占位改为真实宿主调用或精确缺口（无 `host.session.*`、无 `host.fs.watch/patch` 等），缺口错误保留 method 与入参。
- 客户端服务：`layout/sessions/uiWorkspace/workspaces` 改为读取宿主 `conversation-list` 投影与真实事件窗口；`uiWorkspace` 新增 pick/list/create/archive/unarchive。
- 节点渲染：查明 DSH 侧真实注册点为 `conversation.chat.node` 的 `steering` 与 `tool-call` 两个键；前端 `conversationNodes` 只存在于插件上下文，已在 `BrowserClientPluginRuntime` 增加公共 `registerConversationNode(pluginId,key,component,definition,options)`，并按 `ProgrammaticConversationNodeDefinition` 契约补齐 `match/create/update/project`，同时为列表槽位补 `cellId`（否则 `list slot chat.conversation.node requires id`）。修复后 DSH 的 `dsh-client-ui-chat` 与 `dsh-client-ui-tool` 不再装载失败，页面无异常。
- 头部动作：把 `conversation.session.header` 从 skip 名单放开时，DSH 的「归档会话 / 置顶会话」按钮会渲染出来，但同一组按钮重复出现两次（两个 DSH 包各自注册同一宿主槽位），因此回退为屏蔽，保持原生头部单份；去重方案待定。
- P8/P14/跨平台矩阵：见 `docs/architecture/dsh-p8-p14-crossplatform-20261118.md`。P8 结论为复用扩展包代次回滚（回滚点 306、悬空 0），缺口是 `dsh-tree` 子包不在 `data.pluginData.include` 且无版本历史；P14 演练 5 用例全 PASS，审计发现 1 处更新包 journal 全 completed 却永挂 `requires_recovery`。

验证：插件测试 298/298；`go build ./...`、`go vet ./cmd/server/... ./internal/extension/...`、内核包测试通过；前端 `vue-tsc --noEmit`、桌面 `tsc -p tsconfig.json --noEmit` 通过；插件 0.2.5 生效，页面 `messages=14 / composer=1 / slots=21`、无异常、`directory-list` 可用。

### 23.40 头部去重与已知回归（2026-11-18）

- 去重方案：`conversation.header` 允许进入宿主 `chat.conversation.header`，并加入 `DSH_SINGLE_ADMIT_SLOTS` 只准入第一个贡献；`conversation.session.header` 保持屏蔽（它是 DSH 头部内部的子槽，放开会让同一个 DSH 头部渲染两份）。
- 实测结果：原生页头部只出现一组「归档会话 / 置顶会话」，无重复；`messages=14 / composer=1`，无异常。
- 更正：此前记录的「按钮点击不发出宿主调用」是探测时机问题（在会话未绑定完成前点击）。重新在会话就绪后实测：点击「置顶会话」写入 `pinnedAt=2026-11-18T06:53:39Z`；点击「归档会话」把会话移出活跃列表且归档列表计数为 1，反向点击可回到活跃列表。头部动作链路可用。
- 遗留缺陷：DSH 头部按钮的标签不会随宿主状态刷新（宿主侧已取消归档后仍显示「取消归档」），因为贡献只在注册时读取一次工作区快照；需要在 `conversation-archive` / `conversation-pin` 成功后触发 slot 重渲染或 `notifySlot` 到 `chat.conversation.header` 及其子槽。

### 23.41 头部动作实测与标签刷新（2026-11-18 晚）

- 头部单份问题已解决：`conversation.header` 准入宿主 `chat.conversation.header` 并用 `DSH_SINGLE_ADMIT_SLOTS` 限制为唯一贡献，`conversation.session.header` 保持屏蔽。DOM 实测只有一组「归档会话 / 置顶会话」。
- 动作链路实测可用：点击「置顶会话」写入 `pinnedAt=2026-11-18T06:53:39Z`；点击「归档会话」后 `inActive=false`、归档列表计数 1；反向点击可回到活跃列表。
- 标签刷新：新增 `window.__dshSlotNotifiers` 与 `notifyHostSlots()`，归档/取消归档/置顶后都会刷新槽位；同时新增 `__dshLiveCatalogs`，把 `sessions` / `workspaces` 的 `getSnapshot/subscribe` 暴露给桥接层，`useSessions` / `useWorkspaces` 改为 `useSyncExternalStore` 订阅实时快照，归档后标签能立即变为「取消归档」。
- 遗留：在同一次会话内取消归档后，按钮标签仍停留在「取消归档」（宿主状态已恢复 `inActive=true`，刷新页面后标签正确回到「归档会话」）。说明标签来自 DSH 自身组件内状态而非我们提供的快照；后续要么在取消归档后强制重挂载头部贡献，要么把该状态纳入会话快照字段。

### 23.42 头部标签遗留问题的定位（2026-11-18）

- 已做：`__dshBumpSlots()` 触发 React 根重挂载、`setConversationFlags` 把 `archived/pinned` 写进会话快照、归档/取消归档/置顶/取消置顶后调用 `refreshConversationData` + `notifyHostSlots`。
- 实测：归档后标签立刻变「取消归档」；反向后宿主状态正确（`inActive=true`、`archivedIds=[]`），但标签仍是「取消归档」，重挂载与刷新会话快照都不能纠正。
- 定位：页面上 DSH 会话 id 是 `8a6cbe2f-…`，而归档标记写入的 key 是我们投影用的 `536b868e-…`（conversationId），二者没有建立映射；DSH 头部读的是它自己 session 维度的归档状态，因此拿不到我们写入的 false。localStorage 中也没有归档字段可对齐。
- 下一步：建立 `sessionId ↔ conversationId` 双向映射（会话绑定层已有 binding，但未导出别名表），把归档/置顶标记同时写到两个键；或在归档/取消归档后直接调用 DSH 工作区 store 的会话刷新入口。

### 23.43 头部标签无法回收的最终结论

- 追加手段：`setConversationFlags` 写入会话快照、会话目录合并归档列表并暴露 `archived/pinned` 布尔、命令完成后立即 + 900ms + 2500ms 三次触发 `__dshBumpSlots()` 重挂载。
- 证据：拦截 `__dshBumpSlots` 计数为 0（说明 DSH 头部点击走的是它自己的命令通道，不经过我们的 `archiveSession/unarchiveSession`），重挂载后标签仍为「取消归档」，而宿主 `inActive=true`、`archivedSessionIds=[]`、会话快照 `archived=false`。
- 结论：标签由 DSH 归档按钮组件自身的交互状态驱动，只在页面重载后按数据重建；不是我们的数据源问题。要彻底解决必须改 DSH 组件（不可行）或在归档/取消归档后强制整页重载（体验差），因此保持现状并在本文档记录。
- 影响面：仅按钮文案在同一次会话内不回收；功能与状态正确（点击可双向切换，重载后文案正确）。

### 23.44 P14 陈旧 requires_recovery 终结判定

- 问题：审计发现某次更新操作 8 个步骤全部 completed，但操作记录永远停在 `requires_recovery`，每次恢复扫描都会重试并重复写日志。
- 实现：`recoverPackageOperation` 在进入恢复链前先调用 `resolveSupersededPackageOperation`：读取该扩展当前代次指针，若指针指向另一个操作（`current.OperationID != 当前操作`），且该操作状态为 `completed`、扩展相同、`TargetVersion` 相同，则判定当前操作已被超越，直接 `SetOperation(status=completed, step=superseded, detail="operation superseded by completed operation <id>")` 并返回，不再重试。
- 判定保守：目标版本不一致（例如指针停在 2.1.0 而陈旧操作目标是 2.0.0）时不会误判，仍走正常恢复链。
- 测试：新增 `internal/extension/kernel/p14_supersede_termination_test.go` 两个用例（同版本被超越 → true 并返回新操作 id；版本不同 → false）；`go test ./internal/extension/kernel/` 全包通过（96s），既有 5 个 P14 演练用例保持通过。
- Core 已按当前源码重建（darwin arm64 + flat 副本），完整服务重启后 `/api/health` 200。

### 23.45 P8 子包版本快照（部分闭环）

- 现状：插件包本身不含 `dsh-tree`（实测 `.amitiax` 仅 48 个文件、23MB，`grep dsh-tree` 命中 0），子包是运行时 npm 安装的，因此包代次回滚覆盖不到子包版本。
- 实现：`installDshPackage` 成功后调用新增的 `writeDshTreeLock(workspace, pluginDataRoot)`，把安装树 `package-lock.json` 里的全部包版本写入 `dsh-data/dsh-tree-lock.json`（按名字排序，附 `generatedAt`）；`dsh-data` 已在 `pluginData.include` 中，随插件数据备份/回滚走。
- 实测：页面内触发一次 `install` 动作（`npm:@deepseek-ai/dsh-client-ui-chat`）后，`dsh-data/dsh-tree-lock.json` 生成，内容 206 个包版本（如 `@deepseek-ai/cordis-plugin-group@1.1.0`），17.8KB。
- 测试：`tests/package-manager.test.mjs` 新增「records the installed DSH subpackage versions into plugin data」，断言锁文件路径与排序后的包列表；插件测试 299/299 通过。
- 仍缺：按该快照**恢复**子包版本的动作（当前只记录不自动还原），需要 npm 侧的 deterministic 还原流程。

### 23.46 P8 子包按快照确定性还原（补齐）

- 快照：`writeDshTreeLock` 除 `dsh-tree-lock.json`（全部包版本）外，还把安装树的 npm `package-lock.json` 原样复制为 `dsh-data/dsh-tree-package-lock.json`（128KB），并对每个包记录 `optional` / `os` / `cpu`。
- 还原：新增 `restoreDshTreeLock`。有 npm 锁快照时把快照写回 `dsh-tree/package-lock.json` 并执行 `npm ci`（失败时回滚原锁文件），无快照时退回按直接依赖精确版本单次 `npm install`；还原后重新校验全部包版本并重写锁。
- 一致性判定：平台相关的可选包（如 `@deepseek-ai/node-addon-system-linux-x64`、`@koromix/koffi-freebsd-*`）在本机未安装属正常，按 `optional` 标记从差异中排除，不再误报。
- 实测（Electron 页面内调用 `restore-tree`）：首次还原 `pins=101 mismatched=36`（均为未安装的平台可选包）并重建带 optional 标记的锁；再次还原 `pins=101 mismatched=0`，说明子包集合可按快照确定性恢复。
- 测试：`tests/package-manager.test.mjs` 覆盖锁写入路径与按锁还原入参；插件测试 300/300。

### 23.47 directoryPicker 与 sessionSkillCatalog 接真实宿主

- `createDirectoryPickerService(call)`：`list(path)` 调 `host.directory.list`（默认 limit 200，支持 AbortSignal），`createDirectory(path, name)` 调 `host.directory.create`，缺少 path 时抛带原始入参的错误；`pick` 保持缺口（原生对话框由渲染进程 `window.amitiaDesktop.selectDirectory` 提供，宿主侧无对应方法）。
- `createSessionSkillCatalogService(call)`：`refresh()` 调 `host.skill.list` 并规范化条目，`list()`/`snapshot()` 返回缓存与状态，拉取失败时把原始 error 存进 `snapshot().error` 并向上抛出。
- 测试：`tests/host-services-catalog-seams.test.mjs` 两个用例改写为「浏览/建目录走宿主目录接口」「技能目录从 host.skill.list 拉取」，插件测试 300/300 通过。
- 运行态：插件 0.2.26 安装并生效，完整服务重启后 `/api/health` 200。

### 23.48 发送链路回归确认（2026-11-18）

- 复核背景：客户端服务被多轮改写后，需要确认 DSH 发送路径仍然可用。
- 步骤与证据：导航到 `#/chat?conversationId=536b868e…` 并强刷后，DOM `messages=14 / composer=1`、`__dshHostCommand` 为函数、会话 `inActive=true` 且不在归档列表；调用 `conversation-message-send`（字段名是 `content`，不是 `text`，用错会被 schema 以 `additionalProperties(text)` 拒绝）返回 `requestId=dsh-mwluqjh3-1fkbw17z`，会话消息数由 7 变为 8。
- 结论：DSH → Amitia 的发送链路正常，参数契约以 manifest 的 `content` 为准。

### 23.49 sessionQuery.readEvent 落地

- 原状：`readEvent` 直接抛缺口（「Amitia 没有 DSH SessionEventWindow 的窗口与游标查询能力」）。
- 实现：基于既有 `sessionView(sessionId)`（活会话取 `agents.get(id).session.events`，持久会话从 `host.state` 的 `dsh-session:<id>` 还原）按 `seq` 查找事件；`seq` 非数字抛 `DSH_SESSION_EVENT_SEQ_INVALID`，未命中抛 `DSH_SESSION_EVENT_NOT_FOUND` 并带上 sessionId 与 seq。
- 测试：从两个「必须不可用」用例中移除 `readEvent`，插件测试 300/300 通过；插件 0.2.27 安装生效，完整服务重启后 `/api/health` 200。
- 仍未覆盖：`observeSession` / `inspectSession` / `searchEvents` / `filterSessions` / `filterEvents` / `traceSession` / `traceEvent`，它们分别需要持续事件流、DSH Session 句柄、跨会话事件检索、目录过滤谓词与 fork 血缘，Amitia 侧没有对应数据。

### 23.50 P14 终结判定的集成验证

- 新增 `TestP14RecoveryFinalizesSupersededOperation`：构造 `op-stale-recovery`（`requires_recovery`）与 `op-next`（`completed`，代次指针指向它，目标版本相同），直接调用 `recoverPackageOperation`，再读回操作记录。
- 断言：调用无错误，记录变为 `status=completed`、`current_step=superseded`——证明终结分支真的在恢复入口生效，而不只是 `resolveSupersededPackageOperation` 这个纯函数返回 true。
- `go test ./internal/extension/kernel/ -run TestP14` 全组通过（1.9s），`go vet ./internal/extension/kernel/` 无输出。

### 23.51 fs.watch 以轮询落地

- 原状：`fs.watch` 直接抛缺口（缺 `host.workspace.watch`）。
- 实现：不新增后端能力，改为基于既有 `host.workspace.read` 的 `file_stat` 轮询：默认 1000ms，可用 `options.intervalMs` 调整；首次采样只记录基线，之后 `size + modifiedAt/mtime/updatedAt` 变化才触发 `onChange`；回调抛错或读取失败交给 `options.onError`，未提供时以 `[original-error]` 输出原始错误；返回 `{ path, dispose() }` 句柄。
- 测试：`fs 的编辑与补丁保持缺口，watch 用轮询检测变更`——构造两次不同 `file_stat` 结果，5ms 间隔下 80ms 内至少触发 1 次变更回调，`path` 正确；`editText`/`patchText` 仍断言为缺口（`host.workspace.edit` / `host.workspace.patch`）。插件测试 300/300。
- 运行态：插件 0.2.28 安装并生效，完整服务重启后 `/api/health` 200。目录条目 `fs` 原因已同步为「watch 轮询实现，editText/patchText 仍缺」。

### 23.52 fs.editText 按 FsEditRequest 落地

- 契约来源：`@deepseek-ai/dsh-fs` 的 `FsEditRequest { oldString, newString, replaceAll }` 与 `FsEditOutcome { version, before, after }`（`lib/types/types.d.ts` 136-156 行）。
- 实现：`host.workspace.read` 的 `file_read_text` 取原文 → 统一 `\r\n` 为 `\n` → 统计 `oldString` 出现次数；0 次与「多次且 `replaceAll=false`」都抛带原始入参的错误；替换后用 `host.workspace.write` 的 `write_file`（overwrite + atomic）写回；返回 `{ version: 写入结果版本, before, after }`。
- 测试：`fs.editText 按字面量改写，patchText 保持缺口，watch 轮询检测变更`——断言 `hello world` → `hello DSH`、`before` 为 LF 归一化原文、`version=v2`；`patchText` 仍断言 `host.workspace.patch` 缺口；`watch` 轮询仍通过。插件测试 300/300。
- 运行态：插件 0.2.29 安装生效，完整服务重启后 `/api/health` 200（端口 62619）。
- 仍缺：`fs.patchText`（`host.workspace.patch` 的 hunk 应用语义）。

### 23.53 fs.patchText 落地，fs 服务转为完全实现

- 契约来源：`@deepseek-ai/dsh-fs-patch` 的 `PatchHunk { lineNumber, context?, oldLines[], newLines[], isEndOfFile }`。
- 实现：读原文并按 LF 归一化 → 按 `lineNumber` 从大到小处理 hunk（避免行号漂移）→ 在全文里找与 `oldLines` 完全匹配且距锚点最近的块（`oldLines` 为空视为纯插入）→ `splice` 替换 → 用 `write_file`（overwrite + atomic）写回，保留原文件是否以换行结尾；未匹配时抛带 hunk 序号、lineNumber 与 oldLines 的错误。
- 测试：`fs.editText 与 patchText 都走宿主读写，watch 轮询检测变更`——`hello world\nsecond line\n` 用 `{lineNumber:2, oldLines:["second line"], newLines:["pátched line"]}` 得到 `hello world\npátched line\n`，`version=v2`。插件测试 300/300。
- 目录：`fs` 由 partial 改为 implemented，Host 计数变为 implemented 17 / partial 11 / pending 1 / delegated 27 / unavailable 13，覆盖率 90.6%。
- 运行态：插件 0.2.30 安装生效，完整服务重启后 `/api/health` 200（端口 63209）。

### 23.54 回归修复：React 以打包期 ESM vendor 提供

- 根因：`restore-tree`（npm ci）后 `react-dom@18.3.1` 变成由模块路由单独加载，而它只有 CJS 构建（`exports` 无 `import`/`browser`），浏览器侧缺 `module`/`require`。
- 方案（仅插件侧，未改宿主）：新增 `scripts/build-vendor-react.mjs`，在打包前用 esbuild 把 `react` + `react-dom` + `react-dom/client` + `react/jsx-runtime` 打成一份 CJS，再包成「全局单例工厂 + 具名 ESM 导出」写入 `src/runtime/vendor/react-browser.mjs`（48 个导出，含 `createElement`/`useState`/`createRoot`/`jsx`）。单例通过 `globalThis.__AMITIA_REACT_VENDOR__` 保证只有一份 React 实例，避免双实例的 Invalid hook call。
- 命中规则：`readInstalledClientSource` 对 `react`、`react-dom`、`react-dom/client`、`react/jsx-runtime`、`react/jsx-dev-runtime` 直接返回 vendor 产物，不再读 npm 树。
- 构建集成：`scripts/build-package.mjs` 在清理 staging 前调用 vendor 生成脚本；`src/runtime/vendor` 本来就整体拷进包内。
- 测试：新增 `serves the bundled React ESM vendor for react packages`（断言 `vendor=true`、含 `createElement`/`createRoot` 与单例标记），插件测试 301/301。
- 实测（Electron，插件 0.2.33）：页面恢复，`messages=16 / composer=1 / header=["归档会话","置顶会话"]`、`__dshHostCommand` 可用、`directory-list` 正常、无异常；归档/取消归档双向仍正确（`inActive` false → true）。
- 遗留：`readDshClientSource` 里的 `process.env.NODE_ENV` 字符串替换与「非 DSH 包入口回退」是排查期间的过渡代码，React 走 vendor 后已无实际作用，待清理。

### 23.55 host.model.defaults 落地，llm.resolveCallConfig 接真实默认值

- 后端：新增宿主方法 `host.model.defaults`（`MethodModelDefaults`），权限沿用 `provider.invoke`，能力矩阵登记为复用；`ModelDefaultsProvider` 接口由 `modelGatewayAdapter` 实现——读 `chat.Service.ListModels()` 里 `IsActive == 1` 的配置，返回 `{provider, model, temperature, maxTokens, maxOutputTokens, contextWindow}`；未配置启用模型时返回原始错误 `no active chat model configured`。
- 装配：`newModelGatewayAdapter` 返回值改为具体类型 `*modelGatewayAdapter`，以便同时满足 `ModelGatewayPort` 与 `ModelDefaultsProvider`；`services.go` 追加 `WithModelDefaultsProvider(...)`。
- 插件：`llm.resolveCallConfig(config)` 由缺口改为合并 HTTP：显式字段优先，缺省时用宿主默认值补齐 `temperature` / `maxOutputTokens`（回退 `maxTokens`）/ `contextWindow`，`provider`、`model` 同理。
- 测试：新增 `llm.resolveCallConfig 用 host.model.defaults 补齐默认值`（断言默认值合并与显式覆盖，且两次调用各触发一次 `host.model.defaults`），插件测试 302/302；`go build ./...`、`go vet ./cmd/server/... ./internal/extension/kernel/...` 无输出。
- 运行态：Core 按新源码重建，插件 0.2.35 安装生效，完整服务重启后 `/api/health` 200，页面 `messages=16 / composer=1 / header=["归档会话","置顶会话"]`。
- 仍缺：`llm.fileRequestText`（附件引用文本）。

### 23.56 host.model.defaults 运行态验证

- 为取到端到端证据，插件新增动作 `model-defaults`（直接 `host.call("host.model.defaults", {})`），便于在页面内验证路由、鉴权与适配器。
- 实测（Electron，插件 0.2.36）：`{"provider":"deepseek","model":"deepseek-chat","temperature":0.7,"maxTokens":4096,"maxOutputTokens":0,"contextWindow":128000}`——来自当前启用的聊天模型配置。
- 注意：该环境 `maxOutputTokens` 为 0，`llm.resolveCallConfig` 会回退到 `maxTokens`(4096)，符合实现。
- 回归：页面 `messages=16 / composer=1`，发送链路仍正常（消息 8→9）。
- 测试：插件 302/302；`go test ./internal/chat/`（16.3s）与 `go test ./internal/extension/kernel/`（93.9s）均通过。

### 23.57 skills 状态校正为已实现

- 依据：`createSkillsService` 的 list/snapshot 合并宿主 `host.skill.list` 与本地注册表，register/registerProvider/get 为本地注册表，refresh 失败时原始 error 进 `snapshot().error`；`tests/host-services-catalog-seams.test.mjs` 的「skills 的 list 与 snapshot 合并宿主目录与本地注册」与「skills 本地注册与读取真实生效」覆盖。
- 目录：`skills` 由 partial 改为 implemented。Host 计数变为 implemented 18 / partial 10 / pending 1 / delegated 27 / unavailable 13，覆盖率 **91.3%**。
- 复核：页面 `messages=18 / composer=1 / header=["归档会话","置顶会话"]`，`directory-list` 正常，无异常；插件测试 302/302；`/api/health` 200。

### 23.58 approval / userQuestions 补测试并转为已实现

- 新增 `tests/host-services-approval-questions.test.mjs` 四个用例：
  - `approval.request` 把 `{approved:true}` → `allowed-once`、`{approved:false}` → `rejected`，并断言 `host.approval.request` 的 `toolName` 透传；
  - 宿主返回非法结构（`{ok:true}`）时抛 `Amitia host.approval.request 返回了非法结果`；
  - `userQuestions.ask` 经 `host.ui.dialog` 返回 `{result,dialogId}`，映射为 `{question,selected,dialogId}`；
  - 宿主返回空 `result` 时抛 `Amitia host.ui.dialog 返回了非法结果`。
- 目录：`approval`、`userQuestions` 由 partial 改为 implemented。Host 计数 implemented 20 / partial 8 / pending 1 / delegated 27 / unavailable 13，覆盖率 **92.8%**；插件测试 306/306。
- 新增可复用的桌面冒烟脚本 `artifacts/dsh-acceptance-20261117/smoke.mjs`（导航→强刷→检查桥接、错误边界、消息数、composer、头部件、`directory-list`），一条命令输出 JSON 结论。
- 运行态：插件 0.2.37 生效；冒烟结果 `{"bridge":"function","errorBoundary":false,"messages":18,"composer":1,"header":["归档会话","置顶会话"],"listing":{"path":"/Users/untrammelled","count":3}}`。
- 仍 partial：`schedule`（未写用例）、`agents`、`sessions`、`subagents`、`sessionQuery`、`llm`、`directoryPicker`、`directoryPickerController`。

### 23.59 schedule 补测试并转为已实现

- 新增 `tests/host-services-schedule.test.mjs` 两个用例：
  - `schedule.list` 把宿主目录 `{schedules, states}` 映射为 DSH 条目（`id`/`kind`/`status`/`target.sessionId`），并按 `active` 过滤；返回结构为 `{items,shown,total,limit,offset}`；
  - `schedule.create` 以 daily 时间与 `Asia/Shanghai` 时区构造定义，断言 `host.schedule.create` 收到 `name`/`target.type=session_prompt`，并回读目录得到 `title`/`target.sessionId`/`status`。
- 目录：`schedule` 由 partial 改为 implemented。Host 计数 implemented 21 / partial 7 / pending 1 / delegated 27 / unavailable 13，覆盖率 **93.5%**；插件测试 308/308。
- 说明：`cancel/run/view/history/edit/deleteInactive` 的代码路径已接通 `host.schedule.*`，本次用例覆盖 list 与 create 两条主路径。

### 23.60 头部归档文案不回收的最终定位

- 尝试：在 `archiveSession` / `unarchiveSession` 里除了 `workspaceCatalog.refresh()` 之外补充 `sessionCatalog.refresh()`（插件 0.2.38），并保留 `notifyHostSlots` 与三次 `__dshBumpSlots`。
- 实测（p6 脚本）：归档后 `labels=["取消归档"]`、`inActive=false`；取消归档后 `inActive=true` 但 `labels` 仍是 `["取消归档"]`；页面重载后恢复为 `["归档会话","置顶会话"]`。
- 定位：`@deepseek-ai/dsh-client-ui-workspace/lib/client.js` 内部维护自己的 `archivedSessionIds`（105 处 `archived` 引用，含 `archivedUndo` 撤销缓冲），归档后写入本地状态并展示「取消归档 + 撤销」；取消归档走的是宿主 `conversation-archive(false)`，本地状态未随之清理。
- 结论：这是该 UI 包的本地交互状态，插件侧刷新目录、重挂载贡献都无法回收；页面重载后按数据重建即正确。功能与宿主状态始终正确。
- 影响：仅同一次会话内的按钮文案；已记录为已知非阻塞缺陷。

### 23.61 会话派生（fork）尝试与结论

- 实现：`internal/chat/fork.go` 新增 `ForkConversation`（按 `boundarySequence` 复制消息到新会话，重写 `id`/`conversation_id`，回写 `message_count`）；内核新增 `host.conversation.fork`（`ConversationForker` + 路由 + 能力矩阵 `NEW_MINIMAL`）；`cmd/server` 用类型断言 `chat.Service.(ForkConversation)` 接入；插件新增 `conversation-fork` 动作与 `sessions.fork`。
- 实测：动作通过 schema 后返回 `runtime_unavailable ... host.call host.conversation.fork: internal_error: record not found`。
- 根因：DSH 侧看到的会话是**设备归属的 mesh 资源**（`conversation-list` 走 coordination store），而 `chat.Repository.GetConversation` 查的是本地 chat 表（`id = ? AND deleted_at IS NULL`），两者不是同一存储，所以按 DSH 会话 id 必然查不到。
- 处理：插件 `sessions.fork` 回退为明确的缺口错误，并把根因写进错误文案；后端 `host.conversation.fork` 保留但只覆盖本地 chat 会话，不作为 DSH 映射使用。
- 结论：真 fork 需要在 mesh 业务层做会话复制（复制 conversation 资源与其消息事件），属于新增能力，不是接缝问题。
- 回归：插件 0.2.41 生效，冒烟 `bridge=function, messages=18, composer=1, header=[归档会话,置顶会话]`，插件测试 308/308。

### 23.62 会话派生在 mesh 层打通

- 上一轮结论修正：不是“fork 不可做”，而是不能走本地 chat 表。
- 实现：`cmd/server/mesh_owned_chat_actions.go` 新增 `conversation/fork`——读源会话资源，`p.store.List(ctx,"message",role,false)` 取消息资源并按 `body.conversationId` 过滤，为每条消息生成新 `id`（`<forkId>-message-%06d`）、重写 `conversationId/characterId/ownerId` 并写入 `forkOrigin`；新会话继承源会话字段并改写 `id/title/createdAt/updatedAt/messageCount`；一次 `Apply` 提交（请求号命名空间 `fork-chat/<sha256>`）。
- 装配：`WithConversationForker(kernelConversationCreator)` 走 `engine.ManageChat("conversation/fork")`，不再使用只覆盖本地 chat 的 `conversationForkAdapter`。
- 实测（Electron，动作 `conversation-fork`）：返回 `conversationId=d0b47571-40e2-4187-9b3c-2edb8d267695`、`forkedFrom=536b868e…`、`messageCount=9`、`title="fork 验证"`，`inList=true`，随后 `conversation-delete` 清理成功（`cleanup=deleted`）。
- 插件：`sessions.fork` 恢复为真实调用 `conversation-fork`（`command` → 宿主路由），并刷新会话目录；插件 0.2.42 生效。
- 回归：插件测试 308/308；冒烟 `bridge=function, messages=18, composer=1, header=[归档会话,置顶会话]`。

### 23.63 mesh 层 fork 的自动化回归

- 新增 `cmd/server/mesh_owned_chat_fork_test.go`：种入一个会话资源与两条消息资源，调用 `managedConversationAction` 的 `conversation/fork`，断言：
  - 返回 `forkedFrom=conv-1`、`messageCount=2`、`conversationId` 非空且不同于源；
  - 按新会话 id 回查消息资源得到 2 条，且每条都带 `forkOrigin`。
- 结果：`go test ./cmd/server/ -run TestOwnedChatForkCopiesMessagesIntoNewConversation` → ok（1.76s）。
- 已知既有失败（与本次无关）：`TestOwnedChatCleanupPreservesMemoriesAndDeletesCurrentHistory`、`TestOwnedChatCleanupRejectsChangedPreviewAndRole`、`TestOwnedChatCleanupAllowsEmptySelectionAndCreatesBackup`，属陈旧用例（空数据下仍期望清理预览成功）。
