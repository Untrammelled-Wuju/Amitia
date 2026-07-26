# 旧 Skill 分类清单

生成时间：2026-07-25

## 一、Legacy Tool → ToolDefinition (source: legacy_tool)

| # | 旧 Skill ID | Model Name | 来源 | 迁移目标 Tool ID | 状态 |
|---|-------------|------------|------|-----------------|------|
| 1 | dev.amitia.skill.save-memory | save_memory | legacy_tool_adapter.go | legacy_tool/amitia/save-memory | 待迁移 |
| 2 | dev.amitia.skill.save-profile | save_profile | legacy_tool_adapter.go | legacy_tool/amitia/save-profile | 待迁移 |
| 3 | dev.amitia.skill.save-episodic-memory | save_episodic_memory | legacy_tool_adapter.go | legacy_tool/amitia/save-episodic-memory | 待迁移 |
| 4 | dev.amitia.skill.get-current-time | get_current_time | legacy_tool_adapter.go | legacy_tool/amitia/get-current-time | 待迁移 |
| 5 | dev.amitia.skill.create-schedule | create_schedule | legacy_tool_adapter.go | legacy_tool/amitia/create-schedule | 待迁移 |
| 6 | dev.amitia.skill.force-voice-reply | force_voice_reply | legacy_tool_adapter.go | legacy_tool/amitia/force-voice-reply | 待迁移 |
| 7 | dev.amitia.skill.read-need-state | read_need_state | legacy_tool_adapter.go | legacy_tool/amitia/read-need-state | 待迁移 |
| 8 | dev.amitia.skill.read-psyche-state | read_psyche_state | legacy_tool_adapter.go | legacy_tool/amitia/read-psyche-state | 待迁移 |
| 9 | dev.amitia.skill.summarize-memories | summarize_memories | legacy_tool_adapter.go | legacy_tool/amitia/summarize-memories | 待迁移 |

## 二、Internal Control Tool → ToolDefinition (source: internal)

| # | 旧 Skill ID | Model Name | 来源 | 迁移目标 Tool ID | 状态 |
|---|-------------|------------|------|-----------------|------|
| 1 | dev.amitia.skill.agent-skill-activate | agent_skill_activate | agent_skill_runtime.go | internal/agent-skill/agent_skill_activate | 待迁移 |
| 2 | dev.amitia.skill.agent-skill-list-resources | agent_skill_list_resources | agent_skill_runtime.go | internal/agent-skill/agent_skill_list_resources | 待迁移 |
| 3 | dev.amitia.skill.agent-skill-read-resource | agent_skill_read_resource | agent_skill_runtime.go | internal/agent-skill/agent_skill_read_resource | 待迁移 |
| 4 | dev.amitia.skill.agent-skill-get-asset | agent_skill_get_asset | agent_skill_runtime.go | internal/agent-skill/agent_skill_get_asset | 待迁移 |

## 三、Builtin Plugin Tool → ToolDefinition (source: builtin)

| # | 旧 Skill ID | Model Name | 来源 | 迁移目标 Tool ID | 状态 |
|---|-------------|------------|------|-----------------|------|
| 1 | dev.amitia.plugin.diagnostic | plugin_runtime_diagnostic | plugin_host.go | builtin/amitia/plugin_runtime_diagnostic | 待迁移 |

## 四、MCP Tool → ToolDefinition (source: mcp)

| # | 来源 | 迁移目标 Tool ID 模式 | 状态 |
|---|------|---------------------|------|
| N | mcp/skill/runtime.go | mcp/<server-id>/<tool-name> | 待迁移（动态发现） |

## 五、Workflow → WorkflowDefinition (+ ToolAdapter)

| # | 来源 | 迁移目标 | 状态 |
|---|------|----------|------|
| N | workshop/installer | WorkflowDefinition + Tool Adapter | 待迁移（动态安装） |

## 六、Agent Skill → AgentSkillDefinition

| # | 来源 | 迁移目标 | 状态 |
|---|------|----------|------|
| N | agent_skill_service.go | AgentSkillDefinition(catalog) | 待迁移（动态安装） |

## 七、迁移状态统计

| 类型 | 数量 | 适配器 | Registry |
|------|------|--------|----------|
| Legacy Tool | 9 | LegacySkillToTool | ToolRegistry |
| Internal Tool | 4 | InternalSkillToTool | ToolRegistry |
| Builtin Plugin | 1 | BuiltinSkillToTool | ToolRegistry |
| MCP Tool | 动态 | MCPSkillToTool | ToolRegistry |
| Workflow | 动态 | WorkflowToDefinition | WorkflowRegistry |
| Agent Skill | 动态 | AgentSkillToDefinition | AgentSkillCatalog |

## 八、仍依赖 SkillDefinition 的文件

以下文件仍在代码中使用旧 SkillDefinition，需在后续步骤迁移：

| 文件 | 依赖内容 |
|------|----------|
| `extension/protocol.go` | SkillDefinition, SkillHandler, RegisteredSkill |
| `extension/registry.go` | SkillRegistry, Registry |
| `extension/runtime.go` | ModelTools(), ExecuteModelTool(), NewRuntime() |
| `extension/executor.go` | Executor, SkillExecutor |
| `extension/legacy_tool_adapter.go` | Adapt(), RegisterAll() |
| `extension/agent_skill_runtime.go` | registerAgentSkillRuntime() |
| `extension/agent_skill_service.go` | AgentSkillService, SKILL.md 解析 |
| `extension/plugin_host.go` | RegisterSkill(), CallSkill() |
| `extension/workflow_compiler.go` | Compile(), AnalyzeDependencyCycles() |
| `extension/workshop_protocol.go` | ExtensionDraft, WorkflowDefinition |
| `mcp/skill/runtime.go` | build(), RegisterServer() |

## 九、后续步骤

1. 第7步：建立统一 Tool/Capability 模型
2. 第8步：逐类迁移 Legacy/MCP/Plugin/Workflow/AgentSkill
3. 第9步：删除旧 SkillDefinition 及兼容层
4. 第10步：更新前端 API 和 UI 术语
