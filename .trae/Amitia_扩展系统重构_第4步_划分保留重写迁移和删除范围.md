# Amitia 扩展系统重构第 4 步实施文档

## 第 4 步：划分保留、重写、迁移和删除范围

---

## 一、步骤目标

在第 2 步已经建立现有系统调用链地图、第 3 步已经建立数据表与资源归属清单的基础上，对当前 Amitia 的 Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` 扩展包、Workshop、扩展中心及相关运行基础设施进行正式分类。

本步骤必须把现有实现划分为以下四类：

1. **保留并抽取**
2. **改造后复用**
3. **仅用于迁移**
4. **最终删除**

本步骤的目标不是立即修改代码，而是建立后续重构的唯一处理边界，避免出现以下问题：

- 把有价值的底层能力一并重写；
- 为保留历史接口而继续扩展兼容层；
- 新旧系统长期双写；
- 同一功能既保留旧实现又新增一套新实现；
- 删除仍被真实调用的桥接代码；
- 迁移完成后忘记删除旧运行时；
- 新 Extension Kernel 被迫适配错误的旧领域模型。

完成本步骤后，项目必须能够明确回答：

> 当前每一个扩展相关文件、类型、数据表、API、页面、运行资源和测试，最终要被保留、改造、迁移还是删除。

---

## 二、四类处理定义

## 1. 保留并抽取

指现有能力本身设计合理、实现成熟，并且不依赖旧领域模型，可以从原模块抽离为新 Extension Kernel 的基础设施。

这类代码应：

- 保留核心行为；
- 保留现有测试；
- 抽离对旧 Skill、Plugin、Package 等类型的依赖；
- 改造成通用组件；
- 接入新领域模型；
- 不继续承担旧系统生命周期职责。

典型候选包括：

- 包归档安全；
- 路径穿越防护；
- Checksum；
- 签名验证；
- Secret 加密；
- MCP 标准协议 Client；
- MCP Transport；
- OAuth；
- JSON Schema 校验；
- 超时与取消；
- Panic 恢复；
- 幂等；
-执行审计；
-熔断；
-资源限制；
-版本比较；
-原子安装；
-回滚基础能力。

“保留并抽取”不等于原文件完全不改，而是其核心行为和测试价值应被保留。

---

## 2. 改造后复用

指现有能力具有业务价值，但当前领域定义、生命周期、接口或依赖方向不适合新架构，需要在新模型下重构后继续使用。

这类代码通常具有以下特征：

- 业务能力正确；
- 抽象名称错误；
- 与旧 Registry 强耦合；
- 生命周期分散；
- 所有权不清晰；
- 需要替换输入输出类型；
- 需要接入统一 Permission Broker；
- 需要接入统一 Runtime Supervisor；
- 需要从 Skill 模型改成 Capability、Contribution 或 Resource。

典型候选包括：

- Agent Skill 解析器；
- Agent Skill 资源索引；
- Agent Skill Token 预算；
- Workflow Compiler；
- Workflow Executor；
- Extension Executor；
- PermissionEvaluator；
- Plugin Hook 超时与熔断逻辑；
- Plugin 状态 CAS；
- Package 安装和回滚流程；
- MCP Discovery；
- MCP Task；
-扩展执行记录；
-前端 Schema Surface 渲染器。

“改造后复用”必须明确改造目标，不能只写“后续优化”。

---

## 3. 仅用于迁移

指现有实现不应进入新架构，但在旧数据、旧配置、旧扩展包或旧用户状态迁移完成前仍然需要保留。

这类代码必须：

- 进入只读或冻结状态；
- 禁止新增能力；
- 仅供旧数据识别、转换和导出；
- 有明确删除条件；
- 不允许成为新系统的永久兼容层；
- 不参与新数据写入；
- 不允许新 Extension Kernel 反向依赖。

典型候选包括：

- Manifest v1 Parser；
- 旧 `.amitiax` Workflow/Instructions 解析；
- Agent Skill 到 `SkillDefinition` 的适配器；
- MCP Tool 到 `SkillDefinition` 的适配器；
- 旧 Scope Binding 读取逻辑；
- 旧 Package Version 数据读取；
- 旧 Plugin 状态读取；
- 旧 SkillDefinition 数据导出；
- 旧扩展中心 API 兼容入口；
- 旧前端页面的数据导出能力。

所有“仅用于迁移”的代码必须标注：

```text
迁移来源
迁移目标
允许调用者
禁止调用者
停止写入时间
删除步骤
删除条件
```

---

## 4. 最终删除

指现有实现的职责将被新 Extension Kernel 完全替代，或者本身属于错误抽象、重复系统、未接通设计、无用入口或长期历史兼容层。

这类代码应：

- 在迁移完成后彻底删除；
- 不继续增加测试和功能；
- 不被新系统引用；
- 不继续维护双写；
- 不保留“保险兼容”；
- 清理对应路由、页面、数据表和文档。

典型候选包括：

- Go Factory 第三方 Plugin 模型；
- `PluginRegistry` 的 builtin-only 入口；
- `PluginManager` 作为独立插件生命周期所有者；
- MCP Skill Runtime Adapter；
- Agent Skill 伪 Skill 注册；
- 旧 Skill 概念过载结构；
- 只支持 Workflow/Instructions 的旧包安装主链；
- 多套 Enabled 状态同步逻辑；
- 分散的恢复编排；
- 旧扩展中心分散页面；
- 无真实运行入口的 Manifest Plugin 分支；
- 重复的审计表；
- 重复的权限判断；
- 已不可达路由和 API。

---

## 三、分类原则

## 1. 按职责分类，不按文件分类

一个文件中可能同时存在：

- 应保留的安全函数；
- 应改造的业务逻辑；
- 仅用于迁移的解析逻辑；
- 应删除的旧适配器。

不得为了方便将整个文件统一归类。

必须至少细化到：

- 类型；
-函数；
-接口；
-数据结构；
-路由；
-Repository 方法；
-前端组件；
-数据库表；
-测试文件。

---

## 2. 按目标架构价值分类

判断依据不是“当前代码是否能运行”，而是：

- 是否符合统一 Extension Kernel；
- 是否符合统一生命周期；
- 是否符合统一资源所有权；
- 是否符合统一权限；
- 是否符合 `.amitiax` v2；
- 是否支持第三方插件运行时；
- 是否适合 Electron 桌面端；
- 是否能被独立测试；
- 是否会继续传播旧概念。

---

## 3. 不因重写成本高而错误保留

以下理由不能单独成为保留依据：

- 文件很多；
-代码很长；
-已经用了很久；
-改动风险大；
-测试不足；
-其他模块依赖它；
-重写成本高。

如果抽象错误，必须进入“改造后复用”或“最终删除”，不能因为成本而保留为核心架构。

---

## 4. 不因代码旧而错误删除

以下能力即使位于旧模块，也不能直接删除：

- 安全验证；
-签名；
-包归档防护；
-Secret 加密；
-MCP 协议实现；
-回滚；
-幂等；
-审计；
-超时与取消；
-熔断；
-资源清理；
-测试工具。

必须先评估能否抽取。

---

## 5. 迁移代码必须有终点

任何“为了兼容旧用户”的代码都必须有：

- 迁移完成标识；
- 迁移版本；
-统计指标；
-停止写入条件；
-删除步骤；
-失败回退；
-不再调用的验证方式。

禁止出现：

```text
暂时保留，后续再看
```

---

## 四、判定维度

每个对象按以下维度评分。

| 维度 | 说明 |
|---|---|
| 业务价值 | 是否仍是新系统需要的能力 |
| 协议价值 | 是否实现标准协议或通用规范 |
| 安全价值 | 是否包含成熟安全保障 |
| 数据价值 | 是否承载用户资产或历史记录 |
| 可抽离性 | 是否能脱离旧领域模型 |
| 耦合程度 | 是否强依赖 Skill/Plugin/MCP 旧模型 |
| 生命周期正确性 | 是否有明确创建、恢复、关闭和清理 |
| 所有权正确性 | 是否有唯一资源所有者 |
| 可测试性 | 是否有稳定测试或可独立测试 |
| 重复程度 | 是否与其他系统重复 |
| 接通程度 | 是否真实可达 |
| 迁移必要性 | 是否需要读取旧数据 |
| 删除风险 | 删除是否影响现有用户数据 |
| 新架构适配度 | 是否适合 Extension Kernel |

---

## 五、分类决策规则

### 1. 保留并抽取判定

满足多数以下条件：

- 属于通用基础设施；
-不依赖旧领域概念；
-有成熟测试；
-没有重复实现；
-能独立封装；
-新系统仍明确需要；
-安全或协议重写风险高；
-数据行为稳定。

### 2. 改造后复用判定

满足多数以下条件：

- 业务能力仍需要；
-当前类型命名或边界错误；
-依赖旧 Registry；
-生命周期需要统一；
-权限需要统一；
-数据结构可以迁移；
-测试可保留；
-无需完全重写算法。

### 3. 仅用于迁移判定

满足多数以下条件：

- 新系统不应继续使用；
-旧数据仍依赖；
-只能读取旧格式；
-不应再写入；
-可被一次性转换；
-删除前需要确认迁移完成；
-没有长期产品价值。

### 4. 最终删除判定

满足任一高优先条件：

- 属于错误抽象；
-属于重复运行时；
-属于未接通实现；
-新系统已有唯一替代；
-会造成双写；
-会造成多套生命周期；
-没有真实用户数据；
-只是诊断或样例；
-前端入口不可达；
-长期兼容成本高于迁移成本；
-安全边界不适合第三方插件。

---

## 六、必须审查的后端范围

## 1. Extension Runtime 与 Registry

重点文件：

```text
backend/internal/extension/runtime.go
backend/internal/extension/registry.go
backend/internal/extension/service.go
backend/internal/extension/router.go
```

需要分别判断：

- Runtime 总装配；
- Registry 数据结构；
- Tool/Skill ID；
-模型工具转换；
-作用域过滤；
-启停状态；
-恢复逻辑；
-关闭逻辑；
-对 Chat 的接口；
-对 MCP 的注册入口；
-对 Plugin 的注册入口；
-对 Workflow 的注册入口。

重点判断：

- 哪些执行保障可以保留；
-哪些 Registry 结构需要替换；
-哪些 Scope 逻辑仅用于迁移；
-哪些 API 最终删除。

---

## 2. Executor 与 Permission

重点文件：

```text
backend/internal/extension/executor.go
backend/internal/extension/permission.go
backend/internal/extension/side_effect.go
```

重点判断：

- 参数校验；
-超时；
-取消；
-并发；
-幂等；
-副作用；
-审计；
-权限规则；
-风险等级；
-作用域；
-结果标准化。

目标通常应是：

```text
保留执行安全能力
+
改造领域输入输出
+
删除 Skill 专属假设
```

---

## 3. Legacy Tool Adapter

重点文件：

```text
backend/internal/extension/legacy_tool_adapter.go
backend/internal/agent/tool/
```

重点判断：

- 旧工具本体是否继续保留；
-Adapter 是否仅用于迁移；
-哪些旧工具应改造为原生 Capability；
-哪些工具可以直接删除；
-哪些 Tool Schema 转换逻辑可复用。

预期分类通常为：

- 内置工具业务能力：改造后复用；
- Legacy Adapter：仅用于迁移；
- 旧 SkillDefinition 包装：最终删除。

---

## 4. Agent Skill

重点文件：

```text
backend/internal/extension/agent_skill_parser.go
backend/internal/extension/agent_skill_service.go
backend/internal/extension/agent_skill_runtime.go
backend/internal/extension/agent_skill_handler.go
backend/internal/extension/agent_skill_repository.go
backend/internal/extension/agent_skill_protocol.go
```

重点判断：

### 可能保留并抽取

- SKILL.md 解析；
-Frontmatter；
-资源索引；
-安全解压；
-Token 限制；
-资源渐进读取；
-兼容 OpenAI Agent Skills 元数据。

### 可能改造后复用

- Catalog；
-激活；
-作用域；
-Tool Mapping；
-MCP 依赖声明；
-Prompt 注入；
-缓存。

### 仅用于迁移

- 旧 Metadata 表读取；
-旧 Scope Binding；
-旧 Agent Skill 包导入记录；
-旧激活记录转换。

### 最终删除

- Agent Skill → `SkillDefinition`；
-无 Handler 的伪 Skill 注册；
-内部激活工具作为统一 Registry 核心对象；
-旧 Round State 与新系统重复的部分。

---

## 5. MCP

重点目录：

```text
backend/internal/mcp/
backend/internal/mcpapi/
```

### 可能保留并抽取

- Client；
-Transport；
-stdio；
-Streamable HTTP；
-OAuth；
-Token Store；
-JSON-RPC；
-Discovery；
-Resources；
-Prompts；
-Tasks；
-Sampling；
-Elicitation；
-Roots；
-Completion；
-Connection Factory；
-协议错误处理。

### 可能改造后复用

- Connection Manager；
-Server Repository；
-Discovery 保存；
-Tool Allowlist；
-角色作用域；
-重连；
-Task 管理；
-Feature Service；
-Dependency Service。

### 仅用于迁移

- 旧 MCP Server 表读取；
-旧角色绑定；
-旧 Tool Enabled；
-旧 Dependency Link；
-旧 OAuth Token 引用转换。

### 最终删除

```text
backend/internal/mcp/skill/runtime.go
```

以及所有：

- MCP Tool → SkillDefinition；
-MCP API 直接写 Extension Registry；
-重复启用状态同步；
-独立扩展生命周期；
-重复审计模型。

---

## 6. Plugin

重点文件：

```text
backend/internal/extension/plugin_protocol.go
backend/internal/extension/plugin_registry.go
backend/internal/extension/plugin_manager.go
backend/internal/extension/plugin_host.go
backend/internal/extension/plugin_service.go
backend/internal/extension/plugin_repository.go
backend/internal/extension/plugin_surface.go
backend/internal/extension/plugin_builtin_diagnostic.go
```

### 可能保留并抽取

- Hook 超时；
-并发控制；
-熔断；
-事件深度限制；
-状态 CAS；
-命名空间；
-调度隔离；
-执行审计；
-资源限制思想。

### 可能改造后复用

- Plugin State；
-Event Delivery；
-Schedule；
-Surface Schema；
-Host API 权限校验；
-插件配置；
-运行日志。

### 仅用于迁移

- 旧 Plugin 状态表；
-旧 Schedule；
-旧 Event；
-旧 Enabled 状态；
-旧诊断数据。

### 最终删除

- Go Interface 作为第三方插件协议；
-Plugin Factory；
-builtin-only Registry；
-旧 PluginManager 生命周期；
-当前诊断 Plugin；
-插件详情页专用 Surface 绑定；
-Plugin 注册 Skill 的旧路径。

---

## 7. Workflow

重点文件：

```text
backend/internal/extension/workflow_compiler.go
backend/internal/extension/workflow_executor.go
backend/internal/extension/workflow_values.go
backend/internal/extension/workflow_*.go
```

### 可能保留并抽取

- Workflow Schema；
-Compiler；
-Value Resolver；
-节点执行算法；
-错误定位；
-运行上下文；
-循环检测；
-输入输出校验。

### 可能改造后复用

- Host；
-调用 Tool；
-Schedule；
-Notification；
-Memory Candidate；
-Context Contribution；
-权限；
-运行记录。

### 仅用于迁移

- 旧 Workflow 数据读取；
-旧包内 Workflow Entry；
-旧 Registry 注册；
-旧运行记录转换。

### 最终删除

- Workflow → SkillDefinition；
-Workflow 独立安装生命周期；
-重复权限；
-旧 Host 注入结构中泄漏的 Chat/Memory 具体实现。

---

## 8. Package 与 `.amitiax` v1

重点文件：

```text
backend/internal/extension/package_archive.go
backend/internal/extension/package_parser.go
backend/internal/extension/package_service.go
backend/internal/extension/package_installer.go
backend/internal/extension/package_lifecycle.go
backend/internal/extension/package_recovery.go
backend/internal/extension/package_repository.go
backend/internal/extension/schema/manifest.schema.json
```

### 可能保留并抽取

- Archive 安全；
-路径校验；
-文件类型限制；
-Checksum；
-签名；
-发布者信任；
-版本比较；
-安装事务；
-补偿；
-升级；
-回滚；
-Artifact；
-恢复；
-卸载预览；
-依赖检查基础能力。

### 可能改造后复用

- PackageService；
-版本模型；
-Artifact Store；
-Operation Audit；
-Config Migration；
-Import Session；
-依赖解析；
-安装预览。

### 仅用于迁移

- Manifest v1；
-Workflow/Instructions Entry；
-旧安装记录；
-旧版本记录；
-旧 Artifact；
-旧导出格式；
-旧 Package API。

### 最终删除

- 旧 Parser 主路径；
-二选一包模型；
-只支持 Workflow/Instructions 的 Installer；
-Manifest 中未接通的 Plugin 分支；
-旧包类型特判；
-旧 Package 与 Agent Skill/Workflow 的硬编码分支。

---

## 9. Workshop

重点文件：

```text
backend/internal/extension/workshop_*.go
```

重点判断：

- 会话与 Revision 是否仍需要；
-AI 生成能力是否可以成为 `.amitiax` 开发器；
-测试运行器是否可改造成插件开发测试器；
-旧 Skill/Workflow 生成逻辑是否只用于迁移；
-Workshop Installer 是否应删除并统一走 Package Manager；
-前端“技能制作”是否应重建为“扩展开发者模式”。

---

## 七、必须审查的前端范围

重点目录：

```text
front/src/views/extensions/
front/src/views/mcp/
front/src/router/index.ts
front/src/navigation/app-nav.ts
front/src/components/SideNav.vue
front/src/stores/
```

每个页面、组件、Store、API Client 必须分类。

### 可能保留并抽取

- 通用表格；
-权限确认；
-安装预览；
-日志展示；
-版本历史；
-Schema UI 控件；
-错误边界；
-Loading/Empty/Error 状态。

### 可能改造后复用

- 扩展中心布局；
-扩展详情页；
-安装界面；
-本地包导入；
-权限管理；
-运行记录；
-开发者模式；
-Plugin Surface Renderer。

### 仅用于迁移

- 旧 Skill 页面；
-旧 MCP 页面；
-旧 Plugin 页面；
-旧 Agent Skill 页面；
-旧 Package 页面；
-旧 API Client；
-旧路由跳转。

### 最终删除

- 分散的扩展子系统入口；
-重复详情页；
-旧 Skill 概念 UI；
-不可达页面；
-只服务诊断 Plugin 的 UI；
-直接写旧 Enabled 状态的控件；
-旧 Workshop Skill 制作页面。

---

## 八、必须审查的数据表范围

每张表必须分成以下处理方式之一：

```text
保留原表并改名
保留数据并迁入新表
只读保留历史
迁移完成后删除
直接删除空表
```

不得仅写“保留”或“删除”。

每张表必须记录：

- 当前数据量；
-是否存在用户数据；
-是否存在敏感数据；
-是否参与恢复；
-是否参与回滚；
-是否被多个系统写入；
-目标新表；
-迁移方式；
-删除条件；
-备份要求。

---

## 九、分类记录格式

每个对象使用统一格式。

```text
对象编号：
对象类型：
当前系统：
```

| 字段 | 内容 |
|---|---|
| 文件/表/路由 | 实际路径或名称 |
| 类型/函数/组件 | 具体对象 |
| 当前职责 | 实际运行职责 |
| 当前调用者 | 真实调用方 |
| 当前依赖 | 依赖对象 |
| 数据所有权 | 当前所有者 |
| 目标分类 | 保留并抽取 / 改造后复用 / 仅用于迁移 / 最终删除 |
| 判定依据 | 具体原因 |
| 目标组件 | Extension Kernel 中归属 |
| 前置条件 | 何时可处理 |
| 迁移要求 | 数据或行为迁移 |
| 删除条件 | 删除前必须满足 |
| 测试保留 | 哪些测试可复用 |
| 风险等级 | P0/P1/P2/P3 |
| 备注 | 其他信息 |

---

## 十、必须输出的分类矩阵

## 1. 后端文件分类矩阵

至少覆盖：

- Extension；
-Agent Skill；
-MCP；
-MCP API；
-Plugin；
-Workflow；
-Package；
-Workshop；
-Migration；
-Chat/Interaction 集成；
-服务装配。

## 2. 前端文件分类矩阵

至少覆盖：

- 路由；
-导航；
-页面；
-组件；
-Store；
-API；
-类型；
-权限弹窗；
-安装弹窗；
-日志页面。

## 3. 数据表分类矩阵

每张扩展相关表必须有明确处理方式。

## 4. API 分类矩阵

每个旧 API 必须标记：

- 保留并改名；
-兼容迁移；
-替换；
-删除；
-无调用可直接删除。

## 5. 测试分类矩阵

每个测试文件必须标记：

- 直接保留；
-改造成新内核测试；
-仅用于迁移验证；
-删除；
-需要补充。

## 6. 运行资源分类矩阵

覆盖：

- Worker；
-goroutine；
-Cache；
-Registry；
-Connection；
-Subprocess；
-Timer；
-Queue；
-Hook；
-Schedule。

---

## 十一、目标 Extension Kernel 组件映射

所有保留或改造对象必须映射到以下目标组件之一：

```text
Package Manager
Package Store
Manifest Parser
Dependency Resolver
Runtime Supervisor
Contribution Registry
Tool Registry
Agent Skill Catalog
MCP Manager
Workflow Engine
UI Contribution Registry
Hook Pipeline
Permission Broker
Scope Manager
Storage Broker
Secret Broker
Event Bus
Schedule Manager
Audit Store
Migration Manager
Developer Tooling
```

若某对象无法映射，必须说明：

- 新内核是否缺少组件；
-该对象是否应删除；
-是否属于产品层而非内核层。

---

## 十二、冲突判定规则

当一个对象同时符合多个分类时，按以下优先级处理：

### 1. 用户数据优先

包含用户资产的数据读取能力不得直接删除，至少进入“仅用于迁移”。

### 2. 安全能力优先

安全、加密、签名、归档防护优先进入“保留并抽取”。

### 3. 标准协议优先

标准 MCP 协议实现优先保留，产品级生命周期可改造。

### 4. 错误抽象不保留

即使代码成熟，只要其核心领域抽象错误，也不得进入“保留并抽取”，应进入“改造后复用”。

### 5. 重复生命周期必须删除

任何会让新系统出现第二套生命周期的旧 Manager，最终必须删除。

### 6. 兼容适配只允许迁移

所有旧模型到新模型的桥接层，只能是迁移层，不能长期存在。

---

## 十三、实施任务

### Task 1：建立分类判定模板

创建统一表格和对象编号规范。

### Task 2：分类 Extension Runtime 与执行链

完成 Registry、Executor、Permission、Service、Router、Legacy Adapter 的逐对象分类。

### Task 3：分类 Agent Skill

完成 Parser、Service、Runtime、Handler、Repository、Protocol、缓存、激活和资源读取分类。

### Task 4：分类 MCP

完成协议基础设施、Manager、Discovery、Skill Adapter、Dependency、API 和数据表分类。

### Task 5：分类 Plugin

完成 Protocol、Registry、Manager、Host、Surface、Event、Schedule、State 和诊断 Plugin 分类。

### Task 6：分类 Workflow

完成 Compiler、Executor、Host、Values、Repository、运行记录和适配层分类。

### Task 7：分类 Package 与 `.amitiax` v1

完成 Archive、Parser、Installer、Lifecycle、Recovery、Repository、Manifest、Artifact 和 API 分类。

### Task 8：分类 Workshop

完成 Session、Revision、Test Runner、Installer、AI 生成和前端页面分类。

### Task 9：分类前端扩展中心

完成路由、导航、页面、组件、Store 和 API Client 分类。

### Task 10：分类数据库表

完成每张表的保留、迁移、历史保留或删除决策。

### Task 11：分类测试

明确哪些测试直接保留、改造成新内核测试、仅用于迁移或删除。

### Task 12：形成删除依赖图

列出每个最终删除对象的前置迁移条件和依赖解除顺序。

---

## 十四、建议文档结构

建议新增：

```text
docs/extension-kernel/
├── 04-retain-rewrite-migrate-delete.md
├── classification/
│   ├── backend-extension.md
│   ├── backend-agent-skill.md
│   ├── backend-mcp.md
│   ├── backend-plugin.md
│   ├── backend-workflow.md
│   ├── backend-package.md
│   ├── backend-workshop.md
│   ├── frontend.md
│   ├── database.md
│   ├── api.md
│   ├── tests.md
│   └── runtime-resources.md
├── matrices/
│   ├── classification-summary.md
│   ├── target-component-map.md
│   ├── migration-only-map.md
│   └── deletion-dependency-map.md
└── reports/
    ├── preserve-extract.md
    ├── refactor-reuse.md
    ├── migration-only.md
    └── final-delete.md
```

---

## 十五、风险等级

### P0：分类错误会导致数据或安全问题

包括：

- 将用户资产表误判为可删除；
-将 Secret 迁移链误删；
-将回滚 Artifact 删除；
-将 MCP Token Store 重写但无迁移；
-删除仍参与启动恢复的组件。

### P1：分类错误会导致双系统长期存在

包括：

- 旧 PluginManager 被错误保留；
-MCP Skill Adapter 被错误保留；
-旧 Package Installer 被新系统继续调用；
-旧 Scope Binding 继续写入；
-旧权限系统继续判定。

### P2：分类错误会增加重构成本

包括：

- 可抽取基础设施被完全重写；
-错误保留前端页面；
-测试无法复用；
-Repository 边界未提前拆分。

### P3：分类文档不完整

包括：

- 只按文件分类；
-未写删除条件；
-未写目标组件；
-未写数据迁移；
-未写调用者。

---

## 十六、本步骤不做的事情

本步骤明确不做：

- 不修改源代码；
-不移动文件；
-不重命名类型；
-不删除旧接口；
-不增加新接口；
-不修改数据库；
-不创建新表；
-不写迁移脚本；
-不实现 Extension Kernel；
-不实现 `.amitiax` v2；
-不重构前端；
-不开始删除旧页面；
-不修复现有问题；
-不调整测试逻辑。

---

## 十七、验收产物

完成后必须提交：

### 1. 分类主文档

```text
docs/extension-kernel/04-retain-rewrite-migrate-delete.md
```

### 2. 完整对象分类清单

必须覆盖：

- 后端文件；
-类型；
-函数；
-接口；
-Repository；
-数据表；
-前端页面；
-路由；
-API；
-Store；
-运行资源；
-测试。

### 3. 保留并抽取清单

每项必须写：

- 抽取目标；
-需解除的依赖；
-保留测试；
-目标组件。

### 4. 改造后复用清单

每项必须写：

- 当前错误边界；
-目标新模型；
-需要替换的接口；
-需要删除的旧依赖。

### 5. 仅用于迁移清单

每项必须写：

- 迁移来源；
-迁移目标；
-停止写入时间；
-允许调用者；
-删除步骤；
-删除条件。

### 6. 最终删除清单

每项必须写：

- 替代组件；
-依赖解除顺序；
-数据迁移条件；
-测试替代；
-删除步骤。

### 7. 目标组件映射表

所有保留或改造对象都必须映射到 Extension Kernel 的目标组件。

### 8. 删除依赖图

明确哪些旧对象必须在其他迁移完成后才能删除。

### 9. 决策争议清单

对于无法直接分类的对象，必须列出：

- 冲突原因；
-候选分类；
-需要补充的运行验证；
-最终决策负责人。

---

## 十八、验收标准

本步骤通过必须同时满足：

1. 所有扩展相关后端文件已分类。
2. 所有扩展相关前端页面、路由和 API 已分类。
3. 所有扩展相关数据表已分类。
4. 所有运行时资源已分类。
5. 所有测试已分类。
6. 每个对象只有一个主分类。
7. “保留并抽取”对象已明确抽取目标。
8. “改造后复用”对象已明确目标模型。
9. “仅用于迁移”对象已明确删除条件。
10. “最终删除”对象已明确替代组件。
11. 没有使用“暂时保留”“后续再看”等模糊结论。
12. 没有把整个大文件粗略归为一类。
13. 用户数据和 Secret 迁移风险已单独审查。
14. 删除依赖顺序已明确。
15. 本步骤未修改任何运行行为。
16. 第 5 步可以直接依据本分类补齐基线测试。

---

## 十九、退出条件

只有满足以下条件后，才能进入第 5 步“补齐现有系统基线测试”：

- 后端、前端、数据库和运行资源分类全部完成；
-保留能力已明确抽取目标；
-改造能力已明确新领域模型；
-迁移代码已明确停止写入和删除条件；
-删除对象已明确替代和依赖解除顺序；
-所有用户数据表已有处理决策；
-所有 Secret 已有处理决策；
-所有无法分类的对象已形成阻塞清单；
-本步骤未提前实施重构。

---

## 二十、执行约束

执行本步骤时必须遵守：

> 分类的目的不是减少改动量，而是确保新 Extension Kernel 建立后，系统只剩一套正确的扩展模型、生命周期、权限、资源所有权和用户入口。

任何对象只要会导致以下结果之一，就不得进入长期保留：

- 第二套 Registry；
-第二套权限；
-第二套生命周期；
-第二套安装器；
-第二套 UI 扩展系统；
-第二套 MCP Tool 适配；
-第二套 Agent Skill 激活；
-第二套启用状态；
-永久双写；
-永久兼容层。

所有分类结论必须基于第 2 步调用链和第 3 步资源归属，不得脱离源码证据单独判断。
