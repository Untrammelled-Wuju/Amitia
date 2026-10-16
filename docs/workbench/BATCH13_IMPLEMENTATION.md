# Workbench 第 13 轮：Coding Agent Worktree 自动分配

## 本轮接入范围

`harness_multi_agent_delegate` 标记为 `coding_worker` 的任务，经 `MultiAgentCoordinator` 分派时，先调用 `NewCodingWorktreeWorkerRunner`，然后才启动已有的异步 `UnifiedEntry` 子任务。工作树由现有 `GitHandler.CreateTaskWorktree` 调用第 12 轮的 `EnsureTask` 创建或重用，不建立第二套 Git 引擎。子任务收到新工作树的 Workspace ID、RootURI、Name 与 Kind；SpaceID、会话归属和设备身份保持不变。任务结束不自动删除和合并工作树，仍使用现有任务绑定和只读审查 API。

## 核心不变量

- 仅 `WorkerRole == coding_worker` 且 `Source == multi_agent` 的子任务尝试自动分配。普通子任务保持原行为。
- 必须有有效的任务 ID、父会话 ID、来源 Workspace ID 和根 URI。失败时拒绝执行，不回退到父 Workspace。
- 来源 URI 必须是标准 `amitia://workspace/@<mount-id>/` 根挂载形式，且 ID 与已授权 WorkspaceID 一致；包含子路径、查询或不同 ID 时拒绝。
- 设备端必须有本机 DeviceID；目标 WorkspaceDeviceID 非空且与本机不同，禁止本地分配。Cloud Core 不代替其他设备创建工作树。
- 新工作树必须具有正确的任务与会话绑定。工作树创建、任务恢复与重试都经已有 `EnsureTask` 幂等路径。
- 修复原 Git 根目录解析器对 Local Mount 的错误解析：Local Mount 使用真实的 canonical `LocalRoot`，Isolated Mount 仍使用私有隔离根目录，Remote Mount 拒绝作为本地仓库。
- 工作树分配后若子任务未正常完成，保留绑定及文件待审查；不能把未审查成果自动合并到父分支。

## 本轮测试

- Coding Worker 调度适配器：隔离 Go `go test -race -count=1`，覆盖传递真实 Workspace 身份、授权失败禁止派发、非 Coding Worker 不受影响、并发和恢复接口透传。
- 设备端分配适配器：隔离 Go `go test -race -count=1`，覆盖标准 URI、跨设备拒绝、非法 Workspace ID、嵌套路径、错误绑定及无本地身份。
- Git 根目录解析：抽取实际实现的隔离测试，`go test -race -count=1`，覆盖 Local Mount 与符号链接变化保护。
- `backend/internal/gitworktree`：真实 Git 引擎原有测试 `GO111MODULE=off go test -race -count=1` 通过。

## 尚未完成

完整项目使用 Go 1.26.1，而本轮 Linux 环境为 Go 1.23.2；本轮尚未执行完整 `backend/cmd/server` 编译、真实 `MultiAgentCoordinator` 与 SQLite/Workspace 挂载集成、Flutter/Electron/Android 构建。当前接口对合法本地 Coding Worker 生效；远程 Device Mesh 工作树分配、跨进程工作树互斥、任务完成事件驱动审查/自动审批合并均需后续实施。由于任务级工作树与主分支隔离，不能据此宣称整个 Codex 级 Harness 完成。
