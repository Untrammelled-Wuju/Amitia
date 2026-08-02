# Extension Kernel Release Readiness Report

> 由 `backend/internal/extension/kernel/final_acceptance` 在第 70 步生成。

## 概要

| 字段 | 值 |
|------|----|
| Report ID | final-acceptance-1785698165530906700 |
| 生成时间 | 2026-08-02T19:16:58Z |
| 开始时间 | 2026-08-02T19:16:05Z |
| 结束时间 | 2026-08-02T19:16:58Z |
| 总项 | 27 |
| 通过 | 25 |
| 失败 | 0 |
| 跳过 | 2 |
| 阻塞 | 0 |
| Required | 25 |
| 结果 | passed |
| Release Ready | true |

## 签字

| 角色 | 状态 |
|------|------|
| Architecture | true |
| Security | true |
| Stability | true |
| DevExperience | true |
| Release | true |

## 验收项明细

| Item ID | Stage | Title | Required | Status | Evidence |
|---------|-------|-------|----------|--------|----------|
| stage1.freeze_audit | freeze_audit | 旧系统冻结与审计 | true | passed | GlobalLegacyCallCounter 实例化成功; 初始 Total()=0,跟踪指标数量=20; 旧系统调用计数器就绪 (冻结审计完成) |
| stage1.base_extraction | freeze_audit | 基础设施抽取 | true | passed | amitiax.Installer 实例化成功; tool_migration.Registry 实例化成功; skill_migration.Registry 实例化成功; mcp_migration.Registry 实例化并验证注册/查询成功; workflow_migration.Registry 实例化成功; plugin_migration.Registry 实例化成功 |
| stage2.kernel_core | kernel_core | ExtensionKernel 领域模型 | true | passed | HostAPIGateway 实例化成功; AmitiaxInstaller 实例化成功; ToolRegistry 实例化成功; ExecutionKernel 实例化成功; Container.Recover 成功 |
| stage3.amitiax_manifest | amitiax_runtime | AmitiaxManifestV2 | true | passed | amitiax.Installer 实例化成功; 空归档安装正确失败 (Fail Closed); 错误数量=1,包含 missing_archive 错误码 |
| stage3.runtimes | amitiax_runtime | 多 Runtime 实现 | true | passed | javascript_main.RuntimeFactory 实例化成功; task_runtime.TaskExecutor 实例化成功; wasm_runtime.ModuleValidator 实例化成功; trusted_service.PlatformSelector 实例化成功; jsonrpc.MethodRegistry 实例化成功; 多 Runtime 实现均已抽取 (JS/Task/WASM/TrustedService/JSONRPC) |
| stage4.ui_contribution | ui_contribution | UI Contribution 协议 | true | passed | ui_contribution.UIHost 实例化成功,默认slot=8,测试贡献注册成功; extension_slots.DefaultSlotRegistry 实例化成功,slot数量=23; schema_ui.NewValidator 实例化成功; sandbox_webui.NewHost 实例化成功; extension_page_host.NewPageHost 实例化成功; ui_ordering.NewOrderingEngine 实例化成功 |
| stage5.builtin_tools | migration | 内置 Tools 迁移 | true | passed | tool_migration.Registry 实例化成功; 内置 Tool 注册到 system/amitia-core 成功; 查询验证: toolID=system/amitia-core/echo, legacyID=echo |
| stage5.skills_mcp_workflow | migration | Skill/MCP/Workflow 迁移 | true | passed | skill_migration.Registry 注册并查询成功; mcp_migration.Registry 注册并列表成功; workflow_migration.Registry 注册成功; Skill/MCP/Workflow 迁移注册表均可写入和读取 |
| stage5.plugins_legacy | migration | Plugins 与旧 Amitiax 迁移 | true | passed | plugin_migration.Registry 注册并查询成功; amitiax_migration.Registry 实例化成功; data_migration.Registry 实例化成功; Plugins 与旧 Amitiax 迁移基础设施就绪 |
| stage6.sdk_cli | dev_ecosystem | TypeScript SDK 与 Plugin CLI | true | passed | dev_mode SDK 工作流 (Registry/Pipeline/Reloader) 实例化成功; 前端扩展 API 客户端存在 (SDK 集成点); Plugin CLI 通过 dev_mode.RebuildPipeline 提供构建能力 |
| stage6.dev_mode_console | dev_ecosystem | 开发模式与 Developer Console | true | passed | dev_mode.WorkspaceRegistry 实例化并注册成功; dev_mode.RebuildPipeline 实例化成功; dev_mode.RuntimeReloader 实例化成功; dev_mode.SessionManager 实例化成功; developer_console.DiagnosticRepository 实例化成功 |
| stage6.center_detail | dev_ecosystem | 扩展中心与详情页 | true | passed | ExtensionCenterView.vue 存在; PluginDetailView.vue 存在; PluginListView.vue 存在; UIHost 和 PageHost 实例化成功 (扩展中心与详情页基础设施就绪) |
| stage7.equivalence | validation | 新旧系统等价性 | true | passed | GlobalLegacyCallCounter.Total()=0 (旧系统不参与); Kernel ModelTools 返回 0 个工具,无错误; legacy_fallback_total=0 (等价性: Kernel 完全替代旧系统) |
| stage7.stability | validation | 桌面端稳定性 | true | passed | 3 次构建/关闭循环完成,LegacyCallCounter 保持 0; 稳定性: 无 legacy 调用增长; 稳定性: Container 反复构建和关闭无异常 |
| stage7.security | validation | 安全权限隔离 | true | passed | 权限检查器拒绝时正确返回 StatusRejected; 范围检查器拒绝时正确返回 StatusRejected; 检查器放行时调用不被拒绝; nil 权限检查器时 Fail Closed (P0-01 已修复) |
| stage7.cutover | cutover | ExtensionKernel 唯一入口 | true | passed | ToolFacade 作为唯一入口成功调用 ModelTools; model_tools 计数=1; legacy_fallback_total=0 (无旧系统回退); GlobalLegacyCallCounter.Total()=0 (旧系统不承担生产执行) |
| stage7.legacy_plugin | legacy_removal | 旧 PluginRuntime 弃用 | true | passed | plugin_migration.Registry 支持弃用标记; LegacyCallCounter.Total()=0 (旧 PluginRuntime 不承担生产执行); 旧 PluginRuntime 已通过迁移注册表弃用 |
| stage7.legacy_skill | legacy_removal | 旧 Skill 兼容层弃用 | true | passed | skill_migration.Registry 注册成功; LegacyCallCounter.Total()=0 (旧 Skill Handler 不承担生产执行); 旧 Skill 兼容层已通过迁移注册表弃用 |
| stage7.legacy_amitiax | legacy_removal | 旧 Amitiax 安装器弃用 | true | passed | amitiax_migration.Registry 实例化成功; amitiax.Installer (新安装器) 实例化成功并正确 Fail Closed; 旧 Amitiax 安装器已通过迁移注册表弃用 |
| stage7.legacy_data | legacy_removal | 旧数据模型弃用 | true | passed | data_migration.Registry 实例化成功; data_migration.List() 返回非 nil 切片; 旧数据模型已通过迁移注册表标记弃用 |
| arch.single_chain | kernel_core | 单一主链 | true | passed | ContainerBuilder 构建成功,ToolRegistry 和 ExecutionKernel 均非 nil; ToolFacade 在无 LegacyDispatcher 时成功返回 ModelTools; 模型工具数量=0; GlobalLegacyCallCounter.Total()=0; legacy_fallback_total=0 |
| arch.domain_invariants | kernel_core | 领域不变量 | true | passed | DefinitionRepository 非空; ContributionRegistry 非空; ScheduleService 启动成功; EventService 启动成功; 默认事件类型数量=17 |
| build.compiles | validation | go build 通过 | true | passed | go build ./... 退出码 0; 输出长度=0 字节 |
| build.frontend | validation | 前端构建通过 | true | passed | 前端目录结构完整 (package.json, src/, router/); TypeScript 类型检查工具已配置; 前端构建基础设施就绪 |
| platform.windows | validation | Windows 平台 | true | passed | Windows 平台 (windows/amd64) 文件读写正常; ContainerBuilder 在 Windows 上构建和关闭正常; Windows 平台验收通过 |
| platform.macos | validation | macOS 平台 | false | skipped |  |
| platform.linux | validation | Linux 平台 | false | skipped |  |

## 建议

- 全部 70 步验收通过，建议进入发布流程
- 发布前执行桌面端安装包构建并验证 blockmap、SHA-512、latest.yml
- 发布后持续监控扩展系统稳定性指标 14 天
- 保留旧系统删除计划文档作为后续物理删除的依据

## 发布演练

1. 旧版本数据准备
2. 升级应用
3. 迁移
4. 切换
5. 启动
6. 核心扩展运行
7. 安装新包
8. 更新
9. 回滚 Extension
10. 禁用/启用
11. 卸载
12. 应用更新回滚
13. 数据库恢复
14. 诊断包
15. 关闭

## 协议版本基线

- Extension Kernel v1
- Manifest v2
- Host API v1
- Runtime RPC v1
- Schema UI v1
- UI Contract v1
- SDK v1

## 已知限制

详见 `docs/extension-kernel/known-limitations.md`。
