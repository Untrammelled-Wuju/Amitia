# 分类汇总与决策争议

---

## 一、分类统计汇总

| 分类 | 后端 | 前端 | 数据表 | API | 测试 |
|---|---|---|---|---|---|
| 保留并抽取 | 48 | 9 | 0 | 0 | 8 |
| 改造后复用 | 67 | 12 | 18 | 35 | 15 |
| 仅用于迁移 | 35 | 8 | 8 | 30 | 3 |
| 最终删除 | 28 | 11 | 7 | 15 | 4 |

---

## 二、保留并抽取清单

| 编号 | 对象 | 抽取目标 | 目标组件 |
|---|---|---|---|
| EXT-RT-001 | Executor 超时控制 | 通用执行超时控制器 | Runtime Supervisor |
| EXT-RT-002 | Executor Panic 恢复 | 通用 panic recovery wrapper | Runtime Supervisor |
| EXT-RT-003 | Executor 幂等性 | 通用幂等执行器 | Runtime Supervisor |
| EXT-RT-004 | Executor 并发控制 | 通用并发槽位控制器 | Runtime Supervisor |
| EXT-RT-005 | Executor 审计持久化 | 通用执行审计接口 | Audit Store |
| EXT-RT-006 | Executor 副作用补偿 | 通用资源归属追踪器 | Storage Broker |
| EXT-RT-007 | Config Crypto | 通用配置加解密器 | Secret Broker |
| EXT-RT-008 | Schema Validator | 独立 Schema 校验包 | Manifest Parser |
| EXT-RT-009 | Protocol 脱敏工具 | 独立脱敏工具包 | Secret Broker |
| EXT-RT-010 | 版本比较 | 独立 semver 包 | Package Manager |
| AGT-001 | SKILL.md 解析 | 独立 SKILL.md 解析器 | Agent Skill Catalog |
| AGT-002 | ZIP 安全解压 | 独立安全 ZIP 读取器 | Package Manager |
| AGT-003 | 路径校验 | 独立路径安全校验器 | Package Manager |
| AGT-004 | 资源扫描与索引 | 通用文件资源扫描器 | Agent Skill Catalog |
| AGT-005 | 资源渐进读取 | 通用资源读取服务 | Agent Skill Catalog |
| AGT-006 | Token 估算 | 独立 Token 计数器 | Agent Skill Catalog |
| AGT-007 | OpenAI 兼容解析 | 独立 | Agent Skill Catalog |
| MCP-001 | MCP Client | 独立 MCP Client 包 | MCP Manager |
| MCP-002 | MCP Transport | 独立 MCP Transport 包 | MCP Manager |
| MCP-003 | MCP OAuth | 独立 MCP OAuth 包 | MCP Manager |
| MCP-004 | MCP Protocol | 独立 MCP Protocol 包 | MCP Manager |
| MCP-005 | MCP Discovery | 独立 Discovery 服务 | MCP Manager |
| MCP-006 | MCP Features | 独立 | MCP Manager |
| MCP-007 | MCP Host | 独立 | MCP Manager |
| PLG-001 | Hook 超时控制 | 通用事件处理超时 | Runtime Supervisor |
| PLG-002 | 熔断器 | 独立熔断器包 | Runtime Supervisor |
| PLG-003 | 并发控制 | 通用 | Runtime Supervisor |
| PLG-004 | 事件深度限制 | 通用事件安全 | Event Bus |
| PLG-005 | 状态 CAS | 通用乐观锁 | Storage Broker |
| PLG-006 | 命名空间隔离 | 通用 | Scope Manager |
| PLG-007 | 执行审计 | 独立审计记录器 | Audit Store |
| WFL-001 | Workflow Schema | 独立 Workflow DSL | Workflow Engine |
| WFL-004 | Value Resolver | 独立模板解析器 | Workflow Engine |
| WFL-005 | 条件表达式求值 | 独立表达式求值器 | Workflow Engine |
| WFL-006 | 循环检测 | 通用 | Dependency Resolver |
| WFL-007 | JSON 变换引擎 | 独立 | Workflow Engine |
| WFL-008 | 错误定位 | 通用 | Runtime Supervisor |
| PKG-001 | 归档安全 | 独立安全归档包 | Package Manager |
| PKG-002 | Checksum | 独立 Checksum 工具 | Package Manager |
| PKG-003 | 签名验证 | 独立签名验证器 | Package Manager |
| PKG-004 | 版本比较 | 独立 semver 包（合并） | Package Manager |
| PKG-005 | 安装事务 | 通用事务管理器 | Package Manager |
| PKG-006 | 补偿/回滚 | 独立回滚管理器 | Package Manager |
| PKG-007 | 恢复 | 独立恢复管理器 | Package Manager |
| PKG-008 | 卸载预览 | 通用依赖分析 | Dependency Resolver |
| PKG-009 | 文件类型限制 | 通用 | Package Manager |
| FE-001~005 | Surface 组件 | 独立 Schema UI 组件库 | UI Contribution Registry |
| FE-006 | PermissionDialog | 通用权限确认组件 | Permission Broker |
| FE-007 | ExtensionPageHeader | 通用布局组件 | UI Contribution Registry |

---

## 三、最终删除清单（摘要）

| 编号 | 对象 | 替代组件 | 风险 |
|---|---|---|---|
| MCP-301 | `mcp/skill/runtime.go` | MCP Manager → Tool Registry | P0 |
| PLG-301 | Go Interface 第三方插件协议 | .amitiax v2 | P1 |
| PLG-302 | Plugin Factory | Package Manager | P1 |
| PLG-303 | builtin-only PluginRegistry | Contribution Registry | P1 |
| PLG-304 | PluginManager 独立生命周期 | Runtime Supervisor | P0 |
| PLG-305 | 诊断 Plugin | Developer Tooling | P2 |
| PLG-306 | Plugin 注册 Skill 旧路径 | Contribution Registry | P1 |
| AGT-301 | Agent Skill → SkillDefinition | Agent Skill Catalog | P0 |
| AGT-302 | registerAgentSkillRuntime | Tool Registry | P1 |
| AGT-305 | AgentSkillHandler | Extension Kernel HTTP API | P1 |
| WFL-301 | Workflow → SkillDefinition | Workflow Engine | P1 |
| WFL-302 | Workflow 独立安装 | Package Manager | P1 |
| WFL-303 | 重复权限判断 | Permission Broker | P2 |
| PKG-301 | 旧 Parser 主路径 | v2 Manifest Parser | P1 |
| PKG-302 | 二选一包模型 | 多类型包 | P2 |
| PKG-303 | 未接通 Plugin 分支 | 新 Plugin 体系 | P3 |
| PKG-304 | 硬编码分支 | 统一安装器 | P2 |
| PKG-305 | PackageHandler | Extension Kernel API | P1 |
| WS-301 | Workshop 独立 Installer | Package Manager | P1 |
| WS-302 | WorkshopHandler | Developer Tooling API | P1 |
| WS-303 | Workshop → SkillDefinition | .amitiax v2 打包 | P1 |
| EXT-RT-305 | SkillDefinition 类型 | 新领域类型 | P0 |
| EXT-RT-306 | Skill 专属类型 | 新领域类型 | P0 |
| EXT-RT-307 | Service Plugin 委托方法 | Plugin 独立 API | P1 |
| EXT-RT-308 | BeforePrompt/AfterReply | Hook Pipeline | P0 |
| FE-301 | 分散扩展子系统入口 | 统一扩展中心 | P2 |
| FE-302 | 重复详情页 | 统一 Capability 详情页 | P2 |
| FE-303 | 旧 Skill 概念 UI | Capability UI | P2 |
| FE-304 | 直接写旧 Enabled 控件 | Extension Kernel 生命周期 | P1 |
| FE-305 | 旧 Workshop 页面 | Developer Tooling | P2 |
| FE-306 | 旧 MCP 页面 | MCP Manager UI | P2 |

---

## 四、决策争议清单

### 争议 1: Registry 核心结构的分类
- **冲突原因**: Registry 的 map 存储和查询能力非常成熟，但其所有 API 以 `SkillDefinition` 为中心
- **候选分类**: 保留并抽取 vs 改造后复用
- **最终决策**: 改造后复用
- **决策依据**: 尽管存储能力成熟，但核心领域抽象（`SkillRegistry` interface, `SkillDefinition` 类型）是错误的，必须改造
- **负责人**: 架构

### 争议 2: Capability 定义文件（capability.go）的分类
- **冲突原因**: `CapabilityDefinition` 本身是通用概念，但当前绑定 `SkillDefinition.Capabilities` 字段
- **候选分类**: 保留并抽取 vs 改造后复用
- **最终决策**: 改造后复用
- **决策依据**: 定义本身可保留，但需要脱离 Skill 概念，将内置能力定义注册到 Contribution Registry

### 争议 3: Workflow Compiler 的输入接口
- **冲突原因**: Compiler 核心逻辑（循环检测、静态验证）通用，但输入为 `SkillRegistry`
- **候选分类**: 保留并抽取 vs 改造后复用
- **最终决策**: 改造后复用
- **决策依据**: Compiler 核心保留，但输入接口需改为 `ToolRegistry`（通用接口）

### 争议 4: ExtensionService 中 Skill/Plugin 双系统的处理
- **冲突原因**: Service 门面同时管理 Skill 和 Plugin 两套生命周期
- **候选分类**: 改造后复用 vs 拆分后删除 Plugin 部分
- **最终决策**: Plugin 委托方法最终删除，Service 改造为统一门面
- **负责人**: 架构

### 争议 5: Workshop Generator 是否应保留
- **冲突原因**: AI 生成能力有业务价值，但其生成产物是 SkillDefinition
- **候选分类**: 保留并抽取 vs 改造后复用 vs 最终删除
- **最终决策**: 改造后复用
- **决策依据**: AI 生成器本身有价值，改为生成 Capability/Contribution 而非 SkillDefinition

### 争议 6: 前端路由是迁移还是删除
- **冲突原因**: 所有旧路由的页面都存在，前端的 API 调用也都在使用
- **候选分类**: 仅用于迁移 vs 最终删除
- **最终决策**: 旧路由在迁移完成后删除，过渡期作为仅用于迁移
- **决策依据**: 前端必须在新页面就绪后才能切换路由

### 争议 7: MCP 数据表是在原表改造还是新建表迁移
- **冲突原因**: MCP 表结构较为规范，但表名和部分字段绑定旧模型
- **候选分类**: 改造后复用 vs 仅用于迁移
- **最终决策**: 核心表（servers, tools, resources）改造后复用，绑定表（scope_bindings, dependency_links）仅用于迁移
- **决策依据**: 按表职责分别处理，避免一刀切
