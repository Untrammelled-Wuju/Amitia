# 后端 Agent Skill 分类

> 范围：`backend/internal/extension/agent_skill_parser.go`, `agent_skill_service.go`, `agent_skill_runtime.go`, `agent_skill_handler.go`, `agent_skill_repository.go`, `agent_skill_protocol.go`, `agent_skill_metrics.go`

---

## 一、保留并抽取

### AGT-001: SKILL.md / Frontmatter 解析
- **文件**: `agent_skill_parser.go`
- **类型/函数**: `parseSkillMarkdown`, `skillFrontmatter`, `decodeSafeYAML`, `validateYAMLNode`
- **当前职责**: 解析 Agent Skill 的 Markdown frontmatter
- **目标分类**: 保留并抽取
- **判定依据**: SKILL.md 是 OpenAI 标准格式，解析逻辑通用且独立
- **目标组件**: Agent Skill Catalog
- **抽取目标**: 独立 SKILL.md 解析器包
- **需解除依赖**: `AgentSkillLimits` 参数（可抽象）
- **保留测试**: `TestAgentSkillParserValidation`

### AGT-002: ZIP 安全解压
- **文件**: `agent_skill_parser.go`
- **类型/函数**: `readAgentSkillZIP`, Zip Slip 防护
- **当前职责**: ZIP 安全解压
- **目标分类**: 保留并抽取
- **判定依据**: ZIP 安全问题通用，不依赖 Agent Skill
- **目标组件**: Package Manager（归档安全）
- **抽取目标**: 独立安全 ZIP 读取器
- **保留测试**: `TestAgentSkillZIPSecurity`

### AGT-003: 路径校验
- **文件**: `agent_skill_parser.go`
- **类型/函数**: `validateAgentSkillPath`, `validateAgentSkillRelativePath`, `reservedAgentSkillName`, `windowsReservedName`
- **当前职责**: 路径安全校验
- **目标分类**: 保留并抽取
- **判定依据**: 路径穿越防护是通用安全能力
- **目标组件**: Package Manager
- **抽取目标**: 独立路径安全校验器

### AGT-004: 资源扫描与索引
- **文件**: `agent_skill_parser.go`
- **类型/函数**: `scanAgentSkillResources`, `supportedTextResource`
- **当前职责**: 扫描 Skill 资源文件
- **目标分类**: 保留并抽取
- **判定依据**: 资源索引是通用能力，可脱离 Skill 模型
- **目标组件**: Agent Skill Catalog
- **抽取目标**: 通用文件资源扫描器

### AGT-005: 资源渐进读取
- **文件**: `agent_skill_service.go`
- **类型/函数**: `ReadResource`, `ListResources`
- **当前职责**: 按需读取 Skill 资源
- **目标分类**: 保留并抽取
- **判定依据**: 资源渐进读取是通用能力
- **目标组件**: Agent Skill Catalog
- **抽取目标**: 通用资源读取服务

### AGT-006: Token 估算
- **文件**: `agent_skill_service.go`
- **类型/函数**: `estimateTokens`
- **当前职责**: Token 数量估算
- **目标分类**: 保留并抽取
- **判定依据**: 通用 Token 计算
- **目标组件**: Agent Skill Catalog
- **抽取目标**: 独立 Token 计数器

### AGT-007: OpenAI 兼容元数据解析
- **文件**: `agent_skill_parser.go`
- **类型/函数**: `parseAgentSkillOpenAI`, `parsedOpenAIYAML`
- **当前职责**: 解析 OpenAI Agent Skills 元数据
- **目标分类**: 保留并抽取
- **判定依据**: OpenAI 标准格式，通用兼容能力
- **目标组件**: Agent Skill Catalog

### AGT-008: MCP 依赖声明解析
- **文件**: `agent_skill_parser.go`
- **类型/函数**: `parseAgentSkillAmitia`, `validateMCPDependency`
- **当前职责**: 解析 Amitia 扩展的 MCP 依赖
- **目标分类**: 改造后复用（解析逻辑保留，但需改造成通用 Dependency Resolver）
- **目标组件**: Dependency Resolver
- **目标新模型**: 统一依赖声明格式

---

## 二、改造后复用

### AGT-101: AgentSkillService 目录与激活
- **文件**: `agent_skill_service.go`
- **类型/函数**: `AgentSkillService`, `ResolveCatalog`, `Activate`, `PreparePrompt`
- **当前职责**: Agent Skill 目录、激活和 Prompt 注入
- **目标分类**: 改造后复用
- **判定依据**: 目录和激活能力正确，但绑定 Skill 领域模型
- **目标组件**: Agent Skill Catalog
- **目标新模型**: 通用 Capability Catalog + Activation Manager

### AGT-102: AgentSkillService 缓存
- **文件**: `agent_skill_service.go`
- **类型/函数**: `agentSkillArtifactCacheEntry`, `invalidateAgentSkillCaches`, `agentSkillCatalogCacheKey`
- **当前职责**: 目录结果缓存
- **目标分类**: 改造后复用
- **判定依据**: 缓存模式正确，但键绑定旧模型
- **目标组件**: Agent Skill Catalog

### AGT-103: AgentSkillService Round State
- **文件**: `agent_skill_service.go`
- **类型/函数**: `agentSkillRoundState`, `roundState`, `ensureRoundLocked`, `EndRound`
- **当前职责**: Agent Skill 轮次状态
- **目标分类**: 改造后复用
- **判定依据**: 轮次管理逻辑，需改为 Extension Kernel 统一会话管理
- **目标组件**: Runtime Supervisor

### AGT-104: AgentSkillService Prompt 渲染
- **文件**: `agent_skill_service.go`
- **类型/函数**: `renderAgentSkillCatalog`, `renderActiveAgentSkill`, `stripAgentSkillHostTags`
- **当前职责**: 将目录渲染为 Prompt
- **目标分类**: 改造后复用
- **判定依据**: 渲染逻辑可复用，但输入需改为 Capability
- **目标组件**: Agent Skill Catalog

### AGT-105: AgentSkillParser 兼容性分析
- **文件**: `agent_skill_parser.go`
- **类型/函数**: `analyzeAgentSkillCompatibility`
- **当前职责**: 分析兼容性
- **目标分类**: 改造后复用
- **判定依据**: 兼容性分析逻辑正确，需改为通用 Capability 兼容性

---

## 三、仅用于迁移

### AGT-201: agentSkillMetadataRecord 表读取
- **文件**: `agent_skill_repository.go`
- **类型/函数**: `agentSkillMetadataRecord`, `ListAgentSkillRecords`, `GetAgentSkillRecord`
- **当前职责**: Agent Skill 元数据表 CRUD
- **目标分类**: 仅用于迁移
- **迁移来源**: `extension_agent_skill_metadata` 表
- **迁移目标**: 新 Agent Skill 存储
- **停止写入时间**: 迁移完成后
- **删除条件**: 旧元数据全部迁移

### AGT-202: agentSkillActivationRecord 表读取
- **文件**: `agent_skill_repository.go`
- **类型/函数**: `agentSkillActivationRecord`, `SaveAgentSkillActivation`, `ListAgentSkillActivations`
- **当前职责**: 激活记录
- **目标分类**: 仅用于迁移
- **迁移来源**: `extension_agent_skill_activations` 表
- **迁移目标**: 新 Audit Store
- **删除条件**: 激活历史迁移完成

### AGT-203: agentSkillArtifactRecord 编解码
- **文件**: `agent_skill_repository.go`
- **类型/函数**: `encodeAgentSkillArtifact`, `decodeAgentSkillArtifact`, `extractAgentSkillBody`
- **当前职责**: Artifact 编码
- **目标分类**: 仅用于迁移（解码旧 Artifact 格式）
- **迁移来源**: `extension_artifacts` 表旧格式
- **迁移目标**: 新 Artifact Store
- **删除条件**: 旧 Artifact 全部迁移

### AGT-204: LoadAgentSkill 旧版
- **文件**: `agent_skill_repository.go`
- **类型/函数**: `(r *Repository) LoadAgentSkill`
- **当前职责**: 从 DB 加载 Agent Skill
- **目标分类**: 仅用于迁移
- **迁移来源**: 旧 Agent Skill 存储
- **迁移目标**: 新 Catalog 加载器
- **删除条件**: 旧格式数据全部迁移

### AGT-205: SetAgentSkillEnabled / RemoveAgentSkill
- **文件**: `agent_skill_repository.go`
- **类型/函数**: `SetAgentSkillEnabled`, `RemoveAgentSkill`
- **当前职责**: 旧启用/删除逻辑
- **目标分类**: 仅用于迁移
- **禁止写入**: 迁移完成后禁止
- **删除条件**: 旧数据表删除

### AGT-206: InstallAgentSkill 旧版
- **文件**: `agent_skill_repository.go`
- **类型/函数**: `(r *Repository) InstallAgentSkill`
- **当前职责**: 安装 Agent Skill
- **目标分类**: 仅用于迁移
- **迁移目标**: 新 Package Manager 安装流程
- **删除条件**: 安装迁移完成

---

## 四、最终删除

### AGT-301: Agent Skill → SkillDefinition 注册
- **文件**: `agent_skill_service.go`
- **类型/函数**: `buildAgentSkillManifest`, `setInstalledAgentSkillBinding`
- **当前职责**: 将 Agent Skill 包装为 SkillDefinition
- **目标分类**: 最终删除
- **替代组件**: Agent Skill 直接注册为 Capability/Contribution
- **删除步骤**: 新 Catalog 注册完成后删除

### AGT-302: registerAgentSkillRuntime
- **文件**: `agent_skill_runtime.go`
- **类型/函数**: `registerAgentSkillRuntime`, `internalAgentSkillDefinition`
- **当前职责**: Runtime 中注册 Agent Skill 内部工具
- **目标分类**: 最终删除
- **替代组件**: Extension Kernel 内置 Tool Registry

### AGT-303: Runtime PrepareAgentSkillPrompt / EndAgentSkillRound
- **文件**: `agent_skill_runtime.go`
- **类型/函数**: `PrepareAgentSkillPrompt`, `EndAgentSkillRound`
- **当前职责**: Runtime 层面的 Agent Skill Prompt 管理
- **目标分类**: 最终删除
- **替代组件**: Agent Skill Catalog 直接集成到 Extension Kernel

### AGT-304: AgentSkill 旧模型类型
- **文件**: `agent_skill_protocol.go`
- **类型**: `AgentSkillActivation`, `ActivateAgentSkillRequest`, `ActivatedAgentSkill`, `AgentSkillCatalogEntry`
- **当前职责**: Agent Skill 旧模型
- **目标分类**: 最终删除
- **替代组件**: Extension Kernel 统一类型

### AGT-305: AgentSkillHandler
- **文件**: `agent_skill_handler.go`
- **类型/函数**: `AgentSkillHandler`
- **当前职责**: Agent Skill HTTP API
- **目标分类**: 最终删除
- **替代组件**: Extension Kernel 统一 HTTP API
