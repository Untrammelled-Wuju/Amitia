# Workbench 第 26 轮：父 Agent 任务终端诊断与安全身份桥接

## 本轮目标

将第 25 轮的终端运行凭据以严格只读方式接入父 Agent `harness_multi_agent_status` / `harness_multi_agent_wait` 返回的 Coding Worker Worktree 审查摘要。终端运行凭据只能作为运行诊断；Go 测试证据、审批和合并状态继续由原有 Git Worktree 检查点及审计记录决定。

## 模块边界

- `backend/internal/workbenchterminal/task_receipt_store.go`：保留原有 `List` 的“已知活跃进程集合”恢复语义，新增 `ListRecorded`，仅读取磁盘最后状态，不臆测会话是否在线。
- `backend/internal/workbenchterminal/task_receipt_runtime.go`：提供默认记录库的只读构造入口，不公开目录结构给父 Agent。
- `backend/cmd/server/coding_worker_terminal_evidence.go`：专用适配器从已认证请求上下文读取可信本机 Actor，派生与工作区终端一致的所有者键；过滤对话、任务及被审查的 Worktree URI，只汇总有界元数据。
- `backend/internal/interaction/multi_agent_worktree_evidence.go`：`CodingWorktreeReview.terminal` 独立诊断结构，不参与 `testState`、`approvalState` 或 `mergeState` 判断。
- `backend/cmd/server/coding_worker_review.go`：仅在父 Agent 所属工作区通过作用域校验、任务 Worktree 审查成功后读取终端凭据；无法授权或无法读取时返回 `terminal.state=unavailable`，不返回其他用户历史。
- `contracts/workbench/coding-worker-review-v1.schema.json`：新增终端摘要约束、固定证据类别及错误码，不允许声明“测试通过”。

## 权限和一致性

使用 HTTP 认证 Actor 的 SpaceID、DeviceID 和 SessionID 推导终端所有者，不信任聊天参数提供的认证会话号。调用必须为 `PrincipalLocalUI` 且 `IsLocalTrusted`，SpaceID 必须与当前父 Agent 范围一致。终端元数据来自当前认证会话，跨登录会话无法读取；远程 Device Mesh 不允许假冒本机可信 Actor。

`running` 是最后保存到磁盘的状态，**不能据此断言进程仍在运行**；结果将其标为 `unverified`。返回数量至多覆盖当前凭据库的保留范围和读数上限，不构成全量永久审计。

## 验证

- 隔离终端 Go 模块：`go test -race -count=1 ./...` 通过；新增 `ListRecorded` 与原有实时恢复语义回归。
- 隔离父 Agent Git 审查服务：`go test -race -count=1 ./cmd/server` 通过；覆盖已授权附加、缺失范围拒绝和记录服务故障。
- 隔离终端诊断适配服务：`go test -race -count=1 ./cmd/server` 通过；覆盖本机可信会话、跨 Space 拒绝、Worktree URI 过滤、认证会话号来源和元数据不冒充测试成功。
- 终端隔离模块 `GOOS=windows GOARCH=amd64 go test -c` 交叉编译通过。
- JSON Schema 的正反例检查通过。
- 全量 Go 1.26.1、Electron 启动、Flutter Release、真实父 Agent 任务端到端执行未在当前环境完成。

本轮依旧保留安全失败语义：没有可信 Actor 时诊断不可用，**不会**为提高功能显示率而绕过已有认证或 Worktree 绑定。未自动执行测试、批准、合并或删除工作树。
