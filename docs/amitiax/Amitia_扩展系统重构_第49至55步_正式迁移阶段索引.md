# Amitia 扩展系统重构：正式迁移阶段索引

本阶段范围：第 49—55 步。

1. 第 49 步：迁移系统内置 Tools
2. 第 50 步：迁移 Agent Skills
3. 第 51 步：迁移 MCP 系统
4. 第 52 步：迁移 Workflows
5. 第 53 步：迁移官方内置 Plugins
6. 第 54 步：迁移旧 `.amitiax` 扩展包
7. 第 55 步：迁移扩展数据并完成正式切换准备

## 本阶段完成后的系统状态

```text
旧 Tool / Skill / MCP / Workflow / Plugin / Package
→ Read-only Migration Boundary
→ Extension Kernel Domain
→ Contribution Registry
→ Runtime Supervisor
→ Unified Storage / Scope / Permission / Audit
```

## 核心迁移约束

- 旧 Tool Handler 不再直接执行。
- Agent Skill 不再伪装 Tool。
- MCP 不再拥有独立生命周期和 Tool Registry。
- Workflow 不再属于 Skill。
- 官方 Plugin 也必须进入 Extension Kernel。
- 旧 `.amitiax` 不再直接运行或安装。
- Secret 不进入普通迁移快照。
- 迁移不扩大 Enabled、Scope 或 Permission。
- 不采用长期双写。
- 新旧 Scheduler、Event、Tool 和 MCP Runtime 不得同时运行。

下一阶段从第 56 步开始，进入开发者生态和 Extension Center 重建阶段：TypeScript SDK、CLI、开发模式、开发者控制台、扩展中心和扩展详情页。
