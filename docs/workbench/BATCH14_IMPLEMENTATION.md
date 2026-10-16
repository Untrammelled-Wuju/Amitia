# Workbench 第 14 轮：Coding Worker 成果审查回传

## 完成本轮实现的范围

在 Multi-Agent Coordinator 原有 Snapshot 中保留 `workerRole`，仅对 `coding_worker` 附加 `worktreeReview`。协调器在内存内保留来源 Workspace URI（`json:"-"`），避免无鉴权泄露该字段；父 Agent 的 `harness_multi_agent_delegate`、`harness_multi_agent_status` 以及 `harness_multi_agent_wait` 复用相同的审查摘要逻辑。

父 Agent 查询结果前仍必须通过既有父任务/Space/Conversation/Character 身份校验。审查时还要求当前 ExecutionContext 的 Workspace 根 URI 与协调器记录的 Workspace 相等，然后以任务 ID、对话 ID 和父 Workspace URI 只读查询任务绑定的工作树。仅允许唯一匹配的托管工作树，审查结果必须与再次检查的工作树 ID、提交 HEAD、绑定身份一致。

工作树审查摘要包括：Git 基线 HEAD、当前 HEAD、增删行数、变更文件数、未跟踪文件数、冲突文件数、是否截断、是否需要人工审查。不会将大段源代码或完整 Diff 复制进模型工具响应；查看完整 Diff 仍沿用已认证的 `POST /api/workspaces/git/worktrees/review` 接口。

## 状态规则

- `pending`：子 Agent 还未终止，不进行审查读取。
- `ready`：能够读取完整 Git 快照，仅表示可供父 Agent 审查，不代表代码已经测试或人工批准。
- `incomplete`：工作树有未跟踪文件、冲突、Diff/状态截断、或缺少可证实的基线。禁止据此声明验收完成。
- `unavailable`：审查服务未配置、当前 Workspace 不匹配、审查读取失败、或目标已解除绑定，均明确返回失败标记，防止假成功。

任何状态均为 `requiresApproval: true`。不自动合并、不提交代码、不解绑或删除工作树。子 Agent 的 `ResultRef` 继续原样保留，但无法仅凭这段文字把测试标记为“已执行并通过”。

## 已实施验证及限制

已添加针对访问范围、任务/对话绑定、并发重复匹配、状态不一致、局部结果、失败返回和父 Workspace 一致性的 Go 用例。该环境没有项目要求的 Go 1.26.1 和 Flutter SDK；完整服务、HTTP 协议、正式前端和真机需在满足工具链的环境重新编译验证。

本轮不是生产验收。下一轮建议为任务工件生成带版本号的检查点、增加实际测试命令证据和人工审批签名，随后实现安全合并及回滚。
