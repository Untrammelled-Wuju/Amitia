# Workbench 第 12 轮：任务工作树幂等分配与只读审查

## 目标与范围

沿用 Git Worktree Manager、Workspace Mount Registry、Git Engine 和既有 `/api/workspaces/git` 认证路由，避免新建 Git 执行系统。此轮实现任务级幂等 Worktree 申请、自动恢复已绑定工作树、已绑定工作树的审查快照读取和 Electron/Web/Flutter 查看入口。

## 业务合同

1. 调用 `/api/workspaces/git/worktrees/create` 时，若传入合法 `taskId` 和 `conversationId`，使用 `EnsureTask`，同仓库中相同任务返回既有工作树和挂载 URI；分支冲突或任务归属不一致返回冲突。未绑定的普通创建仍保持原语义。
2. 创建时将任务的初始 Git HEAD 写入绑定元数据 `baseHead`，作为审查提交和未提交变更的基准；此字段不存储访问令牌，也不由调用方指定。
3. 新增 `POST /api/workspaces/git/worktrees/review`。必须提供仓库 Workspace URI、托管 Worktree ID、原任务和对话 ID，可额外提供 `expectedHead`；服务端验证绑定身份、托管目录和当前 Workspace 挂载。接口只读，不执行合并、提交或文件修改。
4. 审查返回 Git 状态、相对初始提交的受限 Diff、冲突数、未跟踪文件数、完整性标记。历史旧绑定若缺少 `baseHead`，只能查看当前 HEAD 的未提交修改，明确标记 `reviewIncomplete=true`，不可误判为全量任务结果。
5. 持久化异常、绑定不符、工作树消失或版本变化均拒绝返回成功。新建工作树完成但 Workspace 挂载登记失败时，尽力解除任务绑定并执行 Git 非强制回滚；若清理失败保留明确错误，不删除脏工作目录。
6. Web 和 Flutter 复用现有 Worktree 列表，提供独立的只读审查入口。手机只显示有界长度的 Diff，避免在弹窗加载无限文本。

## 本轮范围边界

此轮还没有对接真正 Coding Agent 的任务调度器，API 中的 `taskId`、`conversationId` 仍来自已认证调用者，绑定元数据不能作为单独的授权凭据。正式自动分配需要在 Agent 运行时由可信服务端导入其任务身份，不可让模型伪造 `taskId` 获得任务归属。自动提交、合并、强制清理、分布式锁及生产环境多设备联调不在此次完成范围。

## 验收方法

通过实际 Git 仓库单元测试覆盖首次申请、重试幂等、跨进程对象恢复、不同对话冲突、并发 12 次申请、脏工作区保留、基线提交在新提交后不变和遗留绑定兼容。按 Go 1.23.2 运行独立 `gitworktree` 测试；完整项目要求 Go 1.26.1，需在符合要求的构建环境再次运行 HTTP 集成与客户端构建。前端静态语法验证、Schema 正反例、压缩包路径/内容校验独立记录。
