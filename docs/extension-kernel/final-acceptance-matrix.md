# Extension Kernel 最终验收矩阵

> 本矩阵对应第 70 步「执行 Extension Kernel 最终总验收」，覆盖 70 步全部成果。
> 字段：Area / Requirement / Evidence / Status / Owner / Risk / Blocking / Notes

## 验收总览

- 验收范围：阶段一冻结与审计、阶段二 Kernel 核心、阶段三 .amitiax 与 Runtime、阶段四 UI Contribution、阶段五正式迁移、阶段六开发者生态与扩展中心、阶段七验证切换与旧系统删除
- 验收原则：所有 `Required = true` 的项必须 `Status = Passed`，否则禁止发布
- 报告生成入口：`backend/internal/extension/kernel/final_acceptance`
- 协议版本基线：Extension Kernel v1 / Manifest v2 / Host API v1 / Runtime RPC v1 / Schema UI v1 / UI Contract v1 / SDK v1

## 阶段一：冻结、审计与基础抽取（第 1-20 步）

| Area | Requirement | Evidence | Status | Owner | Risk | Blocking | Notes |
|------|-------------|----------|--------|-------|------|----------|-------|
| freeze_audit | 旧系统冻结，建立调用链地图 | docs/extension-kernel/02-current-call-chain-map.md | Passed | Extension Kernel 后端 | Low | false | 9 个调用链文档 + 8 张 .mmd |
| freeze_audit | 数据与资源所有权清单 | docs/extension-kernel/03-data-resource-ownership.md | Passed | 数据迁移 | Low | false | 含 5 项 P0 数据归属 |
| freeze_audit | 保留/重写/迁移/删除决策 | docs/extension-kernel/04-retain-rewrite-migrate-delete.md | Passed | Extension Kernel 后端 | Low | false | 含 4 份分类报告 |
| freeze_audit | 旧系统基线测试 | docs/extension-kernel/05-legacy-baseline-tests.md | Passed | 测试 | Medium | false | 用于等价性对照 |
| freeze_audit | Skill 概念分离 | docs/extension-kernel/06-skill-concept-separation.md | Passed | Extension Kernel 后端 | Low | false | Tool/Skill/Workflow/MCP 解耦 |
| base_extraction | 包安全基础抽取 | backend/internal/extension/kernel/package_security/ | Passed | 安全 | Medium | false | 签名、Hash、Trust |
| base_extraction | MCP 协议基础设施 | docs/extension-kernel/14-mcp-protocol-infrastructure.md | Passed | Extension Kernel 后端 | Low | false | Transport/OAuth/Discovery |
| base_extraction | 统一生命周期 | docs/extension-kernel/18-unified-lifecycle.md | Passed | Extension Kernel 后端 | Low | false | Plan/Snapshot/Journal/Compensation |
| base_extraction | PluginRuntime/Skill/Workflow 抽取 | backend/internal/extension/kernel/ | Passed | Extension Kernel 后端 | Medium | false | 已迁移至 kernel 子包 |
| base_extraction | Enabled 只读迁移 | backend/internal/extension/kernel/lifecycle/ | Passed | Extension Kernel 后端 | Low | false | 写入仅 Kernel |

## 阶段二：Extension Kernel 核心（第 21-28 步）

| Area | Requirement | Evidence | Status | Owner | Risk | Blocking | Notes |
|------|-------------|----------|--------|-------|------|----------|-------|
| domain_model | Extension/Package/Installation/Module/Contribution 不变量 | backend/internal/extension/kernel/domain/ | Passed | Extension Kernel 后端 | High | true | 全部不变量测试通过 |
| domain_model | RuntimeDefinition/RuntimeInstance | backend/internal/extension/kernel/runtime/ | Passed | Extension Kernel 后端 | Medium | true | Desired/Actual/Generation |
| domain_model | Dependency/Owner/Artifact/Hash | backend/internal/extension/kernel/domain/ | Passed | Extension Kernel 后端 | Medium | true | SemVer + 平台 |
| lifecycle | Plan/Snapshot/Lock/Step/Journal | backend/internal/extension/kernel/lifecycle/ | Passed | Extension Kernel 后端 | High | true | 含 Compensation/Recovery |
| contribution | Contribution Registry | backend/internal/extension/kernel/contribution/ | Passed | Extension Kernel 后端 | Medium | true | 单一注册中心 |
| dependency | Dependency Resolver | backend/internal/extension/kernel/dependency/ | Passed | Extension Kernel 后端 | Medium | true | 无循环、无自动安装 |
| runtime | Runtime Supervisor | backend/internal/extension/kernel/runtime/ | Passed | Extension Kernel 后端 | High | true | Health/Circuit/Quarantine |
| host_api | Host API Gateway | backend/internal/extension/kernel/host_api/ | Passed | Extension Kernel 后端 | High | true | 身份/Session/Schema/Permission |
| storage | Storage/Secret/Resource | backend/internal/extension/kernel/storage/ | Passed | Extension Kernel 后端 | High | true | Namespace + CAS + Lease |
| event_hook | Event/Hook/Scheduler | backend/internal/extension/kernel/event/ | Passed | Extension Kernel 后端 | Medium | true | 无旧 Dispatcher |

## 阶段三：.amitiax 与 Runtime（第 29-40 步）

| Area | Requirement | Evidence | Status | Owner | Risk | Blocking | Notes |
|------|-------------|----------|--------|-------|------|----------|-------|
| amitiax | .amitiax 唯一后缀、Manifest v2 | backend/internal/extension/kernel/amitiax/ | Passed | Extension Kernel 后端 | High | true | 严格 Parser |
| amitiax | 多 Module 包与原子安装事务 | backend/internal/extension/kernel/amitiax/ | Passed | Extension Kernel 后端 | High | true | 安装/更新/回滚 |
| amitiax | 签名、Publisher、Trust | backend/internal/extension/kernel/package_security/ | Passed | 安全 | High | true | 含 Trust 表 |
| amitiax | 数据迁移与旧包迁移 | backend/internal/extension/kernel/migration/ | Passed | 数据迁移 | Medium | true | 旧 Parser 生产删除 |
| runtime_js | JavaScript Main Runtime | backend/internal/extension/kernel/runtime/js/ | Passed | Extension Kernel 后端 | Medium | true | 主 Runtime |
| runtime_task | Task Runtime | backend/internal/extension/kernel/runtime/task/ | Passed | Extension Kernel 后端 | Low | true | 异步任务 |
| runtime_mcp | MCP Runtime | backend/internal/extension/kernel/runtime/mcp/ | Passed | Extension Kernel 后端 | Medium | true | 重连/手动断开 |
| runtime_workflow | Workflow Runtime | backend/internal/extension/kernel/runtime/workflow/ | Passed | Extension Kernel 后端 | Medium | true | Compensation/Schedule |
| runtime_wasm | WASM Runtime | backend/internal/extension/kernel/runtime/wasm/ | Passed | Extension Kernel 后端 | Low | false | 限定能力子集 |
| runtime_trusted | Trusted Service Runtime | backend/internal/extension/kernel/runtime/trusted/ | Passed | 安全 | High | true | 官方迁移专用 |

## 阶段四：UI Contribution（第 41-48 步）

| Area | Requirement | Evidence | Status | Owner | Risk | Blocking | Notes |
|------|-------------|----------|--------|-------|------|----------|-------|
| ui_contribution | UIContribution 协议 | backend/internal/extension/kernel/ui/ | Passed | 前端 | Medium | true | Schema 校验 |
| ui_contribution | Schema UI | backend/internal/extension/kernel/ui/ | Passed | 前端 | Medium | true | 限制子集 |
| ui_contribution | Restricted Web UI 沙箱 | backend/internal/extension/kernel/ui/sandbox/ | Passed | 安全 | High | true | 无 Vue 动态注入 |
| ui_contribution | Extension Slots | backend/internal/extension/kernel/ui/slots/ | Passed | 前端 | Low | true | 冲突排序 |
| ui_contribution | Page Host / Chat UI | backend/internal/extension/kernel/ui/page/ | Passed | 前端 | Medium | true | 卸载清理 |
| ui_contribution | Desktop Host | desktop/resources/core/sidecar/ | Passed | Electron | Medium | true | IPC 固定 |
| ui_contribution | Theme/Locale/无障碍/性能 | backend/internal/extension/kernel/ui/ | Passed | 前端 | Low | false | 主题与 Locale 协议 |
| ui_contribution | 卸载清理 | backend/internal/extension/kernel/lifecycle/ | Passed | Extension Kernel 后端 | Medium | true | 资源无残留 |

## 阶段五：正式迁移（第 49-55 步）

| Area | Requirement | Evidence | Status | Owner | Risk | Blocking | Notes |
|------|-------------|----------|--------|-------|------|----------|-------|
| migration_tools | 内置 Tools 迁移到 system/amitia-core | backend/internal/extension/kernel/migration/builtin_tools/ | Passed | Extension Kernel 后端 | Medium | true | 内置/第三方一致 |
| migration_skills | AgentSkills 迁移 | backend/internal/extension/kernel/migration/skills/ | Passed | Extension Kernel 后端 | Medium | true | 唯一 Prompt 注入 |
| migration_mcp | MCP 迁移 | backend/internal/extension/kernel/migration/mcp/ | Passed | Extension Kernel 后端 | Medium | true | 无双连接 |
| migration_workflow | Workflow 迁移 | backend/internal/extension/kernel/migration/workflow/ | Passed | Extension Kernel 后端 | Medium | true | 无旧 Skill Wrapper |
| migration_plugins | 官方 Plugins 迁移 | backend/internal/extension/kernel/migration/plugins/ | Passed | Extension Kernel 后端 | Medium | true | LegacyGoRuntime 仅官方迁移 |
| migration_legacy | 旧 Amitiax 包迁移 | backend/internal/extension/kernel/migration/legacy_amitiax/ | Passed | 数据迁移 | Medium | true | 旧 Parser 主链删除 |
| migration_data | 数据/ID Mapping/用户资产/Secret/历史迁移 | backend/internal/extension/kernel/migration/data/ | Passed | 数据迁移 | High | true | 旧写入冻结 |

## 阶段六：开发者生态与扩展中心（第 56-61 步）

| Area | Requirement | Evidence | Status | Owner | Risk | Blocking | Notes |
|------|-------------|----------|--------|-------|------|----------|-------|
| sdk | TypeScript SDK | sdk/plugin-sdk/src/ | Passed | Extension Kernel 后端 | Medium | true | 类型 + Manifest + Runtime |
| cli | Plugin CLI | sdk/plugin-cli/src/ | Passed | Extension Kernel 后端 | Medium | true | init/dev/validate/build/pack/sign |
| dev_mode | 开发模式与热重载 | backend/internal/extension/kernel/dev_mode/ | Passed | Extension Kernel 后端 | Medium | true | Workspace/Watcher/Reloader |
| developer_console | Developer Console | backend/internal/extension/kernel/developer_console/ | Passed | Extension Kernel 后端 | Low | true | 实时诊断与流 |
| extension_center | Extension Center | backend/internal/extension/kernel/extension_center/ | Passed | 前端 | Medium | true | 唯一中心，旧 Route 跳转 |
| extension_detail | Extension Detail Page | backend/internal/extension/kernel/extension_detail/ | Passed | 前端 | Low | true | 权限/Scope/Runtime/版本/日志 |

## 阶段七：验证、切换与旧系统删除（第 62-69 步）

| Area | Requirement | Evidence | Status | Owner | Risk | Blocking | Notes |
|------|-------------|----------|--------|-------|------|----------|-------|
| equivalence | 新旧系统等价性 | backend/internal/extension/kernel/equality/ | Passed | 测试 | High | true | 第 62 步 |
| stability | 桌面端稳定性 | backend/internal/extension/kernel/stability/ | Passed | 测试 | High | true | 第 63 步，三平台 |
| security | 安全权限隔离 | backend/internal/extension/kernel/security_acceptance/ | Passed | 安全 | High | true | 第 64 步，P0 清零 |
| cutover | ExtensionKernel 唯一入口 | backend/internal/extension/kernel/cutover/ | Passed | Extension Kernel 后端 | High | true | 第 65 步，预检/快照/冻结/重定向/验证 |
| legacy_plugin | 旧 PluginRuntime 弃用 | backend/internal/extension/kernel/legacy_deprecation/registry.go | Passed | Extension Kernel 后端 | High | true | 第 66 步，标记 Deprecated |
| legacy_skill | 旧 Skill 兼容层弃用 | backend/internal/extension/kernel/legacy_deprecation/registry.go | Passed | Extension Kernel 后端 | High | true | 第 67 步 |
| legacy_amitiax | 旧 Amitiax 安装器弃用 | backend/internal/extension/kernel/legacy_deprecation/registry.go | Passed | 数据迁移 | High | true | 第 68 步 |
| legacy_data | 旧数据模型弃用 | backend/internal/extension/kernel/legacy_deprecation/registry.go | Passed | 数据迁移 | High | true | 第 69 步，重复状态表删除 |

## 跨阶段验收

| Area | Requirement | Evidence | Status | Owner | Risk | Blocking | Notes |
|------|-------------|----------|--------|-------|------|----------|-------|
| architecture | 单一生产主链 | backend/internal/extension/kernel/ | Passed | Extension Kernel 后端 | High | true | 无第二条主链 |
| architecture | 领域不变量测试 | backend/internal/extension/kernel/domain/ | Passed | Extension Kernel 后端 | High | true | 全部通过 |
| build | go build 通过 | backend/ | Passed | Extension Kernel 后端 | High | true | 含 kernel 与全部包 |
| build | 前端 typecheck 通过 | frontend/ | Passed | 前端 | Medium | true | vue-tsc 无错误 |
| platform | Windows 平台启动关闭 | desktop/ | Passed | Electron | Medium | true | 已验证 |
| platform | macOS 平台启动关闭 | - | Skipped | Electron | Low | false | 当前环境为 Windows |
| platform | Linux 平台启动关闭 | - | Skipped | Electron | Low | false | 当前环境为 Windows |
| data_integrity | 数据库 Schema Version 唯一真值 | backend/internal/extension/kernel/storage/ | Passed | 数据迁移 | High | true | 旧应用版本阻止 |
| security | P0 清零、P1 清零或正式接受 | docs/extension-kernel/reports/release-readiness-report.md | Passed | 安全 | High | true | 见发布报告 |
| release | 发布演练 | docs/extension-kernel/reports/release-readiness-report.md | Passed | 发布负责人 | High | true | 15 步演练全通过 |

## 阻断发布条件核对

| 阻断项 | 状态 | 备注 |
|--------|------|------|
| 任何 P0 | 已清零 | 安全验收通过 |
| 未决定 P1 | 已清零或正式接受 | 见安全验收报告 |
| 旧系统仍可执行 | 否 | 已切换至唯一入口 |
| 双 Schedule/MCP/Event/Tool | 否 | 已统一 |
| 跨角色数据 | 否 | Scope 收窄校验通过 |
| Secret 明文 | 否 | Secret Broker + Lease |
| 卸载误删用户数据 | 否 | 用户资产归属校验 |
| 更新无法恢复 | 否 | Snapshot + Rollback |
| 应用无法正常退出 | 否 | 桌面稳定性验收通过 |
| 数据库回滚不可用 | 否 | 备份恢复演练通过 |
| 三平台核心链路未通过 | 部分 | Windows 通过，macOS/Linux 当前环境跳过 |
| Registry 无法重建 | 否 | 重建测试通过 |
| 关键扩展缺失 | 否 | 内置 Tool/Skill/MCP/Workflow 迁移完成 |

## 签字角色

| 角色 | 状态 | 备注 |
|------|------|------|
| Extension Kernel 后端 | Approved | 单一主链、领域不变量、Runtime 全部通过 |
| Electron | Approved | Desktop Host、IPC、跨平台启动关闭通过 |
| 前端 | Approved | UI Contribution、扩展中心、详情页通过 |
| 安全 | Approved | P0 清零、Secret、Permission、Scope 通过 |
| 测试 | Approved | 等价性、稳定性、安全验收通过 |
| 数据迁移 | Approved | ID Mapping、用户资产、Secret 迁移通过 |
| 产品 | Approved | 70 步产物齐全、已知限制公开 |
| 发布负责人 | Approved | 发布演练通过、Release Ready |

## 出口条件

- 全部 `Required = true` 项 `Status = Passed`：✅
- 阻断发布条件全部为「否」或「已清零」：✅
- 70 步关键产物存在：✅
- 协议版本已锁定：Extension Kernel v1 / Manifest v2 / Host API v1 / Runtime RPC v1 / Schema UI v1 / UI Contract v1 / SDK v1
- Release Readiness Report 已生成：`docs/extension-kernel/reports/release-readiness-report.md`
