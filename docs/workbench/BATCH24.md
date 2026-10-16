# Workbench 第 24 轮：任务终端运行凭据

本轮围绕第 23 轮终端内存事件补充有界持久化生命周期凭据。凭据不包含用户输入、命令、环境变量、完整终端输出或测试结果；仅含会话、任务、对话、工作区、平台模式、时间、退出码和事件序号。**终端成功退出不等于 Coding Agent 完成，也不等于构建测试通过。**

## 模块和接口

- `task_receipt_store.go`：受权限约束的磁盘 JSON 凭据存储；以随机会话 ID 命名，原子替换，保留最多 256 条、每条最多 7 天；无原始终端输出。写入凭据存储失败时终端仍可运行，但不能据此承诺凭据持久化。
- `task_receipt_runtime.go`：会话创建、真实退出、撤销、主动关闭及闲置回收的凭据关联，终态不可回退。
- `task_receipt_handler.go`：`GET /api/workbench/terminal/task-receipts?conversationId=...&taskId=...`，仅可信本机 UI；必须同时匹配会话主体、对话、任务和当前 Worktree 绑定。返回最多 32 条，不返回任何终端输出。
- `terminal-task-receipts-v1.schema.json`：明确状态 `running/exited/closed/revoked/interrupted` 与证据级别 `terminal-metadata-only-not-test-evidence`。
- Web/Electron 增加按任务查看历史凭据；Flutter 接入任务 ID 的标签事件、持久化与只读展示。

## 生命周期边界

服务重启后，已写入 `running` 但不在当前 Runtime 内存中的会话会读取为 `interrupted`，并不代表进程仍在运行；只有实际收到进程退出回调，才记录真实退出码。管理员无法凭借其他对话或不同本机 UI 认证会话查询凭据。任务 Worktree 解除关联后，历史凭据不再通过 API 返回。因为认证会话标识参与主体隔离，用户重新登录、更换认证会话时，旧凭据可能不再可见；该机制不是用户级永久审计账本。共享目录仅允许当前运行账户访问，仍需宿主文件系统权限正确配置。

磁盘位置通过宿主 `os.UserConfigDir()` 下的 `amitia/workbench/terminal-task-receipts` 管理，目录权限要求为 0700，文件为 0600；有条件时原子替换。多进程间不共享写锁，尚不构成多实例一致性方案。服务无法创建安全目录时，凭据端点应明确提示不可用，而不是虚报已经保存。

## 已做检查与未验收

Go 核心最小依赖环境可在 Go 1.23.2 下执行 race 回归，包含文件落盘/重启中断、主体隔离、并发、目录与符号链接保护、终态不可回退、数量上限与关联终端结束/撤销。Go AST、`gofmt`、Web TS/Vue 转译、协议 Schema 正反例也完成定向检查。

**未验收**：项目要求 Go 1.26.1 的完整后端编译、Gin HTTP 实际路由、Flutter analyze/Release、Windows ConPTY 实机、多设备真实联调。没有将本轮成果计为生产构建通过，也没有对 Coding Agent 的运行调度器增加自动执行命令的权限。
