# V9审计问题 → 修复步骤映射矩阵

**制定日期：** 2026-07-03
**依据审计：** V9.0完整代码审计报告
**修复文档：** Amitia-develop_6_168步V3.1审计问题详细修复实施文档_V2.1.md
**代码基线：** 02dd1d79

## V9审计问题清单与映射（83项）

### P0-阻断级（13项）

| V9编号 | 描述 | 关联原B/R | 关联168步 | 主修步骤 | 状态 |
|--------|------|-----------|----------|----------|------|
| P0-01 | InteractionRun非权威、缺少Scope/request_id | B-04,B-10,B-11,R-06,R-18 | 8,17-19,30,31,37 | R06,R07,R08,R09,R10 | OPEN |
| P0-02 | 取消/替代/并发幂等缺失 | B-01 | 37,39-41 | R07,R08,R11 | OPEN |
| P0-03 | 心理/关系/需求固定增量、无Change Budget | B-03,B-16 | 77-112 | R19,R20,R21,R22 | OPEN |
| P0-04 | 无统一Personality/Belief/BDI/Appraisal管线 | B-02 | 43-130 | R17,R18,R19,R23,R24,R25 | OPEN |
| P0-05 | 主动消息伪造用户消息、绕过安全 | B-09 | 131-142 | R26,R36 | OPEN |
| P0-06 | DeliveryIntent缺失、投递前标记SENT | R-03 | 138-142,151-155 | R32,R33,R39 | OPEN |
| P0-07 | 删除墓碑未参与检索、未传播 | B-17,R-12 | 57-74,162,167 | R16,R38,R42,R44 | OPEN |
| P0-08 | 迁移无备份、无验证 | R-05 | 163-167 | R47,R48 | OPEN |
| P0-09 | Outbox无租约/缺少死信原子性 | — | 38,158,160,162 | R28,R30 | OPEN |
| P0-10 | 无启动顺序/Drain/依赖就绪 | R-17 | 163-168 | R51,R52 | OPEN |
| P0-11 | Qdrant/SurrealDB反向覆盖SQLite | R-02 | 57-74,145-150 | R16,R44,R46,R49 | OPEN |
| P0-12 | release构建产物被手工修改 | — | 1,2,163,168 | R03 | OPEN |
| P0-13 | 测试证据未绑定代码哈希 | — | 1,2,16,168 | R01,R02,R04 | OPEN |

### P1-高风险（50项）

| V9编号 | 描述 | 主修步骤 | 状态 |
|--------|------|----------|------|
| P1-01 | Outbox重复投递 | R28 | OPEN |
| P1-02 | Outbox租约过期误覆盖 | R28 | OPEN |
| P1-03 | DeadLetter丢失 | R30 | OPEN |
| P1-04 | DeadLetter重放策略错误 | R30 | OPEN |
| P1-05 | 副作用重复创建 | R31 | OPEN |
| P1-06 | 日志打印冒充发布成功 | R14,R29,R31 | OPEN |
| P1-07 | 检查-写入竞态 | R08 | OPEN |
| P1-08 | 取消不传播到LLM | R11,R13 | OPEN |
| P1-09 | 替代语义不完整 | R11 | OPEN |
| P1-10 | 多套调度器并存 | R12,R37 | OPEN |
| P1-11 | 队列无持久化 | R12 | OPEN |
| P1-12 | 优先级不生效 | R12 | OPEN |
| P1-20 | 主动任务幂等键错误 | R07,R37 | OPEN |
| P1-21 | 主动任务重试加当前时间 | R07,R37 | OPEN |
| P1-22 | channel=all退化为web | R34,R39 | OPEN |
| P1-23 | OutputLease缺失 | R32,R33,R39 | OPEN |
| P1-24 | 投递状态假SENT | R32,R34,R39 | OPEN |
| P1-25 | 向量直接作为权威内容 | R16,R38 | OPEN |
| P1-26 | 检索绕过AuthorityFilter | R16,R38 | OPEN |
| P1-27 | 主动Prompt写入用户消息 | R36 | OPEN |
| P1-28 | 语音/文本各自拼Prompt | R24,R34,R40 | OPEN |
| P1-29 | context.Background裸goroutine | R13 | OPEN |
| P1-30 | 请求完成后继续写入 | R09,R13 | OPEN |
| P1-31 | SafetyGovernor未接入生产 | R17,R23,R26 | OPEN |
| P1-32 | Prompt自由字符串拼接 | R25 | OPEN |
| P1-33 | Context Loader缺数据契约 | R13,R15 | OPEN |
| P1-34 | Belief Resolver未接入主链 | R15,R18 | OPEN |
| P1-35 | 写入不在同一SQLite事务 | R10,R15,R21 | OPEN |
| P1-36 | 固定EnergyDelta每轮-0.01 | R09,R20 | OPEN |
| P1-37 | 依赖熔断/降级缺失 | R29,R53 | OPEN |

### P2-高风险（20项）

| V9编号 | 描述 | 主修步骤 | 状态 |
|--------|------|----------|------|
| P2-01 | Outbox缺少external_operation_id | R28 | OPEN |
| P2-02 | DeadLetter缺少唯一约束 | R30 | OPEN |
| P2-07 | 检索无scope隔离 | R16,R38 | OPEN |
| P2-08 | 主动渠道解析不完整 | R39 | OPEN |
| P2-09 | delivery_id不稳定 | R32,R39 | OPEN |
| P2-10 | request_id使用UnixNano | R06,R07 | OPEN |
| P2-11 | UpdateMetadata缺少版本校验 | R06,R08 | OPEN |
| P2-12 | 状态转换不完整 | R06,R11 | OPEN |
| P2-13 | JSON文件作检查点 | R12 | OPEN |
| P2-14 | 源码新增解释性注释 | R03 | OPEN |
| P2-15 | 迁移无兼容窗口 | R06,R48 | OPEN |
| P2-16 | 旧表直接删除 | R06 | OPEN |
| P2-17 | 启动无迁移前置检查 | R48,R51 | OPEN |
| P2-18 | 信念冲突无CONFLICT/UNKNOWN | R18 | OPEN |
| P2-19 | 测试命令输出格式不统一 | R01 | OPEN |
| P2-20 | 提交混合修改互不相关模块 | R04 | OPEN |

## 统计

| 严重度 | 数量 | OPEN | IN_PROGRESS | CODE_DONE | TESTED | ACCEPTED |
|--------|------|------|-------------|-----------|--------|----------|
| P0-阻断 | 13 | 13 | 0 | 0 | 0 | 0 |
| P1-高风险 | 50 | 50 | 0 | 0 | 0 | 0 |
| P2-高风险 | 20 | 20 | 0 | 0 | 0 | 0 |
| **总计** | **83** | **83** | **0** | **0** | **0** | **0** |
