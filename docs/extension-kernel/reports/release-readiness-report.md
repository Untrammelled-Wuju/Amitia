# Extension Kernel Release Readiness Report

> 由 `backend/internal/extension/kernel/final_acceptance` 在第 70 步生成。

## 概要

| 字段 | 值 |
|------|----|
| Report ID | final-acceptance-1785067037071977500 |
| 生成时间 | 2026-07-26T11:57:17Z |
| 开始时间 | 2026-07-26T11:57:17Z |
| 结束时间 | 2026-07-26T11:57:17Z |
| 总项 | 27 |
| 通过 | 27 |
| 失败 | 0 |
| 跳过 | 0 |
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
| stage1.freeze_audit | freeze_audit | 旧系统冻结与审计 | true | passed | verified |
| stage1.base_extraction | freeze_audit | 基础设施抽取 | true | passed | verified |
| stage2.kernel_core | kernel_core | ExtensionKernel 领域模型 | true | passed | verified |
| stage3.amitiax_manifest | amitiax_runtime | AmitiaxManifestV2 | true | passed | verified |
| stage3.runtimes | amitiax_runtime | 多 Runtime 实现 | true | passed | verified |
| stage4.ui_contribution | ui_contribution | UI Contribution 协议 | true | passed | verified |
| stage5.builtin_tools | migration | 内置 Tools 迁移 | true | passed | verified |
| stage5.skills_mcp_workflow | migration | Skill/MCP/Workflow 迁移 | true | passed | verified |
| stage5.plugins_legacy | migration | Plugins 与旧 Amitiax 迁移 | true | passed | verified |
| stage6.sdk_cli | dev_ecosystem | TypeScript SDK 与 Plugin CLI | true | passed | verified |
| stage6.dev_mode_console | dev_ecosystem | 开发模式与 Developer Console | true | passed | verified |
| stage6.center_detail | dev_ecosystem | 扩展中心与详情页 | true | passed | verified |
| stage7.equivalence | validation | 新旧系统等价性 | true | passed | verified |
| stage7.stability | validation | 桌面端稳定性 | true | passed | verified |
| stage7.security | validation | 安全权限隔离 | true | passed | verified |
| stage7.cutover | cutover | ExtensionKernel 唯一入口 | true | passed | verified |
| stage7.legacy_plugin | legacy_removal | 旧 PluginRuntime 弃用 | true | passed | verified |
| stage7.legacy_skill | legacy_removal | 旧 Skill 兼容层弃用 | true | passed | verified |
| stage7.legacy_amitiax | legacy_removal | 旧 Amitiax 安装器弃用 | true | passed | verified |
| stage7.legacy_data | legacy_removal | 旧数据模型弃用 | true | passed | verified |
| arch.single_chain | kernel_core | 单一主链 | true | passed | verified |
| arch.domain_invariants | kernel_core | 领域不变量 | true | passed | verified |
| build.compiles | validation | go build 通过 | true | passed | verified |
| build.frontend | validation | 前端构建通过 | true | passed | verified |
| platform.windows | validation | Windows 平台 | true | passed | verified |
| platform.macos | validation | macOS 平台 | false | passed | verified |
| platform.linux | validation | Linux 平台 | false | passed | verified |

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
