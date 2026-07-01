# 分层场景样本框架

对应实施计划：V3.1 第3步。

当前首批样本位于 `backend/internal/psyche_testdata/cases.json`，共 80 条，覆盖十类场景：

| 类别 | 范围 |
|---|---|
| ordinary_chat | 普通交流 |
| complex_emotion | 复杂情绪 |
| relationship_conflict | 关系冲突 |
| user_correction | 用户纠正 |
| proactive_message | 主动消息 |
| multi_role | 多角色 |
| cross_channel | 跨渠道 |
| safety | 安全 |
| fault | 故障 |
| runtime_collaboration | 运行时协作 |

每条样本包含输入事件、前置状态、任务优先级、允许变化区间、禁止行为、期望交互状态、输出特征、Fake LLM、Fake Channel 和故障依赖。第164步补齐到不少于 500 条。


## 运行时协作场景（第164步补充）

以下场景覆盖计划第160-168步定义的运行时协作行为，与 untime_collaboration 类别合并使用。

| 场景ID | 目标 | 检查点 |
|---|---|---|
| RC-01 | 连续输入时旧交互被取消，取消结果提交率为0 | 旧InteractionRun状态为CANCELLED，新Run正常完成 |
| RC-02 | P0/P1高优先事件在队列高压下不被普通主动消息阻塞 | 优先级调度器锁定P0通道，队列年龄不超过P0上限 |
| RC-03 | SQLite/Qdrant高严重度一致性差异为0 | Reconciliation巡检报告0个high/critical diff |
| RC-04 | 派生索引可从SQLite权威记录重建 | 重建后Qdrant向量与SQLite来源完全匹配 |
| RC-05 | LLM熔断打开后不每轮重复等待，半开后只允许探测流量 | 熔断状态=OPEN/HALF_OPEN，请求快速失败或降级 |
| RC-06 | 渠道超时后实际成功，迟到回执无重复推送 | 用户可见消息1条，DeliveryIntent最终状态=ACKED |
| RC-07 | 工具UNKNOWN结果不形成成功事实 | 补偿事务写入，事实状态=UNCERTAIN |
| RC-08 | 进程优雅关闭后无永久PROCESSING任务 | 所有InteractionRun在启动后恢复或标记 |
| RC-09 | 进程非正常重启后无僵尸租约或版本断层 | OutputLease全部过期，StateVersion无跳号 |
| RC-10 | 删除墓碑生效后相关数据不再进入检索/主动/Prompt/模型请求 | 墓碑过滤后检索结果0条 |
| RC-11 | OutputLease：并发触发+用户输入时只一个主输出或明确合并 | 合并后ExpressionPlan提交1个DeliveryIntent |
| RC-12 | Delivery Coordinator：多渠道回执乱序时状态收敛 | DeliveryIntent最终状态正确，无重复推送 |
| RC-13 | Trace因果链可查询完整路径 | FilterCausalChain/AggregateRuntimeMetrics返回正确聚合 |
| RC-14 | 前瞻记忆到期触发但完成后不再触发 | ProspectiveMemory trigger在完成后cancelled |
| RC-15 | Runtime Reconciliation自动修复僵尸租约和孤儿Outbox | 巡检前后计数差=已修复数 |
