# 后端 Workshop 分类

> 范围：`backend/internal/extension/workshop_*.go`

---

## 一、保留并抽取

### WS-001: (无纯抽取对象)
Workshop 整体高度耦合 Skill/Workflow 旧模型，无可独立抽取的通用基础设施。

---

## 二、改造后复用

### WS-101: Workshop Session 管理
- **文件**: `workshop_service.go`, `workshop_repository.go`
- **类型/函数**: `WorkshopService.CreateSession`, `GetSession`, `ListSessions`, `Archive`
- **当前职责**: AI 生成会话管理
- **目标分类**: 改造后复用
- **判定依据**: 会话管理能力可改造成扩展开发器
- **目标组件**: Developer Tooling
- **目标新模型**: Extension Developer Session

### WS-102: Workshop Revision 管理
- **文件**: `workshop_service.go`, `workshop_repository.go`
- **类型/函数**: `SaveRevision`, `GetRevision`, `ListRevisions`
- **当前职责**: 版本管理
- **目标分类**: 改造后复用
- **判定依据**: 版本管理是通用能力
- **目标组件**: Developer Tooling
- **目标新模型**: 扩展版本管理

### WS-103: Workshop Generator
- **文件**: `workshop_generator.go`
- **类型/函数**: `WorkshopGenerator`, `Generate`, `generatePlan`, `generateDraft`
- **当前职责**: AI 生成 Skill/Workflow
- **目标分类**: 改造后复用
- **判定依据**: AI 生成能力可改为通用扩展生成器
- **目标组件**: Developer Tooling
- **目标新模型**: Extension AI Generator（生成各种 Capability）

### WS-104: Workshop Validation
- **文件**: `workshop_service.go`
- **类型/函数**: `Validate`, `analyzeCapabilityDeclaration`
- **当前职责**: 验证生成结果
- **目标分类**: 改造后复用
- **判定依据**: 验证能力通用
- **目标组件**: Developer Tooling
- **目标新模型**: Extension Validator

### WS-105: Workshop Test Runner
- **文件**: `workshop_service.go`, `package_test_runner.go`
- **类型/函数**: `Test`, `evaluateAssertions`, `WorkshopTestCase`
- **当前职责**: Workflow 测试运行
- **目标分类**: 改造后复用
- **判定依据**: 测试运行器可改造成通用插件测试器
- **目标组件**: Developer Tooling

### WS-106: Workshop Installer
- **文件**: `workshop_installer.go`
- **类型/函数**: `WorkshopInstaller.Install`, `Restore`, `Rollback`
- **当前职责**: Workshop → SkillDefinition 安装
- **目标分类**: 改造后复用
- **判定依据**: 安装流程应统一走 Package Manager
- **目标组件**: Package Manager
- **目标新模型**: Workshop 产物打包为 .amitiax v2

### WS-107: Workshop Fork
- **文件**: `workshop_service.go`
- **类型/函数**: `ForkSkill`
- **当前职责**: 从已安装 Skill Fork 回 Workshop
- **目标分类**: 改造后复用
- **判定依据**: Fork 能力可改造成扩展编辑入口
- **目标组件**: Developer Tooling

---

## 三、仅用于迁移

### WS-201: 旧 Workshop 数据
- **文件**: `workshop_repository.go`
- **类型/函数**: `extension_workshop_sessions`, `extension_workshop_revisions`, `extension_workshop_test_runs`
- **当前职责**: Workshop 历史数据
- **目标分类**: 仅用于迁移
- **迁移来源**: 旧 Workshop 数据
- **迁移目标**: 新 Developer Tooling
- **删除条件**: Workshop 数据迁移完成

### WS-202: 旧 Skill/Workflow 生成逻辑
- **文件**: `workshop_generator.go`
- **类型/函数**: 只输出 SkillDefinition/WorkflowDefinition
- **当前职责**: 生成 Skill
- **目标分类**: 仅用于迁移
- **迁移来源**: 旧生成格式
- **迁移目标**: 新 Capability 格式生成
- **删除条件**: 生成器改造完成

---

## 四、最终删除

### WS-301: Workshop 独立 Installer
- **文件**: `workshop_installer.go`
- **类型/函数**: `WorkshopInstaller`
- **当前职责**: Workshop 独立安装流程
- **目标分类**: 最终删除
- **替代组件**: Package Manager 统一安装
- **删除步骤**: Workshop 产物通过 Package Manager 安装

### WS-302: WorkshopHandler
- **文件**: `workshop_handler.go`
- **类型/函数**: `WorkshopHandler`
- **当前职责**: Workshop HTTP API
- **目标分类**: 最终删除
- **替代组件**: Extension Kernel HTTP API + Developer Tooling API

### WS-303: Workshop → SkillDefinition 旧路径
- **文件**: `workshop_installer.go`
- **类型/函数**: `skillDefinitionFromDraft`, `skillDefinitionFromManifest`
- **当前职责**: Workshop 产物 → SkillDefinition
- **目标分类**: 最终删除
- **替代组件**: Workshop → .amitiax v2 → Package Manager → Contribution Registry
