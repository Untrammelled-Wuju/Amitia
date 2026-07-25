# 最终删除汇总报告

> 基于 `classification/*.md` 分类结果汇总
> 所有「最终删除」对象：含替代组件、删除步骤、风险等级

---

## 一、统计概览

| 来源模块 | 对象数 |
|---|---|
| 后端 Extension Runtime & Registry | 8 |
| 后端 Agent Skill | 5 |
| 后端 MCP | 4 |
| 后端 Plugin | 7 |
| 后端 Workflow | 3 |
| 后端 Package | 5 |
| 后端 Workshop | 3 |
| 前端 | 6 |
| 数据表（迁移后删除） | 8 |
| API（旧路径兼容层） | 全部 Skill/Plugin/Workshop 旧路径 |
| **合计** | **49** |

---

## 二、删除阶段 1：MCP Skill Adapter（P0，最高优先级）

**依赖**: Tool Registry 和 MCP Manager 就绪

| ID | 对象 | 替代组件 | 数据迁移条件 | 删除步骤 |
|---|---|---|---|---|
| MCP-301 | `mcp/skill/runtime.go` → `Runtime`, `RegisterAll`, `RegisterServer`, `build` | MCP Manager 直接注册 Tool 到 Tool Registry | 无持久化数据 | 1. MCP Manager → Tool Registry 直接注册就绪<br>2. 删除 `mcp/skill/runtime.go` 及测试 |
| MCP-302 | Manager 中通过 Skill Runtime 注册逻辑 | MCP Manager → Tool Registry 直接注册 | 无持久化数据 | 随 MCP-301 删除 |
| MCP-303 | `mcp/model.go` → 重复 `AuditLog` 模型 | 统一 Audit Store | 旧审计日志迁移 | 1. 统一 Audit Store 就绪<br>2. 旧审计日志迁移<br>3. 删除重复模型 |
| MCP-304 | Manager 独立 Connect/Disconnect 生命周期 | Runtime Supervisor 统一管理 | 无持久化数据 | 随 Runtime Supervisor 就绪 |

---

## 三、删除阶段 2：Agent Skill → SkillDefinition 注册（依赖阶段 1）

**依赖**: Agent Skill Catalog 和 Contribution Registry 就绪

| ID | 对象 | 替代组件 | 数据迁移条件 | 删除步骤 |
|---|---|---|---|---|
| AGT-301 | `agent_skill_service.go` → `buildAgentSkillManifest`, `setInstalledAgentSkillBinding` | Agent Skill 直接注册为 Capability/Contribution | Agent Skill 元数据表迁移 | 1. Contribution Registry 就绪<br>2. Agent Skill → Capability 迁移<br>3. 删除包装层 |
| AGT-302 | `agent_skill_runtime.go` → `registerAgentSkillRuntime`, `internalAgentSkillDefinition` | Extension Kernel 内置 Tool Registry | 无持久化数据 | 1. Tool Registry 就绪<br>2. 内置 Agent Skill 工具注册到 Tool Registry<br>3. 删除旧注册 |
| AGT-303 | `agent_skill_runtime.go` → `PrepareAgentSkillPrompt`, `EndAgentSkillRound` | Agent Skill Catalog 直接集成到 Extension Kernel | 无持久化数据 | 随 AGT-301 删除 |
| AGT-304 | `agent_skill_protocol.go` → `AgentSkillActivation`, `ActivateAgentSkillRequest`, `ActivatedAgentSkill`, `AgentSkillCatalogEntry` | Extension Kernel 统一类型 | 无持久化数据（类型） | 前端全部切换到新类型后删除 |
| AGT-305 | `agent_skill_handler.go` → `AgentSkillHandler` | Extension Kernel 统一 HTTP API | 无持久化数据 | 新 HTTP API 就绪后删除 |

---

## 四、删除阶段 3：Plugin 旧体系（依赖阶段 1-2）

**依赖**: Event Bus, Hook Pipeline, Storage Broker, Schedule Manager 就绪

| ID | 对象 | 替代组件 | 数据迁移条件 | 删除步骤 |
|---|---|---|---|---|
| PLG-301 | `plugin_protocol.go` → Go Interface 第三方插件协议 | `.amitiax` v2 扩展包 + Extension Kernel 运行时 | Plugin 状态数据迁移 | 1. .amitiax v2 就绪<br>2. 新 Plugin 体系就绪<br>3. 删除 Go Interface 协议 |
| PLG-302 | `plugin_registry.go` → Plugin Factory 注册 | Package Manager v2 安装流程 | 无持久化数据（注册逻辑） | 随 Package Manager v2 就绪 |
| PLG-303 | `plugin_registry.go` → builtin-only `PluginRegistry` | Contribution Registry 统一管理 | 内置 Plugin 迁移 | 1. Contribution Registry 就绪<br>2. 内置 Plugin 注册到 Contribution Registry<br>3. 删除 `PluginRegistry` |
| PLG-304 | `plugin_manager.go` → `PluginManager`, `Start`, `Stop` | Runtime Supervisor 统一管理 | Plugin 状态/事件/调度表 | 1. Runtime Supervisor 就绪<br>2. Event Bus 就绪<br>3. Schedule Manager 就绪<br>4. BeforePrompt/AfterReply → Hook Pipeline<br>5. 数据迁移完成<br>6. 删除 `PluginManager` |
| PLG-305 | `plugin_builtin_diagnostic.go` → `newDiagnosticPlugin()` | Developer Tooling 内置诊断 | 无持久化数据 | 1. Developer Tooling 诊断就绪<br>2. 删除旧诊断 |
| PLG-306 | `plugin_manager.go` → Plugin 注册 Skill 到 Registry | Plugin → Contribution Registry 直接注册 | 无持久化数据 | 随 PLG-303 删除 |
| PLG-307 | `plugin_surface.go`, `plugin_handler.go` → Surface 与 Plugin ID 绑定 | UI Contribution Registry 统一管理 | 无持久化数据 | 随 UI Contribution Registry 就绪 |

---

## 五、删除阶段 4：Workflow → SkillDefinition 包装（依赖阶段 3）

**依赖**: Workflow Engine 就绪

| ID | 对象 | 替代组件 | 数据迁移条件 | 删除步骤 |
|---|---|---|---|---|
| WFL-301 | `workshop_installer.go` → `skillDefinitionFromDraft`, `buildWorkshopManifest` | Workflow Engine 直接作为 Capability 注册 | 旧 Workflow 数据迁移 | 1. Workflow Engine 就绪<br>2. 旧 Workflow 迁移为 Capability<br>3. 删除包装层 |
| WFL-302 | `workshop_installer.go` → `(i *WorkshopInstaller) Install`（独立安装生命周期） | Package Manager 统一安装 | 无持久化数据（安装逻辑） | 1. Package Manager 就绪<br>2. 删除独立安装逻辑 |
| WFL-303 | `workflow_executor.go` → Adapter 层面重复权限判断 | Permission Broker 统一判定 | 无持久化数据 | 1. Permission Broker 就绪<br>2. 删除重复判断 |

---

## 六、删除阶段 5：旧 Package 体系（依赖阶段 1-4）

**依赖**: Package Manager v2 就绪，所有旧扩展包迁移

| ID | 对象 | 替代组件 | 数据迁移条件 | 删除步骤 |
|---|---|---|---|---|
| PKG-301 | `package_parser.go` → `parseExtensionPackage` 主流程 | v2 Manifest Parser | v1 扩展包全部转换 | 1. v2 Manifest Parser 就绪<br>2. v1 包全部转换为 v2<br>3. 删除 v1 Parser |
| PKG-302 | `package_parser.go`, `package_installer.go` → Workflow/Instructions 二选一包模型 | 多类型 Capability/Contribution 包 | 无持久化数据（解析约束） | 随 PKG-301 删除 |
| PKG-303 | `package_parser.go` → 未接通的 Plugin 分支 | 新 Plugin 体系 | 无数据 | 可直接删除 |
| PKG-304 | `package_installer.go` → `installWorkflowPackage`, `installInstructionsPackage`（硬编码分支） | 统一安装器 + 类型分发 | 无持久化数据 | 随 PKG-301 删除 |
| PKG-305 | `package_handler.go` → `PackageHandler` | Extension Kernel HTTP API | 无持久化数据 | 新 HTTP API 就绪后删除 |

---

## 七、删除阶段 6：旧 Workshop 体系（依赖阶段 1-5）

**依赖**: Developer Tooling 就绪

| ID | 对象 | 替代组件 | 数据迁移条件 | 删除步骤 |
|---|---|---|---|---|
| WS-301 | `workshop_installer.go` → `WorkshopInstaller` | Package Manager 统一安装 | Workshop 数据迁移 | 1. Package Manager 就绪<br>2. Workshop 产物通过 Package Manager 安装<br>3. 删除 `WorkshopInstaller` |
| WS-302 | `workshop_handler.go` → `WorkshopHandler` | Extension Kernel HTTP API + Developer Tooling API | 无持久化数据 | 1. Developer Tooling API 就绪<br>2. 删除 `WorkshopHandler` |
| WS-303 | `workshop_installer.go` → `skillDefinitionFromDraft`, `skillDefinitionFromManifest` | Workshop → .amitiax v2 → Package Manager → Contribution Registry | 无持久化数据（类型转换） | 随 WS-301 删除 |

---

## 八、删除阶段 7：旧类型定义（依赖阶段 1-6）

**依赖**: 所有新类型在前端和后端就绪

| ID | 对象 | 替代组件 | 风险等级 |
|---|---|---|---|
| EXT-RT-305 | `protocol.go` → `SkillDefinition` | 拆分后各新类型（Capability, Contribution, Resource 等） | P0 |
| EXT-RT-306 | `protocol.go` → `SkillSource`, `SkillTrigger`, `RegisteredSkill`, `SkillFilter`, `SkillView`, `SkillDetailView`, `ExecuteSkillRequest`, `SkillHandler`, `SkillResult` | 新领域类型 | P0 |
| EXT-RT-307 | `service.go` → `ListPlugins`, `GetPlugin`, `EnablePlugin` 等 Plugin 委托方法 | Plugin 子系统独立 API（由 Extension Kernel 统一路由） | P1 |
| EXT-RT-308 | `runtime.go` → `(r *Runtime) BeforePrompt`, `AfterReply` | Hook Pipeline 在 Extension Kernel 中统一管理 | P0 |

### 删除步骤

1. 确认所有旧类型无代码引用（编译器报错驱动）
2. 确认新类型已覆盖所有功能
3. 删除旧类型定义
4. 删除旧类型的序列化/反序列化逻辑

---

## 九、删除阶段 8：旧前端页面（依赖阶段 1-7）

**依赖**: Extension Kernel UI 就绪

| ID | 对象 | 替代组件 | 删除步骤 |
|---|---|---|---|
| FE-301 | 路由中多个独立扩展子系统入口（`/extensions/skills`, `/extensions/plugins`, `/extensions/workshop`, `/extensions/agent-skills`） | 统一 `/extensions` 中心 | 1. 统一 UI 就绪<br>2. 导航更新<br>3. 删除旧路由 |
| FE-302 | `SkillDetailView.vue`, `PluginDetailView.vue`（重复详情页） | 统一 Capability 详情页 | 1. Capability 详情页就绪<br>2. 删除旧详情页 |
| FE-303 | SkillListView, SkillDetailView 中的 Skill 专属展示 | Capability 类型展示 | 随 FE-302 删除 |
| FE-304 | 各页面中直接调用旧 Enable/Disable API 的控件 | Extension Kernel 统一生命周期管理 | 1. Extension Kernel 生命周期 API 就绪<br>2. 前端控件切换<br>3. 删除旧控件 |
| FE-305 | `WorkshopSessionView.vue` 中 Skill 生成相关 | Developer Tooling 新扩展制作 | 1. Developer Tooling 就绪<br>2. 删除旧 Workshop UI |
| FE-306 | `MCPServerView.vue` 中以 Skill 视角展示 MCP Tool | MCP Tool 直接展示 | 1. MCP Manager UI 就绪<br>2. 删除旧 MCP 页面 |

---

## 十、删除阶段 9：旧数据表（依赖阶段 1-8）

**依赖**: 数据迁移完成且验证通过

| 表名 | 当前数据量 | 用户数据 | 密钥 | 删除条件 |
|---|---|---|---|---|
| `extension_scope_bindings` | 少量 | 是 | 否 | Scope Manager 就绪 |
| `extension_configs` | 少量 | 是 | 是（加密） | Config Store 就绪 |
| `extension_capability_grants` | 少量 | 是 | 否 | Permission Broker 就绪 |
| `extension_agent_skill_metadata` | 少量 | 是 | 否 | Agent Skill 迁移完成 |
| `extension_agent_skill_activations` | 少量 | 是 | 否 | 审计记录迁移完成 |
| `extension_owned_resources` | 少量 | 否 | 否 | Storage Broker 就绪 |
| `extension_package_installations` | 少量 | 是 | 否 | Package Store 就绪 |
| `extension_package_import_sessions` | 极少 | 否 | 否 | 导入记录迁移 |
| `extension_package_signers` | 极少 | 否 | 否 | 签名者数据迁移 |
| `extension_version_dependencies` | 少量 | 否 | 否 | 依赖数据迁移 |
| `extension_package_exports` | 极少 | 否 | 否 | 导出记录迁移 |
| `extension_states` | 极少 | 否 | 否 | Storage Broker 就绪 |
| `extension_state_revisions` | 极少 | 否 | 否 | 同上 |
| `extension_events` | 少量 | 否 | 否 | Audit Store 就绪 |
| `extension_event_deliveries` | 少量 | 否 | 否 | 同上 |
| `extension_schedules` | 极少 | 否 | 否 | Schedule Manager 就绪 |
| `extension_plugin_runs` | 极少 | 否 | 否 | Audit Store 就绪 |
| `extension_audits` | 少量 | 否 | 否 | 同上 |
| `mcp_server_scope_bindings` | 少量 | 是 | 否 | Scope Manager 就绪 |
| `mcp_server_credentials` | 少量 | 是 | 是（引用） | Secret Broker 就绪 |
| `mcp_tools` | 少量 | 否 | 否 | Tool Registry 就绪 |
| `mcp_dependency_links` | 少量 | 否 | 否 | Dependency Resolver 就绪 |
| `mcp_oauth_sessions` | 极少 | 是 | 是（引用） | Secret Broker 就绪 |

### P0 高风险表备份要求

| 表名 | 风险 | 备份方式 |
|---|---|---|
| `extension_artifacts` | 三系统共享，删除可能丢包 | 全量备份后操作 |
| `mcp_server_credentials` | 含 Secret 引用 | 全量加密备份 |
| `mcp_oauth_sessions` | 含 OAuth Token 引用 | 全量加密备份 |
| `extension_configs` | 含用户加密配置 | 全量加密备份 |

---

## 十一、删除顺序图

```
阶段1: MCP Skill Adapter ──── 无上游依赖
    │
阶段2: Agent Skill → SkillDefinition
    │
阶段3: Plugin 旧体系 ──── 依赖 1-2
    │
阶段4: Workflow 包装层 ──── 依赖 3
    │
阶段5: Package v1 ──── 依赖 1-4
    │
阶段6: Workshop 旧体系 ──── 依赖 1-5
    │
阶段7: 旧类型定义 ──── 依赖 1-6
    │
阶段8: 前端旧页面 ──── 依赖 1-7
    │
阶段9: 旧数据表 ──── 依赖 1-8
```

### 并行删除组

| 组 | 可并行删除 | 前提 |
|---|---|---|
| A | MCP-301, MCP-303 | 无 |
| B | AGT-301, AGT-302 | A 完成 |
| C | PLG-301 到 PLG-307 | B 完成 |
| D | WFL-301, WFL-302, WFL-303 | C 完成 |
| E | PKG-301 到 PKG-305 | D 完成 |
| F | WS-301, WS-302, WS-303 | E 完成 |
| G | EXT-RT-305, 306, 307, 308 | F 完成 |
| H | FE-301 到 FE-306 | G 完成 |
| I | 数据表删除（全部并行） | H 完成 |

---

## 十二、每个删除操作验证清单

每个删除操作完成后必须验证：

1. 编译通过（Go 后端 `go build`，前端 `pnpm build`）
2. 无残留引用（`rg` 搜索确认）
3. 相关测试通过
4. 功能回归：对应新组件功能正常
5. 数据完整性：迁移后的数据可访问

---

## 十三、回滚策略

| 阶段 | 回滚方式 |
|---|---|
| 1-2 | 直接恢复代码（无数据依赖） |
| 3 | 恢复代码 + 恢复 Plugin 状态表 |
| 4-5 | 恢复代码 + 恢复 Package 数据 |
| 6-7 | 恢复代码 + 恢复 Workshop 数据 |
| 8 | Git revert（前端无持久化状态） |
| 9 | 数据表回滚需要从备份恢复 |
