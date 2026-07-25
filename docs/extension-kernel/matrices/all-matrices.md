# 表与 Service 矩阵

> 审计日期: 2026-07-25

## 读/写矩阵

| 表 | Extension Service | AgentSkill Service | Plugin Manager | Package Service | Workshop Service | MCP Service | OwnedResource |
|---|---|---|---|---|---|---|---|
| extensions | CRUD | R,W(enable),D(archive) | W(plugin lifecycle) | R | R,W(install) | - | - |
| extension_versions | CRUD | - | - | CRUD | W | - | - |
| extension_capability_grants | CRUD | - | - | - | - | - | - |
| extension_configs | CRUD | - | R | - | - | - | - |
| extension_runs | CRUD | - | - | - | - | - | - |
| extension_agent_skill_metadata | - | CRUD | - | - | - | - | - |
| extension_agent_skill_activations | - | C,R,D(级联) | - | - | - | - | - |
| extension_owned_resources | - | - | - | - | - | - | CRUD |
| extension_package_* (5 张) | - | - | - | CRUD | - | - | - |
| extension_scope_bindings | CRUD | - | - | - | - | - | - |
| extension_workshop_* (3 张) | - | - | - | - | CRUD | - | - |
| extension_artifacts | D(级联) | CRUD | - | CRUD | CRUD | - | - |
| extension_states | - | - | W(upsert) | - | - | - | - |
| extension_state_revisions | - | - | W | - | - | - | - |
| extension_events | - | - | C,R | - | - | - | - |
| extension_event_deliveries | - | - | CRUD | - | - | - | - |
| extension_schedules | - | - | CRUD | - | - | - | - |
| extension_plugin_runs | - | - | CRUD | - | - | - | - |
| extension_audits | - | - | W | - | - | - | - |
| mcp_* (13 张) | - | - | - | - | - | CRUD | - |

图例: C=Create, R=Read, U=Update, D=Delete/Archive, W=Write(Upsert)

---

# 资源所有权矩阵

> 唯一所有者 → 资源 → 引用者 → 删除前检查

| 资源 | 唯一所有者 | 引用者 | 使用者 | 配置者 | 删除前检查 |
|---|---|---|---|---|---|
| Extension 定义 | Extension Service | AgentSkill, Plugin, Workshop, Package | Chat, API | 用户/包 | enabled 状态、scope bindings |
| Agent Skill | AgentSkill Service | MCP Dependency Service | Chat (Agent Skill Runtime) | 用户 | MCP dependencies, activations |
| Plugin 运行时 | PluginManager | Extension Registry | Chat (Hook 调用) | 用户 | Schedule, Event, Delivery, State |
| Workshop Session | WorkshopService | WorkshopInstaller | 用户（开发） | 用户 | 已安装 Skill |
| Package 安装 | PackageService | AgentSkill, Workshop | 用户（安装） | 用户 | 已安装 Skill, Version |
| MCP Server | 用户 | Agent Skill (dependency), Extension Registry | Chat (Tool 调用) | 用户/MCP API | Scope bindings, credentials, tools, dependencies |
| MCP Tool | Discovery Service | Extension Registry | Chat (Tool 调用) | 用户 (enabled switch) | Server 关联 |
| MCP Secret | EncryptedFileStore | Factory | Manager (连接注入) | MCP API | credential reference |
| Extension Config | Extension Service | Plugin Host | 插件执行 | 用户 | 无 |
| Plugin State | PluginManager | Plugin Host | 插件执行 | 插件代码 | 无 |

---

# 生命周期矩阵

| 操作 | Extension/Skill | Plugin | Workshop | Package | MCP Server | Secret | Artifact |
|---|---|---|---|---|---|---|---|
| **安装** | DB INSERT + Registry Register | DB INSERT + PluginManager.Register | CreateSession | DB INSERT 多表 + 解析 | DB INSERT | Put | DB INSERT blob/JSON |
| **启用** | DB UPDATE enabled=1 + Registry | UpdatePluginLifecycle | CAS WorkshopEnabled | 重新注册 Skill | Manager.Connect 启动连接/子进程 | 无变化 | 无变化 |
| **禁用** | DB UPDATE enabled=0 + Registry Unregister | PluginManager 停止 hooks | CAS WorkshopDisabled | 卸载 Skill | Manager.Disconnect 关闭连接/终止子进程 | 无变化 | 无变化 |
| **升级** | New Version + Registry Replace | 重新 Register（旧版本 Unregister） | 新 Revision → 新生成 → 新安装 | 新 Version → 覆盖旧安装 | UpdateServer → Reconnect | 无自动操作 | 旧 Artifact 归档 + 新 Artifact 创建 |
| **回滚** | 切换 current_version | 恢复旧 Manifest（无自动机制） | 无自动回滚 | 无自动回滚 | 无自动回滚 | 无自动操作 | 无自动操作 |
| **卸载** | DB archived_at + Registry Unregister + Scope Bindings 删除 | PluginManager 完全移除 | Session archived_at | Uninstall 多表级联归档 | DeleteServer 级联删除关联表 | **不删除** | archived_at |
| **崩溃恢复** | Restore() 从 DB 重新注册 | 内存状态丢失，从 DB 恢复 PluginRegistry | Restore() 恢复活跃 Session | Restore() 重新注册已安装 Skill | Manager.Restore() 重建连接 | 文件不变 | 从 DB 重新读取 |

---

# 作用域矩阵

| 数据 | 全局 | 角色 | 会话 | 扩展 | 模块 | 临时执行 |
|---|---|---|---|---|---|---|
| Extension 定义 | ✓ | - | - | - | - | - |
| Extension 启用状态 | - | ✓ (scope binding) | - | - | - | - |
| Agent Skill | - | ✓ | - | - | - | - |
| Plugin 启用 | - | ✓ | - | - | - | - |
| Workshop Session | - | ✓ | - | - | - | - |
| Package 安装 | ✓ | - | - | - | - | - |
| MCP Server | ✓ (定义) | ✓ (scope binding) | - | - | - | - |
| MCP Tool | ✓ (定义) | ✓ (binding 间接) | - | - | - | - |
| Extension Config | - | ✓ (scope 维度) | - | - | - | - |
| Plugin State | - | ✓ (scope 维度) | - | - | - | - |
| Execution Run | - | ✓ | ✓ | - | - | - |
| Audit Log | ✓ | ✓ | ✓ | - | - | - |

---

# 敏感数据矩阵

| Secret 类型 | 存储位置 | 加密方式 | 读取者 | 导出行为 | 卸载行为 |
|---|---|---|---|---|---|
| Extension Config | `extension_configs.config_json` | AES-GCM (enc:v1:) | Plugin Host | 密文导出 | 保留 |
| Plugin State | `extension_states.state_json` | AES-GCM (enc:v1:) | Plugin Host | 密文导出 | 随插件删除 |
| MCP bearer_token | `mcp-secrets.json` | AES-256-GCM (文件) + AAD | Factory.resolveCredential | 不导出 | 不删除 |
| MCP custom_headers | `mcp-secrets.json` | 同上 | Factory | 不导出 | 不删除 |
| MCP stdio_env | `mcp-secrets.json` | 同上 | Factory | 不导出 | 不删除 |
| MCP OAuth code_verifier | `mcp-secrets.json` | 同上 | OAuthManager | 不导出 | 不删除 |
| MCP OAuth Token | 内存 | 无 | OAuthManager | 不导出 | 不适用 |

---

# 清理责任矩阵

| 资源 | 清理函数 | 触发时机 | 是否完整 |
|---|---|---|---|
| Extension | `Repository.Delete()` → archived_at | 用户操作 / 包卸载 | ✓ (软删除 + 级联归档关联表) |
| Agent Skill | `RemoveAgentSkill()` → archived_at | 用户操作 | ✓ (归档 artifact + metadata + activations) |
| Plugin | `Unregister()` + DB archived_at | 用户操作 | ✗ (Schedule/Event/Delivery 不清理) |
| Workshop Session | `Archive()` → archived_at | 用户操作 / 超时 | ✓ |
| Package | `Uninstall()` → 多表级联归档 | 用户操作 | ✓ |
| MCP Server | `DeleteServer()` → 级联删除关联表 | 用户操作 | ✗ (Secret 不删除) |
| Extension Config | `DeleteConfig()` | 用户操作 | ✓ |
| Plugin State | 随插件删除 | 用户操作 | ✓ |
| Owned Resource | `Delete()` | 仅 schedule_create 类型 | ✗ (其他资源类型未覆盖) |
| MCP Secret | `Delete()` | 无自动触发 | ✗ (需手动调用) |
