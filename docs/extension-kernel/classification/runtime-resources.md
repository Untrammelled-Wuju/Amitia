# 运行资源分类

> 范围：所有扩展系统运行时的内存状态、goroutine、连接、缓存、定时器等

---

## 一、Worker / Goroutine

| 资源 | 文件 | 所有者 | 处理方式 |
|---|---|---|---|
| Plugin Hook 并发 goroutine | `plugin_manager.go` | PluginManager | 改造后复用 → Event Bus |
| Plugin 调度 goroutine | `plugin_manager.go` | PluginManager | 改造后复用 → Schedule Manager |
| Workflow 执行 goroutine | `workflow_executor.go` | WorkflowExecutor | 保留并抽取 → Workflow Engine |
| MCP 子进程 | `transport/stdio.go`, `transport/process_*.go` | MCP Manager | 保留并抽取 → MCP Manager |
| Executor handler goroutine | `executor.go` | Executor | 保留并抽取 → Runtime Supervisor |
| Package 安装 goroutine | `package_installer.go` | PackageService | 改造后复用 → Package Manager |
| Workshop 恢复 goroutine | `workshop_installer.go` | WorkshopInstaller | 改造后复用 → Package Manager |

---

## 二、Cache

| 资源 | 文件 | 所有者 | 处理方式 |
|---|---|---|---|
| Registry 内存 items map | `registry.go` | Registry | 改造后复用 → Contribution Registry |
| Registry 内存 modelNames map | `registry.go` | Registry | 最终删除（新模型不需要） |
| Executor 幂等缓存 | `executor.go` | Executor | 保留并抽取 → Runtime Supervisor |
| Executor inFlight 缓存 | `executor.go` | Executor | 保留并抽取 → Runtime Supervisor |
| AgentSkill Artifact 缓存 | `agent_skill_service.go` | AgentSkillService | 改造后复用 → Agent Skill Catalog |
| AgentSkill Catalog 缓存 | `agent_skill_service.go` | AgentSkillService | 改造后复用 → Agent Skill Catalog |
| AgentSkill Round State | `agent_skill_service.go` | AgentSkillService | 改造后复用 → Runtime Supervisor |
| Plugin 状态缓存 | `plugin_registry.go` | PluginRegistry | 最终删除 → Storage Broker |
| Workshop Session 锁 | `workshop_service.go` | WorkshopService | 改造后复用 → Developer Tooling |
| Package 导入 Session 缓存 | `package_service.go` | PackageService | 改造后复用 → Package Manager |

---

##三、Registry（内存注册中心）

| 资源 | 文件 | 所有者 | 处理方式 |
|---|---|---|---|
| Registry (SkillRegistry) | `registry.go` | Runtime | 改造后复用 → Contribution Registry |
| PluginRegistry | `plugin_registry.go` | Runtime | 最终删除 → Contribution Registry 统一 |
| WorkflowAdapterRegistry | `workflow_executor.go` | WorkflowExecutor | 改造后复用 → Workflow Engine |

---

## 四、Connection（连接）

| 资源 | 文件 | 所有者 | 处理方式 |
|---|---|---|---|
| MCP Client Connections | `manager/manager.go` | MCP Manager | 改造后复用 → MCP Manager |
| MCP Subprocess stdin/stdout | `transport/stdio.go` | MCP Transport | 保留并抽取 → MCP Manager |
| MCP Streamable HTTP Client | `transport/streamable_http.go` | MCP Transport | 保留并抽取 → MCP Manager |
| OAuth Sessions | `auth/oauth.go`, `repository.go` | MCP Manager | 保留并抽取 → MCP Manager / Secret Broker |
| HTTP Workflow Client | `workflow_executor.go` | Workflow Executor | 改造后复用 → Workflow Engine |

---

## 五、Subprocess（子进程）

| 资源 | 文件 | 所有者 | 处理方式 |
|---|---|---|---|
| MCP stdio 子进程 | `transport/process_*.go` | MCP Transport | 保留并抽取 → MCP Manager |
| MCP Server 启动进程 | `manager/manager.go` | MCP Manager | 改造后复用 → MCP Manager |

---

## 六、Timer / Schedule（定时器与调度）

| 资源 | 文件 | 所有者 | 处理方式 |
|---|---|---|---|
| Plugin Schedule Timer | `plugin_manager.go` | PluginManager | 改造后复用 → Schedule Manager |
| Executor Timeout Timer | `executor.go` | Executor | 保留并抽取 → Runtime Supervisor |
| Plugin 事件重试 Timer | `plugin_manager.go` | PluginManager | 改造后复用 → Event Bus |
| OwnedResource Cleanup Retry | `owned_resource_repository.go` | Repository | 保留并抽取 → Storage Broker |
| MCP Task 过期清理 | `repository.go` | MCP Repository | 改造后复用 → MCP Manager |

---

## 七、Queue（队列）

| 资源 | 文件 | 所有者 | 处理方式 |
|---|---|---|---|
| Executor handlerSlots (channel) | `executor.go` | Executor | 保留并抽取 → Runtime Supervisor |
| Plugin 事件队列 | `plugin_manager.go` | PluginManager | 改造后复用 → Event Bus |

---

## 八、Hook（钩子）

| 资源 | 文件 | 所有者 | 处理方式 |
|---|---|---|---|
| Plugin BeforePrompt Hook | `plugin_manager.go` | PluginManager | 改造后复用 → Hook Pipeline |
| Plugin AfterReply Hook | `plugin_manager.go` | PluginManager | 改造后复用 → Hook Pipeline |
| Plugin SystemEvent Hook | `plugin_manager.go` | PluginManager | 改造后复用 → Event Bus |

---

## 九、锁

| 资源 | 文件 | 所有者 | 处理方式 |
|---|---|---|---|
| Registry sync.RWMutex | `registry.go` | Registry | 改造后复用 → Contribution Registry |
| Executor idempotencyMu | `executor.go` | Executor | 保留并抽取 → Runtime Supervisor |
| Workshop Session 锁 | `workshop_service.go` | WorkshopService | 改造后复用 → Developer Tooling |
| Plugin 状态锁 | `plugin_registry.go` | PluginRegistry | 改造后复用 → Storage Broker |

---

## 十、迁移关键

- **P0**: MCP 运行中的连接在迁移时不能中断，需要平滑切换
- **P0**: Executor 的幂等缓存是执行安全保障，不可丢失逻辑
- **P1**: Plugin Hook 迁移需要确保 BeforePrompt/AfterReply 事件不丢失
- **P2**: Workshop 锁迁移在用户未操作时进行
