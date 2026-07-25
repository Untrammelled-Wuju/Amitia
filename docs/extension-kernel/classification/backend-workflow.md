# 后端 Workflow 分类

> 范围：`backend/internal/extension/workflow_compiler.go`, `workflow_executor.go`, `workflow_values.go`

---

## 一、保留并抽取

### WFL-001: Workflow Schema
- **文件**: `workshop_protocol.go`（Workflow 类型定义）
- **类型/函数**: `WorkflowDefinition`, `WorkflowStep`, `ConditionExpression`, `WorkflowLimits`, `WorkflowErrorPolicy`
- **当前职责**: Workflow DSL 类型
- **目标分类**: 保留并抽取
- **判定依据**: Workflow DSL 是通用格式，不依赖 Skill 模型
- **目标组件**: Workflow Engine
- **抽取目标**: 独立 Workflow DSL 包

### WFL-002: Workflow Compiler
- **文件**: `workflow_compiler.go`
- **类型/函数**: `WorkflowCompiler`, `Compile`
- **当前职责**: Workflow 编译（依赖解析、循环检测、静态策略验证）
- **目标分类**: 保留并抽取
- **判定依据**: Compiler 核心逻辑通用，但输入绑定 `SkillRegistry`
- **实际分类调整**: 改造后复用（Compiler 核心保留，但需改造输入接口）
- **目标组件**: Workflow Engine
- **目标新模型**: 通用 Workflow Compiler，输入 `ToolRegistry` 替代 `SkillRegistry`

### WFL-003: Workflow Executor
- **文件**: `workflow_executor.go`
- **类型/函数**: `WorkflowExecutor`, `Execute`
- **当前职责**: Workflow 执行引擎
- **目标分类**: 保留并抽取
- **判定依据**: 执行引擎核心逻辑通用，但 Adapter 绑定 Skill
- **实际分类调整**: 改造后复用（执行核心保留，Adapter 改造）
- **目标组件**: Workflow Engine
- **目标新模型**: 通用 Workflow 执行器 + 通用 Adapter 接口

### WFL-004: Value Resolver
- **文件**: `workflow_values.go`
- **类型/函数**: `resolveValue`, `resolveReference`, `resolveJSON`, `renderTemplate`, `formatTemplateValue`
- **当前职责**: Workflow 变量解析
- **目标分类**: 保留并抽取
- **判定依据**: 变量解析和模板引擎是通用能力
- **目标组件**: Workflow Engine
- **抽取目标**: 独立模板解析器

### WFL-005: 条件表达式求值
- **文件**: `workflow_values.go`
- **类型/函数**: `validateCondition`, `evalCondition`, `evalConditionDepth`
- **当前职责**: 条件表达式评估
- **目标分类**: 保留并抽取
- **判定依据**: 通用表达式引擎
- **目标组件**: Workflow Engine
- **抽取目标**: 独立表达式求值器

### WFL-006: 循环检测
- **文件**: `workflow_compiler.go`
- **类型/函数**: 循环依赖检测
- **当前职责**: Skill 依赖循环检测
- **目标分类**: 保留并抽取
- **判定依据**: 通用图算法
- **目标组件**: Dependency Resolver

### WFL-007: JSON 变换引擎
- **文件**: `workflow_values.go`
- **类型/函数**: `transformJSON`, `compareTransformValue`
- **当前职责**: JSON 数据变换
- **目标分类**: 保留并抽取
- **判定依据**: 通用数据变换能力
- **目标组件**: Workflow Engine

### WFL-008: 错误定位
- **文件**: `workflow_executor.go`
- **类型/函数**: 步骤错误上下文
- **当前职责**: 执行错误定位
- **目标分类**: 保留并抽取
- **判定依据**: 通用错误上下文
- **目标组件**: Runtime Supervisor

---

## 二、改造后复用

### WFL-101: WorkflowHostAdapter
- **文件**: `workflow_executor.go`
- **类型/函数**: `WorkflowHostAdapter`, `SideEffectHost`
- **当前职责**: Workflow 宿主接口
- **目标分类**: 改造后复用
- **判定依据**: Host 概念正确，但绑定 Chat/Memory 具体实现
- **目标组件**: Workflow Engine
- **目标新模型**: 通用 Host 接口，不泄漏 Chat/Memory 实现

### WFL-102: HTTPWorkflowAdapter
- **文件**: `workflow_executor.go`
- **类型/函数**: `HTTPWorkflowAdapter`, `secureClient`
- **当前职责**: HTTP 调用适配器
- **目标分类**: 改造后复用
- **判定依据**: HTTP 适配器通用，但需要统一安全策略
- **目标组件**: Workflow Engine

### WFL-103: SkillWorkflowAdapter
- **文件**: `workflow_executor.go`
- **类型/函数**: `SkillWorkflowAdapter`, `workflowCallState`
- **当前职责**: Skill 调用适配器
- **目标分类**: 改造后复用
- **判定依据**: 适配器模式正确，但绑定 Skill
- **目标组件**: Workflow Engine
- **目标新模型**: `ToolAdapter` 替代 `SkillWorkflowAdapter`

### WFL-104: BuildWorkflowAdapters
- **文件**: `workflow_executor.go`
- **类型/函数**: `BuildWorkflowAdapters`, `WorkflowAdapterRegistry`
- **当前职责**: Adapter 工厂
- **目标分类**: 改造后复用
- **判定依据**: 工厂模式正确
- **目标组件**: Workflow Engine

### WFL-105: 运行上下文
- **文件**: `workflow_executor.go`
- **类型/函数**: 执行上下文管理
- **当前职责**: Workflow 执行时的上下文
- **目标分类**: 改造后复用
- **判定依据**: 上下文管理正确
- **目标组件**: Workflow Engine

---

## 三、仅用于迁移

### WFL-201: Workflow Skill 注册（旧）
- **文件**: `workshop_installer.go`
- **类型/函数**: `workflowHandler`, 从 Workflow 构造 SkillHandler
- **当前职责**: Workflow → SkillDefinition 包装
- **目标分类**: 仅用于迁移
- **迁移来源**: 旧 Workflow 数据
- **迁移目标**: Workflow Engine 原生执行
- **删除条件**: 旧 Workflow 全部迁移

### WFL-202: 旧 Host 注入结构
- **文件**: `workflow_executor.go`
- **类型/函数**: `SideEffectHost` 绑定 `ExecutionScope`
- **当前职责**: Workflow 宿主注入
- **目标分类**: 仅用于迁移
- **迁移来源**: 旧 Chat/Memory 集成
- **迁移目标**: 新 Host 接口

---

## 四、最终删除

### WFL-301: Workflow → SkillDefinition 包装层
- **文件**: `workshop_installer.go`
- **类型/函数**: `skillDefinitionFromDraft`, `buildWorkshopManifest`
- **当前职责**: 将 Workflow 包装为 SkillDefinition
- **目标分类**: 最终删除
- **替代组件**: Workflow Engine 直接作为 Capability 注册

### WFL-302: Workflow 独立安装生命周期
- **文件**: `workshop_installer.go`
- **类型/函数**: `(i *WorkshopInstaller) Install`
- **当前职责**: Workflow 独立安装流程
- **目标分类**: 最终删除
- **替代组件**: Package Manager 统一安装

### WFL-303: 重复权限判断
- **文件**: `workflow_executor.go`
- **类型/函数**: Adapter 层面的权限复判
- **当前职责**: Workflow 权限判断
- **目标分类**: 最终删除
- **替代组件**: Permission Broker 统一判定
