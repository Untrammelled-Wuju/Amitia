# 前端扩展中心分类

> 范围：`front/src/views/extensions/`, `front/src/views/mcp/`, `front/src/router/index.ts`, `front/src/stores/`

---

## 一、保留并抽取

### FE-001: Schema Surface Renderer
- **文件**: `front/src/views/extensions/components/SchemaSurfaceRenderer.vue`
- **当前职责**: Schema 驱动的 UI 渲染
- **目标分类**: 保留并抽取
- **判定依据**: Schema → UI 控件映射是通用能力
- **目标组件**: UI Contribution Registry
- **抽取目标**: 独立 Schema UI 组件库

### FE-002: SurfaceAction
- **文件**: `front/src/views/extensions/components/SurfaceAction.vue`
- **当前职责**: Surface Action 按钮
- **目标分类**: 保留并抽取
- **判定依据**: 通用交互组件
- **目标组件**: UI Contribution Registry

### FE-003: SurfaceForm
- **文件**: `front/src/views/extensions/components/SurfaceForm.vue`
- **当前职责**: Surface Form 表单
- **目标分类**: 保留并抽取
- **判定依据**: 通用表单组件
- **目标组件**: UI Contribution Registry

### FE-004: SurfaceStatus
- **文件**: `front/src/views/extensions/components/SurfaceStatus.vue`
- **当前职责**: 状态展示
- **目标分类**: 保留并抽取
- **判定依据**: 通用状态组件
- **目标组件**: UI Contribution Registry

### FE-005: SurfaceTable
- **文件**: `front/src/views/extensions/components/SurfaceTable.vue`
- **当前职责**: 表格展示
- **目标分类**: 保留并抽取
- **判定依据**: 通用表格组件
- **目标组件**: UI Contribution Registry

### FE-006: PermissionDialog
- **文件**: `front/src/views/extensions/components/PermissionDialog.vue`
- **当前职责**: 权限确认弹窗
- **目标分类**: 保留并抽取
- **判定依据**: 权限确认是通用 UX 模式
- **目标组件**: Permission Broker
- **抽取目标**: 通用权限确认组件

### FE-007: ExtensionPageHeader
- **文件**: `front/src/views/extensions/components/ExtensionPageHeader.vue`
- **当前职责**: 扩展页面标题栏
- **目标分类**: 保留并抽取
- **判定依据**: 通用页面布局组件
- **目标组件**: UI Contribution Registry

### FE-008: 前端类型定义（后台映射部分）
- **文件**: `front/src/views/extensions/types.ts`
- **类型**: `CapabilityDefinition`, `ProblemDetail`
- **当前职责**: 通用类型
- **目标分类**: 保留并抽取
- **判定依据**: 这些类型不绑定 Skill 旧概念
- **目标组件**: Extension Kernel 前端类型

### FE-009: 通用状态处理
- **文件**: 所有页面中
- **当前职责**: Loading / Empty / Error 状态
- **目标分类**: 保留并抽取
- **判定依据**: 通用 UI 状态处理
- **目标组件**: UI Contribution Registry

---

## 二、改造后复用

### FE-101: 扩展中心主布局
- **文件**: `ExtensionCenterView.vue`
- **当前职责**: 扩展中心主页
- **目标分类**: 改造后复用
- **判定依据**: 布局有用但需改为新架构
- **目标组件**: Extension Kernel UI
- **目标新模型**: 统一扩展中心

### FE-102: SkillListView
- **文件**: `SkillListView.vue`
- **当前职责**: Skill 列表
- **目标分类**: 改造后复用
- **判定依据**: 列表模式正确但展示 Skill
- **目标组件**: Capability/Contribution 列表页

### FE-103: SkillDetailView
- **文件**: `SkillDetailView.vue`
- **当前职责**: Skill 详情
- **目标分类**: 最终删除（Split 到 Capability 详情页）
- **替代组件**: 每种 Capability 类型独立详情页

### FE-104: PackageManagerView
- **文件**: `packages/PackageManagerView.vue`
- **当前职责**: 包管理
- **目标分类**: 改造后复用
- **判定依据**: 包管理 UI 核心不变
- **目标组件**: Package Manager UI

### FE-105: PluginListView / PluginDetailView
- **文件**: `PluginListView.vue`, `PluginDetailView.vue`
- **当前职责**: Plugin 列表和详情
- **目标分类**: 改造后复用
- **判定依据**: UI 模式正确
- **目标组件**: Extension Kernel Plugin UI

### FE-106: WorkshopListView / WorkshopSessionView
- **文件**: `workshop/WorkshopListView.vue`, `workshop/WorkshopSessionView.vue`
- **当前职责**: Workshop UI
- **目标分类**: 改造后复用
- **判定依据**: 可改造成开发者模式
- **目标组件**: Developer Tooling UI

### FE-107: AgentSkillListView
- **文件**: `agent-skills/AgentSkillListView.vue`
- **当前职责**: Agent Skill 列表
- **目标分类**: 改造后复用
- **判定依据**: 列表模式正确
- **目标组件**: Agent Skill Catalog UI

### FE-108: RunHistoryView
- **文件**: `RunHistoryView.vue`
- **当前职责**: 运行历史
- **目标分类**: 改造后复用
- **判定依据**: 运行记录 UI 通用
- **目标组件**: Audit Store UI

### FE-109: MCPServerView
- **文件**: `front/src/views/mcp/MCPServerView.vue`
- **当前职责**: MCP 管理页面
- **目标分类**: 改造后复用
- **判定依据**: MCP 管理 UI 核心不变
- **目标组件**: MCP Manager UI

### FE-110: Workshop CapabilityRiskList
- **文件**: `workshop/components/CapabilityRiskList.vue`
- **当前职责**: 能力风险列表
- **目标分类**: 改造后复用
- **判定依据**: 通用能力分析组件
- **目标组件**: Developer Tooling

### FE-111: Workshop StructuredDraftEditor / TestResultViewer
- **文件**: `workshop/components/`
- **当前职责**: 草稿编辑器和测试结果查看器
- **目标分类**: 改造后复用
- **判定依据**: 通用开发工具组件
- **目标组件**: Developer Tooling

---

## 三、仅用于迁移

### FE-201: 前端 API Client（旧接口）
- **文件**: `front/src/views/extensions/api.ts`, `front/src/views/mcp/api.ts`
- **当前职责**: 旧 API 调用
- **目标分类**: 仅用于迁移
- **迁移来源**: 旧 API 端点
- **迁移目标**: 新 Extension Kernel API
- **删除条件**: 新 API 就绪

### FE-202: 旧类型（Skill 专属）
- **文件**: `front/src/views/extensions/types.ts`
- **类型**: `SkillView`, `SkillDetail`, `SkillResult`, `PluginView`, `PluginState`, `PluginSchedule`, `PluginManifest`
- **当前职责**: Skill/Plugin 旧类型
- **目标分类**: 仅用于迁移（与后端 API 同步删除）
- **删除条件**: 新类型定义就绪

### FE-203: 路由中的扩展子路径
- **文件**: `front/src/router/index.ts`
- **路由**: `/extensions/skills`, `/extensions/skills/:id`, `/extensions/plugins`, `/extensions/plugins/:id`
- **当前职责**: 旧页面路由
- **目标分类**: 仅用于迁移
- **迁移来源**: 旧路由
- **迁移目标**: 新统一路由
- **删除条件**: 新路由就绪

---

## 四、最终删除

### FE-301: 分散扩展子系统入口
- **文件**: 路由中多个独立扩展入口
- **当前职责**: `/extensions/skills`, `/extensions/plugins`, `/extensions/workshop`, `/extensions/agent-skills`
- **目标分类**: 最终删除
- **替代组件**: 统一 `/extensions` 中心
- **删除步骤**: 统一 UI 完成后删除旧路由

### FE-302: 重复详情页
- **文件**: `SkillDetailView.vue`, `PluginDetailView.vue`
- **当前职责**: 独立详情页
- **目标分类**: 最终删除
- **替代组件**: 统一 Capability 详情页

### FE-303: 旧 Skill 概念 UI
- **文件**: SkillListView, SkillDetailView 中的 Skill 专属展示
- **目标分类**: 最终删除
- **替代组件**: Capability 类型展示

### FE-304: 直接写旧 Enabled 状态的控件
- **文件**: 各页面中的 Enable/Disable 按钮
- **当前职责**: 直接调用旧 Enable API
- **目标分类**: 最终删除
- **替代组件**: Extension Kernel 统一生命周期管理

### FE-305: 旧 Workshop Skill 制作页面
- **文件**: `WorkshopSessionView.vue` 中 Skill 生成相关
- **当前职责**: 旧 Workshop → Skill 制作
- **目标分类**: 最终删除
- **替代组件**: Developer Tooling 新扩展制作

### FE-306: 旧 MCP 页面中的 Skill 适配展示
- **文件**: `MCPServerView.vue`
- **当前职责**: 以 Skill 视角展示 MCP Tool
- **目标分类**: 最终删除
- **替代组件**: MCP Tool 直接展示
