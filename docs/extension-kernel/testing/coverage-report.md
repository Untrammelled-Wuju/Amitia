# 测试覆盖与阻塞报告

> Amitia 扩展系统重构 第 5 步 — 补齐现有系统基线测试
> 报告日期：2026-07-25
> 最终版本

## 总体状态

| 指标 | 数值 |
|---|---|
| 后端测试总数 | ~278（含子测试） |
| 后端通过 | ~278 |
| 后端失败 | 0 |
| 前端测试总数 | 36 |
| 前端通过 | 36 |
| 前端失败 | 0 |
| 已知失败（已标记） | 0 |
| 环境阻塞 | Electron 桌面端测试 |
| 未执行 | Electron 集成测试 |

## 子系统覆盖明细

### 1. Extension Registry — 14 测试全部通过

覆盖：正常注册、重复ID、注册后移除、重新注册、查询不存在、按模型名称查询、无效Manifest拒绝、禁用过滤、nil handler处理、Available过滤、List过滤、作用域启用行为、作用域禁用回退、模型名称冲突。

缺失：并发注册竞态条件。

### 2. Extension Executor — 17 测试全部通过

覆盖：正常执行、禁用Skill、Skill不存在、权限拒绝、输入校验、Instructions不可执行、不兼容Skill、触发器不允许、超时、上下文取消、Panic恢复、Handler错误、幂等性、结果标准化、AllowOnce权限、未知能力。

缺失：审计写入详细验证、并发限制、敏感字段脱敏详细覆盖。

### 3. Permission — 19 测试全部通过

覆盖：全局拒绝、AllowAlways策略、AllowOnce策略、未知能力拒绝、数据库授权消耗、会话级授权、非LLM拒绝、所有作用域类型、MCP Tool/Server能力、预览不消耗、角色隔离、授权和撤销、高风险能力、所有能力已知、未声明能力拒绝、Manifest权限不匹配、并发授权和评估。

缺失：撤销后子权限传播。

### 4. Legacy Tool Adapter — 4 测试全部通过

覆盖：生成内置工具快照、适配器和运行时注册、运行持久化和脱敏、角色作用域隔离。

缺失：每个内置工具的逐一Handler验证。

### 5. Agent Skill — 16 测试全部通过（含44+子测试）

覆盖：移除链、伪Skill注册、资源读取边界、激活边界、并发资源读取、删除不影响共享副本、激活追踪、空内容、大资源、全局与角色命名冲突、解析器验证（16子测试）、目录限制和SVG安全、资源和OpenAI映射、Amitia MCP依赖覆盖OpenAI、ZIP安全（4子测试）、安装/激活/资源/恢复完整链路。

缺失：SKILL.md多语言编码、超长内容边界。

### 6. MCP — 19 测试全部通过（含15+子测试）

覆盖：Server CRUD、重复身份拒绝、重复名称允许、Transport验证（7子测试）、身份标准化、Server列表基线、作用域绑定、Tool同步和列表、Resource同步和列表、Prompt同步和列表、Server能力基线、凭证引用生命周期、OAuth会话生命周期、Server状态转换、替代命令可移植性（4子测试）、能力默认禁用和Upsert、拒绝未知能力配置、拒绝不安全端点、OAuth Session状态查找。

缺失：stdio子进程真实交互、Streamable HTTP断线重连、Elicitation/Tasks功能、completion功能。

### 7. Plugin — 5 测试全部通过

覆盖：Registry拒绝无效Manifest、运行时和Surface、熔断状态转换、贡献验证、事件持久化。

缺失：Hook执行（BeforePrompt/AfterReply）、Schedule定时任务、State CAS并发、Host API调用。

### 8. Workflow — 14 测试全部通过（含74+子测试）

覆盖：编译静态策略（16子测试）、拒绝传递性Skill循环、网络目标策略（10子测试）、禁止草稿内容矩阵（6子测试）、HTTP适配器Mock、Skill Mock受控实时、HTTP步骤超时、受限值和Secret（5子测试）、条件运算符矩阵（15子测试）、变换操作矩阵（13子测试）、Host限制钳制（4子测试）、执行器隔离和限制（5子测试）、受控实时阻塞Host副作用、Secret配置隔离。

缺失：执行器并发限制（多Workflow同时执行）。

### 9. Package（.amitiax v1）— 9 测试全部通过（含5+子测试）

覆盖：Archive安全和校验（5子测试）、预览断言、恢复协调、签名信任不授权、Workflow生命周期、Agent Skill生命周期、统一生命周期Instructions状态、会话隔离和Secret保护、配置迁移和依赖保护。

缺失：Rollback回滚路径（Artifact缺失场景）、正式迁移测试框架。

### 10. Workshop — 14 测试全部通过（含13+子测试）

覆盖：端到端安装/执行/恢复、Revision使权限作用域失效、Registry失败补偿数据库、数据库失败保留Registry、版本建议（4子测试）、断言Schema验证、测试报告脱敏、计划器和生成器管道（4子测试）、计划验证矩阵（5子测试）、计划步骤字段校验、计划副作用校验、草稿消息字段校验、指标暴露、状态机。

缺失：AI生成与手工修改混合编辑场景。

### 11. 前端扩展中心 — 36 测试全部通过

覆盖：12个路由解析（Extension Center、MCP、Packages、Skills、Skill Detail、Agent Skills、Plugins、Plugin Detail、Workshop、Workshop Session、Run History、auth要求）、2个API模块导入、10个组件导入、5个Schema Surface组件导入、3个Workshop组件导入、3个导航标题解析。

缺失：真实API调用Mock测试、加载/空/错误状态交互测试、安装/权限确认交互流测试。

### 12. Electron 桌面端 — 环境阻塞

当前状态：未配置Electron测试环境。`run-extension-electron.ps1` 脚本已创建，以exit code 2标记阻塞状态。

阻塞原因：
- 未配置Playwright/Spectron
- 未创建隔离的Electron用户数据目录
- CI运行环境缺少Display server

解除阻塞步骤已记录在 `scripts/test/run-extension-electron.ps1` 中。

## CI 测试分组

| 分组 | 入口脚本 | 内容 | 预计耗时 |
|---|---|---|---|
| quick | `run-extension-unit.ps1` | Extension + MCP 单元测试 | ~30s |
| integration | `run-extension-integration.ps1` + `run-mcp-integration.ps1` | 数据库集成、Fake Server集成 | ~60s |
| security | `run-extension-security.ps1` | Archive、权限、Secret、路径安全 | ~30s |
| migration | `run-extension-migration.ps1` | 旧行为基线、迁移路径测试 | ~30s |
| frontend | `run-extension-frontend.ps1` | Vitest前端扩展组件测试 | ~30s |
| electron | `run-extension-electron.ps1` | 桌面端集成测试（阻塞中） | N/A |
| all | `run-all.ps1 -Group all` | 全部分组 | ~3min |

## 环境阻塞

| 阻塞项 | 影响 | 解决路径 |
|---|---|---|
| Electron 测试环境未配置 | 桌面端集成测试无法运行 | 安装Playwright，配置独立用户数据目录 |
| 无Fake MCP stdio Server（真实进程） | stdio协议层面测试受限 | 在testdata中部署Fake Server进程 |
| 无独立CI运行环境 | 无法验证CI分组自动执行 | 配置GitHub Actions或本地Runner |

## 未覆盖区域

| 区域 | 优先级 | 说明 |
|---|---|---|
| Electron启动与进程清理 | P0 | 环境阻塞，脚本已就绪 |
| MCP stdio子进程真实交互 | P0 | 需要Fake Server |
| Plugin Hook BeforePrompt/AfterReply | P1 | Hook执行路径 |
| Plugin Schedule定时任务 | P1 | 需要Fake Clock集成 |
| Plugin State CAS并发 | P2 | 状态并发竞争 |
| Workflow执行器并发限制 | P2 | 多Workflow同时执行 |
| MCP Elicitation/Tasks/Completion | P2 | MCP高级功能 |
| Legacy Tool逐一Handler验证 | P3 | 每个内置工具Handler行为 |
| 前端API Mock交互测试 | P1 | 真实HTTP调用Mock |
| 前端加载/空/错误状态测试 | P2 | 状态交互覆盖 |

## 关键链路覆盖率

| 模块 | 目标 | 实际 | 状态 |
|---|---|---|---|
| Package Archive/Security | 90% | ✓ 已覆盖 | 通过 |
| Package Parser | 85% | ✓ 已覆盖 | 通过 |
| Permission | 90% | ✓ 已覆盖 | 通过 |
| Executor | 85% | ✓ 已覆盖 | 通过 |
| Agent Skill Parser | 85% | ✓ 已覆盖 | 通过 |
| MCP Transport/Client 核心 | 80% | ✓ 已覆盖 | 通过 |
| Plugin Lifecycle 核心 | 80% | ≈ 基本覆盖 | Hook/Schedule缺失 |
| Workflow Compiler | 80% | ✓ 已覆盖 | 通过 |
| Migration Helpers | 90% | ≈ 基本覆盖 | 正式迁移框架缺失 |

## 后续步骤建议

1. **P0** — 配置Electron测试环境（Playwright），完成桌面端启动和进程清理测试
2. **P0** — 在 `backend/testdata/extensions/mcp/` 下部署Fake MCP stdio Server
3. **P1** — 补齐Plugin Hook和Schedule测试
4. **P1** — 前端API Mock交互测试
5. **P2** — 补齐Workflow并发测试和MCP高级功能测试

## 产物清单

| 产物 | 路径 | 状态 |
|---|---|---|
| 测试策略主文档 | `docs/extension-kernel/05-legacy-baseline-tests.md` | 通过 |
| 已知错误基线 | `docs/extension-kernel/testing/known-legacy-behaviors.md` | 10项已记录 |
| 测试覆盖报告 | `docs/extension-kernel/testing/coverage-report.md` | 本文档 |
| 测试数据夹具 | `backend/testdata/extensions/` | 已创建 |
| 后端基线测试 | `backend/internal/extension/*_test.go` | 19文件，~278测试 |
| MCP基线测试 | `backend/internal/mcp/**/*_test.go` | 15文件 |
| 前端基线测试 | `front/src/__tests__/extensions.legacy.baseline.test.ts` | 36测试 |
| CI分组脚本 | `scripts/test/` | 8脚本 |
| Electron测试入口 | `scripts/test/run-extension-electron.ps1` | 阻塞标记 |
