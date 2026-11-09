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
| JavaScript SDK | 宿主受控提供 `@amitia/plugin-sdk`，工具按 Manifest `handlerName` 注册和调用 |
| SDK/CLI 真实链路 | 新工具项目已完成 `init`、`build`、严格 `validate`、`pack`、`install`、`enable`、`disable`、再次 `enable` 和 `uninstall` |
| 数据契约执行 | 插件状态按扩展执行字节/键配额；更新按 `data.backup` 备份，卸载按 `data.uninstall` 保留、删除或导出后删除 |
| 崩溃隔离清理 | 运行时重启耗尽或不可恢复失败时，自动释放该扩展的动态贡献、服务、依赖和缓存 |

## 已迁出核心

| 模块 | 插件 | 状态 |
| --- | --- | --- |
| 插件市场、扩展中心与内核入口 | `com.amitia/plugin-market` | `/extensions`、`/kernel`、扩展包、工作流、MCP、Agent Skills 的页面和路由均使用 Tool、UI Provider、Route Registry；宿主固定业务页已删除 |
| 渠道页面与连接 | `com.amitia/channel-*` | 已使用 Service、UI Provider、Route Registry |
| 渠道中心 | 核心模块 | `/channel-center` 和渠道工作区由核心页面提供；渠道发现、接入和 UI 扩展继续通过 Provider、Service 和 Slot 注入 |
| 设置与个人空间 | 核心模块 | `/settings`、`/my-space`、`/me` 及移动端“我的”页面由核心路由和内置页面提供；扩展仍可通过 Slot、Hook、Service 和 Tool 注入能力 |
| 表情与消息渲染 | `com.amitia/emote` | 已使用 Tool、UI Slot、UI Provider |
| 生活系统 | `com.amitia/lifestyle` | 已使用 Tool、UI Slot、UI Provider |
| 运行时调试 | 核心模块 | 核心页面保留，状态和诊断能力继续通过 Host API、Service 和扩展 Slot 提供 |
| BDI 决策可视化 | 核心模块 | Web、桌面和移动端使用核心页面，决策信号仍可由扩展注入 |
| 长期运行维护 | 核心模块 | 核心页面维护状态和配置，维护能力继续通过 Host API 暴露 |
| Kernel 管理页 | 核心模块 | WASM、Hook、可信服务、任务、事件、调度、桌面贡献、更新、诊断和开发模式保留核心入口与扩展贡献 |
| 工作区文件与 Git | 核心模块 | 文件和 Git 工作区使用核心页面，Workspace Bridge、Service 和扩展入口继续保留 |
| 聊天导入 | 核心模块 | 核心导入向导保留，解析、批次和设备会话晋升继续经过 Import Bridge |
| 存储维护 | 核心模块 | 核心存储维护页面保留，清理、备份和恢复继续通过 Storage Host API |
| 归档对话 | 核心模块 | Web `/logs` 与移动端 `/chat-logs` 使用核心页面和 Owner 会话服务；读取、恢复、删除、摘要和导出仅接受具有 `archivedAt` 的会话 |
| 创意工坊 Hub | 核心模块 | `/creative-workshop`、角色卡和桌宠入口由核心页面提供，工坊扩展继续通过 Provider、Service 和 Slot 注入 |
| 游戏中心 Hub | 核心模块 | `/game-center` 和插件详情由核心页面提供，游戏扩展继续走插件生命周期，能力门禁由核心执行 |

## 核心不可替换

| 能力 | 说明 |
| --- | --- |
| Bootstrap、恢复与错误页 | 启动失败时仍能进入宿主恢复边界 |
| UI Provider 修复页 | 用于恢复损坏或冲突的 UI Provider |
| 审批与待处理中心 | 工具、设备和自动化审批是安全边界，保留核心 `/attention` 页面与通知深链 |
| 安装、签名、信任与事务 | 插件不能替换自身安装和权限根 |
| Adapter Broker、Service Contract | 非核心执行语义只能通过受信 Adapter 扩展 |
| 最小聊天、模型、会话与记忆执行边界 | 页面可替换，底层执行协议不可替换 |
| 统筹模式 | 本地模式与云端统筹模式属于 Device Mesh/Core 权限、数据归属和路由规则，不拆成插件；保留扩展事件、服务与第三方接入点 |
| 角色模块 | 角色、详情、团队、世界书、语音、心理状态和生活规则保留为核心业务模块；保留 UI Slot、Hook、Service、Tool 和第三方 UI 注入 |
| 记忆模块 | 记忆总览、知识库、图谱、时间线、情景记忆和用户画像保留为核心业务模块；保留 UI Slot、Hook、Service、Tool 和第三方 UI 注入 |
| 主动任务模块 | 主动任务、定时提醒、队列、执行和历史保留为核心业务模块；保留 UI Slot、Hook、Service、Tool 和第三方 UI 注入 |
| 文件系统和进程监管 | 沙箱、进程树、资源限制留在核心 |
| UI Provider 生命周期清理 | 停用、替换和卸载必须按扩展注销 Provider，并释放路由、页面能力与相关快照 |

## 验收收尾

| 项目 | 目标 |
| --- | --- |
| 第三方替换矩阵 | 用测试插件逐项替换官方市场、渠道、页面、工具和服务 |
| 生命周期零残留 | 安装、启用、停用、更新、回滚、卸载后检查路由、导航、Slot、监听器和进程 |
| 正式资产 | 从当前源码基线重新构建并签名全部固定业务插件与普通插件 |

第三方替换的源码级矩阵已覆盖官方页面、市场入口、渠道、消息渲染器和服务提供者。签名包已完成从当前源码重新构建、验签和真实安装生命周期；最终逐项证据见 `docs/audits/plugin-system-rewrite-acceptance-20261109.md`。

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
| 聊天导入 | 支持 | 支持 | 支持 |
| 主动任务 | 核心模块，支持 | 核心模块，支持 | 核心模块，支持主动任务和持续事项 |
| 存储维护 | 支持 | 支持 | 支持设置内的存储维护页面 |
| 归档对话 | 支持 | 支持 | 支持 |
| 渠道中心 | 支持 | 支持 | 支持 |
| 设置与个人空间 | 支持 | 支持 | 设置子页面支持；个人中心主 Tab 由移动端宿主壳承载 |
| 角色与记忆 | 核心模块，支持 | 核心模块，支持 | 核心模块，支持 |
| 创意工坊 | 支持 | 支持 | 子页面由移动端 Host Runtime Provider 提供，主 Tab 壳保留宿主导航 |
| 游戏中心、Kernel 与扩展子页 | 支持 | 支持 | 由移动端 Provider 动态注册，插件停用后对应路由消失 |
| 聊天导入、归档、渠道与桌宠 | 支持 | 支持 | 已接入同一移动端 Provider 快照 |
