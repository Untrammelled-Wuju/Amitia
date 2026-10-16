# U-Ai Workbench 第 34 轮：检查点审批并发围栏与测试执行证据

本轮继续基于第 33 轮累计源码进行定向修复。当前的 84% 仅为研发范围估算，不代表完整服务或真机生产验收率。所有代码均保存到源码目录，未修改编译产物。

## 真实问题与修复

- `backend/internal/gitworktree/checkpoint.go`：检查点准备、读取、测试前后状态及人工批准共用跨进程 Git 审查锁，与安全合并串行化；测试进程运行阶段不长期占用审查锁。
- 每次启动新验证前先原子写入失败关闭的“未验证”记录，防止上一轮已通过结果在重新测试期间或取消后被继续用于批准。仅当前最新测试运行能写回其结果；迟到的先前运行不能覆盖新任务尝试。
- 父级请求取消后使用有界、独立取消上下文持久化失败结果，不因 HTTP 请求断线忽略取消证据。若落盘失败，保留启动时的未验证标记，审批仍无法从旧成功结果放行。
- 测试失败、超时、取消的实际终态参与 `Passed` 判断，不允许出现取消后 `Passed=true`。明确保留现有自动清理测试进程组逻辑。
- `backend/internal/gitworktree/git_environment.go`：测试子进程环境按最小必要字段白名单传递，仅保留系统工具定位、缓存和语言/临时目录字段；阻止主服务 JWT、数据库、云端密钥、代理和 Go 编译器注入变量默认进入测试环境。这不是进程级 OS 沙盒，尚不能将运行不可信代码标为完全隔离。
- `scripts/workbench/integration-gate.mjs`：在隔离 Go 测试路径清理危险环境、读取结构化 Go 测试完成事件，若实际测试用例数量为 0 则门禁失败；保存 `passingTestEvents` 供验收追溯；构建临时目录执行后清理，避免遗留中间二进制。

## 已执行的定向验收

- `GO111MODULE=off GOTOOLCHAIN=local go test -race -count=1 -timeout 180s ./...` 在 `backend/internal/gitworktree`：通过，覆盖真实 Worktree、检查点、审批、合并及本轮新增的取消恢复场景。
- `TestCheckpointPendingAndCancelledRunCannotApproveStaleSuccess`：真实 Go 测试通过 → 重新测试期间拒绝审批 → 取消测试持久化失败 → 重建 Manager 恢复拒绝状态 → 新测试通过后才可批准。
- `TestCheckpointReviewFencedByMergeLockAcrossManagers`：两个 Manager 竞争审查锁时写请求等待或超时，释放后正常恢复。
- `TestCheckpointTestEnvironmentDoesNotInheritHostSecrets`：测试进程环境不会直接继承敏感变量、网络代理或任意 CGo 编译器参数。
- `GO111MODULE=off go test -race -json -count=1 buffer.go buffer_test.go`：终端输出游标/大缓冲截断 3 项通过，属于独立单元测试，不代表真实 PTY 已通过。
- Windows amd64、macOS amd64、Android arm64 的 gitworktree 包交叉编译成功；未在目标系统运行。
- `node scripts/workbench/integration-gate.mjs --worktree-only`：检查结构化 Go 测试事件、命令返回码及环境可用性；所有被标记为“blocked”的完整工程验收未被当作通过。

## 尚未验收

本容器仅 Go 1.23.2，缺少 Go 1.26.1 主工程工具链、全量依赖、Flutter SDK、Electron 环境与真实设备；因此无法完成完整后端启动、实际认证 HTTP、手机 Release、Windows ConPTY、Coding Agent 真实调度至人工审查的端到端联调。运行 AI 生成的测试代码仍应置于独立操作系统安全沙盒，当前环境变量白名单只是额外的最小化保护，不能单独构成沙盒边界。
