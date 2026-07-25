# Amitia 扩展系统重构与 `.amitiax` 插件平台步骤规划

## 一、命名与目标

统一使用 **`.amitiax`** 作为 Amitia 扩展插件包后缀，不再引入 ToolPkg、PluginPkg 等额外命名。

新的 `.amitiax` 不再只是旧版 Skill/Workflow 分发包，而是 Amitia 唯一的统一扩展包，可按需包含：

- 插件运行时代码
- Agent Tools
- Agent Skills
- MCP 定义
- Workflows
- UI Contributions
- Hooks
- 后台任务
- Providers
- WASM 与资源文件

整体实施顺序固定为：

1. 先审计、优化并重构现有 Skill、MCP、Plugin、Workflow、扩展包系统。
2. 再建立统一 Extension Kernel。
3. 最后实现类似 Operit ToolPkg、但适合 Amitia 桌面端的 `.amitiax` 插件平台。
4. 完成迁移后删除旧系统，禁止新旧两套架构长期并行。

---

# 第一阶段：现有系统冻结与完整基线

## 第 1 步：冻结现有扩展系统功能开发

暂停在旧 Skill、MCP、Plugin、Workflow 和扩展包系统上继续增加新功能，避免重构期间继续扩大历史兼容范围。

## 第 2 步：建立现有系统调用链地图

完整整理安装、启用、注册、执行、恢复、卸载等链路，明确各模块之间的真实调用关系和启动顺序。

## 第 3 步：建立数据表与资源归属清单

列出现有扩展相关数据库表、文件目录、缓存、密钥、定时任务和运行记录，明确每项数据由哪个系统创建和管理。

## 第 4 步：划分保留、重写、迁移和删除范围

将现有代码划分为“直接保留”“抽取后复用”“仅用于迁移”“最终删除”四类，形成后续重构边界。

## 第 5 步：补齐现有系统基线测试

为包解析、权限、工具执行、MCP 连接、Agent Skill 加载、Workflow 执行和插件生命周期补充最低限度回归测试。

---

# 第二阶段：优化并重构现有核心能力

## 第 6 步：解除 Skill 概念过载

将“可执行工具”“Agent Skill 指令”“MCP Tool”“Workflow”从统一的 Skill 概念中拆开，重新定义清晰的领域对象。

## 第 7 步：建立统一 Tool/Capability 模型

把内置工具、插件工具、MCP Tool、Workflow 入口统一为可执行 Capability，同时保留每项能力的真实来源。

## 第 8 步：抽取统一执行安全内核

从现有 Executor 中提取参数校验、超时、取消、并发、幂等、Panic 恢复、副作用记录和执行审计能力。

## 第 9 步：重构统一权限代理

将分散在 Skill、Plugin、MCP 和 Workflow 中的权限判断统一到 Permission Broker，按扩展、能力、作用域和调用上下文授权。

## 第 10 步：统一角色、会话和全局作用域

建立唯一 Scope 模型，统一处理全局、角色、会话、临时调用等作用域，删除不同系统间重复的启用与绑定逻辑。

## 第 11 步：统一运行记录与审计模型

合并扩展执行、插件运行、MCP 操作和工作流执行的公共审计字段，形成统一可检索的执行记录体系。

## 第 12 步：建立统一资源所有权模型

为工具、MCP Server、定时任务、事件订阅、文件、缓存、Secret、UI Contribution 和后台任务记录明确所有者。

## 第 13 步：抽取包安全基础设施

保留并重构现有 ZIP 安全解析、路径校验、Checksum、签名、版本比较、原子安装、升级、回滚和卸载能力。

## 第 14 步：抽取 MCP 协议基础设施

保留现有 MCP Client、Transport、OAuth、Discovery、Resources、Prompts、Tasks、Sampling 等协议实现，移除其独立扩展生命周期职责。

## 第 15 步：重构 Agent Skill 加载器

保留 SKILL.md、Frontmatter、资源索引、Token 预算和渐进加载能力，取消将 Agent Skill 包装成伪 SkillDefinition 的做法。

## 第 16 步：重构 Workflow 执行器

保留工作流定义与执行能力，但将其改为统一 Capability 和扩展资源，不再拥有独立安装体系。

## 第 17 步：提取 Plugin 运行保障能力

保留现有插件熔断、Hook 超时、状态 CAS、事件深度限制、命名空间和定时任务隔离设计，移除 Go Factory 作为第三方插件模型。

## 第 18 步：统一启动、恢复和关闭顺序

将 Extension、MCP、Agent Skill、Workflow 和 Plugin 的恢复逻辑收口到单一编排层，避免应用启动层手动维护依赖顺序。

## 第 19 步：清理重复启用状态

区分“包已安装”“模块已启用”“能力可用”“权限已授权”“连接健康”五种状态，删除多个布尔值互相同步的设计。

## 第 20 步：建立旧系统只读迁移接口

为旧 Skill、MCP、Agent Skill、Workflow 和扩展包建立稳定读取接口，供后续迁移使用，不再允许旧系统产生新的结构类型。

---

# 第三阶段：建立统一 Extension Kernel

## 第 21 步：定义 Extension Kernel 核心领域模型

定义 Extension、Package、Module、Contribution、Capability、Runtime、Permission、Scope、Owned Resource 等核心对象。

## 第 22 步：实现唯一扩展生命周期管理器

统一负责扩展的发现、解析、安装、启用、禁用、启动、停止、升级、回滚、卸载和崩溃恢复。

## 第 23 步：实现统一 Contribution Registry

建立统一贡献注册中心，管理 Tools、Agent Skills、MCP、Workflows、UI、Hooks、Providers 和后台服务。

## 第 24 步：实现统一 Dependency Resolver

统一解析扩展依赖、模块依赖、MCP 依赖、工具依赖和宿主版本要求，并处理依赖冲突与卸载引用。

## 第 25 步：实现统一 Runtime Supervisor

负责插件运行时进程或沙箱的创建、监控、超时、终止、重启、熔断、资源限制和日志收集。

## 第 26 步：实现统一 Host API Gateway

所有插件访问聊天、角色、记忆、文件、网络、事件、桌面、MCP 和工作流的操作都必须经过版本化 Host API。

## 第 27 步：实现插件存储与 Secret Broker

为每个扩展提供隔离的配置、状态、文件、缓存和密钥存储，禁止插件直接访问主数据库和宿主密钥。

## 第 28 步：实现统一 Event Bus 与 Hook Pipeline

统一管理应用生命周期、消息、Prompt、工具、记忆、工作流和桌面事件，并提供确定性的顺序、超时和失败降级。

---

# 第四阶段：升级 `.amitiax` 统一插件包协议

## 第 29 步：设计 `.amitiax` Manifest v2

重新定义包元数据、兼容范围、运行时、模块、权限、依赖、贡献项、资源、迁移、签名和更新来源。

## 第 30 步：设计多模块扩展包结构

允许一个 `.amitiax` 同时包含插件代码、Tools、Agent Skills、MCP、Workflows、UI、Hooks、WASM 和资源文件。

## 第 31 步：重写 `.amitiax` 解析与校验器

替换旧版仅支持 Workflow/Instructions 的解析逻辑，实现完整 Manifest、模块、权限、资源和运行时入口校验。

## 第 32 步：重写 `.amitiax` 安装事务

安装时统一完成包校验、权限预览、依赖解析、数据迁移、资源注册和失败补偿，确保安装过程原子化。

## 第 33 步：实现扩展签名与发布者信任

支持本地未签名包、可信发布者、签名验证、权限变化提示和高风险扩展的额外确认。

## 第 34 步：实现版本升级、回滚和数据迁移

支持 SemVer、宿主兼容判断、升级前备份、迁移脚本、失败回滚和旧版本恢复。

---

# 第五阶段：实现第三方插件运行时

## 第 35 步：确定桌面端默认插件运行时

选择适合 Go + Electron 桌面架构的 JavaScript/TypeScript 隔离运行时，并完成安全、性能和开发体验验证。

## 第 36 步：实现 Main Runtime

为每个插件提供长期运行上下文，用于事件处理、后台状态、定时任务、IPC 和 Tool Handler 注册。

## 第 37 步：实现 Task Runtime

为单次工具调用提供可取消、可超时、可限制资源的隔离执行上下文，避免长期污染插件状态。

## 第 38 步：实现插件内部 JSON-RPC

统一 Main Runtime、Task Runtime、UI Runtime 和宿主之间的通信协议，禁止共享宿主对象和进程指针。

## 第 39 步：实现受信任 Service Runtime

为确实需要子进程、本地服务、硬件或复杂 Computer Use 的高级插件提供独立进程运行模式和更严格授权。

## 第 40 步：实现 WASM 模块支持

允许插件携带跨平台计算模块，通过受限 WASI/Host Function 运行，不允许直接访问宿主系统资源。

---

# 第六阶段：实现桌面端 UI 扩展能力

## 第 41 步：建立 Amitia UI Contribution 协议

定义插件页面、导航、按钮、面板、菜单、消息渲染器、小组件和主题贡献的统一描述方式。

## 第 42 步：升级 Schema UI

将现有插件管理表单渲染器扩展为通用声明式 UI，支持页面、表单、列表、卡片、状态和交互组件。

## 第 43 步：实现沙箱 Web UI

允许复杂插件携带 HTML/CSS/JavaScript 页面，通过 Electron 沙箱容器运行，并仅通过安全 Bridge 调用宿主能力。

## 第 44 步：建立前端 Extension Slot

在主导航、首页、聊天标题栏、输入区、消息操作、角色页、记忆页、设置页和命令面板增加稳定扩展插槽。

## 第 45 步：实现统一插件页面宿主

增加通用 ExtensionPageHost，负责插件页面路由、权限、加载状态、崩溃降级和卸载后的安全退出。

## 第 46 步：实现消息与聊天 UI 扩展

支持输入栏动作、消息操作、消息内容渲染、聊天侧边面板和空状态扩展，同时禁止插件直接修改宿主 DOM。

## 第 47 步：实现桌面级扩展点

支持系统托盘动作、独立插件窗口、桌面小组件、通知和命令面板入口，并统一经过桌面权限控制。

## 第 48 步：实现 UI 冲突和排序策略

定义系统、官方插件、可信插件和普通插件的优先级，以及多插件竞争同一插槽时的排序和排他规则。

---

# 第七阶段：将现有能力接入新内核

## 第 49 步：迁移内置工具

将旧工具通过临时 Adapter 接入新 Tool Registry，并逐步替换为原生 Capability 实现。

## 第 50 步：迁移 Agent Skills

将现有 SKILL.md、资源、激活规则和角色作用域迁移为 Agent Skill Contribution，不再注册为伪工具。

## 第 51 步：迁移 MCP

将现有 MCP Server 和 Tool 配置纳入 Extension Kernel 的资源所有权与生命周期，同时直接注册为 Tool Capability。

## 第 52 步：迁移 Workflows

将现有工作流迁移为 Workflow Contribution，并统一接入工具执行、权限、作用域和审计体系。

## 第 53 步：迁移官方内置 Plugin

将当前 Go 内置诊断 Plugin 改造为新运行时的官方测试扩展，用于验证完整插件生命周期。

## 第 54 步：迁移旧 `.amitiax` 包

提供一次性兼容导入器，将旧版 Workflow/Instructions 包转换为 `.amitiax` Manifest v2。

## 第 55 步：迁移现有扩展数据

迁移安装记录、角色绑定、权限、MCP 配置、Agent Skill 数据、Workflow、执行历史和扩展配置。

---

# 第八阶段：开发者生态与扩展中心

## 第 56 步：提供 TypeScript Plugin SDK

提供 Manifest 类型、Host API 类型、Tool 定义、Hook、UI Contribution 和权限声明的完整 SDK。

## 第 57 步：提供插件 CLI

支持创建、开发、校验、测试、打包、签名和安装 `.amitiax` 插件项目。

## 第 58 步：实现开发者模式与热重载

支持本地目录加载、运行时重启、UI 刷新、权限模拟、日志查看和 Tool 调试。

## 第 59 步：实现插件开发者控制台

集中展示运行状态、Runtime 日志、Host API 调用、Hook 耗时、权限拒绝、崩溃和资源占用。

## 第 60 步：重建扩展中心

将原有 MCP、Agent Skills、插件、技能、扩展包等分散入口统一为“已安装、市场、本地安装、开发者模式、运行与权限”。

## 第 61 步：重建扩展详情页

按扩展展示其 Tools、Agent Skills、MCP、Workflows、UI、后台任务、权限、数据、日志和版本，而不是分散到多套页面。

---

# 第九阶段：切换、验收与删除旧系统

## 第 62 步：建立新旧系统对照验收

逐项验证旧能力在新内核中的等价性，重点检查工具调用、角色作用域、MCP、Agent Skill、Workflow 和包生命周期。

## 第 63 步：执行桌面端稳定性验收

在 Windows、macOS 和 Linux 验证安装、启停、更新、回滚、崩溃恢复、沙箱隔离和 UI 扩展稳定性。

## 第 64 步：执行安全与权限验收

验证越权访问、路径穿越、密钥泄露、插件伪装、Hook 滥用、资源泄漏、无限循环和恶意 UI 等场景。

## 第 65 步：切换 Extension Kernel 为唯一入口

停止旧系统写入，所有新增安装、启用、执行、权限和更新请求统一进入新内核。

## 第 66 步：删除旧 Plugin Runtime

删除 Go Factory、旧 PluginManager 和仅支持 builtin Plugin 的注册体系。

## 第 67 步：删除旧 Skill 兼容抽象

删除 Agent Skill 伪工具、MCP Skill Adapter 和将多种领域对象统一命名为 Skill 的兼容结构。

## 第 68 步：删除旧扩展包解析器

删除只支持 Workflow/Instructions 的 `.amitiax` v1 安装链路，仅保留独立迁移工具。

## 第 69 步：删除重复生命周期与状态表

清理旧 MCP、Plugin、Skill 和 Package 中被新内核替代的状态、恢复逻辑和冗余数据表。

## 第 70 步：完成最终全链路验收

确认安装、启用、工具调用、UI 扩展、MCP、Agent Skill、Workflow、更新、回滚、卸载和崩溃恢复全部由新系统闭环完成。

---

## 最终目标结构

```text
Amitia Extension Kernel
└── .amitiax Extension Package
    ├── Runtime Modules
    ├── Tools
    ├── Agent Skills
    ├── MCP Definitions
    ├── Workflows
    ├── UI Contributions
    ├── Hooks
    ├── Background Tasks
    ├── Providers
    ├── WASM
    └── Assets
```

最终只保留一套扩展身份、一套安装生命周期、一套权限体系、一套资源所有权、一套执行与审计入口，以及一套统一扩展中心。
