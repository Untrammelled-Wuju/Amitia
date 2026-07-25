# 第 3 步：建立数据表与资源归属清单 — 完成报告

> 审计日期: 2026-07-25
> 基于第 2 步调用链地图的完整资源盘点

---

## 一、执行摘要

完成对 Amitia 当前扩展系统的完整资源盘点，覆盖 **38 张数据库表**、**2 类文件存储**、**3 种 Secret 机制**、 **12 个内存结构**和 **10 个 Worker/子进程**。

核心发现：
- **3 个 P0 问题**：多写入者冲突、Secret 不随 Server 清理、双写不一致
- **4 个 P1 问题**：Schedule 禁用后继续触发、Plugin 卸载不清理事件、三系统共享 Artifact 表、依赖链接无外键
- **扩展系统几乎不使用独立文件系统**，数据主要存储在 SQLite 数据库和加密 JSON 文件中

---

## 二、审计覆盖

| 审计项 | 状态 | 详细文档 |
|---|---|---|
| 38 张扩展相关数据库表 | ✓ 完成 | [database-tables.md](inventories/database-tables.md) |
| 7 个 Repository 与表的完整映射 | ✓ 完成 | [repositories.md](inventories/repositories.md) |
| 文件系统路径与 Artifact 存储 | ✓ 完成 | [file-artifacts.md](inventories/file-artifacts.md) |
| 3 类 Secret 存储与加密机制 | ✓ 完成 | [secrets.md](inventories/secrets.md) |
| 12 个内存 Registry/缓存/队列 | ✓ 完成 | [runtime-resources.md](inventories/runtime-resources.md) |
| 10 个 Worker/goroutine/子进程 | ✓ 完成 | [workers-processes.md](inventories/workers-processes.md) |
| 缓存与 DB 一致性分析 | ✓ 完成 | [caches-registries.md](inventories/caches-registries.md) |
| 6 个维度矩阵 | ✓ 完成 | [all-matrices.md](matrices/all-matrices.md) |
| 4 类问题报告 | ✓ 完成 | [all-reports.md](reports/all-reports.md) |
| 前端 localStorage 审计 | ✓ 完成 | 仅 2 个 key（avatar, default-char），无扩展数据 |
| Electron ConfigStore 审计 | ✓ 完成 | 仅部署配置，无扩展数据 |

---

## 三、关键数据统计

### 数据库
- **38 张表**：Extension Core (5) + Agent Skills (2) + Owned Resources (1) + Packages (5) + Scope Bindings (1) + Workshop (4) + Plugin Runtime (7) + MCP (13)
- **3 张跨 Repository 共享表**：`extensions`, `extension_artifacts`, `extension_versions`
- **2 张含加密字段表**：`extension_configs`, `extension_states`

### 内存
- **2 个 Registry**：Extension Registry, Plugin Registry
- **4 个缓存**：Agent Skill Artifact, Catalog, Preview, Round State
- **3 个 Channel**：afterReplyQ, eventWake, eventIngress
- **1 个熔断器**：Plugin Circuit Breaker

### Worker/子进程
- **4 个 Plugin Manager goroutine**：afterReplyWorker, eventIngressWorker, eventWorker, scheduleWorker
- **3 种 MCP goroutine**：Reconnect, Connection Done 监听, Ready Handler 通知
- **2 种 MCP Transport**：stdio 子进程, Streamable HTTP 连接

### Secret
- **3 种加密机制**：AES-GCM (config/state), AES-256-GCM (MCP secrets file)
- **2 个密钥文件**：`{dbPath}.extension-key`, `{dataDir}/mcp-secrets.key`
- **1 个 Secret 文件**：`{dataDir}/mcp-secrets.json`

---

## 四、P0 阻塞项

| 编号 | 问题 | 涉及资源 | 阻塞原因 |
|---|---|---|---|
| P0-1 | extensions 表三写入者（Repository / PluginManager / AgentSkillRepository） | extensions 表 enabled 字段 | 并发写入冲突，需明确单一所有者 |
| P0-2 | MCP Secret 不随 Server 删除清理 | mcp-secrets.json | 安全风险，Secret 无限增长 |
| P0-3 | extension_scope_bindings 与 extensions.enabled 双写 | 两张表 | 状态不一致风险，需统一来源 |

---

## 五、P1 高风险项

| 编号 | 问题 | 建议 |
|---|---|---|
| P1-1 | Plugin 禁用后 Schedule 继续触发 | scheduleWorker 检查 enabled 状态 |
| P1-2 | Plugin 卸载后 Event/Delivery 不清理 | 卸载时级联清理 |
| P1-3 | extension_artifacts 三系统共享 | 拆分为独立表 |
| P1-4 | mcp_dependency_links 无物理外键 | 增加引用完整性检查 |

---

## 六、新 Extension Kernel 组件归属总览

```text
┌─────────────────────────────────────────────┐
│              Extension Kernel               │
├─────────────────────────────────────────────┤
│ Package Store                               │
│   ← extensions, extension_versions,         │
│      extension_package_*                    │
├─────────────────────────────────────────────┤
│ Runtime Supervisor                          │
│   ← extension_states, extension_events,     │
│      extension_schedules, PluginManager     │
│      workers, MCP connections               │
├─────────────────────────────────────────────┤
│ Tool Registry                               │
│   ← extensions (Tool 定义), mcp_tools       │
├─────────────────────────────────────────────┤
│ Agent Skill Catalog                         │
│   ← extension_agent_skill_metadata,         │
│      extension_artifacts (agent-skill)      │
├─────────────────────────────────────────────┤
│ MCP Manager                                 │
│   ← mcp_servers, mcp_tools, mcp_resources,  │
│      mcp_prompts, mcp_dependency_links      │
├─────────────────────────────────────────────┤
│ Workflow Engine                             │
│   ← extension_workshop_*,                   │
│      extension_artifacts (workshop)         │
├─────────────────────────────────────────────┤
│ Permission Broker                           │
│   ← extension_capability_grants,            │
│      extension_scope_bindings,              │
│      mcp_server_scope_bindings              │
├─────────────────────────────────────────────┤
│ Secret Broker                               │
│   ← extension_configs, extension_states,    │
│      mcp_secrets.json, mcp_server_credentials│
├─────────────────────────────────────────────┤
│ Audit Store                                 │
│   ← extension_runs, extension_plugin_runs,  │
│      extension_audits, mcp_audit_logs,      │
│      mcp_operations                         │
└─────────────────────────────────────────────┘
```

---

## 七、表迁移决策汇总

| 决策 | 表 | 数量 |
|---|---|---|
| **保留** | extensions, extension_versions, extension_workshop_sessions, extension_workshop_revisions, mcp_servers | 5 |
| **拆分** | extension_artifacts → 3 张独立表 | 1→3 |
| **合并** | extension_runs + extension_plugin_runs + mcp_audit_logs + mcp_operations → 统一 audit_logs | 4→1 |
| **合并** | extension_events + extension_event_deliveries → 统一 event_bus | 2→1 |
| **只读历史** | extension_state_revisions | 1 |
| **删除重建** | extension_owned_resources | 1 |
| **保持** | 其余 24 张表结构不变 | 24 |

---

## 八、后续第 4 步前置条件

本步骤已满足第 4 步进入条件：

- [x] 所有扩展相关表已找到（38 张）
- [x] 所有文件目录和 Artifact 已找到
- [x] 所有 Secret 存储位置已确认
- [x] 所有后台 Worker 和子进程已确认
- [x] 主要资源所有者已明确
- [x] 无法确认所有权的资源已形成阻塞清单（3 项 P0）
- [x] 生命周期矩阵已完成
- [x] 卸载和回滚涉及的资源已全部列出
- [x] 新 Extension Kernel 的目标归属已初步标记
- [x] 本步骤未引入任何行为修改

---

## 九、产出文档索引

### 清单 (inventories/)
| 文件 | 内容 |
|---|---|
| [database-tables.md](inventories/database-tables.md) | 38 张表完整字段、索引、Repository、读写入口、数据分类、目标处理 |
| [repositories.md](inventories/repositories.md) | 7 个 Repository 到 38 张表的完整操作映射 |
| [file-artifacts.md](inventories/file-artifacts.md) | 文件系统路径、Artifact 存储方案（DB blob vs JSON）、确认不存在的路径 |
| [secrets.md](inventories/secrets.md) | 3 种 Secret 机制、加密细节、引用链、生命周期汇总 |
| [runtime-resources.md](inventories/runtime-resources.md) | 2 个 Registry、4 个缓存、3 个 Channel、MCP 连接状态 |
| [workers-processes.md](inventories/workers-processes.md) | 4 个 Plugin Worker、3 种 MCP goroutine、2 种 Transport、启动/关闭顺序 |
| [caches-registries.md](inventories/caches-registries.md) | 缓存与 DB 一致性分析、权威来源判定 |

### 矩阵 (matrices/)
| 文件 | 内容 |
|---|---|
| [all-matrices.md](matrices/all-matrices.md) | 表-Service 矩阵、所有权矩阵、生命周期矩阵、作用域矩阵、敏感数据矩阵、清理责任矩阵 |

### 报告 (reports/)
| 文件 | 内容 |
|---|---|
| [all-reports.md](reports/all-reports.md) | P0/P1 问题报告、临时资源泄漏报告、迁移目标汇总 |
