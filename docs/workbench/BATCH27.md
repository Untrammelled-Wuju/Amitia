# Workbench 第 27 轮：跨进程合并保护与 Coding Worker 验收状态

## 交付范围

本轮继续维护第 1～26 轮源码，并将 Worktree 安全合并和 Multi-Agent 任务验收接入统一、可测试的状态协议。增量逻辑按平台锁、Git 合并事务、Agent 验收策略、父任务聚合、终端会话身份、JSON Schema 与测试分开存放。

### Git 快进合并的跨进程互斥

使用每个 Git 仓库在 U-Ai 管理目录下专属 `.merges/.apply.lock`，Linux/macOS 用 `flock`，Windows 用 `LockFileEx`。申请锁有 5 秒上限，可被上下文取消。`ApplyApprovedMerge` 在取得操作系统锁之后再读取目标分支、HEAD、工作区状态、审查检查点、测试结果及用户批准；锁覆盖审计的 pending 写入、真实 Git 快进合并及最终审计结果。

本次锁的范围是**使用相同 U-Ai 数据目录和 Worktree Manager 的进程**。外部 Git 客户端不受 U-Ai 进程锁控制；Git 自身的引用锁是另一层保护。流程仍然只允许 `--ff-only`，不支持强制合并、自动推送、自动解决冲突。

### Coding Worker 验收门禁

`EvaluateCodingWorkerVerification` 只对成功结束、Git 审查完整、具有特定检查点、实际测试通过、人工批准并具备匹配的合并审计的任务逐级放行。单独的子 Agent 成功、终端退出码 0 或 Diff 可以读取，都不会被提升为“测试已通过”。来源与目标提交不一致的合并审计会被标记为需要人工核查。

`verificationGate` 作为每个 `worktreeReview` 的只读补充输出。`verificationSummary` 作为父任务聚合输出，仅统计 Coding Worker，区分等待、阻断、已审批待手动合并及已确认合并。`allMergesConfirmed` 仅表示所有该协调任务的 Coding Worker 均有匹配的本地合并审计，**不代表完整工程构建、发布或生产质量验收已通过**。

### 桌面认证会话适配

DesktopSession 的可信认证上下文可能保留 `SessionID`，但没有单独 `DeviceID`。终端 API 的 owner 格式本来允许这种情况，父 Agent 诊断却拒绝读取。本轮父 Agent 改为按照真正的 DesktopSession 身份及原 owner 格式查找相同会话的记录；仍严格要求可信 LocalUI 主体、匹配 Space、对话、Worktree、Task ID 和非空认证会话。空认证会话不得通过诊断适配器读取历史。不会向 Cloud Core、其他设备、其他对话开放读取权限。

### 验证范围

- 独立真实 Git 仓库：Git Worktree 全模块 race 测试，跨进程锁超时、释放、符号链接拒绝、两个进程同时竞争同一审批合并
- Windows amd64 和 macOS arm64 平台专属锁代码交叉编译（不等于实机验证）
- 父 Agent 审查适配器：隔离 Go race 测试；严格作用域、失败关闭、聚合验收门禁
- 纯策略模块：worker 状态、测试、审批、合并证据及终端退出码伪阳性回归
- 两份 JSON Schema：结构验证与正反例验证
- 累计 ZIP：原始源码 + 本轮累计替换包与本轮完整源码逐文件一致

未通过或未执行：完整 Go 1.26.1 工程构建、真实 HTTP→Agent→Multi-Agent 全链路、Flutter Release、Electron Windows 实机运行、Windows/macOS 跨进程锁实机压力测试。后续应优先补齐 SDK 环境与端到端测试，避免将隔离验证误视为正式验收。
