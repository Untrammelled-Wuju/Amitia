# Amitia 扩展系统重构第 4 步：划分保留、重写、迁移和删除范围

> 文档位置：docs/extension-kernel/04-retain-rewrite-migrate-delete.md
> 创建日期：2026-07-25
> 前置步骤：第 2 步（调用链地图）、第 3 步（数据表与资源归属清单）

---

## 一、分类总览

| 分类 | 后端对象数 | 前端对象数 | 数据表数 | API 数 | 测试文件数 |
|---|---|---|---|---|---|
| 保留并抽取 | 48 | 9 | 0 | 0 | 8 |
| 改造后复用 | 67 | 12 | 18 | 35 | 15 |
| 仅用于迁移 | 35 | 8 | 8 | 30 | 3 |
| 最终删除 | 28 | 11 | 7 | 15 | 4 |

---

## 二、分类说明

### 2.1 保留并抽取

从旧模块中抽离为通用基础设施的能力。这些代码不依赖旧领域模型，可以直接作为 Extension Kernel 的基础。

**关键抽取目标：**

- **安全基础设施**：包归档安全、路径穿越防护、Checksum、签名验证、Secret 加密
- **MCP 协议层**：Client、Transport（stdio/Streamable HTTP）、OAuth、Token Store、JSON-RPC
- **执行安全保障**：超时与取消、Panic 恢复、幂等、并发控制、熔断
- **通用工具**：版本比较、原子安装、回滚基础能力、JSON Schema 校验

### 2.2 改造后复用

需要在新 Extension Kernel 领域模型下重构的业务能力。当前实现正确但耦合了错误的抽象。

**关键改造目标：**

- Agent Skill 解析器 → 改为通用 Capability 加载器
- Workflow Compiler/Executor → 解耦旧 WorkflowHost
- Plugin 状态 CAS/事件传递 → 改造成 Hook Pipeline
- MCP Connection Manager → 改造成统一 Connection Pool
- 前端 Schema Surface Renderer → 改造成 UI Contribution Renderer

### 2.3 仅用于迁移

旧数据读取、转换和导出的代码。迁移完成后删除。

**关键迁移路径：**

- Manifest v1 → Manifest v2
- Agent Skill → SkillDefinition → Capability
- MCP Tool → SkillDefinition → Tool Registry
- 旧 Scope Binding → 新 Scope Manager
- 旧 Package Version → 新 Artifact Store

### 2.4 最终删除

职责将被新 Extension Kernel 完全替代的代码。

**关键删除对象：**

- `mcp/skill/runtime.go`（MCP Tool → SkillDefinition 适配器）
- Plugin Factory（Go Interface 作为第三方插件协议）
- PluginManager 独立生命周期
- Agent Skill 伪 Skill 注册
- 旧扩展中心分散页面

---

## 三、详细分类文档

| 子系统 | 文档 |
|---|---|
| Extension Runtime & Registry | [classification/backend-extension.md](classification/backend-extension.md) |
| Agent Skill | [classification/backend-agent-skill.md](classification/backend-agent-skill.md) |
| MCP | [classification/backend-mcp.md](classification/backend-mcp.md) |
| Plugin | [classification/backend-plugin.md](classification/backend-plugin.md) |
| Workflow | [classification/backend-workflow.md](classification/backend-workflow.md) |
| Package | [classification/backend-package.md](classification/backend-package.md) |
| Workshop | [classification/backend-workshop.md](classification/backend-workshop.md) |
| 前端 | [classification/frontend.md](classification/frontend.md) |
| 数据表 | [classification/database.md](classification/database.md) |
| API | [classification/api.md](classification/api.md) |
| 测试 | [classification/tests.md](classification/tests.md) |
| 运行资源 | [classification/runtime-resources.md](classification/runtime-resources.md) |

---

## 四、目标组件映射

详见 [matrices/target-component-map.md](matrices/target-component-map.md)

## 五、仅用于迁移对象映射

详见 [matrices/migration-only-map.md](matrices/migration-only-map.md)

## 六、删除依赖图

详见 [matrices/deletion-dependency-map.md](matrices/deletion-dependency-map.md)

## 七、决策争议清单

详见 [matrices/classification-summary.md](matrices/classification-summary.md)

---

## 八、汇总报告

| 报告 | 文档 |
|---|---|
| 保留并抽取汇总 | [reports/preserve-extract.md](reports/preserve-extract.md) |
| 改造后复用汇总 | [reports/refactor-reuse.md](reports/refactor-reuse.md) |
| 仅用于迁移汇总 | [reports/migration-only.md](reports/migration-only.md) |
| 最终删除汇总 | [reports/final-delete.md](reports/final-delete.md) |

---

## 九、验收状态

- [x] 所有扩展相关后端文件已分类
- [x] 所有扩展相关前端页面、路由和 API 已分类
- [x] 所有扩展相关数据表已分类
- [x] 所有运行时资源已分类
- [x] 所有测试已分类
- [x] 每个对象只有一个主分类
- [x] "保留并抽取"对象已明确抽取目标
- [x] "改造后复用"对象已明确目标模型
- [x] "仅用于迁移"对象已明确删除条件
- [x] "最终删除"对象已明确替代组件
- [x] 没有使用模糊结论
- [x] 没有把整个大文件粗略归为一类
- [x] 用户数据和 Secret 迁移风险已单独审查
- [x] 删除依赖顺序已明确
- [x] 未修改任何运行行为
- [x] "仅用于迁移"对象映射表已创建
- [x] 四类汇总报告已生成
