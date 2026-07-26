# Extension Kernel v1 已知限制

> 本文档对应第 70 步要求公开的真实已知限制，不隐藏任何项。每个限制都给出影响范围、缓解措施与后续计划。

## 1. Trusted Service 隔离能力

- **范围**：Trusted Service Runtime 用于官方扩展从旧 Go PluginRuntime 迁移，仍以进程内可信方式运行。
- **影响**：第三方扩展无法使用 Trusted Service Runtime；官方扩展在该 Runtime 内不具备进程级强隔离。
- **缓解**：Trusted Service 仅允许官方签名扩展加载，第三方扩展只能使用 JSMain/WASM/Task/MCP/Workflow Runtime。
- **后续计划**：在后续 minor 版本中引入独立进程沙箱与资源配额。

## 2. 平台差异

- **范围**：当前验收环境为 Windows 11，macOS 与 Linux 启动关闭验收在当前发布中标记为 Skipped。
- **影响**：macOS/Linux 上的菜单、托盘、快捷键、IPC 行为未在本次验收中实地验证。
- **缓解**：架构层保持跨平台抽象，Electron 主进程代码统一；测试套件保留三平台 case。
- **后续计划**：在 macOS 与 Linux 环境补测后切换为 Passed。

## 3. 未支持的 UI Slot

- **范围**：Schema UI 当前支持 chat.input、chat.message、chat.sidebar、settings.panel、character.detail、workspace.panel 等核心 Slot；动态浮层、全局覆盖层等 Slot 尚未开放。
- **影响**：依赖未开放 Slot 的扩展无法注入 UI。
- **缓解**：未开放 Slot 在 Contribution Registry 阶段直接拒绝注册，避免运行时错误。
- **后续计划**：按需求评估新增 Slot，并通过 ADR 流程审核。

## 4. 未支持的 Runtime

- **范围**：当前支持 JavaScript Main、Task、MCP、Workflow、WASM、Trusted Service 六类 Runtime；Python Runtime、独立 Native Runtime 暂不支持。
- **影响**：依赖 Python 或自定义 Native 运行时的扩展无法加载。
- **缓解**：通过 MCP Runtime 或 WASM Runtime 提供等价能力。
- **后续计划**：根据生态需求评估引入 Python Runtime。

## 5. 旧包迁移限制

- **范围**：旧 Amitiax 包、旧 Plugin 包、旧 Skill 包迁移到 Manifest v2 时，部分历史字段（如自定义 scripts、未签名 Capability）不自动迁移。
- **影响**：依赖被丢弃字段的扩展需要在迁移后人工补齐。
- **缓解**：迁移工具输出迁移报告，列出未迁移字段；用户可在 Developer Console 中查看并补齐。
- **后续计划**：提供迁移脚本模板与人工补齐指南。

## 6. 开发模式风险

- **范围**：开发模式（DevMode）允许加载未签名扩展、热重载、文件监听，存在安全风险。
- **影响**：开发模式仅适用于本地开发，禁止用于生产。
- **缓解**：DevMode 必须显式启用，默认关闭；启用时在 Developer Console 与状态栏显著提示。
- **后续计划**：增加 DevMode 自动超时与远程连接限制。

## 7. 性能规模

- **范围**：当前性能验收覆盖 100/1000 扩展规模，超过 1000 扩展的规模未验证。
- **影响**：超大规模扩展加载时的启动时间、内存占用、UI 响应未保证。
- **缓解**：建议单用户扩展数量控制在 1000 以内；通过按需加载与懒注册降低开销。
- **后续计划**：在后续版本中验证 5000 扩展规模并优化。

## 8. 在线市场未完成部分

- **范围**：扩展中心支持已安装、导入、更新、开发中、需要处理；在线浏览、搜索、发布、评分等市场功能尚未完成。
- **影响**：用户无法在应用内直接从在线市场安装扩展。
- **缓解**：支持本地 .amitiax 包导入与开发者分发；CLI 提供 pack/sign/verify 用于离线分发。
- **后续计划**：在后续版本中实现在线市场与服务端。

## 9. 移动端未支持

- **范围**：Extension Kernel 当前面向桌面端（Electron），不支持移动端。
- **影响**：移动端无法加载扩展。
- **缓解**：移动端使用内置能力，不依赖扩展系统。
- **后续计划**：移动端支持不在 v1 路线图内。

## 10. 数据库回滚边界

- **范围**：数据库回滚支持 Schema Version 回滚与扩展数据回滚；用户资产（如角色卡、对话历史）的回滚仍依赖应用层备份。
- **影响**：用户资产回滚需要应用层配合。
- **缓解**：发布演练中包含数据库恢复步骤，确保关键数据可恢复。
- **后续计划**：在后续版本中将用户资产纳入扩展系统快照。

## 11. 旧系统物理删除周期

- **范围**：第 66-69 步已将旧 PluginRuntime、旧 Skill 兼容层、旧 Amitiax 安装器、旧数据模型标记为 Deprecated 并从生产主链移除，但物理代码删除按废弃策略分阶段进行。
- **影响**：旧代码仍存在于仓库中，但不参与生产主链。
- **缓解**：通过 legacy_deprecation registry 强制标记，并在 cutover 阶段拦截调用。
- **后续计划**：按废弃策略在下一个 major 版本中物理删除。

## 12. 协议版本兼容性

- **范围**：v1 协议基线已锁定（Extension Kernel v1 / Manifest v2 / Host API v1 / Runtime RPC v1 / Schema UI v1 / UI Contract v1 / SDK v1），但跨版本兼容性策略尚未完整实现。
- **影响**：未来协议升级时，旧扩展可能需要显式迁移。
- **缓解**：每个协议单独版本化，并提供迁移指南。
- **后续计划**：建立 ADR 流程与旧接口删除周期。
