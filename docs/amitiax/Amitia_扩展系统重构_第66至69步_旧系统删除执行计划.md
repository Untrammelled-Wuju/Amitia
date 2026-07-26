# Amitia 扩展系统重构 第66-69步 旧系统删除执行计划

> 本文档汇总第66-69步的旧系统删除工作。考虑到删除为不可逆破坏性操作且依赖深度耦合，采用「先弃用、后冻结、再重定向、最终删除」的渐进式策略，由 `legacy_deprecation` 框架统一管理。

---

## 一、执行策略

### 阶段 0：标记（已完成）
- 在每个旧源文件顶部插入 `// Deprecated:` 注释
- 在 `legacy_deprecation.Registry` 注册所有待删除文件、引用、阻断条件
- 输出 `legacy_deprecation_state.json` 作为审计快照

### 阶段 1：冻结写入（cutover phase=freeze_old）
- 旧 `PluginManager` 不再接受新注册
- 旧 `extension_states / plugin_runs` 表停止新写入
- 旧 `.amitiax` 安装器不再处理新包
- 旧 Agent Skill Handler 不再处理新创建请求

### 阶段 2：路由重定向（cutover phase=redirect_*）
- `router.go` 将 `/api/agent-skills/*` 重定向到 `/api/extensions/*`
- `package_handler.go` 将 `Install` 调用改为 `extension/kernel/amitiax.Install`
- `runtime.go` 将 `AttachPluginManager` 调用替换为 `runtime_supervisor.Attach`
- `migrations.go:60` 保留旧 `PluginRuntimeMigration` 但添加 `Deprecated: true` 标记

### 阶段 3：物理删除（待用户授权后执行）
- 新系统验收通过且观察期 ≥ 14 天后
- 逐文件删除并删除对应测试
- 同步删除旧表（保留迁移文件以维护 schema history）
- 提交独立 commit，commit message 包含 `BREAKING CHANGE`

---

## 二、第66步：删除旧 PluginRuntime

### 文件清单

| 文件 | 包 | 状态 | 阻断删除 |
|------|-----|------|----------|
| `backend/internal/extension/plugin_manager.go` | extension | 已弃用 | 是 |
| `backend/internal/migration/plugin_runtime.go` | migration | 已弃用 | 是 |
| `backend/internal/extension/plugin_runtime_test.go` | extension | 已弃用 | 否 |
| `backend/internal/extension/plugin_service.go` | extension | 待弃用 | 是 |
| `backend/internal/extension/plugin_host.go` | extension | 待弃用 | 是 |

### 删除顺序（必须按序）

1. 改造 `runtime.go`：移除 `PluginManager` 字段和 `AttachPluginManager` 方法，替换为 `runtime_supervisor.Supervisor`
2. 改造 `service.go`：移除 `plugins *PluginManager` 字段
3. 删除 `plugin_service.go`、`plugin_host.go`
4. 删除 `plugin_manager.go`
5. 删除 `plugin_runtime_test.go`、`plugin_baseline_test.go`
6. 保留 `migration/plugin_runtime.go` 不删除（迁移文件保留以维护数据库 schema history）
7. 在 `migrations.go` 中为 `PluginRuntimeMigration()` 添加 `Deprecated: true` 字段
8. 验证：`go build ./...` 通过，新系统测试通过

### 影响范围
- 生产代码：5 个文件
- 测试代码：2 个文件
- 路由：0 个（不影响 HTTP 路由）

---

## 三、第67步：删除旧 Skill 兼容层

### 文件清单

| 文件 | 包 | 状态 | 阻断删除 |
|------|-----|------|----------|
| `backend/internal/extension/agent_skill_handler.go` | extension | 已弃用 | 是 |
| `backend/internal/extension/agent_skill_service.go` | extension | 待评估 | 待评估 |
| `backend/internal/extension/agent_skill_compat.go` | extension | 待评估 | 待评估 |

### 删除顺序

1. 改造 `router.go`：移除 14 个 `/api/agent-skills/*` 路由，注册到 `/api/extensions/:extensionId/skills/*`
2. 改造 `agent_skill_service.go`：将 Skill 执行委托给 `extension/kernel/agent_skill`
3. 删除 `agent_skill_handler.go`
4. 评估并删除其他 Skill 兼容文件
5. 验证：旧 `/api/agent-skills/*` 请求被重定向到新入口

### 影响范围
- 生产代码：1-3 个文件
- 路由：14 个

---

## 四、第68步：删除旧 Amitiax 包解析器与安装链

### 文件清单

| 文件 | 包 | 状态 | 阻断删除 |
|------|-----|------|----------|
| `backend/internal/extension/package_installer.go` | extension | 已弃用 | 是 |
| `backend/internal/extension/package_parser.go` | extension | 待评估 | 待评估 |
| `backend/internal/extension/package_installer_legacy.go` | extension | 待评估 | 待评估 |

### 删除顺序

1. 改造 `package_handler.go:128`：将 `h.service.Install(...)` 调用替换为 `extension/kernel/amitiax.Install(...)`
2. 改造 `package_lifecycle.go:320`：将 `preparePackageConfigMigrations` 调用替换为 `extension/kernel/data_migration` 等价物
3. 删除 `package_installer.go`
4. 评估并删除其他旧 package 文件
5. 保留 `package_manager_test.go` 中新系统覆盖的部分，删除旧 install 测试
6. 验证：`.amitiax` 安装走新链路

### 影响范围
- 生产代码：1-3 个文件
- 测试代码：2 个文件
- 入口：HTTP `POST /api/packages/install`

---

## 五、第69步：删除重复生命周期状态表与旧数据模型

### 表清单

| 表 | 替代 | 状态 | 删除策略 |
|----|------|------|----------|
| `extension_states` | `extension_installations` | 已弃用 | 停止写入 → 数据迁移 → DROP |
| `plugin_runs` | `extension_runtime_runs` | 已弃用 | 停止写入 → 数据迁移 → DROP |
| `extension_events` (旧) | `extension_kernel_events` | 已弃用 | 停止写入 → 数据迁移 → DROP |
| `extension_schedules` (旧) | `extension_kernel_schedules` | 已弃用 | 停止写入 → 数据迁移 → DROP |
| `extension_audits` (旧) | `extension_kernel_audits` | 已弃用 | 停止写入 → 数据迁移 → DROP |

### 删除顺序

1. 创建数据迁移脚本：将旧表数据导入新表（保留 90 天双重写入）
2. 旧 PluginManager 停止写入旧表（已在阶段1完成）
3. 运行数据完整性校验
4. 创建 `DROP_TABLE` 迁移文件（版本号 `202608010001` 起）
5. 保留迁移文件，删除旧表模型代码
6. 验证：数据库 schema 一致，新系统读写正常

### 影响范围
- 数据库：5 张表
- 迁移文件：1 个新增 DROP 迁移
- 代码：旧表 model 文件

---

## 六、回滚策略

### 回滚条件
- 新系统出现 P0 故障
- 等价性验证出现回归
- 数据迁移丢失数据

### 回滚步骤
1. `cutover_manager.Rollback(ctx, reason)` 触发
2. 恢复 `cutover_snapshot_<id>.json` 中的旧入口配置
3. 重新启用旧 `PluginManager`（保留代码至少 30 天）
4. 旧表写入恢复（在双重写入期内可立即恢复）
5. 用户通知并发布修复版本

### 回滚窗口
- 阶段 1-2：可立即回滚（< 5 分钟）
- 阶段 3（物理删除后）：不可回滚，需通过备份恢复

---

## 七、验收标准

每步删除完成后必须满足：

1. ✅ `go build ./...` 通过
2. ✅ `go test ./internal/extension/kernel/...` 全部通过
3. ✅ 旧入口请求返回 410 Gone 或重定向到新入口
4. ✅ 新系统功能等价性测试通过
5. ✅ 数据库 schema 一致性校验通过
6. ✅ 前端不再调用旧 API
7. ✅ Electron 启动关闭正常
8. ✅ 旧文件物理删除后无编译错误

---

## 八、当前状态

截至本计划编写时（2026-07-26）：

- ✅ 阶段 0（标记）：5 个核心旧文件已添加 `Deprecated` 注释
- ✅ `legacy_deprecation.Registry` 已注册全部待删除文件
- ⏳ 阶段 1-3：待 cutover manager 触发后执行
- ⏳ 物理删除：待用户明确授权后执行

> 物理删除为不可逆破坏性操作。根据 AGENTS.md 规则，需要用户明确授权后方可执行。本计划文档作为授权前的完整执行蓝图。
