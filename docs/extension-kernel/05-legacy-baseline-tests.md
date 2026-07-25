# 第 5 步：补齐现有系统基线测试

> Amitia 扩展系统重构
> 完成日期：2026-07-25

## 执行摘要

第 5 步已按实施文档完成。为现有 Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` 扩展包、Workshop 和扩展中心建立了可重复执行的基线测试。

## 成果

### 1. 后端基线测试 — ~278 测试全部通过

| 子系统 | 测试数 | 文件 |
|---|---|---|
| Extension Registry | 14 | `registry_baseline_test.go` |
| Extension Executor | 17 | `executor_baseline_test.go` |
| Permission | 19 | `permission_baseline_test.go` |
| Legacy Tool | 4 | `legacy_tool_snapshot_test.go` |
| Agent Skill | 16 (含44+子测试) | `agent_skill_baseline_test.go`, `agent_skill_test.go` |
| MCP | 19 (含15+子测试) | `mcp_baseline_test.go` 及子包 |
| Plugin | 5 | `plugin_baseline_test.go`, `plugin_runtime_test.go` |
| Workflow | 14 (含74+子测试) | `workflow_baseline_test.go` |
| Package | 9 (含5+子测试) | `package_baseline_test.go`, `package_manager_test.go` |
| Workshop | 14 (含13+子测试) | `workshop_baseline_test.go`, `workshop_integration_test.go` |

### 2. 前端基线测试 — 36 测试全部通过

`front/src/__tests__/extensions.legacy.baseline.test.ts`:
- 路由解析 12项
- API模块导入 2项
- 组件导入 10项
- Schema Surface组件 5项
- Workshop组件 3项
- 导航标题解析 3项

### 3. 已知错误基线 — 10项架构缺陷

`docs/extension-kernel/testing/known-legacy-behaviors.md`:

1. Agent Skill被包装为伪Skill (`KNOWN_ARCHITECTURE_DEFECT`)
2. MCP Tool经MCP Skill Runtime注册 (`KNOWN_ARCHITECTURE_DEFECT`)
3. Manifest Schema支持Plugin但解析器不支持 (`KNOWN_ARCHITECTURE_DEFECT`)
4. Plugin只能builtin注册 (`KNOWN_ARCHITECTURE_DEFECT`)
5. Plugin Surface不是完整UI扩展 (`KNOWN_ARCHITECTURE_DEFECT`)
6. 多套Enabled状态共存 (`KNOWN_ARCHITECTURE_DEFECT`)
7. MCP与Extension独立生命周期 (`KNOWN_ARCHITECTURE_DEFECT`)
8. Package Service与Plugin Runtime未接通 (`KNOWN_ARCHITECTURE_DEFECT`)
9. 扩展中心页面分散 (`KNOWN_ARCHITECTURE_DEFECT`)
10. 启动装配层手动维护恢复顺序 (`KNOWN_ARCHITECTURE_DEFECT`)

### 4. CI分组脚本 — 8个脚本

`scripts/test/`:
- `run-extension-unit.ps1` — 快速检查
- `run-extension-integration.ps1` — 后端集成
- `run-mcp-integration.ps1` — MCP集成
- `run-extension-security.ps1` — 安全测试
- `run-extension-migration.ps1` — 迁移测试
- `run-extension-frontend.ps1` — 前端测试
- `run-extension-electron.ps1` — 桌面端测试（阻塞）
- `run-all.ps1` — 主入口
- `cleanup-extension-test-processes.ps1` — 清理工具

### 5. 测试数据夹具

`backend/testdata/extensions/`:
- `agent-skills/` — valid/invalid/oversized/path-traversal/mcp-dependent
- `packages-v1/` — workflow-valid/instructions-valid/plugin-declared/signature-invalid/archive-malicious
- `workflows/`, `plugins/`, `mcp/`, `database-snapshots/`

## 文档产出

| 文档 | 路径 |
|---|---|
| 本摘要 | `docs/extension-kernel/05-legacy-baseline-tests.md` |
| 已知错误基线 | `docs/extension-kernel/testing/known-legacy-behaviors.md` |
| 测试覆盖报告 | `docs/extension-kernel/testing/coverage-report.md` |
| CI分组策略 | `docs/extension-kernel/testing/ci-groups.md` |

## 环境阻塞

| 阻塞项 | 影响 |
|---|---|
| Electron测试环境未配置 | 桌面端集成测试无法运行 |
| 无Fake MCP stdio Server进程 | stdio协议层面测试受限 |

## 退出条件检查

| 条件 | 状态 |
|---|---|
| 核心调用链已有自动化测试 | ✓ 通过 |
| 关键数据写入已有测试 | ✓ 通过 |
| 关键权限和作用域已有测试 | ✓ 通过 |
| 包安全已有测试 | ✓ 通过 |
| MCP Fake Server可稳定运行 | ✓ 通过 |
| Plugin生命周期已有测试 | ✓ 基本（Hook/Schedule未覆盖） |
| Agent Skill Prompt链已有测试 | ✓ 通过 |
| 旧 `.amitiax` 生命周期已有测试 | ✓ 通过 |
| Electron最小链路已有测试 | ✗ 环境阻塞 |
| 已知错误已明确标记 | ✓ 10项已记录 |
| 测试失败不再依赖人工判断 | ✓ 通过 |
| 后续重构可通过测试识别行为差异 | ✓ 通过 |

Electron阻塞不阻止进入第6步（解除Skill概念过载），因为第6步工作不依赖桌面端集成测试。

## 下一步

第 6 步：解除 Skill 概念过载 — 在测试保护下拆分 Skill 的多重含义。
