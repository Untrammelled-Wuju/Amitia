# 删除依赖图

> 明确每个最终删除对象的前置条件和依赖解除顺序

---

## 删除阶段 1：MCP Skill Adapter（无上游依赖）

**条件**: Tool Registry 和 MCP Manager 就绪

```
删除: mcp/skill/runtime.go (MCP-301)
├── 前置: MCP Manager 直接注册 Tool 到 Tool Registry
├── 依赖解除: MCP Repository.GetToolBySkillID → Tool Registry.GetTool
└── 影响: 无持久化数据，纯运行时
```

---

## 删除阶段 2：Agent Skill → SkillDefinition 注册（依赖阶段 1）

**条件**: Agent Skill Catalog 和 Contribution Registry 就绪

```
删除: buildAgentSkillManifest, setInstalledAgentSkillBinding (AGT-301)
├── 前置: Contribution Registry 就绪
├── 前置: Agent Skill → Capability 迁移
├── 依赖解除: Registry.Register(SkillDefinition) → ContributionRegistry.Register(Capability)
└── 影响: Agent Skill 元数据表需迁移

删除: registerAgentSkillRuntime, internalAgentSkillDefinition (AGT-302)
├── 前置: Agent Skill Catalog 就绪
└── 依赖解除: 内置 Agent Skill 工具 → Tool Registry
```

---

## 删除阶段 3：Plugin 旧体系（依赖阶段 1-2）

**条件**: Event Bus, Hook Pipeline, Storage Broker, Schedule Manager 就绪

```
删除: Go Interface 第三方插件协议 (PLG-301)
├── 前置: .amitiax v2 扩展包 + Extension Kernel 运行时
├── 依赖解除: Plugin → Extension Contribution
└── 数据迁移: Plugin 状态表

删除: Plugin Factory (PLG-302)
├── 前置: Package Manager v2 安装流程
└── 依赖解除: Factory → Package Manager

删除: builtin-only PluginRegistry (PLG-303)
├── 前置: Contribution Registry 统一管理
└── 依赖解除: PluginRegistry → ContributionRegistry

删除: PluginManager (PLG-304)
├── 前置: Runtime Supervisor 就绪
├── 前置: Event Bus 就绪
├── 前置: Schedule Manager 就绪
├── 依赖解除:
│   ├── BeforePrompt/AfterReply → Hook Pipeline
│   ├── 事件分发 → Event Bus
│   ├── 调度 → Schedule Manager
│   └── 启停 → Runtime Supervisor
└── 数据迁移: Plugin 状态/事件/调度表
```

---

## 删除阶段 4：Workflow → SkillDefinition 包装（依赖阶段 3）

**条件**: Workflow Engine 就绪

```
删除: skillDefinitionFromDraft, buildWorkshopManifest (WFL-301)
├── 前置: Workflow Engine 作为 Capability 直接注册
├── 依赖解除: Workflow → SkillDefinition → Workflow → Contribution
└── 数据迁移: 旧 Workflow 数据

删除: Workshop 独立安装生命周期 (WFL-302)
├── 前置: Package Manager 统一安装
└── 依赖解除: WorkshopInstaller → PackageManager
```

---

## 删除阶段 5：旧 Package 体系（依赖阶段 1-4）

**条件**: Package Manager v2 就绪，所有旧扩展包迁移

```
删除: 旧 Parser 主路径 (PKG-301)
├── 前置: v2 Manifest Parser 就绪
└── 数据迁移: v1 扩展包全部转换

删除: 二选一包模型 (PKG-302)
├── 前置: 多类型 Capability/Contribution 包支持
└── 无数据依赖

删除: 硬编码分支 (PKG-304)
├── 前置: 统一安装器 + 类型分发
└── 无数据依赖

删除: PackageHandler (PKG-305)
├── 前置: Extension Kernel HTTP API 就绪
└── 依赖解除: 前端调用旧 API → 新 API
```

---

## 删除阶段 6：旧 Workshop 体系（依赖阶段 1-5）

**条件**: Developer Tooling 就绪

```
删除: Workshop 独立 Installer (WS-301)
├── 前置: Package Manager 统一安装
└── 数据迁移: Workshop 数据

删除: WorkshopHandler (WS-302)
├── 前置: Developer Tooling API 就绪
└── 依赖解除: 前端调用旧 API → 新 API

删除: Workshop → SkillDefinition 旧路径 (WS-303)
├── 前置: Workshop → .amitiax v2 打包流程
└── 无持久化数据
```

---

## 删除阶段 7：旧类型定义（依赖阶段 1-6）

**条件**: 所有新类型在前端和后端就绪

```
删除: SkillDefinition, SkillSource, SkillTrigger, RegisteredSkill, SkillFilter, SkillView, SkillDetailView, ExecuteSkillRequest, SkillHandler, SkillResult (EXT-RT-305, 306)
├── 前置: 新贡献/能力/资源类型就绪
├── 前置: 所有旧 API 已切换
└── 风险: P0 - 确保无遗漏引用

删除: PluginManifest, PluginView, PluginState, PluginSchedule (PLG 旧类型)
├── 前提: 新 UI Contribution 类型就绪
└── 风险: P0
```

---

## 删除阶段 8：旧前端页面（依赖阶段 1-7）

**条件**: Extension Kernel UI 就绪

```
删除: SkillListView, SkillDetailView (FE-302, 部分)
├── 前置: Capability 列表和详情页就绪
└── 依赖解除: 前端路由

删除: PluginListView, PluginDetailView (FE-302, 部分)
├── 前置: UI Contribution 页面就绪
└── 依赖解除: 前端路由

删除: 分散扩展子系统入口 (FE-301)
├── 前置: 统一扩展中心就绪
└── 依赖解除: 导航和路由

删除: 旧 Workshop Skill 制作页面 (FE-305)
├── 前置: Developer Tooling 页面就绪
└── 依赖解除: 前端路由
```

---

## 删除阶段 9：旧数据表（依赖阶段 1-8）

**条件**: 数据迁移完成且验证通过

```
删除: extension_scope_bindings → scope_bindings 已完成
删除: extension_configs → capability_configs 已完成
删除: extension_capability_grants → permission_grants 已完成
删除: extension_agent_skill_metadata → agent_skill_registry 已完成
删除: extension_agent_skill_activations → extension_audit_records 已完成
删除: extension_package_installations → package_installations 已完成
删除: extension_states 等 Plugin 状态表 → Storage Broker 已完成
删除: mcp_server_scope_bindings → Scope Manager 已完成
删除: mcp_tools → Tool Registry 已完成
删除: mcp_dependency_links → Dependency Resolver 已完成
```

---

## 并发删除组

可以并行进行的删除操作：

### 并行组 A（与 B 无依赖）
- MCP-301: `mcp/skill/runtime.go`
- MCP-303: 重复审计模型

### 并行组 B（依赖 A）
- AGT-301: Agent Skill → SkillDefinition
- AGT-302: registerAgentSkillRuntime

### 并行组 C（依赖 B）
- PLG-301 到 PLG-307: Plugin 旧体系并行删除

### 并行组 D（依赖 C）
- WFL-301 到 WFL-303: Workflow 旧体系

### 并行组 E（依赖 D）
- PKG-301 到 PKG-305: Package 旧体系

### 并行组 F（依赖 E）
- WS-301 到 WS-303: Workshop 旧体系

### 并行组 G（依赖 F）
- EXT-RT-305, 306: 旧类型定义

### 并行组 H（依赖 G）
- FE-301 到 FE-306: 前端旧页面

### 并行组 I（依赖 H）
- 数据表删除（可并行进行多表删除）
