# U-Ai Workbench 第 33 轮：任务进程清理、跨进程分配和认证验收

本轮继续使用 Batch32 完整源码为输入，保留此前所有累计变更。研发范围估算 83%，与完整构建通过率、生产验收通过率不同。

## 源码变更

- `backend/internal/gitworktree/checkpoint.go`：检查点 Go 测试改成显式 Start/Wait，在任务取消时清理子进程组，失败或超时不得留下伪造成功的测试证据。
- `backend/internal/gitworktree/checkpoint_process_{linux,darwin,windows}.go`：Linux/Android、macOS、Windows 分平台提供进程生命周期适配；Windows 采用进程组和系统 taskkill 子进程树清理，需真实 Windows 验证。
- `backend/internal/gitworktree/allocation_lock.go`、`manager.go`、`binding.go`、`ensure.go`：Git Worktree 的创建、绑定、释放、删除加入按仓库范围的跨进程文件锁，防止多服务进程同时为同一个 Task 分配不同 Worktree。锁目录拒绝符号链接，5 秒有界等待。
- `backend/internal/gitworktree/checkpoint_process_test.go`：使用真实 Go 测试启动延迟写文件子进程，取消时验证子进程不再运行。
- `backend/internal/gitworktree/allocation_lock_test.go`：5 个独立进程并发创建同一任务，只允许单一绑定；检查符号链接目录拒绝。
- `scripts/workbench/authenticated-smoke.mjs`：通过实际本机服务调用只读 GET，检查匿名拒绝、非法对话拒绝和已认证响应契约；默认仅允许 localhost/127.0.0.1/[::1]，不打印凭据，不自动探测外网。
- `scripts/workbench/authenticated-smoke.test.mjs`：模拟服务验证检测器本身，模拟结果不算真实服务验收。
- `scripts/workbench/integration-gate.mjs`：整合新隔离测试与可选 `--live-api`，实时门禁缺少环境或凭据时显示 blocked，不报假通过。

## 真实执行过的测试

1. `GO111MODULE=off GOTOOLCHAIN=local go test -race -count=1 -timeout 180s ./...`（gitworktree 包）：通过。
2. `GO111MODULE=off GOTOOLCHAIN=local go test -race -count=1 -run 'TestCheckpointCancellationStopsChildProcessTree' -timeout 75s`：通过。
3. `GO111MODULE=off GOTOOLCHAIN=local go test -race -count=1 -run 'TestConcurrentAllocationAcrossProcessesSingleWorktree|TestAllocationLockRejectsSymlinkDirectory' -timeout 120s`：通过。
4. `node --test scripts/workbench/authenticated-smoke.test.mjs`：3 个 Node 测试通过（模拟本地服务器）。
5. `node scripts/workbench/integration-gate.mjs --worktree-only`：无失败，3 项通过、15 项阻塞；阻塞是环境或未执行，不是通过。
6. `GOOS=windows/darwin/android GO111MODULE=off GOTOOLCHAIN=local go test -c`：交叉编译通过（不代表实际目标平台运行通过）。

## 尚未通过的验收门槛

本 Linux 环境只有 Go 1.23.2，缺 Go 1.26.1 正式工具链及主工程依赖，无法编译并启动完整后端，且缺 pnpm 前端完整依赖、Flutter SDK、真实 Windows ConPTY、Android Release 和 iOS 测试环境。未对真实认证 API 运行 `--live-api`。整个 Agent 调度→终端→审批→合并端到端仍未在完整服务验证，继续归入待验收。

## 真实本机认证 HTTP 复核方式

在已运行、真实登录且可信的本机服务上设置 `AMITIA_WORKBENCH_BASE_URL`、`AMITIA_WORKBENCH_CONVERSATION_ID` 和 `AMITIA_WORKBENCH_AUTHORIZATION` 或 `AMITIA_WORKBENCH_COOKIE`，然后使用 `node scripts/workbench/integration-gate.mjs --live-api`。认证凭据必须来自真实本机会话，不写入源码或报告。此门禁只验证任务列表读取协议；不声称代替 Worktree 合并和设备验收。
