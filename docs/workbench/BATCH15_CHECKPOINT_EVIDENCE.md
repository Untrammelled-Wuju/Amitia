# Batch 15: Coding Worker 检查点、真实测试证据与人工审批

## 目标及实现范围

第 14 轮父 Agent 可以读取 Git 审查摘要，却不能证明实际执行过测试，也没有针对确定提交的持久化人工批准。第 15 轮改为完整的后端控制链：只有 U-Ai 创建且绑定了 TaskID/ConversationID 的本地 Git Worktree 可以创建检查点；提交 HEAD、基线提交及任务身份共同形成固定 Revision。

只有工作树干净、基线是当前提交祖先、任务绑定仍匹配且 Git 审查完整，才能创建检查点。检查点文件以限制大小和私有权限写入应用数据目录，不写入受执行者控制的 Git 工作树中。

## 四类 HTTP API

- `POST /api/workspaces/git/worktrees/checkpoints/prepare`：校验并创建或者幂等恢复检查点。
- `POST /api/workspaces/git/worktrees/checkpoints/read`：读取当前仍有效的检查点。提交变化或未提交文件出现即返回冲突。
- `POST /api/workspaces/git/worktrees/checkpoints/test`：仅允许本机受信 UI 会话主动请求执行固定测试 Profile，最长期限 90 秒。Go 使用 `go test ./...`；Go race 使用 `go test -race ./...`。关闭工具链在线下载；不使用 shell 拼接参数。
- `POST /api/workspaces/git/worktrees/checkpoints/approve`：仅允许本机受信 UI 会话审批；审批身份由服务端认证上下文生成，客户端不能指定；要求最后一次测试真实通过且 Revision 完全一致。

请求必须包含 workspaceUri、id、taskId、conversationId。Test/Approve 另需 revision，Test 另需 profile。读/写操作均有原有 Workspace 授权门禁。所有失败会返回明确的 HTTP 错误，禁止转化为假成功。

## 工作区和 Agent 联动

Web/Electron Git 工作区、Flutter Git 页均可查看 Diff、建立检查点、主动执行测试以及明确批准。父 Agent `harness_multi_agent_status/wait` 返回编码后的 `checkpointRevision`、`testState`、`testProfile`、`testExitCode`、`approvalState`；只返回状态摘要，不把测试原始日志推入 Agent 上下文。审查摘要为 ready 仅说明 Git Diff 完整，测试和批准状态必须另外判断。

## 安全、并发和回收

任务绑定仍在、没有脏文件时才能写入验收证据。测试执行前后再次验证 Revision 与任务归属。并发测试写入由全局锁保护，发布结果之前重新检查。测试日志摘要最多保留尾部 8 KiB，另保存完整输出的 SHA-256 摘要供内部审计。每个检查点最多保留 6 个测试记录。工作树解绑、删除时清理对应检查点。

本轮没有实现自动合并、原子多服务事务或者加密签名的远程审计证据。Git 文件可能包含危险测试代码；仅可信本机操作者能够显式选择运行，且 Go 测试在与 Runtime 相同的操作系统用户权限下执行，**不是沙盒**。真实多租户执行需要独立容器/进程沙盒、低权限账户、出网控制和资源限制，不应直接向云端远程开放运行接口。

## 验收与剩余风险

- 运行 `GO111MODULE=off go test -race -count=1` 对完整 `backend/internal/gitworktree` 进行真实仓库测试。
- 使用 Go 解析器、gofmt、TypeScript 转译器和 JSON Schema 验证改动。
- 由于 Go 项目完整版本需求与当前 SDK 不一致，HTTP 集成与 Flutter Release 未能运行。
- Flutter 的 90 秒 HTTP 请求是否能完整传播及系统后台切换中执行的续传，还需真机/云端联调。
- 本地测试执行没有独立的 OS 沙盒；不能标记为生产级远程任意代码执行安全功能。
- 后续应实现批准后安全合并（目标分支独立干净、CAS 校验、快进条件、审核记录、可回滚恢复策略）和构建产物证明。
