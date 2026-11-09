# 内置模块迁移矩阵

## 协议层

| 能力 | 状态 |
| --- | --- |
| 单一 Manifest | 后端只保存 `plugincontract.Manifest`；旧 `manifest_v1` 包已删除 |
| 运行时领域映射 | 仅保留 Manifest 到 `domain.ExtensionDefinition` 的单向投影 |
| 市场解析 | 管理员市场直接使用统一 Manifest，不再读取 `extension`/`publisher` 旧字段 |
| 源码契约验收 | 自动遍历并验证 `plugins-builtin/` 与 `plugins-code/` 下全部源码 Manifest |
| 源码目录边界 | 固定业务插件位于 `plugins-builtin/`；普通与市场插件位于 `plugins-code/`，并由测试强制校验 |
| 身份插件分发包 | `email`、`calendar`、`phone`、`wallet`、`browser-profile` 已由源码重新构建并签名 |

## 已迁出核心

| 模块 | 插件 | 状态 |
| --- | --- | --- |
| 插件市场、扩展中心与内核入口 | `com.amitia/plugin-market` | `/extensions`、`/kernel` 根路由、市场页面、导航和路由均使用 Tool、UI Provider、Route Registry；宿主固定占位页和重定向壳已删除 |
| 渠道页面与连接 | `com.amitia/channel-*` | 已使用 Service、UI Provider、Route Registry |
| 表情与消息渲染 | `com.amitia/emote` | 已使用 Tool、UI Slot、UI Provider |
| 生活系统 | `com.amitia/lifestyle` | 已使用 Tool、UI Slot、UI Provider |
| 运行时调试 | `com.amitia/runtime-insights` | 已从固定 Vue 路由迁出 |
| BDI 决策可视化 | `com.amitia/runtime-insights` | Web、桌面与移动端均由插件提供 |
| 长期运行维护 | `com.amitia/runtime-insights` | 状态、配置和维护动作均走 Host API |
| 工作区文件与 Git | `com.amitia/workspace` | 固定路由与 Vue 页面已删除，文件、Git 和隔离工作区统一走 Workspace Bridge |
| 聊天导入 | `com.amitia/chat-import` | 固定路由与 Vue 向导已删除，解析、批次管理和设备会话晋升统一走 Import Bridge |
| 提醒与主动任务 | `com.amitia/reminders` | 固定路由与 Vue 页面已删除，任务、队列和历史统一走现有 Reminder Host API |
| 存储维护 | `com.amitia/storage` | 固定路由与 Vue 页面已删除，清理、备份和恢复分别走独立 Storage Host API |
| 归档对话 | `com.amitia/chat-logs` | 固定路由与 Vue 页面已删除，读取、恢复、删除、摘要和导出统一走 Conversation Logs Host API；列表、消息和操作仅接受具有 `archivedAt` 的会话 |
| 记忆 Hub | `com.amitia/memory-hub` | `/memory`、`/knowledge`、`/memory-manager`、`/memory-timeline`、`/episodic`、`/graph`、`/profiles` 固定路由已删除，页面、路由、二级导航和主导航均由 UI Provider 提供 |
| 角色 Hub | `com.amitia/character-hub` | `/characters`、`/character`、角色详情、生活规则、语音、心理状态、团队和世界书固定路由已删除，页面、路由、二级导航和主导航均由 UI Provider 提供 |
| 创意工坊 Hub | `com.amitia/creative-workshop-hub` | 固定 `/creative-workshop` 与桌宠中心入口已删除，工坊页面、路由和主导航由 UI Provider 提供 |
| 游戏中心 Hub | `com.amitia/game-center-hub` | 固定 `/game-center` 路由已删除，中心页面、路由和导航由 UI Provider 提供，能力门禁仍由核心执行 |

## 核心不可替换

| 能力 | 说明 |
| --- | --- |
| Bootstrap、恢复与错误页 | 启动失败时仍能进入宿主恢复边界 |
| UI Provider 修复页 | 用于恢复损坏或冲突的 UI Provider |
| 审批与待处理中心 | 工具、设备和自动化审批是安全边界，保留核心 `/attention` 页面与通知深链 |
| 安装、签名、信任与事务 | 插件不能替换自身安装和权限根 |
| Adapter Broker、Service Contract | 非核心执行语义只能通过受信 Adapter 扩展 |
| 最小聊天、模型、会话与记忆执行边界 | 页面可替换，底层执行协议不可替换 |
| 文件系统和进程监管 | 沙箱、进程树、资源限制留在核心 |

## 待继续迁移

| 模块 | 目标 |
| --- | --- |
| 角色、记忆、知识详情页 | 保留核心数据契约，页面由插件提供 |
| 桌宠中心 | 使用 `.petx` 包别名共享统一插件生命周期 |
| Kernel 中非恢复类诊断页面 | 保留最小修复入口，其余迁移为开发插件 |

## 验收要求

每个待迁移模块必须同时满足：

1. 宿主固定路由和页面源码已删除。
2. 插件 Manifest、权限、数据契约和平台支持完整。
3. 禁用插件后路由、导航、工具和服务全部消失。
4. 第三方插件可在不修改宿主源码的情况下替换该模块。
5. Web、桌面和移动端的支持状态明确且使用同一 Provider 快照。

## 平台支持状态

| 模块 | Web | Windows / macOS / Linux | Android / iOS |
| --- | --- | --- | --- |
| 插件市场与扩展中心 | 支持 `/extensions` 根页面与市场页面 | 支持 `/extensions` 根页面与市场页面 | 市场页面由插件提供；扩展中心根 Tab 仍由移动端宿主壳承载 |
| 工作区 | 支持 | 支持 | 暂不声明，移动端尚无本地 Workspace 业务页面 |
| 聊天导入 | 支持 | 支持 | 暂不声明，移动端尚无对应导入向导 |
| 提醒与主动任务 | 支持 | 支持 | 暂不声明，移动端任务工作台尚未迁移 |
| 存储维护 | 支持 | 支持 | 暂不声明，移动端尚未提供完整备份恢复工作流 |
| 归档对话 | 支持 | 支持 | 暂不声明，移动端归档工作台尚未迁移 |
