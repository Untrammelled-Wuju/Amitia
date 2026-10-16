# Workbench 第 11 轮：Git Worktree 任务关联与安全解绑

## 基线及实现范围

基线为第 10 轮累计完整源码。此轮在已有 Git Worktree 生命周期上增设了可持久化的任务关联元数据；桌面 Web/Electron 和 Flutter 工具箱可以在创建工作树时按需填入任务 ID 和对话 ID，并能显示和显式解除关联。任务绑定仅用于工作环境关联与误删防护，**不作为用户、Agent 或设备的鉴权凭据**。API 继续使用现有工作区访问控制。

当前不包含 Agent 调度器自动为任务创建工作树、任务结束自动回收、跨设备 Git 执行、沙箱级进程隔离或不同 Space 之间的服务端任务所有权校验。生产使用前仍需要将该元数据与真正的任务身份及 Device Mesh 授权绑定。

## 数据模型

`GitWorktree.Entry.taskBinding` 为可选对象：

```json
{
  "taskId": "task-2026-1",
  "conversationId": "conv-2026-1",
  "boundAt": "2026-10-14T11:00:00Z"
}
```

数据保存在应用私有 `dataRoot/workspaces/agent-worktrees/<repository-key>/.bindings/<worktree-id>.json`，不放进 Git 仓库，也不污染工作树。每个 Git 共用仓库以已有 SHA-256 派生的仓库 key 隔离，Worktree ID 为 32 位随机十六进制字符。任务与对话标识长度上限 128 字符，只允许字母、数字、点、下划线、冒号、连字符，且必须以字母或数字开头。

单实例内 Bind/Release/Create/Remove 在同一进程互斥；绑定写入先创建私有临时文件、写入并同步，再以同目录重命名提交。绑定文件和绑定目录在读写时检查文件类型及符号链接；损坏或异常绑定记录采用拒绝删除策略，不允许自动忽略。该保证限于单运行进程，不代表多进程分布式锁，也不保证断电时的数据库级事务一致性。

## API

均复用当前已认证的 `/api/workspaces/git` POST 路由。

- `POST /worktrees/create`：在原有字段外可传 `taskId`、`conversationId`；两字段必须同时提供且合法。未填写时保持原有无绑定行为。先创建和挂载，再写绑定，绑定失败执行安全回滚。
- `POST /worktrees`：每个受管条目新增可选 `taskBinding`；绑定信息在新的 Manager 实例及进程重启后读取。
- `POST /worktrees/release`：请求体为 `workspaceUri`、`id`、`taskId`、`conversationId`，只有完全匹配存储记录时才解除绑定；此操作不会移除 Git 工作树、删除分支或改变工作区挂载。该接口不是任务权限授予接口。
- `POST /worktrees/remove`：仍为非强制删除；若绑定尚在，则返回 HTTP 409 Conflict。客户端界面默认禁止删除已绑定工作树。

## 测试

在具备 Git 的 Go 1.23.2 隔离纯标准库测试环境下运行 `GO111MODULE=off go test -race -count=1`，覆盖了创建/列出/删除、绑定幂等、重绑冲突、错误身份解绑、重建 Manager 后读取、绑定文件异常拒绝、符号链接阻断，以及 24 个并发创建调用不能超出 16 个的仓库配额。上述检查不代替 Go 1.26.1 全工程构建。

Web 端使用 TypeScript 转译器检查改动脚本语法；Flutter 修改了已有服务方法及工具箱交互，但缺少 Flutter SDK，尚未执行 `flutter analyze`、Android/iOS 构建与真机测试。HTTP 注册使用静态连线校验，不声称完成端到端运行测试。

## 后续衔接

下一步应由可信的 Agent Task Runtime 调用“创建并绑定”入口，使用服务端确认的任务 ID、Conversation ID、Space ID 和目标设备身份，避免前端可伪造标识的归属风险。任务启动后要维护单独的环境记录，任务结束仅将工作树标记为可清理，不应静默删除未提交数据。后续可接入差异审查、结果汇总、人工确认合并以及安全生命周期清理。
