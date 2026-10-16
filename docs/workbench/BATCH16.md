# 第 16 轮 · 人工批准后的 Git Worktree 安全合并

本轮将已有 Worktree 任务检查点、测试证据与人工审批接入可实际执行的目标 Git 分支快进合并（fast-forward only）。合并不会被 Agent 自动调用，不支持 `--force`、自动冲突解决、推送远端或清理任务工作树。

## 双阶段协议

- `POST /api/workspaces/git/worktrees/merges/preview`：使用原任务 Worktree 绑定验证审批、最新测试通过、固定 HEAD、干净工作树、目标当前分支和目标 HEAD，检查目标提交是来源提交的祖先；返回与目标仓库路径、分支、HEAD、来源 HEAD 和检查点版本绑定的 planId。
- `POST /api/workspaces/git/worktrees/merges/apply`：仅允许认证上下文中的可信本机 UI 调用；审批者和执行者身份必须完全一致。每次执行重新预检，并核验 planId 未变。执行前将 pending 审计原子落盘，再执行 `git merge --ff-only --no-edit <source SHA>`；执行结束后持久化结果。
- `POST /api/workspaces/git/worktrees/merges/audit`：查询持久审计记录。任何存在审计记录的任务不得再次执行合并，包括成功、失败和 pending 状态，防止不确定结果下重复写入。

## 不允许的情形

未审批；最后测试失败；来源和检查点提交不一致；来源代码脏；目标代码脏；目标切换分支；目标HEAD变化；无法快进；合并操作者不是审批主体；审计目录符号链接；已尝试合并；跨设备非可信 UI 调用。

## 仍需正式验证

定向 Go 测试覆盖真实仓库合并与并发；但项目完整 Go 1.26.1 编译、Flutter Release、Electron 启动及真实多端操作未在当前 Linux 容器完成。Git 在合并时会更新目标工作区文件，请务必在可信本机备份和确认目标分支后执行。由于外部 Git 进程不受当前进程内互斥锁控制，仍有跨进程 TOCTOU 竞争风险；正式发布前应补 OS 跨进程锁和全链路集成验收。

## Multi-Agent 结果回传

父 Agent 的 `status/wait` 查询额外读取同一任务的合并审计，并验证 TaskID、ConversationID、WorktreeID、来源提交、CheckpointRevision、ApprovedBy 与执行者。仅全部匹配且 appliedHead 与检查点 HEAD 一致时标记 `mergeState=applied`。未产生记录标记 `not_attempted`，进行中的记录标记 `pending`，存在异常或身份不一致标记 `unavailable`，失败记录标记 `failed-needs-review`。无审批记录或测试未通过时不得报告合并结果。相应的 Coding Worker Review V1 Schema 已同步扩展。
