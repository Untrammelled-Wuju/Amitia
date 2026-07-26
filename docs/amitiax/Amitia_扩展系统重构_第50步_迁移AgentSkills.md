# Amitia 扩展系统重构第 50 步实施文档

## 第 50 步：迁移 Agent Skills

---

## 一、步骤目标

将当前所有内置、用户导入、旧 Package 导入和旧 Skill 系统中的 Agent Skill，迁移到第 15 步定义的 Agent Skill Loader、AgentSkillCatalog 和 Extension Kernel Contribution 模型。

目标：

> 使 Agent Skill 只作为声明式指令、知识、资源和激活提示存在，不再拥有执行 Handler、不再伪装 Tool、不再自行安装 MCP、不再独立管理 Enabled、Scope 和生命周期。

---

## 二、迁移对象

包括：

-系统内置 Agent Skill；
-旧 `SKILL.md`；
-用户导入 Skill；
-旧 `.amitiax` 内 Skill；
-Plugin 附带 Skill；
-Workflow Skill 包装；
-MCP Skill 包装；
-旧 SkillManager 中仅包含提示词的项目；
-角色专属 Skill；
-全局 Skill；
-带 references/templates/examples 的 Skill；
-声明 MCP 依赖的 Skill。

---

## 三、分类

迁移前必须将旧 Skill 分类：

```text
Agent Skill
Executable Tool
Workflow Wrapper
MCP Tool Wrapper
Prompt Fragment
Unknown
```

只有真正的 Agent Skill 进入本步骤。

其他类型分别由第 49、51、52 步迁移。

---

## 四、目标建模

系统内置 Skill：

```text
ExtensionID: system/amitia-core
Module: agent-skills
ContributionType: agent_skill
```

用户独立 Skill：

```text
Synthetic Extension
local.user/agent-skill-<stable-id>
```

扩展附带 Skill：

```text
所属 Extension Module
→ Agent Skill Contribution
```

---

## 五、稳定 ID

建议：

```text
agent-skill/<namespace>/<name>
```

或 Contribution 全局 ID。

必须建立：

```text
legacy_skill_id
legacy_name
source_path
→ canonical_agent_skill_id
```

名称变化不能改变稳定 ID。

---

## 六、Source 迁移

记录来源：

```text
builtin
extension_package
user_import
legacy_migration
development_workspace
synthetic_extension
```

Source 不决定 Enabled、Trust 或 Scope。

---

## 七、SKILL.md 规范化

迁移时处理：

-UTF-8；
-BOM；
-换行；
-Frontmatter；
-标题；
-描述；
-版本；
-资源链接；
-路径；
-重复键；
-隐藏字符；
-大文件；
-Token 估算。

不得静默修改语义内容。

---

## 八、资源迁移

资源包括：

```text
references
templates
examples
schemas
assets
```

迁移到 Extension Artifact 或 Synthetic Extension Artifact。

必须：

-Hash；
-MIME；
-大小；
-相对路径；
-Owner；
-加载策略；
-Token 估算。

---

## 九、渐进加载

迁移后按：

```text
L0 元数据
L1 摘要
L2 完整 SKILL.md
L3 资源
L4 大型资产
```

加载。

禁止所有 Skill 全文永久注入系统 Prompt。

---

## 十、激活状态拆分

旧：

```text
enabled
active
loaded
selected
```

拆分为：

-Definition 已加载；
-Contribution Enabled；
-Scope 匹配；
-依赖满足；
-本轮被选择；
-资源已加载。

不得将“未被选择”显示为 Disabled。

---

## 十一、Scope 迁移

旧全局/角色 Skill：

```text
Agent Skill Enabled
+ Scope Binding
```

角色专属迁移为 Character Scope。

不得迁成 Global。

Conversation Skill 必须校验会话归属。

---

## 十二、Enabled 迁移

旧多个 Enabled 冲突时：

-按第 19 步规则；
-无法确认默认 Disabled；
-保留冲突报告；
-不取 OR。

---

## 十三、依赖迁移

Agent Skill 可声明：

-Tool；
-MCP Server；
-Provider；
-Host Feature；
-其他 Skill（如正式支持）。

迁移只创建 DependencyDefinition。

不得在 Loader 阶段安装或连接 MCP。

---

## 十四、MCP 依赖

旧 Skill 若内嵌 MCP 配置：

1.提取 MCP Server Definition；
2.转为 Synthetic/Extension-owned MCP Contribution；
3.Agent Skill 建立依赖引用；
4.使用 Resource Reference；
5.不重复创建相同 Server。

---

## 十五、Tool 引用

Agent Skill 文本中声明 Tool 时：

-映射稳定 Tool ID；
-缺失时 Warning；
-Required/Optional 明确；
-不复制 Tool Schema 到 Skill；
-不创建伪 Handler。

---

## 十六、Prompt 注入迁移

统一链路：

```text
AgentSkillSelector
→ AgentSkillContextProvider
→ Prompt Assembly
```

删除：

-旧 SkillManager Prompt 拼接；
-Plugin Prompt 注入；
-角色 Prompt 直接追加 Skill 全文；
-MCP Skill Adapter 注入；
-Workflow Skill Prompt 注入。

必须保证只有一个 Context 注入路径。

---

## 十七、优先级与冲突

多个 Skill 同时激活时：

-系统安全和核心角色规则优先；
-Skill 不能覆盖系统安全；
-按 Scope、用户选择、激活相关性、稳定 ID；
-保留来源；
-Token Budget；
-冲突可诊断。

---

## 十八、用户编辑

用户修改导入 Skill：

-转为用户资产；
-生成新 Revision；
-或 Fork 到 Synthetic Extension；
-原扩展更新不得覆盖。

---

## 十九、版本和 Revision

Agent Skill Definition 版本与 Revision Hash 分离。

同一版本内容不同：

-开发模式允许 Dev Revision；
-生产包视为冲突。

---

## 二十、历史数据

可迁移：

-用户启用状态；
-Scope；
-来源；
-自定义名称；
-用户修改；
-激活偏好。

不迁移为真值：

-旧缓存；
-某轮当前 Active；
-旧 Token 估算；
-旧 Loader 内存状态。

---

## 二十一、前端

统一页面展示：

-来源；
-Extension；
-版本；
-Enabled；
-Scope；
-依赖；
-资源；
-Token；
-激活策略；
-当前可激活原因；
-最近使用；
-用户修改；
-冲突。

---

## 二十二、迁移过程

建议批次：

1.内置 Agent Skill；
2.用户导入 Skill；
3.扩展附带 Skill；
4.角色专属 Skill；
5.带 MCP 依赖 Skill；
6.旧混合 Skill；
7.清理旧注入链。

---

## 二十三、兼容层

旧 API：

```text
ListSkills
EnableSkill
DisableSkill
GetSkillContent
```

映射：

-List → AgentSkillCatalog；
-Enable/Disable → EnablementService；
-Content → ContextProvider/Resource Reader。

禁止旧 API 直接写旧表。

---

## 二十四、迁移报告

必须列出：

-已迁移；
-分类为 Tool；
-分类为 Workflow；
-分类为 MCP Wrapper；
-Unknown；
-损坏；
-资源缺失；
-Scope 冲突；
-Enabled 冲突；
-依赖缺失；
-用户修改；
-重复 ID；
-仍走旧 Prompt 注入的入口。

---

## 二十五、测试要求

覆盖：

-内置；
-用户；
-角色；
-资源；
-MCP 依赖；
-Tool 依赖；
-Enabled；
-Scope；
-Token Budget；
-渐进加载；
-多个 Skill；
-冲突；
-用户 Fork；
-更新；
-卸载；
-旧 API；
-唯一 Prompt 注入；
-跨平台路径；
-恶意 Markdown；
-性能。

---

## 二十六、实施任务

1. 输出 Agent Skill 全量清单。
2. 分类旧 Skill。
3. 建立稳定 ID 映射。
4. 建立 system 与 Synthetic Extension。
5.迁移 SKILL.md。
6.迁移资源。
7.迁移 Enabled/Scope。
8.迁移依赖。
9.提取嵌入 MCP。
10.接入 AgentSkillCatalog。
11.接入 ContextProvider。
12.迁移 Prompt Assembly。
13.实现用户 Fork。
14.冻结旧 Skill 写入。
15.改造前端。
16.完成回归和迁移报告。

---

## 二十七、验收标准

1. 所有 Agent Skill 有 ContributionDefinition。
2. Agent Skill 不再作为 Tool。
3. Agent Skill 不执行代码。
4. MCP 依赖不由 Loader 安装。
5. Enabled、Scope、Selected 分离。
6. Prompt 只有一个注入路径。
7.渐进加载和 Token Budget 生效。
8.用户修改不会被扩展更新覆盖。
9.旧 Skill 表停止新写。
10.旧 Prompt 注入入口全部可统计并清理。
11.关键测试通过。
12.可进入第 51 步 MCP 迁移。

---

## 二十八、执行约束

> Agent Skill 是指令和资源包，不是执行容器、插件、Workflow 或 MCP Tool 的统一名称。

禁止：

-Skill Handler；
-自动安装 MCP；
-全量 Prompt 永久注入；
-未选中等于 Disabled；
-角色 Skill 迁 Global；
-用户修改被覆盖；
-新旧 Prompt 双注入。
