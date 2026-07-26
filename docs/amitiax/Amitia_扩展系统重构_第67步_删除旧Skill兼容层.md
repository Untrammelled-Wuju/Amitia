# Amitia 扩展系统重构第 67 步实施文档

## 第 67 步：删除旧 Skill 兼容层

---

## 一、步骤目标

在 Tool、Agent Skill、MCP、Workflow 已完全迁移且 Extension Kernel 已成为唯一入口后，删除旧 Skill 概念、SkillManager、Skill Runtime、Skill Registry、Skill Enabled、Skill Prompt 注入和所有把 Tool、Workflow、MCP 包装为 Skill 的兼容层。

本步骤目标：

> 彻底结束“Skill”概念过载，只保留术语明确的 Agent Skill；所有可执行能力使用 Tool，编排使用 Workflow，MCP 使用 MCP Contribution。

---

## 二、删除前置条件

必须满足：

-第 6 步概念拆分完成；
-第 49 步 Tool 迁移完成；
-第 50 步 Agent Skill 迁移完成；
-第 51 步 MCP 迁移完成；
-第 52 步 Workflow 迁移完成；
-模型 Tool 只来自 ToolRegistry；
-Prompt 只从 AgentSkillCatalog；
-旧 Skill API 调用已统计；
-旧 Skill 表无写入；
-所有旧 ID 有映射；
-用户导入 Skill 已迁移或隔离。

---

## 三、删除对象

至少包括：

```text
SkillManager
SkillRegistry
SkillRuntime
SkillExecutor
SkillHandler
SkillToolAdapter (old)
WorkflowSkillAdapter
MCPSkillAdapter
PluginSkillAdapter
SkillEnabledService
SkillScopeService
SkillRunRepository
SkillPromptInjector
SkillAutoInstaller
SkillPackageImporter
SkillResourceLoader (old)
```

---

## 四、保留术语

仅保留：

```text
Agent Skill
AgentSkillDefinition
AgentSkillCatalog
AgentSkillLoader
AgentSkillSelector
AgentSkillContextProvider
```

代码、数据库和 UI 不应再使用无修饰 `Skill` 表示多种对象。

---

## 五、旧 Skill 分类残留

删除前扫描所有旧 Skill：

-已迁 Tool；
-已迁 Agent Skill；
-已迁 Workflow；
-已迁 MCP；
-Unknown；
-Invalid。

Unknown/Invalid 必须有用户可见迁移报告，不能因删除兼容层而静默丢失。

---

## 六、模型 Tool 兼容删除

删除：

-Skill 转 OpenAI Tool；
-Skill Function Schema；
-Skill Handler Dispatch；
-Skill Enabled 过滤；
-Skill Tool Name；
-Skill Run。

模型调用只使用 ToolDefinition。

---

## 七、Prompt 注入删除

删除：

-旧 Skill Prompt 列表；
-每轮注入全部 Skill；
-角色 Prompt 拼 Skill；
-Plugin Skill Prompt；
-MCP Skill Prompt；
-Workflow Skill Prompt；
-旧 Token 预算。

保留 Agent Skill 唯一注入链。

---

## 八、Workflow Skill 删除

旧 Workflow 通过 Skill 执行的 Adapter 删除。

保留：

```text
WorkflowToolAdapter
```

它输出正式 ToolDefinition，不输出 Skill。

---

## 九、MCP Skill 删除

删除：

-Server 作为 Skill；
-MCP Tool 作为 Skill；
-Skill 自动安装 MCP；
-Skill Runtime 转发 MCP；
-Skill Enabled 控制连接。

---

## 十、Plugin Skill 删除

官方 Plugin Tool 已为 Tool Contribution。

删除任何 `plugin_skill` 类型和注册逻辑。

---

## 十一、旧资源加载器

如果旧 Skill Resource Loader 与 Agent Skill Loader 重复：

-迁移必要资源；
-删除旧路径；
-删除旧缓存；
-删除旧 Token 估算；
-删除旧解析；
-保留只读迁移工具到本步骤完成后。

---

## 十二、Enabled 与 Scope

删除旧：

```text
skill_enabled
skill_character_bindings
skill_scope
```

代码访问。

数据表删除留到第 69 步。

新系统：

-Extension/Module/Contribution Enabled；
-Agent Skill Scope Binding；
-本轮 Selection。

---

## 十三、旧 API

例如：

```text
/api/skills
/api/skills/:id/enable
/api/skills/:id/run
```

处理：

-列表跳转 Agent Skill/Tool/Workflow 分类视图；
-Enable 映射 Contribution；
-Run 若目标为 Tool/Workflow，映射正式执行；
-无法识别返回 Gone/迁移信息。

本步骤后不再接受泛化 Skill 创建。

---

## 十四、前端

删除：

-泛 Skill 页面；
-Skill 执行按钮；
-Skill 类型混合列表；
-Skill Enabled 总开关；
-Skill Run 历史；
-Skill MCP 安装；
-Skill Workflow 编辑。

保留：

-Extension Center；
-Agent Skill 专业视图；
-Tool；
-Workflow；
-MCP。

---

## 十五、CLI/SDK

禁止新 SDK/API 使用泛 `defineSkill()` 表示执行能力。

可保留：

```text
defineAgentSkill metadata helper
```

但 Agent Skill 文件仍是 SKILL.md。

---

## 十六、数据库查询

删除所有新业务对旧 Skill 表的访问。

历史查询通过 Legacy Read Gateway，并明确类型。

---

## 十七、日志和指标

删除：

-`skill_execute`；
-`skill_run`；
-`skill_runtime`；

迁为：

-Tool Invocation；
-Workflow Operation；
-Agent Skill Selection；
-MCP Invocation。

---

## 十八、文档术语

全项目文档统一：

-Tool；
-Agent Skill；
-Workflow；
-MCP；
-Extension。

用户界面可使用“技能”中文作为 Agent Skill 展示时，必须明确不会与 Tool 混用。

---

## 十九、静态检查

CI 禁止：

-新 `SkillDefinition`；
-新 `SkillManager`；
-新 `type=skill` 执行对象；
-Tool 注册到 SkillRegistry；
-Workflow/MCP Skill Adapter。

允许白名单：

-AgentSkill；
-Legacy migration file；
-历史 DTO。

---

## 二十、删除顺序

1.删除新创建入口。
2.删除运行入口。
3.删除模型 Tool 兼容。
4.删除 Workflow/MCP/Plugin Adapter。
5.删除 Prompt 旧注入。
6.删除 Enabled/Scope 旧服务。
7.删除旧 Resource Loader。
8.删除前端旧页面。
9.删除旧 API 或转 Adapter。
10.删除类型和依赖。
11.更新文档和测试。

---

## 二十一、回滚

若发现缺失：

-将对象正确分类并迁入 Tool/Agent Skill/Workflow/MCP；
-不得恢复泛 Skill Runtime。

---

## 二十二、测试要求

覆盖：

-模型 Tool；
-Agent Skill Prompt；
-Workflow Tool；
-MCP Tool；
-官方 Plugin Tool；
-旧 Skill ID；
-旧 API；
-用户导入；
-角色 Scope；
-Token；
-前端；
-历史；
-应用启动；
-无旧 Registry；
-无双注入；
-无双执行。

---

## 二十三、实施任务

1.输出 Skill 兼容层清单。
2.确认所有对象分类。
3.删除泛 Skill 创建。
4.删除 Skill Executor/Runtime。
5.删除模型 Tool Adapter。
6.删除 Workflow/MCP/Plugin Skill Adapter。
7.删除旧 Prompt Injector。
8.删除 Enabled/Scope 服务。
9.删除旧 Resource Loader。
10.迁移旧 API。
11.删除前端泛 Skill 页面。
12.统一术语。
13.增加 CI 禁止规则。
14.完成回归。
15.输出删除报告。

---

## 二十四、验收标准

1.不存在泛 Skill Runtime。
2.不存在 SkillManager/Registry。
3.Tool 不再经过 Skill。
4.Workflow 不再作为 Skill。
5.MCP 不再作为 Skill。
6.Plugin Tool 不再作为 Skill。
7.Prompt 只注入 Agent Skill。
8.Agent Skill 不执行代码。
9.旧 API 不再创建泛 Skill。
10.CI 阻止概念回潮。
11.关键回归通过。
12.可进入第 68 步删除旧包解析器。

---

## 二十五、执行约束

> 删除旧 Skill 兼容层后，Amitia 中任何“可执行技能”都必须明确建模为 Tool 或 Workflow，Agent Skill 永远只是指令和资源。

禁止：

-恢复 Skill Handler；
-新增 Skill Runtime；
-Tool 重新注册 Skill；
-Workflow/MCP 包装 Skill；
-双 Prompt 注入；
-为了兼容继续写旧 Skill 表；
-用“Skill”作为所有扩展的后端统一类型。
