# 保留并抽取汇总报告

> 基于 `classification/*.md` 分类结果汇总
> 所有「保留并抽取」对象：其核心逻辑通用且独立，不依赖旧领域模型，可直接抽取为独立包

---

## 一、统计概览

| 来源模块 | 对象数 |
|---|---|
| 后端 Extension Runtime & Registry | 10 |
| 后端 Agent Skill | 8 |
| 后端 MCP | 7 |
| 后端 Plugin | 7 |
| 后端 Workflow | 8 |
| 后端 Package | 9 |
| 前端 | 9 |
| **合计** | **58** |

---

## 二、Runtime Supervisor 相关抽取

### EXT-RT-001: Executor 超时控制
- **来源**: `executor.go` → `(e *Executor) Execute` 中的 `context.WithTimeout`
- **抽取目标**: 通用执行超时控制器，输入 `context.Context` + timeout duration
- **需解除依赖**: `SkillDefinition.Timeout` 字段引用
- **保留测试**: `executor.go` 中超时相关逻辑

### EXT-RT-002: Executor Panic 恢复
- **来源**: `executor.go` → `defer/recover` panic 处理
- **抽取目标**: 通用 panic recovery wrapper
- **需解除依赖**: `ExtensionError` 类型引用（可抽象为通用 error）

### EXT-RT-003: Executor 幂等性
- **来源**: `executor.go` → `idempotencyMu`, `idempotency` map, `inFlight` map
- **抽取目标**: 通用幂等执行器
- **需解除依赖**: `SkillDefinition.Idempotent` 字段

### EXT-RT-004: Executor 并发控制
- **来源**: `executor.go` → `handlerSlots chan struct{}` 槽位控制
- **抽取目标**: 通用并发槽位控制器

### PLG-001: Hook 超时控制
- **来源**: `plugin_manager.go` → Hook 执行超时逻辑
- **抽取目标**: 通用事件处理超时控制器

### PLG-002: 熔断器
- **来源**: `plugin_circuit.go` → 熔断状态机
- **抽取目标**: 独立熔断器包

### PLG-003: 并发控制
- **来源**: `plugin_manager.go` → Hook 并发限制
- **抽取目标**: 通用并发控制

### WFL-008: 错误定位
- **来源**: `workflow_executor.go` → 步骤错误上下文
- **抽取目标**: 通用错误上下文

---

## 三、Audit Store 相关抽取

### EXT-RT-005: Executor 审计持久化
- **来源**: `executor.go` → `CreateRun`, `SetRunStatus`, `UpdateRun` 调用
- **抽取目标**: 通用执行审计接口
- **需解除依赖**: `RunView` 类型

### PLG-007: 执行审计
- **来源**: `plugin_repository.go` → `extension_plugin_runs`, `extension_audits`
- **抽取目标**: 独立审计记录器

---

## 四、Storage Broker 相关抽取

### EXT-RT-006: Executor 副作用补偿
- **来源**: `executor.go` → `RegisterOwnedSideEffects`, `CompensateUnownedSideEffects`
- **抽取目标**: 通用资源归属追踪器

### PLG-005: 状态 CAS
- **来源**: `plugin_repository.go` → CAS 更新逻辑
- **抽取目标**: 通用乐观锁实现

---

## 五、Secret Broker 相关抽取

### EXT-RT-007: Config Crypto
- **来源**: `config_crypto.go` → `configCipher`, `encrypt`, `decrypt`
- **抽取目标**: 通用配置加解密器

### EXT-RT-009: Protocol 脱敏工具
- **来源**: `protocol.go` → `normalizeJSON`, `compactSensitiveJSON`, `redactValue`, `isSensitiveKey`, `hasPlaintextSecret`, `redactJSON`, `restoreRedactedValue`
- **抽取目标**: 独立脱敏工具包

---

## 六、Package Manager 相关抽取

### EXT-RT-008: Schema Validator
- **来源**: `schema_validator.go` → `SchemaValidator`, `Validate`
- **抽取目标**: 独立 Schema 校验包

### EXT-RT-010: 版本比较
- **来源**: `protocol.go` / `registry.go` → semver 比较逻辑
- **抽取目标**: 独立 semver 包

### AGT-002: ZIP 安全解压
- **来源**: `agent_skill_parser.go` → `readAgentSkillZIP`, Zip Slip 防护
- **抽取目标**: 独立安全 ZIP 读取器
- **保留测试**: `TestAgentSkillZIPSecurity`

### AGT-003: 路径校验
- **来源**: `agent_skill_parser.go` → `validateAgentSkillPath`, `validateAgentSkillRelativePath`, `reservedAgentSkillName`, `windowsReservedName`
- **抽取目标**: 独立路径安全校验器

### PKG-001: 归档安全
- **来源**: `package_archive.go` → `readPackageZIP`, `validatePackagePath`, `validatePackageFile`, `stablePackageZIP`
- **抽取目标**: 独立安全归档包

### PKG-002: Checksum
- **来源**: `package_archive.go` → `buildChecksums`, `validateChecksums`, `packageCanonicalDigest`, `packageHash`
- **抽取目标**: 独立 Checksum 工具

### PKG-003: 签名验证
- **来源**: `package_archive.go` → `verifyPackageSignature`, `packageSignatureDocument`
- **抽取目标**: 独立签名验证器

### PKG-004: 版本比较
- **来源**: `package_protocol.go` → semver 逻辑
- **抽取目标**: 独立 semver 包（与 EXT-RT-010 合并）

### PKG-005: 安装事务
- **来源**: `package_installer.go` → `Install` 事务性安装逻辑
- **抽取目标**: 通用安装事务管理器

### PKG-006: 补偿/回滚
- **来源**: `package_installer.go`, `package_recovery.go` → 安装失败补偿、回滚
- **抽取目标**: 独立回滚管理器

### PKG-007: 恢复
- **来源**: `package_recovery.go` → 启动恢复逻辑
- **抽取目标**: 独立恢复管理器

### PKG-009: 文件类型限制
- **来源**: `package_archive.go` → `packageFileKind`
- **抽取目标**: 通用文件类型校验

---

## 七、Agent Skill Catalog 相关抽取

### AGT-001: SKILL.md / Frontmatter 解析
- **来源**: `agent_skill_parser.go` → `parseSkillMarkdown`, `skillFrontmatter`, `decodeSafeYAML`, `validateYAMLNode`
- **抽取目标**: 独立 SKILL.md 解析器包
- **需解除依赖**: `AgentSkillLimits` 参数（可抽象）
- **保留测试**: `TestAgentSkillParserValidation`

### AGT-004: 资源扫描与索引
- **来源**: `agent_skill_parser.go` → `scanAgentSkillResources`, `supportedTextResource`
- **抽取目标**: 通用文件资源扫描器

### AGT-005: 资源渐进读取
- **来源**: `agent_skill_service.go` → `ReadResource`, `ListResources`
- **抽取目标**: 通用资源读取服务

### AGT-006: Token 估算
- **来源**: `agent_skill_service.go` → `estimateTokens`
- **抽取目标**: 独立 Token 计数器

### AGT-007: OpenAI 兼容元数据解析
- **来源**: `agent_skill_parser.go` → `parseAgentSkillOpenAI`, `parsedOpenAIYAML`
- **抽取目标**: 独立 OpenAI 格式解析器

---

## 八、MCP Manager 相关抽取

### MCP-001: MCP Client
- **来源**: `client/connection.go`, `client/request_manager.go`
- **抽取目标**: 独立 MCP Client 包

### MCP-002: MCP Transport
- **来源**: `transport/transport.go`, `transport/stdio.go`, `transport/streamable_http.go`, `transport/security.go`, `transport/process_*.go`
- **抽取目标**: 独立 MCP Transport 包

### MCP-003: MCP OAuth
- **来源**: `auth/oauth.go`, `auth/token_store.go`
- **抽取目标**: 独立 MCP OAuth 包

### MCP-004: MCP Protocol
- **来源**: `protocol/message.go`, `protocol/errors.go`, `protocol/version.go`
- **抽取目标**: 独立 MCP Protocol 包

### MCP-005: MCP Discovery
- **来源**: `discovery/service.go`
- **抽取目标**: 独立 MCP Discovery 服务

### MCP-006: MCP Features
- **来源**: `features/service.go`
- **抽取目标**: 独立 Feature 检测服务

### MCP-007: MCP Host
- **来源**: `host/service.go`, `host/interaction.go`, `host/roots.go`
- **抽取目标**: 独立 MCP Host 包

---

## 九、Workflow Engine 相关抽取

### WFL-001: Workflow Schema
- **来源**: `workshop_protocol.go` → `WorkflowDefinition`, `WorkflowStep`, `ConditionExpression`, `WorkflowLimits`, `WorkflowErrorPolicy`
- **抽取目标**: 独立 Workflow DSL 包

### WFL-004: Value Resolver
- **来源**: `workflow_values.go` → `resolveValue`, `resolveReference`, `resolveJSON`, `renderTemplate`, `formatTemplateValue`
- **抽取目标**: 独立模板解析器

### WFL-005: 条件表达式求值
- **来源**: `workflow_values.go` → `validateCondition`, `evalCondition`, `evalConditionDepth`
- **抽取目标**: 独立表达式求值器

### WFL-006: 循环检测
- **来源**: `workflow_compiler.go` → 循环依赖检测
- **抽取目标**: 通用循环检测器（归入 Dependency Resolver）

### WFL-007: JSON 变换引擎
- **来源**: `workflow_values.go` → `transformJSON`, `compareTransformValue`
- **抽取目标**: 独立 JSON 变换引擎

---

## 十、Dependency Resolver / Event Bus / Scope Manager 相关抽取

### PLG-004: 事件深度限制
- **来源**: `plugin_manager.go` → 事件传播深度限制
- **抽取目标**: 通用事件深度限制器（归入 Event Bus）

### PLG-006: 命名空间隔离
- **来源**: `plugin_protocol.go` → 命名空间定义
- **抽取目标**: 通用命名空间隔离（归入 Scope Manager）

### PKG-008: 卸载预览
- **来源**: `package_lifecycle.go` → 卸载影响分析
- **抽取目标**: 通用依赖分析器（归入 Dependency Resolver）

---

## 十一、前端 UI Contribution Registry 相关抽取

### FE-001: Schema Surface Renderer
- **来源**: `front/src/views/extensions/components/SchemaSurfaceRenderer.vue`
- **抽取目标**: 独立 Schema UI 组件库

### FE-002: SurfaceAction
- **来源**: `front/src/views/extensions/components/SurfaceAction.vue`
- **抽取目标**: 通用交互组件

### FE-003: SurfaceForm
- **来源**: `front/src/views/extensions/components/SurfaceForm.vue`
- **抽取目标**: 通用表单组件

### FE-004: SurfaceStatus
- **来源**: `front/src/views/extensions/components/SurfaceStatus.vue`
- **抽取目标**: 通用状态展示组件

### FE-005: SurfaceTable
- **来源**: `front/src/views/extensions/components/SurfaceTable.vue`
- **抽取目标**: 通用表格组件

### FE-006: PermissionDialog
- **来源**: `front/src/views/extensions/components/PermissionDialog.vue`
- **抽取目标**: 通用权限确认组件（归入 Permission Broker）

### FE-007: ExtensionPageHeader
- **来源**: `front/src/views/extensions/components/ExtensionPageHeader.vue`
- **抽取目标**: 通用页面布局组件

### FE-008: 通用类型定义
- **来源**: `front/src/views/extensions/types.ts` → `CapabilityDefinition`, `ProblemDetail`
- **抽取目标**: 独立前端类型包

### FE-009: 通用状态处理
- **来源**: 所有页面中的 Loading / Empty / Error 状态
- **抽取目标**: 通用 UI 状态组件

---

## 十二、抽取顺序建议

### 第1批：零依赖基础设施（可并行）
- AGT-002: ZIP 安全解压
- AGT-003: 路径校验
- EXT-RT-008: Schema Validator
- EXT-RT-010 / PKG-004: semver 版本比较
- PKG-009: 文件类型限制

### 第2批：数据安全工具（依赖第1批）
- EXT-RT-007: Config Crypto
- EXT-RT-009: Protocol 脱敏工具
- PKG-001: 归档安全
- PKG-002: Checksum
- PKG-003: 签名验证

### 第3批：MCP 协议栈（独立体系，可并行）
- MCP-004: Protocol
- MCP-002: Transport
- MCP-001: Client
- MCP-003: OAuth
- MCP-005: Discovery
- MCP-006: Features
- MCP-007: Host

### 第4批：执行基础（依赖第1-2批）
- EXT-RT-001: 超时控制
- EXT-RT-002: Panic 恢复
- EXT-RT-003: 幂等性
- EXT-RT-004: 并发控制
- PLG-001: Hook 超时
- PLG-002: 熔断器
- PLG-003: 并发控制
- PLG-004: 事件深度限制
- PLG-005: 状态 CAS
- PLG-006: 命名空间隔离
- PLG-007: 执行审计

### 第5批：Workflow DSL + Engine（独立体系）
- WFL-001: Schema
- WFL-004: Value Resolver
- WFL-005: 条件表达式
- WFL-006: 循环检测
- WFL-007: JSON 变换
- WFL-008: 错误定位

### 第6批：Agent Skill 解析（依赖第1-2批）
- AGT-001: SKILL.md 解析
- AGT-004: 资源扫描
- AGT-005: 资源读取
- AGT-006: Token 估算
- AGT-007: OpenAI 解析

### 第7批：前端组件（依赖后端 API 就绪）
- FE-001~005: Surface 组件
- FE-006: PermissionDialog
- FE-007: ExtensionPageHeader
- FE-008: 类型定义
- FE-009: 状态处理

### 第8批：Package Manager 事务（依赖第1-5批）
- PKG-005: 安装事务
- PKG-006: 补偿/回滚
- PKG-007: 恢复
- PKG-008: 卸载预览
