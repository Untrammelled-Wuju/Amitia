# MCP 单轨生产切换审计与验证

日期：2026-10-21

## 原问题

- 电脑端与手机端只使用 `/api/mcp` 旧入口。
- `.amitiax` 的 `mcp_server` 包只写入临时内存 Map，不连接 MCP Server，不执行工具发现，不注册 Agent 工具，重启后丢失。
- 新 MCP 运行时已有 Install、Start、Connect、Discover、OAuth、ToolRegistry 同步等能力，但包贡献安装链路未复用。

## 修复结果

- 原 `backend/internal/mcp` 整体迁入 `backend/internal/extension/mcpruntime`。
- 原 `backend/internal/mcpapi` 迁入 `backend/internal/extension/mcpruntime/api`。
- 删除 `MCPCompatibilityRuntime`、全局内存 MCP Server Map、旧包贡献适配器和相关迁移注释。
- 新增 Kernel `MCPBindingService`，手工 MCP、能力市场和 `.amitiax` 包共享同一持久化与运行生命周期。
- `.amitiax mcp_server` 使用正式 Manifest v1 的 transport、env、secretRef、auth、capability、private-network 和 placement 配置。
- 包安装、启用、停用、卸载和重启恢复统一处理连接、发现、Agent 工具注册与清理。
- 包拥有的 MCP 在电脑端和手机端只允许运行期操作；修改、删除必须通过扩展包更新或卸载。
- local、cloud-core、device-agent 按模块 placement 决定实际运行宿主。

## 验证

- `go test ./internal/extension/mcpruntime/...`
- `go test ./internal/extension/kernel -run TestMCP`
- `go test ./internal/migration -run MCP`
- `go test ./cmd/server -run '^$'`
- 前端 `pnpm --dir front typecheck`
- 前端 `pnpm --dir front build`
- 手机端目标文件 `flutter analyze --no-fatal-infos`

真实生产路径模拟使用正式 `.amitiax` 包完成：

- 包预览、确认、安装；
- 包启用；
- STDIO 实际连接；
- 工具发现；
- Agent 工具目录出现 `source=mcp` 的 Echo 工具；
- 完整项目重启后自动恢复连接、启用状态和 Agent 工具；
- 包停用后连接断开且 Agent 工具目录归零；
- 包卸载后 MCP 绑定、Agent 工具和扩展安装记录全部清除。
