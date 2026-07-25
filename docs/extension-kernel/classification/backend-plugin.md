# 后端 Plugin 分类

> 范围：`backend/internal/extension/plugin_protocol.go`, `plugin_registry.go`, `plugin_manager.go`, `plugin_host.go`, `plugin_service.go`, `plugin_repository.go`, `plugin_surface.go`, `plugin_handler.go`, `plugin_circuit.go`, `plugin_builtin_diagnostic.go`

---

## 一、保留并抽取

### PLG-001: Hook 超时控制
- **文件**: `plugin_manager.go`
- **类型/函数**: Hook 执行超时逻辑
- **当前职责**: Plugin Hook 超时
- **目标分类**: 保留并抽取
- **判定依据**: 通用超时控制能力
- **目标组件**: Runtime Supervisor
- **抽取目标**: 通用事件处理超时控制器

### PLG-002: 熔断器
- **文件**: `plugin_circuit.go`
- **类型/函数**: 熔断状态机
- **当前职责**: Plugin 熔断保护
- **目标分类**: 保留并抽取
- **判定依据**: 熔断是通用安全能力
- **目标组件**: Runtime Supervisor
- **抽取目标**: 独立熔断器包

### PLG-003: 并发控制
- **文件**: `plugin_manager.go`
- **类型/函数**: Hook 并发限制
- **当前职责**: Plugin Hook 并发
- **目标分类**: 保留并抽取
- **判定依据**: 通用并发控制
- **目标组件**: Runtime Supervisor

### PLG-004: 事件深度限制
- **文件**: `plugin_manager.go`
- **类型/函数**: 事件传播深度限制
- **当前职责**: 防止事件无限传播
- **目标分类**: 保留并抽取
- **判定依据**: 通用事件安全保护
- **目标组件**: Event Bus
- **抽取目标**: 通用事件深度限制器

### PLG-005: 状态 CAS
- **文件**: `plugin_repository.go`
- **类型/函数**: CAS 更新逻辑
- **当前职责**: Plugin 状态乐观锁
- **目标分类**: 保留并抽取
- **判定依据**: 通用乐观锁实现
- **目标组件**: Storage Broker
- **抽取目标**: 通用 CAS 状态管理器

### PLG-006: 命名空间隔离
- **文件**: `plugin_protocol.go`
- **类型/函数**: 命名空间定义
- **当前职责**: Plugin 命名空间
- **目标分类**: 保留并抽取
- **判定依据**: 通用命名空间隔离
- **目标组件**: Scope Manager

### PLG-007: 执行审计
- **文件**: `plugin_repository.go`
- **类型/函数**: `extension_plugin_runs`, `extension_audits`
- **当前职责**: Plugin 执行审计
- **目标分类**: 保留并抽取
- **判定依据**: 通用审计能力
- **目标组件**: Audit Store
- **抽取目标**: 独立审计记录器

---

## 二、改造后复用

### PLG-101: Plugin State
- **文件**: `plugin_protocol.go`, `plugin_repository.go`
- **类型/函数**: `PluginState`, `GetPluginStates`, 状态存储
- **当前职责**: Plugin 状态管理
- **目标分类**: 改造后复用
- **判定依据**: 状态模型正确，需改为通用扩展状态
- **目标组件**: Storage Broker
- **目标新模型**: 通用扩展状态存储

### PLG-102: Event Delivery
- **文件**: `plugin_manager.go`
- **类型/函数**: 事件分发、`DispatchBeforePrompt`, `DispatchAfterReply`, `EmitSystemEvent`
- **当前职责**: Plugin 事件分发
- **目标分类**: 改造后复用
- **判定依据**: 事件分发正确，需改为通用 Event Bus
- **目标组件**: Event Bus
- **目标新模型**: 统一事件总线

### PLG-103: Schedule
- **文件**: `plugin_protocol.go`, `plugin_repository.go`
- **类型/函数**: `PluginScheduleDefinition`, `GetPluginSchedules`, `SetPluginScheduleEnabled`
- **当前职责**: Plugin 调度
- **目标分类**: 改造后复用
- **判定依据**: 调度逻辑正确，需改为通用 Schedule Manager
- **目标组件**: Schedule Manager
- **目标新模型**: 统一调度器

### PLG-104: Surface Schema
- **文件**: `plugin_surface.go`
- **类型/函数**: `SurfaceDocument`, `GetPluginSurface`, `ExecutePluginSurfaceAction`
- **当前职责**: Plugin UI 界面定义
- **目标分类**: 改造后复用
- **判定依据**: Surface 模式正确，需改为通用 UI Contribution
- **目标组件**: UI Contribution Registry
- **目标新模型**: 统一 UI Contribution

### PLG-105: Host API 权限校验
- **文件**: `plugin_host.go`
- **类型/函数**: Host API 方法
- **当前职责**: Plugin 允许调用的宿主 API
- **目标分类**: 改造后复用
- **判定依据**: Host API 模式正确，需改为 Permission Broker 统一管理
- **目标组件**: Permission Broker

### PLG-106: 插件配置
- **文件**: `plugin_service.go`
- **类型/函数**: `GetPluginConfig`, `UpdatePluginConfig`, `ResetPluginConfig`
- **当前职责**: Plugin 配置管理
- **目标分类**: 改造后复用
- **判定依据**: 配置管理通用，需改为统一 Config Store
- **目标组件**: Storage Broker

---

## 三、仅用于迁移

### PLG-201: 旧 Plugin 状态表
- **文件**: `plugin_repository.go`
- **类型/函数**: `extension_states`, `extension_state_revisions`
- **当前职责**: Plugin 持久化状态
- **目标分类**: 仅用于迁移
- **迁移来源**: `extension_states`, `extension_state_revisions`
- **迁移目标**: 新 Storage Broker
- **删除条件**: 状态迁移完成

### PLG-202: 旧 Event 表
- **文件**: `plugin_repository.go`
- **类型/函数**: `extension_events`, `extension_event_deliveries`
- **当前职责**: Plugin 事件历史
- **目标分类**: 仅用于迁移
- **迁移来源**: `extension_events`, `extension_event_deliveries`
- **迁移目标**: 新 Audit Store
- **删除条件**: 事件历史迁移完成

### PLG-203: 旧 Schedule 表
- **文件**: `plugin_repository.go`
- **类型/函数**: `extension_schedules`
- **当前职责**: Plugin 调度记录
- **目标分类**: 仅用于迁移
- **迁移来源**: `extension_schedules`
- **迁移目标**: 新 Schedule Manager
- **删除条件**: 调度数据迁移完成

### PLG-204: 旧诊断数据
- **文件**: `plugin_builtin_diagnostic.go`
- **类型/函数**: `newDiagnosticPlugin`
- **当前职责**: 内置诊断 Plugin
- **目标分类**: 仅用于迁移（诊断报告格式转换）
- **迁移来源**: 内置诊断
- **迁移目标**: 新 Developer Tooling
- **删除条件**: 新诊断工具就绪

---

## 四、最终删除

### PLG-301: Go Interface 第三方插件协议
- **文件**: `plugin_protocol.go`
- **类型/函数**: Plugin Go Interface 定义
- **当前职责**: 第三方插件实现接口
- **目标分类**: 最终删除
- **替代组件**: `.amitiax` v2 扩展包 + Extension Kernel 运行时
- **删除步骤**: 新插件体系就绪后删除

### PLG-302: Plugin Factory
- **文件**: `plugin_registry.go`
- **类型/函数**: Plugin Factory 注册
- **当前职责**: 第三方插件工厂
- **目标分类**: 最终删除
- **替代组件**: Package Manager 安装流程
- **删除步骤**: 新扩展安装流程就绪

### PLG-303: builtin-only PluginRegistry
- **文件**: `plugin_registry.go`
- **类型/函数**: `PluginRegistry`
- **当前职责**: 仅 builtin Plugin 注册
- **目标分类**: 最终删除
- **替代组件**: Contribution Registry 统一管理

### PLG-304: PluginManager 独立生命周期
- **文件**: `plugin_manager.go`
- **类型/函数**: `PluginManager`, `Start`, `Stop`
- **当前职责**: Plugin 独立生命周期管理
- **目标分类**: 最终删除
- **替代组件**: Runtime Supervisor 统一管理
- **数据迁移条件**: Running Plugin 状态迁移

### PLG-305: 诊断 Plugin
- **文件**: `plugin_builtin_diagnostic.go`
- **类型/函数**: `newDiagnosticPlugin()` 返回的诊断 Plugin
- **当前职责**: 系统诊断
- **目标分类**: 最终删除
- **替代组件**: Developer Tooling 内置诊断
- **删除步骤**: 诊断功能重写

### PLG-306: Plugin 注册 Skill 的旧路径
- **文件**: `plugin_manager.go`
- **类型/函数**: Plugin → Skill 注册逻辑
- **当前职责**: Plugin 注册 Skill 到 Registry
- **目标分类**: 最终删除
- **替代组件**: Plugin → Contribution Registry 直接注册

### PLG-307: Plugin 详情页专用 Surface 绑定
- **文件**: `plugin_surface.go`, `plugin_handler.go`
- **类型/函数**: Surface 与 Plugin ID 绑定
- **当前职责**: Plugin 专属 UI
- **目标分类**: 最终删除
- **替代组件**: UI Contribution Registry 统一管理
