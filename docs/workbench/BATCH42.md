# U-Ai Workbench Batch 42：跨进程检查点执行围栏与 Agent 恢复身份校验

## 本轮研发范围

基于 Batch 41 累计源码继续开发，完成编码任务测试执行期的跨进程互斥、绑定生命周期保护、父子 Agent 恢复作用域校验、持久化依赖图一致性校验、空 Diff 合并资格拒绝。研发范围完成度暂估 88%，不等同于生产验收率。

## 实现内容

- `gitworktree` 新增检查点测试运行专属文件锁。测试从准备记录到执行完成、写入结果一直持有锁；第二个 Manager 或另一进程重复运行同一 Worktree 测试时直接返回忙碌错误，不覆盖正在执行的证据。
- 任务绑定释放、显式检查点失效和工作树删除需要先获得同一测试锁，避免测试仍在执行时修改被测资源的归属或删除工作目录。并保持与原 `.merges` 审批合并锁独立，避免长时间执行测试阻塞其他任务的审查。
- 原始 Git 环境、审批、检查点、证据写入及快进合并规则保持不变；没有授予无审批合并权限。
- 恢复 Multi-Agent 记录时核验已存在 Goal 的 ID、修订及 Space/Conversation/Character 所属身份，防止已有 Goal 键碰撞被静默复用。
- 恢复 Assignment 列表时校验容量、深度、重复 ID、未知依赖、重复依赖以及环形依赖，拒绝无法稳定调度的持久化工作计划。
- 根据父 Goal 和原始 Assignment 身份核对恢复中的子 Interaction 的 Space、Character、隔离 Conversation 和 RequestID，防止另一个空间的持久化任务结果被采纳。
- Coding Worker 审查门禁拒绝无文件改动、源和目标 HEAD 相同的空合并意图。
- 新增原源码链接隔离 Go race 验证脚本 `scripts/workbench/recovery-isolated.mjs`，接入现有 Workbench 研发验证门禁。

## 验证与边界

- `gitworktree` 完整包使用真实临时 Git 仓库通过 Go 1.23.2 race 测试，其中包括本轮新增跨 Manager、跨进程的检查点锁与绑定生命周期检查。
- 纯恢复校验及 Coding Worker 审查门禁使用原 Go 源码链接与隔离契约桩运行，包含身份、依赖图及无 Diff 等负例；它不能代替完整 Interaction 服务依赖编译。
- Windows/macOS/Linux ARM64 的 `gitworktree` 交叉编译只证明目标代码可编译，不代表实际设备执行成功。
- `integration-gate.mjs --development-only` 明确区分研发验证通过和生产验收阻塞。
- 后端 Go 1.26.1 完整构建、真实 HTTP 身份鉴权、Agent 调度到完整人工审批合并、多端 Flutter/Electron、ConPTY、CDP、Device Mesh 和正式安全性能压测仍未验收。
