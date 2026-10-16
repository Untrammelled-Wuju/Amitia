# Workbench 第 32 轮：Coding Worker 验证证据与 Git 环境隔离

## 研发定位

以第 31 轮累计源码为唯一继续基线，不删除前 31 轮文件。本轮针对 Coding Agent 中 Worktree 测试与合并的可信执行证据修复可复现的集成风险，并增设真实临时 Git 仓库生命周期测试。没有新增第二套 Git 引擎，不修改既有 API 授权语义，也不擅自启动发布或推送 Git。

## 真实源码变更

- `backend/internal/gitworktree/git_environment.go`：统一净化 `GIT_*` 进程环境，防止从父进程继承的仓库、索引、Worktree 和配置覆盖影响受控 Git 命令；受控 Go 测试强制 `GOFLAGS=`、`GOENV=off`、`GOWORK=off`、`GO111MODULE=on`、`GOTOOLCHAIN=local`、`GOPROXY=off` 和 `GOSUMDB=off`。
- `backend/internal/gitworktree/manager.go`：仓库 Git 命令与仓库过滤器检查统一采用隔离环境；维持现有 Hooks 禁用与非交互模式。
- `backend/internal/gitworktree/checkpoint.go`：Go 测试证据改为 `go test -json -count=1` 及 JSON 结构化用例成功事件校验，只有真实用例事件、进程退出码为 0、输出未超过证据上限且修订一致时，才可作为后续人工审批前提。原始普通文本 `ok ...` 不再足以判定通过。
- `backend/internal/gitworktree/checkpoint_test.go`、`git_environment_test.go`：覆盖进程环境注入、跳过测试的 GOFLAGS 注入、用户输出伪造成功标记、跨块 JSON、超长输出与真实已执行测试。
- `backend/internal/gitworktree/coding_lifecycle_test.go`：真实 Git 仓库中测试双任务 Worktree 隔离、真实 Go 测试、恢复、审批拒绝、授权审批、快进合并、重复合并拒绝及第二任务因目标已分叉而拒绝自动合并。
- `scripts/workbench/integration-gate.mjs`：新增 `--worktree-only` 隔离定向验收模式（无需符合完整后端的 Go 1.26.1 才可进行 Go 标准库 Worktree 模块隔离验证）。保留完整 Go 1.26.1 构建、前端与 Flutter 缺失条件的阻塞记录。`--worktree-only` 的退出成功不代表整个项目满足发布门禁。

## 本环境真实执行与结果

1. `GO111MODULE=off GOTOOLCHAIN=local go test -race -count=1 -timeout 180s`：`backend/internal/gitworktree` 全包测试通过，实测包含真实 Git 分支与 Git 工作树操作。
2. `node --check scripts/workbench/integration-gate.mjs`：脚本语法通过。
3. `node scripts/workbench/integration-gate.mjs --worktree-only`：隔离 Git 门禁通过，完整工程构建依赖与生产验收项维持阻塞。
4. 远程 Windows 设备查询状态为离线，不具备对应系统的实际验证条件。

## 明确不成立的结论

以上均不构成 Go 1.26.1 完整后端编译成功、认证 HTTP 联调、真实用户点击审批、真正 Agent 语言任务调度、终端 PTY、Electron/Flutter 构建或多设备协同验收通过。`RunCheckpointTest` 将用户工程测试代码在执行环境内运行，完整沙箱及设备权限防护仍需正式联调审查。

## 后续优先级

具备 Go 1.26.1 与工程依赖后，直接运行完整构建及真实认证 HTTP 联调；随后把实际 Harness 调度、Worktree、终端、测试、审批和安全合并串联到同一进程与真实设备。不要以模拟 Manager 生命周期替代端到端证据。

## 进度口径

第 31 轮累计研发范围估算为 81%，第 32 轮由于可信测试证据、Git 环境隔离及真实隔离 Git 集成测试暂估为 **82%**。80%→85% 完整工程构建集成阶段仍未验收，进度不代表可上线程度。
